# Spec: signal-command

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/cli-patterns.md` - CLI command patterns
4. `cmd/ze/main.go` - command dispatch (add `signal` subcommand)
5. `cmd/ze/hub/main.go` - PID file integration points

## Task

Implement PID file management and `ze signal` CLI command for sending signals to running Ze instances.

**Scope:** PID file lifecycle + `ze signal reload|stop|quit|status` CLI.

**Out of scope (separate specs):**
- Config reload infrastructure — done (specs 222–234)
- Connection handoff via SCM_RIGHTS — see `spec-connection-handoff.md`

**Related gap:** `cmd/ze/hub/main.go:122` has `// TODO: Implement config reload` in the hub orchestrator path. The BGP in-process path (`runBGPInProcess`) wires SIGHUP via the reactor's `SignalHandler`. The hub orchestrator path does not yet call `ReloadFromDisk()`. This wiring should be addressed as part of this spec or as a prerequisite.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - system overview
- [ ] `.claude/rules/cli-patterns.md` - CLI flag/dispatch patterns

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - BGP FSM, session lifecycle (Cease notification on shutdown)

**Key insights:**
- `cmd/ze/main.go` dispatches by command name; `signal` is a new subcommand
- Two startup paths: `runBGPInProcess` (reactor + SignalHandler) and `runOrchestratorWithData` (hub orchestrator)
- PID file must work for both paths

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` - command dispatch; no `signal` command exists
- [ ] `cmd/ze/hub/main.go` - `Run()` dispatches to `runBGPInProcess` or `runOrchestratorWithData`; no PID file; SIGHUP handled by reactor's `SignalHandler` in BGP path, but `// TODO` in hub orchestrator path
- [ ] `internal/plugin/bgp/reactor/signal.go` - `SignalHandler` with `OnShutdown`, `OnReload`, `OnStatus` callbacks; handles SIGTERM/SIGINT/SIGHUP/SIGUSR1

**Behavior to preserve:**
- SIGTERM/SIGINT → graceful shutdown (both paths)
- SIGHUP → reload callback via `SignalHandler.OnReload` (BGP in-process path)
- SIGUSR1 → status dump callback (BGP in-process path)

**Behavior to change:**
- Add PID file write on startup, remove on shutdown (both paths)
- Add `ze signal` CLI command (new subcommand)

## Data Flow (MANDATORY)

### Entry Point
- `ze signal <cmd> <config>` — CLI parses args, resolves PID file location, sends OS signal

### Transformation Path
1. CLI parses command (reload/stop/quit/status) and config path
2. Resolve config path to absolute
3. Compute PID file location (XDG or config dir)
4. Read PID file, verify config path matches
5. Check process alive via `kill(pid, 0)`
6. Send appropriate signal (`SIGHUP`, `SIGTERM`, `SIGQUIT`) or check status

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI → OS | `syscall.Kill(pid, signal)` | [ ] |
| PID file → filesystem | `flock` for mutual exclusion | [ ] |

### Integration Points
- `cmd/ze/main.go` dispatch switch — add `"signal"` case
- `cmd/ze/hub/main.go` — call `pidfile.Acquire()` on startup, `pidfile.Release()` on shutdown
- `internal/plugin/bgp/reactor/signal.go` — existing signal handling (no changes needed)

### Architectural Verification
- [ ] No bypassed layers (PID file is a filesystem convention, no engine coupling)
- [ ] No unintended coupling (pidfile package is standalone)
- [ ] No duplicated functionality (no existing PID file code)
- [ ] Zero-copy preserved where applicable (N/A — filesystem ops)

---

## Design

### 1. PID File Management

#### Location Strategy

Priority order (first accessible wins):

| Priority | Path | Condition |
|----------|------|-----------|
| 1 | `$XDG_RUNTIME_DIR/ze/<config-hash>.pid` | XDG_RUNTIME_DIR set and writable |
| 2 | `/var/run/ze/<config-hash>.pid` | Running as root |
| 3 | `/tmp/ze/<config-hash>.pid` | Fallback (always writable) |

~~Previous design used config-dir fallback. Changed to match socket cascade (`DefaultSocketPath`).~~

**Config hash:** First 8 characters of SHA256 of absolute config path.

#### PID File Content

| Line | Content | Example |
|------|---------|---------|
| 1 | Process ID | `12345` |
| 2 | Absolute config path | `/etc/ze/router.conf` |
| 3 | Start timestamp (RFC 3339) | `2026-01-31T10:30:00Z` |

#### Lifecycle

| Event | Action |
|-------|--------|
| Startup | Create parent dir, write PID file, acquire `flock(LOCK_EX)` |
| Running | Hold flock — prevents duplicate instances |
| Shutdown | Release flock, remove PID file |
| Crash | Stale file detected by failed `flock(LOCK_NB)` |

#### Stale PID Detection

1. Read PID from file
2. Try `flock(fd, LOCK_EX|LOCK_NB)` on the PID file
3. Lock succeeds → stale file (process dead), can overwrite
4. Lock fails (`EWOULDBLOCK`) → process running

### 2. `ze signal` Command

#### CLI Interface

```
ze signal <command> [options] <config>

Commands:
  reload    Send SIGHUP - reload configuration
  stop      Send SIGTERM - graceful shutdown
  quit      Send SIGQUIT - goroutine dump + immediate exit (debug)
  status    Check if process is running (exit 0 = running, 1 = not)

Options:
  --pid-file <path>   Use explicit PID file instead of deriving from config

Arguments:
  <config>            Config file path (used to derive PID file location)
```

#### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (signal sent, or status=running) |
| 1 | Process not running (status command) |
| 2 | PID file not found |
| 3 | Permission denied |
| 4 | Signal delivery failed |

#### Signal Mapping

| Command | Signal | Receiver |
|---------|--------|----------|
| `reload` | `SIGHUP` | `SignalHandler.OnReload` or hub SIGHUP handler |
| `stop` | `SIGTERM` | `SignalHandler.OnShutdown` or hub SIGTERM handler |
| `quit` | `SIGQUIT` | Goroutine dump + immediate exit (Go default, not caught) |
| `status` | `kill(pid, 0)` | Check alive only |

---

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPIDFileLocationXDG` | `internal/pidfile/pidfile_test.go` | Uses XDG_RUNTIME_DIR when set | |
| `TestPIDFileLocationTmpFallback` | `internal/pidfile/pidfile_test.go` | Falls back to /tmp/ze/ when XDG unset | |
| `TestPIDFileLocationAlwaysUsesHash` | `internal/pidfile/pidfile_test.go` | Filename is always config-hash.pid | |
| `TestPIDFileCreate` | `internal/pidfile/pidfile_test.go` | Creates file with correct content format | |
| `TestPIDFileAcquireLock` | `internal/pidfile/pidfile_test.go` | Acquires flock, second acquire fails | |
| `TestPIDFileRelease` | `internal/pidfile/pidfile_test.go` | Releases flock, file removed | |
| `TestPIDFileStaleDetection` | `internal/pidfile/pidfile_test.go` | Detects stale file (no lock holder) | |
| `TestPIDFileConfigHash` | `internal/pidfile/pidfile_test.go` | Consistent hash from config path | |
| `TestSignalCommandReload` | `cmd/ze/signal/main_test.go` | Maps "reload" to SIGHUP | |
| `TestSignalCommandStop` | `cmd/ze/signal/main_test.go` | Maps "stop" to SIGTERM | |
| `TestSignalCommandStatus` | `cmd/ze/signal/main_test.go` | Checks process alive via kill(0) | |
| `TestSignalCommandMissingArgs` | `cmd/ze/signal/main_test.go` | Prints usage, exits 1 | |
| `TestSignalCommandExplicitPIDFile` | `cmd/ze/signal/main_test.go` | --pid-file overrides config-derived path | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PID | 1–4194304 | 4194304 | 0 | N/A (kernel limit) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pid-file-created` | `test/signal/pid-file.ci` | PID file created on startup, contains correct PID | |
| `pid-file-removed` | `test/signal/pid-cleanup.ci` | PID file removed on graceful shutdown | |
| `signal-status-running` | `test/signal/status-running.ci` | `ze signal status` returns 0 for running process | |
| `signal-status-stopped` | `test/signal/status-stopped.ci` | `ze signal status` returns 1 for stopped process | |
| `signal-stop` | `test/signal/stop.ci` | `ze signal stop` triggers graceful shutdown | |

### Future (if deferring any tests)
- `signal-reload` functional test deferred until hub SIGHUP TODO is wired

---

## Files to Modify
- `cmd/ze/main.go` - Add `"signal"` case to command dispatch switch
- `cmd/ze/hub/main.go` - Integrate PID file acquire/release in both `runBGPInProcess` and `runOrchestratorWithData`

## Files to Create
- `internal/pidfile/pidfile.go` - PID file management (location, create, acquire, release, stale detection)
- `internal/pidfile/pidfile_test.go` - PID file unit tests
- `cmd/ze/signal/main.go` - `ze signal` CLI command
- `cmd/ze/signal/main_test.go` - CLI parsing tests
- `test/signal/pid-file.ci` - PID file creation functional test
- `test/signal/pid-cleanup.ci` - PID file cleanup functional test
- `test/signal/status-running.ci` - Signal status (running) functional test
- `test/signal/status-stopped.ci` - Signal status (stopped) functional test
- `test/signal/stop.ci` - Signal stop functional test

---

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write PID file unit tests** - Location resolution, create, lock, stale detection
   → **Review:** XDG fallback covered? Stale detection edge cases?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Fail for the right reason?

3. **Implement `internal/pidfile/pidfile.go`** - Manager struct with Acquire/Release/IsRunning
   → **Review:** Simplest solution? Any coupling to engine internals?

4. **Run tests** - Verify PASS (paste output)

5. **Write signal CLI unit tests** - Command parsing, signal mapping, missing args
   → **Review:** Exit codes covered? --pid-file override?

6. **Run tests** - Verify FAIL

7. **Implement `cmd/ze/signal/main.go`** - Parse args, resolve PID, send signal
   → **Review:** Follows cli-patterns.md? Errors to stderr?

8. **Run tests** - Verify PASS

9. **Integrate PID file into hub** - Acquire in `Run()`, release on shutdown
   → **Review:** Both paths (BGP + orchestrator) covered?

10. **Add dispatch in `cmd/ze/main.go`** - `case "signal": os.Exit(signal.Run(args[1:]))`

11. **Write functional tests** - PID file lifecycle, signal status/stop
    → **Review:** End-user scenarios covered?

12. **Verify all** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero lint issues? All tests deterministic?

---

## RFC Documentation

### Reference Comments
- `// RFC 4271 Section 6.7` - Cease NOTIFICATION on administrative shutdown (relevant for `ze signal stop`)

---

## Implementation Summary

### What Was Implemented
- `internal/pidfile/pidfile.go` — PID file management: Location (XDG/var-run/tmp cascade matching socket), ConfigHash, Acquire (flock), Release, ReadInfo, Noop, stale detection
- `internal/pidfile/pidfile_test.go` — 9 unit tests covering location resolution, create, lock, release, stale detection, config hash, boundary PID
- `cmd/ze/signal/main.go` — `ze signal` CLI: reload/stop/quit/status commands, --pid-file flag, exit codes 0-4
- `cmd/ze/signal/main_test.go` — 12 unit tests covering signal mapping, status, missing args, explicit PID file, send-to-self, map completeness
- `cmd/ze/hub/main.go` — PID file acquire/release integrated into both `runBGPInProcess` and `runOrchestratorWithData` paths
- `cmd/ze/main.go` — Added `signal` case to command dispatch + usage text
- `internal/test/peer/peer.go` — Extended with `action=sigterm` support (constant, parsing, NextSigtermAction, execution)
- `internal/test/runner/record.go` — Added `action=sigterm` case to parseAction
- `test/reload/signal-stop.ci` — Functional test: SIGTERM causes graceful daemon shutdown

### Bugs Found/Fixed
- Tmpfs requirement for daemon.pid: functional tests using `action=sigterm` require at least one `tmpfs=` block so the runner creates TmpfsTempDir and writes `daemon.pid`
- PID lock failure was non-fatal: `acquirePIDFile` returned nil on lock error, allowing duplicate instances. Fixed to return error, making lock failure fatal.

### Design Insights
- The test runner writes `daemon.pid` to TmpfsTempDir only when tmpfs files exist (runner.go:795-798). Tests needing `action=sighup` or `action=sigterm` must use `tmpfs=` for config

### Documentation Updates
- Updated `docs/architecture/testing/ci-format.md` with `action=sigterm` documentation
- Updated `docs/architecture/behavior/signals.md` — replaced speculative ExaBGP-derived Ze code with actual implementation (signal mapping, PID file, ze signal CLI, startup paths)

### Deviations from Plan
- Functional tests placed in `test/reload/` instead of `test/signal/` — the .ci format cannot verify PID file filesystem state or run `ze signal` mid-test; the single testable scenario (SIGTERM shutdown) fits the reload test category
- 4 of 5 planned functional tests (pid-file-created, pid-file-removed, status-running, status-stopped) cannot be expressed in .ci format — covered by unit tests instead
- `TestPIDFileBoundaryPID` added beyond spec (boundary test for PID range)
- 12 signal CLI tests written (spec listed 5) — additional coverage for quit, no-PID-file, resolve-from-config, send-to-self, map-completeness, run-status-with-PID-file
- PID file location cascade changed from XDG/config-dir/error to XDG/var-run/tmp to match socket cascade (`DefaultSocketPath`)
- `acquirePIDFile` returns error on lock failure (fatal) instead of nil (non-fatal) — prevents duplicate instances
- Added `pidfile.Noop()` for stdin/skip cases to satisfy nilnil linter

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| PID file management | ✅ Done | `internal/pidfile/pidfile.go` | Location, Acquire, Release, ReadInfo, stale detection |
| `ze signal` CLI command | ✅ Done | `cmd/ze/signal/main.go:33` | reload/stop/quit/status + --pid-file |
| PID file acquire on startup | ✅ Done | `cmd/ze/hub/main.go` acquirePIDFile helper | Both runBGPInProcess and runOrchestratorWithData |
| PID file release on shutdown | ✅ Done | `cmd/ze/hub/main.go` defer pf.Release() | Both paths |
| Stale PID detection | ✅ Done | `internal/pidfile/pidfile.go:93` isLocked() | flock(LOCK_EX\|LOCK_NB) |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestPIDFileLocationXDG | ✅ Done | `internal/pidfile/pidfile_test.go:19` | |
| TestPIDFileLocationTmpFallback | ✅ Done | `internal/pidfile/pidfile_test.go:37` | |
| TestPIDFileLocationAlwaysUsesHash | ✅ Done | `internal/pidfile/pidfile_test.go:53` | |
| TestPIDFileCreate | ✅ Done | `internal/pidfile/pidfile_test.go:66` | |
| TestPIDFileAcquireLock | ✅ Done | `internal/pidfile/pidfile_test.go:99` | |
| TestPIDFileRelease | ✅ Done | `internal/pidfile/pidfile_test.go:118` | |
| TestPIDFileStaleDetection | ✅ Done | `internal/pidfile/pidfile_test.go:142` | |
| TestPIDFileConfigHash | ✅ Done | `internal/pidfile/pidfile_test.go:174` | |
| TestSignalCommandReload | ✅ Done | `cmd/ze/signal/main_test.go:20` | |
| TestSignalCommandStop | ✅ Done | `cmd/ze/signal/main_test.go:28` | |
| TestSignalCommandStatus | ✅ Done | `cmd/ze/signal/main_test.go:44` | |
| TestSignalCommandMissingArgs | ✅ Done | `cmd/ze/signal/main_test.go:68` | |
| TestSignalCommandExplicitPIDFile | ✅ Done | `cmd/ze/signal/main_test.go:89` | |
| pid-file-created | ❌ Skipped | — | .ci format cannot verify filesystem state; covered by TestPIDFileCreate unit test |
| pid-file-removed | ❌ Skipped | — | .ci format cannot verify filesystem state; covered by TestPIDFileRelease unit test |
| signal-status-running | ❌ Skipped | — | .ci format cannot run `ze signal` mid-test; covered by TestRunStatusWithPIDFile |
| signal-status-stopped | ❌ Skipped | — | .ci format cannot run `ze signal` mid-test; covered by TestSignalCommandStatus |
| signal-stop | 🔄 Changed | `test/reload/signal-stop.ci` | Uses `action=sigterm` in reload category instead of `test/signal/` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/main.go` | ✅ Modified | Added signal dispatch + usage |
| `cmd/ze/hub/main.go` | ✅ Modified | PID file acquire/release in both paths |
| `internal/pidfile/pidfile.go` | ✅ Created | |
| `internal/pidfile/pidfile_test.go` | ✅ Created | 9 tests |
| `cmd/ze/signal/main.go` | ✅ Created | |
| `cmd/ze/signal/main_test.go` | ✅ Created | 12 tests |
| `test/signal/pid-file.ci` | ❌ Skipped | .ci format limitation; see above |
| `test/signal/pid-cleanup.ci` | ❌ Skipped | .ci format limitation; see above |
| `test/signal/status-running.ci` | ❌ Skipped | .ci format limitation; see above |
| `test/signal/status-stopped.ci` | ❌ Skipped | .ci format limitation; see above |
| `test/signal/stop.ci` | 🔄 Changed | Placed in `test/reload/signal-stop.ci` instead |

### Audit Summary
- **Total items:** 29
- **Done:** 20
- **Partial:** 0
- **Skipped:** 8 (functional tests — .ci format cannot verify PID file state or run CLI mid-test; all covered by unit tests)
- **Changed:** 1 (signal-stop.ci placed in test/reload/ instead of test/signal/)

## Checklist

### 🏗️ Design
- [x] No premature abstraction (pidfile used by hub + signal CLI)
- [x] No speculative features (PID file + signal CLI both needed)
- [x] Single responsibility (pidfile manages PID files; signal sends signals)
- [x] Explicit behavior (flock-based locking, clear exit codes)
- [x] Minimal coupling (pidfile is standalone, signal only depends on pidfile)
- [x] Next-developer test (clear function names, documented exit codes)

### 🧪 TDD
- [x] Tests written (9 pidfile + 12 signal = 21 unit tests)
- [x] Tests FAIL verified before implementation
- [x] Implementation complete
- [x] Tests PASS (all 21 unit tests pass)
- [x] Boundary tests cover all numeric inputs (TestPIDFileBoundaryPID)
- [x] Feature code integrated into codebase (`internal/pidfile`, `cmd/ze/signal`, `cmd/ze/hub`)
- [x] Functional tests verify end-user behavior (`test/reload/signal-stop.ci`)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes (all packages)
- [x] `make functional` passes (236 tests: 42+45+23+22+8+96)

### Documentation
- [x] Required docs read
- [x] RFC summaries read
- [x] RFC references: shutdown is via existing SignalHandler (RFC 4271 Cease)
- [x] RFC constraint comments: N/A (PID file is not protocol code)

### Completion
- [x] Architecture docs updated with learnings (`docs/architecture/behavior/signals.md` rewritten with actual Ze implementation)
- [x] Implementation Audit completed (all items have status + location)
- [x] All Partial/Skipped items have user approval (8 skipped functional tests — .ci format limitation, covered by unit tests)
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/237-signal-command.md`
- [x] All files committed together
