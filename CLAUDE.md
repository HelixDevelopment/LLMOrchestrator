# CLAUDE.md

## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- All builds and tests are run manually or via Makefile targets
- This rule is permanent and non-negotiable

## Project Overview

**LLMOrchestrator** is a standalone Go module (`digital.vasic.llmorchestrator`) for managing headless CLI agents with hybrid pipe+file communication.

## Build & Test

```bash
go build ./...
go test ./... -race -count=1
go vet ./...
```

## MANDATORY: Never Remove or Disable Tests

NO test may ever be removed, disabled, skipped, or left broken. All issues must be fixed by addressing root causes.

## Architecture

5 packages in `pkg/`:
- `agent/` - Agent interface, AgentPool (thread-safe), CircuitBreaker, HealthMonitor
- `adapter/` - BaseAdapter + 5 CLI adapters (opencode, claudecode, gemini, junie, qwencode)
- `protocol/` - PipeTransport (JSON-lines), FileTransport (inbox/outbox/shared)
- `parser/` - ResponseParser (JSON/action/issue extraction from raw LLM output)
- `config/` - .env loading, agent path resolution, validation

## Key Patterns

- **BaseAdapter**: Shared process management; each adapter only implements parsing
- **AgentPool**: sync.Mutex + sync.Cond for blocking acquire with context cancellation
- **CircuitBreaker**: 3 consecutive failures = open for 60s
- **Security**: Path traversal protection, response length limits, API key masking

## Conventions

- SPDX license headers on all Go files
- Test files: `*_test.go`, `*_stress_test.go`, `*_security_test.go`, `*_fuzz_test.go`
- No TODO/FIXME in production code

## Dependencies

- Go 1.24+
- github.com/stretchr/testify (test only)
