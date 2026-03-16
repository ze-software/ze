# Spec: zefs-socket-locking

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-03-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/ssh/ssh.go` - ze SSH server (Charm/Wish)
4. `internal/component/ssh/auth.go` - bcrypt password auth
5. `internal/component/ssh/session.go` - per-session CLI model
6. `internal/component/bgp/config/loader.go` - SSH config extraction + startup
7. `cmd/ze/config/cmd_edit.go` - editor (currently direct file access)
8. `cmd/ze/signal/main.go` - signal handling (currently PID file)

## Task

Remove the Unix socket, flock, .lock files, and PID files. SSH becomes the only interface to the daemon. All CLI tools become SSH clients. The daemon serializes all writes internally -- no cross-process locking needed.

**`ze init`** bootstraps the zefs database with SSH credentials (username, password, host:port) before any other command works. Accepts piped stdin or interactive prompts.

When no daemon is running, the editor starts an ephemeral daemon, connects via SSH, and stops it when done (lf pattern: first process becomes the server).

The SSH server supports multiple listen addresses (YANG `leaf-list`), so the daemon can bind both a local address (127.0.0.1:2222) and a public one simultaneously. CLI tools default to connecting to 127.0.0.1:2222. Environment variables override the target:

| Env var | Purpose |
|---------|---------|
| `ze.ssh.host` / `ze_ssh_host` | Override connection host (default 127.0.0.1) |
| `ze.ssh.port` / `ze_ssh_port` | Override connection port (default 2222) |

Example: `ze_ssh_host=10.0.0.1 ze_ssh_port=2222 ze config edit` connects to a remote daemon.

**Motivation:** One interface (SSH) instead of three mechanisms (Unix socket + flock + PID file). SSH already exists in ze with auth, YANG config, and CLI integration. The daemon is the single writer -- flock is solving a problem that does not exist.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - plugin process management
  -> Decision: SSH replaces Unix socket as the external interface
- [ ] `docs/learned/380-ssh-server.md` - SSH server decisions and patterns
  -> Constraint: Subsystem lifecycle, YANG schema registration, bcrypt auth

### Source Files
- [ ] `internal/component/ssh/ssh.go` (303L) - SSH server, Wish middleware chain, Start/Stop
  -> Constraint: implements ze.Subsystem interface; async serve in goroutine
- [ ] `internal/component/ssh/auth.go` (48L) - bcrypt auth, timing-safe user lookup
  -> Constraint: dummy hash for unknown users (timing side-channel prevention)
- [ ] `internal/component/ssh/session.go` (31L) - per-session unified CLI model
  -> Decision: SSH sessions already have full CLI access; no new dispatch needed
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` (65L) - SSH YANG config
  -> Decision: change `listen` from leaf to leaf-list for multiple addresses
  -> Constraint: host-key, idle-timeout, max-sessions under system/ssh
- [ ] `internal/component/bgp/config/loader.go` - extractSSHConfig, startup wiring
  -> Constraint: SSH server created in loader, executor factory wired post-reactor-start
- [ ] `internal/component/plugin/server/server.go` - StartWithContext, Unix socket listener
  -> Decision: remove Unix socket listener; SSH is the only external interface
- [ ] `pkg/zefs/flock_unix.go` (58L) - openLockFd, flock syscalls
  -> Decision: delete -- no cross-process locking needed
- [ ] `pkg/zefs/store.go` (552L) - BlobStore, Create/Open with lockFd
  -> Decision: remove lockFd; keep in-process RWMutex only
- [ ] `pkg/zefs/lock.go` (131L) - Lock/Release with flock + RWMutex
  -> Decision: remove flock calls; keep RWMutex for goroutine safety
- [ ] `internal/component/config/storage/storage.go` (170L) - AcquireLock creates .lock
  -> Decision: remove .lock file creation; daemon serializes writes
- [ ] `cmd/ze/config/cmd_edit.go` - runEditor, wireCommandExecutor, direct storage writes
  -> Decision: editor becomes SSH client; writes go through daemon
- [ ] `cmd/ze/signal/main.go` - PID file lookup, syscall.Kill
  -> Decision: send commands via SSH; no PID file, no kill()
- [ ] `cmd/ze/hub/main.go` - acquirePIDFile
  -> Decision: remove PID file logic entirely

**Key insights:**
- SSH server already exists with auth, YANG config, per-session CLI, command dispatch
- Daemon never writes to database.zefs currently; CLI tools do. Reversing this: daemon owns all writes
- SSH on localhost has negligible latency -- not a real concern
- Liveness detection: dial SSH port (TCP connect). Same as lf's socket dial but on TCP
- `ze init` solves the bootstrap problem: create database with credentials before anything else
- Two access paths to the bus: SSH (external) and in-process (plugins). Unix socket eliminated

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ssh/ssh.go` - SSH server with Wish, listens on configurable host:port (default 127.0.0.1:2222, single address currently)
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` - listen is currently a single leaf, needs leaf-list for multiple addresses
- [ ] `internal/component/ssh/auth.go` - bcrypt password auth with timing-safe lookup
- [ ] `internal/component/ssh/session.go` - creates unified CLI model per SSH session
- [ ] `internal/component/plugin/server/server.go:238-292` - StartWithContext creates Unix socket listener
- [ ] `pkg/zefs/flock_unix.go` - openLockFd(path) creates path+".lock", returns fd for flock
- [ ] `pkg/zefs/store.go` - Create/Open call openLockFd(s.path), store lockFd
- [ ] `pkg/zefs/lock.go` - Lock() flocks lockFd + RWMutex, Release() unlocks both
- [ ] `internal/component/config/storage/storage.go:111` - AcquireLock creates name+".lock", flocks it
- [ ] `cmd/ze/config/cmd_edit.go` - editor writes directly to storage, optionally notifies daemon via socket
- [ ] `cmd/ze/signal/main.go` - reads PID from .pid file, sends signal via syscall.Kill
- [ ] `cmd/ze/hub/main.go` - acquirePIDFile at daemon startup

**Behavior to preserve:**
- In-process RWMutex for goroutine safety within daemon
- Atomic flush (temp+rename) for crash safety
- SSH server auth, YANG config, session model, command dispatch
- Socket permissions concept (SSH handles via listen address + auth)
- WriteGuard interface (internal to BlobStore)

**Behavior to change:**
- Remove Unix socket listener from server.go
- Remove flock/lockFd from BlobStore (single writer, no cross-process locking)
- Remove .lock file creation from storage.go
- Remove PID file mechanism entirely
- Editor becomes SSH client (no direct file access)
- Signal commands sent via SSH (no PID lookup, no kill())
- Add `ze init` for bootstrap
- Add ephemeral daemon startup for editor-without-daemon
- Liveness detection via TCP dial to SSH port (replaces socket file check)

## Data Flow (MANDATORY)

### Entry Point

| Path | Entry | Transport | Auth |
|------|-------|-----------|------|
| Remote user | SSH client | TCP to host:port | bcrypt password |
| Local CLI (`ze config edit`) | SSH client | TCP to 127.0.0.1:port | bcrypt password |
| `ze init` | Direct file creation | None (pre-daemon) | None (bootstrap) |
| Plugin (in-process) | Direct bus access | In-process | N/A |

### Transformation Path
1. CLI tool resolves SSH target: env vars (`ze_ssh_host`, `ze_ssh_port`) override default (127.0.0.1:2222)
2. CLI tool reads SSH credentials from zefs database (username, password)
3. CLI connects via SSH to daemon at resolved target
4. Daemon receives command through SSH session's CLI model
5. Daemon dispatches command via reactor's Dispatcher
6. For writes: daemon writes to database.zefs (single writer, RWMutex only)
7. Response returned through SSH session

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI tool -> Daemon | SSH connection (TCP) | [ ] |
| SSH session -> Dispatcher | CLI model command dispatch (in-process) | [ ] |
| Dispatcher -> Storage | Direct write (single writer, RWMutex) | [ ] |

### Integration Points
- SSH server already wired to reactor Dispatcher via CommandExecutorFactory
- CLI tools need SSH client library to connect to daemon
- `ze init` needs direct zefs write (only pre-daemon command)
- Ephemeral daemon reuses existing SSH server startup path

### Architectural Verification
- [ ] No bypassed layers (all external access through SSH)
- [ ] No unintended coupling (CLI tools depend only on SSH, not on internals)
- [ ] No duplicated functionality (SSH replaces socket + flock + PID)
- [ ] Single writer guaranteed (daemon process only)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze init` (piped stdin) | -> | Creates zefs with SSH credentials | `test/plugin/ze-init-piped.ci` |
| `ze config edit` via SSH | -> | Editor connects to daemon, edits config | `test/plugin/config-edit-ssh.ci` |
| `ze signal stop` via SSH | -> | Sends stop command via SSH | `test/plugin/signal-stop-ssh.ci` |
| `ze status` via SSH | -> | Dials SSH port, reports status | `test/plugin/status-ssh.ci` |
| Editor without daemon | -> | Starts ephemeral daemon, connects | `test/plugin/config-edit-no-daemon.ci` |
| Stale port detection | -> | Dead port detected, ephemeral daemon started | `TestStalePortDetection` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze init` with piped stdin (username, password, host, port) | Creates zefs database with SSH credentials |
| AC-2 | `ze init` without piped input | Prompts interactively for username, password, host, port |
| AC-3 | Any `ze` command before `ze init` | Fails with clear error directing user to run `ze init` |
| AC-4 | `ze config edit` with daemon running | Connects via SSH, edits config through daemon |
| AC-5 | `ze config edit` without daemon | Starts ephemeral daemon, connects via SSH, stops daemon on exit |
| AC-6 | `ze signal stop` with daemon running | Sends stop command via SSH, daemon shuts down |
| AC-7 | `ze signal reload` with daemon running | Sends reload command via SSH, daemon reloads config |
| AC-8 | `ze status` with daemon running | Dials SSH port, reports running |
| AC-9 | `ze status` without daemon | Dial fails, reports not running |
| AC-10 | `ls *.lock *.pid` after any operation | No .lock or .pid files exist |
| AC-11 | Two `ze config edit` sessions simultaneously | Both connect via SSH, daemon serializes writes |
| AC-12 | Daemon starts, SSH port already in use | Fails with clear error (port conflict) |
| AC-13 | Ephemeral daemon starts, SSH port already in use | Fails with clear error (another process on port) |
| AC-14 | SSH server with two listen addresses in config | Daemon binds both (e.g., 127.0.0.1:2222 and 0.0.0.0:2222) |
| AC-15 | `ze_ssh_host=10.0.0.1 ze config edit` | CLI connects to 10.0.0.1 instead of 127.0.0.1 |
| AC-16 | `ze_ssh_port=3333 ze status` | CLI dials port 3333 instead of 2222 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestZeInitPipedStdin` | `cmd/ze/init/main_test.go` | Creates zefs from piped username/password/host/port | |
| `TestZeInitInteractive` | `cmd/ze/init/main_test.go` | Prompts and creates zefs from interactive input | |
| `TestZeInitAlreadyExists` | `cmd/ze/init/main_test.go` | Refuses to overwrite existing database (or prompts) | |
| `TestBlobStoreNoFlock` | `pkg/zefs/store_test.go` | No .lock file created, RWMutex still works | |
| `TestStalePortDetection` | `cmd/ze/config/cmd_edit_test.go` | Dead SSH port detected, ephemeral daemon started | |
| `TestLivePortDetection` | `cmd/ze/config/cmd_edit_test.go` | Live SSH port detected, connects as client | |
| `TestNoUnixSocket` | `internal/component/plugin/server/server_test.go` | Server starts without Unix socket listener | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ze-init-piped` | `test/plugin/ze-init-piped.ci` | Pipe credentials, verify database created | |
| `config-edit-ssh` | `test/plugin/config-edit-ssh.ci` | Edit config through daemon via SSH | |
| `signal-stop-ssh` | `test/plugin/signal-stop-ssh.ci` | Stop daemon via SSH command | |
| `status-ssh` | `test/plugin/status-ssh.ci` | Check daemon status via SSH port dial | |
| `config-edit-no-daemon` | `test/plugin/config-edit-no-daemon.ci` | Editor starts ephemeral daemon, edits, stops | |

## Files to Modify
- `pkg/zefs/store.go` - Remove lockFd field, remove openLockFd calls from Create/Open
- `pkg/zefs/lock.go` - Remove flock calls, keep RWMutex only
- `internal/component/config/storage/storage.go` - Remove .lock file creation from AcquireLock
- `internal/component/plugin/server/server.go` - Remove Unix socket listener from StartWithContext
- `internal/component/ssh/schema/ze-ssh-conf.yang` - Change `listen` from leaf to leaf-list
- `internal/component/ssh/ssh.go` - Support multiple listen addresses (one listener per address)
- `internal/component/bgp/config/loader.go` - Parse leaf-list listen into slice of addresses
- `cmd/ze/config/cmd_edit.go` - Editor becomes SSH client; env var target resolution; ephemeral daemon
- `cmd/ze/signal/main.go` - Send commands via SSH instead of PID file + kill(); env var target resolution
- `cmd/ze/hub/main.go` - Remove acquirePIDFile, remove pidfile import

### Files to Delete
- `pkg/zefs/flock_unix.go` - No longer needed (no cross-process flock)
- `pkg/zefs/flock_other.go` - No longer needed (no-op stubs for flock)
- `internal/core/pidfile/pidfile.go` - Replaced by SSH liveness detection
- `internal/core/pidfile/pidfile_test.go` - Tests for deleted code

### Files to Create
- `cmd/ze/init/main.go` - `ze init` command (bootstrap zefs with SSH credentials)
- `cmd/ze/init/main_test.go` - Tests for ze init

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | No new RPCs (SSH YANG already exists) |
| CLI commands/flags | [x] | `cmd/ze/init/main.go` (new `ze init` subcommand) |
| CLI dispatch | [x] | `cmd/ze/main.go` (register `init` subcommand) |
| SSH client library | [x] | CLI tools need `golang.org/x/crypto/ssh` client |
| SSH credential storage in zefs | [x] | `ze init` writes credentials; CLI tools read them |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: ze init** - Bootstrap command creating zefs with SSH credentials
   - Tests: `TestZeInitPipedStdin`, `TestZeInitInteractive`, `TestZeInitAlreadyExists`
   - Files: `cmd/ze/init/main.go`, `cmd/ze/main.go` (dispatch)
   - Verify: piped stdin creates database; interactive prompts work; existing database handled
2. **Phase: Remove flock** - Strip cross-process locking from BlobStore and storage
   - Tests: `TestBlobStoreNoFlock`
   - Files: delete `flock_unix.go`, `flock_other.go`; modify `store.go`, `lock.go`, `storage.go`
   - Verify: no .lock files created; in-process RWMutex still works
3. **Phase: Remove Unix socket** - SSH is the only external interface
   - Tests: `TestNoUnixSocket`
   - Files: `server.go`
   - Verify: server starts without socket listener; SSH still works
4. **Phase: CLI as SSH client** - Editor, signal, status become SSH clients
   - Tests: `TestStalePortDetection`, `TestLivePortDetection`
   - Files: `cmd_edit.go`, `signal/main.go`
   - Verify: CLI connects via SSH; commands dispatched through daemon
5. **Phase: Remove PID file** - Delete pidfile package, remove from hub
   - Files: delete `internal/core/pidfile/`; modify `cmd/ze/hub/main.go`
   - Verify: no PID files created; liveness via SSH port dial
6. **Phase: Ephemeral daemon** - Editor starts temporary daemon when none running
   - Tests: functional test
   - Files: `cmd_edit.go`
   - Verify: editor starts daemon, connects, edits, stops daemon on exit
7. **Functional tests** - Create after feature works. Cover user-visible behavior.
8. **Full verification** - `make ze-verify`
9. **Complete spec** - Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | No .lock, .pid, or socket files created anywhere (grep for ".lock", ".pid", "unix" in Listen calls) |
| Correctness | All CLI commands work via SSH; daemon serializes writes |
| No flock remnants | grep flock across all .go files finds nothing outside tests |
| No socket remnants | grep for Unix socket creation in server.go finds nothing |
| SSH client | CLI tools connect via SSH with credentials from zefs |
| Bootstrap | `ze init` creates valid database; other commands fail without it |
| Ephemeral daemon | Editor starts/stops daemon cleanly; port released on exit |
| Rule: no-layering | Old mechanisms (socket, flock, PID) fully deleted, not coexisting |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `ze init` command exists | `go build ./cmd/ze/ && ze init --help` |
| No .lock files after operations | `find /tmp -name "*.lock" -path "*ze*"` finds nothing |
| No .pid files after operations | `find /tmp -name "*.pid" -path "*ze*"` finds nothing |
| No Unix socket file | `find /run -name "ze.socket"` finds nothing |
| flock_unix.go deleted | `ls pkg/zefs/flock_unix.go` fails |
| pidfile package deleted | `ls internal/core/pidfile/` fails |
| CLI uses SSH | grep for SSH client connection in `cmd/ze/config/`, `cmd/ze/signal/` |
| ze signal uses SSH | grep Kill cmd/ze/signal/ finds nothing (no syscall.Kill) |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Credentials in zefs | SSH password stored as bcrypt hash, not plaintext |
| SSH client auth | CLI tools authenticate with stored credentials, not hardcoded |
| Ephemeral daemon | Binds to localhost only (127.0.0.1), not 0.0.0.0 |
| Port conflict | Clear error when SSH port already in use, no silent fallback |
| `ze init` stdin | Password not echoed to terminal during interactive prompt |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| SSH connection refused | Check daemon startup, port binding, credentials |
| Ephemeral daemon fails to start | Check port availability, SSH config |
| 3 fix attempts fail | STOP. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| flock on store file works | Atomic rename changes inode, breaks flock | Traced flush() code path | Led to .lock file design |
| Socket coordination needs new RPC protocol | Just flock the socket file instead | User pointed out simplicity | Spec v1 was massively overengineered |
| Cross-process flock needed on socket file | Single writer (daemon) needs no cross-process locking | User pointed out daemon serializes | Spec v2 still had unnecessary flock |
| Unix socket needed alongside SSH | SSH already exists as full interface; socket is redundant | User asked "how do you edit via SSH?" | Spec v2 had two interfaces instead of one |
| SSH on localhost has latency | Negligible -- not a real concern | User called it out | False objection to SSH-only design |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Socket ownership with editor-as-server | Massively overengineered; new RPC protocol, new package, 9 phases | Flock the socket file (v2) |
| Flock the socket file | Still solving a non-problem; daemon is single writer, no flock needed | SSH-only interface (v3) |
| SSH + Unix socket (two interfaces) | Socket is redundant when SSH already exists with auth and CLI | SSH-only (v3) |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Inflating simple changes into architectures | 3x this spec | "Can this be done by removing something?" before adding | Add to memory |
| Inventing coordination where single-writer suffices | 2x this spec | "Who writes? If one process, no locking needed" | Add to memory |
| Adding interfaces instead of using existing ones | 1x this spec | "Does an interface already exist for this?" before creating | Add to memory |

## Design Insights

- Single writer eliminates all locking. If the daemon is the only process that writes, flock/.lock files are solving a non-problem.
- The lf pattern is not "flock the socket" -- it is "one server process, clients connect." Ze already has this with SSH.
- SSH on localhost is not slow. Do not invent alternative transports for local use.
- `ze init` solves the bootstrap problem cleanly: one command before anything else works.

## RFC Documentation

N/A

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:**
- **Skipped:**
- **Changed:**

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
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
