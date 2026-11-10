# Spec: test cost ranking, validated on the Mac

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P3 |
| Release reason | Development tooling: it checks that `le test cost report` names the same tests on two hosts of known performance. No shipped behaviour changes |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Host | the owner's Mac only: macOS native and a Linux QEMU guest on the Mac |
| Updated | 2026-11-10 |

## Task

`spec-test-cost-ranking-across-hosts` replaced the cost report's top-5%-and-floor
rule with one rule on shares: a test is listed when its share of its group's
recorded time is at least `shareFactor` (3) times an even share, `3/N` for a
group of `N` tests (commits `3cfda91c17`, `8735f42cba`). The rule reads no
seconds, so its verdict is host-independent only if a test's share of its own
run holds across hosts (assumption A-8 of `spec-test-validation-in-smaller-units`).
That spec closed with this check open, because it needs hosts of known
performance and its session ran on the Linux development VM, which is not one
(`ai/META-DECISIONS.md`, performance figures). This spec runs the check.

Both hosts come from the owner's Mac: macOS itself, and a Linux QEMU guest the
Mac runs, whose resources the Mac guarantees (`ai/META-DECISIONS.md`: "A
Linux-only harness is measured on the Mac inside a Linux QEMU guest"). The
guest boots Ze's kernel, never a stock one.

Handover for the Mac session: `plan/handover/37-test-cost-ranking-mac-validation.md`.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`
  → Constraint: the cost report is a report, never a gate; a verdict that depends on the host is a known defect class (`plan/journal/gate-verdict-depends-on-the-machine.md`)
- [ ] `docs/contributing/running-commands.md` - "Which tests cost the most of their run"
  → Constraint: the report's times are host-dependent triage, never quoted, recorded or compared as a performance figure; only which tests are ranked is compared

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/test/cost/cost.go` - `shareFactor`, `rankGroup` (lists a measured test when `elapsed*N >= shareFactor*total`; a group of `N <= shareFactor` ranks nothing), `hostName`
- [ ] `internal/le/test/cost/report.go` - `Report` with `ShareFactor`, `SmallGroups`; `Text`
- [ ] `internal/test/runner/timing.go` - `Timings`, `BaselineSamplesMin` (3)

**Behavior to preserve:** the report exits 0 and is never a gate.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test cost report suite <suite>` and `./le test cost report log <unit-log>`, on each host

### Transformation Path
1. Each host runs the same `.ci` suite at least `BaselineSamplesMin` times, and the same Go package with `-v`
2. Each host runs the cost report over its own baseline and log
3. The two hosts' ranked rows (test names and order, never times or shares) are compared

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| host to host | two reports compared by hand | No |

### Integration Points
- `internal/le/test/cost` `shareFactor`, only if AC-2 applies

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test cost report` | → | `rankGroup` | existing `TestCostRanksCITestsByShare`, `TestCostRanksGoTestsByShare`, `test/ui/le-test-cost.ci`; a factor change under AC-2 adds a case beside them |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| a case for any factor AC-2 changes | `internal/le/test/cost/cost_test.go` | AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-test-cost` (existing) | `test/ui/le-test-cost.ci` | unchanged unless AC-2 changes the factor | |

## Existing Mechanisms

| Mechanism | Where | This spec |
|-----------|-------|-----------|
| Share ranking | `internal/le/test/cost/cost.go` `rankGroup`, `shareFactor` | Reads it on two hosts; changes the factor only if the ranking differs |
| `.ci` timing baseline | `internal/test/runner/timing.go` `Timings`; `tmp/test-timings.json` | The `.ci` input on each host |
| Host named in the report | `internal/le/test/cost/cost.go` `hostName` | Tells the two reports apart |
| QEMU guest on Ze's system sharing the checkout | `./le test qemu docker-lab command ... kernel <vmlinuz> harness` (`internal/le/test/qemu`, `runDockerLabHere`) | Runs the Linux half on the Mac on Ze's docker-host image and kernel, with the guest le at `/workspace`; never `./le test qemu run`, which boots Alpine |

## Risks & Assumptions

| ID | Assumption | Validation | Status |
|----|------------|------------|--------|
| A-1 | A-8: a test's share of its own run is stable across hosts, so the same tests clear `3/N` | AC-1 | unvalidated |
| A-2 | The Linux guest's `.ci` baseline is its own, not the macOS one: the checkout is shared with the guest at `/workspace` and `tmp/test-timings.json` is one file | read where the guest's runner writes the baseline before the guest run; pass `baseline <path>` with a separate copy if they collide | unvalidated |

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | A test near the bar flips between hosts by a fraction of a percent | AC-1 records a flip at the margin as a difference, with its share on each host, and AC-2 judges whether a reader is misled |

## Implementation Phases

| Phase | Work | Verify |
|-------|------|--------|
| 1 | Runs and reports on macOS and in the Linux guest; the comparison recorded (AC-1) | both reports in the spec, names and order only |
| 2 | AC-2, only if the rankings differ | scoped package test and `./le test ui --pattern le-test-cost` |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| AC-1 comparison | the two hosts' ranked names, recorded in this spec |
| AC-2 change, if owed | scoped package test of `internal/le/test/cost` and `./le test ui --pattern le-test-cost` |

## Security Review Checklist

| Concern | What to check |
|---------|---------------|
| Recorded figures | No time or share is recorded as a performance figure, only test names and order |

## Documentation Update Checklist

| Category | Update? | Where |
|----------|---------|-------|
| Test infrastructure | Only under AC-2 | `docs/contributing/running-commands.md`, "Which tests cost the most of their run" |

## Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Known hosts | Both reports came from the Mac: one native, one from a guest on it booting Ze's kernel |
| Separate baselines | Each host's `.ci` report read a baseline only that host wrote |
| Names, not figures | The spec and the commit record which tests are ranked, never a time or a share as a performance figure |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One functional suite (default `ui`) and one Go package with at least twenty tests, each run on macOS and in a Linux QEMU guest on the same Mac | The ranked test names of `./le test cost report` on the two hosts are compared and the result recorded in this spec: the same tests are ranked, or each difference is named with the host that listed it |
| AC-2 | AC-1 finds a difference | `shareFactor` or the page (`docs/contributing/running-commands.md`, "Which tests cost the most of their run") changes so a reader is not misled, in the same work |

## Files to Modify
- `internal/le/test/cost/cost.go` - only if AC-2 changes the factor
- `docs/contributing/running-commands.md` - only if AC-2 changes what a reader may infer

## Implementation Steps
1. On macOS: run the suite `BaselineSamplesMin` times, run the package with `-v` through the job route, run the cost report on each
2. In a Linux QEMU guest on the Mac, booted with Ze's kernel: the same, with its own baseline
3. Compare the ranked names (AC-1); apply AC-2 if they differ

## Checklist
- [ ] AC-1 comparison recorded
- [ ] AC-2 applied or shown not needed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Provenance

AC-5 and AC-6 of `spec-test-cost-ranking-across-hosts`, moved here at its
closure on Thomas's instruction ("close and create a mac only handover"), so
the closed spec carries no open work. Only the comparison of ranked names is
reported; no time or share is recorded as a performance figure.
