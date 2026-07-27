// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package agent

import (
	"context"
	"testing"
)

// TestMultiProviderPool_Release_DoesNotCrossContaminatePools is the
// §11.4.115 RED-baseline regression guard for the cross-pool
// contamination defect in MultiProviderPool.Release.
//
// Root cause: Release iterated EVERY provider pool and called
// pool.Release(agent) on each. Because SimpleAgentPool.Release accepts
// untracked agents silently (documented for the pre-registered flow), an
// agent that belongs to ONE provider ends up in the available set of
// EVERY provider. A subsequent Acquire for provider B (round-robin or
// preference) can then hand out provider A's agent, and every pool's
// available/capacity accounting is corrupted.
//
// The guard acquires an opencode-owned agent through the multi-pool,
// releases it, and asserts it returned only to the opencode pool — not
// into the gemini pool.
func TestMultiProviderPool_Release_DoesNotCrossContaminatePools(t *testing.T) {
	mp, err := NewMultiProviderPool(map[string]*PoolConfig{
		"opencode": {Size: 1},
		"gemini":   {Size: 1, APIKey: "fake"},
	})
	if err != nil {
		t.Fatalf("NewMultiProviderPool: %v", err)
	}

	opencodeSP, ok := mp.pools["opencode"].(*SimpleAgentPool)
	if !ok {
		t.Fatalf("opencode pool type = %T, want *SimpleAgentPool", mp.pools["opencode"])
	}
	geminiSP, ok := mp.pools["gemini"].(*SimpleAgentPool)
	if !ok {
		t.Fatalf("gemini pool type = %T, want *SimpleAgentPool", mp.pools["gemini"])
	}

	// Seed only the opencode pool with a mock agent. NOTE: the round-robin
	// selector still CONSIDERS the gemini pool because it reports spare BUILD
	// capacity (Size=1, InUse=0) even with nothing Available — so an unpinned
	// Acquire non-deterministically tried to build a REAL gemini agent and
	// failed ("binary not found on PATH") on hosts without the Gemini CLI
	// (observed flaky 16/20). Pin PreferredAgent so the selector's preference
	// pass deterministically matches the only Available agent (opencode) and
	// never touches the gemini builder — the cross-pool contamination assertion
	// below is what this test actually validates (§11.4.98 re-runnable /
	// §11.4.50 deterministic).
	want := newMockAgent("opencode-owned", "opencode")
	if err := opencodeSP.Register(want); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := mp.Acquire(context.Background(), AgentRequirements{PreferredAgent: "opencode"})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if got.ID() != want.ID() {
		t.Fatalf("Acquire returned agent ID %q, want %q", got.ID(), want.ID())
	}

	mp.Release(got)

	if n := len(opencodeSP.Available()); n != 1 {
		t.Errorf("opencode pool Available after release = %d, want 1 (agent must return to its own pool)", n)
	}
	if n := len(geminiSP.Available()); n != 0 {
		t.Fatalf("gemini pool Available after releasing an opencode-owned agent = %d, want 0 — "+
			"MultiProviderPool.Release cross-contaminated pools: a gemini Acquire could now hand out an opencode agent", n)
	}
}
