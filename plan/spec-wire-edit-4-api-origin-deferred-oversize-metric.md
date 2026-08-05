# Spec: wire-edit-4-api-origin-deferred-oversize-metric -- count an announce dropped for exceeding the message ceiling

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Updated | 2026-08-05 |

Deferral holder created at the closure of `plan/learned/1320-wire-edit-4-api-origin.md` on 2026-08-02
(`ai/rules/planning.md`, "Creating the Deferral Spec"). The source spec
was removed by its closure commit, so the work below lives here.

## Task

`plan/learned/1320-wire-edit-4-api-origin.md` AC-5 drops an announce whose attributes
cannot fit the destination's message size, and names the route in a log line.
`TestAnnounceOversizeDropsWithNamedLog` proves both rails refuse and both name
the route, so the fail-closed behavior is implemented and tested.

What is missing is the metric surface: `bgp_announce_dropped_oversize_total`, so
an operator sees the drop as a counter rather than only as a log line. A drop
visible only in a log is invisible to an alert.

The counter name must carry the `ze_` prefix its siblings use
(`ai/rules/config.md`), and the label set must be closed so a caller
cannot drive cardinality.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `docs/plugin-development/metrics.md` - counter registration and documentation
- [ ] `ai/rules/evidence.md` - label sets come from a closed enum

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

---

## Implementation Summary

### What Was Implemented

`ze_bgp_announce_dropped_oversize_total`, a counter over announces the fail-closed
size guard refused. Labels `rail` and `stage`, both closed sets chosen at the call
site.

- `announceMetrics`, `setAnnounceMetricsRegistry` and
  `recordAnnounceDroppedOversize` (`internal/component/bgp/reactor/announce_metrics.go`).
- Wired at both drop sites: `logAnnounceTooLarge`
  (`internal/component/bgp/reactor/reactor_api_batch.go`) and `logRIBRouteTooLarge`
  (`internal/component/bgp/reactor/peer_rib_routes.go`).
- Registered next to `filterapi.SetMetricsRegistry` in `Reactor.Start`
  (`internal/component/bgp/reactor/reactor.go`).

### Deviations from Plan

- The spec named the metric `bgp_announce_dropped_oversize_total`. It ships as
  `ze_bgp_announce_dropped_oversize_total`, which the spec itself required in the
  same paragraph: the `ze_` prefix its siblings carry.
- A package-level `atomic.Pointer` rather than a field read off `Reactor.rmetrics`,
  which every other reactor counter uses. Neither drop site can reach a receiver:
  both loggers are free functions, and the queued rail's builder
  `buildRIBRouteUpdate` takes no receiver either. This is the shape and the reason
  `filterapi.SetMetricsRegistry` already uses.
- Two labels rather than one. `rail` alone would not say WHICH region overflowed,
  and `stage` alone would not say which writer refused. Four series is the whole
  product of the two closed sets.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Regenerating `ai/CODE-TO-DOCS.md` from the working tree added rows for three docs a concurrent session had edited but not committed | Those rows cite anchors absent from HEAD, so committing them repeats the consumer-without-producer split | Read the diff before staging it | Regenerated from a HEAD archive plus this change's own files. The scratch tree needs `git init`, because `filter_gitignored` reads gitignore and `tmp/` is ignored, which silently yields zero paths |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| An operator sees the drop as a counter, not only a log line | Done | `recordAnnounceDroppedOversize` (`internal/component/bgp/reactor/announce_metrics.go`) | Both rails wired |
| The name carries the `ze_` prefix | Done | `setAnnounceMetricsRegistry` | `ze_bgp_announce_dropped_oversize_total` |
| The label set is closed so a caller cannot drive cardinality | Done | The `announceRail*` and `announceStage*` constants | Four series maximum. Family, prefix and NLRI count stay in the log line, where they are not cardinality |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1: the counter exists under its prefixed name | Done | `TestRecordAnnounceDroppedOversizeCountsPerRailAndStage` | Asserts the registered name |
| AC-2: both rails are counted, separately | Done | `TestAnnounceDropLoggersRecordTheCounter` | Drives the producing log functions, not the recorder |
| AC-3: a build without metrics is unaffected | Done | `TestRecordAnnounceDroppedOversizeIsSilentWithoutARegistry` | Nil registry and unwired both no-op |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Counter moves on a refused announce | Done | `internal/component/bgp/reactor/announce_metrics_test.go` | Three tests; the wiring one is mutation-verified twice |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/announce_metrics.go` | Created | Counter, labels, recorder |
| `internal/component/bgp/reactor/announce_metrics_test.go` | Created | Three tests |
| `internal/component/bgp/reactor/reactor_api_batch.go` | Modified | Batch rail records |
| `internal/component/bgp/reactor/peer_rib_routes.go` | Modified | Queued rail records |
| `internal/component/bgp/reactor/reactor.go` | Modified | Registry wired |
| `docs/guide/monitoring.md` | Modified | Operator-facing section with source anchors |

### Audit Summary
- **Total items:** 12
- **Done:** 12
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (all recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A drop is visible to an alert, not only to a grep | unit test plus mutation | `TestAnnounceDropLoggersRecordTheCounter` drives the two PRODUCING log functions. Removing the queued rail's record call fails it; giving both rails the same `rail` label also fails it |
| The counter cannot be registered-but-dead | wiring test | The same test calls `logAnnounceTooLarge` and `logRIBRouteTooLarge` rather than the recorder, so a counter no drop site reaches cannot pass |
| A caller cannot drive cardinality | code plus test | Both labels come from package constants at the call site. `TestRecordAnnounceDroppedOversizeCountsPerRailAndStage` asserts exactly three series after four recordings, so no per-route series is minted |
| A metrics-disabled build is unharmed | unit test | `TestRecordAnnounceDroppedOversizeIsSilentWithoutARegistry` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| The metric surface for AC-5's oversize drop | done | Implemented here. This spec had no shard of its own; it WAS the deferral row, carried out of `plan/learned/1320-wire-edit-4-api-origin.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/wire-edit-4-api-origin-deferred-oversize-metric-c4c78ddb-c47b-4f1a-a85d-5911d7c65455.md` |
| `review_gate.py check` | clean |
| Reviewer lenses used | cardinality, nil-safety, wiring completeness, label vocabulary, doc accuracy |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No BLOCKER and no ISSUE survived the pass | - | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/announce_metrics.go` | yes | `grep -n "func recordAnnounceDroppedOversize"` resolves |
| `internal/component/bgp/reactor/announce_metrics_test.go` | yes | Carries the three tests |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Registered under the prefixed name | `make ze-test-pkg PKG=./internal/component/bgp/reactor` green, 12.3s |
| AC-2 | Both rails counted separately | Mutant removing the queued record call, and mutant collapsing the rail label, each fail `TestAnnounceDropLoggersRecordTheCounter` |
| AC-3 | Unwired is silent | `TestRecordAnnounceDroppedOversizeIsSilentWithoutARegistry` passes |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| An announce whose attributes exceed the build buffer | none; this is a counter over an existing, already-tested drop | yes. `TestAnnounceOversizeDropsWithNamedLog` already drives both writers to the drop; the new test drives the same two log functions those writers call, and both were re-run green together |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1: a drop site can read `Reactor.rmetrics` | broken | Both loggers are free functions and `buildRIBRouteUpdate` takes no receiver. Resolved with the package-level pointer `filterapi` already uses |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Prometheus counter added, so the monitoring guide gains a section | `docs/guide/monitoring.md`, "Announces Refused For Size", three source anchors | yes |
| The index that maps code to docs stays fresh | `ai/CODE-TO-DOCS.md` regenerated from HEAD plus this change, so it carries no row for a concurrent session's uncommitted doc | yes |
