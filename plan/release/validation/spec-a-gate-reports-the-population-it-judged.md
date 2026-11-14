# Spec: a gate reports the population it judged

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P0 |
| Release reason | Fail-open release boundary: GitHub CI is the release check, and its race pass answers green after racing only the packages the last push touched, so a data race anywhere else cannot turn it red. The scoped runners agents use as day-to-day evidence also answer success for a run that executed no test |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A run that examined nothing prints what a run over a healthy tree prints. This
spec was written on 2026-09-05 against three instances in the verify engine and
the QEMU run. Those are delivered (table below). On 2026-11-14 it was narrowed in
place to the residue: the scoped runners that carry an agent's evidence today
(`ai/META-DECISIONS.md`, "Ze is pre-release": the named tests, the packages, the
functional tests) and the per-stage output of GitHub CI, which is the release
check.

**The rule.** A verdict NAMES the population it judged. A judged population of
zero is `examined nothing`, a third verdict beside pass and fail. Each runner
DECLARES where an empty population is a legitimate answer; an undeclared zero is
a failure. The declaration lives on the runner's own row (the suite row, the
unit group row, the stage), never in a central list.

**What is unmet today, read at the producer on 2026-11-14.**

1. The `.ci` runner answers success for a run that executed nothing.
   `runCISubcommandInner` (`internal/test/cli/ci_runner.go`) and the `le test bgp`
   run path (`internal/test/cli/cmd_bgp.go`) print usage and return no error when
   the selection holds no test, so the process exits 0. `Runner.Run`
   (`internal/test/runner/runner.go`) logs `no tests selected` and returns true.
   `runTest` (`internal/test/runner/runner_exec.go`) returns true for a test with
   a skip reason, so a run whose every test skipped is a pass.
2. `./le test functional <suite>` and the gating run read only that exit code.
   `runPrepared` (`internal/le/test/functional/run.go`) hands `Run.Record` the
   suite's exit status and nothing else, so cause 1 reaches the gating verdict.
3. `./le test unit <group>` reads only an exit code. The group action calls
   `gaterun.Run` (`internal/le/gaterun/gaterun.go`), whose `ActionReport` carries
   the action, the command and the code. `go test` exits 0 over a pattern whose
   packages hold no test, and over a package whose tests all skip.
4. The QEMU run still passes an all-skipped suite. `allTestsRun.suite`
   (`internal/le/test/qemu/alltests.go`) fails a suite that printed no summary
   line, but a suite that printed `pass 0/0` (every test skipped) has
   `suiteTally.Seen` true and keeps its exit code 0. The go test phases (unit,
   installer, integration) leave `PhaseResult.Counted` false and are judged by
   exit code alone.
5. **CI's race pass judges only the push.** `.github/workflows/verify.yml` runs
   each stage of `./le verify list full` as its own `./le <action>` process, and
   never enters `runMode` (`internal/le/verify/engine/run.go`), so
   `publishChangeScope` never runs and `ZE_VERIFY_SCOPE_PACKAGES` is unset. With
   no published scope, `Selector.Select` (`internal/le/repo/changed/changed.go`)
   takes no `./...` widening and selects the Go files between `ResolveAnchor`'s
   anchor and HEAD. The workflow's step "Anchor the change set at the push or pull
   request base" sets that anchor to the push's `before` commit. So on a push to
   `main`, the three `verify deps unit-race-changed part <i> of 3` stages race the
   groups and packages of the Go files that push changed, plus the bare `ze_core`
   check on `./cmd/ze/hub`, and a push that changes no `.go` file races nothing
   and exits 0. A local full run races the whole tree, because `scopeFor` publishes
   `repochanged.WholeTree` for the full population and `Select` honors it. No
   other workflow races the tree: the QEMU unit phase runs without `-race`
   (`unitPhaseName`). The `raceParts` comment (`internal/le/verify/engine/stages.go`)
   still states that every CI clone races the whole tree; that was true on
   2026-10-17 (`5699693e1d`) and stopped being true when `1eb9e09b76` (2026-10-24)
   anchored the change set on git.
6. **The race pieces claim `Whole`.** `raceStages` declares them `Whole`
   because "a piece selects its groups from the anchor through the refs, not from
   the published scope", but `Select` reads the published scope and races `./...`
   when it says so. In a changed or fast run the pieces race the change set only,
   yet `JudgesInPart` treats them as a full judgement, so a partial green can
   replace a full run's race red in the failure index. `TestAWholeStageReachesNoChangeScopeReader`
   (`internal/le/changescope_test.go`) misses it because its scan is
   package-granular and does not follow the call into `internal/le/repo/changed`.

**Delivered since this spec was written.** History only: each row's fix is at
its commit, and this spec owes it nothing.

| Original instance | Fix | Commit |
|-------------------|-----|--------|
| 1. `unit-race-changed` examined nothing in a detached worktree, because its population was the working tree | `Selector.Select` adds the Go files committed since `ResolveAnchor`'s anchor; an empty selection is `Report.Skipped` with a `Reason`, and an unresolved directory is a refusal | `1eb9e09b76`, `1122dcc7b0` |
| 2. A QEMU phase verdict was an exit code alone, so `ALL PHASES PASSED` rendered over phases that ran nothing | `PhaseResult` carries `Tests` and `Counted` from `suiteTally`; a suite with no summary line fails; `Planned` and `Unreached` name the population | `9000fbb335` |
| 3. The published packages answer had no in-run reader | `Scope.Resolve` reads it through `fromFile` for `repochanged.Packages` (the functional suite selection, `arch enumeration check`, the changed unit pass, `go lint run changed`), and `Selector.Select` reads its `./...` widening | `1eb9e09b76` and earlier |

The 2026-09-05 certificate work is removed, not delivered: the certificate
(`tmp/ze-verify.status`) gates nothing. `verificationState`
(`internal/le/commit/verification.go`) only reports it, `deriveDebt`
(`internal/le/commit/owed.go`) reads no verify record, and CI writes none.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the change-set selection, the published scope, the race pieces and `Whole`
  → Decision: the full population is the release check and publishes `./...` and every feature tag, because a change set anchored on the upstream holds almost nothing over a pushed tree ("What a run publishes depends on its population")
  → Constraint: unset scope is the widest reading for every stage except the race pass, which then selects its own change set; that exception is cause 5
  → Constraint: `JudgesInPart` fails closed, and only a stage whose action judges its whole subject whatever scope was published may declare `Whole`
- [ ] `docs/architecture/testing/qemu-integration.md` - "A run says what it planned, what it reached, and how many tests ran"
  → Constraint: a suite that prints no summary line executed nothing and is a failure; the summary producer is `Display.Summary`
- [ ] `docs/architecture/testing/runner-architecture.md` - the `.ci` runner a suite runs on
  → Constraint: every suite prints its summary through one producer, so one reader of that line serves every consumer
- [ ] `docs/contributing/testing.md` - a gate accounts for every member it does not walk
  → Decision: the accounting is a set difference, never a count floor, and every member the walk missed owes a stated reason
- [ ] `docs/contributing/running-commands.md` - `./le test unit pkg` and its outcomes
  → Constraint: `ai/rules/commands.md` already tells the reader not to trust a runner's exit code because a runner that does not recognize its selector prints usage and exits 0; the output is what gets fixed (`ai/rules/writing.md`)

### RFC Summaries (Scope: protocol)
Not applicable. This spec changes no protocol behavior and no wire format.

**Key insights:** (minimal context to resume after compaction)
- `PkgReport.read` (`internal/le/test/unit/pkg.go`) already refuses exit 0 with no passed package as `no test ran`, exit 1. It is the shape the group action and the QEMU go test phases extend.
- `suiteTally` (`internal/le/test/qemu/alltests_tally.go`) already reads executed counts from the runner's summary line; the functional run needs the same reader, so it moves to the producer's side rather than being copied.
- CI bypasses `runMode`, so anything `runMode` does (scope publication, stage timing, the failure index) does not happen in CI.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/test/runner/runner.go` - `Runner.Run` and `runSelected`: an empty selection logs `no tests selected` and returns true
- [ ] `internal/test/runner/runner_exec.go` - `runTest` sets `StateSkip` and returns true for a record with a `SkipReason`
- [ ] `internal/test/runner/display.go` - `Display.Summary` prints nothing when no test executed and none skipped
- [ ] `internal/test/cli/ci_runner.go` - `RunCISubcommand` maps a nil error to 0; `runCISubcommandInner` returns nil after printing usage when `selected == 0`
- [ ] `internal/test/cli/cmd_bgp.go` - the `le test bgp` run path prints usage and returns nil when `selected == 0`
- [ ] `internal/le/test/functional/run.go` - `Execute` answers the child's code; `runPrepared` passes it to `Run.Record`; the run fails only on `FailedNames`
- [ ] `internal/le/test/functional/report.go` - `GatingReport` and its four verdict lists
- [ ] `internal/le/test/functional/suites.go` - `Suite` (`Name`, `Args`, `Summary`, `Why`, `Scaled`, `Warm`): the row a declaration would sit on
- [ ] `internal/le/test/unit/actions.go` - `runner` calls `gaterun.Run` with the group's argv
- [ ] `internal/le/test/unit/groups.go` - `Group`; `Argv` runs `go vet` instead of `go test` for a cross-targeted group
- [ ] `internal/le/test/unit/pkg.go` - `PkgReport`, `read`, `summary`, the outcome vocabulary
- [ ] `internal/le/gaterun/gaterun.go` - `Run` and `ActionReport`: an exit code and the argv
- [ ] `internal/le/test/qemu/alltests.go` - `allTestsRun.suite`: no summary line fails, `Seen` with zero tests keeps the child's code
- [ ] `internal/le/test/qemu/alltests_tally.go` - `suiteTally`: `Seen`, `Tests`, `Skipped`
- [ ] `internal/le/test/qemu/alltests_report.go` - `PhaseResult` `Tests` and `Counted`; `AllTestsReport.Text`
- [ ] `internal/le/verify/deps/verifydeps.go` - `Report` with `Skipped` and `Reason`; `runUnitRaceChanged`, `skipUnitRaceChanged` (its reason still says "green baseline"), `planUnitRaceChanged`, `changedSelection`
- [ ] `internal/le/repo/changed/changed.go` - `Selector.Select`: the published `./...` widening, then `ResolveAnchor`, then the committed `.go` diff
- [ ] `internal/le/repo/changed/anchor.go` - `ResolveAnchor`: `@{u}`, then `refs/remotes/origin/HEAD`, then a refusal
- [ ] `internal/le/repo/changed/scope.go` - `newScope`, `Packages`, `Scope.Resolve`, `fromFile`
- [ ] `internal/le/verify/engine/run.go` - `runMode` publishes the change scope once per run
- [ ] `internal/le/verify/engine/scope.go` - `publishChangeScope`, `selectChangeSet`, `scopeFor` (full publishes `WholeTree`)
- [ ] `internal/le/verify/engine/stages.go` - `raceParts`, `Stage.Whole`, `JudgesInPart`, `raceStages` (declared `Whole`)
- [ ] `internal/le/population/population.go` - `Claim`, `Claim.Assess` (refuses an empty population), `Exemptions`
- [ ] `.github/workflows/verify.yml` - the anchor step and the stage loop over `./le verify list full`
- [ ] `internal/le/commit/verification.go` - `verificationState` reports the certificate and refuses nothing

**Behavior to preserve:** (unless the user explicitly said to change it)
- A real red keeps its own non-zero code everywhere; the population verdict never overwrites it.
- `-h`, `help` and the list forms of a `.ci` runner still print and exit 0: they ask for no verdict.
- `PkgReport`'s outcome names and JSON keys, which `./le test unit pkg | json` callers read.
- `skipEmptyRacePiece`: a piece the hash deal left empty records `skipped` with its reason and exits 0, because piece 1 always runs the core check.
- A local `./le verify deps unit-race-changed` with no published scope still races the change set since the anchor: a person asking about their own change wants that answer.
- The CI shard rule (round robin over `./le verify list full`) and `TestEachShardRunsOnePieceOfTheStaticcheckMatrix`.

**Behavior to change:**
- A `.ci` run that selected nothing, or whose every selected test skipped, answers `examined nothing` and a non-zero code.
- The functional runs, the unit groups and the QEMU go test phases carry what they executed and fail an undeclared zero.
- CI's race pass races the population a local full run races.
- Every race piece names the population it raced and where that population came from, and no longer claims `Whole`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Population accounting | `internal/le/population/population.go`, `Claim`, `Claim.Assess`, `Coverage` | Turns a claimed fixed set into `Population`, `Walked`, `Blind`, `Unexcused`, `Healed`, and refuses an empty population | Does not apply as the verdict type: a test selection varies per run and may legitimately reach zero, which `Assess` refuses by construction. It stays the tool for fixed enumerable sets (`suiteCoverage` already uses it for the QEMU phase list) |
| Stage skip report | `internal/le/verify/deps/verifydeps.go`, `Report.Skipped`, `Report.Reason`, `Report.Text` | A verify deps stage that executed no test says so and why, and renders `executed no test, <reason>` | Extends: the race pieces' reason names the population source and the anchor commit instead of the stale "green baseline", and the executed line names how many packages it raced |
| QEMU phase count | `internal/le/test/qemu/alltests_report.go`, `PhaseResult.Tests`, `PhaseResult.Counted`; `alltests_tally.go`, `suiteTally` | A functional suite phase carries its executed count from the runner's summary line; no line is a failure | Extends: an all-skipped suite is `examined nothing`, and the go test phases become counted from their package result lines. `suiteTally` moves beside its producer `Display.Summary` so the functional run reads the same line with the same code |
| Unit package outcomes | `internal/le/test/unit/pkg.go`, `PkgReport`, `read`, `summary`, `outcomeNoTestRan` | `./le test unit pkg` refuses exit 0 with no passed package as `no test ran`, exit 1, and treats vet as its own outcome | Extends: the group action answers through the same report and outcome vocabulary, and the QEMU go test phases read their counts with the same `runlog.PackageResults` |
| Package result lines | `internal/le/runlog/runlog.go`, `PackageResults`, `PackageCounts` | Counts `ok`, `no test files` and `FAIL` package lines of a `go test` log | Extends: becomes the one counter for every go test runner this spec touches |
| Stage whole or partial declaration | `internal/le/verify/engine/stages.go`, `Stage.Whole`, `whole`, `JudgesInPart`; `internal/le/changescope_test.go`, `TestAWholeStageReachesNoChangeScopeReader` | A stage declares whether its action judges its whole subject whatever scope was published; the default is in part | Extends: the race pieces drop `Whole`, and the structural test learns the race selector's read of the scope so the claim cannot return |
| Change-set anchor | `internal/le/repo/changed/anchor.go`, `ResolveAnchor`; `.github/workflows/verify.yml`, "Anchor the change set at the push or pull request base" | Computes the merge base the change set starts from; CI points it at the push's `before` commit | Does not change: it is right for the change-set stages. The race pass in CI stops reading it because the full population publishes the whole tree |
| Published change scope | `internal/le/verify/engine/scope.go`, `publishChangeScope`, `scopeFor`; `internal/le/repo/changed/scope.go`, `WholeTree`, `fromFile` | `runMode` publishes one selection per run; the full population publishes `./...` and every tag | Extends: CI's stage loop runs under the scope this producer writes for the full population, so CI and a local full run share one answer. No second producer is written |
| Judged of total | `internal/le/go/staticcheck/staticcheckfeaturematrix.go`, `Notice` (`Judged`, `Total`, `Reached`), `minMatrixRows` | The Staticcheck matrix prints the rows it judged out of the total and never judges zero rows | Does not apply as code: it is the precedent for the rendered shape (judged of claimed) that the `.ci` runner and the unit groups adopt |
| Verify outcome codes | `docs/architecture/testing/verify-freshness-scope.md`, "Native stage execution": 0, 1, 2, and 3 (`Unjudged`) | A stage that reached no verdict answers 3, and `runCode` carries it up | Extends: `examined nothing` answers 3 in the `.ci` runner, so a verify stage that wraps a suite carries it up with no new code |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test <suite> <selector>` and `./le test bgp <suite> <selector>`: an agent or person runs `.ci` tests. Format: runner words, then ids, `--pattern TEXT`, `--start ID` or `-a`.
- `./le test functional <suite>` and the gating run: the native functional action over the suite table.
- `./le test unit <group>` and `./le test unit all`: the group table.
- `le test qemu all` inside the guest.
- `.github/workflows/verify.yml` on a push or pull request: the per-shard stage loop.

### Transformation Path
1. The `.ci` runner discovers and selects records, runs each, and prints the summary line through `Display.Summary`.
2. `runCISubcommandInner` or the bgp run path turns the runner's answer into the process exit code.
3. `Execute` (functional) or `allTestsRun.suiteChild` (QEMU) streams the child and receives its code; the tally reader reads the summary line from the same stream.
4. `Run.Record` or `allTestsRun.suite` turns code plus count into the suite verdict, consulting the suite row's declaration.
5. For a unit group, `gaterun` runs `go test`, and the group answer reads the job log with `runlog.PackageResults`.
6. In CI, the workflow publishes the full population's scope once per shard, then runs each stage; `Selector.Select` reads the published `./...` and the race piece deals its share.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` runner process ↔ its caller | exit code and the stdout summary line `pass N/M ...` or `fail N/M ...` | No |
| `go test` ↔ unit group and QEMU go test phase | exit code and the package result lines in the job log | No |
| CI workflow ↔ every stage process | `ZE_VERIFY_SCOPE_PACKAGES` and `ZE_VERIFY_SCOPE_TAGS` naming the published files | No |
| verify deps stage ↔ verify engine and failure index | `Report` JSON and `Stage.Whole` read by `JudgesInPart` | No |

### Changed-Wire Census

**Search evidence:** scope: `internal/`, `cmd/`, `.github/workflows/`, `test/` and the `le` launcher. Terms: `no tests selected`, `selected == 0`, `Summary(`, `suiteTally`, `RunCISubcommand`, `ErrTestsFailed`, `PackageResults`, `ZE_VERIFY_SCOPE_PACKAGES`, `ScopeFileKey`, `publishChangeScope`, `Whole`, `JudgesInPart`. Positive control: `suiteTally` is found as the known reader of the summary line, and `TestEachShardRunsOnePieceOfTheStaticcheckMatrix` (`internal/le/workflowcheck/workflowcheck_test.go`) as the known reader of the workflow's stage loop. Implementation re-runs the search before the first edit, because other sessions edit these trees.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `.ci` runner exit code | writer | `internal/test/cli/ci_runner.go` `RunCISubcommand`; `internal/test/cli/cmd_bgp.go` run path | a nil error maps to 0, including the `selected == 0` usage branch | change: examined nothing answers 3 with its message |
| `.ci` runner exit code | reader | `internal/le/test/functional/run.go` `Execute`, `runPrepared` | the child code goes to `Run.Record` and nothing else is read | change: 3 is read as examined nothing and judged against the suite row |
| `.ci` runner exit code | reader | `internal/le/test/qemu/alltests.go` `allTestsRun.suite` | the code is kept when a summary line was seen | change: 3 and an all-skipped tally fail unless the selection declares empty |
| `.ci` runner exit code | reader | `internal/le/test/stressrepro/selected.go` `RunSelected` | already fails a zero-exit child with no summary or an all-skipped summary (`qemu-integration.md`) | preserve: a 3 is a failed child there already |
| `.ci` summary line | writer | `internal/test/runner/display.go` `Display.Summary` | prints nothing with no executed and no skipped test | preserve the line; add the examined-nothing line beside it |
| `.ci` summary line | reader | `internal/le/test/qemu/alltests_tally.go` `suiteTally` | the only reader today | change: moves to the runner package so the functional run reads it too |
| `go test` package result lines | writer | the Go toolchain's `go test`, started through `gotoolchain.Toolchain.GoTest` by `internal/le/test/unit/groups.go` `Group.Argv` and the QEMU go test phases | `go test` without `-v` prints one `ok`, `?` or `FAIL` line per package | preserve: Ze does not write these lines |
| `go test` package result lines | reader | `internal/le/runlog/runlog.go` `PackageResults`, used by `internal/le/test/unit/pkg.go` `readLog` | counts passed, no-test and failed packages | extend: the group action and the QEMU go test phases read through it |
| published change scope files and variables | writer | `internal/le/verify/engine/scope.go` `publishChangeScope` | runs only inside `runMode` | extend: CI reaches the same writer before its stage loop |
| published change scope files and variables | reader | `internal/le/repo/changed/changed.go` `Selector.Select`; `internal/le/repo/changed/scope.go` `Scope.Resolve`, `fromFile` | `Select` widens to `./...`; `Resolve` serves `repochanged.Packages` | preserve both readers |
| `Stage.Whole` | writer | `internal/le/verify/engine/stages.go` `whole`, called by `raceStages` and `fullStages` | `raceStages` returns its pieces through `whole` | change: `raceStages` stops calling `whole` |
| `Stage.Whole` | reader | `internal/le/verify/engine/stages.go` `JudgesInPart`; `internal/le/verify/engine/artifacts.go` `carryForward` | a `Whole` stage's verdict replaces any earlier one | preserve the readers; the race pieces now read as judged in part |

### Integration Points
- `Display.Summary` (`internal/test/runner/display.go`): the one producer of the count line; the examined-nothing line is written beside it.
- `PkgReport` and `runlog.PackageResults`: the existing unit outcome vocabulary and counter.
- `publishChangeScope` and `scopeFor`: the existing producer of the full population's scope.
- `Stage.Whole` and `JudgesInPart`: the existing declaration of a stage's judgement.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | CI stops bypassing the scope publication the engine performs for every local run |
| No unintended coupling (components stay isolated) | Yes | Everything changed is under `internal/le/`, `internal/test/` and the workflow; no product package is touched, and `internal/test/runner` imports nothing from `internal/le` |
| No duplicated functionality (extends existing, does not recreate) | Yes | One summary reader moved to its producer, one package counter, one scope producer, one outcome vocabulary |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Build-host tooling, no wire encoding and no hot path |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | One le action registers in the `verify` area like every other. Each empty-population declaration is a field on the row that already registers the suite, group or stage |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched the suite table (`internal/le/test/functional/suites.go`), the unit group table (`internal/le/test/unit/groups.go`), `vmSuites` (`internal/le/test/qemu/alltests.go`) and `fullStages`: each declaration is read from the row itself, and no list names which suites may be empty |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | On a push to `main`, CI's race pieces race only the packages of the Go files the push changed | Read chain: the workflow's stage loop never enters `runMode`; `Selector.Select` with no published scope uses `ResolveAnchor`; the anchor step sets the upstream to `before`. Read, not executed | AC-6 is unnecessary and the release reason falls to P1 | Read the `Unit tests: changed groups (race-instrumented), part <i> of 3:` note in a shard log of the latest `verify` run on `main`, and compare its package list with `./...` | unvalidated |
| A-2 | No legitimate caller relies on a `.ci` runner exiting 0 for an empty selection | `ai/rules/commands.md` names that exit 0 as a defect; callers found by the census | A caller turns red on change and must declare its empty case on its own row | Census search re-run at implementation, plus the functional `runner` suite | unvalidated |
| A-3 | Some suites legitimately skip every test on some hosts | `qemu-integration.md`: `policy` skips its tests unprivileged | Without a declaration the gating run turns red on those hosts | `TestADeclaredAllSkippedSuiteStaysGreen` over the real suite rows | unvalidated |
| A-4 | The whole-tree race pass fits the CI job bound in three pieces | `raceParts` was cut to three pieces when CI raced the whole tree, before 2026-10-24; `timeout-minutes: 180` | The piece count rises, which `TestEachShardRunsOnePieceOfTheStaticcheckMatrix` and the shard count constrain | The shard durations of the first CI run after AC-6, read from the run | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Failing an undeclared zero turns runs red where nothing is wrong, and a red nobody can act on gets routed around (`plan/journal/gate-red-where-nothing-blocks-on-it.md`) | A gating or QEMU run red on an unmodified tree, naming a suite as examined nothing | Each such suite is checked: a host condition gets a declaration on its row with the reason; a broken selector is the defect being caught and is fixed |
| R-2 | Racing the whole tree in CI surfaces data races that were hidden since 2026-10-24 | The first CI run after AC-6 is red in packages this spec never touched | That red is the product speaking and is reported, never suppressed (`ai/rules/pre-release.md`); each race gets a journal row owned by the spec that fixes it |
| R-3 | The whole-tree race pass outruns the job bound | A race shard cancelled at 180 minutes | Raise `raceParts` with the shard rule's test, never narrow the population |
| R-4 | Moving `suiteTally` changes what the QEMU run reads | A QEMU tally test red after the move | The move keeps the type and its tests; the QEMU package imports it from the runner package |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Agents' day-to-day evidence and CI's verdict. A wrong verdict either fails runs that tested something or keeps passing runs that tested nothing. No shipped binary behavior changes |
| How is it reverted? | Single commit revert per phase; no artifact format is persisted |
| Who else touches this path? | `internal/test/runner`, `internal/le/test/**`, `internal/le/verify/**` and `.github/workflows/verify.yml` are edited by other sessions often. `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` reuses this spec's shape (its AC-10). Re-read the trees before editing |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test plugin --pattern <text matching nothing>` | → | `runCISubcommandInner`, the bgp run path, `Runner.Run` answering examined nothing | `test/runner/runner-empty-selection-examines-nothing.ci` |
| `./le test <suite>` where every selected test skips | → | `Runner.Run` and `Display.Summary` rendering examined nothing, exit 3 | `TestAnAllSkippedRunExaminesNothing` |
| `./le test functional <suite>` whose child examined nothing | → | `runPrepared`, `Run.Record` reading the count and the row's declaration | `TestASuiteThatExecutedNoTestFailsTheGatingRun` |
| `./le test unit <group>` over a pattern holding no test | → | the group answer through `PkgReport` | `TestAUnitGroupThatRanNoTestIsNotAPass` |
| `le test qemu all` with a suite whose every test skipped | → | `allTestsRun.suite` and the go test phases | `TestASuiteWhoseEveryTestSkippedIsNotAPass` |
| `.github/workflows/verify.yml` stage loop | → | the scope the engine publishes for the full population, read by `Selector.Select` | `TestCIStagesRunUnderTheFullPopulationScope` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le test <suite>` or `./le test bgp <suite>` with a selector that matches no test | Prints `examined nothing: <selector> matched no test of <suite>`, then the usage, and exits 3 | `-h`, `help` and the list forms print and exit 0, because they ask for no verdict |
| AC-2 | A `.ci` run whose every selected test skipped | Prints `examined nothing: <n> selected, <n> skipped` with each skip reason, and exits 3. A run with at least one executed test keeps today's summary line and code | A run where some tests skipped and at least one executed: its verdict is that test's, and the skips stay listed |
| AC-3 | `./le test functional <suite>` or the gating run, where a suite answers 3 or its summary shows zero executed | The suite's line names `<n> executed, <m> skipped`; an examined-nothing suite is in `FailedNames` and the run exits non-zero. A suite whose row declares the host condition under which skipping all is legitimate answers `examined nothing (declared: <why>)`, and the run stays green | Suites whose row carries the declaration, because a host that lacks the privilege a suite needs is a real state (A-3) |
| AC-4 | `./le test unit <group>` or `./le test unit all` | Each group answer names the packages that passed, held no test, or failed, read from the job log. A group run that exits 0 with no passed package answers `no test ran` and exit 1, in `PkgReport`'s vocabulary | A cross-targeted group, which runs `go vet` by design and answers the existing vet outcome |
| AC-5 | `le test qemu all` | A suite phase whose summary shows zero executed tests is failed with the reason, unless the run's selection (`only needs-linux`, `test <path>`) declares that suite may hold no selected test; the unit, installer and integration phases carry `Counted` from their package result lines, and one that exits 0 with no passed package fails | A suite with no test marked `option=needs-linux` under `only needs-linux`, because the selection asked for none there; it renders as examined nothing, not as a pass line |
| AC-6 | A push to `main` or a pull request, in `.github/workflows/verify.yml` | Every stage of the shard runs under the scope the engine publishes for the full population, so the three race pieces race their hash-dealt share of `./...` and the matrix and functional stages read the same answer a local full run reads | A push that changes no `.go` file still races the whole tree; the piece the deal leaves empty keeps its existing skip |
| AC-7 | Any race piece finishes | Its answer names the population and its source (`published whole tree`, `published change scope`, or `change set since <anchor commit>`) and the number of packages it raced; a skip's reason names the anchor commit | A piece the hash deal left empty, which keeps `skipEmptyRacePiece`'s reason |
| AC-8 | A changed or fast run whose race piece is green, over a failure index holding a full run's race red | The full run's red stands, carried with `carried-from`, because the race pieces are judged in part in a partial population; `TestAWholeStageReachesNoChangeScopeReader` refuses a `Whole` claim on them | The `go lint run` and `go patch-stamps check` `Whole` claims, which read no scope |
| AC-9 | A reader looks for the rule | `docs/contributing/testing.md` states the verdict shape once: the population claimed, the number judged, `examined nothing` as the third verdict, exit 3, and the declaration on the runner's own row, with source anchors to each producer | None: every other page and spec points at that section rather than restating it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestASelectionThatMatchesNothingExaminesNothing` | `internal/test/cli/ci_runner_test.go` | AC-1 for the generic `.ci` subcommand and the bgp run path | |
| `TestHelpFormsStillExitZero` | `internal/test/cli/ci_runner_test.go` | AC-1 survivor | |
| `TestAnAllSkippedRunExaminesNothing` | `internal/test/runner/runner_test.go` | AC-2: exit 3 and the line | |
| `TestOneExecutedTestKeepsTheRunsVerdict` | `internal/test/runner/runner_test.go` | AC-2 survivor | |
| `TestASuiteThatExecutedNoTestFailsTheGatingRun` | `internal/le/test/functional/run_test.go` | AC-3 failure half | |
| `TestADeclaredAllSkippedSuiteStaysGreen` | `internal/le/test/functional/run_test.go` | AC-3 declaration, over the real suite rows | |
| `TestAUnitGroupThatRanNoTestIsNotAPass` | `internal/le/test/unit/testunit_test.go` | AC-4 | |
| `TestACrossTargetedGroupAnswersVet` | `internal/le/test/unit/testunit_test.go` | AC-4 survivor | |
| `TestASuiteWhoseEveryTestSkippedIsNotAPass` | `internal/le/test/qemu/alltests_tally_test.go` | AC-5 suite half | |
| `TestGoTestPhasesCarryTheirPackageCount` | `internal/le/test/qemu/alltests_test.go` | AC-5 go test half | |
| `TestCIStagesRunUnderTheFullPopulationScope` | `internal/le/workflowcheck/workflowcheck_test.go` | AC-6: the workflow publishes the full population's scope through the engine before its stage loop | |
| `TestRaceChangedExecutesTheWholeTreeAFullRunPublished` | `internal/le/verify/deps/racecoverage_test.go` | AC-6 selection half, existing and kept | |
| `TestRacePieceNamesItsPopulationSource` | `internal/le/verify/deps/racecoverage_test.go` | AC-7 | |
| `TestAPartialRaceGreenDoesNotReplaceAFullRaceRed` | `internal/le/verify/engine/artifacts_test.go` | AC-8 | |
| `TestAWholeStageReachesNoChangeScopeReader` | `internal/le/changescope_test.go` | AC-8: extended to the race selector's scope read | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| executed tests in a `.ci` run | 0..N | 1 executed | 0 executed is examined nothing (AC-1, AC-2) | N/A |
| passed packages in a go test run | 0..N | 1 passed | 0 passed with exit 0 is `no test ran` (AC-4, AC-5) | N/A |
| packages a race piece raced | 0..N | 0 with the hash-deal skip | 0 with no stated reason is refused (AC-7) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `runner-empty-selection-examines-nothing` | `test/runner/runner-empty-selection-examines-nothing.ci` | An agent mistypes a test name and reads `examined nothing` and a non-zero code instead of a usage text that exits 0 | |

### Interop Tests (Scope: protocol)
Not applicable. Build-host tooling with no protocol peer (`ai/rules/interop-and-goal-validation.md`).

## Files to Modify
- `internal/test/runner/runner.go` - `Runner.Run` reports examined nothing for an empty or all-skipped selection
- `internal/test/runner/display.go` - the examined-nothing line beside `Display.Summary`
- `internal/test/cli/ci_runner.go` - `runCISubcommandInner` and `RunCISubcommand` answer 3 for examined nothing
- `internal/test/cli/cmd_bgp.go` - the bgp run path answers the same
- `internal/le/test/qemu/alltests_tally.go` - `suiteTally` moves to `internal/test/runner`, beside its producer
- `internal/le/test/qemu/alltests.go` - all-skipped suite and counted go test phases
- `internal/le/test/qemu/alltests_report.go` - the counts the go test phases now carry
- `internal/le/test/functional/run.go` - `runPrepared` and `Run.Record` read the count and the declaration
- `internal/le/test/functional/suites.go` - the `Suite` row gains the declaration
- `internal/le/test/functional/report.go` - the per-suite executed line
- `internal/le/test/unit/actions.go` - the group answer reads the job log through `PkgReport`
- `internal/le/test/unit/pkg.go` - the report shared with the group action
- `internal/le/verify/deps/verifydeps.go` - the race piece names its population source; the stale "green baseline" reason
- `internal/le/verify/engine/stages.go` - the race pieces drop `Whole`; the stale `raceParts` and `raceStages` comments
- `internal/le/verify/engine/scope.go` - the scope the full population publishes is reachable from the workflow through one le action
- `internal/le/changescope_test.go` - the `Whole` scan learns the race selector's scope read
- `.github/workflows/verify.yml` - publish the full population's scope before the stage loop
- `docs/contributing/testing.md` - the rule's one statement (AC-9)
- `docs/architecture/testing/verify-freshness-scope.md` - CI's race population, the race pieces' `Whole` claim
- `docs/architecture/testing/qemu-integration.md` - the all-skipped suite and the counted go test phases
- `docs/functional-tests.md` - exit 3 and the examined-nothing line
- `docs/architecture/testing/ci-format.md` - the runner's exit codes, which the `.ci` runner files declare as their design page
- `ai/rules/commands.md` - the directive that tells the reader to distrust a runner's exit 0 is narrowed to what stays true once the runner refuses

## Files to Create
- `test/runner/runner-empty-selection-examines-nothing.ci` - functional proof of AC-1

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Build-host tooling; nothing an operator configures |
| YANG validation constraints | No | No YANG leaf added |
| YANG custom validators | No | No YANG leaf added |
| CLI commands/flags | Yes | One le action exposing the full population's scope to the workflow; implementation names it in the existing `verify` area after reading `ai/rules/cli.md` |
| CLI grammar (keyword before value) | Yes | That action follows `docs/architecture/cli/command-namespacing.md`; `./le cli grammar` judges it |
| Editor autocomplete | No | No config leaf added |
| Functional test for new RPC/API | Yes | `test/runner/runner-empty-selection-examines-nothing.ci` |
| Pipe completeness | Yes | The counts are payload fields, so `\| json` renders the same structure the text reads |
| Env var registration | No | `ZE_VERIFY_SCOPE_PACKAGES` and `ZE_VERIFY_SCOPE_TAGS` are registered in `internal/le/repo/changed/scope.go` |
| Doctor check for runtime dependencies | No | No path, socket, service, module, port or binary is added |
| Doctor code per problem class | No | Build-host tooling; `ze doctor` reports on the shipped box |
| Prometheus counters/metrics | No | Build-host tooling exports no metric |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Nothing an operator of the shipped binary reaches |
| 2 | Config syntax changed? | No | No config surface |
| 3 | CLI command added/changed? | Yes | `docs/functional-tests.md` and `docs/contributing/running-commands.md` for the runner's exit 3; `docs/guide/command-reference.md` documents the shipped `ze` CLI, not le |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | No | Build-host tooling |
| 7 | Wire format changed? | No | No wire encoding |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol behavior |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/contributing/testing.md` |
| 11 | Affects daemon comparison? | No | No shipped behavior |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md`, `docs/architecture/testing/qemu-integration.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | One le action is added; le's command inventory derives from registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/release/validation/spec-a-gate-reports-the-population-it-judged.md` at implementation. Declared by the changed files: `docs/architecture/testing/ci-format.md` (affected: the `.ci` runner's exit 3 and the examined-nothing line), `docs/architecture/core-design.md` (unaffected: no command engine or composition-root fact changes; the one new le action registers like every other), and the pages in rows 10 and 12 |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `verify-freshness-scope.md` states that the race pieces judge the same packages in every population, and `stages.go` that every CI clone races the whole tree; both statements change |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the declaration field on the suite row and the examined-nothing code path, with nothing failing yet
   - Tests: `TestASelectionThatMatchesNothingExaminesNothing`, `runner-empty-selection-examines-nothing`
   - Files: `internal/test/runner/runner.go`, `internal/test/cli/ci_runner.go`, `internal/test/cli/cmd_bgp.go`, `internal/le/test/functional/suites.go`
   - Verify: the tests fail on today's exit 0, then pass
2. **Phase: the `.ci` runner and its callers** -- all-skipped, the moved tally, the functional verdict
   - Tests: `TestAnAllSkippedRunExaminesNothing`, `TestOneExecutedTestKeepsTheRunsVerdict`, `TestASuiteThatExecutedNoTestFailsTheGatingRun`, `TestADeclaredAllSkippedSuiteStaysGreen`
   - Files: runner, display, the tally move, functional run and report
   - Verify: the gating run over the real suite table names each declared suite and why
3. **Phase: go test runners** -- unit groups and the QEMU go test phases
   - Tests: `TestAUnitGroupThatRanNoTestIsNotAPass`, `TestACrossTargetedGroupAnswersVet`, `TestASuiteWhoseEveryTestSkippedIsNotAPass`, `TestGoTestPhasesCarryTheirPackageCount`
   - Files: `internal/le/test/unit/actions.go`, `pkg.go`, `internal/le/test/qemu/alltests.go`, `alltests_report.go`
4. **Phase: the race pass** -- CI scope, population naming, `Whole`
   - Tests: `TestCIStagesRunUnderTheFullPopulationScope`, `TestRacePieceNamesItsPopulationSource`, `TestAPartialRaceGreenDoesNotReplaceAFullRaceRed`, `TestAWholeStageReachesNoChangeScopeReader`
   - Files: `verifydeps.go`, `stages.go`, `scope.go`, `changescope_test.go`, `.github/workflows/verify.yml`
   - Verify: after the push, read each race shard's package list in the CI log (A-1, A-4) and record the shard durations
5. **Phase: pages** -- AC-9 and the pages in the documentation checklist, in the same commits as the code they describe

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named symbol |
| Correctness | A real red keeps its own code; exit 3 never replaces a 1; a declared empty stays 0 and says why |
| Naming | The phrase is `examined nothing` in every rendering; JSON keys kebab-case (`executed`, `skipped`, `examined-nothing`) |
| Data flow | One summary reader, one package counter, one scope producer; CI and a local full run read one published answer |
| Rule: `ai/rules/no-layering.md` | `suiteTally` is moved, not copied; the race pieces' `Whole` claim is deleted, not overridden |
| Rule: `ai/rules/principles.md` | Each declaration is on its own row and is named as the guard it is; no list of suites allowed to be empty exists anywhere |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| An empty `.ci` selection exits 3 | `./le test functional runner` running `runner-empty-selection-examines-nothing` |
| The runner and suite verdicts | `./le test unit pkg path ./internal/test/runner/`, `./le test unit pkg path ./internal/test/cli/` and `./le test unit pkg path ./internal/le/test/functional/` |
| The unit group verdict | `./le test unit pkg path ./internal/le/test/unit/` |
| The QEMU verdicts | `./le test unit pkg path ./internal/le/test/qemu/` |
| The race pass and CI | `./le test unit pkg path ./internal/le/verify/deps/`, `./le test unit pkg path ./internal/le/verify/engine/`, `./le test unit pkg path ./internal/le/workflowcheck/`, `./le test unit pkg path ./internal/le/ run TestAWholeStageReachesNoChangeScopeReader`, then the race shards of the next CI `verify` run |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Authorization that could fail open | CI's verdict is the release check: a scope publication that fails must fail the shard, never leave the variables unset and fall back to the push diff |
| Input validation | The selector text echoed in `examined nothing: <selector> ...` is printed as data, never re-parsed |
| Resource exhaustion | The whole-tree race pass is bounded by the piece count and the job timeout (R-3) |
| Error leakage | Messages carry test names, package paths and commit ids only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A suite turns red as examined nothing on an unmodified tree | Read why it skipped: a host condition gets a declaration with its reason; anything else is the defect this spec catches |
| The whole-tree race pass finds a race in an untouched package | Journal row owned by the spec that fixes it; never narrow the population |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The population was correct where the engine ran and empty by construction where it did not. Instance 1 was a derivation that read the working tree inside a clean worktree; cause 5 is a derivation that reads a scope CI never publishes. The shape to look for is a run that works only when a step another entry point performs has already happened.
- A tool output that tells the reader to read past it is the defect `ai/rules/writing.md` names. `ai/rules/commands.md` carries such a paragraph about runner exit codes; AC-1 fixes the output so the paragraph can shrink.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

No open journal row names these causes (searched `plan/journal/` for `no tests selected`, `selects nothing`, `usage`, `all-skipped`, `race pass`, `unit-race` on 2026-11-14).

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| CI's stage loop runs under the scope the engine publishes for the full population, through one le action calling the producer `publishChangeScope` uses | Run each shard through `./le verify worktree part <i> of 6` | A detached worktree has no `@{u}`, so `ResolveAnchor` falls to `refs/remotes/origin/HEAD`, which on a push to `main` is HEAD itself, and the line-scoped arch stages would judge nothing (`verify-freshness-scope.md`, "The scoped run and its record") |
| Same | Make the race piece race `./...` itself whenever no scope is published | It would change what a person gets from a standalone `./le verify deps unit-race-changed` and give the race pass a second source of its population beside the published scope |
| Same | Keep CI racing the push diff and document it | The full population is the release check by design, and a release check narrowed to one push judges almost nothing (`verify-freshness-scope.md`) |
| `examined nothing` answers 3 | A new exit code, or 1 | 3 already means "no verdict" in the verify engine, and `runCode` carries it up, so every wrapper reads it with no new code; 1 says a test found the product wrong, which nothing did |
| The declaration lives on the suite row, the group row and the run's selection | A list of suites allowed to be empty | A second declaration of what the row already says (`ai/rules/principles.md`) |
| `suiteTally` moves to `internal/test/runner` beside `Display.Summary` | Copy it into the functional package | One reader per line format; the producer and its reader change together |
| The race pieces drop `Whole` | Keep `Whole` and teach `JudgesInPart` an exception | A stage whose population follows the published scope is judged in part by definition; an exception is a second rule |

## Known Limitations

- Gates outside the runners named here (`./le test weakened check`, `./le rfc check`, the hooks, `./le doc check all`) adopt the shape through `plan/post-release/engineering/spec-gates-share-one-semantic-population.md`, AC-10, which reuses the statement AC-9 lands.
- `captureStdout` (`internal/component/config/schema/cli/main_test.go`) deadlocks past the 64 KB pipe buffer. It is a hang, visible as a timeout, not a signal read as passed, so this rule does not reach it; its record is the 2026-09-03 row in `plan/journal/net-pipe-deadlock.md`.

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
