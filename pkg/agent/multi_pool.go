// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
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

// NewMultiProviderPool creates a multi-provider pool
func NewMultiProviderPool(configs map[string]*PoolConfig) (*MultiProviderPool, error) {
	pools := make(map[string]AgentPool)

	for provider, cfg := range configs {
		switch provider {
		case "opencode":
			pools[provider] = NewOpenCodePool(cfg)
		case "claude-code":
			pools[provider] = NewClaudeCodePool(cfg)
		case "gemini":
			pools[provider] = NewGeminiPool(cfg)
		case "junie":
			pools[provider] = NewJuniePool(cfg)
		case "qwen-code":
			pools[provider] = NewQwenCodePool(cfg)
		default:
			return nil, fmt.Errorf("unknown provider: %s", provider)
		}
	}

	return &MultiProviderPool{
		pools:    pools,
		selector: NewRoundRobinSelector(),
	}, nil
}

// Acquire gets an agent from the best available pool
func (m *MultiProviderPool) Acquire(ctx context.Context, req AgentRequirements) (Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find best pool based on requirements
	selected := m.selector.Select(m.pools, req)
	if selected == "" {
		return nil, ErrNoSuitableAgent
	}

	return m.pools[selected].Acquire(ctx, req)
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

	// Update providers list
	r.providers = make([]string, 0, len(pools))
	for name := range pools {
		r.providers = append(r.providers, name)
	}

	if len(r.providers) == 0 {
		return ""
	}

	// Try to find a provider that can meet requirements
	for i := 0; i < len(r.providers); i++ {
		idx := (r.counter + i) % len(r.providers)
		provider := r.providers[idx]

		// Check if this provider has available agents that meet requirements
		if pool, ok := pools[provider]; ok {
			available := pool.Available()
			for _, agent := range available {
				if meetsRequirements(agent, req) {
					r.counter = (r.counter + 1) % len(r.providers)
					return provider
				}
			}
		}
	}

	// Fallback: just return the first provider with any available agent
	for _, provider := range r.providers {
		if pool, ok := pools[provider]; ok {
			if len(pool.Available()) > 0 {
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

// Mock pool implementations for different providers
func NewOpenCodePool(cfg *PoolConfig) AgentPool {
	// Implementation would create OpenCode-specific pool
	return NewMockPool("opencode", cfg.Size)
}

func NewClaudeCodePool(cfg *PoolConfig) AgentPool {
	return NewMockPool("claude-code", cfg.Size)
}

func NewGeminiPool(cfg *PoolConfig) AgentPool {
	return NewMockPool("gemini", cfg.Size)
}

func NewJuniePool(cfg *PoolConfig) AgentPool {
	return NewMockPool("junie", cfg.Size)
}

func NewQwenCodePool(cfg *PoolConfig) AgentPool {
	return NewMockPool("qwen-code", cfg.Size)
}

// MockPool is a placeholder implementation
type MockPool struct {
	name      string
	agents    []Agent
	available []Agent
	mu        sync.Mutex
}

func NewMockPool(name string, size int) *MockPool {
	agents := make([]Agent, size)
	available := make([]Agent, size)

	for i := 0; i < size; i++ {
		// Would create actual agent instances
		agents[i] = nil // Placeholder
		available[i] = agents[i]
	}

	return &MockPool{
		name:      name,
		agents:    agents,
		available: available,
	}
}

func (p *MockPool) Register(agent Agent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.agents = append(p.agents, agent)
	p.available = append(p.available, agent)
	return nil
}

func (p *MockPool) Acquire(ctx context.Context, req AgentRequirements) (Agent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.available) == 0 {
		return nil, ErrNoAgentsAvailable
	}

	agent := p.available[0]
	p.available = p.available[1:]
	return agent, nil
}

func (p *MockPool) Release(agent Agent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.available = append(p.available, agent)
}

func (p *MockPool) Available() []Agent {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]Agent, len(p.available))
	copy(result, p.available)
	return result
}

func (p *MockPool) HealthCheck(ctx context.Context) []HealthStatus {
	return []HealthStatus{}
}

func (p *MockPool) Shutdown(ctx context.Context) error {
	return nil
}

var (
	ErrNoSuitableAgent   = fmt.Errorf("no suitable agent available")
	ErrNoAgentsAvailable = fmt.Errorf("no agents available in pool")
)
