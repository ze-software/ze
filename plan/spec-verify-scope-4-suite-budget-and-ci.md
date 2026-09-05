# Spec: verify-scope-4-suite-budget-and-ci

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/spec-verify-scope-0-umbrella.md` |
| Phase | 5/5. The cell said 4/5 and was wrong: phase 5 has producers in the tree, `.github/workflows/verify.yml` sharded six ways and `internal/le/workflowcheck` pinning them |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two independent faults, both outside the selector work, and neither depending on
it.

**Fault A: the `plugin` suite runs at its own wall-clock cap.** `ZE_SUITE_TIMEOUT
?= 600s` (`internal/le/functional/suites.go`) caps every suite, and `SUITE_RUN` applies it
through `timeout --kill-after=10s`. On the full verify run of 2026-08-18T21:43
the `plugin` suite measured **599.7 seconds** against that 600 second cap, and
five of its tests failed reporting `start ze-peer: context canceled`
(`bfd-sessions-show` among them). Six of 24 suites went red on that run.

A suite whose runtime equals its cap produces failures that read as product
defects and are not. The gate then teaches nothing, which is the same disease as
the debt ledger: a red nobody can attribute is a red nobody acts on.

**→ Constraint:** the first phase must ESTABLISH the mechanism before changing
anything. `context canceled` is consistent with the cap killing the process
group, and it is equally consistent with per-test budgets starving under
contention at `ZE_PLUGIN_PARALLEL ?= 8`. `plan/journal/parallel-copies-collide-on-a-deterministic-port.md`
records that the same suite gives three disjoint failing sets across three runs
at different `-p` values, so the failing NAMES are not stable and must not be
used as the signal. The stable signal is the suite's total runtime against its
cap.

**Fault B: CI runs the whole hour on one runner.** `.github/workflows/verify.yml`
has one job, `runs-on: ubuntu-latest`, whose only step is `make
./le verify current mode full`. It carries no `timeout-minutes`, so it inherits the
360 minute default. Every push pays the full sequential run on a runner with
far fewer cores than the dev box. `internal/le/` pins
the workflow's shape, so any change here changes that test too.

Note the scale difference from the local box: the owner refused a 38-way local
fan-out because six sessions share 32 cores (umbrella, Owner Decisions). A
GitHub job is a separate machine, so sharding there takes nothing from anybody.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` test file format: embedded files, options, expectations and commands
- [ ] `docs/functional-tests.md` - the suites, the runner, and the per-suite budgets
  → Constraint: a generous timeout is a synonym for an unknown one (`ai/rules/completion.md`)
- [ ] `ai/rules/commands.md` - how test and build commands are invoked
  → Constraint: prefer `make`; a bare `go test` drops feature tags and fakes reds

**Key insights:**
- `plan/journal/parallel-copies-collide-on-a-deterministic-port.md` holds the prior art for this suite's instability, including the measurement that `plugin` gives 604/604 at `-p 8` and 601/604 at `-p 32` on a 16-core host.
- Only the `bgp` and `vpp` paths call `runner.ReservePorts` (`internal/test/runner/ports.go`); suites registered through `registerCIRoot` take a deterministic port and reserve nothing. That is why suites cannot simply be run concurrently, and it bounds what this spec may do about Fault A.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/functional/suites.go` - `ZE_SUITE_TIMEOUT`, `ZE_SUITE_KILL_AFTER`, `SUITE_RUN`, `ZE_PLUGIN_PARALLEL`, the `run_suite` loop
- [ ] `internal/test/runner/ports.go` - `ReservePorts` and its two callers
- [ ] `.github/workflows/verify.yml` - the single job and its single step
- [ ] `internal/le/` - what the workflow's shape is pinned to
- [ ] `tmp/verify/run-20260818T214315Z-./le verify current mode full-34884788/29-./le functional.log` - the measured run

**Behavior to preserve:**
- The per-suite cap stays. It exists because a stuck subprocess holding an output pipe made `cmd.Wait()` block indefinitely, and `timeout` signals the whole process group.
- Exit 124 stays a suite failure like any other.
- CI keeps running every gate that `stagesForMode` lists. Sharding splits WHERE they run, never WHETHER.
- `ZE_SUITE_TIMEOUT` stays overridable from the command line.

**Behavior to change:**
- The `plugin` suite stops running at its cap, by whichever mechanism phase 1 establishes.
- A suite that is killed by its cap says so distinctly, rather than surfacing as N test failures.
- CI runs the stages across more than one job.

## Data Flow (MANDATORY)

### Entry Point
- `./le functional`, and a push or pull request to GitHub.

### Transformation Path
1. `_./le functional` runs `run_suite` per suite, each wrapped in `timeout`.
2. `timeout` signals the suite's process group on expiry and returns 124.
3. `run_suite` counts a non-zero exit as a suite failure and continues.
4. In CI, one job runs every stage in `stagesForMode` in order.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| make ↔ suite process group | `timeout --kill-after` | No |
| Workflow ↔ stage list | `./le verify current mode full` | No |
| Workflow shape ↔ its pin | `internal/le/` | No |

### Integration Points
- `run_suite` - gains the distinct cap-expiry report.
- `stagesForMode` - stays the single stage list; the workflow selects from it rather than duplicating it.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | One number answers three questions: `(Suite).Budget` (`internal/le/functional/budget.go`) is read by the `timeout` that kills the suite, by the runtime line and by the warning, so no consumer holds a second value |
| No unintended coupling (components stay isolated) | Yes | The workflow carries a shard COUNT and an arithmetic rule, never a stage name. `internal/le/workflowcheck` reads the native stage population and the workflow, and neither imports the other |
| No duplicated functionality (extends existing, does not recreate) | Yes | The expiry report reuses the declared-failure-group protocol rather than inventing a second channel: `failureGroupLine` (`internal/le/functional/run.go`) emits the same line the wiring gate and the lint gate emit |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Test tooling and CI, off any data path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `budgetDefaults` is a map keyed by suite name, and `registerScaledParallelKeys` derives its env entries from the suite table rather than listing them. A suite that needs its own budget adds one map entry; a new scaled suite adds none |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `context canceled` failures come from the suite cap rather than from per-test contention | The suite measured 599.7s against a 600s cap | The cap is not the cause and raising it fixes nothing | Phase 1: re-run `plugin` alone with a raised cap and compare the failing set | confirmed |
| A-2 | Raising or removing the cap for this suite does not reintroduce the hang the cap was added for | The cap's comment names a stuck subprocess holding an output pipe | A stuck suite wedges the run again | Keep the cap, raise its value, and keep `--kill-after` | confirmed |
| A-3 | Sharding CI does not need per-shard `ZE_RUN_SLOTS` tuning | Each GitHub job is its own machine | Shards contend and are slower than one job | Measure one sharded run against the current single-job baseline | **CONFIRMED 2026-09-05 at closure, from run 33963534338.** Its six shards took 4837s, 2662s, 2095s, 787s, 647s and 642s. They are separate runners, so nothing contends: 11670s of serial work finished in the 4837s the longest shard took, a 2.4x cut. The BALANCE the spec predicted does not hold on CI hardware, where `./le functional` dominates its shard, and that is what the Known Limitation about splitting the suite already names |

**→ Decision (A-1, phase 1 measurement, 2026-08-19): CONFIRMED. The cap is
binding, and phase 4 must give the `plugin` suite a budget of its own rather
than raise the shared one.**

the retired `ze-functional-plugin-test ZE_SUITE_TIMEOUT=1800s` (current: `./le functional plugin`) ran the suite to
completion in **855 seconds**, which is 142% of the 600 second cap. Under the
shipped cap the suite is killed with about four minutes of work left, so the
2026-08-18 run did not measure a slow suite. It measured a suite that never
reached its end.

The `context canceled` set did not survive. No test reported it, and
`bfd-sessions-show` passed. That is what makes the cap the producer of those
five failures rather than per-test contention.

The run is not clean, and it is unclean for a different reason: 11 tests failed
(`aaa-radius-fallback`, `bfd-auth-meticulous-persist`,
`as112-probe-anycast-not-loopback`, seven `mcp-*`, and id 507). That set
intersects the 2026-08-18 set nowhere, which is the instability
`plan/journal/parallel-copies-collide-on-a-deterministic-port.md` already
records for this suite. It is not evidence about the cap, and this spec must
not chase it.

**The measurement is contaminated, and is recorded as such.** Five other
sessions share this checkout. The load average was 6.56 when the run started
and 18.72 when it ended, on 32 cores. So 855s is an upper bound taken under
contention, and 600s (a kill, so a floor) is the lower bound from the full
verify run. The true runtime is between them and above the cap either way,
which is the only fact phase 4 needs.

**What phase 4 must do.** `ZE_SUITE_TIMEOUT` is shared by all 24 suites, and it
is what protects the other 23. Raising it globally to clear one suite buys
`plugin` its margin by removing everybody else's. Phase 4 gives `plugin` its own
budget at its own `run_suite` line, and leaves the shared default at 600s.
Splitting the suite is the structural answer to 663 tests in one suite, and it
stays available, but it costs the `all_suites` and RFC-tier bookkeeping in R-2
and it is not needed to stop the false reds. Whatever number phase 4 picks,
AC-3's runtime record and warning are what stop it creeping back.

Evidence: `tmp/session/2026-08-19-48ff6743-8005-4e2d-9354-ab0a08c5dd43/scratch/phase1-plugin-cap1800.log`

**→ Decision (phase 4, 2026-08-19): `ZE_SUITE_TIMEOUT_PLUGIN ?= 1500s`, and
`ZE_SUITE_TIMEOUT` stays 600s for the other 23 suites.** The number is derived
from the measurement rather than picked: the warning point, at
`ZE_SUITE_WARN_PERCENT` (80%) of the budget, must sit 40% above the measured
855s, or a contended box warns on every run and the warning names no creep.
855 × 1.40 / 0.80 = 1496s, rounded up to the whole minute. The kill then lands
at 1.75× the measurement, which is a wedge and not a busy box.
`test_the_plugin_budget_keeps_its_warning_above_the_measurement` holds the
derivation, so lowering the budget without making the suite faster is a red.

**→ Decision (phase 4): A-2 is CONFIRMED.** The budget stays finite and stays a
process-group kill. `SUITE_RUN_PLUGIN` is
`timeout --kill-after=$(ZE_SUITE_KILL_AFTER) $(ZE_SUITE_TIMEOUT_PLUGIN)`, and
both `test_the_cap_stays_finite_and_kills_the_process_group` and
`test_every_per_suite_budget_is_wired_on_every_path` refuse an override that
drops `--kill-after`, is not finite, or reaches only one of the two paths that
run the suite.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Raising the cap hides a genuine slowdown instead of fixing it | The suite creeps back to the new cap | Record the suite's runtime per run and fail when it exceeds a recorded fraction of its cap |
| R-2 | Splitting the `plugin` suite changes which `.ci` files are gating, and lowers an RFC tier | `./le rfc check` reports a tier change | Any split adds every new suite name to `all_suites` in the same commit |
| R-3 | Sharding CI drops a stage silently | A gate stops running and nobody notices | The workflow derives its shards from `./le verify current mode full-list`, and a test asserts the union equals the full stage list |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A suite hangs instead of being killed, or a CI shard silently drops a gate. Nothing at runtime |
| How is it reverted? | Single commit revert per fault; the two are independent |
| Who else touches this path? | Every session runs `./le functional`; every push runs the workflow |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a suite that exceeds its cap | → | `(*Run).Record` and `expirySummary` (`internal/le/functional/run.go`) | `TestRecordReportsAKillAsAKill`, `TestTheKillPublishesAFailureGroupWithARerun` |
| `./le functional` | → | `(Suite).Budget` and `WarnPercent` (`internal/le/functional/budget.go`) | `TestBudgetAnswersOneNumberToThreeQuestions`, `TestWarnPercentBoundaries` |
| a push to GitHub | → | the six-shard workflow | `TestVerifyShardsCoverEveryNativeStage`, `TestEachShardRunsOnePieceOfTheStaticcheckMatrix` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The `plugin` suite runs on the dev box | It completes below its cap, with margin, and reports no `context canceled` failure |
| AC-2 | A suite is killed by its wall-clock cap | The run reports the cap expiry as such, naming the suite and its budget, and does not present it as N test failures |
| AC-3 | A suite's runtime exceeds a recorded fraction of its cap | The run says so, so the creep is visible before it becomes a red |
| AC-4 | `./le verify current mode full-list` gains or loses a stage | The CI shards follow it with no second list to edit |
| AC-5 | A push runs CI | The union of the shards' stages equals the full stage list, asserted by a test |
| AC-6 | `./le rfc check` runs before and after | No `.ci` file loses its `functional/verify` tier |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRecordReportsAKillAsAKill`, `TestTheKillPublishesAFailureGroupWithARerun` | `internal/le/functional/functional_test.go` | AC-2: exit 124 reads as a budget expiry, and publishes a failure group carrying its rerun | PASS |
| `TestBudgetAnswersOneNumberToThreeQuestions`, `TestAnOwnBudgetVariableTakesOverTheSuite` | `internal/le/functional/functional_test.go` | AC-3: one budget answers the kill, the runtime line and the warning, and a per-suite variable takes the suite over everywhere | PASS |
| `TestWarnPercentBoundaries`, `TestDurationSecondsMeasuresWhatItCan` | `internal/le/functional/functional_test.go` | AC-3: the warning threshold and the one number a report divides by, at their boundaries | PASS |
| `TestCommandLineWrapsEverySuiteInTimeout` | `internal/le/functional/functional_test.go` | The cap stays finite and applied through `timeout` on the process group, on every path that runs a suite | PASS |
| `TestThePluginBudgetKeepsItsWarningAboveTheMeasurement` | `internal/le/functional/functional_test.go` | The derivation itself: the budget and the warn percentage together must leave the warning above the 855s the suite was measured at | PASS, written at closure |
| `TestVerifyShardsCoverEveryNativeStage` | `internal/le/workflowcheck/workflowcheck_test.go` | AC-4, AC-5: the shards' union is the native stage population, with no second list | PASS |
| `TestEachShardRunsOnePieceOfTheStaticcheckMatrix` | `internal/le/workflowcheck/workflowcheck_test.go` | AC-5: the matrix pieces are dealt one per shard rather than repeated | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ZE_SUITE_TIMEOUT` | 1s-N | N | 0s (kills every suite immediately) | N/A |
| suite runtime as a fraction of cap | 0.0-1.0 | the chosen warn threshold | N/A | 1.0 (the cap fired) |
| CI shard count | 1-N | N | 0 (no stage runs) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `verify-scope-suite-cap` | `test/runner/verify-scope-suite-cap.ci` | A developer's suite exceeds its budget and the run names the budget rather than blaming the tests | replaced by `TestMakeExpandsTheBudgetReport`, see below | <!-- doc-links: ignore (artifact a later phase of this spec will create) -->

**→ Decision (phase 2, 2026-08-19): the `.ci` row is covered by
`TestMakeExpandsTheBudgetReport`, and no `.ci` file was written.** The scenario
needs the report make actually expands, and a `.ci` reaches that only by
re-entering `make` from inside a running functional suite. Every route there is
worse than the test that replaced it: `./le functional runner` re-enters
the suite that is running the `.ci`, and the combined target rebuilds the whole
isolated binary set to print a report string. `TestMakeExpandsTheBudgetReport`
drives the recipe from `make --dry-run` output, with the shipped
`ZE_SUITE_TIMEOUT` value in it and a real `timeout` kill returning 124, and
takes no admission slot. Anything the `.ci` would have added over it is the
tier it runs at, not the behavior it proves.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Scope is tooling. No wire-visible behavior changes | |

## Files to Modify
- `internal/le/functional/budget.go` - the shared budget, the per-suite override, the warn percentage and the duration reader
- `internal/le/functional/run.go` - `(*Run).Record`, `expirySummary` and `failureGroupLine`: the expiry report and the group it publishes
- `.github/workflows/verify.yml` - sharding
- `internal/le/workflowcheck` - the pinned shape follows the shards
- `docs/functional-tests.md` - the budgets and what a cap expiry looks like

## Files to Create
- `internal/le/functional/budget.go` - the cap and runtime contract, with the derivation in its header
- `test/runner/verify-scope-suite-cap.ci` - not written; `TestMakeExpandsTheBudgetReport` carries the end-to-end proof (see the Functional Tests decision above) <!-- doc-links: ignore (artifact a later phase of this spec will create) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Test tooling and CI, no runtime config |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | Make variables only |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | N-A | No RPC or API added |
| Pipe completeness | N-A | No `ze` CLI output added |
| Env var registration | N-A | `ZE_SUITE_TIMEOUT` is a make variable, not a `ze.*` leaf |
| Doctor check for runtime dependencies | N-A | No new runtime path, socket, port, module, or binary |
| Prometheus counters/metrics | N-A | No daemon-observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Developer tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Unless a suite split changes `all_suites`, which AC-6 checks |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: the budgets and the cap-expiry report |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep for anchors naming `test-functional.mk` and `verify.yml` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/functional-tests.md` documents `ZE_SUITE_TIMEOUT` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- establish the mechanism before changing it
   - Tests: `TestRunSuiteReportsCapExpiryDistinctly` written and failing
   - Files: `internal/le/`
   - Verify: re-run `plugin` alone with a raised cap on a quiet tree and record whether the `context canceled` set survives. A-1 is confirmed or broken HERE, and the rest of the phase order depends on the answer
2. **Phase: Cap reporting** -- exit 124 reads as a budget expiry, naming the suite and its cap
   - Tests: `TestRunSuiteReportsCapExpiryDistinctly`, `verify-scope-suite-cap.ci`
   - Files: `internal/le/functional/suites.go`
   - Verify: AC-2 holds
3. **Phase: Runtime record and creep warning** -- record each suite's runtime, warn near the cap
   - Tests: `TestSuiteRuntimeRecorded`
   - Files: `internal/le/functional/suites.go`
   - Verify: AC-3 holds
4. **Phase: The `plugin` suite's budget** -- apply what phase 1 established
   - Tests: the existing `plugin` suite, run to completion with margin
   - Files: `internal/le/functional/suites.go`, and `all_suites` if a split is the answer
   - Verify: AC-1 and AC-6 hold
5. **Phase: CI sharding** -- derive shards from `./le verify current mode full-list`
   - Tests: `TestWorkflowShardsCoverEveryStage`
   - Files: `.github/workflows/verify.yml`, `internal/le/`
   - Verify: AC-4 and AC-5 hold

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | A cap expiry is distinguishable from a test failure in the run's output AND in `tmp/ze-verify-failures.json` |
| Correctness | Phase 1's finding is recorded, and phase 4 acts on it rather than on the assumption |
| Naming | One name for the per-suite budget, used by the make variable and the report |
| Data flow | The CI shards derive from `stagesForMode` through `./le verify current mode full-list`, never from a second list |
| Rule: `ai/rules/completion.md` | Raising a timeout is not a fix on its own. AC-3 exists so the creep stays visible |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `plugin` completes with margin | `grep 'suite plugin' -A 2` in the next full run's stage log |
| A cap expiry is distinct | `TestRunSuiteReportsCapExpiryDistinctly` |
| CI shards cover every stage | `TestWorkflowShardsCoverEveryStage` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Removing or over-raising the cap lets a stuck suite hold the job slot indefinitely, which is what `ze-run.sh` admission exists to prevent. The cap must stay finite |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A suite whose runtime equals its cap is indistinguishable, from the outside, from a suite whose tests are broken. That is why AC-3 records the runtime rather than only raising the number: the measurement is the thing that keeps the fix honest.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Establish the mechanism before changing the cap | Raise the cap and see | The journal records three disjoint failing sets for this suite across three runs, so the failing names are not a signal. The runtime against the cap is |
| Keep the cap finite | Remove it for the `plugin` suite | The cap exists because a stuck subprocess holding an output pipe blocked `cmd.Wait()` indefinitely. Removing it reopens that |
| Shard CI, do not fan out locally | Shard both, or neither | A GitHub job is its own machine, so it takes nothing from the other sessions. The local box is shared, which is why the owner refused the local fan-out |
| Each shard selects its own stages from `./le verify current mode full-list` at run time, by round-robin on the line number | A setup job that runs the same command and emits a JSON matrix; a small number of named shards each holding a stage subset | Both alternatives carry stage NAMES through the YAML, which is the second list this design exists to refuse. Selecting in the shard leaves the workflow holding a count and an arithmetic rule, so `stagesForMode` stays the only list. The count is stated once, in the matrix: the shard reads it back as `strategy.job-total` |
| Six shards | Four, five, eight | Measured on the 2026-08-18 full run: round-robin at six puts `./le functional` (1472s), `./le staticcheck-feature-matrix check` (874s) and `ze-unit-test-race-changed` (638s) on three runners, and the heaviest shard holds 1492s against the 1472s floor that `./le functional` alone sets. Four puts lint, staticcheck and the functional suite together (2924s); eight puts staticcheck back with the functional suite (2349s). More shards buy nothing until that suite is split |
| A shard runs one `make` per stage with `ZE_VERIFY_MODE=1` | `make a b c` in one invocation; omit the variable | `execStage` runs each stage as its own `make --no-print-directory <name>` with `ZE_VERIFY_MODE=1`, and the functional runner reads that variable (`VerifyModeEnabled`) to turn a silent environment skip into a hard failure. One invocation per stage also keeps a red from hiding the stages after it |

## Known Limitations
- This spec does not make the suites run concurrently. Only the `bgp` and `vpp` paths reserve ports, so concurrent suites would collide on deterministic ports. That prerequisite is recorded in `plan/journal/parallel-copies-collide-on-a-deterministic-port.md` and is not in scope here.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- Fault A: `(Suite).Budget`, `BudgetVar`, `WarnPercent`, `KillAfter`, `DurationSeconds` and `budgetDefaults` (`internal/le/functional/budget.go`). One number per suite answers three questions: the `timeout` that kills it, the runtime line, and the warning. `plugin` carries its own `1500s`; the other 23 keep the shared `600s`.
- `(*Run).Record`, `expirySummary` and `failureGroupLine` (`internal/le/functional/run.go`): an exit 124 reports the BUDGET rather than N test failures, and publishes a declared failure group carrying the rerun. The report reuses the protocol the wiring and lint gates already speak instead of a second channel.
- Fault B: `.github/workflows/verify.yml` runs six shards, each selecting its own stages from the native population by round robin on `strategy.job-total`. The workflow carries a count and an arithmetic rule; `internal/le/workflowcheck` asserts the union is the whole population and that the six matrix pieces are dealt one per shard.

### Bugs Found/Fixed
- **The derivation was a comment, not a contract.** The plugin budget is `855 x 1.40 / 0.80 = 1496s`, rounded to 1500s, and the two halves of that arithmetic lived in separate constants with separate tests. Halving `DefaultWarnPercent` would leave both green and make the suite warn on every run, at which point the number means nothing. `TestThePluginBudgetKeepsItsWarningAboveTheMeasurement` now relates them: the warning point must stay above the measured runtime plus its contention margin, and the budget must stay above the measurement itself.

### Documentation Updates
- None needed. `docs/functional-tests.md` carries "Per-suite wall-clock budget" with `ZE_SUITE_TIMEOUT`, `ZE_SUITE_TIMEOUT_PLUGIN` and `ZE_SUITE_WARN_PERCENT` in a table, worked override examples, and "One suite's budget is its own" stating the derivation and why the shared budget was not raised.

### Deviations from Plan
- **`test/runner/verify-scope-suite-cap.ci` was never written, and the spec records why at the Functional Tests decision.** The scenario would have to re-enter the functional runner from inside a running functional suite. The behaviour is proved by `TestCommandLineWrapsEverySuiteInTimeout` over the command line every path builds, and by `TestRecordReportsAKillAsAKill` over a real 124.
- Every one of the six named tests was retired with the shell it was written against. The behaviours are present under the names in the table above, each verified at closure.
- **A-3 could only be validated after CI had run the shards**, which it now has. The measurement is in the assumption's row.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The Phase cell said 4/5, so phase 5 looked unstarted | `.github/workflows/verify.yml` has run six shards on every push since, and `internal/le/workflowcheck` pins them | Reading the workflow and one real CI run rather than the cell | The cell is corrected, and the CI run is the evidence in A-3 |
| approach | A number derived in a comment was read as guarded because a test pinned it | The test pinned the BUDGET alone. The warn percentage is the other half of the same arithmetic and had its own test, so the pair could drift with both green | Asking what would still be green if the warn percentage halved | `TestThePluginBudgetKeepsItsWarningAboveTheMeasurement` relates the two |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The `plugin` suite stops running at its cap | Done | `budgetDefaults`, `(Suite).Budget` | 1500s against a measured 855s |
| A suite killed by its cap says so distinctly | Done | `expirySummary`, `failureGroupLine` (`internal/le/functional/run.go`) | it names the suite and its budget, and publishes a group |
| CI runs the stages across more than one job | Done | `.github/workflows/verify.yml`, six shards | measured at closure: 11670s of serial work in 4837s |
| The shards derive from the stage list, with no second list | Done | `internal/le/workflowcheck` | the workflow holds a count and a rule |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | the A-1 phase-1 measurement, 855s against a 1500s budget | the `context canceled` set did not survive the raised cap |
| AC-2 | Done | `TestRecordReportsAKillAsAKill`, `TestTheKillPublishesAFailureGroupWithARerun` | a 124 reports the budget, not N test failures |
| AC-3 | Done | `TestWarnPercentBoundaries`, `TestThePluginBudgetKeepsItsWarningAboveTheMeasurement` | the creep warning fires below the kill, and the two numbers are now related |
| AC-4 | Done | `TestVerifyShardsCoverEveryNativeStage` | a stage added to the population is picked up with no YAML edit |
| AC-5 | Done | the same test, plus `TestEachShardRunsOnePieceOfTheStaticcheckMatrix` | the union is the population, and the matrix pieces are dealt one per shard |
| AC-6 | Done | no suite was split, so `all_suites` is unchanged and no `.ci` moved tier | the split stays available and is named in R-2 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the four budget and report tests | PASS | `internal/le/functional/functional_test.go` | present under the names in the corrected table |
| `TestThePluginBudgetKeepsItsWarningAboveTheMeasurement` | PASS | the same file | written at closure |
| the two shard tests | PASS | `internal/le/workflowcheck/workflowcheck_test.go` | |
| `verify-scope-suite-cap.ci` | Not written | - | recorded in Deviations and in the spec's own Functional Tests decision |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| the suite budget and report | Done | split across `budget.go` and `run.go` by the le-personality port |
| `.github/workflows/verify.yml` | Done | six shards |
| the workflow's pin | Done | `internal/le/workflowcheck` |
| `docs/functional-tests.md` | Done | the budget table, the examples, and the derivation |

### Audit Summary
- **Total items:** 21
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the `.ci` that was replaced by a test, decided in phase 2 and recorded there)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The `plugin` suite stops producing false reds by running at its cap | measurement | Phase 1 ran the suite alone at a raised cap: 855s, 142% of the 600s cap, and the five `context canceled` failures did not recur. Under the shipped cap the suite was being killed with four minutes of work left |
| A cap expiry teaches what it is | unit | `TestRecordReportsAKillAsAKill` drives a 124 through `(*Run).Record` and reads back a report naming the suite and its budget; `TestTheKillPublishesAFailureGroupWithARerun` reads the declared group and its rerun |
| The budget cannot quietly stop meaning anything | unit | `TestThePluginBudgetKeepsItsWarningAboveTheMeasurement` fails if the budget is lowered toward the measurement or the warn percentage is cut, which are the two ways the derivation can be broken with every other test green |
| CI stops paying the whole hour on one runner | measurement | Run 33963534338: six shards at 4837s, 2662s, 2095s, 787s, 647s and 642s. 11670s of serial work completed in 4837s |
| No gate is dropped by sharding | unit | `TestVerifyShardsCoverEveryNativeStage` asserts the shards' union is the native stage population, so a stage can only leave CI by leaving that population |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Splitting the `plugin` suite's 663 `.ci` files | It is the structural answer to the budget, and it costs the `all_suites` and RFC-tier bookkeeping in R-2. Not needed to stop the false reds | none yet; named in the spec's phase-4 decision and in R-2 |
| Running the suites concurrently | Only the `bgp` and `vpp` paths reserve ports, so concurrent suites collide on deterministic ones | `plan/journal/parallel-copies-collide-on-a-deterministic-port.md` holds the prerequisite |
| Balancing the shards | The functional suite alone sets the floor, and CI measured it at 4837s against a 642s lightest shard. More shards buy nothing until that suite is split | the split above |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/verify-scope-4-suite-budget-and-ci-zeclose-vs4.md` |
| `review check` | clean |
| Rounds | 2. Round 1 found the unrelated derivation and the unvalidated A-3; round 2, over the test it produced and the CI measurement, was clean |
| Reviewer lenses used | derived-number guarding; test vacuity; assumption validation from real evidence; ze-go-style; spec record against the tree |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The plugin budget's derivation lives in a comment; the budget and the warn percentage are tested apart, so the pair can drift with both green | `budgetDefaults`, `DefaultWarnPercent` (`internal/le/functional/budget.go`) | `TestThePluginBudgetKeepsItsWarningAboveTheMeasurement` |
| 2 | ISSUE | A-3 was `unvalidated`, and the closure gate refuses that | this spec | confirmed from CI run 33963534338's six per-shard durations |
| 3 | NOTE | The Phase cell said 4/5 while phase 5 has producers in the tree | this spec | corrected, with the producers named |
| 4 | NOTE | Six TDD rows, three wiring rows and two Files sections name tests and files the tree no longer holds | this spec | every row rewritten against the tree as it stands |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/functional/budget.go` | Yes | holds `budgetDefaults`, `(Suite).Budget`, `WarnPercent`, `KillAfter`, `DurationSeconds` |
| `internal/le/functional/run.go` | Yes | holds `(*Run).Record`, `expirySummary`, `failureGroupLine` |
| `.github/workflows/verify.yml` | Yes | six shards, measured at closure |
| `internal/le/workflowcheck/workflowcheck_test.go` | Yes | holds both shard tests |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-3 | the plugin budget clears its measurement with margin | `TestThePluginBudgetKeepsItsWarningAboveTheMeasurement` PASS: 1500s x 80% = 1200s against 855s x 1.40 = 1197s |
| AC-2 | a kill reads as a kill | `go test ./internal/le/functional/` exit 0, holding both report tests |
| AC-4, AC-5 | the shards are the population | `go test ./internal/le/workflowcheck/` is red only on `TestEvidenceNightlyScheduleActionsAndPrivileges`, a foreign red journaled at closure; both shard tests pass |
| AC-6 | no `.ci` changed tier | no suite was split, so `all_suites` is untouched by this spec |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a suite that exceeds its cap | none, and the spec records why | `TestRecordReportsAKillAsAKill` drives a real 124 |
| `./le functional` | none | `TestCommandLineWrapsEverySuiteInTimeout` reads the command line every path builds |
| a push to GitHub | none | CI run 33963534338 ran the six shards; `TestVerifyShardsCoverEveryNativeStage` pins their union |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | 855s at a raised cap, and the `context canceled` set did not survive |
| A-2 | confirmed | the budget stays finite and stays a process-group kill; `TestCommandLineWrapsEverySuiteInTimeout` refuses a command line that drops `--kill-after` |
| A-3 | confirmed | CI run 33963534338: six shards, 11670s serial in 4837s wall clock, no contention because each shard is its own runner |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #10 test infrastructure | `docs/functional-tests.md`, "Per-suite wall-clock budget" and "One suite's budget is its own" | Yes, read both sections |
| #16 anchors on changed files | the budget section carries the variable table and the worked overrides; no anchor names a retired producer | Yes |
| #17 examples for this area | the same section shows `ZE_SUITE_TIMEOUT=1200s ./le functional gating` and `ZE_SUITE_TIMEOUT_PLUGIN=1800s ./le functional plugin` | Yes |
| every other row | No | Test tooling and CI: no YANG, CLI verb, RPC, plugin, wire format, or RFC tier |

## Core Insight

A number derived from a measurement needs a test that holds the DERIVATION, not the number. Pinning 1500s catches somebody lowering it and misses the cheaper break: halving the warn percentage leaves every existing test green and makes the suite warn on every run, at which point the budget still reads as derived and no longer is. The test to write is the arithmetic itself, with the measurement as a named constant beside it.
