// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultOpenCodeConfig(t *testing.T) {
	cfg := DefaultOpenCodeConfig()

	assert.Equal(t, "opencode", cfg.BinaryPath)
	assert.True(t, cfg.Headless)
	assert.True(t, cfg.NonInteractive)
	assert.Equal(t, 120*time.Second, cfg.Timeout)
	assert.Equal(t, 4096, cfg.MaxTokens)
}

func TestNewOpenCodeAdapter(t *testing.T) {
	cfg := &OpenCodeConfig{
		BinaryPath: "echo",
		Headless:   true,
	}

	adapter := NewOpenCodeAdapter(cfg)
	assert.NotNil(t, adapter)
	assert.Equal(t, cfg, adapter.config)
}

func TestOpenCodeAdapter_BuildArgs(t *testing.T) {
	tests := []struct {
		name     string
		config   *OpenCodeConfig
		expected []string
	}{
		{
			name: "headless and non-interactive",
			config: &OpenCodeConfig{
				Headless:       true,
				NonInteractive: true,
			},
			expected: []string{"--headless", "--non-interactive"},
		},
		{
			name: "with provider and model",
			config: &OpenCodeConfig{
				Headless:       true,
				NonInteractive: true,
				Provider:       "anthropic",
				Model:          "claude-3.5-sonnet",
			},
			expected: []string{"--headless", "--non-interactive", "--provider", "anthropic", "--model", "claude-3.5-sonnet"},
		},
		{
			name: "with extra flags",
			config: &OpenCodeConfig{
				Headless:       true,
				NonInteractive: true,
				ExtraFlags:     []string{"--verbose", "--debug"},
			},
			expected: []string{"--headless", "--non-interactive", "--verbose", "--debug"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewOpenCodeAdapter(tt.config)
			args := adapter.buildArgs()
			assert.Equal(t, tt.expected, args)
		})
	}
}

func TestOpenCodeParser_ExtractJSON(t *testing.T) {
	parser := NewOpenCodeParser()

	tests := []struct {
		name     string
		input    string
		hasError bool
	}{
		{
			name:  "valid JSON",
			input: `Some text before {"key": "value"} some text after`,
		},
		{
			name:  "nested JSON",
			input: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "no JSON",
			input:    "No JSON here",
			hasError: true,
		},
		{
			name:     "unbalanced JSON",
			input:    `{"key": "value"`,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ExtractJSON(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestOpenCodeParser_Parse(t *testing.T) {
	parser := NewOpenCodeParser()

	tests := []struct {
		name     string
		input    string
		expected *Response
		hasError bool
	}{
		{
			name:  "valid response",
			input: `{"content": "Hello, world!", "model": "gpt-4", "tokens_in": 10, "tokens_out": 5}`,
			expected: &Response{
				Content:   "Hello, world!",
				Model:     "gpt-4",
				TokensIn:  10,
				TokensOut: 5,
			},
		},
		{
			name:     "invalid JSON",
			input:    `not valid json`,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
