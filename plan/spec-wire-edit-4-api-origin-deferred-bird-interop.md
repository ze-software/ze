# Spec: wire-edit-4-api-origin-deferred-bird-interop -- a live BIRD peer accepts an API-originated route

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Updated | 2026-08-02 |

Deferral holder created at the closure of `plan/learned/1320-wire-edit-4-api-origin.md` on 2026-08-02
(`ai/rules/deferral-tracking.md`, "Creating the Deferral Spec"). The source spec
was removed by its closure commit, so the work below lives here.

## Task

`plan/learned/1320-wire-edit-4-api-origin.md` converged the two announce rails on one
writer. Its interop row was not reached in the implementation session.

The property is currently proven by unit tests over both rails and by
`test/plugin/wire-edit-api-origin-order.ci`, which pins the exact wire bytes
through the daemon. A live peer is stronger evidence and is still owed: an
interop scenario in which BIRD accepts an API-originated route and installs the
attributes in the expected order.

`ai/rules/interop-and-goal-validation.md` makes an interop test mandatory for a
protocol feature, so this is an owed deliverable, not an optional extra.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `ai/rules/interop-and-goal-validation.md` - required interop assertion per feature type
- [ ] `test/interop/scenarios/` - the scenario layout and `check.py` contract

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` (the announce rail under test)
- [ ] `test/plugin/wire-edit-api-origin-order.ci` (the byte-level property this must confirm against a real peer)

**Behavior to preserve:** the emitted bytes are already pinned; the scenario must confirm them against BIRD, never relax them.

## Data Flow (MANDATORY)

### Entry Point
`ze` announces a route through the API to a BIRD peer in the interop lab.

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
| A-1 | BIRD's route dump exposes attribute values in a form the check can assert. | Existing scenarios under `test/interop/scenarios/` assert against BIRD output. | Assert on session survival plus the installed prefix and next-hop only, and say so. | (fill during design) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A scenario that only asserts the session stays up proves nothing about attribute order. | (fill during design) | (fill during design) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| An operator announces a route with communities through the API | -> | the shared writer emits it; BIRD installs it | `test/interop/scenarios/NN-wire-edit-api-origin/check.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An API-originated route with COMMUNITIES and LARGE_COMMUNITIES | BIRD installs the route and reports both values |
| AC-2 | The same route | The session stays established after the exchange |
| AC-3 | The scenario reverted against the pre-convergence encoder | The check fails, proving it discriminates |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (interop scenario, not a Go unit test) | `test/interop/scenarios/NN-wire-edit-api-origin/check.py` | AC-1, AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/interop/scenarios/NN-wire-edit-api-origin/check.py` plus existing `test/plugin/wire-edit-api-origin-order.ci` | `test/interop/scenarios/` | a live BIRD peer installs an API-originated route | |

## Files to Modify
- `test/interop/scenarios/NN-wire-edit-api-origin/` (new)

- `internal/component/bgp/reactor/reactor_api_batch.go` - re-read only; the scenario must not need a code change

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
