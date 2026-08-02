# Spec: wire-edit-4-api-origin-deferred-oversize-metric -- count an announce dropped for exceeding the message ceiling

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Updated | 2026-08-02 |

Deferral holder created at the closure of `plan/spec-wire-edit-4-api-origin.md` on 2026-08-02
(`ai/rules/deferral-tracking.md`, "Creating the Deferral Spec"). The source spec
was removed by its closure commit, so the work below lives here.

## Task

`plan/spec-wire-edit-4-api-origin.md` AC-5 drops an announce whose attributes
cannot fit the destination's message size, and names the route in a log line.
`TestAnnounceOversizeDropsWithNamedLog` proves both rails refuse and both name
the route, so the fail-closed behavior is implemented and tested.

What is missing is the metric surface: `bgp_announce_dropped_oversize_total`, so
an operator sees the drop as a counter rather than only as a log line. A drop
visible only in a log is invisible to an alert.

The counter name must carry the `ze_` prefix its siblings use
(`ai/rules/config-naming.md`), and the label set must be closed so a caller
cannot drive cardinality.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `docs/plugin-development/metrics.md` - counter registration and documentation
- [ ] `ai/rules/derive-not-hardcode.md` - label sets come from a closed enum

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `internal/component/bgp/reactor/reactor_metrics.go` (`reactorMetrics`, the sibling counters)
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` (the drop site AC-5 already implements)

**Behavior to preserve:** the drop itself, its log line and the named route are already correct and must not change.

## Data Flow (MANDATORY)

### Entry Point
An API announce whose attributes exceed the destination's maximum message size.

### Transformation Path
(fill during design)

### Boundaries Crossed
| From | To | Format |
|------|----|--------|
| (fill during design) | (fill during design) | (fill during design) |

### Integration Points
| Point | Component |
|-------|-----------|
| (fill during design) | (fill during design) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The drop site is reachable on both announce rails from one counted helper. | `TestAnnounceOversizeDropsWithNamedLog` drives both rails through the same refusal. | Two increment sites, which must then be proven to agree. | (fill during design) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A counter incremented on one rail only reads as half the drops, which is worse than none. | (fill during design) | (fill during design) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| An operator announces a route too large for the peer | -> | the announce is dropped and the counter increments | `test/plugin/` announce oversize `.ci`, or an extension of `TestAnnounceOversizeDropsWithNamedLog` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An oversize announce on the batch rail | `ze_bgp_announce_dropped_oversize_total` increments by one |
| AC-2 | The same on the queued rail | The same counter increments by one |
| AC-3 | The counter's label set | Closed, from a compile-time enum |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestAnnounceOversizeIncrementsCounter | `internal/component/bgp/reactor/reactor_metrics_behavioral_test.go` | AC-1, AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/plugin/announce-oversize-counter.ci` | `test/plugin/` | an operator scrapes the counter after an oversize announce is dropped | |

## Files to Modify
- `internal/component/bgp/reactor/reactor_metrics.go`
- `internal/component/bgp/reactor/reactor_api_batch.go`
- `docs/plugin-development/metrics.md`

## Implementation Steps

1. (fill during design)

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] `make ze-verify` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
