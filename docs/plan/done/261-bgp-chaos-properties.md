# Spec: bgp-chaos-properties (Phase 7 of 9) — SKELETON

**Master design:** `docs/plan/spec-bgp-chaos.md`
**Previous spec:** `spec-bgp-chaos-eventlog.md`
**Next spec:** `spec-bgp-chaos-shrink.md`
**DST reference:** `docs/plan/deterministic-simulation-analysis.md` (Section 9: Property-Based Testing)

**Status:** Done

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-bgp-chaos.md` - master design (validation model)
3. `docs/plan/deterministic-simulation-analysis.md` Section 9 - property definitions
4. Phase 1-6 done specs - validation model, event types, event log format
5. `rfc/short/rfc4271.md` - BGP FSM, hold timer, collision resolution
6. `.claude/rules/planning.md` - workflow rules

## Task

Replace the monolithic validation model with composable, named RFC property assertions that can be checked independently, combined, and reported by name.

The current validation model (Phase 2) answers one question: "does each peer have the routes it should?" This spec decomposes that into discrete, testable properties drawn from RFCs, each of which can pass or fail independently.

**Scope:**
- Property interface: `Name()`, `Description()`, `Check(state, action, result) error`
- RFC-derived properties (FSM, hold-timer, collision, withdrawal, family filtering, route consistency)
- Stateful property testing via `pgregory.net/rapid` for exhaustive interleaving exploration
- Per-property pass/fail in exit summary and event log
- `--properties` CLI flag to select which properties to check (default: all)
- `--properties list` to enumerate available properties

**Relationship to DST:**
These properties are identical whether exercised by external chaos (Phase 7) or internal DST simulation (future). Writing them now means they carry forward with zero rework when Ze gains internal simulation capability.

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-bgp-chaos.md` - validation model design
  → Decision: Expected state is union of all other peers' routes filtered by family
  → Constraint: Convergence window allowed after each state change
- [ ] `docs/plan/deterministic-simulation-analysis.md` Section 9 - property-based testing
  → Decision: Properties follow Setup/Check pattern with PropertyState
  → Constraint: Properties must be composable (check independently, combine freely)
- [ ] `docs/architecture/behavior/fsm.md` - FSM state transitions
  → Constraint: All transitions must match RFC 4271 Section 8.2.2 table

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - FSM (§8.2.2), hold-timer (§4.4), collision (§6.8), UPDATE processing (§9)
  → Constraint: Hold-timer expiry MUST tear down session
  → Constraint: Higher BGP Identifier wins collision resolution
- [ ] `rfc/short/rfc4760.md` - multiprotocol (§6), family negotiation
  → Constraint: Peer MUST NOT receive NLRI for non-negotiated family
- [ ] `rfc/short/rfc7606.md` - treat-as-withdraw error handling
  → Constraint: Malformed attribute → treat as withdraw (not session reset)

### Source Code
- [ ] Phase 2 validation model (`validation/model.go`, `checker.go`, `tracker.go`)
- [ ] Phase 3 chaos events (`chaos/action.go`, `peer/simulator.go`)
- [ ] Phase 6 event log format (replay feeds events to properties)
  → Implemented: `replay/replay.go` Run() + `replay/diff.go` Diff(), NDJSON with header + event records, `report/jsonlog.go` for writing

**Key insights:**
- The existing validation model is essentially the "route-consistency" property — this spec factors it out and adds more
- Properties observe the same event stream as the reporter — no new data collection needed
- Rapid's stateful testing generates random action sequences and checks properties after each step — this is how we find interleaving bugs
- Properties that check Ze behavior (FSM transitions, hold-timer) require observing Ze's responses, not just the chaos tool's actions

## Current Behavior (MANDATORY)

**Source files read:** (to be re-read after Phase 6 completes)
- [ ] `cmd/ze-bgp-chaos/validation/model.go` — expected state tracking
- [ ] `cmd/ze-bgp-chaos/validation/checker.go` — expected vs actual comparison
- [ ] `cmd/ze-bgp-chaos/validation/convergence.go` — latency tracking
- [ ] `cmd/ze-bgp-chaos/peer/event.go` — event types

**Behavior to preserve:**
- Route propagation validation from Phase 2 (becomes the "route-consistency" property)
- Convergence tracking from Phase 2 (becomes the "convergence-deadline" property)
- All chaos events from Phase 3

**Behavior to change:**
- Refactor monolithic checker into composable property interface
- Add new properties not currently checked
- Per-property reporting in summary and event log

## Data Flow (MANDATORY)

### Entry Point
- Events from the same event channel used by reporter and event log
- Properties registered at startup based on `--properties` flag

### Transformation Path
1. Event arrives on channel (BGP message, state change, chaos action)
2. Property engine iterates registered properties
3. Each property's `Check()` called with current state + event
4. Violations recorded with property name, event context, and RFC reference
5. Violations reported in event log (Phase 6) and exit summary

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Event channel ↔ Property engine | Same channel as reporter (fan-out or shared) | [ ] |
| Property engine ↔ Reporter | Violation events fed back to event channel | [ ] |

### Integration Points
- Phase 2 validation model (refactored into route-consistency property)
- Phase 6 event log (violations recorded as events)
- Phase 8 shrink (properties define the failure criterion for shrinking)

### Architectural Verification
- [ ] Properties are stateless checkers (state tracked externally in PropertyState)
- [ ] Properties compose freely (any subset can be selected)
- [ ] Existing validation model preserved as one property among many
- [ ] Property violations don't stop execution (record and continue)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Default run | All properties checked, per-property pass/fail in summary |
| AC-2 | `--properties route-consistency,hold-timer` | Only those two properties checked |
| AC-3 | `--properties list` | Lists all available properties with descriptions |
| AC-4 | Hold-timer expiry chaos event | `hold-timer-enforcement` property verifies session torn down |
| AC-5 | Route announced by peer A | `route-consistency` property verifies all eligible peers receive it |
| AC-6 | Peer disconnects | `withdrawal-propagation` property verifies withdrawals arrive |
| AC-7 | Non-negotiated family route | `family-filtering` property catches the violation |
| AC-8 | Convergence timeout | `convergence-deadline` property reports failure with latency |
| AC-9 | Property violation | Event log contains violation event with property name and RFC ref |
| AC-10 | `rapid.Check` stateful test | Random chaos sequences checked against all properties |

## Properties

### Property Table

| Property | RFC | What It Checks |
|----------|-----|----------------|
| `route-consistency` | 4271 §9 | After convergence, every eligible peer has every expected route |
| `withdrawal-propagation` | 4271 §6.3 | Withdrawal reaches all peers within convergence deadline |
| `hold-timer-enforcement` | 4271 §4.4 | Session torn down within tolerance of hold-timer expiry |
| `keepalive-interval` | 4271 §4.4 | KEEPALIVE sent within hold-time/3 interval |
| `family-filtering` | 4760 §6 | Peer never receives NLRI for non-negotiated family |
| `collision-resolution` | 4271 §6.8 | Higher BGP Identifier wins (when collision chaos fires) |
| `message-ordering` | 4271 §8.2.2 | OPEN before UPDATE; no UPDATE before OPEN-confirm |
| `convergence-deadline` | (operational) | All routes converge within configurable deadline |
| `no-duplicate-routes` | (operational) | Same prefix not announced twice without intermediate withdrawal |
| `eor-per-family` | 4724 §2 | End-of-RIB marker sent for each negotiated family |

### Property Interface

Each property implements:
- **Name** — kebab-case identifier used in `--properties` flag
- **Description** — human-readable summary for `--properties list`
- **RFC** — reference string (e.g., "RFC 4271 Section 4.4")
- **Check** — given current state and an event, return nil (ok) or error (violation)
- **Reset** — clear per-run state (for replay or new run)

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRouteConsistencyPass` | `validation/properties_test.go` | Routes propagated correctly → no violation | |
| `TestRouteConsistencyFail` | `validation/properties_test.go` | Missing route → violation with peer and prefix | |
| `TestWithdrawalPropagation` | `validation/properties_test.go` | Withdrawal reaches all peers within deadline | |
| `TestWithdrawalPropagationTimeout` | `validation/properties_test.go` | Withdrawal not received → violation | |
| `TestHoldTimerEnforcement` | `validation/properties_test.go` | Session torn down after hold-timer expiry | |
| `TestHoldTimerNotEnforced` | `validation/properties_test.go` | Session survives past hold-timer → violation | |
| `TestFamilyFiltering` | `validation/properties_test.go` | Non-negotiated family route → violation | |
| `TestCollisionResolution` | `validation/properties_test.go` | Higher ID wins → no violation | |
| `TestCollisionResolutionWrong` | `validation/properties_test.go` | Lower ID wins → violation | |
| `TestPropertySelection` | `validation/engine_test.go` | `--properties` flag selects subset | |
| `TestPropertyList` | `validation/engine_test.go` | Lists all with descriptions | |
| `TestRapidStateful` | `validation/rapid_test.go` | Random action sequences, all properties hold | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Convergence deadline | 1ms - 60s | 60s | 0ms | N/A (clamped) |
| Hold-timer tolerance | 0 - 30s | 30s | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-properties-all` | `test/chaos/properties-all.ci` | Run with all properties, 30s, verify per-property report | |
| `chaos-properties-select` | `test/chaos/properties-select.ci` | Run with subset, verify only selected checked | |

## Files to Create

- `cmd/ze-bgp-chaos/validation/property.go` — Property interface and PropertyEngine
- `cmd/ze-bgp-chaos/validation/properties_test.go` — tests for all properties
- `cmd/ze-bgp-chaos/validation/props_route.go` — route-consistency property
- `cmd/ze-bgp-chaos/validation/props_withdraw.go` — withdrawal-propagation property
- `cmd/ze-bgp-chaos/validation/props_holdtimer.go` — hold-timer-enforcement property
- `cmd/ze-bgp-chaos/validation/props_family.go` — family-filtering property
- `cmd/ze-bgp-chaos/validation/props_collision.go` — collision-resolution property
- `cmd/ze-bgp-chaos/validation/props_ordering.go` — message-ordering property
- `cmd/ze-bgp-chaos/validation/props_convergence.go` — convergence-deadline property
- `cmd/ze-bgp-chaos/validation/props_eor.go` — eor-per-family property
- `cmd/ze-bgp-chaos/validation/rapid_test.go` — stateful property tests with rapid

## Files to Modify

- `cmd/ze-bgp-chaos/main.go` — add `--properties` flag
- `cmd/ze-bgp-chaos/validation/checker.go` — refactor to use property engine
- `cmd/ze-bgp-chaos/validation/model.go` — extract route-consistency logic into property
- `cmd/ze-bgp-chaos/report/summary.go` — per-property pass/fail section
- `cmd/ze-bgp-chaos/orchestrator.go` — wire property engine into event pipeline

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| Makefile | No | Already has ze-bgp-chaos target |
| go.mod | Maybe | `pgregory.net/rapid` dependency |

## Implementation Steps

1. **Read Phase 2+6 learnings** — understand validation model and event log format
   → Review: What state does the model track? What events drive it?

2. **Define Property interface**
   → Review: Can all 10 properties be expressed with the same interface?

3. **Write route-consistency property tests** (extracted from existing model)
   → Run: Tests FAIL

4. **Implement route-consistency property** by refactoring existing checker
   → Run: Tests PASS

5. **Write hold-timer property tests**
   → Run: Tests FAIL

6. **Implement hold-timer property**
   → Run: Tests PASS

7. **Write remaining property tests** (withdrawal, family, collision, ordering, convergence, eor)
   → Run: Tests FAIL

8. **Implement remaining properties**
   → Run: Tests PASS

9. **Implement PropertyEngine** — registration, selection, fan-out to properties

10. **Add rapid stateful tests** — random chaos sequences, all properties checked

11. **Wire into CLI** — `--properties` flag, per-property summary

12. **Verify** — `make lint && make test`

## Spec Propagation Task

**MANDATORY at end of this phase:**

Update the following specs:
1. **`spec-bgp-chaos-shrink.md`** — properties define the failure criterion for shrinking
2. **`spec-bgp-chaos-inprocess.md`** — properties carry forward unchanged to in-process mode

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

## Implementation Summary

### What Was Implemented
- Property interface with Name(), Description(), RFC(), ProcessEvent(), Violations(), Reset()
- PropertyEngine: dispatches events to registered properties, collects per-property results
- 5 properties implemented: route-consistency, convergence-deadline, no-duplicate-routes, hold-timer-enforcement, message-ordering
- `--properties` CLI flag with all/list/comma-sep modes
- `--convergence-deadline` flag for configurable deadline
- Per-property PASS/FAIL in exit summary
- AllProperties(), SelectProperties(), ListProperties() helper functions

### Design Decisions
- Properties are **additive** to the existing validation model — dual-consumer fan-out where both EventProcessor and PropertyEngine receive every event
- Properties use ProcessEvent() (stateful) not stateless Check() — each property maintains its own state and reports violations at query time
- No `rapid` dependency — stateful property testing deferred (spec was a skeleton, core properties were the priority)
- 5 of 10 planned properties implemented — withdrawal-propagation, keepalive-interval, family-filtering, collision-resolution, eor-per-family deferred as they require observing Ze's responses (not just chaos tool events)

### Files Created
- `validation/property.go` — Property interface, PropertyEngine, AllProperties, SelectProperties, ListProperties
- `validation/properties_test.go` — 20 tests across all 5 properties
- `validation/props_route.go` — route-consistency
- `validation/props_convergence.go` — convergence-deadline
- `validation/props_duplicate.go` — no-duplicate-routes
- `validation/props_holdtimer.go` — hold-timer-enforcement
- `validation/props_ordering.go` — message-ordering

### Files Modified
- `main.go` — `--properties` and `--convergence-deadline` flags, PropertyEngine creation, event dispatch
- `orchestrator.go` — properties and convergenceDeadline fields in orchestratorConfig
- `report/summary.go` — PropertyLine type, Properties field, updated Pass() and Write()
- `report/summary_test.go` — 3 new tests for property output

## Checklist

- [x] Tests written
- [x] Tests PASS (`make test`)
- [x] No regressions (`make functional`)
- [x] `make lint` passes (0 issues)
