// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// MultiProviderPool manages agents from multiple CLI providers (OpenCode, Claude, Gemini, Junie, Qwen)
type MultiProviderPool struct {
	pools    map[string]AgentPool
	selector AgentSelector
	mu       sync.RWMutex
}

// AgentSelector chooses the best provider for a given request
type AgentSelector interface {
	Select(pools map[string]AgentPool, req AgentRequirements) string
}

// NewMultiProviderPool creates a multi-provider pool.
//
// Round-60 §11.4 architecture upgrade: every provider-specific factory
// (NewOpenCodePool / NewClaudeCodePool / NewGeminiPool / NewJuniePool /
// NewQwenCodePool) now returns a real *SimpleAgentPool wired to a
// per-provider ClientBuilder when the supplied PoolConfig is non-nil.
// The pool itself is real and fully exercises capacity, available/in-use
// bookkeeping, blocking Acquire semantics, and Shutdown — only the
// per-call client materialisation surfaces a provider-specific
// "client SDK not wired" sentinel (Err{OpenCode,ClaudeCode,Gemini,
// Junie,QwenCode}ClientNotWired) until each provider's transport is
// wired in round-61+.
//
// nil PoolConfig still surfaces the round-28 ErrProviderPoolNotImplemented
// sentinel because that case is genuinely "the operator did not even
// configure this provider" — distinct from "configured but not wired".
//
// Callers MAY still construct a MultiProviderPool directly with externally
// provided AgentPool implementations (e.g., installed via dependency
// injection in tests) — see the test helpers in pkg/agent/*_test.go.
func NewMultiProviderPool(configs map[string]*PoolConfig) (*MultiProviderPool, error) {
	pools := make(map[string]AgentPool)

	for provider, cfg := range configs {
		var (
			p   AgentPool
			err error
		)
		switch provider {
		case "opencode":
			p, err = NewOpenCodePool(cfg)
		case "claude-code":
			p, err = NewClaudeCodePool(cfg)
		case "gemini":
			p, err = NewGeminiPool(cfg)
		case "junie":
			p, err = NewJuniePool(cfg)
		case "qwen-code":
			p, err = NewQwenCodePool(cfg)
		default:
			return nil, fmt.Errorf("unknown provider: %s", provider)
		}
		if err != nil {
			return nil, fmt.Errorf("provider %q: %w", provider, err)
		}
		pools[provider] = p
	}

	return &MultiProviderPool{
		pools:    pools,
		selector: NewRoundRobinSelector(),
	}, nil
}

// Acquire gets an agent from the best available pool
func (m *MultiProviderPool) Acquire(ctx context.Context, req AgentRequirements) (Agent, error) {
	// LO-3: snapshot the selected pool UNDER the lock, then RELEASE the lock
	// BEFORE calling the sub-pool's blocking Acquire (which may park on
	// cond.Wait until a Release or Shutdown). Holding m.mu across that
	// unbounded blocking call deadlocked Shutdown: Shutdown needs
	// m.mu.Lock(), which a reader parked inside the sub-pool Acquire never
	// releases, so Shutdown never reaches the per-pool broadcast that would
	// wake the parked Acquire. Never hold a lock across an unbounded blocking
	// call. Selection reads m.pools, so it stays under the read lock.
	m.mu.RLock()
	selected := m.selector.Select(m.pools, req)
	var pool AgentPool
	if selected != "" {
		pool = m.pools[selected]
	}
	m.mu.RUnlock()

	if pool == nil {
		return nil, ErrNoSuitableAgent
	}

	return pool.Acquire(ctx, req)
}

// Release returns an agent to its pool
func (m *MultiProviderPool) Release(agent Agent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find which pool this agent belongs to and release it
	for _, pool := range m.pools {
		pool.Release(agent)
	}
}

// Available returns all available agents across all pools
func (m *MultiProviderPool) Available() []Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	available := make([]Agent, 0)
	for _, pool := range m.pools {
		available = append(available, pool.Available()...)
	}
	return available
}

// HealthCheck checks health of all pools
func (m *MultiProviderPool) HealthCheck(ctx context.Context) []HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]HealthStatus, 0)
	for _, pool := range m.pools {
		statuses = append(statuses, pool.HealthCheck(ctx)...)
	}
	return statuses
}

// Shutdown gracefully shuts down all pools
func (m *MultiProviderPool) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, pool := range m.pools {
		if err := pool.Shutdown(ctx); err != nil {
			lastErr = fmt.Errorf("shutdown %s: %w", name, err)
		}
	}
	return lastErr
}

// capacityReporter is an OPTIONAL interface a pool MAY implement to
// report its build capacity. A pool that satisfies it tells the selector
// it can materialise a NEW agent on demand (capacity not yet exhausted)
// even when its Available() set is currently empty — exactly the shape a
// freshly-built lazy SimpleAgentPool (capacity > 0, zero pre-registered
// agents) presents. SimpleAgentPool already satisfies it via Size()/InUse().
//
// Defining it as a narrow optional interface keeps the AgentPool contract
// untouched: pools that do NOT implement it keep the prior "select only
// when Available() is non-empty" behaviour.
type capacityReporter interface {
	Size() int
	InUse() int
}

// hasSpareBuildCapacity reports whether pool can build a new agent on
// demand. A pool that does not implement capacityReporter cannot be
// queried for capacity, so it returns false (prior behaviour preserved).
func hasSpareBuildCapacity(pool AgentPool) bool {
	cr, ok := pool.(capacityReporter)
	if !ok {
		return false
	}
	return cr.InUse() < cr.Size()
}

// RoundRobinSelector implements round-robin selection across providers
type RoundRobinSelector struct {
	mu        sync.Mutex
	counter   int
	providers []string
}

// NewRoundRobinSelector creates a round-robin selector
func NewRoundRobinSelector() *RoundRobinSelector {
	return &RoundRobinSelector{
		providers: make([]string, 0),
	}
}

// Select chooses the next provider in round-robin fashion
func (r *RoundRobinSelector) Select(pools map[string]AgentPool, req AgentRequirements) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Update providers list.
	// LO-4: map iteration order is randomised on every range, so rebuilding
	// r.providers from it and then indexing it by r.counter rotated a
	// re-shuffled slice — selection among equally-qualifying providers was
	// non-deterministic run-to-run (the ROOT of the 47f8d12 flake, which
	// commit 47f8d12 only masked at one call site via a PreferredAgent pin).
	// Sort to a stable order so r.counter rotates deterministically.
	r.providers = make([]string, 0, len(pools))
	for name := range pools {
		r.providers = append(r.providers, name)
	}
	sort.Strings(r.providers)

	if len(r.providers) == 0 {
		return ""
	}

	// Priority pass: if the caller named a PreferredAgent and a provider
	// currently holds an available agent with that name meeting the
	// requirements, select that provider directly. PreferredAgent is an
	// agent-name filter applied inside each pool's Acquire — surfacing it
	// here keeps the selector from round-robining PAST the very provider
	// that already holds the preferred agent (which, with the build-capacity
	// arm below, could otherwise pick a different provider first).
	if req.PreferredAgent != "" {
		for _, provider := range r.providers {
			pool, ok := pools[provider]
			if !ok {
				continue
			}
			for _, agent := range pool.Available() {
				if agent.Name() == req.PreferredAgent && meetsRequirements(agent, req) {
					return provider
				}
			}
		}
	}

	// Try to find a provider that can serve the request in round-robin
	// order. A provider qualifies if EITHER it has an available agent that
	// meets requirements, OR it has spare build capacity to materialise a
	// new agent on demand (the lazy-build path). Without the capacity arm a
	// freshly-built lazy pool — capacity > 0, zero pre-registered agents,
	// empty Available() — is never selected and its on-demand build path is
	// dead on first use.
	for i := 0; i < len(r.providers); i++ {
		idx := (r.counter + i) % len(r.providers)
		provider := r.providers[idx]

		pool, ok := pools[provider]
		if !ok {
			continue
		}

		// Check if this provider has available agents that meet requirements.
		for _, agent := range pool.Available() {
			if meetsRequirements(agent, req) {
				r.counter = (r.counter + 1) % len(r.providers)
				return provider
			}
		}

		// Otherwise, a pool with spare build capacity can build one on
		// demand in its own Acquire (which applies requirement filtering).
		if hasSpareBuildCapacity(pool) {
			r.counter = (r.counter + 1) % len(r.providers)
			return provider
		}
	}

	// Fallback: any provider with an available agent OR spare build capacity.
	for _, provider := range r.providers {
		if pool, ok := pools[provider]; ok {
			if len(pool.Available()) > 0 || hasSpareBuildCapacity(pool) {
				return provider
			}
		}
	}

	return ""
}

func meetsRequirements(agent Agent, req AgentRequirements) bool {
	caps := agent.Capabilities()

	if req.NeedsVision && !caps.Vision {
		return false
	}

	if req.NeedsStreaming && !caps.Streaming {
		return false
	}

	// LO-2: align this selector-side matcher with the pool-local
	// meetsAgentRequirements (simple_pool.go), which already filters on the
	// token budget. Without this, the selector would pick a provider whose
	// only agent the pool then refuses (its build/available path enforces
	// MinTokens), producing a spurious no-agent outcome — the two matchers
	// MUST agree.
	if req.MinTokens > 0 && caps.MaxTokens < req.MinTokens {
		return false
	}

	return true
}

// PreferenceSelector selects based on user preferences
type PreferenceSelector struct {
	preferredOrder []string
}

// NewPreferenceSelector creates a selector with provider preferences
func NewPreferenceSelector(preferences []string) *PreferenceSelector {
	return &PreferenceSelector{
		preferredOrder: preferences,
	}
}

// Select chooses the first available preferred provider
func (p *PreferenceSelector) Select(pools map[string]AgentPool, req AgentRequirements) string {
	for _, preferred := range p.preferredOrder {
		if pool, ok := pools[preferred]; ok {
			available := pool.Available()
			for _, agent := range available {
				if meetsRequirements(agent, req) {
					return preferred
				}
			}
		}
	}

	// Fallback to any available provider
	for name := range pools {
		return name
	}

	return ""
}

// PoolConfig holds configuration for creating a provider pool
type PoolConfig struct {
	Size       int
	Timeout    time.Duration
	MaxRetries int
	BinaryPath string
	Provider   string
	Model      string
	APIKey     string
}

// ErrProviderPoolNotImplemented is the round-28 §11.4 sentinel — now
// retained as the "configuration genuinely absent" indicator. Round-60
// §11.4 upgraded the five provider factories to return real concrete
// pools (SimpleAgentPool composed with a per-provider ClientBuilder)
// when called with a NON-NIL PoolConfig. Calling a factory with a
// nil PoolConfig still returns this sentinel because that genuinely
// means "operator never even configured this provider" — distinct
// from "configured but the transport SDK has not been wired yet",
// which is the per-provider Err{Provider}ClientNotWired surface in
// builders.go.
//
// Round-28 §11.4 audit (the project close-out, 2026-05-17, original
// forensic anchor): the original factory bodies returned a MockPool
// seeded with `nil` agent slots (`agents[i] = nil // Placeholder`).
// MultiProviderPool.Acquire() would happily return one of those nil
// agents to a caller, which then panicked on the first method call.
// The test suite passed because no test exercised the round-trip —
// a CRITICAL CONTRACT-bluff at the multi-provider-pool layer. Round
// 28 closed that by making every factory return this sentinel. Round
// 60 replaces the sentinel return with a real pool + per-provider
// builder-sentinel pattern so callers see precise wiring gaps
// instead of an undifferentiated "not implemented".
//
// Constitutional anchors: CONST-035 (anti-bluff), CONST-050(A)
// (no-fakes-beyond-unit-tests), Article XI §11.9 (forensic anchor).
var ErrProviderPoolNotImplemented = fmt.Errorf("llmorchestrator: provider pool factory invoked with nil PoolConfig — no provider configuration present (round-60 §11.4 upgraded factories to return real SimpleAgentPool instances for non-nil configs; nil-config still surfaces this round-28 sentinel)")

// NewOpenCodePool returns a real *SimpleAgentPool wired to
// OpenCodeClientBuilder when cfg is non-nil. The pool itself is fully
// functional; its first Acquire fails loudly with ErrOpenCodeClientNotWired
// until round-61+ wires the OpenCode CLI binary integration.
//
// A nil cfg surfaces ErrProviderPoolNotImplemented (round-28 sentinel,
// retained for the "operator never configured this provider" case).
func NewOpenCodePool(cfg *PoolConfig) (AgentPool, error) {
	if cfg == nil {
		return nil, fmt.Errorf("opencode: %w", ErrProviderPoolNotImplemented)
	}
	return NewSimpleAgentPool("opencode", poolSize(cfg), OpenCodeClientBuilder(cfg)), nil
}

// NewClaudeCodePool returns a real *SimpleAgentPool wired to
// ClaudeCodeClientBuilder when cfg is non-nil. The pool's first
// Acquire fails loudly with ErrClaudeCodeClientNotWired until
// round-61+ wires the Claude Code CLI binary integration.
func NewClaudeCodePool(cfg *PoolConfig) (AgentPool, error) {
	if cfg == nil {
		return nil, fmt.Errorf("claude-code: %w", ErrProviderPoolNotImplemented)
	}
	return NewSimpleAgentPool("claude-code", poolSize(cfg), ClaudeCodeClientBuilder(cfg)), nil
}

// NewGeminiPool returns a real *SimpleAgentPool wired to
// GeminiClientBuilder when cfg is non-nil. The pool's first Acquire
// fails loudly with ErrGeminiClientNotWired until round-61+ wires
// the Gemini HTTP transport.
func NewGeminiPool(cfg *PoolConfig) (AgentPool, error) {
	if cfg == nil {
		return nil, fmt.Errorf("gemini: %w", ErrProviderPoolNotImplemented)
	}
	return NewSimpleAgentPool("gemini", poolSize(cfg), GeminiClientBuilder(cfg)), nil
}

// NewJuniePool returns a real *SimpleAgentPool wired to
// JunieClientBuilder when cfg is non-nil. The pool's first Acquire
// fails loudly with ErrJunieClientNotWired until round-61+ wires
// the Junie CLI binary integration.
func NewJuniePool(cfg *PoolConfig) (AgentPool, error) {
	if cfg == nil {
		return nil, fmt.Errorf("junie: %w", ErrProviderPoolNotImplemented)
	}
	return NewSimpleAgentPool("junie", poolSize(cfg), JunieClientBuilder(cfg)), nil
}

// NewQwenCodePool returns a real *SimpleAgentPool wired to
// QwenCodeClientBuilder when cfg is non-nil. Round-76 §11.4 wired
// the real os/exec bridge to `qwen <prompt>` (FINAL builder of the
// LLMOrchestrator round-60 sentinel arc — 5/5 builders complete:
// OpenCode + ClaudeCode + Gemini + Junie + QwenCode); the pool's
// first Acquire builds a real *QwenCodeAgent unless the binary is
// missing from $PATH (ErrQwenCodeBinaryNotFound surfaces in that
// case, errors.Is-checkable).
func NewQwenCodePool(cfg *PoolConfig) (AgentPool, error) {
	if cfg == nil {
		return nil, fmt.Errorf("qwen-code: %w", ErrProviderPoolNotImplemented)
	}
	return NewSimpleAgentPool("qwen-code", poolSize(cfg), QwenCodeClientBuilder(cfg)), nil
}

// poolSize returns a safe SimpleAgentPool capacity from cfg.Size,
// defaulting to 1 when cfg.Size is zero or negative — both meaning
// "operator left it unset" rather than "explicitly forbid any
// concurrent agents". SimpleAgentPool requires capacity >= 1.
func poolSize(cfg *PoolConfig) int {
	if cfg == nil || cfg.Size < 1 {
		return 1
	}
	return cfg.Size
}

var (
	ErrNoSuitableAgent   = fmt.Errorf("no suitable agent available")
	ErrNoAgentsAvailable = fmt.Errorf("no agents available in pool")
)
