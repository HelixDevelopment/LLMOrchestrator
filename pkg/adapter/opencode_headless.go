// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package adapter provides CLI agent adapters for LLM orchestration.
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// OpenCodeConfig holds configuration for OpenCode adapter
type OpenCodeConfig struct {
	BinaryPath     string            `json:"binary_path"`
	Provider       string            `json:"provider"`
	Model          string            `json:"model"`
	WorkingDir     string            `json:"working_dir"`
	Headless       bool              `json:"headless"`
	NonInteractive bool              `json:"non_interactive"`
	Timeout        time.Duration     `json:"timeout"`
	MaxTokens      int               `json:"max_tokens"`
	Temperature    float64           `json:"temperature"`
	SystemPrompt   string            `json:"system_prompt"`
	ExtraFlags     []string          `json:"extra_flags"`
	EnvVars        map[string]string `json:"env_vars"`
}

// DefaultOpenCodeConfig returns default configuration
func DefaultOpenCodeConfig() *OpenCodeConfig {
	return &OpenCodeConfig{
		BinaryPath:     "opencode",
		Headless:       true,
		NonInteractive: true,
		Timeout:        120 * time.Second,
		MaxTokens:      4096,
		Temperature:    0.7,
		EnvVars:        make(map[string]string),
		ExtraFlags:     make([]string, 0),
	}
}

// OpenCodeAdapter implements the Agent interface for OpenCode CLI
type OpenCodeAdapter struct {
	config  *OpenCodeConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.Reader
	stderr  io.Reader
	parser  *OpenCodeParser
	mu      sync.Mutex
	running bool
}

// NewOpenCodeAdapter creates a new OpenCode adapter
func NewOpenCodeAdapter(cfg *OpenCodeConfig) *OpenCodeAdapter {
	if cfg == nil {
		cfg = DefaultOpenCodeConfig()
	}
	return &OpenCodeAdapter{
		config: cfg,
		parser: NewOpenCodeParser(),
	}
}

// Start launches the OpenCode process
func (a *OpenCodeAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("already running")
	}

	args := a.buildArgs()

	a.cmd = exec.CommandContext(ctx, a.config.BinaryPath, args...)
	a.cmd.Dir = a.config.WorkingDir
	a.cmd.Env = a.buildEnv()

	stdin, err := a.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	a.stdin = stdin

	stdout, err := a.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	a.stdout = stdout

	stderr, err := a.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	a.stderr = stderr

	if err := a.cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	a.running = true
	return nil
}

func (a *OpenCodeAdapter) buildArgs() []string {
	args := []string{}

	if a.config.Headless {
		args = append(args, "--headless")
	}
	if a.config.NonInteractive {
		args = append(args, "--non-interactive")
	}
	if a.config.Provider != "" {
		args = append(args, "--provider", a.config.Provider)
	}
	if a.config.Model != "" {
		args = append(args, "--model", a.config.Model)
	}

	args = append(args, a.config.ExtraFlags...)

	return args
}

func (a *OpenCodeAdapter) buildEnv() []string {
	env := os.Environ()

	for k, v := range a.config.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

// Stop terminates the OpenCode process
func (a *OpenCodeAdapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	if a.cmd != nil && a.cmd.Process != nil {
		if err := a.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("kill process: %w", err)
		}
	}

	a.running = false
	return nil
}

// IsRunning returns whether the adapter is running
func (a *OpenCodeAdapter) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// Send sends a prompt and returns the response
func (a *OpenCodeAdapter) Send(ctx context.Context, prompt string) (*Response, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil, fmt.Errorf("adapter not running")
	}

	req := map[string]interface{}{
		"prompt": prompt,
		"config": map[string]interface{}{
			"max_tokens":  a.config.MaxTokens,
			"temperature": a.config.Temperature,
		},
	}

	if a.config.SystemPrompt != "" {
		req["system_prompt"] = a.config.SystemPrompt
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	data = append(data, '\n')

	_, err = a.stdin.Write(data)
	if err != nil {
		return nil, fmt.Errorf("write to stdin: %w", err)
	}

	responseChan := make(chan *Response, 1)
	errChan := make(chan error, 1)

	go func() {
		response, err := a.readResponse(ctx)
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- response
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errChan:
		return nil, err
	case response := <-responseChan:
		return response, nil
	case <-time.After(a.config.Timeout):
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

func (a *OpenCodeAdapter) readResponse(ctx context.Context) (*Response, error) {
	buf := make([]byte, 65536)
	n, err := a.stdout.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read stdout: %w", err)
	}

	raw := string(buf[:n])
	return a.parser.Parse(raw)
}

// Response represents an LLM response
type Response struct {
	Content      string `json:"content"`
	Model        string `json:"model"`
	TokensIn     int    `json:"tokens_in"`
	TokensOut    int    `json:"tokens_out"`
	FinishReason string `json:"finish_reason"`
}

// OpenCodeParser parses OpenCode output
type OpenCodeParser struct{}

// NewOpenCodeParser creates a new parser
func NewOpenCodeParser() *OpenCodeParser {
	return &OpenCodeParser{}
}

// Parse parses raw output into a Response
func (p *OpenCodeParser) Parse(raw string) (*Response, error) {
	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &resp, nil
}

// ExtractJSON extracts JSON from mixed content
func (p *OpenCodeParser) ExtractJSON(content string) (map[string]interface{}, error) {
	start := -1
	for i, c := range content {
		if c == '{' {
			start = i
			break
		}
	}
	if start == -1 {
		return nil, fmt.Errorf("no JSON found")
	}

	end := -1
	depth := 0
	for i := start; i < len(content); i++ {
		if content[i] == '{' {
			depth++
		} else if content[i] == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if end == -1 {
		return nil, fmt.Errorf("unbalanced JSON")
	}

	jsonStr := content[start:end]
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return result, nil
}
