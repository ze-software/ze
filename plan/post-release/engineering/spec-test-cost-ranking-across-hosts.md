# Spec: test cost ranking across hosts

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P3 |
| Release reason | Development tooling: it changes which tests `le test cost report` names, and checks the names hold on two machines. No shipped behaviour changes |
| Scope | tooling |
| Depends | - |
| Phase | 3/3 |
| WIP cap | claimed above the cap (19/19) on Thomas's one-off exception for this spec |
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
| AC-5 (moved to `spec-test-cost-ranking-mac-validation` AC-1 at closure, on Thomas's instruction) | One functional suite and one Go package, each run on two hosts of known performance (the owner's Mac and a real Linux host, `ai/META-DECISIONS.md`, performance figures) | The ranked rows of `./le test cost report` on the two hosts are compared, and the result is recorded: the same tests are ranked, or the difference is named |
| AC-6 (moved to `spec-test-cost-ranking-mac-validation` AC-2 at closure) | AC-5 finds the ranking differs between the hosts | `shareFactor` or the page change so a reader is not misled, in the same work |
| AC-7 | Any change above | `docs/contributing/running-commands.md`, "Which tests cost the most of their run", states the share rule, the small-group case and the absent floor, in the same change |

## Files to Modify
- `internal/le/test/cost/cost.go` - share bar in `rankGroup`; delete `topPercent`, `floorGo`, `floorCI`, `route.floor`; restate the threshold comment and `Why`
- `internal/le/test/cost/report.go` - `ShareFactor` replaces `FloorSeconds` and `TopPercent`; header and empty-group sentences
- `internal/le/test/cost/golog.go` - the comment that cites a 2 s floor
- `internal/le/test/cost/cost_test.go` - the cases above
- `test/ui/le-test-cost.ci` - rewritten for the share rule
- `docs/contributing/running-commands.md` - AC-7

## Risks & Assumptions

| ID | Assumption | Validation | Status |
|----|------------|------------|--------|
| A-1 | No caller outside `internal/le/test/cost` reads `floor-seconds`, `top-percent`, `topPercent`, `floorGo`, `floorCI` or `route.floor` | grep over the tree, excluding plan/ | confirmed 2026-11-10: only the package itself and the `<!-- source: -->` anchor in `running-commands.md` |
| A-2 | A-8 of the parent spec: shares are stable across hosts | AC-5 on two known hosts | moved, not confirmed: it is A-1 of `spec-test-cost-ranking-mac-validation`, with AC-5 and AC-6 |

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | Fast packages list a dominant test that costs under a second | The row shows its seconds; recorded as a decision |

## Implementation Phases

| Phase | Work | Verify |
|-------|------|--------|
| 1 | Rewrite the unit cases and `le-test-cost.ci` for the share rule (tests only) | main thread: the tests go red against the current rule; `ze-test-judge` verdict on each rewrite |
| 2 | Share bar in `rankGroup`, `Report.ShareFactor`, delete the old constants, restate `Why`, comments and the page (AC-7) | main thread: scoped package test and `./le test ui le-test-cost` green |
| 3 | AC-5 and AC-6 on the two known hosts | owner-run reports, compared |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Share bar in `rankGroup`, old cut-offs deleted | `grep -n -E 'topPercent|floorGo|floorCI|FloorSeconds|TopPercent' internal/le/test/cost/*.go` finds nothing |
| Unit tests for AC-1 to AC-4 | `./le job run label unit-pkg quiet command go test -race -tags "$tags" ./internal/le/test/cost/` exit 0 |
| `.ci` over the real entry point | `./le test ui --pattern le-test-cost` passes against `le` built from HEAD |
| Page states the rule (AC-7) | `docs/contributing/running-commands.md`, "Which tests cost the most of their run" |
| AC-5, AC-6 homed | `plan/post-release/engineering/spec-test-cost-ranking-mac-validation.md`, handover `plan/handover/37-test-cost-ranking-mac-validation.md` |

## Security Review Checklist

| Concern | What to check |
|---------|---------------|
| Untrusted input | The baseline and the Go log are read from paths the caller names; a huge value in either cannot overflow the integer bar comparison into a wrong verdict at realistic sizes, and an absurd one only misranks a triage report that exits 0 |
| Division by zero | A group whose recorded time is zero ranks nothing (`TestCostGroupWithNoRecordedTimeRanksNothing`) |

## Documentation Update Checklist

| Category | Update? | Where |
|----------|---------|-------|
| Test infrastructure | Yes | `docs/contributing/running-commands.md`, "Which tests cost the most of their run", in `3cfda91c17` |
| CLI reference | Yes | the action's `Why` text in `internal/le/test/cost/cost.go`, rendered by `le help test cost` |
| Every other category | No | the change is le tooling with no config, wire, RFC, plugin or operator surface |

## Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| One set | The share's denominator and `N` count the same tests |
| No seconds in the verdict | Nothing in `rankGroup` compares a duration with a constant |
| No layering | `topPercent`, `floorGo`, `floorCI`, `route.floor`, `FloorSeconds`, `TopPercent` are gone, not kept |
| Small group | `N <= 3` lists nothing and the text says why |
| Exact boundary | A share exactly at `3/N` is listed; the comparison is not distorted by float rounding in a way a test can observe |
| Page | `running-commands.md` states the rule the code applies, with no 5% or floor left |

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

## Implementation Summary

### What Was Implemented
- `3cfda91c17`: `rankGroup` (`internal/le/test/cost/cost.go`) lists a measured test when `elapsed*N >= shareFactor*total` in integer nanoseconds, `N` counting every test of the group; `tooFewToRank` makes a group of `N <= shareFactor` rank nothing; `topPercent`, `floorGo`, `floorCI` and `route.floor` are deleted. `Report.ShareFactor` and `Report.SmallGroups` (`report.go`) replace `FloorSeconds` and `TopPercent`, and `Report.Text` states the share rule and each too-small group. The action's `Why` derives its text from `shareFactor`. Tests and `test/ui/le-test-cost.ci` rewritten; page updated.
- `8735f42cba`: `TestCostGroupWithNoRecordedTimeRanksNothing` pins the `total == 0` guard in `rankGroup`.

### Bugs Found/Fixed
- None found at closure.

### Documentation Updates
- `docs/contributing/running-commands.md`, "Which tests cost the most of their run": the share rule, the small-group case and the absent floor, source anchors `shareFactor, answerReport, rankGroup` and `Report, SmallGroup, Report.Text, notice` (in `3cfda91c17`).
- `docs/architecture/testing/runner-architecture.md` describes the report as ranking by share with no threshold named: still true, unchanged.
- `./le doc check all`, run by the main thread: red only on the wiki command catalog drift and a stale anchor in `docs/architecture/api/text-format.md`, neither from this change.

### Deviations from Plan
- AC-5 and AC-6 moved to `plan/post-release/engineering/spec-test-cost-ranking-mac-validation.md` on Thomas's instruction to close and hand the Mac-only work over (`plan/handover/37-test-cost-ranking-mac-validation.md`).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first draft of the Mac handover ran the Linux half through `./le test qemu run kernel <path>` | That action boots an Alpine guest whatever kernel it is given; Ze's own system is `./le test qemu docker-lab command ... harness` (`runDockerLabHere`, `internal/le/test/qemu/actions.go`) | Closure review read the action's `Why` and parameters | Handover step 4 and the new spec's mechanism row corrected before they landed |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Replace the top-5% count and the seconds floors with one share bar | Done | `internal/le/test/cost/cost.go` `rankGroup`, `shareFactor` | |
| Validate A-8 for the new rule on two hosts | Changed | `plan/post-release/engineering/spec-test-cost-ranking-mac-validation.md` AC-1, AC-2 | moved on Thomas's instruction |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestCostRanksCITestsByShare`, `TestCostBarIsInclusive`, `TestCostBarCountsUnmeasuredTests`, `TestCostRanksGoTestsByShare` | |
| AC-2 | Done | `TestCostRanksCITestsByShare`, `test/ui/le-test-cost.ci` seq 1 | |
| AC-3 | Done | `TestCostSmallGroupRanksNothing` | text line asserted |
| AC-4 | Done | `TestCostRankingIgnoresScale` | scales 1, 7, 1000 |
| AC-5 | Changed | moved to `spec-test-cost-ranking-mac-validation` AC-1 | Work Not Done |
| AC-6 | Changed | moved to `spec-test-cost-ranking-mac-validation` AC-2 | Work Not Done |
| AC-7 | Done | `docs/contributing/running-commands.md`, "Which tests cost the most of their run" | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestCostRanksCITestsByShare` rewritten | Done | `internal/le/test/cost/cost_test.go` | judge-approved |
| bar boundary | Done | `TestCostBarIsInclusive` | |
| group of three | Done | `TestCostSmallGroupRanksNothing` | |
| scale | Done | `TestCostRankingIgnoresScale` | |
| `TestCostRanksGoTestsByShare` rewritten | Done | `internal/le/test/cost/cost_test.go` | judge-approved |
| `le-test-cost` rewritten | Done | `test/ui/le-test-cost.ci` | judge-approved |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/test/cost/cost.go` | Done | `3cfda91c17` |
| `internal/le/test/cost/report.go` | Done | `3cfda91c17` |
| `internal/le/test/cost/golog.go` | Done | `3cfda91c17`, `millisecondsOf` comment |
| `internal/le/test/cost/cost_test.go` | Done | `3cfda91c17`, `8735f42cba` |
| `test/ui/le-test-cost.ci` | Done | `3cfda91c17` |
| `docs/contributing/running-commands.md` | Done | `3cfda91c17` |

### Audit Summary
- **Total items:** 21
- **Done:** 19
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (AC-5, AC-6 moved, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A test is listed on its share alone, so a dominant second test is named and the count follows cost, not suite size | functional | `./le test ui --pattern le-test-cost` PASS against le build `20261110T221758Z-3cfda91c17`: `demo: A` (40%) and `demo: B` (30%) listed, `demo: C` (14%) rejected; every new unit test was red against the old rule before the code changed (main thread log `tmp/session/2026-11-10-121211/scratch/job-unit-pkg-5f20139d.log`) |
| The verdict reads no seconds, so it cannot flip with host speed | unit | `TestCostRankingIgnoresScale` ranks X, Y at scales 1, 7 and 1000; nothing in `rankGroup` compares a duration with a constant |
| A-8 holds on two hosts | cross-host comparison | not done here: `plan/post-release/engineering/spec-test-cost-ranking-mac-validation.md` AC-1 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| AC-5: compare the ranked rows on two hosts of known performance | needs the owner's Mac; this session ran on the Linux development VM; Thomas ordered closure with a Mac-only handover | `plan/post-release/engineering/spec-test-cost-ranking-mac-validation.md` |
| AC-6: change `shareFactor` or the page if the rankings differ | depends on AC-5 | `plan/post-release/engineering/spec-test-cost-ranking-mac-validation.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/test-cost-ranking-across-hosts-ca636815-7415-4846-b553-86ca52f89c23.md` (9 files, verdict clean) |
| `./le spec review check` | clean |
| Rounds | 1 |
| Reviewer lenses used | wiring, removed-behavior audit (the floor's div-by-zero guard is re-established as `total == 0`), boundary arithmetic and overflow bound, untrusted input (baseline negatives dropped by `runner.ReadTimings`, Go log negatives by the regex and `Elapsed < 0`), Go style pass (no new `panic`, loops bounded by input), stale comments, page drift; pre-checks `./le repo check` all passed, `./le test weakened check base 3cfda91c17~1` clean |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | 0 BLOCKER, 0 ISSUE over the code diff | | |

NOTEs: a report whose only groups are too small prints both the "no test takes at least 3x" line and the too-few line, accurate and left as is; a baseline `avg-ms` past `time.Duration`'s range converts implementation-defined, as before this change, and only misranks a report that exits 0.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/test/cost/{cost,report,golog,cost_test}.go` | yes | `ls internal/le/test/cost/` |
| `test/ui/le-test-cost.ci` | yes | `ls test/ui/le-test-cost.ci` |
| `plan/post-release/engineering/spec-test-cost-ranking-mac-validation.md`, `plan/handover/37-test-cost-ranking-mac-validation.md` | yes | carried in commit A |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-4 | the share bar, the boundary, the small group, the scale | main thread: `./le job run label unit-pkg quiet command go test -race -tags "$tags" ./internal/le/test/cost/` exit 0 |
| AC-1 | old cut-offs gone | `grep -c -E 'topPercent|floorGo|floorCI|FloorSeconds|TopPercent' internal/le/test/cost/*.go` is 0 in every file |
| AC-7 | page states the rule | `running-commands.md` carries "A group of three tests or fewer ranks nothing" |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le test cost report suite <suite>` | `test/ui/le-test-cost.ci` | read: seq 1 runs `le test cost report suite demo baseline /dev/stdin` through `answerReport`; PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | grep outside `plan/` finds no reader of the old names or JSON fields |
| A-2 | moved, not confirmed | A-1 of `spec-test-cost-ranking-mac-validation` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Test infrastructure page | `rankGroup`, `tooFewToRank`, `shareFactor` read at closure | yes |
| CLI reference (`Why`) | `cost.go` `actions` derives the text from `shareFactor` | yes |
