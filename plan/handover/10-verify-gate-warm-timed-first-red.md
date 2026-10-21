# Handover: verify-gate-warm-timed-first-red (implementation, part 1 of 2)

Spec: `plan/spec-verify-gate-warm-timed-first-red.md` (in-progress, claimed by
session 7bb0320c). Written 2026-10-21 by the implementation agent at its
tool-call budget. The next implementation agent carries the SAME package and
finishes it; review is a separate phase after that.

## Landed (commits on main)

| Commit | Subject | ACs |
|--------|---------|-----|
| ce93be1bba | verify: publish the worktree failure index to the checkout | AC-8 |
| 1b8693f797 | le: build every Go command with -trimpath; share the lint cache | AC-1..AC-5 |
| 77253d7cc8 | verify: record each stage's start and duration | AC-6; AC-7 report and index half only |

Files and symbols:
- `internal/le/verify/engine/artifacts.go`: `PublishFailureIndex(source, target, logs)`, `movedLogPath`; `artifactStage` gained `start`, `seconds`.
- `internal/le/verify/lifecycle.go`: index publication after the log save (`relativeLogs`), `sharedLintCacheLink` beside the two existing links.
- `internal/le/go/toolchain/gotoolchain.go`: `Environment` appends exactly one GOFLAGS entry from `TrimpathGoFlags`; `EffectiveGoFlags` is the one resolver (moved from `job.goTestDefaults`, deleted); `withoutKey`.
- `internal/le/go/staticcheck/judge.go`: `judgeEnvironment(environ, cache, dir) ([]string, error)` appends `gotoolchain.TrimpathGoFlags`; private composition deleted. `internal/le/rfc/check_compile.go`: argv `-trimpath` dropped.
- Four tests find the root with `os.Getwd` (accept_only_lint, workflowcheck, local_data_functional_coverage, allocgate).
- `internal/le/verify/engine/run.go`: `StageReport.Start`, `.Seconds`; `### Stage timing:` line before the unchanged result line.
- Docs edited: `docs/architecture/testing/verify-freshness-scope.md`, `docs/contributing/running-commands.md`, `docs/architecture/testing/tracked-build-gate.md`.

## Evidence

- Red before fix: `TestAWorktreeRunPublishesItsFailureIndex` (no index at checkout); `TestEnvironmentComposesTrimpathWithTheEffectiveGOFLAGS`; the four root tests under `GOFLAGS=-trimpath` (TestCIAcceptOnlyLint, TestEveryWorkflowNativeActionExists and 15 more red); `TestEveryStageLogCarriesItsTimingBeforeItsResult` (compile red).
- Green after: `./le job run label unit-pkg quiet command go test -race -tags "$tags"` over `./internal/le/verify/...`, `./internal/le/commit/`, `./internal/le/go/toolchain/`, `./internal/le/go/staticcheck/`, `./internal/le/rfc/`, `./internal/le/job/`, and under `GOFLAGS=-trimpath` `./internal/component/command/registry/`, `./internal/perf/`, `./internal/test/runner/` (root tests green).
- Known foreign reds: `TestCIAcceptOnlyLint` is red at HEAD for `test/parse/iface-unnumbered-loopback.ci` (3c2c96ec01, another session). `internal/le/workflowcheck` could not build: untracked `internal/le/test/qemu/vmrecord.go` (another session) does not compile, so its repaired root helper is unproven under -trimpath.
- `TestAWorktreeSharesTheCheckoutLintCache` was written with its implementation, not run red first: the next agent should break `sharedLintCacheLink` once and record the red.

## Still to do (same package)

1. AC-7 rest: `internal/le/verify/engine/stagetimes.go` (append one TSV line per reached stage, single O_APPEND write: stage, seconds, exit, mode, commit, start; latest-per-stage reader that skips and counts malformed lines, negative/NaN/Inf seconds), written to `<checkout>/tmp/ze-verify-stage-times.tsv` by the lifecycle (worktree) and by the in-place run. Tests `TestStageTimesReachTheCheckoutOnGreenAndRed`, `TestAMalformedHistoryLineIsSkippedAndCounted`.
2. Phase 1 wiring: `first-red` keyword on `verify worktree` and `verify current` (`actions.go`, `current.go`), into `Options` and `worktreeArgv` (AC-13), refused with `part` (AC-14, exit 2, `test/runner/verify-first-red-refuses-a-cut.ci`), `TestFirstRedReachesTheEngineFromBothEntryPoints`, `TestFirstRedIsSeparateWorkFromAWholeRun`.
3. AC-9 cost order in `runMode` for an uncut run (unrecorded first in declared order, then ascending latest seconds, ties declared); `verify list` unchanged; cut runs dealt order.
4. AC-10 stop at first red, unreached stages `reached: false` in report and index, `stopped-at`; lifecycle line `stopped at <stage>, <n> stages not run` plus the command that runs them all.
5. AC-11 `structuralGateReds` charges unreached structural stages; AC-12 stale certificate test; AC-15 `verify reds` names a stopped run; `test/runner/verify-first-red-stops.ci` with fixture.
6. `docs/contributing/running-commands.md`: the `first-red` keyword and the history file.

## Spec corrections made or owed

- `TestOverridesComposeTrimpathWithTheEffectiveGOFLAGS` is `TestEnvironmentComposesTrimpathWithTheEffectiveGOFLAGS`: `Overrides` does not see the inherited environment, so the composition lives in `Environment`.
- `TestStaticcheckEnvironmentCarriesTrimpathFromTheToolchain` is the existing `TestJudgeRunsWithPathIndependentCompiles`, extended (a rename was refused as a test weakening).
- `TestGoTestDefaultsUseTheToolchainResolver`: covered by the existing GOENV cases in `internal/le/job/process_test.go` (lines near 281) now running through `gotoolchain.EffectiveGoFlags`; no new test.
- The spec's TDD table and Wiring table still carry the old names: update them.

## Part 2 (continuation agent, 2026-10-21)

Landed: 88fb40e846 "verify: cost-ordered runs, first-red stop, stage-time history" (AC-7 rest, AC-9..AC-15, docs).

- `engine/stagetimes.go`: `Plan{History, FirstRed}`, `StageTimesPath`, `ReadStageCosts` (latest per stage, malformed counted), `costOrder`, `AppendStageTimes` (one O_APPEND write per run). The engine itself reads and appends at `Plan.History` (the checkout root), passed by `lifecycle.run` and `runFast`; there is no separate publication step.
- `engine/run.go`: `RunMode`/`RunPart` take `plan`; `planOrder`, `recordStageTimes`, `StoppedMarker`; `Report.StoppedAt`, `Report.Unreached`. `artifacts.go`: `reached`, `stopped-at`, unreached entries.
- `lifecycle.go`: `Options.FirstRed`, refusal in `Options.part` (`firstRedWithPart`), `worktreeArgv(sha, part, firstRed)`, stop line. `actions.go`: keywords `first-red`, `part`, `of` (part/of did NOT exist on `verify worktree` before; the spec's Current Behavior was wrong). `current.go`: `first-red` on `verify current`, `fastArgv(firstRed)`.
- `commit/verification.go`: `structuralReds.Unreached`, charged; `prepare.go` names them. `reds.go`: `Reds.StoppedAt`, `stoppedAt`.
- Tests: engine `TestAnUncutRunRunsCheapestRecordedStageFirst`, `TestFirstRedStopsAndRecordsEveryUnreachedStage`, `TestARunWithoutFirstRedStillRunsPastARed` (run_test.go), `TestAMalformedHistoryLineIsSkippedAndCounted`, `TestEveryReachedStageIsAppendedToTheHistory` (stagetimes_test.go), `TestACutRunKeepsTheDealtOrder` (part_test.go), `TestAStoppedRunIsStaleForEveryPathList` (status_test.go); verify `firstred_test.go`: `TestFirstRedReachesTheEngineFromBothEntryPoints`, `TestFirstRedIsSeparateWorkFromAWholeRun`, `TestFirstRedWithAPartIsRefused`, `TestStageTimesReachTheCheckoutOnGreenAndRed`, `TestVerifyListKeepsTheDeclaredOrder`, `TestRedsNamesAStoppedRun`; commit `TestAnUnreachedStructuralStageIsCharged`. `test/runner/verify-first-red-refuses-a-cut.ci` with fixture `misc_fixture_runner_first_red.go` is NOT run yet (needs a rebuilt le).
- Green: `go test -race` under `./le job run` over `./internal/le/commit/ ./internal/le/verify/... ./internal/test/fixture/`.
- Recorded red: `TestAWorktreeSharesTheCheckoutLintCache` with the `os.Symlink` call in `sharedLintCacheLink` removed failed at its `os.Readlink` assertion (no such file or directory); restored, green.
- The new tests were written after the code (they could not compile before it); no red phase was recorded for them.

Still owed (same package):
1. `test/runner/verify-first-red-stops.ci` and its fixture (throwaway checkout, `le verify current fast first-red` stops, the log names unreached stages, `le verify status check` answers STALE). Not written.
2. Run `verify-first-red-refuses-a-cut.ci` (`./le test functional runner`).
3. Spec edits (the plan/ write hook needs the Edit tool and the budget ran out): TDD, Wiring and Deliverables tables to the names above and to Part 1's "Spec corrections"; Current Behavior's `actions.go` line (part/of added, refusal in `Options.part`); Decision: the engine owns the history through `Plan.History`. `TestStageResultStillParsesALogWithATimingLine` was not found by name; check `reds_test.go`.

## Owed to the main thread

Review (independent), `./le go lint run`, `./le test unit all`, `./le test functional gating`, `./le doc check verify`, the full `./le verify worktree`, and the AC-16 two-run measurement.
