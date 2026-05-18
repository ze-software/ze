# Spec: diag-crash-capture

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/7 |
| Updated | 2026-05-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/core/slogutil/slogutil.go` - logging infra, backend selection, env vars
4. `internal/core/slogutil/syslog.go` - syslog handler, Dial pattern
5. `internal/core/slogutil/ring.go` - in-memory log ring buffer
6. `cmd/ze/main.go:163` - main() entry point where crash capture must wire
7. `cmd/ze/hub/main.go` - hub Run(), signal handling, no global panic handler

## Task

Ze runs on gokrazy appliances where stderr goes nowhere useful. When the process panics (in any goroutine), the stack trace is written to stderr by the Go runtime and lost. Existing component-level panic recovery (BGP reactor, plugin delivery, API handlers) keeps those subsystems alive, but any panic outside those recovery points kills the process silently. The in-memory log ring buffer (512 entries of pre-crash context) is also lost.

This spec adds crash capture: redirect stderr to syslog (reusing `ze.log.destination`) and persist crash reports to disk so they survive restarts and can be read via CLI.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - system architecture
  -> Constraint: Small core + registration pattern. Components register at startup via init().
- [ ] `ai/rules/design-context.md` - design context loading
  -> Constraint: Check internal/core/ for existing shared patterns before creating new ones.
- [ ] `ai/patterns/config-option.md` - env var pattern
  -> Constraint: Use env.MustRegister for all env vars.

### RFC Summaries (MUST for protocol work)
- Not protocol work; no RFC summaries needed.

**Key insights:**
- slog has syslog backend via `ze.log.backend=syslog` and `ze.log.destination=<addr>`, but panics bypass slog entirely (Go runtime writes directly to fd 2)
- Existing log ring buffer (512 entries) provides pre-crash context but is RAM-only
- Global panic handler is missing from main() and hub.Run()
- gokrazy has no systemd journal; stderr capture is the only path to preserve panic output
- `internal/core/health/` exists for component health checks (Registry with Status checks)
- `ze.log.destination` already knows how to reach syslog; crash capture reuses it
- `GlobalLogRing()` is consumed by `internal/component/cmd/log/log.go` for `show log recent`
- `version.Release()`, `version.BuildDate()` provide version info for crash reports

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go:163-170` - main() entry: version stamp, register commands, dispatch. No defer recover(). No stderr redirect.
- [ ] `cmd/ze/hub/main.go:112-780` - hub.Run(): signal handling, startup, wait loop, shutdown. No global panic handler.
- [ ] `internal/core/slogutil/slogutil.go:42-49` - env var registrations for ze.log, ze.log.backend, ze.log.destination
- [ ] `internal/core/slogutil/syslog.go` - syslog.Dial(network, raddr, LOG_INFO|LOG_DAEMON, "ze"). Falls back to stderr on error.
- [ ] `internal/core/slogutil/ring.go` - LogRing with 512 entries, globalLogRing singleton, Snapshot(limit, level, component)
- [ ] `internal/component/bgp/reactor/peer_run.go:167` - safeRunOnce recover(): catches peer panics, logs via slog, enters backoff reconnect
- [ ] `internal/component/plugin/process/delivery.go:29-31` - safeBridgeCall recover(): catches plugin panics, logs with debug.Stack()
- [ ] `cmd/ze/hub/api.go:286-291` - API streaming panic recovery

**Behavior to preserve:**
- Existing component-level panic recovery (BGP reactor, plugin delivery, API) continues as-is
- slog configuration via ze.log.* env vars unchanged
- Log ring buffer behavior unchanged
- syslog.go handler creation pattern unchanged

**Behavior to change:**
- stderr (fd 2) redirected to a pipe at process startup
- Pipe reader forwards all stderr output to syslog and to original fd (tee)
- On crash detection, dump ring buffer + stderr content to a crash file on disk
- Global defer in main() catches main-goroutine panics and writes last-gasp syslog message

## Data Flow (MANDATORY)

### Entry Point
- Go runtime panic: runtime writes goroutine stack traces to fd 2 (stderr)
- Main goroutine panic: caught by defer recover() in main()
- Normal stderr output: fmt.Fprintf(os.Stderr, ...) from anywhere in the process

### Transformation Path
1. **Startup (cmd/ze/main.go:main):** Before any other work, call `crashlog.Init()`. This saves the original fd 2, creates an os.Pipe, dup2's the write end onto fd 2, and starts a reader goroutine. Uses `env.Get("ze.log.destination")` for syslog target.
2. **Steady state:** The reader goroutine reads lines from the pipe. Each line is written to: (a) the original stderr fd (so terminal output still works), (b) syslog via a direct syslog.Writer connection (when ze.log.destination is configured).
3. **Panic detected:** The reader goroutine sees the "goroutine N [running]:" pattern (Go panic signature). It switches to crash accumulation mode: collects the full stack trace until the pipe closes.
4. **Pipe closed (process dying):** The reader goroutine writes the accumulated crash report (stack trace + ring buffer snapshot + timestamp/version) to a crash file on disk.
5. **Main goroutine panic (belt-and-suspenders):** The defer in main() catches the panic value, writes a one-line syslog message with the panic value and abbreviated stack, then re-panics (so the runtime still produces the full trace on stderr, which the pipe captures).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Go runtime -> crashlog pipe | fd 2 dup2 redirect | [ ] |
| crashlog pipe -> syslog | direct syslog.Writer (same address as ze.log.destination) | [ ] |
| crashlog pipe -> crash file | os.Create to ze.crash.dir | [ ] |
| crashlog pipe -> original stderr | write to saved fd | [ ] |

### Integration Points
- `slogutil.GlobalLogRing()` - snapshot ring buffer for pre-crash context
- `env.Get("ze.log.destination")` - reuse existing syslog address config
- `env.MustRegister()` - register ze.crash.dir, ze.crash.keep env vars
- `version.Release()` / `version.BuildDate()` - include in crash report

### Architectural Verification
- [ ] No bypassed layers (crash capture is below slog, captures what slog cannot)
- [ ] No unintended coupling (crashlog package only depends on core/env, core/slogutil, core/version)
- [ ] No duplicated functionality (extends syslog, does not replace slogutil)
- [ ] Zero-copy preserved where applicable (N/A, crash path is not hot)

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `main()` first line | -> | `crashlog.Init()` | `TestInitRedirectsStderr` |
| Go panic on any goroutine | -> | pipe reader -> syslog + file | `TestPanicCapturedToFile` |
| `ze show crashes` CLI | -> | `crashlog.ListCrashes()` | `TestShowCrashesListsFiles` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Process starts with `ze.log.destination` set | stderr is redirected; pipe reader goroutine sends stderr lines to syslog at that address |
| AC-2 | Process starts without `ze.log.destination` | stderr redirect still happens for crash file persistence; syslog forwarding is skipped (no syslog target) |
| AC-3 | A goroutine panics (not main) | Full stack trace appears in syslog (if configured) AND in a crash file under `ze.crash.dir` |
| AC-4 | Main goroutine panics | defer handler writes panic value + stack to syslog before re-panicking; pipe captures the full trace |
| AC-5 | Crash file written | Contains: timestamp, version, build date, panic stack trace, last 64 ring buffer entries |
| AC-6 | More than `ze.crash.keep` crash files exist | Oldest files are removed, only `ze.crash.keep` most recent remain |
| AC-7 | `ze show crashes` invoked | Lists crash files with timestamp and first line of panic; shows "no crashes recorded" if empty |
| AC-8 | `ze show crashes latest` invoked | Displays full content of most recent crash file |
| AC-9 | `ze.crash.dir` not set | Autodetection probes candidates in priority order; first writable path is used; resolved path logged at startup |
| AC-9b | `ze.crash.dir` explicitly set but not writable | Warning at startup; syslog capture still works; file persistence skipped |
| AC-9c | No candidate writable (all 4 probes fail) | Warning at startup; syslog capture still works; file persistence skipped |
| AC-10 | Normal (non-panic) stderr output | Still appears on original stderr AND forwarded to syslog; no crash file created |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInitRedirectsStderr` | `internal/core/crashlog/stderr_test.go` | Init() creates pipe, reads from it | |
| `TestTeeToOriginalStderr` | `internal/core/crashlog/stderr_test.go` | Lines written to fd 2 appear on original stderr | |
| `TestSyslogForwarding` | `internal/core/crashlog/stderr_test.go` | Lines forwarded to syslog writer when configured | |
| `TestPanicDetection` | `internal/core/crashlog/detect_test.go` | "goroutine N [running]:" pattern triggers crash mode | |
| `TestCrashFileWritten` | `internal/core/crashlog/persist_test.go` | Crash report written to correct path with correct content | |
| `TestCrashFileRotation` | `internal/core/crashlog/persist_test.go` | Old crash files removed when count exceeds keep limit | |
| `TestRingBufferIncluded` | `internal/core/crashlog/persist_test.go` | Crash file includes ring buffer snapshot | |
| `TestNoCrashDir` | `internal/core/crashlog/persist_test.go` | Graceful degradation when dir not writable | |
| `TestAutodetectFirstWritable` | `internal/core/crashlog/persist_test.go` | Probes candidates in order, picks first writable | |
| `TestAutodetectAllFail` | `internal/core/crashlog/persist_test.go` | All candidates unwritable: warning, file persistence disabled | |
| `TestAutodetectExplicitOverride` | `internal/core/crashlog/persist_test.go` | ze.crash.dir set: skips probe, uses explicit path | |
| `TestLastGasp` | `internal/core/crashlog/lastgasp_test.go` | LastGaspHandler writes to syslog on panic | |
| `TestShowCrashesListsFiles` | `internal/core/crashlog/list_test.go` | ListCrashes returns sorted crash file summaries | |
| `TestShowCrashesLatest` | `internal/core/crashlog/list_test.go` | LatestCrash returns full content of newest file | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ze.crash.keep` | 1-100 | 100 | 0 (clamped to 1) | 101 (clamped to 100) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-crash-capture` | `test/crash/crash-capture.ci` | Start ze, trigger panic via chaos, verify crash file exists and contains stack trace | |

### Future (if deferring any tests)
- Integration test with real syslog server (requires network syslog listener in test infra)

## Files to Modify
- `cmd/ze/main.go` - Wire crashlog.Init() as first action in main(), add defer crashlog.LastGaspHandler()
- `internal/core/slogutil/ring.go` - Export ring snapshot format for crash file inclusion (may already be sufficient via Snapshot())

## Files to Create
- `internal/core/crashlog/crashlog.go` - Package doc, Init() entry point, env var registrations
- `internal/core/crashlog/stderr.go` - Stderr redirect logic (pipe + reader goroutine + tee)
- `internal/core/crashlog/dup2_unix.go` - syscall.Dup2 implementation (`//go:build unix` covers linux + darwin + freebsd)
- `internal/core/crashlog/dup2_unsupported.go` - No-op stub (`//go:build !unix`)
- `internal/core/crashlog/detect.go` - Panic pattern detection in stderr stream
- `internal/core/crashlog/persist.go` - Crash file write + rotation
- `internal/core/crashlog/lastgasp.go` - Defer panic handler for main goroutine
- `internal/core/crashlog/list.go` - ListCrashes(), LatestCrash() for CLI
- `internal/core/crashlog/stderr_test.go` - Stderr redirect tests
- `internal/core/crashlog/detect_test.go` - Panic detection tests
- `internal/core/crashlog/persist_test.go` - Crash file persistence tests
- `internal/core/crashlog/lastgasp_test.go` - Last-gasp handler tests
- `internal/core/crashlog/list_test.go` - List/latest crash tests
- `test/crash/crash-capture.ci` - Functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (show crashes is a local file read, not RPC) |
| CLI commands/flags | Yes | `ze show crashes` via show command registration |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/crash/crash-capture.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add crash capture |
| 2 | Config syntax changed? | No | N/A (env vars only, no YANG config) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - add `ze show crashes` |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/operations.md` - add crash capture section |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - crash capture as differentiator |
| 12 | Internal architecture changed? | No | N/A (new core package, additive) |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
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

1. **Phase: Wiring (MANDATORY FIRST)** - register crashlog.Init() in main(), create package skeleton
   - Tests: `TestInitRedirectsStderr`
   - Files: `internal/core/crashlog/crashlog.go`, `cmd/ze/main.go`
   - Verify: Init() is called; wiring test fails because feature logic is a stub

2. **Phase: Stderr redirect** - pipe creation, dup2, tee to original stderr
   - Tests: `TestInitRedirectsStderr`, `TestTeeToOriginalStderr`
   - Files: `internal/core/crashlog/stderr.go`, `dup2_unix.go`, `dup2_unsupported.go`
   - Verify: stderr output appears on original fd and is readable from pipe

3. **Phase: Syslog forwarding** - forward stderr lines to syslog using ze.log.destination
   - Tests: `TestSyslogForwarding`
   - Files: `internal/core/crashlog/stderr.go`
   - Verify: stderr lines appear in syslog writer

4. **Phase: Panic detection** - pattern match "goroutine N [running]:" in stderr stream
   - Tests: `TestPanicDetection`
   - Files: `internal/core/crashlog/detect.go`
   - Verify: panic signature triggers crash accumulation mode

5. **Phase: Crash file persistence** - write crash report to disk, include ring buffer, rotate
   - Tests: `TestCrashFileWritten`, `TestCrashFileRotation`, `TestRingBufferIncluded`, `TestNoCrashDir`
   - Files: `internal/core/crashlog/persist.go`
   - Verify: crash files created, rotated, contain ring buffer

6. **Phase: Last-gasp handler** - defer in main() for main-goroutine panics
   - Tests: `TestLastGasp`
   - Files: `internal/core/crashlog/lastgasp.go`, `cmd/ze/main.go`
   - Verify: main goroutine panic sends syslog message before re-panic

7. **Phase: CLI integration** - `ze show crashes` and `ze show crashes latest`
   - Tests: `TestShowCrashesListsFiles`, `TestShowCrashesLatest`
   - Files: `internal/core/crashlog/list.go`
   - Verify: CLI reads crash dir, lists/displays files

8. **Functional tests** - Create after feature works. Cover user-visible behavior.
9. **Full verification** - `make ze-verify`
10. **Complete spec** - Fill audit tables, write learned summary, delete spec.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Panic detection regex does not false-positive on normal goroutine dumps (runtime.Stack vs panic) |
| Naming | Crash file format: `crash-YYYYMMDD-HHMMSS.log`. Env vars: `ze.crash.*` |
| Data flow | Stderr redirect happens before any other init; pipe reader outlives main goroutine |
| Rule: no-layering | No wrapper around syslog.Writer; direct use |
| Rule: graceful-degradation | Every failure path (no syslog, no crash dir) degrades without blocking startup |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/core/crashlog/` package exists | `ls internal/core/crashlog/*.go` |
| Init() called in main() | `grep -n 'crashlog.Init' cmd/ze/main.go` |
| LastGaspHandler wired as defer | `grep -n 'defer.*LastGasp\|defer.*crashlog' cmd/ze/main.go` |
| Crash files written on panic | Functional test `test-crash-capture` |
| `ze show crashes` works | Functional test or manual verification |
| Env vars registered | `grep -n 'MustRegister.*crash' internal/core/crashlog/` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Crash dir path must not be user-controlled beyond env var; no path traversal |
| Resource exhaustion | Crash file rotation enforced; no unbounded accumulation |
| Information leakage | Crash files contain stack traces with internal paths; ensure crash dir permissions are restrictive (0700) |
| Syslog injection | Stderr lines forwarded as-is; syslog protocol handles escaping |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| dup2 not available on platform | Build tag stub (`dup2_unsupported.go`, `//go:build !unix`) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Env Vars

| Env var | Type | Default | Description |
|---------|------|---------|-------------|
| `ze.crash.dir` | string | (autodetect) | Override crash report directory. If not set, Init() probes candidates in order until one is writable. |
| `ze.crash.keep` | int | `5` | Number of crash files to retain (1-100) |

### Crash Dir Autodetection

Crash files must always be written. If `ze.crash.dir` is set, use it. Otherwise, probe in order and use the first writable candidate:

| Priority | Candidate | Rationale |
|----------|-----------|-----------|
| 1 | `/perm/ze/crash/` | gokrazy persistent partition (survives reboot) |
| 2 | `<config-dir>/crash/` | Next to config (via `paths.DefaultConfigDir()`); traditional installs where config is under a writable prefix |
| 3 | `/var/lib/ze/crash/` | FHS standard state directory for daemons |
| 4 | `/tmp/ze-crash/` | Last resort; cleared on reboot but captures the crash that just happened before supervisor restarts ze |

Probe means: attempt `os.MkdirAll(candidate, 0700)`, then create+remove a temp file. First success wins. If all fail, log a warning to stderr (pre-syslog, so it at least appears in whatever captures stderr on this platform) and disable file persistence.

The resolved path is logged at startup: `crash dir: /perm/ze/crash/` so the operator knows where to look.

Crash capture to syslog (via `ze.log.destination`) works regardless of crash dir resolution.

## Crash File Format

Two sources write crash files:

**Last-gasp handler (main goroutine panics):** writes via HandlePanic() with full
ring buffer context and debug.Stack() trace. Reliable (runs in defer before os.Exit).

```
=== Ze Crash Report ===
Time: 2026-05-18T14:23:45Z
Version: 0.9.1
Build: 2026-05-17
Uptime: 3h42m18s

=== Panic ===
<panic value>

=== Stack Trace ===
<debug.Stack() output>

=== Recent Log (last 64 entries) ===
<ring buffer snapshot, newest first>
```

**Pipe reader (non-main goroutine panics):** detects "goroutine N [running]:" pattern
in stderr, captures ring buffer at detection time, accumulates the panic trace.
Best-effort (races against process exit, but syslog forwarding is line-by-line and reliable).

```
=== Recent Log (pre-crash) ===
<ring buffer snapshot>

=== Panic ===
goroutine 42 [running]:
<full runtime output from stderr>
```

## Design Insights

- **Why stderr redirect, not just defer recover():** Go panics in non-main goroutines cannot be caught by a defer in main(). The runtime writes the stack trace to fd 2 and calls exit(2). The only way to capture arbitrary goroutine panics is to intercept fd 2 itself.
- **Why tee to original stderr:** Terminal users (dev mode, manual testing) still need to see panic output. The redirect must not suppress it.
- **Why reuse ze.log.destination:** The user already configures one syslog target. Crash reports should go to the same place. Adding a separate crash syslog config would create configuration divergence with no benefit.
- **Why ring buffer in crash file:** The stack trace tells you where the panic happened. The ring buffer tells you what was happening before the panic (config reload, peer flap, plugin restart). Both are needed for diagnosis.
- **Why syslog.LOG_CRIT for crash messages:** Distinguishes crash reports from normal log traffic in syslog infrastructure. Allows syslog rules/filters to route crash reports to high-priority channels.

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

## RFC Documentation

N/A - not protocol work.

## Implementation Summary

### What Was Implemented
- `internal/core/crashlog/` package: Init(), HandlePanic(), stderr pipe redirect, syslog forwarding, panic detection, crash file persistence with rotation, crash dir autodetection
- `internal/core/crashlog/dup2_unix.go` + `dup2_unsupported.go`: platform-split fd2 redirect via syscall.Dup2 (works on linux + darwin)
- `internal/core/crashlog/list.go`: ListCrashes(), LatestCrash(), CrashDir() for CLI queries
- `internal/component/cmd/show/crashes.go`: ze-show:crashes and ze-show:crashes-latest RPC handlers
- `cmd/ze/main.go`: crashlog.Init() wired as first call in main()
- 13 unit tests + 1 wiring test
- Documentation: features.md, operations.md, command-reference.md, comparison.md

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/features.md`: added Crash Capture row
- `docs/guide/operations.md`: added Crash Capture section with config table and CLI examples
- `docs/guide/command-reference.md`: added show crashes section
- `docs/comparison.md`: added crash capture row

### Deviations from Plan
- Removed current.log/CleanShutdown approach: pipe reader now only writes crash files when it detects a panic pattern (no continuous file writing, no cleanup needed)
- HandlePanic + os.Exit(2) lives in cmd/ze/main.go (hook constraints prevent panic/os.Exit in library code)
- File count consolidated from 8 to 6 source files (detect.go merged into stderr.go, separate stderr files merged)
- promotePreviousCrash removed (no current.log to promote)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/core/crashlog/`, `cmd/ze/main.go`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass, defer with user approval)
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

### Completion (BLOCKING, before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/725-diag-crash-capture.md`
- [ ] Summary included in commit
