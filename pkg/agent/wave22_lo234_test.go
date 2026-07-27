// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Wave-22 LO-2 / LO-3 / LO-4 regression guards (§11.4.115 RED→GREEN each,
// §11.4.6 evidence-backed). Each test reproduces the real defect on the
// pre-fix code (RED) and passes only on the fixed code (GREEN); reverting the
// corresponding fix flips the test back to FAIL (anti-tautology).

// ---------------------------------------------------------------------------
// LO-2 — SimpleAgentPool.Acquire build path skipped the capability check.
//
// The build path handed out a freshly-built agent WITHOUT
// meetsAgentRequirements(a, req), while the available path
// (takeAvailableLocked) filtered. So a pool whose builder yields a 128k-token
// agent returned it for a MinTokens=500000 request — a capability-contract
// violation.
// ---------------------------------------------------------------------------

// TestLO2_Acquire_BuildPath_HonoursCapabilityContract builds a 128k-token
// agent and asks for MinTokens=500000. Pre-fix: the build path returns the
// 128k agent + nil err (RED). Fixed: the built agent is parked and Acquire
// returns ErrNoSuitableAgent (GREEN).
func TestLO2_Acquire_BuildPath_HonoursCapabilityContract(t *testing.T) {
	builder := func(_ context.Context) (Agent, error) {
		a := newMockAgent("small-128k", "opencode")
		a.caps.MaxTokens = 128000 // below the 500000 the caller requires
		return a, nil
	}
	pool := NewSimpleAgentPool("opencode", 1, builder)

	got, err := pool.Acquire(context.Background(), AgentRequirements{MinTokens: 500000})
	if err == nil {
		maxTok := -1
		if got != nil {
			maxTok = got.Capabilities().MaxTokens
		}
		t.Fatalf("LO-2: Acquire built + returned a %d-token agent for MinTokens=500000 with nil error "+
			"— the build path bypassed meetsAgentRequirements (capability-contract violation)", maxTok)
	}
	if !errors.Is(err, ErrNoSuitableAgent) {
		t.Errorf("LO-2: Acquire err = %v, want errors.Is(ErrNoSuitableAgent)", err)
	}
	if got != nil {
		t.Errorf("LO-2: Acquire returned non-nil agent (%d tokens) alongside error", got.Capabilities().MaxTokens)
	}

	// Positive control: a request the built agent CAN satisfy still succeeds,
	// and the parked agent is reused (no second build) — proves the fix does
	// not break the happy path or leak the capacity slot.
	ok, err := pool.Acquire(context.Background(), AgentRequirements{MinTokens: 100000})
	if err != nil {
		t.Fatalf("LO-2: compatible Acquire(MinTokens=100000) failed: %v", err)
	}
	if ok == nil || ok.Capabilities().MaxTokens != 128000 {
		t.Fatalf("LO-2: compatible Acquire returned %v, want the parked 128k agent reused", ok)
	}
}

// TestLO2_SelectorMatcher_HonoursMinTokens pins the sibling half of LO-2:
// multi_pool.go's package-level meetsRequirements OMITTED MinTokens while the
// pool-local meetsAgentRequirements included it. Two providers each hold one
// available agent; only the 600k one meets MinTokens=500000. Sorted order puts
// the 128k ("a-small") provider FIRST, so the round-robin primary loop visits
// it first. Pre-fix (MinTokens ignored): its 128k agent "matches" and is
// selected (RED). Fixed: the matcher rejects the 128k agent, so the primary
// loop advances to the qualifying 600k provider (GREEN). Both providers have
// available agents, so the selector's requirement-agnostic last-resort
// fallback arm is never reached — this isolates the matcher itself.
func TestLO2_SelectorMatcher_HonoursMinTokens(t *testing.T) {
	smallPool := newMockPool("a-small", 0)
	small := newMockAgent("small-128k", "a-small")
	small.caps.MaxTokens = 128000
	if err := smallPool.Register(small); err != nil {
		t.Fatalf("Register small: %v", err)
	}
	largePool := newMockPool("b-large", 0)
	large := newMockAgent("large-600k", "b-large")
	large.caps.MaxTokens = 600000
	if err := largePool.Register(large); err != nil {
		t.Fatalf("Register large: %v", err)
	}
	pools := map[string]AgentPool{"a-small": smallPool, "b-large": largePool}

	sel := NewRoundRobinSelector()
	got := sel.Select(pools, AgentRequirements{MinTokens: 500000})
	if got == "a-small" {
		t.Fatalf("LO-2: RoundRobinSelector.Select = %q for MinTokens=500000 — the selector matcher accepted a 128k "+
			"agent (meetsRequirements ignored MinTokens; disagrees with the pool-local matcher)", got)
	}
	if got != "b-large" {
		t.Errorf("LO-2: Select for MinTokens=500000 = %q, want %q (only the 600k provider qualifies)", got, "b-large")
	}

	// Positive control: a request both agents satisfy still selects. A FRESH
	// selector (counter=0) deterministically visits the sorted-first provider.
	if got := NewRoundRobinSelector().Select(pools, AgentRequirements{MinTokens: 100000}); got != "a-small" {
		t.Errorf("LO-2: Select for MinTokens=100000 = %q, want %q (both qualify; sorted rotation from counter=0)", got, "a-small")
	}
}

// ---------------------------------------------------------------------------
// LO-3 — MultiProviderPool.Acquire held m.mu.RLock across the blocking
// sub-pool Acquire → Shutdown (m.mu.Lock) deadlock.
// ---------------------------------------------------------------------------

// lo3AlwaysSelect is a test-only AgentSelector that always returns the same
// provider, so a saturated cap-1 SimpleAgentPool's blocking Acquire is
// actually reached (the default RoundRobinSelector would skip a full pool with
// no spare capacity, and the deadlock would never manifest).
type lo3AlwaysSelect struct{ provider string }

func (s lo3AlwaysSelect) Select(_ map[string]AgentPool, _ AgentRequirements) string {
	return s.provider
}

// TestLO3_Acquire_DoesNotHoldLockDuringBlockingSubAcquire saturates a cap-1
// sub-pool, parks a second Acquire inside it, then calls Shutdown under a
// watchdog. Pre-fix: Acquire holds m.mu.RLock while the parked sub-Acquire
// waits, so Shutdown's m.mu.Lock() never returns → deadlock → watchdog fires
// (RED). Fixed: the lock is released before the blocking sub-Acquire, so
// Shutdown returns promptly and the parked Acquire unblocks with the
// pool-closed error (GREEN).
func TestLO3_Acquire_DoesNotHoldLockDuringBlockingSubAcquire(t *testing.T) {
	builder, _ := makeMockBuilder("opencode")
	sp := NewSimpleAgentPool("opencode", 1, builder)
	mp := &MultiProviderPool{
		pools:    map[string]AgentPool{"opencode": sp},
		selector: lo3AlwaysSelect{provider: "opencode"},
	}

	// Check out the only agent so a second Acquire must block in the sub-pool.
	if _, err := mp.Acquire(context.Background(), AgentRequirements{}); err != nil {
		t.Fatalf("first Acquire: %v", err)
	}

	// Park a second Acquire — it blocks on the sub-pool's cond.Wait().
	acqDone := make(chan error, 1)
	go func() {
		_, err := mp.Acquire(context.Background(), AgentRequirements{})
		acqDone <- err
	}()

	// Let the second Acquire reach the blocking wait (pre-fix: it now holds
	// m.mu.RLock) BEFORE Shutdown attempts m.mu.Lock().
	time.Sleep(150 * time.Millisecond)
	select {
	case err := <-acqDone:
		t.Fatalf("second Acquire returned without blocking (err=%v) — cannot exercise the deadlock", err)
	default:
	}

	shutDone := make(chan error, 1)
	go func() {
		shutDone <- mp.Shutdown(context.Background())
	}()

	select {
	case err := <-shutDone:
		if err != nil {
			t.Fatalf("LO-3: Shutdown returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("LO-3: Shutdown did not return within 2s — MultiProviderPool.Acquire held m.mu across the " +
			"blocking sub-pool Acquire, deadlocking Shutdown's m.mu.Lock()")
	}

	select {
	case err := <-acqDone:
		if !errors.Is(err, ErrSimpleAgentPoolClosed) {
			t.Errorf("LO-3: parked Acquire returned err = %v, want ErrSimpleAgentPoolClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("LO-3: parked Acquire did not unblock after Shutdown within 2s")
	}
}

// ---------------------------------------------------------------------------
// LO-4 — RoundRobinSelector rebuilt r.providers from map-iteration order every
// call, so indexing it by r.counter rotated a re-shuffled slice →
// non-deterministic selection among equally-qualifying providers.
// ---------------------------------------------------------------------------

// TestLO4_RoundRobinSelector_DeterministicRotation registers three
// equally-qualifying providers and asserts a fixed sorted rotation over many
// calls. Pre-fix: r.providers is re-shuffled per call, so the sequence is
// random and diverges from the sorted rotation almost immediately (RED).
// Fixed: r.providers is sorted, so selection is a deterministic
// alpha→bravo→charlie rotation (GREEN).
func TestLO4_RoundRobinSelector_DeterministicRotation(t *testing.T) {
	names := []string{"alpha", "bravo", "charlie"}
	pools := make(map[string]AgentPool, len(names))
	for _, name := range names {
		p := newMockPool(name, 0)
		if err := p.Register(newMockAgent(name+"-agent", name)); err != nil {
			t.Fatalf("Register(%s): %v", name, err)
		}
		pools[name] = p
	}

	sel := NewRoundRobinSelector()

	// Deterministic expectation: providers sorted, rotated by the counter.
	want := []string{"alpha", "bravo", "charlie"}
	const cycles = 30 // 90 calls: a re-shuffled (buggy) sequence matching the
	// sorted rotation for all 90 has probability ~(1/3)^90 — effectively zero.
	got := make([]string, 0, cycles*len(want))
	for i := 0; i < cycles*len(want); i++ {
		got = append(got, sel.Select(pools, AgentRequirements{}))
	}
	for i, g := range got {
		exp := want[i%len(want)]
		if g != exp {
			t.Fatalf("LO-4: Select call %d = %q, want %q — round-robin over equally-qualifying providers is not a "+
				"fixed deterministic rotation (map-iteration order leaked into selection)\nfull sequence: %v", i, g, exp, got)
		}
	}
}
