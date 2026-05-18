# Spec: chaos-actions-v2

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-05-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/architecture/chaos-web-dashboard.md` - "New Chaos Action Types" and "Replay Constraint" sections
3. `internal/chaos/engine/action.go` - ChaosAction struct, ActionType enum, name maps
4. `internal/chaos/engine/scheduler.go` - Scheduler, defaultWeights, Tick()
5. `internal/chaos/peer/simulator.go` - main select loop, chaos channel dispatch
6. `internal/chaos/peer/simulator_actions.go` - executeChaos() switch
7. `internal/chaos/peer/event.go` - Event struct (ChaosAction string field)
8. `internal/chaos/report/jsonlog.go` - jsonEvent struct, ProcessEvent(), LogControl()
9. `internal/chaos/replay/replay.go` - logEvent struct, Run()
10. `cmd/ze-chaos/scheduler.go` - runScheduler(), handleManualTrigger(), dispatchAction()
11. `cmd/ze-chaos/main.go` - CLI flags
12. `internal/chaos/web/control.go` - chaosActionTypes(), chaosActionIcon/Label/Impact

## Task

Add 6 new parameterized chaos action types to the chaos package. These are independent of the web dashboard — they work with the automatic scheduler and can be triggered via the future web UI.

New actions: **ClockDrift**, **RouteBurst**, **WithdrawalBurst**, **RouteFlap**, **SlowPeer**, **ZeroWindow**

Each action has configurable parameters (count, duration, etc.). All actions emit standard `EventChaosExecuted` events with parameters recorded in the NDJSON event log for replay.

**Parent spec:** `plan/spec-chaos-web-dashboard.md`
**Design doc:** `docs/architecture/chaos-web-dashboard.md` (sections "New Chaos Action Types" and "Replay Constraint")

## Required Reading

### Architecture Docs
- [x] `docs/architecture/chaos-web-dashboard.md` - "New Chaos Action Types" and "Replay Constraint" sections
  -> Constraint: Manual triggers must be indistinguishable from scheduler events in the log
  -> Decision: chaos-params field added to NDJSON for parameterized actions
- [x] `internal/chaos/engine/scheduler.go` - Scheduler.Tick() selects action via weighted random from defaultWeights (8 entries, not 10)
  -> Constraint: New actions disabled by default in scheduler; enabled via --chaos-actions
  -> Decision: Scheduler needs EnabledActions or similar filter; selectAction() currently picks from fixed defaultWeights
- [x] `internal/chaos/peer/simulator.go` - RunSimulator main loop: select on ctx, readerDone, keepalive, chaosCh, routeCh
  -> Constraint: executeChaos() returns ChaosResult{Disconnected bool}, emits EventChaosExecuted with action.Type.String()
  -> Decision: Event.ChaosAction is a string; must also propagate Params for NDJSON logging
- [x] `internal/chaos/report/jsonlog.go` - jsonEvent struct has ChaosAction string field but no chaos-params
  -> Decision: Add ChaosParams map[string]string to jsonEvent (omitempty for backwards compat)
- [x] `internal/chaos/replay/replay.go` - logEvent struct mirrors jsonEvent; eventTypeFromString map for parsing
  -> Decision: Add ChaosParams to logEvent for parameterized replay
- [x] `internal/chaos/peer/simulator_actions.go` - executeChaos() switch on action.Type for 9 existing types
  -> Decision: Add 6 new cases; some need conn access (ZeroWindow), timing (ClockDrift, SlowPeer), route gen (RouteBurst)
- [x] `internal/chaos/web/control.go` - chaosActionTypes() returns 9 names; chaosActionIcon/Label/Impact switch statements
  -> Decision: Add 6 new entries to all switch statements and chaosActionTypes() list
- [x] `internal/chaos/web/state.go` - ManualTrigger already has Params map[string]string
  -> Decision: handleManualTrigger in cmd/ze-chaos/scheduler.go must pass t.Params to ChaosAction
- [x] `cmd/ze-chaos/scheduler.go` - handleManualTrigger creates ChaosAction{Type: actionType} without params
  -> Decision: Must pass t.Params into ChaosAction.Params

**Key insights:**
- ChaosAction struct needs a Params map[string]string for parameterized actions
- Peer simulator needs handlers for each new action type in executeChaos() switch
- NDJSON format extended with chaos-params (backwards compatible, old events omit it via omitempty)
- Event struct needs ChaosParams field to propagate params from simulator to reporter
- ZeroWindow uses net.TCPConn.SetReadBuffer(0) for TCP window manipulation
- SlowPeer is distinct from existing SlowRead (SlowRead delays reads; SlowPeer delays writes)
- ClockDrift is a timing action affecting keepalive loop, not a one-shot like most others
- New actions added to defaultWeights with opt-in filter; existing actions unaffected
- ManualTrigger.Params already collected by web UI but not passed to ChaosAction

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/chaos/engine/action.go` - 9 ActionType constants (TCPDisconnect..SlowRead), ChaosAction{Type ActionType}
- [x] `internal/chaos/engine/scheduler.go` - 8 defaultWeights (SlowRead not in scheduler), Scheduler.Tick() with weighted random
- [x] `internal/chaos/peer/simulator.go` - RunSimulator with select on chaosCh, emits EventChaosExecuted{ChaosAction: action.Type.String()}
- [x] `internal/chaos/peer/simulator_actions.go` - executeChaos() switch with 9 cases, returns ChaosResult
- [x] `internal/chaos/peer/event.go` - Event struct with ChaosAction string field (no params)
- [x] `internal/chaos/report/jsonlog.go` - jsonEvent with ChaosAction string, no chaos-params field
- [x] `internal/chaos/replay/replay.go` - logEvent mirrors jsonEvent, no chaos-params
- [x] `cmd/ze-chaos/scheduler.go` - handleManualTrigger ignores t.Params, creates ChaosAction{Type: actionType}
- [x] `internal/chaos/web/control.go` - chaosActionTypes() returns 9 names, ManualTrigger.Params already collected but unused

**Behavior to preserve:**
- Existing 9 chaos actions unchanged in behavior
- Existing NDJSON format backwards compatible (old records still valid, omitempty on new fields)
- Scheduler behavior unchanged when new actions not enabled (--chaos-actions flag absent)
- Existing web dashboard trigger UI works for existing action types

**Behavior to change:**
- ChaosAction struct extended with Params map[string]string
- Event struct gets ChaosParams map[string]string for NDJSON propagation
- jsonEvent gets ChaosParams field (omitempty for backwards compat)
- logEvent gets ChaosParams for replay
- 6 new ActionType constants and Name constants
- 6 new cases in executeChaos() switch
- 6 new entries in chaosActionTypes(), icon, label, impact functions
- defaultWeights extended with 6 new entries (behind opt-in filter)
- --chaos-actions CLI flag to enable specific new actions in scheduler
- handleManualTrigger passes t.Params to ChaosAction.Params

## Data Flow (MANDATORY)

### Entry Point
- Scheduler.Tick() or manual trigger -> ChaosAction on peer channel

### Transformation Path
1. Scheduler selects action type (weighted random, respecting --chaos-actions filter)
2. ChaosAction sent to peer's chaos channel with type + params
3. Peer simulator executes action (TCP manipulation, route burst, etc.)
4. Peer emits EventChaosExecuted with action name + params
5. Event flows through Reporter to all consumers including JSONLog
6. JSONLog writes chaos-params field for parameterized actions

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Scheduler -> Peer | ChaosAction on buffered channel | [ ] |
| Peer -> Reporter | EventChaosExecuted event | [ ] |
| Reporter -> JSONLog | NDJSON record with chaos-params | [ ] |

### Integration Points
- `chaos.ChaosAction` struct — Extended with Params map field
- `peer/simulator.go` chaos switch — New cases for 6 action types
- `report/jsonlog.go` event format — chaos-params field added to NDJSON records
- `chaos/scheduler.go` Tick() — --chaos-actions filter for opt-in to new types
- `replay/replay.go` — Parse chaos-params from log records for replay

### Architectural Verification
- [ ] No bypassed layers (actions flow through normal scheduler → peer → reporter pipeline)
- [ ] No unintended coupling (new actions self-contained in chaos package)
- [ ] No duplicated functionality (extends existing action dispatch, doesn't recreate)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ClockDrift action with drift=+3s | Peer sends next keepalive 3s late |
| AC-2 | ClockDrift with drift > hold time | Rejected (validation error) |
| AC-3 | RouteBurst with count=500, family=ipv4/unicast | Peer announces 500 extra routes rapidly |
| AC-4 | WithdrawalBurst with count=100 | Peer withdraws exactly 100 routes |
| AC-5 | WithdrawalBurst with count > announced | Withdraws all announced routes (clamped) |
| AC-6 | RouteFlap count=50, cycles=3, interval=100ms | 3 withdraw+announce cycles of 50 routes, 100ms apart |
| AC-7 | SlowPeer delay=2s, duration=30s | All outgoing messages delayed 2s for 30s, then normal |
| AC-8 | ZeroWindow duration=15s | TCP recv window set to zero for 15s, then restored |
| AC-9 | New action with --event-log | chaos-params appears in NDJSON record |
| AC-10 | Replay log containing parameterized actions | Actions replay correctly, validation matches |
| AC-11 | --chaos-actions=RouteBurst,RouteFlap | Only these new actions used by scheduler (existing actions still active) |
| AC-12 | No --chaos-actions flag | New actions not used by scheduler (backwards compatible) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestClockDriftParams | `internal/chaos/engine/action_params_test.go` | ClockDrift param validation, drift range | |
| TestRouteBurstParams | `internal/chaos/engine/action_params_test.go` | RouteBurst count and family params | |
| TestWithdrawalBurstParams | `internal/chaos/engine/action_params_test.go` | WithdrawalBurst count validation | |
| TestRouteFlapParams | `internal/chaos/engine/action_params_test.go` | RouteFlap cycles, interval params | |
| TestSlowPeerParams | `internal/chaos/engine/action_params_test.go` | SlowPeer delay and duration params | |
| TestZeroWindowParams | `internal/chaos/engine/action_params_test.go` | ZeroWindow duration param | |
| TestSchedulerNewActionsDisabledByDefault | `internal/chaos/engine/scheduler_test.go` | Tick() never returns new action types without opt-in | |
| TestSchedulerChaosActionsFilter | `internal/chaos/engine/scheduler_test.go` | EnabledActions filters to specified types | |
| TestActionParamsSerialization | `internal/chaos/engine/action_params_test.go` | Params serialize to map for JSON logging | |
| TestJSONLogChaosParams | `internal/chaos/report/jsonlog_test.go` | Parameterized events include chaos-params | |
| TestJSONLogBackwardsCompat | `internal/chaos/report/jsonlog_test.go` | Non-parameterized events unchanged | |
| TestReplayParameterizedActions | `internal/chaos/replay/replay_test.go` | Parameterized events replay correctly | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| RouteBurst count | 1-10000 | 10000 | 0 | N/A (capped) |
| WithdrawalBurst count | 1-10000 | 10000 | 0 | N/A (capped) |
| RouteFlap cycles | 1-50 | 50 | 0 | 51 (capped) |
| ClockDrift abs(drift) | 0 to holdTime-1s | holdTime-1s | N/A | holdTime |
| SlowPeer delay | 100ms-30s | 30s | 99ms (clamp) | N/A |
| ZeroWindow duration | 1s-120s | 120s | 0 | N/A (capped) |

### Functional Tests

Functional tests deferred: the chaos tool requires a running Ze instance and multi-peer orchestration.
Unit tests cover parameter validation, scheduler filtering, NDJSON serialization, and replay parsing.
Integration is verified via the existing ze-chaos in-process mode tests.

## Files to Modify

- `internal/chaos/engine/action.go` - Add 6 new ActionType constants, Name constants, maps, reconnect map entries
- `internal/chaos/engine/scheduler.go` - Add new defaultWeights, opt-in filter (EnabledActions)
- `internal/chaos/peer/simulator_actions.go` - Add 6 new cases to executeChaos() switch
- `internal/chaos/peer/simulator.go` - Pass ChaosParams from Event to emit, add sendDelay for SlowPeer
- `internal/chaos/peer/event.go` - Add ChaosParams map[string]string field to Event
- `internal/chaos/report/jsonlog.go` - Add ChaosParams to jsonEvent, populate in ProcessEvent()
- `internal/chaos/replay/replay.go` - Add ChaosParams to logEvent
- `cmd/ze-chaos/main.go` - Add --chaos-actions flag
- `cmd/ze-chaos/scheduler.go` - Pass Params from ManualTrigger to ChaosAction, pass enabled list to scheduler
- `internal/chaos/web/control.go` - Add 6 new entries to chaosActionTypes/Icon/Label/Impact
- `internal/chaos/engine/action_test.go` - Update tests for new action types
- `internal/chaos/engine/scheduler_test.go` - Add tests for opt-in filter

## Files to Create

- `internal/chaos/engine/action_params.go` - Parameter validation per new action type
- `internal/chaos/engine/action_params_test.go` - Tests for parameter validation
- `internal/chaos/report/jsonlog_test.go` - (update) Tests for chaos-params field

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `engine.Scheduler.Tick()` | -> | `selectAction()` with new types | `TestSchedulerChaosActionsFilter` |
| `executeChaos()` switch | -> | 6 new action handlers | `TestClockDriftAction` etc. in action_params_test |
| `peer.Event.ChaosParams` | -> | `report.JSONLog.ProcessEvent()` | `TestJSONLogChaosParams` |
| `--chaos-actions` flag | -> | `SchedulerConfig.EnabledActions` | `TestSchedulerNewActionsDisabledByDefault` |
| `handleManualTrigger()` | -> | `ChaosAction.Params` | Manual trigger dispatches params |
| `chaosActionTypes()` | -> | Dashboard trigger buttons | 6 new buttons in sidebar |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Critical Review Checklist

| # | What to verify | How |
|---|----------------|-----|
| 1 | New ActionType iota values don't break existing action dispatch | Check iota ordering, ensure existing values unchanged |
| 2 | Params map is nil-safe in all consumers (NDJSON, replay, web) | grep for `.Params[` and check nil guards |
| 3 | defaultWeights total recalculated correctly with new entries | Verify init() sums all weights including new |
| 4 | EnabledActions filter doesn't affect existing 9 actions | Test with no filter: existing actions still selected |
| 5 | ClockDrift drift validation rejects drift >= holdTime | Unit test boundary |
| 6 | RouteBurst/WithdrawalBurst count clamped not rejected | Values above max are capped, not errored |
| 7 | executeChaos() new cases return correct Disconnected flag | None of 6 new actions should disconnect |
| 8 | NDJSON backwards compat: events without params omit field | omitempty tag on ChaosParams |
| 9 | Replay skips unknown event types gracefully | Existing continue on unknown |
| 10 | Web UI chaosActionTypes() includes all 15 action names | Count entries |

## Deliverables Checklist

| # | Deliverable | Verification |
|---|-------------|-------------|
| 1 | 6 new ActionType constants | `grep -c 'Action.*ActionType = iota' engine/action.go` shows 15 |
| 2 | Params field on ChaosAction | `grep 'Params' engine/action.go` |
| 3 | Parameter validation functions | `grep 'func Validate' engine/action_params.go` |
| 4 | 6 new cases in executeChaos | `grep -c 'case engine.Action' peer/simulator_actions.go` shows 15 |
| 5 | ChaosParams on Event struct | `grep 'ChaosParams' peer/event.go` |
| 6 | chaos-params in jsonEvent | `grep 'chaos-params' report/jsonlog.go` |
| 7 | chaos-params in logEvent | `grep 'chaos-params' replay/replay.go` |
| 8 | EnabledActions in SchedulerConfig | `grep 'EnabledActions' engine/scheduler.go` |
| 9 | --chaos-actions flag | `grep 'chaos-actions' cmd/ze-chaos/main.go` |
| 10 | Web UI updated for 15 actions | `grep -c 'engine.Name' web/control.go` shows 15 |
| 11 | Unit tests pass | `make ze-unit-test` |
| 12 | Lint passes | `make ze-lint` |

## Security Review Checklist

| # | Concern | Check |
|---|---------|-------|
| 1 | ZeroWindow: TCP socket manipulation | Only on peer's own conn, duration-bounded |
| 2 | RouteBurst: memory exhaustion via large count | Count capped at 10000, uses existing route generator |
| 3 | SlowPeer: goroutine leak if duration expires during shutdown | Context cancellation must interrupt sleep |
| 4 | Params map from web: user-controlled strings | Params parsed/validated before use, no injection vectors |
| 5 | ClockDrift: integer overflow in duration parsing | Use time.ParseDuration, validated range |

## Implementation Steps

1. **Extend ChaosAction (TDD)** - Add Params map, parameter validation per type, 6 new ActionType constants
2. **Extend Event + NDJSON + Replay** - ChaosParams field through the pipeline
3. **Implement action execution in simulator (TDD)** - Handle each new type in executeChaos()
4. **Add scheduler filter (TDD)** - EnabledActions, new defaultWeights, --chaos-actions flag
5. **Update web UI** - chaosActionTypes/Icon/Label/Impact for 6 new actions
6. **Wire manual trigger params** - handleManualTrigger passes Params through

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Audit

Updated during implementation.

## Checklist

### Goal Gates
- [ ] AC-1..AC-12 demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] `make ze-lint` passes

- [ ] Existing chaos tests still pass (no regressions)

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
