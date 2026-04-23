# Spec: diag-2-event-history -- Event History and FSM Transition Log

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-04-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/plugin/server/dispatch.go:375` -- deliverEvent, the central event dispatch
4. `internal/component/bgp/reactor/peer_run.go` -- BGP FSM callback handler
5. `internal/component/l2tp/tunnel.go` -- L2TP tunnel FSM states
6. `internal/component/l2tp/session_fsm.go` -- L2TP session FSM states
7. Parent: `plan/spec-diag-0-umbrella.md`

## Task

Add event history and FSM transition logging so Claude can answer "what happened
in the last 5 minutes?" Three capabilities:

1. **Global event ring** -- captures the last N events across all namespaces with timestamp, namespace, event type. Taps into `Server.deliverEvent()` at the dispatch level.
2. **BGP peer FSM history** -- per-peer ring of state transitions with timestamp, from/to state, reason. Hooks into the existing FSM callback in `peer_run.go`.
3. **L2TP tunnel/session FSM history** -- per-tunnel and per-session ring of state transitions. Hooks into the existing state change points in `tunnel_fsm.go` and `session_fsm.go`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- small-core + registration pattern
  → Constraint: global ring lives in the plugin server (where deliverEvent is); FSM rings live in their respective components
- [ ] `ai/patterns/cli-command.md` -- YANG RPC + handler pattern
  → Constraint: new show commands follow same pattern as diag-1

### RFC Summaries
- [ ] Not protocol work.

**Key insights:**
- No wildcard event subscription exists. The global ring must tap `Server.deliverEvent()` directly (dispatch.go:375, LSP-confirmed) rather than subscribing to each event type individually.
- `deliverEvent` is the single dispatch point for ALL events (engine-side and plugin-side). A ring append here captures everything.
- BGP FSM: `fsm.State` type (fsm/state.go:23) with 6 states. FSM callback `SetCallback(StateCallback)` receives (from, to State). Peer-level callback in peer_run.go:297 has access to timestamp, peer address, reason string, and session stats.
- L2TP tunnel FSM: `L2TPTunnelState` (tunnel.go:17) with 5 states (idle/wait-ctl-reply/wait-ctl-conn/established/closed). State changes happen in tunnel_fsm.go.
- L2TP session FSM: `L2TPSessionState` (session.go:16) with states for the session lifecycle.
- L2TP observer event ring already demonstrates the ring buffer pattern (diag-1 confirmed this works).
- 9+ event namespaces registered: bgp, bgp-rib, l2tp, interface, config, vpp, fib, system, system-rib, static, sysctl.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/dispatch.go` (450+ LOC) -- `deliverEvent` at line 375 validates event, converts to typed IDs, dispatches to engine subscribers and plugin subscribers. Central point for all events.
  → Constraint: ring append must not block dispatch. Use non-blocking append (fixed-size, overwrite oldest).
- [ ] `internal/component/plugin/server/engine_event.go` (200 LOC) -- `Emit()`, `Subscribe()`, `dispatchEngineEvent()`. Engine subscribers fire synchronously.
  → Constraint: the global ring is NOT an engine subscriber. It's a tap in deliverEvent itself, firing before subscriber dispatch.
- [ ] `internal/component/bgp/reactor/peer_run.go` (384 LOC) -- FSM callback registered at line 297. Has `from` and `to` PeerState, timestamp via clock, peer address, reason string (lines 357-360).
  → Constraint: FSM callback runs on the peer's goroutine. Ring append must be fast (no allocation).
- [ ] `internal/component/bgp/fsm/state.go` (65 LOC) -- `fsm.State` with 6 states (Idle/Active/Connect/OpenSent/OpenConfirm/Established).
  → Decision: BGP history stores PeerState (4 states) not fsm.State (6 states), since PeerState is what operators see.
- [ ] `internal/component/l2tp/tunnel.go` (43 LOC for state) -- `L2TPTunnelState` with 5 states.
  → Constraint: tunnel state changes happen inline in tunnel_fsm.go, not via callback. Need to add a record call at each transition.
- [ ] `internal/component/l2tp/session_fsm.go` -- Session FSM transitions.
  → Constraint: same as tunnel -- need to add record calls at transitions.
- [ ] `internal/component/l2tp/observer.go` -- existing eventRing pattern for ring buffer.
  → Decision: reuse the same ring buffer pattern (fixed array, head/count, snapshot returns copy).

**Behavior to preserve:**
- All existing event dispatch behavior unchanged
- BGP FSM callback behavior unchanged (metrics, logging)
- L2TP tunnel/session FSM behavior unchanged
- Event bus performance characteristics unchanged (no allocation on emit path)

**Behavior to change:**
- `deliverEvent` gains a ring append call (one branch, no allocation)
- BGP peer FSM callback gains a ring append call
- L2TP tunnel/session FSM transitions gain ring append calls

## Data Flow (MANDATORY)

### Entry Point (Global Event Ring)

1. Any component calls `bus.Emit(namespace, eventType, payload)`
2. `Server.deliverEvent()` validates the event
3. **NEW:** ring.Append(timestamp, namespace, eventType) -- non-blocking, fixed-size
4. Engine subscribers dispatch (existing)
5. Plugin subscribers dispatch (existing)

### Entry Point (BGP FSM History)

1. BGP FSM transitions to a new state
2. `peer_run.go` FSM callback fires with (from, to) PeerState
3. **NEW:** peer.fsmHistory.Append(timestamp, from, to, reason)
4. Existing callback logic (metrics, logging) continues

### Entry Point (L2TP FSM History)

1. Tunnel/session FSM transitions in tunnel_fsm.go / session_fsm.go
2. **NEW:** tunnel/session.fsmHistory.Append(timestamp, from, to, trigger)
3. Existing transition logic continues

### Transformation Path

1. Event enters via `bus.Emit()` or FSM callback fires
2. Ring append stores fixed-size record (timestamp + namespace/type string or state enum) -- O(1), no allocation
3. Query enters via CLI/MCP dispatch -> handler -> ring.Snapshot(filters) -> copy returned
4. Handler wraps snapshot in `plugin.Response` JSON

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| deliverEvent ↔ global ring | Direct append call in dispatch.go | [ ] |
| FSM callback ↔ per-peer ring | Direct append in peer_run.go | [ ] |
| Tunnel FSM ↔ per-tunnel ring | Direct append in tunnel_fsm.go | [ ] |
| CLI handler ↔ ring snapshot | Query via facade method | [ ] |

### Integration Points

- `internal/component/plugin/server/dispatch.go` -- add ring append in deliverEvent
- `internal/component/bgp/reactor/peer_run.go` -- add ring append in FSM callback
- `internal/component/l2tp/tunnel_fsm.go` -- add ring append at state transitions
- `internal/component/l2tp/session_fsm.go` -- add ring append at state transitions
- New show commands: `show event recent`, `show bgp peer history`, `show l2tp tunnel history`, `show l2tp session history`

### Architectural Verification

- [ ] No bypassed layers (ring taps are in the dispatch/callback path, not bypassing anything)
- [ ] No unintended coupling (global ring lives in plugin/server; FSM rings live in their components)
- [ ] No duplicated functionality (new ring buffers, not duplicating observer)
- [ ] Zero-copy preserved (ring stores value types, snapshot returns copies)

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show event recent` | → | Global ring snapshot | `test/plugin/show-event-recent.ci` |
| `show bgp peer history <peer>` | → | Per-peer FSM ring | `test/plugin/show-bgp-peer-history.ci` |
| `show l2tp tunnel history <tid>` | → | Per-tunnel FSM ring | `test/l2tp/show-tunnel-history.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show event recent` after BGP peer connects | JSON array of recent events with timestamp, namespace, event-type (newest first) |
| AC-2 | `show event recent count 5` | Last 5 events only |
| AC-3 | `show event recent namespace bgp` | Only events from the bgp namespace |
| AC-4 | `show event namespaces` | List of registered namespaces with total event counts |
| AC-5 | `show bgp peer history <peer>` for a peer that went through connect/established | JSON array of FSM transitions: timestamp, from-state, to-state, reason |
| AC-6 | `show bgp peer history <peer>` for unknown peer | Error: "peer not found" |
| AC-7 | `show l2tp tunnel history <tid>` for a tunnel that completed handshake | JSON array of tunnel FSM transitions: timestamp, from-state, to-state, trigger |
| AC-8 | `show l2tp session history <sid>` for a session that established | JSON array of session FSM transitions |
| AC-9 | Global ring does not block event dispatch | Event delivery latency unchanged (benchmark test) |
| AC-10 | All new commands visible in MCP tools/list | Auto-generated from YANG RPCs |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGlobalEventRingAppend` | `internal/component/plugin/server/event_ring_test.go` | AC-1: ring captures events |  |
| `TestGlobalEventRingOverflow` | `internal/component/plugin/server/event_ring_test.go` | Ring wraps at capacity |  |
| `TestGlobalEventRingFilterNamespace` | `internal/component/plugin/server/event_ring_test.go` | AC-3: namespace filter |  |
| `TestGlobalEventRingFilterCount` | `internal/component/plugin/server/event_ring_test.go` | AC-2: count limit |  |
| `TestBGPFSMHistory` | `internal/component/bgp/reactor/peer_history_test.go` | AC-5: transitions recorded |  |
| `TestBGPFSMHistoryUnknownPeer` | `internal/component/bgp/reactor/peer_history_test.go` | AC-6: error for unknown |  |
| `TestL2TPTunnelFSMHistory` | `internal/component/l2tp/tunnel_fsm_test.go` | AC-7: tunnel transitions |  |
| `TestL2TPSessionFSMHistory` | `internal/component/l2tp/session_fsm_test.go` | AC-8: session transitions |  |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| event ring count | 1 - 10000 | 10000 | 0 | 10001 |
| event ring capacity (config) | 100 - 100000 | 100000 | 99 | 100001 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-show-event-recent` | `test/plugin/show-event-recent.ci` | Operator queries recent events after peer connects | |
| `test-show-bgp-peer-history` | `test/plugin/show-bgp-peer-history.ci` | Operator queries peer FSM history | |

### Future
- L2TP tunnel/session history functional tests need L2TP daemon running

## Files to Modify

- `internal/component/plugin/server/dispatch.go` -- add ring append in deliverEvent
- `internal/component/plugin/server/server.go` -- add global ring field on Server, initialize
- `internal/component/bgp/reactor/peer_run.go` -- add FSM history ring append
- `internal/component/bgp/reactor/peer.go` -- add fsmHistory field on Peer
- `internal/component/l2tp/tunnel.go` -- add fsmHistory field on L2TPTunnel
- `internal/component/l2tp/tunnel_fsm.go` -- add ring append at state transitions
- `internal/component/l2tp/session.go` -- add fsmHistory field on session
- `internal/component/l2tp/session_fsm.go` -- add ring append at state transitions
- `internal/component/l2tp/subsystem_snapshot.go` -- add TunnelFSMHistory/SessionFSMHistory facade
- `internal/component/l2tp/service_locator.go` -- add history methods to Service interface
- `internal/component/l2tp/schema/ze-l2tp-api.yang` -- add tunnel-history, session-history RPCs
- `internal/component/cmd/l2tp/schema/ze-l2tp-cmd.yang` -- augment show tree
- `internal/component/cmd/l2tp/l2tp.go` -- add history handlers

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | ze-l2tp-api.yang, new show-event YANG |
| CLI commands/flags | Yes | YANG ze:command augments |
| Editor autocomplete | Yes | YANG-driven |
| Functional test | Yes | test/plugin/*.ci |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | No | -- |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | -- |
| 6 | Has a user guide page? | Yes | `docs/guide/diagnostics.md` |
| 7-12 | No | -- | -- |

## Files to Create

- `internal/component/plugin/server/event_ring.go` -- global event ring buffer type
- `internal/component/plugin/server/event_ring_test.go` -- ring buffer tests
- `internal/component/bgp/reactor/peer_history.go` -- per-peer FSM history ring
- `internal/component/bgp/reactor/peer_history_test.go` -- FSM history tests
- New YANG for `show event` commands
- `test/plugin/show-event-recent.ci`
- `test/plugin/show-bgp-peer-history.ci`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4-13 | Standard flow |

### Implementation Phases

1. **Phase: Global Event Ring** -- ring buffer type + deliverEvent hook
   - Tests: `TestGlobalEventRingAppend`, `TestGlobalEventRingOverflow`, `TestGlobalEventRingFilterNamespace`, `TestGlobalEventRingFilterCount`
   - Files: `event_ring.go`, `dispatch.go`, `server.go`
   - Verify: tests fail -> implement ring + hook -> tests pass

2. **Phase: Global Event Ring CLI** -- show event commands
   - Tests: functional test
   - Files: new YANG module, handler registration
   - Verify: command dispatches and returns recent events

3. **Phase: BGP FSM History** -- per-peer FSM ring
   - Tests: `TestBGPFSMHistory`, `TestBGPFSMHistoryUnknownPeer`
   - Files: `peer_history.go`, `peer.go`, `peer_run.go`
   - Verify: tests fail -> implement ring + callback hook -> tests pass

4. **Phase: BGP FSM History CLI** -- show bgp peer history command
   - Tests: functional test
   - Files: handler, YANG
   - Verify: command works end-to-end

5. **Phase: L2TP FSM History** -- per-tunnel and per-session FSM rings
   - Tests: `TestL2TPTunnelFSMHistory`, `TestL2TPSessionFSMHistory`
   - Files: tunnel.go, tunnel_fsm.go, session.go, session_fsm.go, subsystem_snapshot.go
   - Verify: tests fail -> implement -> tests pass

6. **Phase: L2TP FSM History CLI** -- show l2tp tunnel/session history
   - Tests: functional test
   - Files: l2tp.go handlers, YANG
   - Verify: commands work

7. **Full verification** -> `make ze-verify`

### Critical Review Checklist

| Check | What to verify |
|-------|---------------|
| Completeness | Every AC-N has implementation |
| Correctness | Ring append never blocks; snapshot returns copies; nil rings handled |
| Naming | JSON keys kebab-case; YANG follows existing pattern |
| Data flow | Ring taps are in dispatch/callback path, not bypassing |
| Rule: no-layering | Direct ring append, no wrapper abstractions |
| Performance | Ring append is O(1) fixed-size, no allocation |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Global event ring captures events | `show event recent` returns events after activity |
| Namespace filter works | `show event recent namespace bgp` filters correctly |
| BGP FSM history per-peer | `show bgp peer history <peer>` shows transitions |
| L2TP tunnel history | `show l2tp tunnel history <tid>` shows transitions |
| L2TP session history | `show l2tp session history <sid>` shows transitions |
| No dispatch latency impact | Ring append is non-blocking (code review) |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | peer address, tunnel-id, session-id validated |
| Secret redaction | Event ring stores namespace/type/timestamp only, no payloads |
| Resource exhaustion | Ring size bounded by compile-time constant or config |
| Concurrency | Ring accessed from dispatch goroutine; snapshot under lock |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in phase that introduced it |
| Test fails wrong reason | Fix test |
| Lint failure | Fix inline |
| 3 fix attempts fail | STOP. Ask user. |

## Design Alternatives

### Global Ring: deliverEvent tap (CHOSEN)

Append one record per event directly in `Server.deliverEvent()`. The ring stores only (timestamp, namespace, eventType) as value types. No payload stored (keeps the ring small and avoids retaining large payload references).

**Gains:** Captures every event. Single hook point. No per-namespace subscription overhead.
**Costs:** Modifies the dispatch hot path (but append is O(1) with no allocation).

### Global Ring: per-namespace subscribers (REJECTED)

Subscribe to each (namespace, eventType) pair individually.

**Rejected:** 50+ subscriptions needed. Each adds overhead to subscription dispatch. No catch-all mechanism exists.

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

- The global event ring is best placed as a tap in `deliverEvent` rather than as a subscriber, because no wildcard subscription exists and subscribing to 50+ event types individually would be wasteful.
- The ring should NOT store payloads. Payloads can be large (entire BGP UPDATE messages via redistevents) and heterogeneous. Storing only (timestamp, namespace, eventType) keeps the ring fixed-size and avoids holding references to large data.
- BGP PeerState (4 states) is the right granularity for operator-visible history, not fsm.State (6 states). Operators think in terms of connecting/active/established, not OpenSent/OpenConfirm.

## RFC Documentation

Not protocol work.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

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
- None (pre-implementation)

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above

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
- [ ] Feature code integrated
- [ ] Integration completeness proven
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/653-diag-2-event-history.md`
- [ ] **Summary included in commit**
