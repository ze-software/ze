# Spec: a gate reports the population it judged

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A gate that examined nothing prints what a gate over a healthy tree prints.
Three producers in this repository do it today, and each was read at its
source on 2026-09-05.

The class was measured four times on one day. `plan/journal/green-that-could-not-have-been-red.md`
holds eleven mechanisms and `plan/journal/gate-excludes-part-of-its-population.md`
holds the population half. Two instances were fixed the same day:
`declaredSymbols` (`internal/le/repository/wiring.go`), whose population is
`git diff HEAD` plus untracked so a clean tree examined nothing, and
`internal/component/doctor`, which failed to typecheck for days so its tests
never built.

The instances differ in mechanism and share one property: a signal read as
"passed" that carries no information about the product. So the deliverable is
the RULE, and the three repairs are what proves it holds.

**The rule.** A gate's verdict NAMES the population it judged. A judged
population of zero is `examined nothing`, which is a third verdict beside pass
and fail. Each gate DECLARES whether an empty population is a legitimate
answer for it; an undeclared zero is a failure.

Ze already owns both halves of that rule and applies neither at the verdict
boundary. `internal/le/population` turns a claimed set into an accounting and
refuses an empty population outright. `staticcheckfeaturematrix.Notice`
carries `Judged` of `Total` and prints both. The verify stage report next door
carries an exit code and nothing else.

**The three instances.**

1. `runUnitRaceChanged` (`internal/le/verify/deps/verifydeps.go`) returns a
   zero code when `plan.Changed.Empty()`. Its population comes from
   `Selector.ChangedFiles` (`internal/le/changed/changed.go`), which asks
   `git diff --name-only`, `git diff --cached --name-only` and
   `git ls-files --others`. `./le verify worktree` runs the engine at a
   PRISTINE detached worktree: `runWorktree` (`internal/le/verify/lifecycle.go`)
   calls `git worktree add --detach` at the resolved sha and then
   `verifyengine.Run` at that path. All three queries answer empty there, so
   the only race-instrumented unit pass in the pre-commit gate examines
   nothing on EVERY run, and the stage answers 0.
   `TestRaceChangedEmptyPopulationRunsNoTests`
   (`internal/le/verify/deps/verifydeps_test.go`) asserts that exit code, so a
   test currently pins the defect.
2. `PhaseResult` (`internal/le/qemu/alltests_report.go`) carries `Code` and no
   count of tests executed. A phase that runs and executes zero tests answers
   0, and `AllTestsReport.Text` renders `ALL PHASES PASSED`. The 2026-09-05
   run was caught executing zero tests in 28 suites only because BusyBox
   `timeout` rejected `--kill-after` and exited non-zero. Commit `f26969c5c`
   removed both causes. It did not change the property: the phase verdict is
   still an exit code alone.
3. `publishChangeScope` (`internal/le/verify/engine/scope.go`) writes
   `scope-packages.txt` and names it in `ZE_VERIFY_SCOPE_PACKAGES`. The tags
   half has a consumer inside the run, `Derive`
   (`internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go`). The
   packages half has none: its only reader is the standalone
   `scopePackagesHere` action (`internal/le/changed/actions.go`), which no
   stage invokes. The run therefore publishes one change-set answer and the
   unit stage ignores it, re-deriving its own with the git queries of
   instance 1. An artifact nobody consults is a claim nobody checks.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the certificate, the detached-worktree run, and the published change scope
  → Decision: one selection per run is published to every stage, and a run that cannot select publishes nothing, because unset is the widest reading
  → Constraint: the page states that every scoped stage reads those files. AC-5 makes that true for the packages half, which today no stage reads
- [ ] `docs/architecture/testing/qemu-integration.md` - what the VM run proves
  → Constraint: the report exists to carry the facts a reader cannot recover from the streamed output. A test count is one of them
- [ ] `docs/contributing/testing.md` - a gate accounts for every member it does not walk
  → Decision: the accounting is a set difference, never a count floor, and every member the walk missed owes a stated reason that is itself rechecked

### RFC Summaries (Scope: protocol)
Not applicable. This spec changes no protocol behavior and no wire format.

**Key insights:** (minimal context to resume after compaction)
- `population.Claim.Assess` already refuses an empty population and reports `Population`, `Walked`, `Blind`, `Unexcused` and `Healed`. Seven callers use it; no verify stage does.
- The certificate already has the precedent for the propagation: `Skipped` is written by `WriteCertificate` and turned into `STALE` by `CheckCertificate` (`internal/le/verify/engine/status.go`).
- `staticcheckfeaturematrix` never judges zero rows: `minMatrixRows` is 2 and `Notice.Text` prints judged-of-total.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/verify/deps/verifydeps.go` - plans and runs each dependency stage; `runUnitRaceChanged` returns 0 on an empty change set; `Report` carries `Code`, `Children`, `Packages`, `Changed`
- [ ] `internal/le/changed/changed.go` - `Selector.ChangedFiles` asks git three questions about the WORKING TREE; `Selection.Empty` states what nothing-changed looks like
- [ ] `internal/le/verify/lifecycle.go` - adds a detached worktree at the resolved sha, then runs the engine at that pristine path
- [ ] `internal/le/verify/engine/run.go` - the stage loop reads `StageReport.Code` and nothing else; `StageReport` carries `Identity`, `Code`, `Log`, `Failure`
- [ ] `internal/le/verify/engine/status.go` - `Certificate`, `WriteCertificate`, `CheckCertificate`; `Skipped` is the existing route from "less was judged" to `STALE`
- [ ] `internal/le/verify/engine/scope.go` - `publishChangeScope` writes both answers and names both variables
- [ ] `internal/le/changed/scope.go` - `ScopeFileKey`, `ScopeTagsKey`, `Scope.Resolve`, `fromFile`, `widen`
- [ ] `internal/le/changed/actions.go` - `scopePackagesHere` is the only reader of the packages answer
- [ ] `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go` - `Notice` carrying `Judged`, `Total` and `Reached`, plus `minMatrixRows`: the shape this spec generalizes
- [ ] `internal/le/qemu/alltests_report.go` - `PhaseResult` and `AllTestsReport`; `Selection` and the no-phase guard are already there, the count is not
- [ ] `internal/le/qemu/alltests.go` - every `PhaseResult` is built from `allTestsRun.child`, an exit code
- [ ] `internal/le/population/population.go` - `Claim`, `Coverage`, `Assess`, `Exemptions`; the mechanism this spec reuses
- [ ] `internal/le/verify/deps/verifydeps_test.go` - `TestRaceChangedEmptyPopulationRunsNoTests` asserts the defect
- [ ] `internal/le/verify/engine/stages_structural_test.go` - the existing structural tests over the stage table

**Behavior to preserve:** (unless the user explicitly said to change it)
- The certificate's `exit=`, `timestamp=`, `mode=`, `skipped=`, `git_sha=`, `tree_hash=` line format and their order. A reader parses those keys.
- `Skipped` continuing to produce `STALE` in `CheckCertificate`.
- A legitimately empty change set staying a non-failure, so a clean tree does not turn the gate red.
- `AllTestsReport.Text` rendering `no phase ran` for a run that never started, and the `selection:` line for a filtered run.
- Every stage's exit code semantics for a real failure: a red stays red with the same code.

**Behavior to change:**
- A stage verdict gains the population it judged and the population it claimed.
- A judged population of zero renders `examined nothing`, never a pass line.
- `unit-race-changed` in a detached worktree judges the verified commit's own diff instead of a working tree that cannot differ.
- The published packages answer gains its in-run reader, which is that same stage.
- A QEMU phase carries the number of tests it executed, and zero is a phase failure.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le verify worktree [commit <revision>]` -- an agent or a person runs the pre-commit gate. `runHere` (`internal/le/verify/actions.go`) resolves the commit, adds a detached worktree, and calls `verifyengine.Run` at that path.
- `./le verify current mode full|changed` -- the same engine over the shared checkout.
- `./le qemu run command "./le qemu all-tests"` -- the in-VM run that produces `AllTestsReport`.
- Format at entry: a keyword-value argument list resolved by `leaction`; no file or wire input.

### Transformation Path
1. `verifyengine.runMode` publishes one change scope with `publishChangeScope` and iterates the stage table in `stages.go`.
2. Each stage runs through `verifydeps.run`, which plans the stage (population derivation) and then executes it.
3. `planUnitRaceChanged` derives the population with `changedSelection`, which runs the three git queries through the stage's own executor.
4. `runUnitRaceChanged` executes the planned commands, or returns early when the population is empty.
5. `validateResult` turns the action result into a `StageReport`; the loop reads that report's code.
6. `WriteCertificate` writes `tmp/ze-verify.status`; `CheckCertificate` later answers FRESH or STALE from it.
7. In the VM, `allTestsRun.Execute` builds one `PhaseResult` per suite from a child exit code and `AllTestsReport.Text` renders the verdict.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| verify engine ↔ verify deps stage | `verifyengine.ActionRunner` gives an `ActionResult`, turned into `StageReport` by `validateResult` | No |
| verify run ↔ every stage process | `ZE_VERIFY_SCOPE_PACKAGES` and `ZE_VERIFY_SCOPE_TAGS` name two files under the run's log directory | No |
| verify run ↔ a later reader | `tmp/ze-verify.status`, read by `CheckCertificate` and by `./le commit create` | No |
| host ↔ guest VM | ssh child processes; each phase's verdict is the child's exit code | No |
| gate ↔ population accounting | `population.Claim` and `population.Coverage` | No |

### Integration Points
- `internal/le/population` - the existing accounting type. The stage verdict reuses `Coverage`'s counts rather than declaring a second pair of fields.
- `changed.Scope.Resolve` - already consumes a published packages answer; the race stage becomes its in-run caller.
- `verifyengine.WriteRequest` and its `Skipped` field - the existing "less was judged" route into the certificate, which the new field sits beside.
- `suiteCoverage` (`internal/le/qemu/alltests.go`) - the phase-list population is already accounted for with `population.Claim`; the per-phase test count is what is missing.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The race stage stops re-deriving a change set the run already published, so one selection reaches every stage as `scope.go` states |
| No unintended coupling (components stay isolated) | Yes | Everything changed is under `internal/le/`; no component or plugin package is touched |
| No duplicated functionality (extends existing, does not recreate) | Yes | The counts reuse `internal/le/population`; the certificate field sits beside `skipped=` rather than starting a second artifact |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Build-host tooling, no wire encoding and no hot path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The empty-population declaration is a field on the stage's own entry in the existing stage table, read by a structural test over that table. No new central list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The three git queries in `Selector.ChangedFiles` all answer empty inside a `git worktree add --detach` checkout, so the race stage's population is empty on every `./le verify worktree` run | Read at `ChangedFiles` (`internal/le/changed/changed.go`) and at the worktree add in `internal/le/verify/lifecycle.go`; chain read, not executed | The stage is not permanently vacuous and AC-3 is unnecessary; AC-1 and AC-2 stand unchanged | `TestRaceChangedInADetachedWorktreeJudgesTheCommitsOwnDiff`, which fails against today's derivation | unvalidated |
| A-2 | A verified commit always has a parent to diff against on this repository | `git log` on main | The no-parent case must fall back to the whole tree | `TestRaceChangedFallsBackToTheWholeTreeForARootCommit` | unvalidated |
| A-3 | `ze-test` reports a count a phase can read from its own output or exit protocol | the suite runner in `internal/le/functional`; the 2026-09-05 QEMU log quoted counts such as `plugin 712/741` | The count must come from a machine-readable channel the runner adds, which enlarges AC-4 | `TestPhaseResultCarriesTheTestsItExecuted` | unvalidated |
| A-4 | No reader outside this repository parses `tmp/ze-verify.status` positionally, so a new key can be appended | `ReadCertificate` (`internal/le/verify/engine/status.go`) parses `key=value` into a map | The new key must go into a sidecar artifact instead | `TestCertificateNamesTheStagesThatExaminedNothing` plus a grep for readers of `ze-verify.status` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Making an empty population fail turns every clean-tree run red, and a red nobody can act on gets routed around | `./le verify current` red on an unmodified checkout | Zero is a failure ONLY where the stage did not declare empty legitimate. `unit-race-changed` declares it legitimate after AC-3, because a commit that touches no Go file is a real state |
| R-2 | AC-3 makes the pre-commit gate run a race pass it has never run, so long-hidden data races surface at once and the gate goes red for unrelated reasons | The first `./le verify worktree` after AC-3 lands is red in packages this spec never touched | That red is the product speaking and is reported, never suppressed (`ai/rules/pre-release.md`). Each race found gets a journal row and the work in hand closes |
| R-3 | The race pass over a large commit costs materially more wall clock than the zero it costs today | Stage duration in the run's log directory | Measure before and after and record both numbers. The population is one commit's diff, which is the same order as the working-tree diff the stage was designed for |
| R-4 | Six sessions share this checkout, so the certificate is written and read concurrently | Two runs disagreeing about the same field | `WriteCertificate` already writes atomically; the new field follows the same write |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The pre-commit gate. A wrong empty-population verdict either refuses every commit or certifies nothing, and `./le commit create` reads that certificate. No shipped binary behavior changes |
| How is it reverted? | Single commit revert. The artifacts are regenerated by the next run, and an old certificate simply lacks the new key |
| Who else touches this path? | Sessions hold `internal/le/qemu/**`, `internal/le/verify/**` and `internal/le/job/**` as of 2026-09-05. Implementation must re-read those trees before editing |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le verify worktree` at a commit | → | `planUnitRaceChanged` deriving the commit's own diff, `runUnitRaceChanged` reporting its judged count | `TestRaceChangedInADetachedWorktreeJudgesTheCommitsOwnDiff` |
| `./le verify current mode full` over a tree with no changed Go file | → | `runUnitRaceChanged` answering `examined nothing` | `TestRaceChangedEmptyPopulationAnswersExaminedNothing` |
| `./le verify status check` reading `tmp/ze-verify.status` | → | `WriteCertificate` and `CheckCertificate` carrying the examined-nothing stage names | `TestCertificateNamesTheStagesThatExaminedNothing` |
| `./le qemu all-tests` inside the VM | → | `PhaseResult` carrying the executed test count, `AllTestsReport.Text` refusing `ALL PHASES PASSED` | `TestAPhaseThatExecutedNoTestIsNotAPass` |
| A new stage added to `verifyengine` stages | → | the stage table's empty-population declaration | `TestEveryVerifyStageDeclaresWhatAnEmptyPopulationMeans` |
| An operator running `./le verify current mode changed` | → | the whole chain, from the command to the rendered verdict | `test/ui/le-verify-names-what-it-judged.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any verify stage finishes | Its structured report names the population it claimed and the number it judged, and the rendered stage line prints both. A stage whose judged count is zero prints `examined nothing` and never a pass line |
| AC-2 | A verify stage finishes with a judged count of zero and its stage-table entry does not declare an empty population legitimate | The run fails, names the stage, and says the stage examined nothing. A stage that DOES declare it legitimate answers `examined nothing` with exit 0 |
| AC-3 | `./le verify worktree commit <sha>` where `<sha>` changes at least one Go file | The race-instrumented unit stage judges the packages that commit changed against its parent, and its judged count is greater than zero. Where `<sha>` changes no Go file, the count is zero and the stage answers `examined nothing` |
| AC-4 | A QEMU phase runs to completion and executes zero tests | The phase is recorded as failed, is named in `Failed`, and the run does not render `ALL PHASES PASSED`. Every phase's payload carries the number of tests it executed |
| AC-5 | A verify run publishes `scope-packages.txt` | The race-instrumented unit stage reads that published answer instead of running its own git queries, so one selection sizes the run. A published answer with no in-run reader fails `TestEveryPublishedRunArtifactHasAReader` |
| AC-6 | A verify run in which any stage examined nothing writes its certificate | The certificate names those stages, and `./le verify status check` prints them in its answer rather than an unqualified PASS |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryVerifyStageVerdictNamesItsJudgedPopulation` | `internal/le/verify/engine/stages_structural_test.go` | AC-1 over the whole registered stage table, so a new stage cannot be added without an answer | |
| `TestEveryVerifyStageDeclaresWhatAnEmptyPopulationMeans` | `internal/le/verify/engine/stages_structural_test.go` | AC-2: each stage-table entry carries the declaration, and an undeclared entry fails the test | |
| `TestAnUndeclaredEmptyPopulationFailsTheRun` | `internal/le/verify/engine/run_test.go` | AC-2 at the run loop: a zero-judged stage without the declaration sets a non-zero run code and names the stage | |
| `TestRaceChangedEmptyPopulationAnswersExaminedNothing` | `internal/le/verify/deps/verifydeps_test.go` | AC-2, replacing `TestRaceChangedEmptyPopulationRunsNoTests`, which asserts the defect | |
| `TestRaceChangedInADetachedWorktreeJudgesTheCommitsOwnDiff` | `internal/le/verify/deps/verifydeps_test.go` | AC-3: a clean tree at a commit still yields the commit's changed packages | |
| `TestRaceChangedFallsBackToTheWholeTreeForARootCommit` | `internal/le/verify/deps/verifydeps_test.go` | AC-3 boundary: a commit with no parent | |
| `TestRaceChangedReadsTheRunsPublishedPackageAnswer` | `internal/le/verify/deps/verifydeps_test.go` | AC-5: the stage consumes `scope-packages.txt` rather than re-deriving | |
| `TestEveryPublishedRunArtifactHasAReader` | `internal/le/verify/engine/scope_test.go` | AC-5: an artifact the run publishes and no stage reads fails the test | |
| `TestCertificateNamesTheStagesThatExaminedNothing` | `internal/le/verify/engine/status_test.go` | AC-6 write side, and that `skipped=` is unchanged | |
| `TestFreshnessReportsWhatTheLastRunDidNotExamine` | `internal/le/verify/engine/status_test.go` | AC-6 read side: `CheckCertificate` surfaces the names | |
| `TestPhaseResultCarriesTheTestsItExecuted` | `internal/le/qemu/report_test.go` | AC-4 payload half | |
| `TestAPhaseThatExecutedNoTestIsNotAPass` | `internal/le/qemu/report_test.go` | AC-4 verdict half: zero tests is a failure and blocks `ALL PHASES PASSED` | |
| `TestAllPhasesPassedStillRendersWhenEveryPhaseRanTests` | `internal/le/qemu/report_test.go` | AC-4 does not break the healthy run | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| judged population count | 0..N | 0 with a declaration, 1 without | negative, unreachable by construction | N/A |
| tests executed in a QEMU phase | 0..N | 1 | 0 is the failing case AC-4 names | N/A |
| commit parents used to derive the race population | 0..1 | 1 | 0 falls back to the whole tree (A-2) | a merge commit uses its first parent |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-verify-names-what-it-judged` | `test/ui/le-verify-names-what-it-judged.ci` | An agent runs the verification gate and reads, from the command's own output, how much each stage judged and which stages judged nothing | |

### Interop Tests (Scope: protocol)
Not applicable. This spec changes build-host tooling only, and no wire-visible behavior. `ai/rules/interop-and-goal-validation.md` exempts tooling with no protocol peer.

## Files to Modify
- `internal/le/verify/deps/verifydeps.go` - the stage report carries its judged and claimed counts; `runUnitRaceChanged` answers `examined nothing`; `planUnitRaceChanged` derives the population from the verified commit and from the run's published answer
- `internal/le/verify/engine/run.go` - `StageReport` carries the counts; the loop fails an undeclared zero and collects the examined-nothing stage names
- `internal/le/verify/engine/stages.go` - each stage-table entry declares whether an empty population is legitimate
- `internal/le/verify/engine/status.go` - the certificate carries and re-reads the examined-nothing stage names
- `internal/le/verify/engine/scope.go` - the published packages answer gains its in-run reader and the artifact-to-reader check
- `internal/le/changed/changed.go` - `Selector` gains the commit-range derivation AC-3 needs
- `internal/le/qemu/alltests_report.go` - `PhaseResult` carries the executed test count; `AllTestsReport.Text` refuses `ALL PHASES PASSED` when a phase executed none
- `internal/le/qemu/alltests.go` - each phase records the count its child reported
- `internal/le/verify/deps/verifydeps_test.go` - `TestRaceChangedEmptyPopulationRunsNoTests` asserts the defect and is corrected
- `docs/architecture/testing/verify-freshness-scope.md` - the certificate's new key, the stage verdict's counts, and the packages answer's in-run reader
- `docs/architecture/testing/qemu-integration.md` - what a phase verdict now carries
- `docs/contributing/testing.md` - the rule, beside the population accounting it extends

## Files to Create
- `test/ui/le-verify-names-what-it-judged.ci` - functional coverage of the rendered verdict

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Build-host tooling; nothing an operator configures |
| YANG validation constraints | No | No YANG leaf added |
| YANG custom validators | No | No YANG leaf added |
| CLI commands/flags | No | No new verb or keyword; existing verdicts gain fields |
| CLI grammar (keyword before value) | N-A | No argument added |
| Editor autocomplete | No | No config leaf added |
| Functional test for new RPC/API | Yes | `test/ui/le-verify-names-what-it-judged.ci` |
| Pipe completeness | Yes | The counts are payload fields, so `\| json`, `\| yaml` and `\| table` render them from the same structure the prose rendering reads |
| Env var registration | No | `ZE_VERIFY_SCOPE_PACKAGES` and `ZE_VERIFY_SCOPE_TAGS` are already registered in `internal/le/changed/scope.go` |
| Doctor check for runtime dependencies | No | No new path, socket, service, module, port or binary |
| Prometheus counters/metrics | No | Build-host tooling exports no metric |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Nothing an operator of the shipped binary reaches |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | Yes | `./le verify` and `./le qemu all-tests` output changes; the pages in row 12 carry those, and `docs/guide/command-reference.md` documents the shipped `ze` CLI rather than the build-host tool |
| 4 | API/RPC added/changed? | No | No RPC touched |
| 5 | Plugin added/changed? | No | No plugin touched |
| 6 | Has a user guide page? | No | Build-host tooling |
| 7 | Wire format changed? | No | No wire encoding touched |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface touched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol behavior touched |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the new `.ci`, and `docs/contributing/testing.md` for the rule |
| 11 | Affects daemon comparison? | No | No shipped behavior changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md` and `docs/architecture/testing/qemu-integration.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata touched |
| 14 | Prometheus counters added/changed? | No | No metric added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration surface changes; the stage table gains a field on entries that already exist |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, re-run `./le spec citation anchors spec plan/pre-release/spec-a-gate-reports-the-population-it-judged.md` at implementation. The `// Design:` declarations of the files above are `docs/architecture/testing/verify-freshness-scope.md`, `docs/architecture/testing/qemu-integration.md` and `docs/contributing/testing.md`, all three named here |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/testing/verify-freshness-scope.md` shows the certificate's key list and states that every scoped stage reads the published files; both statements change |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- give the stage table its declaration and the verdict its counts, with nothing reading them yet
   - Tests: `TestEveryVerifyStageDeclaresWhatAnEmptyPopulationMeans`, `TestEveryVerifyStageVerdictNamesItsJudgedPopulation`
   - Files: `internal/le/verify/engine/stages.go`, `internal/le/verify/engine/run.go`, `internal/le/verify/deps/verifydeps.go`
   - Verify: both structural tests fail because the fields do not exist, then pass with every stage answering
2. **Phase: the rule at the run loop** -- an undeclared zero fails the run and a declared zero renders `examined nothing`
   - Tests: `TestAnUndeclaredEmptyPopulationFailsTheRun`, `TestRaceChangedEmptyPopulationAnswersExaminedNothing`
   - Files: `internal/le/verify/engine/run.go`, `internal/le/verify/deps/verifydeps.go`, `internal/le/verify/deps/verifydeps_test.go`
   - Verify: correcting `TestRaceChangedEmptyPopulationRunsNoTests` is part of this phase, not a later cleanup
3. **Phase: the race stage judges something** -- derive the population from the verified commit and from the run's published answer
   - Tests: `TestRaceChangedInADetachedWorktreeJudgesTheCommitsOwnDiff`, `TestRaceChangedFallsBackToTheWholeTreeForARootCommit`, `TestRaceChangedReadsTheRunsPublishedPackageAnswer`, `TestEveryPublishedRunArtifactHasAReader`
   - Files: `internal/le/changed/changed.go`, `internal/le/verify/deps/verifydeps.go`, `internal/le/verify/engine/scope.go`
   - Verify: run `./le verify worktree` at a commit that touches a Go file and read the stage's judged count in the log. Record the wall clock against R-3
4. **Phase: the certificate carries it** -- write and re-read the examined-nothing stage names
   - Tests: `TestCertificateNamesTheStagesThatExaminedNothing`, `TestFreshnessReportsWhatTheLastRunDidNotExamine`
   - Files: `internal/le/verify/engine/status.go`, `internal/le/verify/engine/run.go`
   - Verify: `./le verify status check` prints the names; `skipped=` behavior is unchanged
5. **Phase: the QEMU phase verdict** -- a phase carries its executed test count and zero is a failure
   - Tests: `TestPhaseResultCarriesTheTestsItExecuted`, `TestAPhaseThatExecutedNoTestIsNotAPass`, `TestAllPhasesPassedStillRendersWhenEveryPhaseRanTests`
   - Files: `internal/le/qemu/alltests_report.go`, `internal/le/qemu/alltests.go`
   - Verify: replay the 2026-09-05 failure shape, a phase whose child exits 0 having run nothing, and confirm the run refuses to print `ALL PHASES PASSED`
6. **Phase: pages and functional coverage** -- the three pages and the `.ci`
   - Tests: `le-verify-names-what-it-judged`
   - Files: `test/ui/le-verify-names-what-it-judged.ci`, the three documentation pages
   - Verify: `./le docvalid` clean, and the `.ci` fails when the counts are removed from the rendering

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named symbol, and AC-1 is enforced over the whole stage table rather than the stages this spec edited |
| Feature completeness | The chain from `./le verify worktree` to the certificate to `./le verify status check` carries the counts end to end, with no link that drops them |
| Correctness | A declared-legitimate zero stays exit 0; an undeclared zero is a failure; a real red keeps its own code and is not overwritten by the population verdict |
| Naming | JSON keys kebab-case (`judged`, `population`, `examined-nothing`); the certificate key matches the field name; the prose says `examined nothing` in every rendering |
| Data flow | One change-set selection sizes the run. After AC-5 the race stage has no git query of its own |
| Rule: `ai/rules/principles.md` | No zero is reachable as a valid-looking answer. The declaration is named as the guard it is, and a legitimate zero is distinguishable from a failed derivation |
| Rule: `ai/rules/simplicity.md` | The counts reuse `internal/le/population`; no second accounting type is introduced, and the certificate gains one key rather than a second artifact |
| Rule: `ai/rules/no-layering.md` | The race stage's own git derivation is DELETED when the published answer replaces it. No fallback keeps both |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Every stage-table entry declares its empty-population meaning | `go test ./internal/le/verify/engine/ -run TestEveryVerifyStageDeclaresWhatAnEmptyPopulationMeans` |
| The race stage judges a non-empty population at a commit that changed Go files | `./le verify worktree commit <sha> keep`, then read the stage log's judged count |
| The certificate carries the examined-nothing names | `cat tmp/ze-verify.status` after a run in which one stage judged nothing |
| A zero-test QEMU phase is a failure | `go test ./internal/le/qemu/ -run TestAPhaseThatExecutedNoTest` |
| No published run artifact lacks a reader | `go test ./internal/le/verify/engine/ -run TestEveryPublishedRunArtifactHasAReader` |
| The three pages agree with the code | `./le docvalid` and `./le spec citation anchors spec plan/pre-release/spec-a-gate-reports-the-population-it-judged.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The commit revision reaching the new derivation is already resolved by `resolveCommit`; the derivation must take the resolved sha and never an operator string |
| Authorization that could fail open | The certificate is what authorizes a push. A parse failure of the new key must widen to "examined nothing is unknown" and refuse freshness, never to an unqualified PASS |
| Resource exhaustion | The race pass over a large commit is bounded by that commit's diff, and R-3 records the measured cost |
| Error leakage | Stage names and package paths only; no credential reaches the certificate |

## Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| Phase 3 surfaces a data race in an unrelated package | Journal row per `ai/rules/completion.md`, report the red, do not suppress the race pass to make it green |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The repository already wrote this rule twice and applied it in two places out of many. `internal/le/population` is the set-difference half and `staticcheckfeaturematrix.Notice` is the count half. What was missing is the boundary they were never applied at: the verdict a stage hands back.
- A count floor is not the rule. The comment on `population.Assess` says so, and this spec agrees: the count is what a READER needs, and the set difference is what a gate with an enumerable population needs. The two answer different questions and both are owed.
- The pre-commit gate's detached worktree is what makes instance 1 permanent rather than occasional. A population derived from the working tree is empty by construction in a checkout that has no working-tree change, so the same derivation that is right in the shared checkout is vacuous in the worktree. That is the general shape to look for: a population whose derivation is correct in one place and empty by construction in another.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The verdict names the population it judged, and zero is a third verdict | The reporting type stops being an exit code alone | That names the mechanism and not the rule. It says the type must be richer without saying what it must carry, so it cannot be enforced by a test |
| Each gate declares whether an empty population is legitimate | Every empty population fails | A clean tree legitimately changes nothing. Failing it makes the gate red where nothing is wrong, and a red nobody can act on gets routed around, which is `plan/journal/gate-red-where-nothing-blocks-on-it.md` |
| The declaration lives on the stage's own table entry | A central list of stages whose zero is allowed | A second declaration of what already exists. `ai/rules/principles.md` requires the feature's own registration to carry it |
| `population.Claim` is kept where the population is a fixed enumerable set, not made the general rule | Declare every gate's population up front and refuse on disagreement | It cannot express a population derived per run. The change set legitimately varies and legitimately reaches zero, so a declaration there is either permanently red or a rubber stamp |
| The certificate gains one key beside `skipped=` | A separate examined-nothing artifact | `Skipped` is the proven route from "less was judged" to a non-fresh certificate. A second artifact is a second thing to read and to go stale |
| The race stage reads the run's published packages answer and its own git derivation is deleted | Keep the git derivation as a fallback when the published answer is absent | `ai/rules/no-layering.md`. A fallback keeps the vacuous derivation alive in exactly the run where it is vacuous |

## Known Limitations

- `captureStdout` (`internal/component/config/schema/cli/main_test.go`) is out of scope. It calls `fn()` synchronously and reads the pipe afterwards, so output past the 64 KB pipe buffer deadlocks, and `TestPrintedSchemaFormsStillRender/show` hung 20 minutes with 80,964 bytes pending. It is a HANG, not a signal that reads as passed: it produces no verdict at all and is loudly visible as a timeout. Its mechanism is a pipe reader ordering, which nothing in this spec's rule reaches, and its record is the 2026-09-03 row in `plan/journal/net-pipe-deadlock.md`. The fix is a reader goroutine started before `fn()`, which is the same repair the two 2026-03-21 rows in that file already carry.
- The rule is enforced over the verify stage table and the QEMU phase list. A gate outside those two, and there are many under `internal/le/`, is not swept by this spec. Sweeping them is separable work whose home is the journal pass over `plan/journal/gate-excludes-part-of-its-population.md`.
- `./le verify worktree` defaults to HEAD, so the pre-commit gate verifies the commit before the pending work. Whether that is the right commit to verify is a separate question this spec does not answer; AC-3 makes the race stage judge whatever commit the gate was pointed at.

## RFC Documentation (Scope: protocol)

Not applicable. This spec implements no protocol behavior.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
