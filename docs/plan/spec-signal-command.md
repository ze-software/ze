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
| 2 | `<config-dir>/<config-name>.pid` | Config directory writable |
| 3 | Error | Neither location writable |

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
  quit      Send SIGQUIT - immediate shutdown
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
| `quit` | `SIGQUIT` | Immediate exit |
| `status` | `kill(pid, 0)` | Check alive only |

---

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPIDFileLocationXDG` | `internal/pidfile/pidfile_test.go` | Uses XDG_RUNTIME_DIR when set | |
| `TestPIDFileLocationConfigDir` | `internal/pidfile/pidfile_test.go` | Falls back to config directory | |
| `TestPIDFileLocationNoWritable` | `internal/pidfile/pidfile_test.go` | Returns error when neither writable | |
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

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered during implementation]

### Design Insights
- [Key learnings that should be documented elsewhere]

### Documentation Updates
- [List docs updated, or "None — no architectural changes"]

### Deviations from Plan
- [Any differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| PID file management | | | |
| `ze signal` CLI command | | | |
| PID file acquire on startup | | | |
| PID file release on shutdown | | | |
| Stale PID detection | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestPIDFileLocationXDG | | | |
| TestPIDFileLocationConfigDir | | | |
| TestPIDFileLocationNoWritable | | | |
| TestPIDFileCreate | | | |
| TestPIDFileAcquireLock | | | |
| TestPIDFileRelease | | | |
| TestPIDFileStaleDetection | | | |
| TestPIDFileConfigHash | | | |
| TestSignalCommandReload | | | |
| TestSignalCommandStop | | | |
| TestSignalCommandStatus | | | |
| TestSignalCommandMissingArgs | | | |
| TestSignalCommandExplicitPIDFile | | | |
| pid-file-created | | | |
| pid-file-removed | | | |
| signal-status-running | | | |
| signal-status-stopped | | | |
| signal-stop | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/main.go` | | |
| `cmd/ze/hub/main.go` | | |
| `internal/pidfile/pidfile.go` | | |
| `internal/pidfile/pidfile_test.go` | | |
| `cmd/ze/signal/main.go` | | |
| `cmd/ze/signal/main_test.go` | | |
| `test/signal/pid-file.ci` | | |
| `test/signal/pid-cleanup.ci` | | |
| `test/signal/status-running.ci` | | |
| `test/signal/status-stopped.ci` | | |
| `test/signal/stop.ci` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### 🏗️ Design
- [ ] No premature abstraction (3+ concrete use cases exist?)
- [ ] No speculative features (is this needed NOW?)
- [ ] Single responsibility (each component does ONE thing?)
- [ ] Explicit behavior (no hidden magic or conventions?)
- [ ] Minimal coupling (components isolated, dependencies minimal?)
- [ ] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs (last valid, first invalid above/below)
- [ ] Feature code integrated into codebase (`internal/*`, `cmd/*`)
- [ ] Functional tests verify end-user behavior (`.ci` files)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code
- [ ] RFC constraint comments added

### Completion
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit completed (all items have status + location)
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
