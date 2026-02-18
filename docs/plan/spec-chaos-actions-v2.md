# Spec: chaos-actions-v2

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/architecture/chaos-web-dashboard.md` - "New Chaos Action Types" and "Replay Constraint" sections
3. `cmd/ze-chaos/chaos/` - scheduler, action types
4. `cmd/ze-chaos/peer/simulator.go` - chaos action execution
5. `cmd/ze-chaos/peer/event.go` - Event struct (ChaosAction field)
6. `cmd/ze-chaos/report/jsonlog.go` - NDJSON event log format

## Task

Add 6 new parameterized chaos action types to the chaos package. These are independent of the web dashboard — they work with the automatic scheduler and can be triggered via the future web UI.

New actions: **ClockDrift**, **RouteBurst**, **WithdrawalBurst**, **RouteFlap**, **SlowPeer**, **ZeroWindow**

Each action has configurable parameters (count, duration, etc.). All actions emit standard `EventChaosExecuted` events with parameters recorded in the NDJSON event log for replay.

**Parent spec:** `docs/plan/spec-chaos-web-dashboard.md`
**Design doc:** `docs/architecture/chaos-web-dashboard.md` (sections "New Chaos Action Types" and "Replay Constraint")

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/chaos-web-dashboard.md` - New action definitions, parameters, weights, replay constraint
  -> Constraint: Manual triggers must be indistinguishable from scheduler events in the log
  -> Decision: chaos-params field added to NDJSON for parameterized actions
- [ ] `cmd/ze-chaos/chaos/scheduler.go` - How Tick() selects and dispatches actions
  -> Constraint: New actions disabled by default in scheduler; enabled via --chaos-actions
- [ ] `cmd/ze-chaos/peer/simulator.go` - How chaos actions are executed
  -> Constraint: Actions received on chaos channel, executed in peer goroutine
- [ ] `cmd/ze-chaos/report/jsonlog.go` - NDJSON event format
  -> Decision: Add chaos-params field for parameterized actions

**Key insights:**
- ChaosAction struct needs a Params map for parameterized actions
- Peer simulator needs handlers for each new action type
- NDJSON format extended with chaos-params (backwards compatible — old events omit it)
- ZeroWindow requires TCP socket access (`net.TCPConn.SetReadBuffer`)
- New actions use default weights in scheduler but are opt-in via --chaos-actions flag

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze-chaos/chaos/scheduler.go` - Scheduler with 10 action types and weights
- [ ] `cmd/ze-chaos/chaos/actions.go` - ChaosAction type definition (if exists)
- [ ] `cmd/ze-chaos/peer/simulator.go` - Chaos action switch in peer goroutine
- [ ] `cmd/ze-chaos/peer/event.go` - Event struct with ChaosAction string field
- [ ] `cmd/ze-chaos/report/jsonlog.go` - NDJSON format for chaos events

**Behavior to preserve:**
- Existing 10 chaos actions unchanged
- Existing NDJSON format backwards compatible (old records still valid)
- Scheduler behavior unchanged when new actions not enabled

**Behavior to change:**
- ChaosAction struct extended with Params field
- Event struct ChaosAction field may carry parameter info
- NDJSON records for chaos events include chaos-params when present
- --chaos-actions flag to opt-in to new action types in scheduler
- Replay understands parameterized actions

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
| TestClockDriftAction | `chaos/actions_test.go` | ClockDrift created with drift param, validates range | |
| TestRouteBurstAction | `chaos/actions_test.go` | RouteBurst with count and family params | |
| TestWithdrawalBurstAction | `chaos/actions_test.go` | WithdrawalBurst clamped to announced count | |
| TestRouteFlapAction | `chaos/actions_test.go` | RouteFlap with cycles and interval params | |
| TestSlowPeerAction | `chaos/actions_test.go` | SlowPeer with delay and duration params | |
| TestZeroWindowAction | `chaos/actions_test.go` | ZeroWindow with duration param | |
| TestSchedulerNewActionsDisabledByDefault | `chaos/scheduler_test.go` | Tick() never returns new action types without opt-in | |
| TestSchedulerChaosActionsFilter | `chaos/scheduler_test.go` | --chaos-actions filters to specified types | |
| TestActionParamsSerialization | `chaos/actions_test.go` | Params serialize to map for JSON logging | |
| TestJSONLogChaosParams | `report/jsonlog_test.go` | Parameterized events include chaos-params | |
| TestJSONLogBackwardsCompat | `report/jsonlog_test.go` | Non-parameterized events unchanged | |
| TestReplayParameterizedActions | `replay/replay_test.go` | Parameterized events replay correctly | |

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

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-chaos-route-burst | `test/chaos/route-burst.ci` | Run with --chaos-actions=RouteBurst, verify burst events in log | |
| test-chaos-replay-params | `test/chaos/replay-params.ci` | Replay log with parameterized actions, verify same result | |

## Files to Modify

- `cmd/ze-chaos/chaos/scheduler.go` - Add --chaos-actions filter, new action weights
- `cmd/ze-chaos/chaos/actions.go` - Extend ChaosAction with Params field (or create if not exists)
- `cmd/ze-chaos/peer/simulator.go` - Handle 6 new action types in chaos switch
- `cmd/ze-chaos/peer/event.go` - (Minor) Ensure Event can carry action params
- `cmd/ze-chaos/report/jsonlog.go` - Add chaos-params field to event records
- `cmd/ze-chaos/replay/replay.go` - Parse chaos-params from log records
- `cmd/ze-chaos/main.go` - Add --chaos-actions flag

## Files to Create

- `cmd/ze-chaos/chaos/actions_v2.go` - New action type definitions with parameter validation
- `cmd/ze-chaos/chaos/actions_v2_test.go` - Tests for new actions
- `test/chaos/route-burst.ci` - Functional test
- `test/chaos/replay-params.ci` - Functional test

## Implementation Steps

1. **Extend ChaosAction (TDD)** - Add Params map, parameter validation per type
2. **Implement action execution in simulator (TDD)** - Handle each new type
3. **Extend NDJSON format (TDD)** - chaos-params field, backwards compat
4. **Extend replay parser (TDD)** - Parse chaos-params from log records
5. **Add scheduler filter (TDD)** - --chaos-actions flag, opt-in for new types
6. **Functional tests** - Route burst with event log, replay with params

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| ClockDrift action | ❌ Not implemented | — | No v2 actions built |
| RouteBurst action | ❌ Not implemented | — | |
| WithdrawalBurst action | ❌ Not implemented | — | |
| RouteFlap action | ❌ Not implemented | — | |
| SlowPeer action | ❌ Not implemented | — | |
| ZeroWindow action | ❌ Not implemented | — | |
| Parameterized NDJSON logging | ⚠️ Partial | `report/jsonlog.go:139` LogControl | "control" record type exists for control events, but no chaos-params field for parameterized actions |
| Replay support for params | ❌ Not implemented | — | No parameterized action replay |
| --chaos-actions filter | ❌ Not implemented | — | No opt-in flag for new action types |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ❌ Not implemented | — | ClockDrift not built |
| AC-2 | ❌ Not implemented | — | ClockDrift validation not built |
| AC-3 | ❌ Not implemented | — | RouteBurst not built |
| AC-4 | ❌ Not implemented | — | WithdrawalBurst not built |
| AC-5 | ❌ Not implemented | — | WithdrawalBurst clamping not built |
| AC-6 | ❌ Not implemented | — | RouteFlap not built |
| AC-7 | ❌ Not implemented | — | SlowPeer not built |
| AC-8 | ❌ Not implemented | — | ZeroWindow not built |
| AC-9 | ❌ Not implemented | — | chaos-params NDJSON field not built |
| AC-10 | ❌ Not implemented | — | Parameterized replay not built |
| AC-11 | ❌ Not implemented | — | --chaos-actions filter not built |
| AC-12 | ✅ Done (existing) | — | Without --chaos-actions, existing behavior unchanged (backwards compat) |

### Audit Summary
- **Total items:** 21
- **Done:** 1 (AC-12 — existing behavior preserved by default)
- **Partial:** 1 (NDJSON control records exist but no chaos-params)
- **Not implemented:** 19 (entire spec not started)

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
