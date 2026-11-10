# Spec: test cost ranking across hosts

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P3 |
| Release reason | Development tooling: it changes which tests `le test cost report` names, and checks the names hold on two machines. No shipped behaviour changes |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

## Task

`spec-test-validation-in-smaller-units` built `./le test cost report`, which
lists a test when it is in the top `topPercent` (5) of its own suite or package
run, counted in tests, and its time is above its route's floor (`floorGo` 2 s,
`floorCI` 15 s), ranked by share (`internal/le/test/cost/cost.go`,
`rankGroup`). A `.ci` test is ranked only from `runner.BaselineSamplesMin` (3)
baseline samples (`internal/test/runner/timing.go`). Its assumption A-8 says a
test's share of its own run is stable enough across hosts to rank candidates,
where its absolute time is not. That spec could not validate it: it needs two
machines, and the session had one.

That rule has two cut-offs that work against what it measures, and Thomas
approved replacing them with one rule on share alone:

1. **The 5% counts tests, not time.** A 20-test suite lists one test whatever
   the time split: with A at 40% and B at 30% of the run, B is never named.
   The number listed follows the suite's size, not its cost.
2. **The floors are absolute seconds.** A 14 s `.ci` is dropped on one host and
   listed on a host twice as slow, at the same 14% share. The floor alone can
   flip the ranking between hosts, which is the variable A-8 set out to remove.

This spec replaces both with: **a test is listed when its share of its group's
recorded time is at least `shareFactor` (3) times an even share, `1/N` for a
group of `N` tests.** It then validates A-8 for the new rule on two hosts.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`
  → Constraint: a check whose verdict depends on the host is a known defect class (`plan/journal/gate-verdict-depends-on-the-machine.md`); the cost report is a report, never a gate
  → Constraint: le is a personality of `cmd/ze`; the report answers through the shared renderer, never a private JSON mode
- [ ] `docs/contributing/running-commands.md` - "Which tests cost the most of their run"
  → Constraint: its times are host-dependent triage, never quoted, recorded or compared as a performance figure
- [ ] `ai/rules/cli.md`
  → Constraint: the report's JSON fields `floor-seconds` and `top-percent` are an agent-facing output; the change replaces them, it does not keep them beside the new field
- [ ] `ai/rules/no-layering.md`
  → Constraint: `topPercent`, `floorGo`, `floorCI` and `route.floor` are deleted, never kept as a fallback beside the share rule

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/test/cost/cost.go` - `rankGroup` sums the group's time over every test, sorts the tests with enough samples, keeps the top `ceil(5% of candidates)`, lists those above the route floor; `hostName` names the host. The comment above the thresholds gives the floor's purpose: it keeps a suite of fast tests from listing its least-fast one. The action's `Why` text restates the old rule
  → Constraint: the share's denominator is the whole group's time (unmeasured tests included) while the 5% counts only measured tests; the new rule uses one set for both
- [ ] `internal/le/test/cost/report.go` - `Report.FloorSeconds`, `Report.TopPercent`, and the text lines "listed: the top N% of each ... above Ns" and "no test is in the top N% of its ... and above Ns"
- [ ] `internal/le/test/cost/golog.go` - `millisecondsOf` comment cites the 2 s floor
- [ ] `internal/le/test/cost/cost_test.go` - `TestCostRanksCITestsByShare` (40-test suite, top 5% is two, an at-floor third; a two-test suite pins the floor boundary), `TestCostRanksGoTestsByShare` (2 s floor boundary)
- [ ] `test/ui/le-test-cost.ci` - a suite with two long tests, an at-floor test, nineteen 1 s tests and a one-sample test; rejects `demo: at-floor`
- [ ] `internal/test/runner/timing.go` - `Timings` per suite and test, `BaselineSamplesMin`

**Behavior to preserve:** the report exits 0 and is never a gate; a `.ci` test
with fewer than `BaselineSamplesMin` samples is listed as unmeasured, never
ranked; rows sort ranked first by share, then unmeasured; the report names the
host.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test cost report suite <suite>` and `./le test cost report log <unit-log>`

### Transformation Path
1. The baseline or Go log yields one measured entry per test, grouped by suite or package
2. `rankGroup` computes each test's share of its group's total and lists it when the share is at least `shareFactor / N`
3. `Report.Text` and the JSON renderer state the rule and the rows
4. On each of two hosts the same suite and package are run, reported, and the ranked rows compared

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| report to reader | text and JSON through the shared renderer | the unit tests and `le-test-cost.ci` |
| host to host | two reports compared by hand | No |

### Integration Points
- `internal/le/test/cost` thresholds and `Report` fields

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test cost report suite <suite>` | → | `rankGroup` | `test/ui/le-test-cost.ci`, rewritten for the share rule |
| `./le test cost report log <path>` | → | `rankGroup` | `TestCostRanksGoTestsByShare`, rewritten for the share rule |

## Decisions

- → Decision: the bar is `share >= shareFactor / N`, with `shareFactor = 3` and `N` every test in the group, measured or not, so the share and the bar count the same set. The bar uses only shares, so no part of the verdict reads seconds or the host.
- → Decision: no absolute floor. Its job (a suite of fast tests not naming its slowest) is given up: a package whose whole run takes under a second can now list a test that dominates it. The report is triage of one suite or package at a time, and a reader sees the seconds on the row, so the cost is a row the reader dismisses, never a wrong verdict. A host-independent replacement for the floor does not exist, because "cheap" is a time.
- → Decision: a group of `N <= shareFactor` tests lists nothing, because `3/N >= 100%` and only a lone test reaches 100%. A group of three or fewer tests is read whole without a report. The report's text says so when it lists nothing for such a group.
- → Decision: the JSON field `share-factor` replaces `floor-seconds` and `top-percent`; nothing keeps the old two.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCostRanksCITestsByShare`, rewritten: a 20-test suite with A 40%, B 30%, C 14%, seventeen sharing 16%; A and B listed, C not (bar 15%), a one-sample test unmeasured | `internal/le/test/cost/cost_test.go` | AC-1, AC-2 | |
| the bar boundary: a test at exactly `3/N` listed, one just under not | `internal/le/test/cost/cost_test.go` | AC-1 | |
| a group of three tests lists nothing, however split | `internal/le/test/cost/cost_test.go` | AC-3 | |
| the same suite at every time doubled ranks the same rows (the verdict reads no seconds) | `internal/le/test/cost/cost_test.go` | AC-4 | |
| `TestCostRanksGoTestsByShare`, rewritten: a package ranked by the same bar from a `-v` and a `-json` log, with no 2 s boundary | `internal/le/test/cost/cost_test.go` | AC-1 | |

The rewritten cases replace assertions of the old rule (the top-5% count, the
floor boundary, `at-floor`). Each rewrite is judged by an independent
`ze-test-judge` agent before it lands, on the ground that the old expectation
encodes the replaced rule (`ai/META-DECISIONS.md`, test changes).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-test-cost` (rewritten) | `test/ui/le-test-cost.ci` | the 20-test baseline above: `demo: A` and `demo: B` listed with their shares, `demo: C` rejected, the one-sample test listed as unmeasured, the header states the 3x-even-share rule | |

## Existing Mechanisms

| Mechanism | Where | This spec |
|-----------|-------|-----------|
| Cost ranking | `internal/le/test/cost/cost.go` `rankGroup`, `topPercent`, `floorGo`, `floorCI`, `route.floor` | Replaces the two cut-offs with the share bar inside `rankGroup`; deletes the constants and `route.floor` (`route.name` stays) |
| Report header and JSON | `internal/le/test/cost/report.go` `Report`, `Text` | Replaces `FloorSeconds`, `TopPercent` and their sentences with `ShareFactor` |
| `.ci` timing baseline | `internal/test/runner/timing.go` `Timings`, `BaselineSamplesMin` | Unchanged; still decides unmeasured |
| Host named in the report | `internal/le/test/cost/cost.go` `hostName` | Tells the two hosts' reports apart in AC-5 |
| The command's `Why` text | `internal/le/test/cost/cost.go` (`Why:` "list the tests in the top 5% ...") | Restated for the share rule |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A group of `N` tests (`N > 3`) | A measured test is listed when its share of the group's total is at least `3/N`, and not otherwise; rows keep their share-descending order |
| AC-2 | The 20-test example (A 40%, B 30%, C 14%, seventeen sharing 16%) | A and B listed, C not |
| AC-3 | A group of three or fewer tests | No ranked row; the text says the group is too small to rank |
| AC-4 | The same group with every time scaled by a constant | The same rows are listed |
| AC-5 | One functional suite and one Go package, each run on two hosts of known performance (the owner's Mac and a real Linux host, `ai/META-DECISIONS.md`, performance figures) | The ranked rows of `./le test cost report` on the two hosts are compared, and the result is recorded: the same tests are ranked, or the difference is named |
| AC-6 | AC-5 finds the ranking differs between the hosts | `shareFactor` or the page change so a reader is not misled, in the same work |
| AC-7 | Any change above | `docs/contributing/running-commands.md`, "Which tests cost the most of their run", states the share rule, the small-group case and the absent floor, in the same change |

## Files to Modify
- `internal/le/test/cost/cost.go` - share bar in `rankGroup`; delete `topPercent`, `floorGo`, `floorCI`, `route.floor`; restate the threshold comment and `Why`
- `internal/le/test/cost/report.go` - `ShareFactor` replaces `FloorSeconds` and `TopPercent`; header and empty-group sentences
- `internal/le/test/cost/golog.go` - the comment that cites a 2 s floor
- `internal/le/test/cost/cost_test.go` - the cases above
- `test/ui/le-test-cost.ci` - rewritten for the share rule
- `docs/contributing/running-commands.md` - AC-7

## Implementation Steps
1. Rewrite the unit cases and `le-test-cost.ci` for the share rule; confirm they go red against the current rule (paste output); get the `ze-test-judge` verdict on each rewrite
2. Replace the cut-offs in `rankGroup` and the `Report` fields; delete the old constants; confirm green
3. Update the page and the stale comments (AC-7)
4. Commit; this piece stands on its own before the cross-host check
5. On the two known hosts, run the suite and the package, run the cost report on each, compare the ranked rows (AC-5); apply AC-6 if they differ

## Checklist
- [ ] AC-1 to AC-4 implemented and tested
- [ ] AC-5 comparison recorded
- [ ] AC-6 applied or shown not needed
- [ ] AC-7 page updated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Provenance

A-8 of `spec-test-validation-in-smaller-units`, carried here by its closure
because it could not be validated on one host. The share-only rule replaced
the top-5%-and-floor rule on Thomas's approval, after the review that found
the count cut-off hides a dominant second test and the seconds floor makes the
ranking depend on the host. Only the cross-host comparison is reported; no
time or share is recorded as a performance figure.
