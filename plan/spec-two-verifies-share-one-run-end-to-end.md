# Spec: two verifies share one run, proven through the real binary

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Prove through the shipped `le` binary that two `le verify` invocations of one
mode over one tree produce ONE run and two identical verdicts.

`runCurrent` and the `verify worktree` lifecycle claim the `verify` label before
any work (`internal/le/verify/current.go`, `internal/le/verify/lifecycle.go`),
and `TestTwoVerifiesShareOneRun` (`internal/le/verify/current_test.go`) drives
that path in two real operating-system processes: the second re-executes the
test binary, attaches, runs no stage, and exits with the holder's status. What
it does NOT drive is the `le` binary itself, so nothing proves that the
composition root gives a user at the command line the behavior the package
tests measure.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the admission seam and what a verification claims before it works
- [ ] `docs/functional-tests.md` - the `.ci` grammar and the runner suite this scenario would join

**Key insights:**
- The admission is already correct. What is missing is a proof that reaches the binary.
- The obstacle is the stage population a real `verify current` runs, not the sharing.

## Current Behavior

**Source files read:**
- [ ] `internal/le/verify/current.go` - `currentHere` hands `runCurrent` the native in-process dispatcher, so a real invocation runs the real stages.
- [ ] `internal/le/verify/actions.go` - `actionRunner`, `setActionRunner`: the composition root supplies the dispatcher, and a test cannot name a different one from outside the package.
- [ ] `internal/le/verify/engine/stages.go` - `StagesForMode`: the 45-stage changed-mode population a scenario would drive.
- [ ] `internal/le/verify/current_test.go` - `TestTwoVerifiesShareOneRun`: the two-process proof that exists, and the assertions a `.ci` would carry.
- [ ] `test/runner/verify-reds-in-flight.ci` - the sibling scenario that landed, and the fixture shape to mirror.

**Behavior to preserve:**
- The verify entry point runs the real stage population for a real user. A test-only stage list that the product never takes is a second code path (`ai/rules/no-layering.md`).

**Behavior to change:**
- A scenario in `test/runner/` drives two real `le verify` processes and asserts one run directory, one holder, one follower, and one shared verdict.

## Data Flow

### Entry Point
- `ze-test fixture runner/verify-shares-one-run` starts two `le verify` processes against a throwaway checkout.

### Transformation Path
1. The fixture builds a throwaway git checkout with one commit.
2. It starts the first `le verify` and waits for the `verify` entry to appear in `tmp/.ze-jobs/`.
3. It starts the second, which reads that entry, matches label, key and `InputHash`, and attaches.
4. Both processes exit; the fixture reads both statuses and counts the run directories under `tmp/verify/`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| fixture ↔ `le` binary | two child processes, statuses read with no pipe between | No |
| holder ↔ follower | the registry entry and its log file | No |

### Integration Points
- `internal/test/fixture` - where the driver and its `register_*.go` registration live.
- `job.Admit` / `(*Admission).attach` - the behavior under test, unchanged by this spec.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| two concurrent `le verify` processes | → | `runCurrent` reaching `Admit`, and `attach` answering the second | `test/runner/verify-shares-one-run.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two `le verify` processes started together over one checkout | Exactly one run directory appears, and both processes exit with the same status |
| AC-2 | The second process's output | Says it attached, and names no stage it ran |
| AC-3 | The scenario's wall time | Fits inside the runner suite's budget rather than running a whole verification twice |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTwoVerifiesShareOneRun` | `internal/le/verify/current_test.go` | AC-1, AC-2 at package level; exists and passes today | pass |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `verify-shares-one-run` | `test/runner/verify-shares-one-run.ci` | Two verifies started together produce one run and two identical verdicts | absent |

## Files to Modify
- `internal/le/verify/actions.go` - whatever seam lets a scenario drive the entry point over a bounded stage population without giving the product a second path.

## Files to Create
- `test/runner/verify-shares-one-run.ci`
- `internal/test/fixture/misc_fixture_runner_shares.go`
- `internal/test/fixture/register_verify_shares_one_run.go`

## Implementation Steps

**Step 1: settle the seam.** A real `le verify current mode changed` runs 45
stages beginning with `verify lint/run`, and `verify lint/run` and the
staticcheck matrix measured 1220 to 2341 seconds each on 2026-09-05. So the
scenario cannot drive the population the product runs. Decide how a scenario
reaches the entry point over a bounded population, or find a different
observation point that reaches the binary. `ai/rules/no-layering.md` governs the
answer: a test-only branch the product never takes is not one.

**Step 2: write the scenario** against that seam, mirroring
`test/runner/verify-reds-in-flight.ci` and its fixture.

## Provenance

Homed here 2026-09-05 when `spec-verification-answers-one-agent` closed. That
spec listed `test/runner/verify-shares-one-run.ci` in Files to Create and left
the row's Status cell empty; its sibling `test/runner/verify-reds-in-flight.ci`
was written and passes. Closure found the file absent and homed the row here
rather than dropping it, because dropping an item is the owner's decision.
Thomas has not commissioned the work.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
