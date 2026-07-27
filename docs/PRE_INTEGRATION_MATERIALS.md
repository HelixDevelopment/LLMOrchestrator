# LLMOrchestrator — Pre-Integration Materials

**Revision:** 1
**Last modified:** 2026-07-15T11:19:52Z
**Purpose:** Consolidated pre-integration materials (gate before any integration/deployment work).

> Consolidation-and-verification pass. This document does not restate the design
> narratives — it cross-references the canonical sources
> ([`../ARCHITECTURE.md`](../ARCHITECTURE.md), [`../API_REFERENCE.md`](../API_REFERENCE.md),
> [`../README.md`](../README.md)) and records which pre-integration materials have
> been verified against the real repository. Every statement below is grounded
> in the repository tree; anything not determinable from the code is marked
> `UNKNOWN:` with its reason.

---

## 1. Purpose / What it is

LLMOrchestrator is a **standalone Go module** (`digital.vasic.llmorchestrator`,
`go 1.25` per [`../go.mod`](../go.mod)) that manages **headless CLI LLM agents**
— OpenCode, Claude Code, Gemini, Junie, Qwen Code — behind a unified agent-pool
interface with a hybrid pipe+file communication protocol
([`../README.md`](../README.md) Overview).

It is **shared infrastructure** — deliberately project-agnostic and reusable
(`../README.md`: *"its specialised responsibility makes it reusable, and that
reusability is destroyed the moment any consumer's specifics leak in"*). It
defines its own types and does **not** import consumer modules; per
[`../ARCHITECTURE.md`](../ARCHITECTURE.md) Key Design Decision 5, *"LLMOrchestrator
… does not import LLMsVerifier, VisionEngine, or DocProcessor. HelixQA bridges
them."*

The repository ships in two consumable shapes (both evidenced in-tree):

- a **Go library** — the `pkg/` packages, imported by consumers;
- a **standalone CLI binary** — `cmd/orchestrator` ([`../cmd/orchestrator/main.go`](../cmd/orchestrator/main.go)).

---

## 2. Architecture overview

Full design narrative + Mermaid diagrams (component, sequence, class, circuit-breaker
state, acquisition flowchart) live in [`../ARCHITECTURE.md`](../ARCHITECTURE.md) — not
repeated here. Grounded structural summary (from the real `pkg/`, `internal/`,
`cmd/` trees):

| Package / dir | Real files (evidence) | Responsibility |
|---|---|---|
| `pkg/agent` | `agent.go`, `pool.go`, `simple_pool.go`, `multi_pool.go`, `health.go`, `builders.go`, `claudecode_agent.go`, `gemini_agent.go`, `junie_agent.go`, `opencode_agent.go` (+ `_unix`/`_windows`), `qwencode_agent.go` | `Agent` interface, `AgentPool`, `SimplePool`, `MultiPool`, `HealthMonitor`, `CircuitBreaker`, per-CLI agents |
| `pkg/adapter` | `base.go`, `claudecode.go`, `gemini.go`, `junie.go`, `opencode.go`, `opencode_headless.go`, `qwencode.go` | `BaseAdapter` (shared process spawn/pipe/shutdown) + 5 CLI adapters |
| `pkg/protocol` | `pipe.go`, `file.go`, `message.go` | `PipeTransport` (JSON-lines over stdin/stdout), `FileTransport` (inbox/outbox/shared), `PipeMessage`, `FileMessage`, `validatePath` |
| `pkg/parser` | `parser.go` | `ResponseParser` — structured extraction of actions / issues / JSON from raw LLM output |
| `pkg/config` | `config.go` | `.env` loading, agent-binary path resolution, `Validate()` |
| `pkg/i18n` | `bundle.go`, `translator.go`, `bundles/` | `Translator` interface + `NoopTranslator` default (loud-echo anti-bluff) |
| `internal/archdoc` | `archdoc.go` | internal (not part of the public API surface) |
| `cmd/orchestrator` | `main.go`, `i18n_msg.go` | standalone CLI entry point |

**Orchestration / routing model** (grounded in `../ARCHITECTURE.md` + `pkg/agent/health.go`):

- **Pool-based capability routing** — callers `Acquire(ctx, requirements)` an
  agent from the pool; the pool matches on capabilities (vision / streaming /
  tool-use / token budget per `../README.md` Features) and returns a matching
  `Agent`, blocking on a `sync.Cond` until one is available or the context is
  cancelled.
- **Per-agent circuit breaker** — `pkg/agent/health.go`:
  `DefaultFailureThreshold = 3` consecutive failures → circuit `Open`;
  `DefaultRecoveryTimeout = 60s` → `HalfOpen` probe → `Closed` on success.
- **Adapter subprocess execution** — each adapter spawns its CLI agent as a
  child process and exchanges JSON-line messages over stdin/stdout via
  `PipeTransport`, falling back to `FileTransport` (inbox/outbox/shared) for
  large artifacts (`../ARCHITECTURE.md` Key Design Decision 4).

**Routing is to external CLI agent binaries via subprocess, not to LLM provider
APIs over the network** — the orchestrator does not embed provider SDKs (see §3).

**Stack:** Go (`go 1.25`); standard library for process/pipe/signal handling;
`gopkg.in/yaml.v3` (i18n bundles) and `github.com/stretchr/testify` (tests) are
the only non-stdlib dependencies (§3).

---

## 3. Dependencies

**Own-org submodule dependencies:** NONE.
- [`../helix-deps.yaml`](../helix-deps.yaml): `deps: []` (Catalogue-Check note:
  *"inspected go.mod, go.sum; own-org deps: none"*), `transitive_handling.recursive: true`,
  `conflict_resolution: operator-required`.
- No `.gitmodules` file exists in the repository (verified absent) → zero git
  submodules.

**Go module dependencies** (from [`../go.mod`](../go.mod)):

| Dependency | Version | Kind | Role |
|---|---|---|---|
| `github.com/stretchr/testify` | `v1.11.1` | direct | test assertions |
| `gopkg.in/yaml.v3` | `v3.0.1` | direct | i18n bundle parsing |
| `github.com/davecgh/go-spew` | `v1.1.2-…` | indirect | testify transitive |
| `github.com/kr/pretty` | `v0.3.1` | indirect | test transitive |
| `github.com/pmezard/go-difflib` | `v1.0.1-…` | indirect | testify transitive |
| `github.com/rogpeppe/go-internal` | `v1.14.1` | indirect | test transitive |
| `gopkg.in/check.v1` | `v1.0.0-…` | indirect | test transitive |

No runtime network/HTTP/LLM-SDK Go dependency is present — the only direct
dependencies are a test framework and a YAML parser.

**Downstream LLM-provider dependency:** the actual LLM providers are reached
**indirectly, via external CLI agent binaries** — not via a Go SDK. The
orchestrator resolves and spawns the configured agent CLIs
(`opencode`, `claude`, `gemini`, `junie`, `qwen-code`) whose paths come from
config ([`../pkg/config/config.go`](../pkg/config/config.go) `DefaultConfig` +
`HELIX_AGENT_*_PATH`). Provider API keys are supplied to those CLIs via
environment variables enumerated in [`../.env.example`](../.env.example):
`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GOOGLE_API_KEY`, `GROQ_API_KEY`,
`MISTRAL_API_KEY`, `DEEPSEEK_API_KEY`, `XAI_API_KEY`, `TOGETHER_API_KEY`,
`QWEN_API_KEY`, `JUNIE_API_KEY`. The orchestrator itself does not call these
providers directly.

**Infrastructure dependencies:** none evidenced in-repo (no database, message
broker, or container manifest). Runtime prerequisites are the external agent CLI
binaries (present on the host at the configured paths) plus their provider keys.

---

## 4. Deploy / Distribution design

Dual distribution, both evidenced in-tree:

- **As a Go library** — consumers import the `pkg/*` packages
  (`digital.vasic.llmorchestrator/pkg/agent`, `.../pkg/adapter`,
  `.../pkg/protocol`, `.../pkg/parser`, `.../pkg/config`, `.../pkg/i18n`). This
  is the primary integration path: HelixQA bridges LLMOrchestrator to other
  Helix components (`../ARCHITECTURE.md` Key Design Decision 5). Consumers inject
  their own `i18n.Translator` (`../README.md` Overview).
- **As a standalone CLI binary** — built with `go build ./cmd/orchestrator`
  (`../README.md` Quick Start).

**Distribution slice:** the Go module `digital.vasic.llmorchestrator` in source
form. There is **no container image, Dockerfile, or compose manifest anywhere in
the repository** (verified: `find` for `Dockerfile*` / `docker-compose*` /
`compose*.yml|yaml` returned zero hits). The module is mirrored to multiple
remotes for consumption — [`../upstreams/`](../upstreams/) provides
`push-all.sh`, `setup-remotes.sh`, `sync-all.sh`,
`VasicDigitalGitHub.sh`, `VasicDigitalGitLab.sh`.

`UNKNOWN:` any versioned/binary release artifact channel (e.g. tagged binary
releases) — the repository evidences source distribution + a build command only;
no release-packaging step is present in-tree.

---

## 5. Ports

`UNKNOWN: library + subprocess-orchestration CLI, no own listen port.`

Evidence: a grep over all non-test `*.go` for `ListenAndServe` / `http.Serve` /
`net.Listen` / `Addr:` / `:8xxx` / `:9xxx` / `PORT` / `HTTP_PORT` returned **zero
hits**. The CLI (`../cmd/orchestrator/main.go`) opens no socket — it builds an
agent pool and blocks on `SIGINT`/`SIGTERM`. Communication with agents is over
child-process **stdin/stdout pipes** (`PipeTransport`) and the **filesystem**
(`FileTransport` inbox/outbox/shared), not a network port.

---

## 6. Health

**No network health endpoint** (no `/health`, `/healthz`, or `/ready` route
exists — consistent with §5: there is no HTTP server).

Health is a **programmatic, in-process** concept:

- [`../pkg/agent/health.go`](../pkg/agent/health.go) — `HealthMonitor` +
  `CircuitBreaker` (`CircuitClosed` / `CircuitOpen` / `CircuitHalfOpen`;
  `DefaultFailureThreshold = 3`, `DefaultRecoveryTimeout = 60s`).
- API surface (see [`../API_REFERENCE.md`](../API_REFERENCE.md)):
  `Agent.Health(ctx) HealthStatus` per agent, and
  `AgentPool.HealthCheck(ctx) []HealthStatus` for the whole pool.

`UNKNOWN:` any externally-scrapable health/readiness probe — none is exposed;
health is observable only via the Go API to callers embedding the library.

---

## 7. How it boots

**As a CLI** (`../cmd/orchestrator/main.go`):

1. `go build ./cmd/orchestrator` → run the produced binary.
2. `initI18n()` — wires embedded locale bundles; locale from
   `LLMORCHESTRATOR_LOCALE` / `LC_ALL` / `LANG`, degrading to English.
3. `version` subcommand → prints the version banner (`version = "0.1.0"`) and exits.
4. Config load — `config.LoadFromEnvironment()` (OS env, `HELIX_*` + API-key
   vars), or `config.LoadFromEnv($HELIX_ENV_FILE)` when `HELIX_ENV_FILE` is set;
   then `cfg.Validate()` (exit 1 on failure).
5. `agent.NewPool()` → register each `cfg.EnabledAgents` entry
   (`opencode` / `claude-code` / `gemini` / `junie` / `qwen-code`) via its
   adapter constructor; unresolvable binaries are skipped with a warning.
6. Prints the ready banner, then blocks until `SIGINT`/`SIGTERM`, then
   `pool.Shutdown(ctx)`.

**As a library:** consumed by importing the `pkg/*` packages (no `main` boot) —
the caller constructs a pool/config and drives `Acquire`/`Send`/`Release`
directly (see [`../API_REFERENCE.md`](../API_REFERENCE.md)).

---

## 8. Materials status (verify pass)

Legend: **HAS-VERIFIED** = the material exists in the repo and its content was
confirmed against the real tree during this pass.

| Pre-integration material | Location (evidence) | Status |
|---|---|---|
| README / overview | [`../README.md`](../README.md) | HAS-VERIFIED |
| Architecture narrative + diagrams | [`../ARCHITECTURE.md`](../ARCHITECTURE.md), [`ARCHITECTURE.md`](ARCHITECTURE.md) | HAS-VERIFIED |
| API reference | [`../API_REFERENCE.md`](../API_REFERENCE.md) | HAS-VERIFIED |
| User guide | [`../USER_GUIDE.md`](../USER_GUIDE.md) | HAS-VERIFIED (present) |
| Contributing guide | [`../CONTRIBUTING.md`](../CONTRIBUTING.md) | HAS-VERIFIED (present) |
| Changelog | [`../CHANGELOG.md`](../CHANGELOG.md) | HAS-VERIFIED (present) |
| Dependency manifest | [`../helix-deps.yaml`](../helix-deps.yaml) | HAS-VERIFIED (`deps: []`) |
| Go module + deps | [`../go.mod`](../go.mod), `../go.sum` | HAS-VERIFIED |
| Config template | [`../.env.example`](../.env.example) | HAS-VERIFIED |
| Build/test targets | [`../Makefile`](../Makefile) | HAS-VERIFIED |
| Test-coverage ledger | [`test-coverage.md`](test-coverage.md) | HAS-VERIFIED (present) |
| Host-power-management anchor | [`HOST_POWER_MANAGEMENT.md`](HOST_POWER_MANAGEMENT.md) | HAS-VERIFIED (present) |
| Anti-bluff Challenges | `../challenges/` | HAS-VERIFIED (present) |
| License | [`../LICENSE`](../LICENSE) | HAS-VERIFIED (present) |
| Container/deploy manifest | — | UNKNOWN: none in repo (no Dockerfile/compose; §4) |
| Network port | — | UNKNOWN: none — library + subprocess CLI (§5) |
| Network health endpoint | — | UNKNOWN: none — programmatic health only (§6) |

**Verdict:** **HAS-VERIFIED.** Every pre-integration material required for this
gate is present in the repository and was confirmed against the real tree; no
material had to be gap-filled. The three `UNKNOWN:` rows are determinations
(this component is a Go library + subprocess-orchestration CLI, not a network
server), not missing artifacts.
