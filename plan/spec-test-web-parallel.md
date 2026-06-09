# Spec: test-web-parallel

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/8 |
| Updated | 2026-06-09 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/test/cli/cmd_web.go` - current sequential web loop + single shared ze daemon
4. `internal/component/web/testing/runner.go` - `Browser`, `runWBTestCase`, agent-browser integration
5. `internal/test/cli/cmd_editor.go:150-200` - migration template: `ParallelRunner` with `TestSet`
6. `internal/test/runner/parallel.go` - `ParallelRunner[T]` (already exists, configurable concurrency)
7. `internal/test/runner/ports.go` - `ReservePorts`, `FindFreePortRange` (already exists)
8. `.claude/rules/agent-browser.md` - agent-browser CLI usage

## Task

Make web (`.wb`) tests run **in parallel** with full per-test isolation. Today `internal/test/cli/cmd_web.go` runs 72 `.wb` files **sequentially** against **one shared ze daemon** and **one global browser** (`close --all`). Tests that mutate config (`set … commit`) would corrupt each other if overlapped, and the global browser teardown kills all sessions.

**No external dependencies.** All required infrastructure already exists:
- `runner.ParallelRunner[T]` (`internal/test/runner/parallel.go`) with configurable concurrency, display, timing baselines, failure reporting.
- `runner.TestSet[T]` + `runner.BaseTest` (`internal/test/runner/base.go`) for test discovery and selection.
- `runner.ReservePorts` / `runner.FindFreePortRange` (`internal/test/runner/ports.go`) for per-test port isolation.
- The editor test command (`internal/test/cli/cmd_editor.go:150-200`) demonstrates the exact migration pattern: `NewParallelRunner` -> `AddTestWithNick` per test -> `SetOnFail` -> `pr.Run(ctx)`.

**Two isolation changes unlock parallel web:**

| Singleton today | Per-test replacement |
|-----------------|----------------------|
| One ze daemon (shared port + tmpdir, `zeTestStartWebServer` at L280) | Per-test ze daemon: own port (via `runner.ReservePorts`) + own `os.MkdirTemp` config dir |
| One global browser, `close --all` (`runner.go:239`, `runner.go:407`) | Per-test `agent-browser --session <nick>` (or `AGENT_BROWSER_SESSION=<nick>` env): isolated cookies/tabs/refs. `close` (own session) replaces `close --all` |

**Also:** migrate the web command from its bespoke sequential loop (L191-239) to `ParallelRunner[*zeTestWebTest]`, and apply a **web-specific concurrency cap** (~4, since each test costs one Chrome + one ze daemon). The editor command at `cmd_editor.go:150-200` is the direct template.

**Out of scope:** per-assertion trace output; changing `.wb` file format or assertions; stress-mode (`RunWithCount`) for web.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as -> Decision: / -> Constraint: annotations. -->
- [ ] `internal/component/web/testing/runner.go` - `Browser`, `runWBTestCase`, `runAgent*`, `agentEnv`
  -> Constraint: `Browser` has NO session field. `runAgent`/`runAgentOutput`/`runAgentWithEnv` all call `agentEnv()` which builds env from `os.Environ()` globally. `closeBrowser()` does `close --all`. `ensureInitScript` uses a package-level `sync.Once` (safe for concurrent use, writes once). Per-session isolation requires: (a) threading a session name through every `runAgent` call as `AGENT_BROWSER_SESSION=<nick>` in the env, and (b) replacing `close --all` with `close` (own session only).
- [ ] `internal/test/cli/cmd_web.go` - daemon + loop
  -> Constraint: one `zeTestStartWebServer(zeBin, listenAddr)` for all tests (L280-299); one free port (L140-153); sequential loop (L191-239); custom output formatting (not using Display/ParallelRunner). `zeTestWebTest` embeds `runner.BaseTest` + `Path string`. `zeTestWebServer` holds `cmd *exec.Cmd` + `tempDir string` with `stop()` for cleanup. `time.Sleep(3 * time.Second)` for daemon readiness (replace with port-probe loop).
- [ ] `internal/test/runner/parallel.go` - `ParallelRunner[T]`
  -> Constraint: already provides configurable concurrency (`SetConcurrency`), display, timing baselines, failure callbacks (`SetOnFail`, `SetOnReport`), semaphore-based scheduling. No changes needed to this file.
- [ ] `internal/test/runner/ports.go` - port allocation
  -> Constraint: `ReservePorts(base, count)` returns `*PortReservation` with advisory flock-based locks. `FindFreePortRange(base, count)` finds N consecutive free ports. Use `ReservePorts` per test to avoid port collisions between concurrent daemons.
- [ ] `internal/test/cli/cmd_editor.go:150-200` - migration template
  -> Constraint: exact pattern to follow: `NewParallelRunner[*EditorTest](colors)` -> `SetLabel/SetQuiet/SetVerbose/SetBaseDir` -> loop `AddTestWithNick` with run function -> `SetOnFail` callback -> `pr.Run(ctx)`. Web migration follows this but adds per-test daemon lifecycle inside the run function.
- [ ] `.claude/rules/agent-browser.md` + `agent-browser skills get core --full`
  -> Constraint: `--session <name>` (or `AGENT_BROWSER_SESSION`) = isolated browser context. "Run multiple browsers in parallel" is a documented, supported workflow. Each session has its own cookies, tabs, and refs.

### RFC Summaries
- N/A - test harness change, not a wire protocol.

**Key insights:**
- agent-browser supports parallel isolated sessions natively (`--session <name>`). The browser layer is not a blocker.
- `ParallelRunner[T]` and all supporting infrastructure already exist. No new framework code needed.
- The editor command is the exact migration template. Web adds per-test daemon lifecycle on top.
- The real cost is per-test ze daemon startup (one daemon + one Chrome per concurrent test); a low web concurrency cap (~4) bounds resource use.
- `ensureInitScript()` uses `sync.Once` and is safe for concurrent calls. The init script file is shared read-only.
- `agentEnv()` builds env from `os.Environ()` each call, so adding `AGENT_BROWSER_SESSION` per-test is safe (no global mutation).
- VALIDATION: confirm one agent-browser daemon cleanly serves N concurrent `--session` browsers, and that `close` (not `close --all`) only closes the caller's session.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/cli/cmd_web.go` - sequential loop (L191-239), shared daemon (L280-299), custom output
  -> Constraint: migration target; replace bespoke loop with `ParallelRunner[*zeTestWebTest]` + per-test daemon adapter. Keep `zeTestWebTest` struct (embeds `BaseTest` + `Path`), selection, and CLI flags.
- [ ] `internal/component/web/testing/runner.go` - `Browser` + agent-browser glue
  -> Constraint: must become session-scoped. `Browser` needs a `session` field. All `runAgent*` functions need a session-aware env variant. `Close()` must use `close` (own session), not `close --all`. `agentEnv()` needs a session parameter or the session must be set via env before calling.
- [ ] `internal/test/runner/parallel.go` - `ParallelRunner[T]` (no changes needed)
  -> Constraint: use as-is. Provides everything: concurrency, display, timing, failure callbacks.
- [ ] `internal/test/runner/ports.go` - port allocation (no changes needed)
  -> Constraint: use `ReservePorts` per test. Each reservation holds flock until released.

**Behavior to preserve:**
- Same `.wb` assertions and pass/fail outcomes.
- Same skip-when-agent-browser-absent behavior (`exec.LookPath("agent-browser")`).
- Same CLI flags: `-a`, `-p`, `-v`, `-q`, `-l`, `--start`, `--port`, positional test IDs.
- Same test discovery from `test/web/*.wb` using `filepath.WalkDir`.
- `RunWBFile(path, baseURL)` interface unchanged (called per test with test-specific baseURL).

**Behavior to change:**
- Web tests run concurrently (bounded by web cap ~4) instead of sequentially.
- Each test gets its own ze daemon (own port + own tmpdir) instead of sharing one.
- Each test gets its own browser session (`--session <nick>`) instead of a global browser.
- Output flows through `ParallelRunner`'s `Display` (progress, timing, failure groups) instead of custom formatting.
- Daemon readiness checked by port probe instead of `time.Sleep(3 * time.Second)`.

## Data Flow (MANDATORY)

### Entry Point
- `ze-test web -a` -> `cmdWebMain` -> discover `.wb` files -> select -> `ParallelRunner[*zeTestWebTest]`

### Transformation Path
1. Discover `.wb` files into `TestSet[*zeTestWebTest]` (unchanged from today).
2. Apply selection filters (`-a`, `-p`, `--start`, positional IDs) (unchanged).
3. Build `ParallelRunner[*zeTestWebTest]`, set concurrency cap to `webConcurrency` (~4).
4. For each test (concurrently, bounded by cap):
   a. `ReservePorts(base, 1)` -> get isolated port.
   b. `os.MkdirTemp("", "ze-web-test-*")` -> get isolated config dir.
   c. Start per-test ze daemon on reserved port with isolated config dir.
   d. Probe port until daemon is ready (replace `time.Sleep`).
   e. Call `RunWBFile(path, "https://127.0.0.1:<port>")` with session-scoped browser.
   f. Tear down: `close` own browser session -> kill daemon -> release port reservation -> remove tmpdir.
5. `ParallelRunner` aggregates results/timing/failure-groups via `Display`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| test -> isolated daemon | per-test port (ReservePorts) + per-test tmpdir; teardown on completion | [ ] |
| test -> isolated browser | `AGENT_BROWSER_SESSION=<nick>` threaded through `agentEnv`; `close` not `close --all` | [ ] |
| web suite -> ParallelRunner | `ParallelRunner[*zeTestWebTest]` with web concurrency cap | [ ] |

### Integration Points
- `runner.ParallelRunner[T]` from `internal/test/runner/parallel.go` (used as-is).
- `runner.ReservePorts` from `internal/test/runner/ports.go` (used as-is).
- `runner.TestSet[T]` from `internal/test/runner/base.go` (already used by web today).
- `webtesting.RunWBFile` from `internal/component/web/testing/runner.go` (needs session parameter).

### Architectural Verification
- [ ] No bypassed layers (web uses the existing ParallelRunner engine)
- [ ] No unintended coupling (per-test daemon fully isolated; no shared state between concurrent tests)
- [ ] No duplicated functionality (bespoke sequential loop deleted, not forked alongside ParallelRunner)
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-test web -a` | -> | `ParallelRunner[*zeTestWebTest]` runs `.wb` concurrently | `TestWebUsesParallelRunner` |
| two `.wb` tests doing `set...commit` | -> | per-test daemon isolation (no cross-contamination) | `TestParallelWebTestsIsolatedDaemons` |
| concurrent browser actions | -> | per-test `AGENT_BROWSER_SESSION` isolation | `TestBrowserSessionScopedEnv` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze-test web -a` with concurrency > 1 | `.wb` tests run concurrently and all pass |
| AC-2 | Two tests both mutate config and commit | Neither sees the other's config (isolated daemons, separate tmpdir + port) |
| AC-3 | Concurrent browser interactions | Each test's refs/cookies isolated (own `--session <nick>`) |
| AC-4 | agent-browser absent | Suite skips gracefully, same as today |
| AC-5 | Web suite vs sequential baseline | Same pass/fail set; wall-clock reduced when >1 test selected |
| AC-6 | `--port` flag provided | Ignored for per-test allocation (or error: incompatible with parallel) |
| AC-7 | Display output | Progress, timing, failure groups shown via ParallelRunner Display |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBrowserSessionScopedEnv` | `internal/component/web/testing/runner_test.go` | AC-3: session-scoped `agentEnv` produces correct `AGENT_BROWSER_SESSION` | planned |
| `TestBrowserCloseOwnSession` | `internal/component/web/testing/runner_test.go` | AC-3: `Close()` uses `close` (own session), not `close --all` | planned |
| `TestPerTestDaemonStartStop` | `internal/test/cli/cmd_web_test.go` | AC-2: daemon starts on reserved port, stops cleanly | planned |
| `TestWebConcurrencyCap` | `internal/test/cli/cmd_web_test.go` | AC-1: concurrency capped at webConcurrency | planned |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Web concurrency cap | 1..max | max (all tests) | 0 (clamps to 1) | > test count (clamps to count) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/web/*.wb` (existing 72 tests) | `test/web/` | All `.wb` scenarios pass when run in parallel | planned |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Not applicable | - | - | harness change, not a wire protocol | N/A |

### Future (if deferring any tests)
- (none planned)

## Files to Modify

- `internal/test/cli/cmd_web.go` - replace sequential loop with `ParallelRunner[*zeTestWebTest]`; per-test daemon lifecycle (start/probe/stop); remove shared daemon; handle `--port` incompatibility
- `internal/component/web/testing/runner.go` - session-scoped `Browser` (add `session` field); session-aware `agentEnv`; `Close()` uses `close` not `close --all`; new `RunWBFileWithSession(path, baseURL, session)` or add session to `Browser` constructor

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Test infrastructure changed | Yes | `docs/functional-tests.md` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | CLI flags unchanged |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - document parallel web execution |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | test harness, not architecture |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, etc.? | No | - |
| 16 | Any changed source file referenced by doc anchors? | [ ] | grep during implementation |
| 17 | Existing docs show config/CLI/API examples? | [ ] | grep during implementation |

## Files to Create
- (none expected; all changes are modifications to existing files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Validation spike** -- confirm agent-browser `--session` parallel isolation
   - Run: start agent-browser, open two `--session` browsers concurrently, verify isolated refs/cookies, verify `close` only closes own session.
   - Measure: resource cost of N concurrent sessions to calibrate `webConcurrency` cap.
   - Outcome: go/no-go for the approach. If `--session` doesn't isolate, stop and redesign.

2. **Phase: Session-scoped Browser** -- make `Browser` session-aware
   - Tests: `TestBrowserSessionScopedEnv`, `TestBrowserCloseOwnSession`
   - Files: `internal/component/web/testing/runner.go`
   - Changes:
     - Add `session string` field to `Browser`.
     - New constructor: `NewBrowserWithSession(baseURL, session string)`.
     - `agentEnvWithSession(session string)` adds `AGENT_BROWSER_SESSION=<session>` to env when non-empty.
     - All `runAgent*` functions gain session parameter (or `Browser` methods pass it via env).
     - `Close()`: if session is set, use `close` (closes own session); if empty, `close --all` (backward compat).
   - Verify: tests fail (no session threading) -> implement -> tests pass.

3. **Phase: Per-test daemon lifecycle** -- start/stop ze daemon per test
   - Tests: `TestPerTestDaemonStartStop`
   - Files: `internal/test/cli/cmd_web.go`
   - Changes:
     - Extract `zeTestStartWebServer` into a per-test function that takes port + tmpdir.
     - Replace `time.Sleep(3s)` with port-probe loop (try TCP connect until success or timeout).
     - Each test: `ReservePorts` -> `MkdirTemp` -> start daemon -> probe -> run -> stop -> cleanup.
   - Verify: tests fail -> implement -> tests pass.

4. **Phase: ParallelRunner migration** -- replace bespoke loop
   - Tests: `TestWebUsesParallelRunner`, `TestWebConcurrencyCap`
   - Files: `internal/test/cli/cmd_web.go`
   - Changes:
     - Follow `cmd_editor.go:150-200` pattern exactly.
     - `NewParallelRunner[*zeTestWebTest](colors)` -> `SetLabel("web")` -> `SetConcurrency(webConcurrency)`.
     - Loop: `AddTestWithNick` per selected test; run function does per-test daemon + session-scoped browser + `RunWBFile`.
     - `SetOnFail` callback: print error + trace (from current verbose output).
     - Delete the bespoke sequential loop, custom progress formatting, and manual pass/fail counting.
     - Handle `--port` flag: error if set with parallel mode, or ignore with warning.
   - Verify: tests fail -> implement -> tests pass -> all 72 `.wb` tests pass in parallel.

5. **Phase: Isolation integration test** -- prove parallel tests don't contaminate
   - Tests: `TestParallelWebTestsIsolatedDaemons`
   - Files: `internal/test/cli/cmd_web_test.go`
   - Run two `.wb` tests that both do `set...commit` concurrently. Verify neither sees the other's config.
   - Verify: test fails without isolation -> passes with per-test daemon.

6. **Functional tests** -- run existing `.wb` suite in parallel
   - Run `ze-test web -a` and verify all 72 tests pass.
   - Compare pass/fail set with sequential baseline (same results, lower wall-clock).

7. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)

8. **Complete spec** -> Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`. TWO commits: commit A saves code + tests + spec + learned summary; commit B does `git rm` of the spec.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Per-test daemon truly isolated (separate port + tmpdir); no shared mutable state between concurrent tests |
| Naming | Session names use test nick (stable, unique); no collision between concurrent sessions |
| Data flow | Browser session env threaded through all `runAgent*` calls; no path bypasses session scoping |
| Cleanup | Every test cleans up: daemon killed, port reservation released, tmpdir removed, browser session closed |
| Backward compat | `NewBrowser(baseURL)` still works (empty session = legacy `close --all` behavior) |
| Rule: no-layering | Bespoke sequential loop fully deleted, not left alongside ParallelRunner |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Session-scoped Browser | `grep -n 'session' internal/component/web/testing/runner.go` |
| Per-test daemon lifecycle | `grep -n 'ReservePorts\|MkdirTemp' internal/test/cli/cmd_web.go` |
| ParallelRunner migration | `grep -n 'ParallelRunner' internal/test/cli/cmd_web.go` |
| Bespoke loop removed | `grep -c 'for.*range.*selectedTests' internal/test/cli/cmd_web.go` returns 0 |
| All 72 .wb tests pass | `ze-test web -a` exits 0 |
| Unit tests pass | `go test ./internal/component/web/testing/ ./internal/test/cli/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Session names derived from test nicks (controlled input, not user-supplied) |
| Resource exhaustion | Concurrency cap prevents unbounded daemon/Chrome spawning; cleanup on test failure/timeout |
| Temp file cleanup | All `MkdirTemp` dirs removed in defer, including on panic/timeout |
| Port exhaustion | `ReservePorts` with flock prevents port collision; reservation released on cleanup |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| agent-browser `--session` doesn't isolate | STOP at Phase 1. Report finding. Redesign (multiple agent-browser daemons?) |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-test ze daemon for isolation | Shared daemon + config namespacing | `.wb` tests do `set ... commit` which mutates the daemon's config store; only separate daemons give true isolation. The cost is startup time, bounded by the concurrency cap |
| `AGENT_BROWSER_SESSION=<nick>` per test | One browser with tabs; multiple agent-browser daemons | Native isolated-session support in agent-browser; lightest correct option. Each session has independent cookies/tabs/refs |
| Web concurrency cap ~4 | Reuse default cap of 20 | Each web test = one Chrome + one ze daemon; 20 concurrent would exhaust memory. Calibrate exact value in Phase 1 spike |
| Use existing `ParallelRunner[T]` | Build web-specific runner | ParallelRunner already has everything needed (concurrency, display, timing, failure callbacks). The editor command proves the pattern works. Zero framework code needed |
| Port-probe loop for daemon readiness | `time.Sleep(3s)` (current) | Sleep is flaky (too short on slow machines, wasted time on fast ones). TCP probe is deterministic |

## Known Limitations
- Per-test daemon + Chrome is resource-heavy; web cap is deliberately low (~4). Measured in Phase 1.
- `--port` flag becomes incompatible with parallel execution (each test needs its own port).
- Daemon startup time is serialized per test within the concurrency window; no shared daemon optimization.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (or N/A -- non-protocol feature)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
