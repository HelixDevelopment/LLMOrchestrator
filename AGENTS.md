## INHERITED FROM Helix Constitution

> Base agent rules live in the Helix Constitution submodule at the
> parent project's `constitution/AGENTS.md` and the universal
> `constitution/Constitution.md` it references. **READ THOSE FIRST.**
> The base file is authoritative for any topic not covered here.
> Module-specific rules below extend them; they never weaken them.

Critical universal rules every CLI agent (Claude Code, Cursor, Aider,
Codex, Gemini CLI) MUST honour while working in this module:

- **No bluffing.** Every PASS carries positive evidence. Constitution §11.4.
- **Mutation-paired gates.** Every new gate has a paired mutation
  proving it catches regressions. Constitution §1.1.
- **No guessing language** (`likely`, `probably`, `maybe`, `seems`).
  Constitution §11.4.6.
- **Credentials never tracked.** `.env` patterns git-ignored; runtime-load
  only. Constitution §11.4.10.
- **Never force-push.** Force-push requires explicit per-session
  authorization AND a green post-op gate. Constitution §9.
- **CONTINUATION.md kept in sync** in every non-trivial commit.
  Constitution §12.10.
- **60% RAM cap.** Heavy work wrapped in bounded execution scope.
  Constitution §12.6.

Canonical reference: <https://github.com/HelixDevelopment/HelixConstitution>

---

# AGENTS.md — LLMOrchestrator

## INHERITED FROM constitution/AGENTS.md

All rules in `constitution/AGENTS.md` (and the `constitution/Constitution.md` it references) apply unconditionally. This file's rules below extend them — they MUST NOT weaken any inherited rule. Use `constitution/find_constitution.sh` from the parent project root to resolve the absolute path of the submodule from any nested location.

## Module Identity

LLMOrchestrator (`digital.vasic.llmorchestrator`, Go 1.25) is a
standalone, project-not-aware, fully decoupled Go module for spawning,
managing, and communicating with multiple headless CLI LLM agents
(OpenCode, Claude Code, Gemini, Junie, Qwen Code) via a hybrid
pipe+file communication protocol. It never imports a consuming
project's namespace; project-specific behaviour (translations, agent
binary paths, API keys) is injected at runtime via the `i18n.Translator`
and `config` contracts.

## Responsibilities

- Spawn and supervise headless CLI agent processes and expose a
  thread-safe `Agent`/`AgentPool` abstraction with capability matching
  (vision / streaming / tool-use / token budget) (`pkg/agent`).
- Per-agent health monitoring via a circuit breaker — 3 consecutive
  failures marks an agent unhealthy (`pkg/agent`).
- Provide 5 CLI adapters (OpenCode, Claude Code, Gemini, Junie, Qwen
  Code) on top of a shared `BaseAdapter` process-management layer
  (`pkg/adapter`).
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

## Testing Boundaries

- Build: `go build ./...`. Vet: `go vet ./...` (zero warnings).
- Unit + integration suite: `go test ./... -race -count=1` — verified
  green across all 9 packages in this checkout (`.`, `cmd/orchestrator`,
  `internal/archdoc`, `pkg/adapter`, `pkg/agent`, `pkg/config`,
  `pkg/i18n`, `pkg/parser`, `pkg/protocol`; `challenges/runner` has no
  test files, it is the Challenge entry point).
- Fuzz targets live in `pkg/parser` (`FuzzParser_Parse`,
  `FuzzParser_ExtractJSON`, `FuzzParser_ExtractActions`); run via
  `make fuzz` (30s per target).
- Mocks/stubs/placeholders are permitted ONLY in `*_test.go` unit
  tests. The `challenges/` tree (runner + wrapper script + chaos/DDoS/
  scaling/stress/UI/UX scripts under `challenges/scripts/`) exercises
  the real, fully-implemented system: real filesystem I/O, real JSON
  encoding, real parser execution — never a mock.
- The Challenge wrapper supports a paired-mutation mode
  (`LLMORCH_MUTATE_RUNNER=1`) that MUST flip a normally-passing
  invariant to a failure, proving the gate itself is not a bluff:
  ```bash
  bash challenges/llmorchestrator_describe_challenge.sh normal   # exit 0
  bash challenges/llmorchestrator_describe_challenge.sh mutate   # exit 99
  ```
- Agents MUST NOT weaken, stub, or skip tests, and MUST NOT introduce a
  dependency on any consuming project's namespace. Every change keeps
  `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1`
  green.

## Integration Points For Consumers

A consuming project wires this module by:

1. Implementing `i18n.Translator` and calling
   `i18n.SetPkgTranslator(...)` at startup (otherwise the built-in
   `NoopTranslator` is used, which is safe but untranslated).
2. Providing `.env` (copied from `.env.example`, mode 0600,
   git-ignored) with agent binary paths / API keys, loaded via
   `config.LoadFromEnv`.
3. Choosing a transport per call site: `protocol.PipeTransport` for
   real-time JSON-lines exchange, or `protocol.FileTransport` for
   inbox/outbox/shared-directory exchange.
4. Acquiring agents through `agent.Pool.Acquire(ctx, requirements)`
   rather than constructing adapters directly, so capability matching
   and circuit-breaker health tracking apply.

## Multi-Remote Distribution

- `upstreams/` (lowercase) holds one recipe script per remote
  (`GitHub.sh`, `VasicDigitalGitHub.sh`, `VasicDigitalGitLab.sh`, plus
  `push-all.sh` / `sync-all.sh` / `setup-remotes.sh`).
- `install_upstreams.sh` reads every `*.sh` recipe under `upstreams/`
  and configures the corresponding git remote.
- `make upstream-push` / `make upstream-sync` invoke
  `upstreams/push-all.sh` / `upstreams/sync-all.sh` directly.
- This module declares no own-org submodule dependencies
  (`helix-deps.yaml`: `deps: []`).

## Anti-Bluff Testing Rules

- Unit tests: mocks OK.
- Integration / Challenge / fuzz / stress / chaos / DDoS / scaling /
  UI / UX suites: real system only, no mocks (see
  `challenges/scripts/` for the supplementary Challenge set).
- Every PASS carries positive evidence — see `README.md`
  "Anti-bluff guarantees" for the exact invariant-to-evidence mapping
  (real disk I/O round trip, real JSON field-preservation check, real
  parser execution against 5-locale fixtures, real defensive-contract
  check for empty input, real i18n verbatim-passthrough check).
- No bare `t.Skip()` without a tracked-ticket marker.


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

---

## CONST-036: LLMsVerifier Single Source of Truth Mandate

**Rule**: LLMsVerifier SHALL BE the sole authoritative source for:
1. All model metadata (names, IDs, context windows, capabilities)
2. All provider metadata (endpoints, auth types, supported models)
3. All verification status (verified, partial, failed, pending)
4. All scoring data (overall scores, capability scores, tier rankings)

**Prohibition**: NO hardcoded model lists, NO hardcoded provider lists, NO simulated model discovery. Any code path that presents a model or provider listing to a user MUST fetch that listing from the LLMsVerifier subsystem or its cached replica.

**Anti-Bluff Verification**:
- Challenge script `challenges/scripts/verifier_hardcode_check.sh` scans all Go source files for hardcoded model arrays.
- The only permitted hardcoded data is the 7-entry fallback list in `internal/verifier/fallback_models.go`.

---

## CONST-037: Model Provider Anti-Bluff Guarantee

**Rule**: Every model displayed to an end user MUST have been verified by LLMsVerifier within the last 24h. Models older than this MUST display a "stale" indicator and be deprioritized.

**Anti-Bluff Testing**:
- Unit tests MAY mock the verifier client.
- Integration tests MUST start the verifier server and perform real provider discovery.
- The Makefile target `make test-verifier-integration` MUST exist and run without mocks.

---

## CONST-038: Real-Time Model Status Accuracy

**Rule**: Model status (available, rate-limited, cooldown, offline, deprecated) displayed to users MUST reflect the actual state as known by LLMsVerifier within 60 seconds.

**Polling vs. Push**:
- If WebSocket/SSE push is unavailable, the system MUST poll LLMsVerifier at most every 60s.
- The TUI MUST display a "last updated" timestamp with every model listing.
- Models in "cooldown" or "rate-limited" state MUST show the estimated recovery time if known.

---

## CONST-039: All Providers and Models Integration Mandate

**Rule**: HelixCode MUST integrate with ALL providers that LLMsVerifier supports, subject only to:
1. The provider being explicitly disabled in configuration (`enabled: false`)
2. The API key being absent and the provider requiring one
3. The provider being marked `deprecated` in the verifier database

**Minimum Provider Set** (SHALL NOT be reduced without constitutional amendment):
OpenAI, Anthropic, Gemini, DeepSeek, Groq, Mistral, xAI, OpenRouter, Ollama, Llama.cpp.

---

## CONST-040: MCP / LSP / ACP / Embedding / RAG / Skills / Plugins Integration Mandate

**Rule**: LLMsVerifier integration SHALL extend beyond basic model listing to cover ALL capability dimensions:

1. **MCP**: The verifier MUST report which models support MCP tool calling.
2. **LSP**: The verifier MUST report code-analysis capabilities.
3. **ACP**: The verifier MUST report multi-agent coordination support.
4. **Embedding**: The verifier MUST report `supports_embeddings` for each model.
5. **RAG**: The verifier MUST report context-window sizes for chunking strategies.
6. **Skills / Plugins**: The verifier MUST track plugin compatibility.

**Prohibition**: Capability flags MUST NOT be hardcoded. The `Provider.GetCapabilities()` method MUST return data sourced from the verifier's `VerificationResult` fields.

---

## Free AI Providers

- **XAI (Grok)**: `grok-3-fast-beta`, `grok-3-mini-fast-beta`
- **OpenRouter**: Free models from various providers
- **GitHub Copilot**: `gpt-4o`, `claude-3.5-sonnet` (with subscription)
- **Qwen**: 2,000 requests/day free tier

---

## Host Power Management — Hard Ban (CONST-033)

**Host Power Management is Forbidden.**

You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition. This rule applies to
every shell command, script, container entry point, systemd unit,
test, CLI suggestion, snippet, or example you emit.

## Common Issues

1. **Build fails**: Run `make logo-assets` then `make build`
2. **Database errors**: Check `HELIX_DATABASE_PASSWORD`
3. **Worker SSH failures**: Verify SSH key authentication
4. **LLM timeouts**: Check provider status and config
5. **Redis connection failures**: Check `HELIX_REDIS_PASSWORD` and `redis.enabled`
6. **Test skips**: Ensure `SKIP-OK: #<ticket>` marker is present for any intentional skips

---

## Resources & References

- **Constitution**: `CONSTITUTION.md`
- **CLAUDE.md**: `CLAUDE.md`
- **Gap Analysis**: `HELIXCODE_GAP_ANALYSIS.md`
- **Zero-Bluff Plan**: `HELIXCODE_ZERO_BLUFF_PLAN.md`
- **Testing Strategy**: `ANTI_BLUFF_TESTING_STRATEGY.md`
- **OpenAPI Spec**: `HelixCode/api/openapi.yaml`
- **Docker Guide**: `HelixCode/DOCKER_DEPLOYMENT.md`

---

<!-- END host-power-management addendum (CONST-033) -->


## MANDATORY ANTI-BLUFF COVENANT — END-USER QUALITY GUARANTEE (User mandate, 2026-04-28)

**Forensic anchor — direct user mandate (verbatim):**

> "We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"

This is the historical origin of the project's anti-bluff covenant.
Every test, every Challenge, every gate, every mutation pair exists
to make the failure mode (PASS on broken-for-end-user feature)
mechanically impossible.

**Operative rule:** the bar for shipping is **not** "tests pass"
but **"users can use the feature."** Every PASS in this codebase
MUST carry positive evidence captured during execution that the
feature works for the end user. Metadata-only PASS, configuration-
only PASS, "absence-of-error" PASS, and grep-based PASS without
runtime evidence are all critical defects regardless of how green
the summary line looks.

**Tests AND Challenges (HelixQA) are bound equally** — a Challenge
that scores PASS on a non-functional feature is the same class of
defect as a unit test that does. Both must produce positive end-
user evidence; both are subject to the §8.1 five-constraint rule
and §11 captured-evidence requirement.

**Canonical authority:** parent
[`docs/guides/ATMOSPHERE_CONSTITUTION.md`](../../docs/guides/ATMOSPHERE_CONSTITUTION.md)
§8.1 (positive-evidence-only validation) + §11 (bleeding-edge
ultra-perfection quality bar) + §11.3 (the "no bluff" CLAUDE.md /
AGENTS.md mandate) + **§11.4 (this end-user-quality-guarantee
forensic anchor — propagation requirement enforced by pre-build
gate `CM-COVENANT-PROPAGATION`)**.

**§11.4.1 extension (Phase 33, 2026-05-05) — FAIL-bluffs equally
forbidden.** A test that crashes for a script-internal reason
(undefined variable under `set -u`, regex error, malformed assertion,
missing argument) and produces a FAIL exit code is just as misleading
as a PASS-bluff. Both let real defects ship undetected. Per parent
[Constitution §11.4.1](../../../../docs/guides/ATMOSPHERE_CONSTITUTION.md#114-end-user-quality-guarantee--forensic-anchor-user-mandate-2026-04-28),
every test MUST fail ONLY for genuine product defects — script-bug
failures must be fixed at the source layer (helper library, shared
lib, test source), not patched in individual call sites.

Non-compliance is a release blocker regardless of context.

**§11.4.2 extension (Phase 34, 2026-05-06) — Recorded-evidence
requirement.** A test that emits PASS without captured visual or
audio evidence of the user-visible feature actually working on the
screen the user would see is a §11.4 PASS-bluff. Bug #13 (VK Video
on PRIMARY display while a passing test claimed playback PASS)
demonstrated the gap exactly. Closing it requires the recording +
analyzer infrastructure (Bug #14 — `dual_display_record.sh` /
`action_timeline.sh` / Go `recording-analyzer` / `helixqa-bridge`).
Per Constitution §11.4.2 every PASS for a user-visible feature
MUST be cross-checked by the analyzer against the dual-display
recording + action timeline. A PASS that lacks at least one matched
timeline event in the analyzer findings is treated as a §11.4
PASS-bluff.

Non-compliance is a release blocker regardless of context.

**§11.4.3 extension (Phase 34, 2026-05-06) — Per-device-topology
test dispatch.** Tests that depend on hardware topology (secondary
HDMI present/absent, microphone present/absent, etc.) MUST detect
topology at test entry and dispatch the topology-appropriate
variant. A test running the wrong variant for the actual topology
and PASSing is a §11.4 PASS-bluff. Bug #18 (Lampa+TorrServe E2E)
demonstrated the pattern: D1 (secondary HDMI) and D2 (primary only)
get separate test variants behind a `dumpsys display`-based
dispatcher. Per Constitution §11.4.3 every topology-touching test
MUST have such a dispatcher OR explicit topology gates with
SKIP-with-reason fallback.

Non-compliance is a release blocker regardless of context.

**§11.4.4 extension (User mandate, 2026-05-06) —
Test-interrupt-on-discovery + retest-from-clean-baseline.** A test
cycle that continues running past a freshly discovered defect is
itself a §11.4 PASS-bluff: it produces "all green" summaries while
the codebase under test is known-broken at the moment those greens
were recorded. Phase 34.S' D1 demonstrated the violation when Bug
#26 (hard-floor probe lifecycle) and Bug #27 (analyzer FAIL-bluff
on non-video tests) were discovered mid-cycle and the cycle was
allowed to continue, accumulating 13+ false-positive ANALYZER FAIL
banners. Per Constitution §11.4.4 the moment any defect is re-
discovered, re-produced, or newly identified during a test cycle,
the cycle MUST stop on both devices. **Then**: (1) fix at root cause
per §11.4.1, (2) land validation/verification tests for the fix —
pre-build gate AND on-device test AND paired meta-test mutation,
(3) full rebuild via `scripts/build.sh` (regardless of whether the
fix touched host script / Go binary / firmware — host-only fixes
still get a full rebuild for retest baseline integrity),
(4) re-flash D1 + D2, (5) repeat full `test_all_fixes.sh` from the
beginning sequentially per §12.6, (6) end the cycle with
`meta_test_false_positive_proof.sh` proving no gate is itself a
bluff gate. Tests AND HelixQA Challenges are bound equally —
Challenges that score PASS on a non-functional feature are the same
class of defect as PASS-bluff unit tests; both must produce
positive end-user evidence per §11.4.2 + §11.4.3.

Non-compliance is a release blocker regardless of context.

**§11.4.4 expansion (User mandate, 2026-05-06) — Systematic
debugging + four-layer test coverage + documentation + no-bluff
certification.** Augments the §11.4.4 base covenant with four
non-negotiable additional requirements per the User mandate of
2026-05-06: (a) **Systematic debugging via superpowers skills.**
Before applying any fix, run in-depth systematic debugging using the
available `superpowers:*` skills (debugging, root-cause analysis,
architectural-impact). Symptom patches are forbidden. The debugging
output MUST identify root cause at source layer, blast radius across
related tests/features/subsystems, and the regression-protection
seam. (b) **Four-layer test coverage per fix.** Every fix lands with
positive evidence in **every applicable layer**: pre-build gate
(catches at source), post-build gate (catches in assembled image —
proves bytes landed, cf. Fix #122 APK_LIB_MAP misroute), post-flash
on-device test (fully automated, anti-bluff per §8.1, captured-
evidence per §11.4.2, topology-dispatched per §11.4.3, orchestrator-
wired in `test_all_fixes.sh`), HelixQA test bank entry
(`banks/atmosphere.yaml` + per-feature additions), HelixQA full QA
session coverage (Challenge-driven dispatch — bank entry without
Challenge coverage is a §11.4 PASS-bluff), and meta-test paired
mutation. Skipping a layer because "this fix only touches X" is
forbidden. (c) **Documentation update for every fix.** Required:
`docs/Issues.md` → `docs/Fixed.md` migration on closure, parent
CLAUDE.md Applied Fixes Reference row, affected user-facing guides
(`docs/guides/*.md`), affected diagrams/flowcharts/architecture
docs, per-version `docs/changelogs/<tag>.md` entry. Documentation
drift after a fix is itself a §11.4 violation. (d) **No-bluff
certification per cycle.** Before tagging: `meta_test_false_positive
_proof.sh` returns all gates green AND every gate's paired mutation
FAILs (no bluff gates); `docs/Issues.md` open-set is empty or every
entry explicitly classified out-of-scope-for-this-tag with operator
sign-off (no known issues hidden); full suite returns zero new FAILs
on either device (no working feature regressed); every gate has a
paired mutation; every test produces positive evidence; every
assertion catches its own negation (no error-prone or bluff-proof
leftover).

Non-compliance is a release blocker regardless of context.

**§11.4.5 — Audio + video quality analysis comprehensiveness (User mandate, 2026-05-07)**

**Forensic anchor — direct user mandate (verbatim, 2026-05-07):**

> "We MUST HAVE still analyzing of recorded materials and comprehensive
> validation and verification for issues we used to test! For example
> if there is audio at all or video, if so, is it good and proper or
> is it faulty? Does it have glitches, frame issues and other possible
> obstructions? IMPORTANT: Make sure that all existing tests and
> Challenges do work in anti-bluff manner — they MUST confirm that all
> tested codebase really works as expected!"

§11.4.2 mandates *captured* evidence; §11.4.5 mandates the **content**
of that evidence be analyzed for quality, not merely for presence. A
test that captures a 0-byte mp4 (Bug #24) and PASSes because "the
recording file exists" is the exact PASS-bluff pattern §11.4 forbids.
Content-quality analysis is what closes that gap.

**Audio quality analysis — every audio test that PASSes MUST verify
ALL of:** (1) **Presence** — non-trivial RMS amplitude in captured
WAV / `/proc/asound/.../pcm*p/sub0/hw_params`. (2) **Channel count**
— `ffprobe -show_streams` matches the test's claim (2.0 / 5.1 / 7.1).
(3) **Sample rate + bit depth** — match the codec / pipeline under
test. (4) **Glitch census** — XRUN / FastMixer underrun-overrun-partial
/ AudioFlinger writeError counts above tolerance MUST classify
explicitly (PASS within budget, WARN above, FAIL on hard limits per
§11.4.1 SKIP-vs-FAIL decision tree). (5) **Coexistence-artifact
census** — for tests that exercise WiFi/BT alongside audio: BT TX
queue overflow, A2DP src underflow, coex notification storms, 2.4 GHz
radio contention.

**Video quality analysis — every video test that PASSes MUST verify
ALL of:** (1) **Presence** — captured screen recording has non-zero
file size AND `ffprobe -count_frames` reports decoded-frame total > 0.
0-byte mp4 (Bug #24) is the canonical PASS-bluff and triggers §11.4.4
STOP. (2) **Routing target** — analyzer + action-timeline confirms
video appeared on the *intended* display (primary vs secondary HDMI;
Bug #13 pattern). (3) **Frame health** — drop count, frame-time
variance (jitter), freeze detection (SSIM > 0.99 for ≥ 1 s), tearing.
(4) **Obstruction census** — Tesseract OCR scan for hostile overlays
(`Application not responding`, `Force close`, sign-in dialog,
geo-restriction overlay, ad break, paywall, `App is not certified`).
(5) **Resolution + codec** — captured frame dimensions match the
test's claim; downgrade is a PASS-bluff.

**Challenges (HelixQA) are bound equally** — every Challenge that
asserts PASS MUST run all five audio + five video layers. A Challenge
that scores PASS without applicable analysis is the same class of
defect as a unit test that does.

**Tooling guarantee:** audio = `tinycap` + `aplay --dump-hw-params` +
`ffprobe` + `/proc/asound` parsers (`lib/audio_validation.sh` per
§11.2.5). Video = `screenrecord` + `ffprobe -count_frames` +
`recording-analyzer` + Tesseract OCR (`scripts/dual_display_record.sh`
+ `cmd/recording-analyzer/` per §11.4.2.A and §11.4.2.C). Tests
dispatched against video evidence MUST honor §11.4.4
test-interrupt-on-discovery when the analyzer reports empty input —
do not silently absorb that as a generic PASS-bluff banner.

Non-compliance is a release blocker regardless of context.



## MANDATORY §12 HOST-SESSION SAFETY — INCIDENT #2 ANCHOR (2026-04-28)

**Second forensic incident:** on 2026-04-28 18:36:35 MSK the user's
`user@1000.service` was again SIGKILLed (`status=9/KILL`), this time
WITHOUT a kernel OOM kill (systemd-oomd inactive, `MemoryMax=infinity`)
— a different vector than Incident #1. Cascade killed `claude`,
`tmux`, the in-flight ATMOSphere build, and 20+ npm MCP server
processes. Likely cumulative cgroup pressure + external watchdog.

**Mandatory safeguards effective 2026-04-28** (full text in parent
[`docs/guides/ATMOSPHERE_CONSTITUTION.md`](../../../../docs/guides/ATMOSPHERE_CONSTITUTION.md)
§12 Incident #2):

1. `scripts/build.sh` MUST source `lib/host_session_safety.sh` and
   call `host_check_safety` BEFORE any heavy step.
2. `host_check_safety` has 7 distress detectors including conmon
   cgroup-events warnings (#6) and current-boot session-kill events
   (#7).
3. Containers MUST be clean-slate destroyed + rebuilt after any
   suspected §12 incident. `mem_limit` is per-container, not
   per-user-slice — operator MUST cap Σ `mem_limit` ≤ physical RAM
   − user-session overhead.
4. 20+ npm-spawned MCP server processes are a known memory multiplier;
   stop non-essential MCPs before heavy ATMOSphere work.
5. **Investigation: Docker/Podman as session-loss vector.** Per-container
   cgroups don't prevent cumulative user-slice pressure; conmon
   `Failed to open cgroups file: /sys/fs/cgroup/memory.events`
   warnings preceded the 18:36:35 SIGKILL by 6 min — likely correlated.

This directive applies to every owned ATMOSphere repo and every
HelixQA dependency. Non-compliance is a Constitution §12 violation.



## MANDATORY §12.6 MEMORY-BUDGET CEILING — 60% MAXIMUM (User mandate, 2026-04-30)

**Forensic anchor — direct user mandate (verbatim):**

> "We had to restart this session 3rd time in a row! The system of
> the host stays with no RAM memory for some reason! First make sure
> that whatever we do through our procedures related to this project
> MUST NOT use more than 60% of total system memory! All processes
> MUST be able to function normally!"

**The mandate.** Project procedures MUST NOT use more than **60%
of total system RAM** (`HOST_SAFETY_MAX_MEM_PCT`). The remaining
40% is reserved for the operator's other workloads so the host can
keep serving them while project work proceeds.

**Three consecutive session-loss SIGKILLs on 2026-04-30** during
1.1.5-dev — every one happened while `scripts/build.sh` was running
`m -j5` AOSP. Each Soong/Ninja job peaks at ~5–8 GiB RSS;
collective RSS overran the 60% envelope and the kernel OOM-killer
escalated, taking down `user@1000.service`. **§12.1's pre-flight
check (refusing to start if host already distressed) was not enough**
— the missing piece was an active CONSTRAINT on heavy work itself.

**Mandatory protections (rock-solid):**

1. `HOST_SAFETY_MAX_MEM_PCT` defaults to 60 in
   `scripts/lib/host_session_safety.sh`.
2. `HOST_SAFETY_BUDGET_GB` is computed at source-time from
   `MemTotal × MAX_PCT/100`.
3. `bounded_run` clamps `MemoryMax` down to the budget if the
   caller asks for more (cgroup-level enforcement via
   `systemd-run --user --scope -p MemoryMax=…`).
4. `host_safe_parallel_jobs` and `host_safe_build_jobs` return
   the safe `-j` count given an estimated per-job RSS, capped at
   `nproc`.
5. `scripts/build.sh` wraps `m -j` in `bounded_run`. If the
   build's collective RSS exceeds the budget, only the scope is
   OOM-killed; `user@<uid>.service` stays alive.

**Captured-evidence enforcement.** Pre-build gate
`CM-MEMBUDGET-METATEST` locks all 7 invariants and fires every
pre-build run.

**No escape hatch.** §12.6 has NO operator-facing override flag.
The cap exists for the operator's own protection; bypassing it is
the bluff the §11.4 covenant specifically prohibits. Operators who
need more headroom should reduce parallelism, close other
workloads, or add RAM — NOT raise the percentage.

**Canonical authority:** parent
[`docs/guides/ATMOSPHERE_CONSTITUTION.md`](../../docs/guides/ATMOSPHERE_CONSTITUTION.md)
§12.6.

Non-compliance is a release blocker regardless of context.
*Built with zero-bluff commitment. Every feature actually works.*

## §11.4.69 — Universal Sink-Side Positive-Evidence Taxonomy + Mechanical Enforcement (cascaded from constitution submodule §11.4.69)

> Verbatim user mandate (2026-05-20): *"THIS MUST HAPPEN NEVER AGAIN!!! We MUST HAVE this all working! Not just for audio but for every single piece of the System!!! Proper full automation when executed with success MUST MEAN that manual testing will be as much positive at least regarding the success results! ... Solution MUST BE universal, generic that solves working flows for all System components and for all future and all existing projects! ... Everything we do MUST BE validated and verified with rock-solid proofs and anti-bluff policy enforcement and fulfillment!"*

Universal generalisation of §11.4.68 (audio-specific) across every user-visible feature class. Every user-visible feature MUST map to one entry in the closed-set §11.4.69 sink-side evidence taxonomy (`audio_output`, `audio_input`, `video_display`, `network_throughput`, `network_connectivity`, `bluetooth_a2dp`, `bluetooth_pair`, `touch_input`, `sensor`, `gpu_render`, `storage_read`, `storage_write`, `mediacodec_decode`, `mediacodec_encode`, `miracast`, `cast`, `boot_service`, `package_install`, `permission_grant`, `wifi_link`, `wifi_throughput`, `ethernet_link`, `display_topology`, `drm_playback`, `subtitle_render` — open to additions, never contraction). Every PASS for a feature in the taxonomy MUST cite a captured-evidence artefact path matching the required evidence shape. New helper contracts (additive during grace, mandatory after 2026-06-19): `ab_pass_with_evidence <description> <evidence_path>` (verifies path exists + non-empty), `ab_skip_with_reason <description> <closed-set-reason>` (reasons: `geo_restricted`, `operator_attended`, `hardware_not_present`, `topology_unsupported`, `network_unreachable_external`, `feature_disabled_by_config`; forbids `network_unreachable_external` for any taxonomy feature with a sink-side probe); bare `ab_pass` deprecated (WARN pre-grace, FAIL post-grace). Three pre-build gates + paired §1.1 mutations: `CM-SINK-EVIDENCE-PER-FEATURE`, `CM-NO-FAIL-OPEN-SKIP`, `CM-AB-PASS-WITH-EVIDENCE-EVERYWHERE`. No escape hatch — no `--skip-evidence`, `--config-only-pass`, `--allow-fail-open-skip`, `--legacy-ab-pass-permitted` flag.

**Cascade requirement:** This anchor (verbatim or by `§11.4.69` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-69-PROPAGATION` enforces the anchor literal across the consumer fleet; paired mutation strips the literal → gate FAILs. Severity-equivalent to a §11.4 PASS-bluff at the sink-side-evidence layer.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.69 for the full mandate.


## §11.4.75 — Mechanical Enforcement Without Exception (cascaded from constitution submodule §11.4.75)

> Verbatim user mandate (2026-05-20): *"Why do these violations still happen!? This is a serious problem! We cannot rely on stability nor consistency if we cannot respect our Constitution, mandatory rules and constraints! Is there a way to make this always respected, followed and applied without exception fully and unconditionally!? WE MUST HAVE THIS WORKING FLAWLESSLY!!! Do investigate the root causes of such problems! Once all problems are identified WE MUST apply proper mechanisms for this not to happen NEVER EVER AGAIN!"*

The §11.4 covenant historically relied on agent + operator vigilance; three 2026-05-19→20 forensic incidents proved that late-binding enforcement fires hours-to-days after the violator commit reaches every remote. §11.4.75 closes the gap with FIVE independent mechanical enforcement layers — bypassing any single layer does not bypass the discipline: (1) local `pre-commit` git hook (refuses staged `.md` lacking sibling `.html`+`.pdf`); (2) `commit_all.sh` integration (`_constitution_sibling_check` + auto-`sync_all_markdown_exports.sh` self-repair); (3) local `pre-push` git hook (re-runs siblings + propagation-gate subset); (4) `post-commit` auto-repair hook (auto-generates orphan-`.md` siblings, idempotent + recursion-guarded); (5) local-only final-gate ritual (remote CI DISABLED per User mandate — operator runs `pre_build_verification.sh` + meta-test before every tag per §11.4.40). Helper contracts: `scripts/install_git_hooks.sh`, `scripts/git_hooks/{pre-commit,pre-push,post-commit,commit-msg}`, `_constitution_sibling_check`. The `commit-msg` hook enforces a `Bypass-rationale: <reason>` footer when `--no-verify` is detected; `docs/audit/bypass_events.md` accumulates the audit trail. Five gates with paired §1.1 mutations: `CM-COVENANT-114-75-PROPAGATION`, `CM-GIT-HOOKS-INSTALL-SCRIPT`, `CM-GIT-HOOKS-SOURCE-DIR`, `CM-COMMIT-ALL-SIBLING-CHECK`, `CM-CI-WORKFLOW-PRESENT`. No escape hatch — no `--skip-hooks`, `--bypass-enforcement`, `--allow-orphan-md`, `--ci-not-applicable`, `--mechanical-enforcement-not-needed` flag.

**Cascade requirement:** This anchor (verbatim or by `§11.4.75` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-75-PROPAGATION`; paired mutation strips the literal → gate FAILs. Severity-equivalent to a §11.4 PASS-bluff at the enforcement layer.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.75 for the full mandate.


## §11.4.76 — Containers-Submodule Mandate (cascaded from constitution submodule §11.4.76)

> Verbatim user mandate (2026-05-20): *"For any work or requirements of running services or codebase inside the Containers (Docker / Podman / Qemy / Emulators, and so on) we MUST USE / INCORPORATE the Containers Submodule properly: https://github.com/vasic-digital/containers (git@github.com:vasic-digital/containers.git). Containers Submodule contains all means for us to Containerize our code and services! If any feature or Containing System is missing or not supported we MUST EXTEND IT properly like we do all of our projects! No bluff work is allowed of any kind!"*

For ANY containerized workload (Docker / Podman / Qemu / Kubernetes / container-backed emulators), every consuming project MUST: (1) install `vasic-digital/containers` (`digital.vasic.containers`) as a Git submodule; (2) consume via `replace` directive during development + pinned commit SHAs in production; (3) boot infra on-demand via `pkg/boot` + `pkg/compose` + `pkg/health` so operators are never required to start `podman machine` / `docker compose up` manually — the boot is part of the test entry point (the on-demand-infra invariant); (4) extend the Submodule (PR upstream) for missing runtimes / lifecycle primitives — never reimplement in-project (per §11.4.74); (5) anti-bluff: integration tests claiming to exercise containerized components MUST actually boot them via the Submodule — short-circuit fakes that bypass boot are a §11.4 violation. Tracker rows touching containerization MUST record `Catalogue-Check: extend vasic-digital/containers@<sha>` (or `reuse`). Planned gate `CM-CONTAINERS-USED` scans container-touching PRs for `digital.vasic.containers/...` imports; paired mutation strips the import + asserts FAIL.

**Cascade requirement:** This anchor (verbatim or by `§11.4.76` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-76-PROPAGATION`; paired mutation strips the literal → gate FAILs.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.76 for the full mandate.


## §11.4.77 — Regeneration-Mechanism-Required Mandate (cascaded from constitution submodule §11.4.77)

> Verbatim user mandate (2026-05-20): *"We must be sure that after excluding anything from Git versioning we still have the mechanism which will out of the box obtain or re-generate missing content!"*

Every `.gitignore` entry excluding (a) >~100 MiB OR (b) any artefact essential to building / running / testing the project MUST carry a documented + automated mechanism to either re-obtain (download from authoritative source: vendor tarball, SDK installer, npm/pip/cargo/go-mod/container registry, dedicated git submodule, S3/GCS) OR re-generate (run from tracked source via build pipeline, code-gen, asset render, captured-evidence replay, container build). Required artefacts per qualifying entry: (1) `.gitignore-meta/<entry-slug>.yaml` declaring pattern + mechanism-type + script-path + expected-disk-usage + vendor-url-or-source + integrity hash + requires-network + requires-credentials; (2) a non-interactive entry in `scripts/setup.sh` post-clone bootstrap; (3) a pre-build gate verifying regenerated content present OR a recent `.gitignore-meta/.regenerated/<slug>.ok` stamp; (4) README + `docs/guides/*.md` describing the mechanism + manual fallback + time/disk budget + §11.4.10 credentials. Bare `.gitignore` additions without the mechanism are a §11.4 PASS-bluff variant — codebase appears complete but a fresh clone cannot build/run. No escape hatch — no `--skip-regen-mechanism`, `--gitignore-is-enough`, `--operator-already-has-content` flag. Planned gate `CM-GITIGNORE-REGEN-MECHANISM` + paired §1.1 mutation (strip a required YAML key → gate FAILs).

**Cascade requirement:** This anchor (verbatim or by `§11.4.77` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-77-PROPAGATION`; paired mutation strips the literal → gate FAILs. Severity-equivalent to a §11.4 PASS-bluff at the repository-hygiene layer.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.77 for the full mandate.


## §11.4.78 — CodeGraph Code-Intelligence Mandate (cascaded from constitution submodule §11.4.78)

> Verbatim user mandate (2026-05-20): *"Make codegraph MANDATORY CHOICE for this purpose for all of our project ... All project which do not have configured and installed codegraph yet MUST DO IT and MUST USE IT!"*

Every consuming project worked on by AI coding agents MUST install, initialize, and use **CodeGraph** (`https://github.com/colbymchenry/codegraph`, npm `@colbymchenry/codegraph`) — a local SQLite semantic code-knowledge-graph exposed to agents over MCP (100% local, no cloud). (1) Install globally via npm with a user-writable npm prefix (no `sudo`). (2) `codegraph init` + `codegraph index`: `.codegraph/config.json` is tracked, `.codegraph/codegraph.db` is gitignored with `codegraph index` as its §11.4.77 regeneration mechanism; the `config.json` `exclude` list MUST exclude every credential/secret path per §11.4.10. (3) Wire `codegraph serve --mcp` into every CLI agent (Claude Code `.mcp.json`, OpenCode `opencode.json`, Qwen Code `.qwen/settings.json`, Crush `.crush.json`, host-local otherwise) referencing the bare `codegraph` command on `PATH` (no hardcoded host path). (4) Cover the integration with an anti-bluff suite whose per-agent end-to-end layer uses an unforgeable challenge (a fact obtainable only by calling a CodeGraph MCP tool, e.g. index node count via `codegraph_status`); a genuinely un-drivable agent is a documented SKIP per §11.4.3, never a faked PASS. (5) Document in `docs/CODEGRAPH.md`, kept in sync per §11.4.12 / §11.4.65. CodeGraph is consumed as the published npm package (§11.4.74) — not a git submodule, adds no Git remote. Planned gate `CM-CODEGRAPH-WIRED` + paired §1.1 mutation (strip a secret-exclusion → gate FAILs).

**Cascade requirement:** This anchor (verbatim or by `§11.4.78` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-78-PROPAGATION`; paired mutation strips the literal → gate FAILs.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.78 for the full mandate.


## §11.4.79 — Own-Org Submodules MUST Be Included in the CodeGraph Index (cascaded from constitution submodule §11.4.79)

> Verbatim user mandate (2026-05-21): *"All Submodules we use in the project and that are part of organizations to which we have the full access via GitHub, GitLab and other CLIs MUST BE included into the codegraph database and initialized / scanned / synced!"*

Refines §11.4.78's exclude-list with a per-submodule-ownership split: (a) own-org submodules (full write access via the project's CLIs — canonical orgs `vasic-digital` + `HelixDevelopment`) MUST be INCLUDED in the index; (b) third-party submodules (the §11.4.74 `no-match → vendor` path) MUST be EXCLUDED. Operational steps: (1) `git submodule update --remote --merge` to pull latest before re-indexing, respecting load-bearing pins on third-party submodules; (2) adjust `.codegraph/config.json` exclude list to keep own-org paths in scope; (3) re-index via `scripts/codegraph_setup.sh`; (4) verify via `scripts/codegraph_validate.sh` with ≥1 probe resolving a symbol living ONLY inside an own-org submodule; (5) paired §1.1 mutation — temporarily add the own-org submodule to exclude → validate MUST FAIL on the cross-submodule probe → restore. An index that lies about reachable symbols is a PASS-bluff against AI agents. Own-org submodules silently excluded without an audit trail in `.codegraph/config.json` comments is a release blocker.

**Cascade requirement:** This anchor (verbatim or by `§11.4.79` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-79-PROPAGATION`; paired mutation strips the literal → gate FAILs.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.79 for the full mandate.


## §11.4.80 — CodeGraph Regular-Update + Sync Automation Mandate (cascaded from constitution submodule §11.4.80)

> Verbatim user mandate (2026-05-21): *"We MUST regularly check for the updates and execute codegraph npm updates so the latest version of it is always installed on the host machine! ... Make sure we have proper full automation bash scripts which will run regularly and that these are part of the constitution Submodule ... Make sure all updates, sync processes we do and important codegraph related events are all documented under docs/codegraph in Status and Status_Summary documents ... and regularly export them like all other Status docs into the PDF and HTML!"*

Three deliverables (all living in the constitution submodule, inherited by reference per §3 — consuming projects invoke at `${CONST_DIR}/scripts/codegraph_*.sh`, never copy): (1) `scripts/codegraph_update.sh` — npm-installs latest `@colbymchenry/codegraph` after a registry version check; appends old/new version to `docs/codegraph/Status.md`; anti-bluff verifies `codegraph --version` reflects the new version after install (npm exit 0 ≠ working binary). (2) `scripts/codegraph_sync.sh` — after a successful update runs `codegraph status` → `codegraph sync .` → `codegraph status` → the project's `scripts/codegraph_validate.sh`; appends every step's output to BOTH the project's and the constitution's `docs/codegraph/Status.md`. (3) `docs/codegraph/Status.md` + `Status_Summary.md` append-only ledgers, exported to `.html` + `.pdf` per §11.4.65. Cadence: weekly floor (per §11.4.45). A consuming project that has not run `codegraph_update.sh` in >2 weeks AND has open AI-agent work is a release blocker. Paired §1.1 mutation: downgrade installed version → script detects drift → restore.

**Cascade requirement:** This anchor (verbatim or by `§11.4.80` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-80-PROPAGATION`; paired mutation strips the literal → gate FAILs.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.80 for the full mandate.


## §11.4.81 — Cross-Platform-Parity Mandate (cascaded from constitution submodule §11.4.81)

> Verbatim user mandate (2026-05-21): *"Any Linux-only blocker / issue we have MUST BE created macOS and other supported platforms equivalent! So, depending on platform proper implementation will be used for particular OS! EVERYTHING MUST BE PROPERLY EXTENDED AND UPDATED!"*

Every consuming project whose supported-platforms manifest lists more than one OS MUST, for every feature/test/gate/challenge/mutation depending on platform-specific primitives, ship a per-OS-equivalent implementation chosen at runtime via `uname -s` (or equivalent detection). Three sub-mandates: **(A) Per-OS implementation REQUIRED** — Linux cgroup/systemd/`/proc` primitives MUST have documented per-OS equivalents (POSIX `setrlimit`/`ulimit`, macOS `launchd`, BSD `rctl`, Windows Job Object) chosen via runtime dispatch. **(B) Per-OS tests REQUIRED** — every platform-dependent gate test MUST have `case "$(uname -s)" in` branches with positive captured evidence per §11.4.2 + §11.4.5 in each branch; SKIP-with-reason acceptable ONLY when the platform genuinely cannot enforce the invariant. **(C) Honest kernel-gap citation + adjacent equivalent test REQUIRED** — where a Linux primitive has NO equivalent due to a documented kernel limitation (canonical: XNU does not enforce `RLIMIT_AS` for unprivileged processes), the test MUST detect the gap at runtime, SKIP with exact kernel reason + reproducer + honest-gap-doc link, AND provide an ADJACENT test exercising the closest invariant the platform CAN enforce (e.g. `RLIMIT_CPU`+`SIGXCPU` as the macOS proxy), itself anti-bluff with a paired §1.1 mutation. Gate `CM-CROSS-PLATFORM-PARITY` scans for `case "$(uname -s)"` blocks asserting a non-SKIP branch (or honest-gap citation) per platform in the manifest; paired mutation strips a Darwin branch → gate FAILs. No escape hatch.

**Cascade requirement:** This anchor (verbatim or by `§11.4.81` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-81-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker on multi-platform projects.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.81 for the full mandate.


## §11.4.82 — Iteration-Speedup Discipline Mandate (cascaded from constitution submodule §11.4.82)

> Verbatim user mandate (2026-05-22): *"How can we speed-up this whole development and fixing process? ... Do not forget to all speed optimizations critical rules and mandatory constraints MUST BE all added into our root (constitution Submodule) Constitution.md, CLAUDE.md, AGENTS.md and QWEN.md and all other relevant constitution Submodules files!"*

Iteration cycle time is a first-order quality enabler. Every consuming project's build / test / commit / debug pipeline MUST adopt these speedup disciplines AS MANDATORY (each independently enforceable): (A) Phase-1 forensic (`superpowers:systematic-debugging`) before any speculative source patch — speculative patches without FACT-grade root cause are §11.4.6 + §11.4.82 violations; (B) Live-ADB-First (or live-equivalent) before any rebuild — strengthens §11.4.51 to a release-blocker mandate; (C) 30-second pre-flight before launching rebuild orchestrators (device/sink reachability, host memory/disk, no stale locks, no orphan processes); (D) persistent build caches outside containers (`ccache`/`sccache`/Gradle daemon bind-mounted to host); (E) module-only rebuild for loadable-module-only changes; (F) parallel multi-device testing with separate `qa-results/<TS>/<device-tag>/` outputs; (G) subagent scope discipline + worktree isolation (≤30 min budget, single-responsibility, `isolation: "worktree"` default); (H) lock-file + stale-process hygiene (clean `.git/index.lock`, disable auto git-gc in concurrent repos); (I) cycle telemetry per §11.4.24 (commit hash, per-phase wall-clock, speedup-flag set, outcome — aggregated weekly). Gate `CM-ITERATION-SPEEDUP-DISCIPLINE` audits recent cycles for telemetry citing which of (A)-(I) applied; paired §1.1 mutation strips the speedup-flag column → gate FAILs. No escape hatch — no `--skip-phase1-forensic`, `--no-pre-flight`, `--rebuild-everything-always`, `--unlimited-subagent-scope`, `--ignore-locks`, `--no-telemetry` flag.

**Cascade requirement:** This anchor (verbatim or by `§11.4.82` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-82-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.82 for the full mandate.


## §11.4.83 — docs/qa/ End-User Evidence Mandate (cascaded from constitution submodule §11.4.83)

> Verbatim user mandate (2026-05-22): *"every feature that ships MUST carry a recorded e2e communication transcript + any attached materials under `docs/qa/<run-id>/` (per-feature subdirectories). A feature with no QA transcript is itself a §107 PASS-bluff — it claims to work but has no auditable runtime evidence. Bot-driven automation MUST preserve full bidirectional communication threads as proof."*

Every feature that ships MUST carry a recorded end-to-end communication transcript plus any attached materials (screenshots, request/response payloads, audio, file uploads) committed under `docs/qa/<run-id>/` — one directory per feature run. Operative rule: (1) every consuming project MUST maintain a `docs/qa/` tree, each new feature under `docs/qa/<run-id>/` where `<run-id>` is monotonic + greppable (timestamp / ATM-NNN / other workable-item ID per §11.4.54); (2) transcripts MUST be full bidirectional — every prompt/command sent + every response received (one-sided is not a transcript); (3) attached materials MUST be committed in-repo (no external-only links — that is a §11.4.13 sink-side violation); (4) bot-driven / agent-driven QA automation MUST preserve the full conversation thread as the proof artefact; (5) release gates MUST refuse to tag a version that has any feature-shipping commit without its matching `docs/qa/<run-id>/` directory. A feature with no QA transcript is a §11.4 / §107 PASS-bluff. Composes with §11.4.2 / §11.4.5 / §11.4.13 / §11.4.65 / §11.4.69 / §1.1.

**Cascade requirement:** This anchor (verbatim or by `§11.4.83` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-83-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker — no `--qa-evidence-optional` escape hatch.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.83 for the full mandate.


## §11.4.84 — Working-Tree Quiescence Rule for Subagent Commits (cascaded from constitution submodule §11.4.84)

> Verbatim user mandate (2026-05-22): *"no subagent commit may proceed while any concurrent mutation gate is in flight in the same checkout. Before `git add`, the committing agent MUST `grep` its own working tree for mutation markers (`MUTATED for paired`, `// always pass`, `return json.Marshal` shortcut paths, etc.). Any unexplained file in the staging area triggers ABORT."*

No subagent (or main-thread) commit may proceed while any concurrent mutation gate, paired-mutation experiment, or other in-flight mutation is live in the same checkout. Before `git add`, the committing agent MUST grep its own working tree for mutation markers (`MUTATED for paired`, `// always pass`, `return json.Marshal` shortcut paths, `// MUTATION` / `# MUTATION` annotations, `_mutated_*` filename suffixes, etc.) and explicitly account for every modified file in the staging area; any unexplained file → ABORT. (Forensic case: a logo-fix subagent's `git add` swept an `// always pass` JWT-verify mutation residue into an unrelated commit pushed to all four mirrors — a real security-defect window.) Operative rule: (1) pre-`git add` greps for mutation markers + cross-checks `git status --porcelain` against the subagent's declared scope; unaccounted entries → ABORT; (2) any active mutation gate MUST be serialised (mutate → assert FAIL → restore → assert PASS) and the working tree verifiably clean before any unrelated commit; (3) concurrent subagents in the SAME checkout MUST coordinate through a lockfile (`.git/MUTATION_IN_PROGRESS`) — cleaner solution is `git worktree add` per subagent (composes with §11.4.20/§11.4.70); (4) post-commit `mutation-residue-scanner` MUST run before push — any commit containing a mutation marker → push BLOCKED.

**Cascade requirement:** This anchor (verbatim or by `§11.4.84` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-84-PROPAGATION`; paired mutation strips the literal → gate FAILs. A mutation marker that lands in a tagged commit is a critical defect regardless of how briefly it persisted.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.84 for the full mandate.


## §11.4.85 — Stress + Chaos Test Mandate (cascaded from constitution submodule §11.4.85)

> Verbatim user mandate (2026-05-24): *"Every fix or improvement you do MUST BE covered with full automation stress and chaos tests so we are sure nothing can break the functionality and all edge cases are monitored and polished and additionally fixed if that is needed! Everything must produce rock solid proofs and follow fully no-bluff policy!"*

Every fix or improvement landed MUST ship with full-automation **stress** AND **chaos** test suites exercising edge cases, sustained load, concurrent contention, and failure-injection. Happy-path coverage alone is a §11.4 / §107 PASS-bluff at the resilience layer. **Stress** (closed-set): sustained load (N ≥ 100 iterations OR ≥ 30 s wall-clock, p50/p95/p99 latency recorded) + concurrent contention (N ≥ 10 parallel invocations, no deadlock/leak) + boundary conditions (empty/max/off-by-one, each categorised). **Chaos** (closed-set, per fix-class appropriateness): process-death injection + network-fault injection (drop/delay/reorder) + input-corruption injection + resource-exhaustion injection (disk full, OOM, FD exhaustion — refuse cleanly OR degrade, NEVER crash) + state-corruption injection (mid-flight lock loss, partial-write). Every stress + chaos PASS MUST cite a captured-evidence artefact path per §11.4.5 + §11.4.69. Helper library `stress_chaos.sh` provides `ab_stress_run`, `ab_stress_concurrent`, `ab_chaos_kill_pid_during`, `ab_chaos_drop_network_during`, `ab_chaos_corrupt_file_during`, `ab_chaos_oom_pressure_during`, `ab_chaos_disk_full_during`, each composing with `ab_pass_with_evidence` / `ab_skip_with_reason`. Cleanup non-negotiable in `trap '...' EXIT` (cleanup failure = §11.4.14 violation). Four-layer coverage per §11.4.4(b) + paired §1.1 mutation (strip chaos-injection or evidence-capture → gate FAILs). No escape hatch — no `--skip-stress`, `--no-chaos`, `--happy-path-suffices`, `--stress-test-later` flag.

**Cascade requirement:** This anchor (verbatim or by `§11.4.85` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-85-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.85 for the full mandate.


## §11.4.86 — Roster/Corpus-Backed Status-Doc Auto-Sync Mandate (cascaded from constitution submodule §11.4.86)

> Verbatim user mandate (2026-05-25): *"Make sure that assets and players Status docs are ALWAYS regularly updated and in sync like all others Status docs — any time we add or modify the assets content(s) or we change or add new / remove existing pre-installed video and audio player apps! This MUST WORK OUT OF THE BOX!"*

Some Status docs (§11.4.45) are backed by a tracked roster (installed apps/components) or a tracked asset corpus (test/media asset directory) rather than narrative alone. Their freshness MUST NOT depend on operator vigilance — the moment a roster/corpus member changes (app added/removed/renamed; asset added/modified/removed) the Status doc + Status_Summary + HTML + PDF MUST resync out of the box, mechanically. Mechanism (all must hold): (1) drift-proof fingerprint — sha256 of the sorted member list (NOT mtime), persisted in a sidecar beside the Status doc; (2) a sync helper that regenerates the fingerprint + re-exports HTML+PDF via the §11.4.65 exporter, wired so sync is automatic; (3) a pre-build gate that FAILs when the live fingerprint differs from the persisted one (mirrors §11.4.12 `CM-ISSUES-SUMMARY-SYNC` + §11.4.45 `sync_integration_status`); (4) a paired §1.1 mutation corrupting the fingerprint and asserting the gate FAILs. Classification: universal — the consuming project supplies the specific docs, roster/corpus sources, helper, and gate name per §11.4.35.

**Cascade requirement:** This anchor (verbatim or by `§11.4.86` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-86-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker — no `--skip-roster-sync`, `--allow-status-drift`, `--roster-sync-not-applicable` flag.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.86 for the full mandate.


## §11.4.87 — Endless-Loop Autonomous Work + Zero-Idle Agent Dispatch + Anti-Bluff Testing Mandate (cascaded from constitution submodule §11.4.87)

> Verbatim user mandate (2026-05-26): *"continue in endless loop fully autonomously"* (and any semantically-equivalent phrasing).

When the operator instructs an AI agent to continue in an endless autonomous loop, the agent MUST treat it as a HARD-CONTRACT covenant: (A) continue working until `docs/Issues.md` Status-column has zero non-terminal entries AND `docs/CONTINUATION.md` §3 Active work is empty AND no background subagent is mid-execution AND no external dependency is in-flight; (B) dispatch background subagents for parallelisable work — main + every subagent operate concurrently, "waiting for results" is the ONLY acceptable idle reason; (C) every closure lands four-layer test coverage per §11.4.4(b) with captured-evidence (audio/video/network/UI/sysfs physical proofs); (D) the §11.4 anti-bluff covenant family (§11.4.1 / §11.4.2 / §11.4.6 / §11.4.7 / §11.4.27 / §11.4.50 / §11.4.52 / §11.4.68 / §11.4.69 / §11.4.83) is the operative truth-discipline — tests AND HelixQA Challenges bound equally; (E) the loop terminates ONLY on all-conditions-met, explicit operator STOP, host-session-safety demand, or scheduled wake on a known-future-actionable signal. No escape hatch — no `--idle-OK`, `--skip-endless-loop`, `--bluff-permitted-for-this-task`, `--metadata-only-test-suffices`, `--no-physical-proof-required` flag.

**Cascade requirement:** This anchor (verbatim or by `§11.4.87` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-87-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.87 for the full mandate.


## §11.4.88 — Background-Push Mandate: Commit-Lock Release Immediately After Commit, Push Runs Detached (cascaded from constitution submodule §11.4.88)

Forensic anchor (2026-05-26): a single `commit_all.sh` held its flock ~5 hours because `do_push` ran synchronously after the commit landed — every subsequent commit blocked on a slow mirror push irrelevant to the local commit's durability. Implementation seam for §11.4.87(B) zero-idle. The mandate: (A) `.git/.commit_all.lock` MUST be released IMMEDIATELY after `git commit` returns 0 — the commit is durable on local disk regardless of remote push outcome; (B) push runs detached via `nohup ./push_all.sh ... > <log> 2>&1 &` + `disown` — the orchestrator's exit code reports COMMIT success, NOT push success; (C) `push_all.sh` acquires per-remote flock `.git/.push.<remote>.lock` so concurrent invocations targeting the same remote serialize but different-remote invocations run in parallel; (D) backgrounded push failures land in `qa-results/push_failures/<ts>_<remote>.log` — the next autonomous-loop tick checks per §11.4.87(A) "no external dependency in-flight" gate; (E) synchronous-push escape: explicit `--sync-push` CLI flag preserves legacy behaviour for §11.4.41 force-push merge-first audit paths. Gates `CM-COVENANT-114-88-PROPAGATION` + `CM-BACKGROUND-PUSH-WIRED` + paired §1.1 mutations. Synchronous push (without `--sync-push`) = §11.4 PASS-bluff at the execution layer.

**Cascade requirement:** This anchor (verbatim or by `§11.4.88` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-88-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker — no escape hatch beyond `--sync-push` for force-push events.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.88 for the full mandate.


## §11.4.89 — Background Test Execution Mandate (cascaded from constitution submodule §11.4.89)

> Verbatim user mandate (2026-05-27): *"Any tests we are executing, especially long test cycles, MUST BE performed in background in parallel with main work stream! This MUST NOT block our capabilities to work on queued workable items. Main work stream can be blocked or sit iddle only if absolutely needed and if it depends hard on results of some background execution."*

Symmetric anchor to §11.4.88 (background push) at the test-execution layer. Mandate: (A) long-running tests (>30 s expected: `pre_build`, `meta_test`, `test_all_fixes`, `recent_work_validate`, HelixQA banks, 4-phase cycles, full-suite retests, audio supervisors, dual-display recorders) MUST run via `nohup ... > <log> 2>&1 &` + `disown` with the log under a known dir (`qa-results/<test_id>_<ts>.log`); (B) the main stream proceeds to the §11.4.42 priority queue immediately; (C) hard-dependency gating — poll an exit-status file or `pgrep -af <test>` before steps that need the exit code, surfacing as §11.4.66 interactive options if the test is still running; (D) failures land in `<log>` files, the next loop tick checks; (E) foreground execution permitted ONLY for <30 s tests OR explicit operator authorisation; (F) per-script flock serialises same-script invocations, different-script invocations parallel. Gates `CM-COVENANT-114-89-PROPAGATION` + `CM-BACKGROUND-TEST-EXECUTION-WIRED` + paired §1.1 mutations.

**Cascade requirement:** This anchor (verbatim or by `§11.4.89` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-89-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker — no escape hatch beyond explicit per-invocation operator authorisation.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.89 for the full mandate.


## §11.4.90 — Obsolete Status + Per-Item Obsolescence Audit (cascaded from constitution submodule §11.4.90)

> Verbatim user mandate (2026-05-27): *"Bug No 6 ... seems obsolete after latest request for new behavior ... mark obsolete tickets with some light gray background ... text - the description to be strikethrough styled ... review all existing open or resolved workable items if they are obsolete - not valid any more ... There MUST NOT be any mistake! No bluff is allowed of any kind!"*

The §11.4.15 Status closed-set is extended with a terminal `Obsolete (→ Fixed.md)` value (orthogonal to Type per §11.4.16). Obsolescence reasons (closed vocabulary): `superseded-by-design-change | superseded-by-later-mandate | feature-removed | duplicate-of | unsupported-topology`. Every Obsolete heading MUST carry an `**Obsolete-Details:**` line (Since + Reason + Superseding-item + Triple-check evidence) within 8 non-blank lines. The §11.4.23 colorizer adds a `cell-status-obsolete` class — light-gray `#E0E0E0` background + strikethrough description. Audit cadence: every release-gate sweep per §11.4.40 + §11.4.42; triple-check is non-negotiable per the operator mandate. Composes with §11.4.15 / §11.4.16 / §11.4.19 / §11.4.21 / §11.4.23 / §11.4.33 / §11.4.34 / §11.4.40 / §11.4.42 / §11.4.66 / §11.4.71. Gates `CM-COVENANT-114-90-PROPAGATION` + `CM-ITEM-OBSOLETE-DETAILS` + `CM-OBSOLETE-COLORIZER-WIRED` + paired §1.1 mutations.

**Cascade requirement:** This anchor (verbatim or by `§11.4.90` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-90-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.90 for the full mandate.


## §11.4.91 — Summary-Doc Clarity Mandate (cascaded from constitution submodule §11.4.91)

> Verbatim user mandate (2026-05-27): *"Summary docs - Issues_Summary some not clear one line descriptions - like 'Composes with' ... For each workable item we MUST HAVE clearly understandable meaning ... every team member can clearly understand what that particular workable item is exactly about! There cannot be misunderstanding or unclearity of any kind and no bluff allowed!"*

Every summary entry (Issues_Summary, Fixed_Summary, README doc-link, Status_Summary pages 1+2, all one-liners) MUST contain a self-contained meaningful description ≥ 6 words OR ≥ 40 chars naming SUBJECT + PROBLEM/GOAL. Forbidden one-liner anti-patterns: section labels (`Composes with`, `Closure criteria`, `Fix direction`, etc.); bare metadata fragments (`Critical`, `Bug`, `In progress`, etc.); section-marker echoes; a §-letter alone. Generators (`generate_issues_summary.sh` / `generate_fixed_summary.sh` / `update_readme_doc_links.sh` / `generate_status_summary.sh`) MUST extract from the H1/H2 heading line per the §11.4.54 ATM-NNN convention, NEVER from arbitrary downstream text, and MUST refuse anti-pattern rows — emitting a `(MISSING DESCRIPTION — fix source heading)` placeholder with visual highlight. Gate `CM-SUMMARY-CLARITY-DESCRIPTIONS` scans every summary; an anti-pattern match = FAIL. Audit cadence: every §11.4.40 + §11.4.42 sweep.

**Cascade requirement:** This anchor (verbatim or by `§11.4.91` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-91-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.91 for the full mandate.


## §11.4.92 — Multi-Pass Change-Evaluation Discipline (cascaded from constitution submodule §11.4.92)

> Verbatim user mandate (2026-05-27): *"Every change to the project or codebase we do MUST BE evaluated in several passes and in in-depth analisys for potential new issues or problems it can introduce! ... no bluff of any kind! After we do change or set of changes this mandatory steps MUST BE taken!"*

Every non-trivial change MUST pass a 5-pass evaluation BEFORE it is commit-ready: **(Pass 1)** main-task verification — change achieves the stated goal, captured-evidence per §11.4.5/§11.4.69; **(Pass 2)** regression-blast-radius analysis — enumerate every direct dependency, demonstrate no contract break; **(Pass 3)** cross-feature interaction analysis — audit parallel features sharing state/timing/hardware/shell environment; **(Pass 4)** deep-research validation per §11.4.8 — external precedent OR "NO external solution found — original work" + CodeGraph queries per §11.4.78/§11.4.79; **(Pass 5)** anti-bluff confirmation per §11.4 / §11.4.1 / §11.4.6 / §11.4.27 / §11.4.50 / §11.4.52 / §11.4.69 / §11.4.83 — no new bluff surface introduced. Each pass is documented (commit footers OR `docs/` entries OR `qa-results/` evidence). Only after all 5 passes complete may commit/push/test/release proceed. Trivial exemption: typo / revision-bump / MD-export-regen IF zero source touched AND the commit message cites the exemption explicitly. Gates `CM-COVENANT-114-92-PROPAGATION` + `CM-MULTI-PASS-EVALUATION-EVIDENCE` + paired §1.1 mutations.

**Cascade requirement:** This anchor (verbatim or by `§11.4.92` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-92-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.92 for the full mandate.


## §11.4.93 — SQLite-Backed Single-Source-of-Truth for Workable Items (cascaded from constitution submodule §11.4.93)

> Verbatim user mandate (2026-05-27): *"There MUST be single source of truth for all of our workable items - SQlite database ... proper scripts (we recommend Go programs) ... reduce a chance for sync to be broken ... generate always all docs from DB or to re-generate Db from all docs we have in opposite direction"*

The text-based Issues/Fixed/Summary/CONTINUATION constellation is converted to a SQLite-DB-backed single source of truth. Schema mandatory tables: `items` (atm_id PK + Type + Status incl. Obsolete + Severity + title + description ≥40 chars + created/modified + composes_with JSON + current_location); `item_history` (append-only audit per §11.4.34 By/Reason/Evidence); `obsolete_details` (§11.4.90); `operator_block_details` (§11.4.21); `firebase_metadata` (§11.4.47); `meta` (schema version + last sync + integrity hash). A Go binary at `cmd/workable-items/` provides `sync md-to-db` / `db-to-md` / `diff` / `validate` / `add` / `close`; bidirectional regen is byte-identical round-trip (closed-set whitespace/section-order tolerance). `commit_all.sh` refuses on non-empty diff; `sync_issues_docs.sh` invokes the Go binary; pre-build runs `workable-items validate`. Anti-bluff: unit + integration + stress (1000-row insert + 10 concurrent writers) + chaos (mid-write SIGKILL + corrupt-DB recovery + disk-full) + paired §1.1 mutation + HelixQA Challenge `CME-WORKABLE-ITEMS-001`. The Go binary lives in the constitution submodule (`constitution/scripts/workable-items/`) per §11.4.74. Gates `CM-COVENANT-114-93-PROPAGATION` + `CM-WORKABLE-ITEMS-DB-PRESENT` + `CM-WORKABLE-ITEMS-MD-DB-IN-SYNC` + paired §1.1 mutations. (NOTE: the DB tracking rule is AMENDED by §11.4.95 — DB is TRACKED, not gitignored.)

**Cascade requirement:** This anchor (verbatim or by `§11.4.93` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-93-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker — text-based-only trackers are a §11.4 PASS-bluff at the data-architecture layer.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.93 for the full mandate.


## §11.4.94 — Zero-Idle Priority-First Parallel-By-Default Operating Mode (cascaded from constitution submodule §11.4.94)

> Verbatim user mandate (2026-05-27): *"We MUST NEVER sit iddle / wait or sleep if there is possibility for us to work on something ... Always check if there is a possibility to work on something while we are not working actively on something! Pick always by priority - most critical workable items and other tasks MUST BE done first! ... Stay still / iddle if nothing is left to be done at all or waiting for something that is blocking us / you!!!"*

§11.4.94 binds §11.4.20 + §11.4.42 + §11.4.58 + §11.4.70 + §11.4.72 + §11.4.82 + §11.4.87 + §11.4.88 + §11.4.89 into a single always-on enforcement: (A) idle ONLY when every queued item is genuinely blocked on an external dependency (hardware / network upstream / build/test completion the conductor cannot accelerate) OR operator STOP OR §12 host-safety — "don't see what to do" is NEVER valid; (B) before ANY wake/sleep the conductor MUST survey parallel-work feasibility per §11.4.42 + §11.4.72 + §11.4.87, identify non-contending items, and dispatch in parallel per §11.4.20/§11.4.70 (subagent) + §11.4.58 (PWU disjoint scope) + §11.4.89 (background long tests); (C) priority order MANDATORY — pick highest-severity + §11.4.72 audio-first the conductor can autonomously progress; (D) subagent-driven default for non-trivial; (E) background default for >30 s wall-clock work via `nohup`+`disown`; (F) stability-preserving (composes with §11.4.92 multi-pass + §11.4.84 quiescence + §12.6–§12.9 host safety); (G) progress updates surfaced at milestone boundaries. Gates `CM-COVENANT-114-94-PROPAGATION` + `CM-PARALLEL-WORK-AUDIT` + paired §1.1 mutations.

**Cascade requirement:** This anchor (verbatim or by `§11.4.94` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-94-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.94 for the full mandate.


## §11.4.95 — Workable-Items SQLite DB Is TRACKED in Git, NEVER Gitignored (cascaded from constitution submodule §11.4.95)

> Verbatim user mandate (2026-05-27): *"We shall not Git ignore our workable items SQlite DB since it is our single source of truth ... workable items SQlite DB regularly commited and pushed to all upstreams!"*

§11.4.93's earlier "gitignored per §11.4.30" clause is AMENDED — the DB at `docs/workable_items.db` is TRACKED in git, NEVER gitignored. It IS authoritative source data, NOT a build artefact. Every `workable-items sync md-to-db` that mutates state MUST stage + commit + push the DB alongside the MD regen per §11.4.19 atomic-move + §2.1 multi-upstream push. A WAL-checkpoint (`PRAGMA wal_checkpoint(TRUNCATE)`) is required before commit-stage so the transient `.db-wal` + `.db-shm` sidecars (gitignored per §11.4.30) are safely discardable. The §11.4.77 regeneration mechanism does NOT apply — the DB IS the source. Destructive DB ops require §9.2 hardlinked-backup + operator authorization; §11.4.41 force-push merge-first applies if DB history ever needs rewrite. Gates `CM-COVENANT-114-95-PROPAGATION` + `CM-WORKABLE-ITEMS-DB-TRACKED` + paired §1.1 mutation.

**Cascade requirement:** This anchor (verbatim or by `§11.4.95` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-95-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.95 for the full mandate.


## §11.4.96 — Safe-Parallel-Work-With-Long-Build Catalogue + Mandate (cascaded from constitution submodule §11.4.96)

> Verbatim user mandate (2026-05-27): *"Are there except AOSP build process any other active jobs being done at the moment? Can we work on something in parallel while build is in progress so we slowly cleanup our slate? ... do as much as possible work in background in parallel with main work stream and oreferrably using subagents-driven approach!"*

An operational catalogue for the canonical long-running workload (multi-hour containerised build per §12.9). **SAFE during build:** (A) MD/docs work; (B) generator/helper script work under `scripts/`; (C) pre-build + meta-test gate authoring + paired §1.1 mutations; (D) on-device test scripts; (E) constitution submodule edits + push; (F) any submodule commit + push per §11.4.88; (G) read-only live-ADB probes (`dumpsys`/`getprop`/`cat /proc/...`/`screencap`/`logcat`); (H) subagent dispatch per §11.4.20/§11.4.70 + §11.4.84 quiescence; (I) web research + external API queries with §11.4.10 credentials; (J) workable-items DB ops per §11.4.93+§11.4.95; (K) backgrounded pre-build + meta-test execution per §11.4.89. **UNSAFE during build:** (α) `git checkout`/`reset --hard`/`clean -df` on the source tree (use `git worktree`); (β) mass file deletes/renames under built source trees; (γ) submodule pointer updates affecting built artefacts; (δ) `out/` mutations; (ε) `make clean`/`m clobber`/`rm -rf out/`; (ζ) container destruction; (η) disk-filling breaching §12.9 free-space minimum; (θ) §12 host-session-safety breaches. Conductor responsibility: before EVERY pause point during a long build, consult the catalogue, identify (A)-(K) queue items per §11.4.42+§11.4.72, and dispatch ≥1 per §11.4.20/§11.4.70 subagent default + §11.4.89 background. "Build running, nothing else to do" is NEVER true per §11.4.94+§11.4.96. Gates `CM-COVENANT-114-96-PROPAGATION` + `CM-PARALLEL-WORK-DURING-BUILD-AUDIT` + paired §1.1 mutations.

**Cascade requirement:** This anchor (verbatim or by `§11.4.96` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-96-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.96 for the full mandate.




## §11.4.97 — Maximum-Use-of-Idle-Time + Progress-Update Cadence (cascaded from constitution submodule §11.4.97)

> Verbatim user mandate (2026-05-27): *"keep it working, we should do as much as possible, if not it all but as much as we can as long as there is iddle time! it MUST be used! ... keep us updated about all progress and all phisycal proofs and gathered data as you progress through all open workable items!"*

Operating-mode capstone strengthening §11.4.87 + §11.4.94 + §11.4.96: (A) every minute of conductor idle time during which work could autonomously progress AND is not genuinely blocked = a §11.4.97 violation; "as much as possible, if not it all but as much as we can" is operative — dispatch CONTINUOUSLY through the entire idle window, not just at scheduled wakes; (B) progress-update cadence — emit an operator-facing 1-line update at every commit landed / subagent return / constitutional anchor / captured evidence / milestone closure, no operator prompt required; (C) continuous physical-proof gathering per §11.4.5 + §11.4.6 + §11.4.69 — every autonomous closure cites captured-evidence (evidence path goes into the §11.4.93 `item_history.evidence_path` when the DB lands); (D) composes with §11.4.5/6/13/20/27/42/50/52/69/70/72/83/85/87/88/89/94/96; (E) the idle-only-when-blocked closed-set is unchanged from §11.4.94(A). Gates `CM-COVENANT-114-97-PROPAGATION` + `CM-IDLE-TIME-AUDIT` + paired §1.1 mutations.

**Cascade requirement:** This anchor (verbatim or by `§11.4.97` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Propagation gate `CM-COVENANT-114-97-PROPAGATION`; paired mutation strips the literal → gate FAILs. Release blocker.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.97 for the full mandate.
