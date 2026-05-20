# Spec: graceful-listener-migration

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/web/server.go` - WebServer, ListenAndServe, Shutdown
4. `cmd/ze/hub/main_reload.go` - doReload, handleSIGHUPReload
5. `cmd/ze/hub/main.go` - service startup, signal handling
6. `internal/component/config/loader_extract.go` - ExtractWebConfig, ServerEndpoint

## Task

When the listen address (IP or port) of a service changes on config reload (SIGHUP
or commit), the daemon currently requires a full restart. Instead, the daemon should
bind the new listener(s) before closing the old one(s), achieving zero-downtime
migration when there is no address conflict.

When two services swap addresses (e.g., web moves from :3443 to :8080 while API
moves from :8080 to :3443), the coordinator must detect the conflict and sequence
the releases so only the conflicting address experiences a brief gap.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/web-interface.md` - web server infrastructure
  -> Constraint: WebServer owns transport (TLS, listen, serve) but not application logic
- [ ] `docs/architecture/hub-architecture.md` - SIGHUP config reload orchestration
  -> Constraint: reload is lock-step from a SINGLE tree snapshot

### RFC Summaries (MUST for protocol work)
N/A - no protocol work.

**Key insights:**
- `http.Server.Serve(ln)` can be called multiple times concurrently on the same server; each call tracks its own listener
- Closing a specific `net.Listener` causes only that listener's `Serve` goroutine to exit; others are unaffected
- `http.Server.Shutdown()` closes ALL tracked listeners and drains connections; it cannot selectively close one listener
- The current `doReload` path does not touch any listener; web/SSH/MCP/LG/API servers are start-once
- Six services bind listeners: web, SSH, MCP, LG, API (REST + gRPC), L2TP

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/server.go` - WebServer struct, ListenAndServe (all-or-nothing bind, Serve per listener, Shutdown closes all)
  -> Constraint: `configured`/`bound` fields are set once at startup, never updated
  -> Constraint: listeners are not stored after ListenAndServe; only bound addresses are kept
- [ ] `cmd/ze/hub/main_reload.go` - doReload calls plugin reload + provider refresh + engine.Reload; never touches servers
  -> Constraint: reload sequence is: load -> plugin-apply -> provider-refresh -> subsystem-reload -> apply tuning
- [ ] `cmd/ze/hub/main.go` - startWebServer/startLGServer/startMCPServer called once at startup; Shutdown in cleanup
- [ ] `cmd/ze/hub/main_servers.go` - startWebServer creates WebServer, registers routes, launches ListenAndServe in goroutine
- [ ] `cmd/ze/hub/api.go` - startAPIServers creates REST and/or gRPC servers with ListenAddrs
- [ ] `cmd/ze/hub/mcp.go` - startMCPServer creates HTTP server with multiple listeners
- [ ] `internal/component/config/loader_extract.go` - ServerEndpoint{Host, Port}, extractServerList, ExtractWebConfig, ExtractMCPConfig, ExtractAPIConfig
  -> Constraint: ServerEndpoint.Listen() returns "host:port" string

**Behavior to preserve:**
- All-or-nothing semantics for initial startup (if any bind fails, none serve)
- TLS configuration (cert/key) is unchanged by listener migration
- Route handler state (mux, sessions, SSE broker) survives migration
- Shutdown still closes all listeners and drains active connections
- `Addresses()` / `Address()` return current bound addresses

**Behavior to change:**
- Listeners must be stored (not discarded) so they can be selectively closed
- New listeners can be added to a running server
- Old listeners can be removed from a running server
- `doReload` must detect listener config changes and trigger migration

## Data Flow (MANDATORY)

### Entry Point
- SIGHUP signal or config commit triggers `doReload`
- `doReload` calls `load()` to get the new config tree
- New config tree is compared against current listener state

### Transformation Path
1. `doReload` extracts new listen configs from tree (ExtractWebConfig, etc.)
2. Hub coordinator collects current bound addresses from all running servers
3. Per-service diff: compute keep/add/remove sets
4. Cross-service conflict detection: any address in one service's remove set AND another's add set
5. Phase 1: non-conflicting services call `Reconfigure(newAddrs)` (bind new, then close old)
6. Phase 2: conflicting pairs sequenced (close old on releasing service, then bind new on acquiring service)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree -> listen addrs | ExtractWebConfig / ExtractMCPConfig / etc. | [ ] |
| Hub coordinator -> WebServer | Reconfigure(ctx, newAddrs) method call | [ ] |
| WebServer -> net.Listener | Direct listener close + new Serve goroutine | [ ] |

### Integration Points
- `doReload` in `cmd/ze/hub/main_reload.go` - add listener migration step after provider refresh
- `WebServer` in `internal/component/web/server.go` - add Reconfigure method
- `Engine.Reload` subsystems (L2TP, PPPoE) - already have Reload; may need listener migration

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP with changed web listen addr | -> | WebServer.Reconfigure | `TestWebServerReconfigure` |
| SIGHUP with changed web listen addr (hub) | -> | reloadListeners in doReload | `TestDoReloadChangesWebListener` |
| Config commit with changed web listen addr | -> | reloadListeners via reloadAfterCommit | `TestCommitReloadChangesWebListener` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | WebServer.Reconfigure called with one new addr, one removed addr | New addr is bound and serving before old addr is closed; old addr closed after |
| AC-2 | WebServer.Reconfigure called with addr that fails to bind | Error returned; all original listeners still serving; no partial state |
| AC-3 | WebServer.Reconfigure called with same addrs as current | No-op; no listeners closed or opened |
| AC-4 | WebServer.Reconfigure called with additional addr | New addr added; existing addrs unchanged |
| AC-5 | WebServer.Reconfigure called with fewer addrs | Removed addr closed; remaining addrs unchanged |
| AC-6 | Addresses() returns correct addrs after Reconfigure | Reflects the new set, not the old |
| AC-7 | Active connections on a removed listener drain gracefully | In-flight requests complete before listener goroutine exits |
| AC-8 | doReload detects web listen addr change and calls Reconfigure | Web server migrates to new addr on SIGHUP |
| AC-9 | Two services swap addresses (web :3443->:8080, API :8080->:3443) | Coordinator detects conflict, sequences releases, both end up on correct addr |
| AC-10 | Service disabled on reload (web enabled->false) | Web server shut down entirely |
| AC-11 | Service enabled on reload (web false->true) | Web server started fresh |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWebServerReconfigureAddListener` | `internal/component/web/server_test.go` | AC-4 | |
| `TestWebServerReconfigureRemoveListener` | `internal/component/web/server_test.go` | AC-5, AC-7 | |
| `TestWebServerReconfigureSwapListener` | `internal/component/web/server_test.go` | AC-1 | |
| `TestWebServerReconfigureBindFails` | `internal/component/web/server_test.go` | AC-2 | |
| `TestWebServerReconfigureNoop` | `internal/component/web/server_test.go` | AC-3 | |
| `TestWebServerReconfigureAddresses` | `internal/component/web/server_test.go` | AC-6 | |
| `TestListenerDiffKeepAddRemove` | `internal/component/web/server_test.go` | diff computation | |
| `TestConflictDetection` | `cmd/ze/hub/listener_migrate_test.go` | AC-9 | |
| `TestConflictDetectionNoConflict` | `cmd/ze/hub/listener_migrate_test.go` | no false positives | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-web-listener-migration` | `test/web/listener-migration.ci` | SIGHUP changes web port, verify new port serves | |

### Future (if deferring any tests)
- SSH/MCP/LG/API Reconfigure methods are structurally identical to web; test web thoroughly, extend to others in follow-up spec
- L2TP uses UDP, different listener model; separate spec

## Files to Modify
- `internal/component/web/server.go` - add listener tracking map, Reconfigure method, update ListenAndServe
- `internal/component/web/server_test.go` - unit tests for Reconfigure
- `cmd/ze/hub/main_reload.go` - add listener migration step to doReload
- `cmd/ze/hub/listener_migrate.go` - new: cross-service conflict detection and sequencing
- `cmd/ze/hub/listener_migrate_test.go` - new: conflict detection tests
- `cmd/ze/hub/main.go` - store server references for reload access

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/web/listener-migration.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/web-interface.md` - document Reconfigure lifecycle |

## Files to Create
- `cmd/ze/hub/listener_migrate.go` - cross-service conflict detection and migration orchestration
- `cmd/ze/hub/listener_migrate_test.go` - tests for conflict detection
- `test/web/listener-migration.ci` - functional test

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

1. **Phase: Wiring (MANDATORY FIRST)** - listener tracking and Reconfigure skeleton
   - Tests: `TestWebServerReconfigureNoop`, `TestWebServerReconfigureAddresses`
   - Files: `internal/component/web/server.go` (add listener map, stub Reconfigure)
   - Verify: Reconfigure exists and is callable; wiring test fails because migration logic is stubbed

2. **Phase: Listener diff** - compute keep/add/remove sets from old and new address lists
   - Tests: `TestListenerDiffKeepAddRemove`
   - Files: `internal/component/web/server.go` (listenerDiff function)
   - Verify: diff correctly classifies addresses into three sets

3. **Phase: WebServer.Reconfigure** - bind new, close old, update state
   - Tests: `TestWebServerReconfigureAddListener`, `TestWebServerReconfigureRemoveListener`, `TestWebServerReconfigureSwapListener`, `TestWebServerReconfigureBindFails`
   - Files: `internal/component/web/server.go`
   - Verify: all unit tests pass; active connections on removed listeners drain

4. **Phase: Cross-service conflict detection** - collect all service addresses, detect swaps
   - Tests: `TestConflictDetection`, `TestConflictDetectionNoConflict`
   - Files: `cmd/ze/hub/listener_migrate.go`, `cmd/ze/hub/listener_migrate_test.go`
   - Verify: swap between two services detected; independent changes not flagged

5. **Phase: Hub integration** - wire into doReload
   - Tests: `TestDoReloadChangesWebListener`
   - Files: `cmd/ze/hub/main_reload.go`, `cmd/ze/hub/main.go`
   - Verify: SIGHUP with changed web config triggers Reconfigure

6. **Functional tests** - end-to-end listener migration
7. **Full verification** - `make ze-verify`
8. **Complete spec** - learned summary, spec closure

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Bind-before-close ordering guaranteed; no race between close and re-bind |
| Naming | Reconfigure, listenerDiff follow existing ze naming |
| Data flow | Config tree -> ExtractWebConfig -> Reconfigure -> net.Listener lifecycle |
| Rule: no-layering | No new abstraction layer; Reconfigure is a method on existing WebServer |
| Rule: rollback | Failed Reconfigure leaves server in original state |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| WebServer.Reconfigure method | `grep -n 'func.*WebServer.*Reconfigure' internal/component/web/server.go` |
| Listener tracking map in WebServer | `grep -n 'listeners.*map' internal/component/web/server.go` |
| listenerDiff function | `grep -n 'func listenerDiff' internal/component/web/server.go` |
| Cross-service conflict detection | `grep -n 'func.*detectConflicts' cmd/ze/hub/listener_migrate.go` |
| doReload calls listener migration | `grep -n 'reloadListeners\|Reconfigure' cmd/ze/hub/main_reload.go` |
| Unit tests pass | `go test ./internal/component/web/ -run TestWebServerReconfigure -v` |
| Conflict tests pass | `go test ./cmd/ze/hub/ -run TestConflict -v` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | newAddrs validated same as initial ListenAddrs (non-empty, no empty strings) |
| Race conditions | Reconfigure must be serialized (mutex); concurrent Reconfigure calls must not corrupt listener map |
| Resource leaks | Every opened listener must be closed on error path; goroutines must exit when listener closes |
| Bind escalation | Reconfigure must not bypass insecure mode (127.0.0.1 enforcement) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
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

### Key design decision: close listener directly, not via http.Server.Shutdown

`http.Server.Shutdown()` is a terminal operation: it sets `shuttingDown` permanently,
drains all connections, and any future `Serve()` call returns `ErrServerClosed`
immediately. This makes it unusable for selective listener removal.

Instead, close the `net.Listener` directly. This causes only that listener's
`Accept()` loop to return an error, exiting that `Serve` goroutine. The
`*http.Server` remains healthy for other listeners and new `Serve()` calls.

In-flight connections on the closed listener are NOT affected by listener close
(they are tracked by the server's connection tracking, independent of the listener).
They will be drained if/when the server eventually shuts down, or will naturally
close when the request completes.

### Key design decision: per-service Reconfigure, not a global listener pool

Each service (web, SSH, MCP, etc.) owns its own listeners and Reconfigure method.
The hub coordinator orchestrates the order across services but does not manage
individual listeners. This keeps service boundaries clean and avoids a shared
listener pool abstraction that would couple unrelated services.

## RFC Documentation

N/A - no protocol work.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/740-graceful-listener-migration.md`
- [ ] Summary included in commit
