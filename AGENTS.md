# Supported Agents

## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- All builds and tests are run manually or via Makefile targets
- This rule is permanent and non-negotiable

## Agent Overview

| Agent | CLI Binary | Headless Mode | Vision | Max Tokens |
|-------|-----------|---------------|--------|------------|
| OpenCode | `opencode` | `--headless --non-interactive` | Via configured LLM | 128K |
| Claude Code | `claude` | `--print --output-format json` | Native (Claude) | 200K |
| Gemini | `gemini` | `--non-interactive` | Native (Gemini) | 1M |
| Junie | `junie` | `--headless` | Via configured LLM | 128K |
| Qwen Code | `qwen-code` | `--headless --non-interactive` | Native (Qwen-VL) | 128K |

## OpenCode

OpenCode is a multi-provider CLI agent. It supports OpenAI, Anthropic, and Google providers. Vision capabilities depend on the configured LLM backend.

**Response format**: JSON with `content`, `tool_use`, `tokens_in`, `tokens_out` fields.

## Claude Code

Anthropic's official CLI for Claude. Uses `--print --output-format json` for headless operation. Native vision through Claude's multimodal capabilities.

**Response format**: JSON with `result`, `usage.input_tokens`, `usage.output_tokens`, `model` fields.

## Gemini

Google's Gemini CLI agent. Supports the largest context window (1M tokens). Native vision through Gemini's multimodal capabilities.

**Response format**: JSON with `text`, `token_count`, `finish_reason` fields.

## Junie

JetBrains' AI coding assistant. Uses `--headless` mode. Vision capabilities depend on the configured backend.

**Response format**: JSON with `response`, `status`, `tokens` fields.

## Qwen Code

Alibaba's Qwen-VL coding assistant. Native vision through Qwen-VL model. Uses `--headless --non-interactive` mode.

**Response format**: JSON with `output`, `token_usage.input`, `token_usage.output`, `model` fields.

## Adding a New Agent

1. Create `pkg/adapter/youragent.go`
2. Embed `*BaseAdapter`
3. Implement `parseYourAgentResponse(raw string) (agent.Response, error)`
4. Set appropriate flags in the constructor
5. Add tests in `pkg/adapter/adapter_test.go`
6. Update this document
