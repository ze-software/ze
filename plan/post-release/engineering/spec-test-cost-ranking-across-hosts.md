# Spec: test cost ranking across hosts

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Development tooling: it checks that `le test cost report` names the same candidates on two machines. No shipped behaviour changes |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

## Task

`spec-test-validation-in-smaller-units` built `./le test cost report`, which
lists a test when it is in the top `topPercent` (5) of its own suite or package
run and its time is above its route's floor (`floorGo` 2 s, `floorCI` 15 s),
ranked by share (`internal/le/test/cost/cost.go`, `rankGroup`). A `.ci` test is
ranked only from `runner.BaselineSamplesMin` (3) baseline samples
(`internal/test/runner/timing.go`). Its assumption A-8 says a test's share of
its own run is stable enough across hosts to rank candidates, where its
absolute time is not. That spec could not validate it: it needs two machines,
and the session had one. This spec validates it.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`
  → Constraint: a check whose verdict depends on the host is a known defect class (`plan/journal/gate-verdict-depends-on-the-machine.md`); the cost report is a report, never a gate
- [ ] `docs/contributing/running-commands.md` - "Which tests cost the most of their run"
  → Constraint: its times are host-dependent triage, never quoted, recorded or compared as a performance figure

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/test/cost/cost.go` - `rankGroup` sums the group's time, ranks tests with enough samples, lists the top `topPercent` above the route floor; `hostName` names the host
- [ ] `internal/test/runner/timing.go` - `Timings` per suite and test, `BaselineSamplesMin`

**Behavior to preserve:** the report exits 0 and is never a gate.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test cost report suite <suite>` and `./le test cost report log <unit-log>` on each host

### Transformation Path
1. Each host runs the same suite and package and keeps its baseline or `-v` log
2. Each host runs the cost report; the two reports' ranked rows are compared

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| host to host | two reports compared by hand | No |

### Integration Points
- `internal/le/test/cost` thresholds, only if AC-2 applies

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test cost report` | → | `rankGroup` | the existing `TestCostRanksCITestsByShare`, `TestCostRanksGoTestsByShare`; a threshold change under AC-2 adds a case beside them |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| a case for any threshold AC-2 changes | `internal/le/test/cost/cost_test.go` | AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-test-cost` (existing) | `test/ui/le-test-cost.ci` | unchanged unless AC-2 changes a threshold | |

## Existing Mechanisms

| Mechanism | Where | This spec |
|-----------|-------|-----------|
| Cost ranking | `internal/le/test/cost/cost.go` `rankGroup`, `topPercent`, `floorGo`, `floorCI` | Reads it on two hosts; changes it only if the ranking flips |
| `.ci` timing baseline | `internal/test/runner/timing.go` `Timings`, `BaselineSamplesMin`; `tmp/test-timings.json` | The input on each host |
| Host named in the report | `internal/le/test/cost/cost.go` `hostName` | Tells the two reports apart |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One functional suite and one Go package, each run on two hosts of known performance (the owner's Mac and a real Linux host, `ai/META-DECISIONS.md`, performance figures) | The ranked rows of `./le test cost report` on the two hosts are compared, and the result is recorded: the same tests are ranked, or the difference is named |
| AC-2 | The ranking differs between the hosts | The cost report's thresholds or its page (`docs/contributing/running-commands.md`, "Which tests cost the most of their run") change so a reader is not misled, in the same work |

## Files to Modify
- `internal/le/test/cost/cost.go` - only if AC-2 changes a threshold
- `docs/contributing/running-commands.md` - only if AC-2 changes what a reader may infer

## Implementation Steps
1. Run the suite and the package on both hosts, run the cost report on each, compare the ranked rows (AC-1)
2. If they differ, change the thresholds or the page (AC-2)

## Checklist
- [ ] AC-1 comparison recorded
- [ ] AC-2 applied or shown not needed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Provenance

A-8 of `spec-test-validation-in-smaller-units`, carried here by its closure
because it could not be validated on one host. Only the comparison is
reported; no time or share is recorded as a performance figure.
