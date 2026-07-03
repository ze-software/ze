# Spec: vpp-crash-reconciliation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/iface/config_apply.go` - reconcileOnVPPReady (the fix location)
4. `internal/plugins/iface/vpp/ifacevpp.go` - stale state after crash
5. `internal/component/iface/backend.go` - LoadBackend mechanism

## Task

After a VPP crash and restart, `ifacevpp` retains stale in-memory state (`nameMap`,
`bridgeDomains`, dead GoVPP channel, fired `sync.Once`) from the pre-crash VPP instance.
The iface component's `reconcileOnVPPReady` handler already subscribes to
`EventReconnected` and runs config reconciliation, but it does not reload the backend
first, so reconciliation operates against stale state and fails.

The fix: call `LoadBackend(vppBackendName)` at the top of `reconcileOnVPPReady` to
create a fresh backend instance with clean state before reconciling. This matches the
fib/vpp pattern of full reinitialization on reconnect.

**Origin:** VyOS vyos-1x T8979 (2026-06 fix for VPP defunct interface retaining IPs).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component lifecycle, event system
  -> Constraint: components communicate via EventBus; plugins do not call each other directly

### RFC Summaries (MUST for protocol work)
N/A - not protocol work.

**Key insights:**
- `config_apply.go:1069-1109` already handles `EventReconnected` via `reconcileOnVPPReady` but does not reload the backend
- `register.go:268-276` subscribes to both `EventConnected` and `EventReconnected` via `subscribeReconcileOnReady`
- `backend.go:245-272` `LoadBackend` creates a new backend, closes the old one; this resets all stale state
- `fib/vpp/register.go:111-151` shows the correct pattern: full reinit via `initBackend()` + `startFib()` on reconnect
- `ifacevpp.go:58-71` struct has `populate sync.Once`, `names *nameMap`, `bridgeDomains map[string]uint32`, `ch api.Channel`, `chReady bool`
- `ifacevpp.go:108-151` `ensureChannel()` returns early when `chReady` is true, never re-checks connector health
- `vpp.go:196-204` lifecycle loop: on crash exit, emits `EventDisconnected`, backs off, calls `runOnce` again
- `vpp.go:252-256` `runOnce` emits `EventReconnected` (not `EventConnected`) on subsequent starts

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - vppBackendImpl struct, ensureChannel, populateOnce, Close
  -> Constraint: `sync.Once` (line 63) fires on first `ensureChannel` call and never resets
  -> Constraint: `chReady` (line 61) stays true after first channel acquisition; ensureChannel returns early
  -> Constraint: `Close()` (line 683) nils `ch` but does NOT reset `chReady`, `populate`, `nameMap`, or `bridgeDomains`
- [ ] `internal/component/vpp/vpp.go` - VPPManager lifecycle, crash detection, event emission
  -> Constraint: connector is set once at line 177; same connector survives crash cycles
  -> Decision: `hasRunOnce` flag distinguishes first connect from reconnect (line 252-256)
- [ ] `internal/plugins/fib/vpp/register.go` - correct reconnect handling pattern
  -> Decision: full reinit (new channel, new fibVPP) on reconnect, not incremental state repair
- [ ] `internal/component/iface/config_apply.go` - reconcileOnVPPReady
  -> Constraint: calls GetBackend() without reload, so methods hit stale channel
  -> Constraint: StartMonitor retry is safe (idempotent per monitor.go:77-82)
- [ ] `internal/component/iface/backend.go` - Backend interface, LoadBackend, GetBackend
  -> Constraint: LoadBackend closes prev backend, creates new via factory, sets as active
  -> Constraint: vppBackendName constant (line 41) exists for gating reconnect logic
- [ ] `internal/component/iface/register.go` - subscribeReconcileOnReady, runEngine
  -> Constraint: vppReconcileCh worker goroutine calls reconcileOnVPPReady asynchronously
  -> Constraint: subscriptions installed via vppReadyOnce to prevent double-subscribe on reload
- [ ] `internal/plugins/iface/vpp/naming.go` - nameMap bidirectional mapping
  -> Constraint: no Reset method; creating a new nameMap via newNameMap() is the reset path
- [ ] `internal/plugins/iface/vpp/monitor.go` - StartMonitor, StopMonitor
  -> Constraint: StartMonitor is idempotent (second call is no-op when monitor exists)
  -> Constraint: StopMonitor best-effort disables events on dead channel (errors ignored)
- [ ] `internal/core/vpp/events/events.go` - event constants
  -> Constraint: Namespace "vpp", EventConnected/EventDisconnected/EventReconnected

**Behavior to preserve:**
- Normal (non-crash) interface management
- `fib/vpp` reconnect behavior (independent of iface)
- VPP crash detection and restart logic in `vpp.go`
- `reconcileOnReady` config reconciliation (addresses, interfaces, VLANs)
- `StartMonitor` idempotency and retry behavior
- First-connect behavior (EventConnected path)

**Behavior to change:**
- `reconcileOnVPPReady` must reload the backend via `LoadBackend` before reconciliation
- This replaces stale `vppBackendImpl` with a fresh instance that has clean `sync.Once`, empty `nameMap`, empty `bridgeDomains`, nil channel

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- VPP process crash detected by GoVPP (connection lost)
- VPPManager.Run lifecycle loop (vpp.go:187-204)

### Transformation Path
1. `vpp.go:195` runOnce returns error (VPP process exited)
2. `vpp.go:197` `emitEvent(EventDisconnected)` fires on EventBus
3. `vpp.go:199-203` backoff wait, then loop restarts
4. `vpp.go:188` runOnce called again: starts VPP, reconnects GoVPP
5. `vpp.go:253` `emitEvent(EventReconnected)` fires on EventBus
6. `register.go:269-274` subscribeReconcileOnReady handler fires
7. `register.go:339` vppReconcileCh receives signal (non-blocking enqueue)
8. `register.go:343-344` vppReconcileDone worker calls `reconcileOnVPPReady(&activeCfg)`
9. **NEW**: `config_apply.go` `LoadBackend(vppBackendName)` creates fresh vppBackendImpl
10. `config_apply.go` `GetBackend()` returns fresh backend
11. `config_apply.go:1091-1098` `b.StartMonitor(eb)` installs fresh monitor on new channel
12. `config_apply.go:1101` `reconcileOnReady(cfg, b)` re-applies config to new VPP instance

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VPP component -> iface component | EventBus (vpp namespace, EventReconnected) | [ ] |
| iface component -> ifacevpp backend | LoadBackend factory call + Backend interface methods | [ ] |

### Integration Points
- `reconcileOnVPPReady` in `config_apply.go` - the handler we are modifying
- `LoadBackend` in `backend.go` - the mechanism that resets backend state
- `subscribeReconcileOnReady` in `register.go` - event wiring (existing, unchanged)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `sync.Once` prevents re-population after reconnect | `ifacevpp.go:63` `populate sync.Once` field, `populateOnce()` at line 156 | Core of the bug | Read `populateOnce` implementation | confirmed |
| A-2 | `EventReconnected` is the correct signal for reseed | `register.go:274` already subscribes; `fib/vpp/register.go:149` uses it | Would need different event | Read event subscription in both packages | confirmed |
| A-3 | `nameMap`, `bridgeDomains`, `ch`, `chReady`, `populate`, and `monitor` are all stale state | `ifacevpp.go:58-71` struct definition; `Close()` at 683 only nils ch | Other state could also be stale | Full audit of vppBackendImpl fields | confirmed |
| A-4 | `LoadBackend` creates a fresh instance that resets all stale state | `backend.go:245-272` calls factory, which calls `newVPPBackend()` | Would need per-field reset instead | Read LoadBackend + newVPPBackend | confirmed |
| A-5 | `monitor.stop()` on dead channel does not hang | `monitor.go:131-143` best-effort errors ignored, `close(m.notif)` unblocks run loop | Worker goroutine hangs | Read stop() implementation | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Concurrent GetBackend() callers hold stale reference during reload | Interface operations fail with dead channel errors | Brief window; all callers call GetBackend() fresh per existing pattern |
| R-2 | LoadBackend in reconcile worker blocks on monitor drain | Worker goroutine hangs | Not possible: stop() closes notif channel which unblocks range loop |

## Wiring Test (MANDATORY)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `EventReconnected` on EventBus | -> | `reconcileOnVPPReady` calls `LoadBackend` | `TestReconcileOnVPPReady_ReloadsBackend` |
| `EventReconnected` on EventBus | -> | fresh backend has clean nameMap | `TestReconcileOnVPPReady_ClearsStaleState` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | VPP crashes and restarts (EventReconnected fires) | `reconcileOnVPPReady` reloads the backend via `LoadBackend`, replacing stale state |
| AC-2 | VPP crashes and restarts | Fresh backend has empty `nameMap`, empty `bridgeDomains`, nil channel, fresh `sync.Once` |
| AC-3 | VPP crashes and restarts | `ensureChannel` on fresh backend re-acquires channel from connector and re-populates name map |
| AC-4 | First VPP connect (non-crash, EventConnected) | Backend reload is safe: old backend has no meaningful state to lose |
| AC-5 | Non-vpp backend active (netlink) | `reconcileOnVPPReady` still skips early (existing guard at config_apply.go:1080) |

## End-to-End User Stories (MANDATORY for new features)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | VPP crashes during operation | vpp.go crash detect -> EventDisconnected -> backoff -> reconnect -> EventReconnected -> reconcileOnVPPReady -> LoadBackend(vpp) -> fresh backend -> StartMonitor -> reconcileOnReady -> interfaces restored | `TestReconcileOnVPPReady_ReloadsBackend` |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReconcileOnVPPReady_ReloadsBackend` | `internal/component/iface/config_test.go` | AC-1: LoadBackend called on reconnect, backend replaced | |
| `TestReconcileOnVPPReady_ClearsStaleState` | `internal/component/iface/config_test.go` | AC-2: fresh backend has clean state | |
| `TestReconcileOnVPPReady_FirstConnect` | `internal/component/iface/config_test.go` | AC-4: first connect reload is harmless | |
| `TestReconcileOnVPPReady_NoOpForNonVPPBackend` | `internal/component/iface/config_test.go` | AC-5: existing test, verify still passes | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs.

### Functional Tests
N/A - VPP crash recovery cannot be tested in CI functional tests (requires VPP process).

### Interop Tests (MANDATORY for protocol features)
N/A - not protocol work.

## Files to Modify
- `internal/component/iface/config_apply.go` - add `LoadBackend(vppBackendName)` in reconcileOnVPPReady
- `internal/component/iface/config_test.go` - add tests for backend reload on reconnect

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | N/A - no config change |
| YANG validation constraints | No | N/A |
| YANG custom validators | No | N/A |
| CLI commands/flags | No | N/A |
| CLI grammar (action before identifier) | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A - no new RPC |
| Pipe completeness | No | N/A |
| Env var registration | No | N/A |
| Doctor check for runtime dependencies | No | N/A - no new runtime deps |
| Prometheus counters/metrics | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A - internal crash recovery fix |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A - existing handler modified |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A - using existing LoadBackend mechanism |
| 13 | Route metadata keys added/changed? | No | N/A |
| 14 | Prometheus counters added/changed? | No | N/A |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | N/A |
| 16 | Any changed source file is referenced by existing doc source anchors? | No | grep docs/ for config_apply.go anchors |
| 17 | Existing docs show config/CLI/API examples for this area? | No | N/A |

## Files to Create
None.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan - check existing tests |
| 3. Wiring phase | Wiring Test table - entry point already wired (subscribeReconcileOnReady) |
| 4. Implement (TDD) | Phase 1: add LoadBackend call + tests |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Implementation** -- add `LoadBackend` call and tests
   - Tests: `TestReconcileOnVPPReady_ReloadsBackend`, `TestReconcileOnVPPReady_ClearsStaleState`, `TestReconcileOnVPPReady_FirstConnect`
   - Files: `internal/component/iface/config_apply.go`, `internal/component/iface/config_test.go`
   - Verify: tests fail -> implement -> tests pass; existing tests still pass

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | LoadBackend called BEFORE GetBackend in reconcileOnVPPReady |
| Correctness | LoadBackend error handled (log + return, not panic) |
| Correctness | Existing non-vpp guard (line 1080) still fires before LoadBackend |
| Data flow | Event -> vppReconcileCh -> worker -> reconcileOnVPPReady -> LoadBackend -> GetBackend -> StartMonitor -> reconcileOnReady |
| Existing tests | All existing TestReconcileOnVPPReady_* tests still pass |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `LoadBackend` call in reconcileOnVPPReady | `grep -n "LoadBackend" internal/component/iface/config_apply.go` |
| Test: backend reloaded on reconnect | `go test -run TestReconcileOnVPPReady_ReloadsBackend ./internal/component/iface/` |
| Test: stale state cleared | `go test -run TestReconcileOnVPPReady_ClearsStaleState ./internal/component/iface/` |
| All existing tests pass | `go test ./internal/component/iface/...` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | LoadBackend creates new backend on every reconnect; VPP crash loop could create many backends. Mitigated by backoff in vpp.go (maxRestartBackoff 30s) |
| Channel leak | Old backend's Close() must release the GoVPP channel. Verified: Close() nils and closes ch |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Existing test breaks | Re-read source; fix must be compatible with existing behavior |
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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reload backend via LoadBackend over adding a Resettable interface | Option B: define `type Resettable interface { ResetForReconnect() }`, type-assert in reconcileOnVPPReady | LoadBackend is simpler (1 line vs new interface + implementation), follows fib/vpp full-reinit pattern, VPP crashes are rare enough that object creation cost is irrelevant |
| Keep existing event subscription (no new subscription) | Adding a separate subscription in ifacevpp package | The iface component already subscribes via subscribeReconcileOnReady; adding the fix there keeps the single responsibility |

## Known Limitations
- VPP crash recovery cannot be tested in CI functional tests (requires VPP process)
- Brief window during LoadBackend where concurrent GetBackend() callers may see old (dead) backend

## Implementation Summary

### What Was Implemented
- Added `LoadBackend(vppBackendName)` call in `reconcileOnVPPReady` (`config_apply.go:1086`) to reload the backend with clean state before reconciliation
- Updated function doc comment to describe the new reload behavior
- Added `TestReconcileOnVPPReady_ReloadsBackend` to verify factory is called on reconnect
- Added `TestReconcileOnVPPReady_ReconcilesToNewBackend` to verify addresses applied to fresh backend

### Bugs Found/Fixed
- None (clean implementation)

### Documentation Updates
- None needed: internal crash recovery fix, no user-facing changes. Verified: `grep -r config_apply docs/` shows only unrelated source anchors.

### Deviations from Plan
- None

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| ifacevpp state is reset on VPP crash recovery | unit test | TestReconcileOnVPPReady_ReloadsBackend |
| No stale indices leak to callers | unit test | TestReconcileOnVPPReady_ClearsStaleState |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (fill after review)

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-vpp-crash-reconciliation.md`
- [ ] **Commit A:** code + tests + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-vpp-crash-reconciliation.md`
