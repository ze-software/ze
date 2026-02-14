# Spec: bgp-chaos-chaos (Phase 3 of 5) — SKELETON

**Master design:** `docs/plan/spec-bgp-chaos.md`
**Previous spec:** `spec-bgp-chaos-validation.md`
**Next spec:** `spec-bgp-chaos-families.md`

**Status:** Skeleton — to be fleshed out after Phase 2 completes.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-bgp-chaos.md` - master design (chaos event table, weights)
3. Phase 1 + Phase 2 done specs - learnings and actual APIs
4. `.claude/rules/planning.md` - workflow rules

## Task

Add chaos event injection to `ze-bgp-chaos`: a seed-based scheduler that disrupts peer sessions at configurable rates, testing Ze's route server resilience.

**Scope:**
- Chaos scheduler: seed-based event selection with configurable rate and interval
- All 10 chaos event types (see master design)
- Warmup period before chaos begins
- Validation integration: chaos events trigger expected-state updates
- Reconnection lifecycle: full disconnect → reconnect → route replay cycle

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-bgp-chaos.md` - master design (chaos event table with weights)
  → Decision: 10 event types with weighted random selection
  → Constraint: Warmup period before chaos starts
- [ ] `docs/architecture/core-design.md` - BGP session lifecycle, NOTIFICATION codes
  → Constraint: NOTIFICATION cease/admin-reset = code 6, subcode 4

### Source Code
- [ ] Phase 1 + Phase 2 implementation files (paths TBD)
  → Constraint: Must use actual session API for disconnect/reconnect
  → Constraint: Must use validation model API for expected-state updates

**Key insights:**
- _To be filled after Phase 2 completes_

## Current Behavior (MANDATORY)

**Source files read:** (to be re-read after Phase 2 completes)
- [ ] `cmd/ze-bgp-chaos/main.go` — CLI entry point, flag parsing
- [ ] `cmd/ze-bgp-chaos/peer/simulator.go` — per-peer goroutine with event reporting
- [ ] `cmd/ze-bgp-chaos/peer/session.go` — TCP + OPEN + KEEPALIVE exchange
- [ ] `cmd/ze-bgp-chaos/peer/sender.go` — route UPDATE building and sending
- [ ] `cmd/ze-bgp-chaos/orchestrator.go` — multi-peer lifecycle coordination
- [ ] `cmd/ze-bgp-chaos/validation/model.go` — expected state model

**Behavior to preserve:**
- All Phase 1 + 2 functionality
- Validation correctness through chaos events

**Behavior to change:**
- Add chaos event injection layer on top of existing orchestration

## Data Flow (MANDATORY)

### Entry Point
- Chaos scheduler runs as goroutine alongside orchestrator
- Picks events based on seed RNG, dispatches to peer simulators

### Transformation Path
1. Scheduler tick (every `--chaos-interval`)
2. RNG check against `--chaos-rate`
3. If firing: select event type (weighted), select target peer (random established)
4. Execute event on peer simulator
5. Update validation model (expected-state change)
6. Peer handles aftermath (reconnect if disconnected)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Scheduler ↔ Peer | Method call or channel | [ ] |
| Scheduler ↔ Validation | Method call | [ ] |

### Integration Points
- Phase 2 orchestrator (for peer selection)
- Phase 2 validation model (for expected-state updates)
- Phase 1 session (for disconnect/reconnect methods)

### Architectural Verification
- [ ] Chaos events use same session API as normal operation
- [ ] Validation model stays consistent through chaos

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | --chaos-rate 0.0 | No chaos events fired |
| AC-2 | --chaos-rate 1.0 --chaos-interval 1s | Event every ~1 second |
| AC-3 | Same seed, same chaos-rate | Identical event sequence |
| AC-4 | TCP disconnect event | Peer disconnects, others get withdrawals, peer reconnects |
| AC-5 | NOTIFICATION event | Clean disconnect with cease code |
| AC-6 | Hold-timer expiry event | Ze detects and tears down session |
| AC-7 | Partial withdrawal event | Subset withdrawn, peer stays connected |
| AC-8 | Disconnect during initial burst | Partial routes withdrawn, remaining sent after reconnect |
| AC-9 | --warmup 10s | No chaos during first 10 seconds |
| AC-10 | All chaos events over 5-minute run | Validation still tracks correctly |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSchedulerDeterministic` | `chaos/scheduler_test.go` | Same seed → same event sequence | |
| `TestSchedulerRateZero` | `chaos/scheduler_test.go` | Rate 0.0 → no events | |
| `TestSchedulerRateOne` | `chaos/scheduler_test.go` | Rate 1.0 → event every interval | |
| `TestSchedulerWarmup` | `chaos/scheduler_test.go` | No events during warmup period | |
| `TestEventWeights` | `chaos/events_test.go` | Weighted selection matches expected distribution | |
| `TestDisconnectEvent` | `chaos/executor_test.go` | Closes TCP, triggers reconnect | |
| `TestNotificationEvent` | `chaos/executor_test.go` | Sends NOTIFICATION, closes cleanly | |
| `TestHoldTimerEvent` | `chaos/executor_test.go` | Stops KEEPALIVEs | |
| `TestPartialWithdrawalEvent` | `chaos/executor_test.go` | Withdraws subset of routes | |
| `TestDisconnectDuringBurst` | `chaos/executor_test.go` | Interrupts initial route sending | |

_Additional tests to be identified after Phase 2._

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| chaos-rate | 0.0-1.0 | 1.0 | N/A (clamp) | N/A (clamp) |
| chaos-interval | >0 | any duration | 0 | N/A |
| warmup | >=0 | any duration | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-disconnect-recovery` | `test/chaos/disconnect-recovery.ci` | Disconnect peer, verify withdrawal + replay | |
| `chaos-all-events` | `test/chaos/all-events.ci` | High chaos rate, verify validation accuracy | |

## Files to Create

- `cmd/ze-bgp-chaos/chaos/scheduler.go` - seed-based event scheduling
- `cmd/ze-bgp-chaos/chaos/scheduler_test.go`
- `cmd/ze-bgp-chaos/chaos/events.go` - event type definitions
- `cmd/ze-bgp-chaos/chaos/events_test.go`
- `cmd/ze-bgp-chaos/chaos/executor.go` - event execution on peers
- `cmd/ze-bgp-chaos/chaos/executor_test.go`

## Files to Modify

- `cmd/ze-bgp-chaos/orchestrator.go` - integrate chaos scheduler
- `cmd/ze-bgp-chaos/peer/simulator.go` - chaos event handling
- `cmd/ze-bgp-chaos/peer/session.go` - reconnection support
- `cmd/ze-bgp-chaos/peer/sender.go` - partial/full withdrawal methods

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| Makefile | No | Already added in Phase 1 |

## Implementation Steps

1. **Read Phase 1 + 2 learnings** - understand session API, validation model
   → Review: Can I disconnect a peer? Resume KEEPALIVEs? Trigger partial withdrawal?

2. **Write scheduler tests** - determinism, rate control, warmup
   → Run: Tests FAIL

3. **Implement scheduler** - seed-based event selection
   → Run: Tests PASS

4. **Write event type tests** - weighted selection
   → Run: Tests FAIL

5. **Implement event types** - all 10 events
   → Run: Tests PASS

6. **Write executor tests** - event execution on peers
   → Run: Tests FAIL

7. **Implement executor** - dispatch events, handle reconnection
   → Run: Tests PASS

8. **Wire into orchestrator** - scheduler alongside peer management

9. **Verify** - `make lint && make test`

10. **Update follow-on specs** (Spec Propagation Task)

## Spec Propagation Task

**MANDATORY at end of this phase:**

Before marking this spec complete, update the following specs:

1. **`spec-bgp-chaos-families.md`** — Update with:
   - Does chaos work per-family or per-session?
   - Any chaos events that are family-specific?

2. **`spec-bgp-chaos-reporting.md`** — Update with:
   - Chaos event log format
   - Dashboard chaos section fields
   - Chaos stats for exit summary

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

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Chaos scheduler | | | |
| 10 event types | | | |
| Warmup period | | | |
| Validation integration | | | |
| Reconnection lifecycle | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |
| AC-10 | | | |

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)

### Quality Gates (SHOULD pass)
- [ ] `make lint` passes
- [ ] Follow-on specs updated (Spec Propagation Task)
- [ ] Implementation Audit completed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs

### Completion
- [ ] Spec Propagation Task completed
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-bgp-chaos-chaos.md`
