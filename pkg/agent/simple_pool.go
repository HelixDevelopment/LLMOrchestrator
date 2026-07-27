// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ClientBuilder lazily constructs a single Agent instance on demand.
//
// Round-60 §11.4 architecture: every concrete provider-specific pool
// is composed of (a) a generic SimpleAgentPool that manages capacity,
// available/in-use bookkeeping, and blocking Acquire semantics, plus
// (b) an injected ClientBuilder closure that knows how to talk to the
// real provider SDK / CLI / HTTP endpoint. Until each provider's
// ClientBuilder is wired (round-61+), the per-provider builders in
// builders.go return sentinel errors so SimpleAgentPool.Acquire fails
// loudly on first use instead of pretending it succeeded.
type ClientBuilder func(ctx context.Context) (Agent, error)

// ErrSimpleAgentPoolClosed is returned by Acquire/Release after Shutdown.
var ErrSimpleAgentPoolClosed = errors.New("agent.SimpleAgentPool: pool is closed")

// SimpleAgentPool is a concrete AgentPool that lazily materialises
// Agent instances via an injected ClientBuilder, bounded by a fixed
// capacity, with blocking Acquire semantics when the pool is exhausted.
//
// Round-60 §11.4 forensic anchor: this replaces the
// "every factory returns sentinel" (round-28) stop-gap with a real
// concrete pool implementation. The pool itself is real; only the
// per-provider client-builders remain sentinel-returning until round-61+
// wires the actual SDK integrations. That preserves the anti-bluff
// guarantee — calling code gets either a real Agent or a loud error,
// never a nil-agent panic-trap — while letting each provider's SDK
// wiring land in its own round without re-touching pool plumbing.
//
// Concurrency: SimpleAgentPool is safe for concurrent use by many
// goroutines; Acquire blocks when capacity is reached and no agent
// is available, waking on Release or ctx cancellation.
//
// Constitutional anchors: CONST-035 (anti-bluff covenant),
// CONST-050(A) (no-fakes-beyond-unit-tests), Article XI §11.9.
type SimpleAgentPool struct {
	name     string
	builder  ClientBuilder
	capacity int

	mu        sync.Mutex
	cond      *sync.Cond
	available []Agent            // ready to hand out
	inUse     map[Agent]struct{} // currently held by callers
	allAgents []Agent            // every Agent the pool has ever created or accepted (for Shutdown)
	building  int                // in-flight builder calls that have reserved a capacity slot
	closed    bool
}

// NewSimpleAgentPool returns a SimpleAgentPool of the given capacity
// that uses builder to materialise Agent instances on demand.
//
// capacity MUST be >= 1; builder MUST be non-nil. Bad inputs panic
// because they are programmer errors that no caller should ship.
func NewSimpleAgentPool(name string, capacity int, builder ClientBuilder) *SimpleAgentPool {
	if capacity < 1 {
		panic(fmt.Sprintf("agent.NewSimpleAgentPool(%q): capacity %d < 1", name, capacity))
	}
	if builder == nil {
		panic(fmt.Sprintf("agent.NewSimpleAgentPool(%q): builder is nil", name))
	}
	p := &SimpleAgentPool{
		name:      name,
		builder:   builder,
		capacity:  capacity,
		available: make([]Agent, 0, capacity),
		inUse:     make(map[Agent]struct{}, capacity),
		allAgents: make([]Agent, 0, capacity),
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// Name returns the pool's provider name (e.g. "opencode", "gemini").
func (p *SimpleAgentPool) Name() string { return p.name }

// Size returns the maximum number of concurrent agents this pool will hold.
func (p *SimpleAgentPool) Size() int { return p.capacity }

// InUse returns the count of agents currently checked out by callers.
func (p *SimpleAgentPool) InUse() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.inUse)
}

// Register accepts a pre-constructed Agent into the pool's available
// set. Useful for tests and dependency-injection scenarios where the
// caller already owns the Agent lifecycle. Returns ErrSimpleAgentPoolClosed
// after Shutdown.
//
// Registering does NOT bypass the capacity ceiling — pools full of
// pre-registered agents simply will not call the builder at all.
func (p *SimpleAgentPool) Register(a Agent) error {
	if a == nil {
		return errors.New("agent.SimpleAgentPool.Register: nil agent")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrSimpleAgentPoolClosed
	}
	p.available = append(p.available, a)
	p.allAgents = append(p.allAgents, a)
	p.cond.Broadcast()
	return nil
}

// Acquire returns an Agent matching req. If an available agent exists
// it is handed out immediately. Otherwise, if total checked-out count
// is below capacity, the injected ClientBuilder is invoked to make a
// new Agent. If neither is possible, Acquire blocks until a Release
// frees a slot or ctx is cancelled.
//
// If the builder returns an error, that error is wrapped and returned
// to the caller — this is the loud-failure path that surfaces the
// per-provider "client SDK not wired" sentinel until round-61+ wiring.
func (p *SimpleAgentPool) Acquire(ctx context.Context, req AgentRequirements) (Agent, error) {
	// Goroutine to wake waiters on ctx cancellation, mirroring pool.go's pattern.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			p.cond.Broadcast()
		case <-done:
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		if p.closed {
			return nil, ErrSimpleAgentPoolClosed
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Fast path: hand out a ready agent that meets requirements.
		if a := p.takeAvailableLocked(req); a != nil {
			p.inUse[a] = struct{}{}
			return a, nil
		}

		// Build path: capacity available → invoke the injected builder.
		// p.building counts slots reserved by in-flight builder calls so
		// that concurrent Acquire callers cannot all pass this check and
		// each build past the ceiling (capacity over-provisioning race):
		// the mutex is dropped during the build, and without the
		// reservation the released-lock window let every waiter observe
		// the same "capacity available" state.
		if len(p.inUse)+len(p.available)+p.building < p.capacity {
			// Reserve the slot, then release the mutex while building —
			// the builder may do network I/O or fork a process.
			p.building++
			p.mu.Unlock()
			a, err := p.builder(ctx)
			p.mu.Lock()
			p.building--
			if err != nil {
				// The reserved slot is freed; wake a waiter that may now
				// be able to build.
				p.cond.Broadcast()
				return nil, fmt.Errorf("agent.SimpleAgentPool(%q): builder failed: %w", p.name, err)
			}
			if a == nil {
				p.cond.Broadcast()
				return nil, fmt.Errorf("agent.SimpleAgentPool(%q): builder returned nil agent without error", p.name)
			}
			// Re-check closed state — Shutdown may have raced us.
			if p.closed {
				p.cond.Broadcast()
				return nil, ErrSimpleAgentPoolClosed
			}
			p.allAgents = append(p.allAgents, a)
			// LO-2: the BUILD path must honour the SAME capability contract the
			// AVAILABLE path already enforces via takeAvailableLocked →
			// meetsAgentRequirements. Without this, a freshly-built agent that
			// does NOT meet req (e.g. Capabilities().MaxTokens < req.MinTokens)
			// was handed out silently — a capability-contract violation
			// (§11.4.108): the caller asked for MinTokens=500000 and received a
			// 128k agent + nil err.
			if !meetsAgentRequirements(a, req) {
				// Park the built agent as available so a later Acquire with
				// compatible requirements can reuse it — accounting stays exact
				// (p.building was already decremented above; the agent now
				// occupies its capacity slot as an available entry) — wake a
				// waiter, and return the typed no-suitable-agent error rather
				// than a requirement-violating agent.
				p.available = append(p.available, a)
				p.cond.Broadcast()
				return nil, ErrNoSuitableAgent
			}
			p.inUse[a] = struct{}{}
			return a, nil
		}

		// Wait path: capacity exhausted and no matching available agent.
		p.cond.Wait()
	}
}

// takeAvailableLocked removes and returns the first available Agent
// that meets req. Returns nil if none match. Caller must hold p.mu.
func (p *SimpleAgentPool) takeAvailableLocked(req AgentRequirements) Agent {
	// Preferred-name first pass.
	if req.PreferredAgent != "" {
		for i, a := range p.available {
			if a.Name() == req.PreferredAgent && meetsAgentRequirements(a, req) {
				p.available = append(p.available[:i], p.available[i+1:]...)
				return a
			}
		}
	}
	// Any-match second pass.
	for i, a := range p.available {
		if meetsAgentRequirements(a, req) {
			p.available = append(p.available[:i], p.available[i+1:]...)
			return a
		}
	}
	return nil
}

// meetsAgentRequirements is the SimpleAgentPool-local capability matcher.
// Kept separate from pool.go's matcher to avoid coupling the two
// implementations across files that may evolve independently.
func meetsAgentRequirements(a Agent, req AgentRequirements) bool {
	caps := a.Capabilities()
	if req.NeedsVision && !caps.Vision {
		return false
	}
	if req.NeedsStreaming && !caps.Streaming {
		return false
	}
	if req.MinTokens > 0 && caps.MaxTokens < req.MinTokens {
		return false
	}
	return true
}

// Release returns an Agent to the pool's available set.
//
// Only agents this pool currently has checked out (tracked in p.inUse)
// are returned to availability. An agent that is NOT in-use — a double
// Release, or an agent that belongs to a different pool (e.g. when a
// MultiProviderPool fans Release out to every underlying pool) — is
// ignored. Without this guard a duplicate would be appended to
// p.available, letting a capacity-1 pool hand the SAME agent instance
// to two concurrent callers and letting a foreign agent leak into a
// pool that never built it (capacity-accounting corruption).
func (p *SimpleAgentPool) Release(a Agent) {
	if a == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	if _, tracked := p.inUse[a]; !tracked {
		// Not currently checked out by this pool: double-release or
		// foreign agent. Dropping it keeps available/in-use accounting
		// exact and prevents duplicate hand-out.
		return
	}
	delete(p.inUse, a)
	p.available = append(p.available, a)
	p.cond.Broadcast()
}

// Available returns a snapshot of currently available agents.
func (p *SimpleAgentPool) Available() []Agent {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Agent, len(p.available))
	copy(out, p.available)
	return out
}

// HealthCheck invokes Health on every Agent the pool has materialised
// or accepted (available + in-use). Returns one HealthStatus per agent.
func (p *SimpleAgentPool) HealthCheck(ctx context.Context) []HealthStatus {
	p.mu.Lock()
	agents := make([]Agent, len(p.allAgents))
	copy(agents, p.allAgents)
	p.mu.Unlock()

	if len(agents) == 0 {
		return []HealthStatus{}
	}
	statuses := make([]HealthStatus, len(agents))
	var wg sync.WaitGroup
	for i, a := range agents {
		wg.Add(1)
		go func(idx int, ag Agent) {
			defer wg.Done()
			statuses[idx] = ag.Health(ctx)
		}(i, a)
	}
	wg.Wait()
	return statuses
}

// Shutdown marks the pool closed, wakes all waiters, and Stops every
// agent the pool has ever held. Idempotent.
func (p *SimpleAgentPool) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	agents := make([]Agent, len(p.allAgents))
	copy(agents, p.allAgents)
	p.available = nil
	p.inUse = make(map[Agent]struct{})
	p.allAgents = nil
	p.cond.Broadcast()
	p.mu.Unlock()

	var errs []error
	for _, a := range agents {
		if a == nil {
			continue
		}
		if a.IsRunning() {
			if err := a.Stop(ctx); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Compile-time assertion: SimpleAgentPool satisfies the AgentPool contract.
var _ AgentPool = (*SimpleAgentPool)(nil)
