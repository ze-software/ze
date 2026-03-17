# Spec: plugin-tcp-transport

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/process-protocol.md` - plugin process management
4. `internal/component/plugin/process/process.go` - process lifecycle
5. `internal/component/plugin/ipc/socketpair.go` - current socket pair creation
6. `internal/component/plugin/server/startup.go` - 5-stage handshake

## Task

Add a TCP connect-back transport for plugins alongside the existing internal (net.Pipe) and external (socketpair + exec) modes. A plugin running anywhere that can open a TCP connection to the engine can participate in the standard 5-stage handshake and event loop. This removes the hard dependency on Unix socketpairs and fd passing, enabling plugins that run outside the local process tree (remote hosts, containers, WASI runtimes).

### Motivation

- Unix socketpairs and SCM_RIGHTS fd passing are not available on all platforms (WASI, Windows)
- External subprocess plugins require `os/exec` which is not available under WASI
- Remote plugin hosting (different machine, container, cloud) is not possible with socketpairs
- TCP connect-back uses only `net.Dial` / `net.Listen` -- available in Go stdlib on all platforms including WASI preview 2

### Non-Goals

- SSH transport (future spec, builds on this one)
- Replacing internal plugins (`net.Pipe` stays for goroutine-based plugins)
- Replacing local subprocess plugins (socketpair stays for fork-based plugins)
- WASI compilation itself (separate spec: `spec-wasi-support.md`)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - plugin process management
  → Constraint: 5-stage handshake protocol is fixed; transport must not alter the protocol
  → Constraint: Socket A = plugin-to-engine RPCs, Socket B = engine-to-plugin callbacks
- [ ] `docs/architecture/core-design.md` - system architecture overview
  → Decision: plugins communicate via `net.Conn`; transport is already abstracted

### RFC Summaries (MUST for protocol work)
- Not protocol work (internal transport, not BGP wire format)

**Key insights:**
- All RPC code uses `net.Conn` interface -- any transport that provides `net.Conn` slots in
- Process struct has `rawEngineA` and `rawCallbackB` fields set by `SetRawConns(net.Conn, net.Conn)` -- transport-agnostic
- `InitConns()` peeks first byte for JSON/text mode detection -- works on any `net.Conn`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/process/process.go` - process lifecycle, `startInternal()` / `startExternal()`, `SetRawConns()`, `InitConns()`
  → Constraint: `SetRawConns(engineA, callbackB net.Conn)` is the interface between transport and protocol
  → Constraint: `InitConns()` peeks mode from Socket A, creates `PluginConn` or returns raw for text mode
- [ ] `internal/component/plugin/ipc/socketpair.go` - `NewInternalSocketPairs()` (net.Pipe), `NewExternalSocketPairs()` (syscall.Socketpair), `PluginFiles()` (fd extraction for exec)
  → Constraint: `DualSocketPair` struct holds engine-side and plugin-side `net.Conn` per socket
- [ ] `internal/component/plugin/ipc/fdpass.go` - `SendFD()` / `ReceiveFD()` via SCM_RIGHTS -- only used for external plugins
- [ ] `internal/component/plugin/ipc/rpc.go` - `PluginConn` wraps `rpc.Conn`, created from `net.Conn`
- [ ] `internal/component/plugin/server/startup.go` - 5-stage handshake, tier-based barrier
- [ ] `internal/component/plugin/types.go` - `PluginConfig` struct: `Name`, `Run`, `Internal` bool, `Encoder`, `StageTimeout`
- [ ] `internal/component/ssh/ssh.go` - SSH server on port 2222, uses Wish/Charmbracelet, unrelated to plugin IPC
- [ ] `pkg/plugin/rpc/conn.go` - `Conn` struct, NUL-framed JSON RPC over `net.Conn`
- [ ] `pkg/plugin/rpc/text_conn.go` - `TextConn`, newline-framed text RPC over `net.Conn`

**Behavior to preserve:**
- Internal plugins use `net.Pipe()` -- unchanged
- External subprocess plugins use `socketpair()` + `exec.Command` -- unchanged
- 5-stage handshake protocol (declare-registration, configure, declare-capabilities, share-registry, ready)
- JSON-RPC framing (NUL-delimited) and text-mode framing (newline-delimited)
- Protocol mode detection via first-byte peek on Socket A
- `DirectBridge` for internal plugins (hot path bypass)
- Tier-based barrier synchronization for plugin startup ordering

**Behavior to change:**
- Add TCP listener in engine for plugin connect-back
- Add `startTCP()` path in Process alongside `startInternal()` / `startExternal()`
- Add auth frame as first message before 5-stage handshake
- Add session pairing to match two TCP connections into one `DualSocketPair`
- Extend `PluginConfig` with TCP connect-back fields

## Data Flow (MANDATORY)

### Entry Point
- Plugin initiates: `net.Dial("tcp", engineAddr)` twice (once for Socket A, once for Socket B)
- First frame on each connection: auth + channel identification
- Engine accepts on TCP listener, authenticates, pairs connections by session ID

### Transformation Path
1. Engine starts TCP listener on configured port (e.g., `127.0.0.1:9179`)
2. Plugin opens first TCP connection, sends auth frame: `{"auth":"pluginname","pass":"secret","channel":"engine"}`
3. Engine validates credentials, assigns session UUID, responds: `{"ok":true,"session":"<uuid>"}`
4. Plugin opens second TCP connection, sends: `{"auth":"pluginname","pass":"secret","channel":"callback","session":"<uuid>"}`
5. Engine matches by session UUID, responds: `{"ok":true}`
6. Engine calls `process.SetRawConns(tcpConnA, tcpConnB)` -- from here, identical to socketpair path
7. Normal 5-stage handshake proceeds on the two `net.Conn`s
8. Event delivery uses same `PluginConn` / `TextConn` / batching pipeline

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network ↔ Engine | TCP accept + auth frame | [ ] |
| Engine ↔ Process | `SetRawConns(net.Conn, net.Conn)` -- same as today | [ ] |
| Process ↔ Plugin | RPC over `net.Conn` -- same as today | [ ] |

### Integration Points
- `process.go`: new `startTCP()` method alongside `startInternal()` / `startExternal()`
- `server/startup.go`: no changes -- receives `Process` with `rawEngineA`/`rawCallbackB` already set
- `types.go`: `PluginConfig` gains TCP fields
- New: TCP listener + auth handler + session pairing logic

### Architectural Verification
- [ ] No bypassed layers -- TCP connections feed into same `SetRawConns` entry point
- [ ] No unintended coupling -- TCP listener is optional, only started if TCP plugins configured
- [ ] No duplicated functionality -- reuses all existing RPC, handshake, and delivery code
- [ ] Zero-copy preserved -- TCP `net.Conn` supports same read/write patterns

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin config with `tcp://` address | -> | TCP listener starts, plugin connects, 5-stage completes | `test/plugin/tcp-connect-back.ci` |
| Plugin connects with wrong password | -> | Engine rejects, connection closed | `test/plugin/tcp-connect-back-auth-fail.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin config contains `tcp://user:pass@host:port` | Engine starts TCP listener on specified port |
| AC-2 | Plugin connects with correct credentials | Auth succeeds, session UUID assigned, both channels paired |
| AC-3 | Plugin connects with wrong credentials | Connection rejected with error, no session created |
| AC-4 | Both TCP connections paired by session UUID | `SetRawConns()` called, 5-stage handshake proceeds normally |
| AC-5 | TCP plugin completes 5-stage handshake | Plugin reaches Running state, receives events, can send RPCs |
| AC-6 | TCP plugin disconnects | Engine detects closed connection, cleans up Process |
| AC-7 | Second connection does not arrive within timeout | First connection cleaned up, session expired |
| AC-8 | No TCP plugins configured | No TCP listener started |
| AC-9 | TCP listener bind fails (port in use) | Engine logs error, continues without TCP plugins |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTCPAuthSuccess` | `internal/component/plugin/ipc/tcp_test.go` | Auth frame parsing and validation | |
| `TestTCPAuthWrongPass` | `internal/component/plugin/ipc/tcp_test.go` | Rejection on bad credentials | |
| `TestTCPSessionPairing` | `internal/component/plugin/ipc/tcp_test.go` | Two connections matched by session UUID | |
| `TestTCPSessionTimeout` | `internal/component/plugin/ipc/tcp_test.go` | Unpaired connection cleaned up after timeout | |
| `TestTCPPluginHandshake` | `internal/component/plugin/process/process_tcp_test.go` | Full 5-stage handshake over TCP connections | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| TCP port | 1-65535 | 65535 | 0 | 65536 |
| Session pairing timeout | 1s-60s | 60s | 0s | N/A (capped) |
| Auth frame max size | 1-4096 bytes | 4096 | 0 | 4097 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `tcp-connect-back` | `test/plugin/tcp-connect-back.ci` | Plugin connects via TCP, exchanges messages | |
| `tcp-connect-back-auth-fail` | `test/plugin/tcp-connect-back-auth-fail.ci` | Plugin with wrong password is rejected | |

### Future (if deferring any tests)
- SSH transport tests (separate spec)
- Multi-plugin TCP tests (multiple plugins connecting simultaneously)
- Network partition / reconnect behavior

## Files to Modify
- `internal/component/plugin/types.go` - add TCP fields to `PluginConfig`
- `internal/component/plugin/process/process.go` - add `startTCP()` method
- `internal/component/plugin/server/startup.go` - handle TCP plugin startup (if any changes needed beyond `SetRawConns`)
- `pkg/plugin/sdk/sdk.go` - add TCP dial mode for plugin SDK

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | Not needed -- no new RPCs, transport-level only |
| RPC count in architecture docs | [ ] | No |
| CLI commands/flags | [ ] | No -- config-driven |
| CLI usage/help text | [ ] | No |
| API commands doc | [ ] | No |
| Plugin SDK docs | [x] | `.claude/rules/plugin-design.md` -- add TCP mode |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/tcp-connect-back.ci` |

## Files to Create
- `internal/component/plugin/ipc/tcp.go` - TCP listener, auth handler, session pairing
- `internal/component/plugin/ipc/tcp_test.go` - unit tests for TCP auth and pairing
- `internal/component/plugin/process/process_tcp_test.go` - TCP handshake integration test
- `test/plugin/tcp-connect-back.ci` - functional test
- `test/plugin/tcp-connect-back-auth-fail.ci` - functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
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

1. **Phase: TCP auth and session pairing** -- TCP listener, auth frame protocol, session UUID matching
   - Tests: `TestTCPAuthSuccess`, `TestTCPAuthWrongPass`, `TestTCPSessionPairing`, `TestTCPSessionTimeout`
   - Files: `internal/component/plugin/ipc/tcp.go`, `tcp_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Process TCP startup** -- `startTCP()` in process.go, `PluginConfig` TCP fields
   - Tests: `TestTCPPluginHandshake`
   - Files: `process/process.go`, `types.go`, `process/process_tcp_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: SDK TCP dial** -- plugin-side TCP connection mode
   - Tests: integration with phase 2 test
   - Files: `pkg/plugin/sdk/sdk.go`
   - Verify: tests fail -> implement -> tests pass

4. **Functional tests** -- `.ci` tests for end-to-end TCP connect-back
   - Files: `test/plugin/tcp-connect-back.ci`, `test/plugin/tcp-connect-back-auth-fail.ci`

5. **Full verification** -- `make ze-verify`

6. **Complete spec** -- audit tables, learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Auth frame validated before any protocol processing; session timeout cleans up |
| Naming | Auth frame JSON keys use kebab-case |
| Data flow | TCP connections feed into `SetRawConns()` -- no parallel path |
| Rule: no-layering | TCP is additive (new mode), not replacing existing modes |
| Rule: goroutine-lifecycle | TCP listener is long-lived goroutine, not per-connection spawn for accept loop |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `ipc/tcp.go` exists | `ls internal/component/plugin/ipc/tcp.go` |
| TCP auth unit tests pass | `go test -run TestTCPAuth ./internal/component/plugin/ipc/` |
| Session pairing unit tests pass | `go test -run TestTCPSession ./internal/component/plugin/ipc/` |
| Functional test exists | `ls test/plugin/tcp-connect-back.ci` |
| `PluginConfig` has TCP fields | `grep -n 'tcp\|TCP\|ConnectAddr' internal/component/plugin/types.go` |
| Plugin design rules updated | `grep -n 'tcp' .claude/rules/plugin-design.md` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Auth frame: max size (4096), valid JSON, required fields present |
| Credential handling | Password not logged, not stored in Process struct after auth |
| Bind address | Default to `127.0.0.1` (localhost only), not `0.0.0.0` |
| Session ID | Cryptographically random UUID, not sequential |
| Timeout | Unpaired connections cleaned up; auth frame must arrive within timeout |
| DoS | Rate limit on auth failures per source IP; max pending sessions |
| No credential in config plaintext | Consider env var or file reference for password |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
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

## RFC Documentation

Not applicable -- internal transport, not BGP protocol.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-9 all demonstrated
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
