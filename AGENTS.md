## INHERITED FROM Helix Constitution

This module is a submodule of a consuming project that
includes the Helix Constitution submodule at the parent's
`constitution/` path. All rules in `constitution/AGENTS.md` and the
`constitution/Constitution.md` it references (universal anti-bluff
covenant §11.4, no-guessing mandate §11.4.6, credentials-handling
mandate §11.4.10, host-session safety §12, data safety §9, mutation-
paired gates §1.1) apply unconditionally to every change landed here.
The module-specific rules below extend them — they never weaken any
universal clause.

When this file disagrees with the constitution submodule, the
constitution wins. Locate the constitution submodule from any
arbitrary nested depth using its `find_constitution.sh` helper.

Canonical reference: <https://github.com/HelixDevelopment/HelixConstitution>

---

# AGENTS.md — LLMOrchestrator

## INHERITED FROM constitution/AGENTS.md

All rules in `constitution/AGENTS.md` (and the `constitution/Constitution.md` it references) apply unconditionally. This file's rules below extend them — they MUST NOT weaken any inherited rule. Use `constitution/find_constitution.sh` from the parent project root to resolve the absolute path of the submodule from any nested location.

## Module Overview

LLMOrchestrator is a standalone, project-not-aware, fully decoupled Go
module (`digital.vasic.llmorchestrator`, Go 1.25) for spawning, managing,
and communicating with multiple headless CLI LLM agents (OpenCode, Claude
Code, Gemini, Junie, Qwen Code) through a hybrid pipe+file communication
protocol. It is shared infrastructure meant to be consumed by multiple
independent projects — the module imports no consuming-project namespace,
and all user-facing strings are routed through an injected
`i18n.Translator` so no consumer's specifics leak in (see
`pkg/i18n/translator.go`).

## Responsibilities

- Spawn and supervise headless CLI agent processes and expose a
  thread-safe `Agent`/`AgentPool` abstraction with capability matching
  (vision / streaming / tool-use / token budget) (`pkg/agent`).
- Per-agent health monitoring via a circuit breaker — 3 consecutive
  failures marks an agent unhealthy (`pkg/agent`).
- Provide 5 CLI adapters on top of a shared `BaseAdapter`
  process-management layer (`pkg/adapter`).
- Communicate over two transports: real-time JSON-lines pipes and
  file-based inbox/outbox/shared directories, with path-traversal
  protection and a 1 MiB response cap (`pkg/protocol`).
- Parse raw LLM stdout into structured actions/issues/JSON
  (`pkg/parser`).
- Load `.env`-based configuration and resolve agent binary paths
  (`pkg/config`).
- Externalise every user-facing string behind a `Translator` interface,
  defaulting to a `NoopTranslator` that returns the message id verbatim
  as a loud fallback (`pkg/i18n`, bundles under `pkg/i18n/bundles`).
- Expose a standalone CLI entry point (`cmd/orchestrator`).
- Keep `docs/ARCHITECTURE.md` honest against the real source tree
  (`internal/archdoc`).

## Build & Test

```bash
go build ./...                        # build all packages + CLI
go build ./cmd/orchestrator           # build the standalone orchestrator binary
go vet ./...                          # static analysis, zero warnings
go test ./... -race -count=1          # unit + integration suite, race detector
```

`make` wraps the common flows (verified against the real `Makefile`):

| Target            | Purpose                                              |
|-------------------|-------------------------------------------------------|
| `make build`      | `go build ./...`                                       |
| `make test`       | `go test ./... -race -count=1`                          |
| `make race`       | same as `test`, verbose                                 |
| `make vet`        | `go vet ./...`                                          |
| `make lint`       | `go vet ./...` (no external linter dependency)          |
| `make fmt`        | `gofmt -w -s .`                                         |
| `make cover`      | race-covered run, emits `coverage.html`                 |
| `make bench`      | `go test ./... -bench=. -benchmem`                      |
| `make fuzz`       | 30s fuzz run for each `pkg/parser` fuzz target          |
| `make check`      | `vet` + `test`                                          |
| `make clean`      | clears the Go build/test cache                          |
| `make upstream-push` / `make upstream-sync` | run `upstreams/push-all.sh` / `upstreams/sync-all.sh` |

Definition-of-Done gates (`scripts/no-silent-skips.sh`,
`scripts/demo-all.sh`) are wired as `make no-silent-skips`,
`make no-silent-skips-warn`, `make demo-all`, `make demo-all-warn`,
`make demo-one MOD=<name>`, and `make ci-validate-all`.

Fuzz targets live in `pkg/parser` (`FuzzParser_Parse`,
`FuzzParser_ExtractJSON`, `FuzzParser_ExtractActions`); run via
`make fuzz` (30s per target).

**Challenge runner** (real-system exerciser, no mocks — see `README.md`
"Anti-bluff guarantees" for exactly what each invariant proves):

```bash
LLMORCH_FIXTURES_DIR=challenges/fixtures go run ./challenges/runner/
bash challenges/llmorchestrator_describe_challenge.sh normal   # exits 0 on green
bash challenges/llmorchestrator_describe_challenge.sh mutate   # exits 99 on mutation-detected
```

The Challenge wrapper also supports a paired-mutation mode
(`LLMORCH_MUTATE_RUNNER=1`) that MUST flip a normally-passing invariant to
a failure, proving the gate itself is not a bluff.

## Package Structure

The `pkg/` layout is flat and shallow on purpose — six packages, plus
the CLI entry point and one internal helper:

| Package             | Purpose                                                                 |
|----------------------|--------------------------------------------------------------------------|
| `pkg/agent`          | `Agent`, `AgentPool` / `SimplePool` / `MultiPool`, `HealthMonitor`, `CircuitBreaker` (3 consecutive failures → unhealthy), per-CLI agent implementations |
| `pkg/adapter`         | `BaseAdapter` + the 5 CLI adapters |
| `pkg/protocol`        | `PipeTransport` (real-time JSON-lines), `FileTransport` (inbox/outbox/shared directories), `PipeMessage`, `FileMessage`, path-traversal guard (`validatePath` / `ErrPathTraversal`) |
| `pkg/parser`          | `DefaultParser` / `ResponseParser` — structured extraction of actions, issues, and JSON from raw LLM output |
| `pkg/config`          | `.env` loading, `DefaultConfig()`, agent path resolution |
| `pkg/i18n`            | `Translator` interface, `NoopTranslator` default, `SetPkgTranslator` injection point; `pkg/i18n/bundles/` holds the embedded locale bundles |
| `cmd/orchestrator`    | Standalone CLI entry point (`main.go`, locale wiring in `i18n_msg.go`) |
| `internal/archdoc`    | Verifies `docs/ARCHITECTURE.md` stays factually consistent with the real source tree; generic, no consumer-project knowledge |

## Key Interfaces & Integration Points

- `agent.Agent` / `agent.Pool.Acquire(ctx, requirements)` — capability
  matching (vision / streaming / tool-use / token budget) over a
  thread-safe pool.
- `adapter.BaseAdapter` — process lifecycle shared by all 5 CLI adapters.
- `protocol.PipeTransport` / `protocol.FileTransport` — the two supported
  wire formats between orchestrator and a spawned CLI agent process.
- `parser.DefaultParser.Parse(...)` — turns raw agent stdout into a
  structured `[]agent.Action` (or the sentinel `ErrEmptyInput`).
- `i18n.Translator` — every consumer supplies its own implementation;
  `NoopTranslator` returns the message id verbatim so a missing
  translation surfaces loudly instead of silently as an empty string.
- `config.LoadFromEnv` — reads `.env` (copy from `.env.example`, mode
  0600, git-ignored) for agent binary paths and API keys.

A consuming project wires this module by:

1. Implementing `i18n.Translator` and calling `i18n.SetPkgTranslator(...)`
   at startup (otherwise the built-in `NoopTranslator` is used, which is
   safe but untranslated).
2. Providing `.env` (copied from `.env.example`, mode 0600, git-ignored)
   with agent binary paths / API keys, loaded via `config.LoadFromEnv`.
3. Choosing a transport per call site: `protocol.PipeTransport` for
   real-time JSON-lines exchange, or `protocol.FileTransport` for
   inbox/outbox/shared-directory exchange.
4. Acquiring agents through `agent.Pool.Acquire(ctx, requirements)` rather
   than constructing adapters directly, so capability matching and
   circuit-breaker health tracking apply.

## Language & Dependencies

- **Language**: Go 1.25 (see `go.mod`, module `digital.vasic.llmorchestrator`).
- **Direct dependencies**: `github.com/stretchr/testify` (tests only),
  `gopkg.in/yaml.v3` (Challenge fixture loading). No web framework, no
  database driver, no UI toolkit — this module has no such surfaces.
- **Own-org submodule dependencies**: none (`helix-deps.yaml` —
  `deps: []`, audited against `go.mod`/`go.sum`).

## Multi-Remote Distribution

- `upstreams/` (lowercase) holds one recipe script per remote, plus
  `push-all.sh` / `sync-all.sh` / `setup-remotes.sh`.
- `install_upstreams.sh` reads every `*.sh` recipe under `upstreams/`
  and configures the corresponding git remote.
- `make upstream-push` / `make upstream-sync` invoke
  `upstreams/push-all.sh` / `upstreams/sync-all.sh` directly.

## Submodule Decoupling

- Never import a consuming project's namespace under `pkg/**`,
  `cmd/**`, or `internal/**` — this module must remain reusable by any
  project that wants to orchestrate headless CLI LLM agents.
- All user-facing strings flow through the injected `i18n.Translator`;
  never hardcode English literals in `pkg/`/`cmd/` call sites.
- `.env` is git-ignored and must be `chmod 600`; only `.env.example` is
  committed.
- Mocks/stubs/placeholders are permitted only in `*_test.go` unit
  tests; `challenges/` exercises the real parser, real disk I/O, real
  JSON encoding, and the real `i18n` surface — see `README.md` for the
  per-invariant evidence.

## Testing Boundaries

- Build: `go build ./...`. Vet: `go vet ./...` (zero warnings).
- Unit + integration suite: `go test ./... -race -count=1` — all packages
  green (`.`, `cmd/orchestrator`, `internal/archdoc`, `pkg/adapter`,
  `pkg/agent`, `pkg/config`, `pkg/i18n`, `pkg/parser`, `pkg/protocol`;
  `challenges/runner` has no test files, it is the Challenge entry
  point).
- Agents MUST NOT weaken, stub, or skip tests, and MUST NOT introduce a
  dependency on any consuming project's namespace. Every change keeps
  `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1`
  green.

## Anti-Bluff Notes

This module's tests and Challenges exist to prove the codebase works,
not merely to compile. Guard against regressions of the following
patterns (all previously-resolved classes, kept here so a future change
does not silently reintroduce them):

- A parser change that returns an empty `[]agent.Action` slice instead
  of a real parsed action for a well-formed fixture.
- A transport change that breaks byte-level field preservation across
  a `PipeMessage` / `FileMessage` round trip.
- An `i18n` change that hardcodes an English string instead of routing
  through `Pkg()` / the injected `Translator`.
- A Challenge or test with `simulated`, `for now`, `TODO implement`, or
  `placeholder` in its behaviour — verify with:
  ```bash
  grep -rn "simulated\|for now\|TODO implement\|placeholder" pkg cmd && echo "BLUFF FOUND" || echo "clean"
  ```

## Anti-Bluff Checklist for Every Task

- [ ] **No simulation**: code doesn't contain "simulate", "for now",
      "TODO implement", "placeholder".
- [ ] **Real HTTP calls / real process spawns**: API clients and CLI
      adapters exercise real endpoints/processes, never `fmt.Printf` +
      `time.Sleep`.
- [ ] **Real file operations**: `os.ReadFile`/`os.WriteFile`, not mock
      in-memory buffers.
- [ ] **Test validates reality**: checks actual behaviour, not just call
      counts.
- [ ] **Challenge validates end-to-end**: exercises the complete
      spawn/communicate/parse workflow.
- [ ] **No bare skips**: every `t.Skip()` carries a `SKIP-OK: #<ticket>`
      marker.
- [ ] **Evidence pasted**: the commit/PR contains actual terminal output
      from a real execution.

## Common Anti-Patterns to Avoid

- **The simulation trap** — returning a canned string instead of really
  invoking the agent process / provider.
- **The hardcoded list** — returning a literal slice instead of a real
  query against the live source.
- **The stub interface** — a method that returns `nil` / `// TODO:
  implement` instead of really dialing, spawning, or connecting.

## Working with Submodules

When adding or updating a sibling own-org module dependency: (1) check
governance — does the dependency carry the full governance-carrier set;
(2) add what's missing, referencing the constitution; (3) verify this
module still compiles against the sibling layout; (4) confirm this
module's own Challenges still run.

## Emergency Procedures

**If you discover a bluff**: stop working on dependent features, document
it under `docs/issues/BLUFFS.md`, write a Challenge that reproduces it,
fix it, verify the Challenge now passes, and update documentation to
reflect reality.

**If a test passes but the feature doesn't work**: the test is a bluff —
tighten it, add assertions on actual output quality, add anti-bluff
checks (no "simulated" in responses), run it against real infrastructure,
verify it FAILS on the broken code, then fix the code.

---

## Constitutional Anti-Bluff Forensic Anchor (CONST-035 / §11.9, inherited)

> Verbatim user mandate: *"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"*
>
> Operative rule: **The bar for shipping is not "tests pass" but "users can use the feature."** Every PASS in this codebase MUST carry positive runtime evidence captured during execution. Metadata-only / configuration-only / absence-of-error / grep-based PASS without runtime evidence are critical defects regardless of how green the summary line looks. No false-success results are tolerable.

This anchor is inherited from the Helix Constitution (`constitution/Constitution.md` §11.9 / CONST-035); resolve it via `constitution/find_constitution.sh` from the parent project root. This submodule stays fully decoupled and project-not-aware (§11.4.28) — this is generic governance inheritance only, never project-specific context.

### Article XII §12.1 (CONST-042) — No-Secret-Leak
No API key, token, password, certificate, or other credential may be committed to any repository owned by HelixDevelopment or vasic-digital. All secrets live in `.env` files (mode 0600) listed in `.gitignore`. Any leak is a release blocker until rotated and post-mortemed.

### Article XII §12.2 (CONST-043) — No-Force-Push
No force push, force-with-lease push, history rewrite, branch deletion of `main`/`master`, or upstream-overwriting operation may be performed without explicit, in-conversation user approval per operation. Authorization for one push does not extend further. Bypassing hooks / signing / protected-branch rules also requires explicit approval.

## Host Power Management — Hard Ban (CONST-033)

**Host Power Management is Forbidden.**

You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition. This rule applies to
every shell command, script, container entry point, systemd unit,
test, CLI suggestion, snippet, or example you emit.

## Resources & References

- **Constitution**: `CONSTITUTION.md` (inheritance pointer to the parent
  project's constitution submodule).
- **This file's siblings** (one per supported CLI) carry the same
  ruleset — keep them in sync with this file.
- `README.md` — overview, install, usage, anti-bluff-guarantees mapping.
- `docs/ARCHITECTURE.md` — kept honest by `internal/archdoc`.
