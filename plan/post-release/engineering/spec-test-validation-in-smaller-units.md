# Spec: test-validation-in-smaller-units

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Development tooling: it cuts the time a session spends proving one side of a change through a run that also exercises every other side. No shipped behaviour changes |
| RFC gaps | - |
| Depends | - |
| Phase | 9/9 |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas, 2026-11-09: "Consider the tests and how they are doing more than one
thing and see how we can break down the validation in smaller units to not
waste so much time with testing a particular side of a test." Then: "Write a
specification to improve LE and the testing with all the things you found
there."

**The problem.** Several test routes judge one thing by running everything
around it, so a session that needs one answer pays for all of them, and a red
in a part it did not touch reads as a red in the part it did. Measured on the
2026-11-09 session (each commit named was read for this spec):

| What the session needed to know | What it had to run | What failed instead |
|---------------------------------|--------------------|---------------------|
| Does the BGP checker judge this capture right | a whole interop scenario: cross-compile, image build, containers, sessions, traffic, checker. 5 to 10 minutes, twice per red walk | `c77a8dd2f3`: every le process panicked at start on a 65-character help line, so every lab helper container (speaker, relay, injector) failed its readiness wait, and four scenarios failed one after another before anyone saw that no scenario had reached its checker |
| Does the oracle speaker's verdict hold | the same scenario | `ac1a28dc21`, `41fb15dad0`: the speaker dialled before Ze listened. A setup race, read as the feature's red |
| Is the LOCAL_PREF observer right | the same scenario | `7081cfdfa7`: the observer judged legal batching wrong. Its fix was proven by a unit test over the frames the lab captured, which the session rebuilt by hand from the failure dump |
| Is this wait's verdict a timeout | the same scenario | `1c2ca3dea8`: a caller's deadline was reported as the command's own timeout |
| Does this fixture rule hold | a functional `.ci` through the direct `le test <suite>` route, whose le is the committed build | the uncommitted fixture code was not in the binary that ran; the rule (`requireDaemonRoot`, `internal/test/fixture/daemon_store.go`) was provable as a plain unit test in under a second |
| Did my change redden this package | the whole package | `internal/le/commit` carried five failures from other sessions' edits; each needed triage before the session's own tests could be read |
| Does this test discriminate this break | a red walk | Go breaks go through an overlay (the RFC recorder already does it), but only for an RFC-tagged unit; a walk of anything else, of a non-Go file, or of a test that reads source files at run time, still edits the shared tree |

**The goal.** Each of these questions gets a run that answers only it:

1. A broken lab helper is refused in seconds, before any image build, and a
   scenario's failure says which phase it failed in (setup, readiness,
   checker), so a setup red is never read as the feature's red.
2. A checker is judged offline against what a real lab run recorded, so a
   checker fix is proven in seconds, and the lab runs once per scenario to
   prove the product end to end.
3. The direct functional route says when the runner it runs lacks the working
   tree's fixture code, and how to get one that has it.
4. A failing test in a package run can be attributed to the session's own
   files or not, by re-running only the failing tests with those files put back
   to HEAD through a Go overlay.
5. A red walk of any Go break, against a unit test, a `.ci` or an interop
   scenario, runs as one `./le` action through the RFC recorder's overlay
   machinery, with no RFC tag needed and the shared tree untouched.
6. A test that answers more than one question, or answers its question
   through a heavier route than the question needs, is caught while it is
   being written or grown, by the `ze-*` skills used in normal development,
   not by a later clean-up pass. Goals 1 to 5 make the existing routes cheaper
   to run; this goal keeps new tests from re-creating the problem.

Thomas, 2026-11-10: "Please also consider how we can improve our ze- skills to
identify when a test is overly long and could be broken down in smaller units.
So it can be done part of normal development to keep testing fast."

**What "overly long" means here.** Two kinds of signal, used together:

| Signal | What it catches | Why it is not enough alone |
|--------|-----------------|----------------------------|
| Shape, read from the test text: several independent behaviours under one test name, so its red does not say which broke; an assertion that a unit test could decide, proven through a daemon, `.ci` or lab (the `requireDaemonRoot` and LOCAL_PREF observer rows above); a diff that appends a new behaviour's assertions to an existing test instead of adding a test | The cause: what to split and where the cheaper route is | A reviewer reads only the tests the diff touches, so a test that was always long is never looked at |
| Cost, read from the run the session already made: the tests that take the largest share of their suite or package | Where the time goes, including tests no diff touches | A slow test can be slow for a good reason (a real end-to-end path); time names a candidate, never a verdict |

The cost signal partly exists: the functional runner keeps a per-test timing
baseline and flags a test that is slower than its own history. It never flags
a test that has always been long, and Go unit runs keep no per-test timing at
all.

**Two findings are owned elsewhere and are not repeated here.**

| Finding | Owner |
|---------|-------|
| Load-sensitive tests mix behaviour and timing (two storage `.ci` cases timed out under load average 50 and passed on rerun; the P-35 load-red members) | `plan/post-release/engineering/spec-test-verdicts-independent-of-load.md` (AC-2 event barriers, AC-3 stall kill, AC-4 named timers, AC-7 wall-clock ratios), and P-35 of `plan/post-release/engineering/spec-tooling-one-off-defects.md`, which records that no adaptive host-load policy is approved. The "timing bound checked by an injected delay, not by real load" idea is an input for that spec's AC-4 and AC-7, not new scope here |
| `TestAnswerAnswersTheInventory` couples the cheap declared plugin list with an hour-long whole-tree proof | `plan/post-release/engineering/spec-inventory-fast-registry-checks.md` (ready) |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every change keeps
  → Constraint: a feature registers and is discovered; the new le actions register through `leroot.Register`/`leroot.RegisterActions` from `init()` in their own package, plus one blank import in `internal/le/register.go`, and no other list learns their names
- [ ] `ai/doors/le-tooling.md` - the door for `internal/le`, `internal/test`, `test/`
  → Constraint: a package sits at the path its command name predicts (`TestEveryCommandIsFoundAtThePathItsNamePredicts`), so `le test redwalk` is `internal/le/test/redwalk` and `le test attribution` is `internal/le/test/attribution`
  → Constraint: no product code imports `internal/le`; everything here lives under `internal/le` or `internal/test`, which only le links (`TestNormalZeLinksNoInternalLe`)
  → Constraint: every concurrent session is an outside party; nothing here may write a tracked file or rebind a shared Docker tag
- [ ] `ai/doors/core.md` - read because the spec names `internal/core/hostload`
  → Constraint: nothing here adds to `internal/core`; the contended-verdict classifier stays as it is
- [ ] `docs/architecture/testing/interop.md` - "How It Works", "Scenario Structure", "Prove a scenario discriminates"
  → Decision: `Suite.Run` (`internal/le/interoplab/lab.go`) already orders Docker probe, orphan report, `Preflight`, `DockerKernel`, image preparation, then scenarios; the helper start probe joins `Preflight`, after `StageBinaries`, before any image build
  → Decision: a bespoke checker is already split into a lab-I/O body and a pure predicate (`check_rfc_predicate.go`) driven offline by `TestBespokeCheckerBranches`; record/replay extends that split to every checker at the `CheckerLab` boundary instead of asking each checker to be rewritten
  → Constraint: a scenario that passes either way proves acceptance, not correctness; replay never replaces the lab's one end-to-end pass for product behaviour, it replaces re-running the lab to judge the checker
- [ ] `docs/architecture/testing/runner-architecture.md` - "Which binaries a fixture reaches", "Timing baseline", "Contended run verdicts"
  → Constraint: a `.ci` `le` exec head runs the runner's own executable (`ownExecutable`, `Runner.lePath`); in the direct route that is the committed build under `bin/builds/`, so compiled fixtures run that build commit's code
  → Constraint: a contended verdict is a classification, never an automatic retry; nothing here retries
- [ ] `docs/architecture/testing/ci-format.md` - "Process Commands", "Sleeps and their justification markers"
  → Constraint: no `.ci` syntax changes; the stale-runner notice is runner output, not a directive
- [ ] `docs/contributing/running-commands.md` - "Launcher builds do not overwrite running binaries", "A functional suite is not the runner binary", "Testing one package while you develop it", "`quiet`", "The changed-set selector"
  → Constraint: le builds follow HEAD, never the working tree; the build directory name carries the commit (`ParseBuildName`, `BuildName.Sha`, `internal/le/go/toolchain/builds.go`), so a running le names the commit it was built from without a new record
  → Decision: `./le test functional <suite>` already builds `le` from the working tree into an isolated set (`buildCommands`, `internal/le/test/functional/binaries.go`); the stale case is only the direct `le test <suite>` route, which the notice covers
  → Decision: a named-test run already exists (`./le job run label unit-pkg quiet command go test -run ...`); this spec adds no second test launcher, only the attribution of a red the run already reported
  → Constraint: the quiet log's failure lines come from one declaration, `internal/le/runlog`; attribution reads failing tests through it
- [ ] `docs/contributing/rfc-conformance-gates.md` - the recorder's routes and "Every carrier applies the break through a Go overlay"
  → Decision: `writeOverlay`, `setGoFlagsOverlay`, `observationRunner.environment`, `argv`, `carrierArgv`, `runFunctional` and `exec` (`internal/le/rfc/discriminate_observe.go`) already run a break against a Go unit, one `.ci` (through `testfunctional.Prepare`) and an interop scenario (the lab's native action with `GOFLAGS=-overlay`); the red walk moves that machinery to a shared package rather than copying it
  → Constraint: the recorder judges an interop red by the scenario's progress line (`interopScenarioVerdict`, `internal/le/rfc/discriminate_interop.go`, markers `: pass (` and `: FAIL (`) and the checker anchor `scenario <name> assertion N:` that `namesAssertion` cuts out of the rest of the line; the phase label must leave both readable
- [ ] `ai/rules/cli.md` - new le actions
  → Constraint: keyword before value, a verb from the closed vocabulary (`le help ai verbs`): `check` (gate, exit 1 on a defect) for the red walk, `report` for attribution
- [ ] `ai/rules/testing.md`, `ai/rules/precommit-verify.md` - tests first, scoped evidence
  → Constraint: no test is weakened; replay tests add proof beside the lab, they never replace a lab assertion
  → Constraint: splitting a test moves assertions between test names; when `./le test weakened check` reports the old name as deleted or weakened, the split goes to an independent `ze-test-judge` agent like any other weakening, and passes only when every old assertion is present in a new test
- [ ] `ai/rules/rule-format.md` - the testing rule is built from point files
  → Constraint: the "one question per test" directive is a new point file under `ai/rules/points/testing/` plus its manifest row; `ai/rules/testing.md`, `CORE.md` and `TRIGGERS.md` are regenerated by `./le ai rules update condensed`, never edited by hand
- [ ] `ai/skills/ze-test.md`, `ai/skills/ze-spec.md`, `ai/skills/ze-implement.md`, `ai/skills/ze-review.md`, `ai/skills/ze-review-deep.md`, `ai/skills/ze-verify.md` - the canonical skill sources (the `.claude/skills/*/SKILL.md` copies derive from them)
  → Constraint: the check is written once, in the testing rule; each skill names the step where it applies and points at the rule, so no skill carries its own copy of the criteria (`ai/rules/principles.md`, "declared once")
  → Decision: `/ze-review` step 1 already sizes the diff against its problem; test shape becomes a step beside it, and Agent 4 (Test Coverage Gaps) of `/ze-review-deep` gains the same question
- [ ] `docs/architecture/testing/runner-architecture.md` - "Timing baseline and auto-timeout"
  → Decision: `tmp/test-timings.json` (EMA per `.ci` test, `Timings.IsSlow`, `FormatSlowTests`, `internal/test/runner/timing.go`) is the `.ci` cost source; the cost report reads it and keeps no second timing file for `.ci`
  → Constraint: baseline updates are suppressed during a contended run; the cost report inherits that and never reads a contended run's times

**Key insights:**
- Every interop red the session met was in setup or in the checker, not in Ze; the lab's verdict has no phase, so each was read as a Ze red first.
- `CheckerLab` is already an interface "a leaf test can implement without Docker" (`lab.go`), so record/replay is a decorator plus an implementation, with no checker edited.
- The overlay machinery for all three carriers exists and is reachable only through an RFC tag; the red walk is its second caller.
- The functional isolated set already builds le from the tree; the stale fixture case is the direct route only.
- The skills already judge a diff's size against its problem, and a test's strength, but never a test's shape: nothing asks whether one test name holds several questions, or whether a `.ci` proves what a unit test could.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/interoplab/lab.go` - `Suite.Run` probes Docker, reports orphans, runs `Preflight`, runs `DockerKernel(StagedZe)`, prepares images, then runs each scenario and prints `interop: [i/n] <name>: pass (<elapsed>)` or `: FAIL (<elapsed>): <error>`. `runScenario` returns `ScenarioResult{Name, Passed, Error, CleanupErrors, Diagnostics}`; its error comes from pre-clean, network, prepare, validation, image preparation, start wrapping, container run, readiness (`lab.waitPeer` when `peer.Ready` is set) or the checker, and nothing says which. `CheckerLab` has ten methods; `Lab` implements it over Docker
- [ ] `internal/le/interoplab/zebuild.go` - `StageBinaries` cross-compiles each `LabBinary{Name, Base, Output}` for the daemon's architecture and judges only the build's exit; a staged binary that panics at start passes preflight
- [ ] `internal/le/interoplab/kernelcheck.go` - `kernelProbeContainer` mounts the staged ze read-only into a `OneShotContainer` run through `RunOneShot`; the route for running a staged binary in a labelled, cancel-safe container already exists
- [ ] `internal/le/interoplab/bgp/run.go` - `LabBinaries` stages `ze`, le (`linuxle`) and `ze with ze_le`; `suiteFor` wires `StageBinaries` as `Preflight`
- [ ] `internal/le/interoplab/bgp/extended_relay.go` - the relay writes `<prefix>-ze.jsonl` and `<prefix>-frr.jsonl`; `extendedRelayDump` reads them back through `CheckerLab.Query` into a failed checker's report; nothing keeps them as test input
- [ ] `internal/le/rfc/discriminate_observe.go`, `internal/le/rfc/discriminate_interop.go` - the overlay and carrier machinery, and the interop verdict parser
- [ ] `internal/le/test/functional/binaries.go` - `buildCommands` builds ze, ze-stripped, le and ze-fakefilter from the tree into the isolated set; `Environment` sets `LE_TEST_NO_BUILD=1` and `ZE_BIN`
- [ ] `internal/test/runner/runner.go` - `NewRunner` takes `lePath` from `ownExecutable`; `Runner.Build`, when `NoBuild()` is false (the direct route), builds `ze` and ze-fakefilter from the tree and never says which commit its own le carries
- [ ] `internal/le/go/toolchain/builds.go` - `ParseBuildName` reads `<UTC time>-<sha>[-<goos>-<goarch>]`; `BuildName.Sha`
- [ ] `internal/le/register.go` - one blank import per le package
- [ ] `internal/test/fixture/` - 497 Go files, 54 test files; fixture decision logic is already unit-testable in the package
- [ ] `internal/test/runner/timing.go` - `Timings.IsSlow` flags a `.ci` test over 2x its own EMA after three samples; `FormatSlowTests` prints those in the summary. Nothing ranks tests against each other
- [ ] `internal/le/runlog/runlog.go` - picks failure lines by prefix (`failureLine`); keeps no per-test elapsed
- [ ] `internal/le/test/unit/actions.go` - a component group runs its argv through the injected `actionRunner` (`gaterun.Run`); whether its log carries per-test elapsed is A-7
- [ ] `ai/skills/ze-test.md` (steps 1 to 3: draft, run, assert on state), `ai/skills/ze-review.md` (step 0 weakening audit, step 1 sizing, step 3 functional coverage), `ai/skills/ze-review-deep.md` (Agent 4), `ai/skills/ze-implement.md`, `ai/skills/ze-spec.md` (Step 4 WRITE), `ai/skills/ze-verify.md` (scoped run and report) - none asks about test shape or test cost

**Behavior to preserve:**
- The progress line prefix `interop: [i/n] <scenario>: pass (` and `: FAIL (`, and the checker anchor `scenario <name> assertion N:` the recorder reads
- `rfc record` behaviour and every recorder test, through the overlay move
- `Suite.Run` order: no image is built before the preflight and kernel check pass
- `./le test functional` isolated-set behaviour and the direct `le test <suite>` route's exit codes; the notice changes no verdict
- No test is retried, no deadline is widened

**Behavior to change:**
- A scenario failure carries its phase
- A staged lab binary that cannot start is refused at preflight
- Every scenario's checker calls are recorded, and a recording can be replayed offline
- The direct functional route names a stale runner
- New: `le test redwalk check` and `le test attribution report`
- New: `le test cost report`, ranking a run's tests by their share of the run
- The testing rule gains "one question per test, at the cheapest route that decides it"; six skills apply it at the step where a test is planned, written, grown, reviewed or run

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lab preflight chain | `internal/le/interoplab/lab.go` `Preflights`, `PreflightCheck`; `zebuild.go` `StageBinaries` | Runs declared host checks in order before any image build | Extends: a `LabBinary` may declare a start probe, run by the same preflight after its build |
| Staged-binary probe container | `internal/le/interoplab/kernelcheck.go` `kernelProbeContainer`; `docker.go` `OneShotContainer`, `RunOneShot` | Mounts the staged ze read-only into a labelled one-shot container and runs it | Extends: the start probe mounts each probed binary the same way, so a darwin host probes a linux binary and a cancelled probe is removed by name |
| Per-image once-per-run probe | `lab.go` `imageStarts`, `wrapStarts`, `Docker.probeLinkTools` | Probes an image's link tools once per run | Does not apply: that probe needs a started scenario; the start probe needs none and runs earlier |
| Scenario result and progress line | `lab.go` `ScenarioResult`, `Suite.Run`, `scenarioLine`, `SuiteReport.Text` | One verdict per scenario, phase unknown | Extends: `ScenarioResult` gains the phase; the line names it after the existing markers |
| Checker surface | `lab.go` `CheckerLab`, `Lab`, `CheckContext` | Interface a checker consumes; documented as implementable without Docker | Extends: a recording decorator around `Lab` and a replaying implementation; no checker changes |
| Pure predicate split | `internal/le/interoplab/bgp/check_rfc_predicate.go`, `TestBespokeCheckerBranches` | Five bespoke checkers judged offline over hand-made inputs | Extends: replay supplies real inputs for the whole checker, not only its predicate; the split stays |
| Relay capture | `bgp/extended_relay.go` `captureExtendedRelay`, `extendedRelayDump` | Writes frames to jsonl in the relay container, read through `CheckerLab.Query` | Does not apply directly: a checker reads the capture through `CheckerLab`, so the recording already holds the bytes the checker saw; no relay change |
| Overlay carriers | `internal/le/rfc/discriminate_observe.go` `writeOverlay`, `setGoFlagsOverlay`, `observationRunner.environment`, `argv`, `carrierArgv`, `runFunctional`, `exec`; `discriminate_interop.go` verdict readers | Runs one Go break against a unit, a `.ci` or a scenario, tree untouched, for an RFC record | Extends by moving: the overlay writer, carrier runners and verdict readers move to `internal/le/test/overlay`, and both `rfc record` and `le test redwalk` call it. No copy stays in `internal/le/rfc` (no layering) |
| Named-test job run and quiet log | `internal/le/job/job.go` `Admission.Run`; `internal/le/runlog` | Runs one argv admitted, keeps a log and its failure lines | Extends: attribution reads the failing tests from a quiet log through `runlog` and runs its re-runs through the same admission |
| Changed-set selector | `internal/le/repo/changed/selector.go`; `running-commands.md` "The changed-set selector" | Packages and tags a change reaches | Does not apply: attribution needs the session's own files, which the selector cannot tell from other sessions' edits; the caller names them, as `./le commit create` does |
| Isolated functional set | `internal/le/test/functional/binaries.go` `buildCommands`, `Prepare` | Builds le from the working tree for `./le test functional` | Does not apply: that route is never stale; the notice names it as the fix |
| Build name | `internal/le/go/toolchain/builds.go` `ParseBuildName`, `BuildName.Sha` | Reads a build directory's commit | Extends: the direct runner reads its own executable's build commit |
| Contended verdict | `internal/test/runner/hostload.go`, `internal/core/hostload` | Classifies a red under load | Does not apply: owned by `spec-test-verdicts-independent-of-load.md` |
| `.ci` timing baseline | `internal/test/runner/timing.go` `Timings`, `IsSlow`, `FormatSlowTests`; `tmp/test-timings.json` | Per-test EMA; flags a test slower than its own history | Extends: the cost report reads the same file and ranks tests against their suite; the slow-against-history flag stays as it is |
| Diff sizing in review | `ai/skills/ze-review.md` step 1 | "Is this change bigger than the problem it solves?" | Extends: a test-shape step beside it asks the same of each test the diff adds or grows |
| Test coverage agent | `ai/skills/ze-review-deep.md` Agent 4 | Behaviour versus execution, edge cases, a `.ci` for the feature | Extends: one more question, pointing at the testing rule |
| Weakening audit and judge | `./le test weakened check`, `ai/skills/ze-test-judge.md` | Refuses a test change that lowers what it proves without an independent approval | Does not change: a split the audit flags goes through it, and is approved only when every old assertion survives in a new test |
| `.ci` authoring steps | `ai/skills/ze-test.md` steps 1 to 3 | Draft outside `test/`, run as a draft, assert on state | Extends: a step before drafting names the one question and the cheapest route; the draft run's cost is read before promotion |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test integration <lab>` (with or without a scenario selector): the lab suite
- A Go test in `internal/le/interoplab/<lab>/` that replays a recording from its `testdata/`
- `le test <suite>` direct: the functional runner
- `./le test redwalk check ...` and `./le test attribution report log <quiet-log> package <pkg> tags <tags> file <path> [file <path> ...]`: new actions
- `./le test cost report ...`: new action, run by `/ze-verify` after its scoped run and by `/ze-test` after a draft run
- The `ze-*` skills, at the steps named in AC-20 to AC-23

### Transformation Path
1. Lab: `StageBinaries` builds; the start probe runs each probed binary in a one-shot container; a refusal ends the run with a setup error before images
2. Lab: `runScenario` wraps `Lab` in the recorder, runs setup, readiness and the checker, tags the result with the phase reached, writes the recording to session scratch, names its path in the failure's diagnostics
3. Replay: a test opens a recording, builds a replaying `CheckerLab`, calls the scenario's registered checker, and asserts the verdict and assertion number
4. Direct runner: `Runner.Build` (direct route only) resolves its own executable, parses the build commit from its directory name, lists Go files that differ between that commit and the working tree under the packages le links from `internal/test/`, and prints the notice
5. Red walk: parse the break, write the overlay, run the carrier clean then broken, judge both with the shared verdict readers, print the result, remove the overlay directory
6. Attribution: read failing tests from the quiet log, write an overlay mapping each file named by a repeated `file <path>` to its HEAD content (or deletion for a file HEAD lacks), re-run each failing test once as on disk and once under the overlay, classify into the seven AC-15 classes
7. Cost: read per-test times (the `.ci` baseline file for a suite; a unit run's per-test elapsed for a package), compute each test's share of its suite or package, print the tests above the AC-18 thresholds, each with its share, its time and the question to ask of it

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le ↔ Docker daemon | `RunOneShot` of the probe container | No |
| le ↔ child `go test`/`le test`/lab action | argv and `GOFLAGS=-overlay=<path>` in the child environment | No |
| lab run ↔ later test | a jsonl recording file in session scratch, copied by hand into `testdata/` | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `test/`, `.github/`, `scripts/`, `docs/`; terms `": FAIL ("`, `": pass ("`, `ScenarioResult`, `failed-names`, `interopVerdictFail`, `interopScenarioVerdict`. Known control: `internal/le/rfc/discriminate_interop.go` reads `": FAIL ("` (found). The other Go hit for `FAIL (`, `internal/le/cli/dispatch/report.go`, prints a different report. `ScenarioResult` JSON fields are rendered by `SuiteReport.Text` and the shared renderer; the stress reports carry their own types. The checker recording is a new format with one writer and one reader, both new.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| interop progress line | writer | `internal/le/interoplab/lab.go` `Suite.Run` | prints `: pass (` / `: FAIL (<elapsed>): <error>` | change: ` in <phase>` between `)` and `: <error>`; markers unchanged |
| interop progress line | reader | `internal/le/rfc/discriminate_interop.go` `interopScenarioVerdict`, `interopAttribution`, `namesAssertion` | reads from the marker on and cuts the checker anchor out of the rest | preserve: the anchor is still in the rest; a recorder test over a labelled line proves it |
| interop progress line | reader | CI job logs (`.github/workflows/` run the action and read no line) | grep of `.github/` for the markers finds none | preserve |
| `ScenarioResult` JSON | writer | `lab.go` `runScenario` | `name`, `passed`, `error`, ... | change: add `phase` (`setup`, `readiness`, `checker`), omitted on pass |
| `ScenarioResult` JSON | reader | `lab.go` `SuiteReport.Text`; the `| json` renderer | renders fields | change: `Text` prints the phase |
| checker recording (jsonl) | writer | new `internal/le/interoplab/record.go` recording `CheckerLab` | new | new format: one line per call (method, peer, argv, environment names, answer, error text, exit code), headed by a version line |
| checker recording (jsonl) | reader | new `internal/le/interoplab/replay.go` replaying `CheckerLab` | new | new; refuses a recording of another version |

### Integration Points
- `interoplab.Suite.Run`, `runScenario` - phase, recorder wrap, recording path
- `interoplab.LabBinary`, `StageBinaries` - start probe
- each lab's `LabBinaries` - le declares its probe
- `internal/le/rfc` - imports the moved overlay package
- `internal/test/runner.Runner.Build` - the notice
- `internal/le/runlog` - failing test names for attribution
- `internal/le/job` - admission for every child run

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the probe runs through `RunOneShot`; children run through `job` admission |
| No unintended coupling (components stay isolated) | Yes | all code stays in `internal/le` and `internal/test`; no product package changes |
| No duplicated functionality (extends existing, does not recreate) | Yes | the overlay machinery moves to one package with two callers |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no product hot path; text built with `textbuf` per house style |
| Registration over hardcoding, outbound | Yes | the two actions register from their own `init()`; probes are declared on `LabBinary` by each lab |
| Registration over hardcoding, inbound | Yes | searched `internal/le/register.go` (one blank import per package is the registration route), `internal/le/rfc/carriers.go` `interopTrees` (read from lab catalogs), `scenarioOperations` (untouched); the red walk derives its interop carriers from the same lab catalogs the recorder reads |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every le process runs the registration checks that panicked in `c77a8dd2f3`, so `le help` in the probe container would have panicked too | `c77a8dd2f3` body: "every le process panicked at start" | the probe argv must reach the failing path; choose one that builds the full root | a unit test that stages a le with an over-long help line and sees the probe refuse | confirmed: source, `leroot.DispatchWithStartup` (`internal/le/le/root/dispatch.go`) runs `checkRegistrations` before any answer, help included; a real lab run printed `interop: start probe le (le help): pass` on a healthy le; a real session le carrying a 68-character summary panicked the same way at start, proving the class |
| A-2 | Every checker reaches the lab only through `CheckerLab` | `CheckerLab` doc comment; `CheckContext.Lab` | a checker that dials a container directly escapes the recording; replay of it is refused, not wrong | grep of `internal/le/interoplab/*/` checkers for `exec.Command`, `net.Dial`, direct `Docker` use | confirmed, with one exception: `internal/le/interoplab/ipsec/mobike_netns_process_linux.go` runs `exec` outside `CheckerLab`, so its scenario's replay is refused rather than wrong (Phase F grep) |
| A-3 | Replaying answers in recorded order, matched by method, peer and argv, reproduces a checker's control flow, poll loops in `wait.go` included | `wait.go` polls through `Exec` | a checker that branches on wall time diverges; replay then refuses at the first unmatched call | replay of three recordings: a plain session checker, a poll-heavy one, a bespoke one | partly confirmed: a poll loop that ended on a ready answer replays the same probes in order; a poll that ended on its deadline may start one extra probe on replay, which is refused or answered by the next same-argv call, so such a recording is judged by a lab run (`docs/architecture/testing/interop.md`, replay limits) |
| A-4 | A Go overlay can map a file to its HEAD content, and can delete a file HEAD lacks (an empty replacement) | `go help build`, `-overlay` | attribution of a session that added a file cannot run; it answers `undetermined` | `TestOverlayDeletesAFileHeadLacks` | confirmed: `TestOverlayDeletesAFileHeadLacks` passes |
| A-5 | Moving the recorder's overlay code to a shared package changes no recorder verdict | the code is moved, not edited | a recorder test goes red | the whole `internal/le/rfc` package test after the move | confirmed: the 62 discriminate tests in `internal/le/rfc` pass after the move; the whole package runs past 10 minutes on its corpus tests, and none of them failed |
| A-6 | `internal/test/runner` may import `internal/le/go/toolchain` | `internal/test/fixture/ui_fixture_le_checks_answers.go` already imports it | the build-name parse needs a home outside `internal/le` | `./le arch tier check` | confirmed: `internal/test/runner` imports the toolchain build-name parse (`gotoolchain.ParseBuildName`), and the tier check shows no finding on it |
| A-7 | A unit group run or a `./le job run ... go test` run can keep per-test elapsed (`go test -json` `Elapsed`, or the `--- PASS: X (Ns)` lines of `-v`) in its log without changing how its verdict and failure lines are read | `go help testflag`; `runlog` reads lines by prefix | the Go half of the cost report needs the run to write a second, `-json` log beside the quiet one | read `gaterun.Run`'s argv and a unit group log | partly confirmed: a `-v` log keeps per-test elapsed and its runlog failure lines still read; `-json` would break runlog; `./le test unit <group>` adds neither, so the cost report reads the log of a `./le job run ... go test -v` run |
| A-8 | A test's share of its own suite or package run is stable enough across hosts to rank candidates, where its absolute time is not | the ranking compares tests run on the same host in the same run | ranking flips between hosts; the report names the host it read and stays a hint, which it already is | compare one suite's ranking on two machines | unvalidated: needs two machines; the report names the host it read |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Recordings fill scratch: the BGP lab has 234 scenarios | scratch size after a full run | full runs keep only failed scenarios' recordings; selector runs keep all; each answer is bounded at a declared byte cap, marked truncated, and replay refuses a truncated answer |
| R-2 | A recording carries a secret a checker read (a PSK, a RADIUS secret) and is copied into `testdata/` | review of a committed recording | the recorder writes environment variable names, never values; the doc says a recording is reviewed before commit like any fixture |
| R-3 | The phase label breaks a reader that parses the line | the recorder test over a labelled line | the label sits after the elapsed time, both markers unchanged; census rows above |
| R-4 | Fail-fast (D1) stops a full run whose setup failure was one scenario's own config | a scenario named in a fail-fast stop passes alone | it counts only consecutive setup or readiness failures; the stop names every scenario not run, never as passed |
| R-5 | Attribution says `not-caused` for a red the session's change causes only together with another session's edit | the red persists after the other session commits | the verdict states what it compared (the named files at HEAD vs on disk, everything else as on disk) |
| R-6 | The stale-runner notice fires on every direct run in a busy checkout and is ignored | count of notices in a day's logs | it fires only when a changed Go file belongs to a package le links from `internal/test/`, and prints at most ten paths plus a count |
| R-7 | A red walk killed mid-run leaves its overlay directory | leftover `run-*` directories in scratch | the overlay lives in session scratch, never in the tree; the shared tree is never written, so a kill cannot leave a break behind |
| R-8 | A split loses an assertion on the way | `./le test weakened check` reports the old test deleted | the split is judged by `ze-test-judge`; the rule says a split moves every assertion, and the review step asks where each old assertion now lives |
| R-9 | The shape check is applied to tests whose length is their purpose (a whole-path `.ci`, a reload sequence, a lifecycle test) and splits them into pieces that no longer prove the sequence | a split `.ci` whose parts each start a daemon | the rule names the exception: a test whose question is the sequence itself stays one test; the step asks "would its red say which behaviour broke", not "is it long" |
| R-10 | The cost report is read as a performance figure | a duration quoted in a report or commit | the report is a developer triage list, prints shares first, and its doc says its times are host-dependent and never compared or recorded (`ai/META-DECISIONS.md`, performance figures) |
| R-11 | Six skills each grow their own wording of the check and drift | two skills give different criteria | the criteria live only in the rule point; each skill carries one line naming its step and the rule |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: a lab, a functional run or a red walk gives a wrong or missing verdict to a developer |
| How is it reverted? | Single commit revert per phase |
| Who else touches this path? | `spec-tooling-one-off-defects.md` (in progress, edits `internal/le/interoplab/*` and `internal/le/rfc`), `spec-interop-checkers-cannot-pass-on-an-empty-read.md`, `spec-harness-cannot-pass-on-its-own-failure.md`, `spec-test-verdicts-independent-of-load.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `Suite.Run` with a staged le that panics | → | start probe in `StageBinaries` | `TestStageBinariesRefusesABinaryThatCannotStart` |
| `Suite.Run`, scenario failing in readiness | → | `runScenario` phase tag | `TestScenarioFailureNamesItsPhase` |
| `Suite.Run`, any scenario | → | recording `CheckerLab` | `TestRunScenarioRecordsCheckerCalls` |
| Go test replaying a recording | → | replaying `CheckerLab` | `TestReplayGivesTheLabVerdict` |
| `le test bgp plugin` direct, stale build | → | `Runner.Build` notice | `TestRunnerNamesAStaleBuild`, `test/runner/stale-runner-notice.ci` |
| `./le test redwalk check` | → | shared overlay carriers | `TestRedwalkCheckUnitRoute`, `test/ui/le-test-redwalk.ci` |
| `./le test attribution report` | → | overlay to HEAD and re-run | `TestAttributionClassifiesEachFailingTest`, `test/ui/le-test-attribution.ci` |
| `./le rfc record` after the move | → | `internal/le/test/overlay` | the existing `internal/le/rfc` recorder tests, unchanged |
| `./le test cost report suite <suite>` | → | `.ci` baseline ranking | `TestCostRanksCITestsByShare`, `test/ui/le-test-cost.ci` |
| `./le test cost report log <unit-log>` | → | Go per-test ranking | `TestCostRanksGoTestsByShare` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A scenario fails before its checker runs (pre-clean, network, prepare, validation, images, start) | Its progress line reads `interop: [i/n] <name>: FAIL (<elapsed>) in setup: <error>` and its `ScenarioResult` carries `phase: setup`; a readiness failure says `in readiness`; a checker failure says `in checker` and still carries `scenario <name> assertion N:` | A passing line is unchanged, because nothing failed and the recorder reads `: pass (` |
| AC-2 | The recorder reads a phase-labelled FAIL line | `interopBrokenUnobserved` and `interopAttribution` give the same verdicts as for the unlabelled line, and an `in setup` or `in readiness` red is refused as not the cited assertion, naming the phase | None: a setup red never proves a checker assertion |
| AC-3 | A lab stages a le that panics or exits non-zero at start | `Suite.Run` returns a setup error before any image build, naming the binary, its output path, the probe argv, the exit status, the first lines of its stderr, and the fix (correct the start failure the stderr names, then run `./le test integration <lab>` again) | A binary that declares no probe is not run, because a staged daemon needs a config to start; `ze` stays covered by `DockerKernel` |
| AC-4 | A healthy staged le | The probe passes and the suite proceeds to images and scenarios; the probe's elapsed time is printed on its progress line | None |
| AC-5 | Any scenario runs | Every `CheckerLab` call its checker makes is written, in order, to `<session scratch>/interop-recordings/<lab>/<scenario>-<run id>.jsonl`; a failed scenario's diagnostics name that path; a full run keeps failed scenarios' recordings, a selector run keeps all | A call's environment values are not written, only names, so no secret lands in scratch |
| AC-6 | A test replays a recording of a passing scenario, and one of a failing scenario, through the scenario's registered checker | The replayed verdict equals the lab's: pass replays as pass; fail replays as fail with the same assertion number and cause text | None: a different verdict is a replay defect |
| AC-7 | A checker issues a call the recording does not hold at that position, or the recording is exhausted, truncated or of another version | Replay returns an error naming the expected and the actual call (method, peer, argv) and says the checker must be judged by a lab run | None: guessing an answer would let a checker change pass on data it never saw |
| AC-8 | `7081cfdfa7`'s observer fix | A replay test over a committed recording of `bgp-local-pref-strip-gobgp` passes on the fixed observer and fails on the pre-fix observer (red walk recorded in the commit); `TestLocalPrefStripChecksDestinationWire` keeps its hand-built frames | The hand-built cases stay, because they cover polarities no lab run produced |
| AC-9 | `le test <suite>` direct, its le a `bin/builds/<time>-<sha>` build, and Go files under a package le links from `internal/test/` differ between `<sha>` and the working tree | The runner prints, before the first test, one notice naming the build commit, up to ten changed paths and their count, and the fix: `./le test functional <suite>` or `./le --name session test <suite> ...`; a failing summary repeats it | No notice under `./le test functional` (isolated set from the tree), under `--name` builds, or when no linked file differs, because those runners carry the tree's fixture code |
| AC-10 | `./le test redwalk check break <patch> unit <Test> package <pkg>` where the clean run passes and the broken run fails naming the test | Exit 0; prints the clean pass line, the broken FAIL line, and the overlay's file list; every tracked file's content is byte-identical before and after, including when the run is killed | None |
| AC-11 | The same with the broken run still green | Exit 1: the test does not discriminate this break, with the broken run's excerpt | None |
| AC-12 | The same with the clean run red, or a broken build that does not compile | Exit 1 naming which: a clean red is no baseline; a compile failure is not a red of the unit | None |
| AC-13 | `./le test redwalk check break <patch> ci <suite>/<name>` and `... scenario <lab>/<name>` | The AC-10 to AC-12 verdicts, judged through the moved `.ci` result reader and interop verdict reader; an interop red counts only `in checker` | A setup or readiness red is refused, per AC-2 |
| AC-14 | A break patch names a non-Go file | Exit 2 before any run: the refusal names the file, says a Go overlay cannot reach it, and gives the fix (D2 decided as b; the private-root route for non-Go files is not built here) | None: walking a non-Go break through a Go overlay would report a green the break never reached |
| AC-15 | `./le test attribution report log <quiet-log> package <pkg> tags <tags> file <path> [file <path> ...]` after a package run with failures (`file` repeats, as in `./le commit create`: le has no trailing-list keyword) | One row per failing test, each in exactly one class and none folded into another: `caused` (green with the named files at HEAD), `not-caused` (red both ways), `undetermined` (a build does not compile, with the compiler's first line), `added` (HEAD holds no such test, so the named files add the red test), `red-at-head-only` (green as on disk, red at HEAD), `not-reproduced` (green both ways), `not-run` (the package as on disk holds no test of that name); exit 1 when a row is `caused` or `added`, else 2 when a row is `undetermined` or `not-run` (judged nothing), else 0 | Tests that passed in the log are not re-run, because the question concerns only the reds |
| AC-16 | `./le rfc record` on any route after the move | Same records and refusals as before; `internal/le/rfc` holds no overlay writer or carrier runner of its own | None: no layering |
| AC-18 | `./le test cost report suite <suite>` over a `.ci` baseline, and `./le test cost report log <unit-log>` over a unit run that kept per-test elapsed | One row per test whose share of its suite or package run is in the top 5% of that run and above a floor (2 s for a Go test, 15 s for a `.ci`), sorted by share; each row gives the share, the time and the question "does its red say which behaviour broke, and could a cheaper route decide it". Thresholds are declared once in the package. Exit 0 always: it is a report, never a gate | A contended run's times are not read (the baseline already suppresses them). A test with fewer than three baseline samples is listed as unmeasured, not ranked |
| AC-19 | The testing rule | A new point under `ai/rules/points/testing/`: a test answers one question, through the cheapest route that decides it (unit, then `.ci`, then lab); a test fails it when its red would not say which behaviour broke, or when a unit test could decide its assertion; a test whose question is a sequence (reload, lifecycle, whole path) stays one test; a split moves every assertion and goes to `ze-test-judge` when the weakening audit flags it. `ai/rules/testing.md` regenerated | None |
| AC-20 | `/ze-spec` Step 4 and `/ze-implement` | `/ze-spec`: each TDD Test Plan row validates one behaviour and its route is the cheapest that decides it. `/ze-implement`: a phase adds a new test for a new behaviour rather than appending to an existing test. Each points at the AC-19 rule | None |
| AC-21 | `/ze-test` | A step before drafting names the test's one question and why a unit test cannot decide it; after the draft run, the step reads `./le test cost report suite <suite>` for the draft | None |
| AC-22 | `/ze-review` and `/ze-review-deep` Agent 4 | A test-shape step beside the sizing step, applied to every test the diff adds or grows: a new or grown test that breaks the AC-19 rule is an ISSUE; a pre-existing test the diff does not touch is never a finding | A long test the diff does not touch is out of the review's scope; the cost report surfaces it instead |
| AC-23 | `/ze-verify` after a scoped run | The report ends with the cost report's rows for the suites and packages it ran, as NOTE lines, never as a failure | A full-mode `/ze-verify` runs nothing, so it prints none |
| AC-17 | Docs | `interop.md` gains "Which layer proves what" (producer unit test for product behaviour, replay for checker logic, one lab pass per scenario end to end, start probe for helpers) and documents the phase, the start probe, recording and replay; `running-commands.md` documents redwalk, attribution and the notice; `runner-architecture.md` "Which binaries a fixture reaches" names the notice; `ai/patterns/functional-test.md` says fixture decision logic is unit-tested in `internal/test/fixture` and the `.ci` proves the real path; `running-commands.md` documents `le test cost report` and says its times are host-dependent triage, never a performance figure; `runner-architecture.md` "Timing baseline" names the cost report as the baseline's second reader | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStageBinariesRefusesABinaryThatCannotStart` | `internal/le/interoplab/zebuild_probe_test.go` | AC-3, through a fake runner answering the probe with exit 2 and a panic line | |
| `TestStageBinariesProbesOnlyDeclaredBinaries` | same | AC-3 survivor | |
| `TestScenarioFailureNamesItsPhase` | `internal/le/interoplab/lab_phase_test.go` | AC-1, one case per phase | |
| `TestInteropVerdictReadsAPhaseLabelledLine` | `internal/le/test/overlay/interop_test.go` | AC-2 | |
| `TestRunScenarioRecordsCheckerCalls` | `internal/le/interoplab/record_test.go` | AC-5, order and no environment values | |
| `TestReplayGivesTheLabVerdict` | `internal/le/interoplab/replay_test.go` | AC-6 | |
| `TestReplayRefusesAnUnrecordedCall` | same | AC-7, each refusal shape | |
| `TestLocalPrefStripReplaysTheLabRecording` | `internal/le/interoplab/bgp/check_local_pref_wire_test.go` | AC-8 | |
| `TestRunnerNamesAStaleBuild` | `internal/test/runner/stale_test.go` | AC-9, each survivor | |
| `TestRedwalkCheckUnitRoute` | `internal/le/test/redwalk/redwalk_test.go` | AC-10 to AC-12 | |
| `TestRedwalkLeavesTheTreeUntouched` | same | AC-10, killed child | |
| `TestAttributionClassifiesEachFailingTest` | `internal/le/test/attribution/attribution_test.go` | AC-15, each class | |
| `TestOverlayDeletesAFileHeadLacks` | `internal/le/test/overlay/overlay_test.go` | A-4 | |
| `TestCostRanksCITestsByShare` | `internal/le/test/cost/cost_test.go` | AC-18, `.ci` baseline, floor, unmeasured rows | |
| `TestCostRanksGoTestsByShare` | same | AC-18, unit log | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| notice path list | 0-10 printed | 10 | N/A | the 11th path is counted, not printed |
| recording answer cap | 0 to the declared byte cap | the cap | N/A | cap+1 is marked truncated and replay refuses it |
| fail-fast count (if D1 picks b) | 1 to N | the declared count | 0 is refused at declaration | N/A |
| cost floor | Go 2 s, `.ci` 15 s | a test at the floor is not listed | N/A | the first test above it is listed when its share qualifies |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `stale-runner-notice` | `test/runner/stale-runner-notice.ci` | A developer runs a suite directly with a stale le and is told so, and how to fix it | |
| `le-test-redwalk` | `test/ui/le-test-redwalk.ci` | A developer walks a break against a unit test and reads the verdict; the tree is unchanged | |
| `le-test-attribution` | `test/ui/le-test-attribution.ci` | A developer attributes a package run's reds | |
| `le-test-cost` | `test/ui/le-test-cost.ci` | A developer reads which tests of a suite cost the most, from a fixture baseline | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-local-pref-strip-gobgp` (existing) | `test/interop/scenarios/` | GoBGP | The recording a real run writes replays to the lab's verdict (AC-6, AC-8) | |

## Files to Modify
- `internal/le/interoplab/lab.go` - `ScenarioResult.Phase`, phase in `runScenario`, recorder wrap, progress line, `SuiteReport.Text`
- `internal/le/interoplab/zebuild.go` - `LabBinary` start probe, run after `stageBinary`
- `internal/le/interoplab/kernelcheck.go` - share the read-only mount route of `kernelProbeContainer`
- `internal/le/interoplab/bgp/run.go` - le declares its probe; every other lab that stages le declares it in its own `LabBinaries`
- `internal/le/interoplab/bgp/check_local_pref_wire_test.go` - AC-8
- `internal/le/rfc/discriminate_observe.go`, `internal/le/rfc/discriminate_interop.go` - call the moved package
- `internal/test/runner/runner.go` - the notice in `Runner.Build`
- `internal/le/register.go` - three blank imports
- `internal/le/test/unit/actions.go` or the job route - keep per-test elapsed, per A-7
- `ai/rules/points/testing/manifest.md` - the new point's row
- `ai/skills/ze-spec.md`, `ai/skills/ze-implement.md`, `ai/skills/ze-test.md`, `ai/skills/ze-review.md`, `ai/skills/ze-review-deep.md`, `ai/skills/ze-verify.md` - one step each, pointing at the rule (AC-20 to AC-23); `.claude/skills/*` regenerated from them
- `docs/architecture/testing/interop.md`, `docs/architecture/testing/runner-architecture.md`, `docs/contributing/running-commands.md`, `docs/contributing/rfc-conformance-gates.md` (overlay source anchors), `ai/patterns/functional-test.md`

## Files to Create
- `internal/le/interoplab/record.go`, `internal/le/interoplab/replay.go` and their tests
- `internal/le/test/overlay/` - moved overlay writer, carrier runners, verdict readers
- `internal/le/test/redwalk/` - `le test redwalk check`
- `internal/le/test/attribution/` - `le test attribution report`
- `internal/le/test/cost/` - `le test cost report`
- `ai/rules/points/testing/one-question-per-test.md` - the AC-19 point
- `test/ui/le-test-cost.ci`
- `internal/le/interoplab/bgp/testdata/bgp-local-pref-strip-gobgp/` - one reviewed recording
- `test/runner/stale-runner-notice.ci`, `test/ui/le-test-redwalk.ci`, `test/ui/le-test-attribution.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le actions declare their keywords on registration; no daemon RPC |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `internal/le/test/redwalk`, `internal/le/test/attribution` registrations |
| CLI grammar (keyword before value) | Yes | keywords `break`, `unit`, `package`, `ci`, `scenario`, `log`, `tags`, `files`; `./le cli grammar` |
| Editor autocomplete | N-A | le completion derives from registration |
| Functional test for new RPC/API | Yes | `test/ui/le-test-redwalk.ci`, `test/ui/le-test-attribution.ci` |
| Pipe completeness | Yes | both actions answer through the shared renderer, so `| json` works |
| Env var registration | N-A | none added |
| Doctor check for runtime dependencies | N-A | development tooling; no runtime dependency of `ze` |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | developer tooling, no `features/` entry |
| 2 | Config syntax changed? | N-A | none |
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md` (le, not `ze`) |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | N-A | none |
| 7 | Wire format changed? | N-A | no protocol wire |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | the recorder's behaviour is preserved (AC-16) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`, `runner-architecture.md`, `ai/patterns/functional-test.md`, and `docs/functional-tests.md` where it names the direct route |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | Yes | `docs/contributing/rfc-conformance-gates.md` source anchors for the moved overlay code. `docs/architecture/core-design.md` (declared by `internal/le/register.go`) is unaffected: two blank imports follow the composition-root pattern it already describes |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | le commands only; `le help` derives from registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/post-release/engineering/spec-test-validation-in-smaller-units.md` at implementation; known: `interop.md` anchors on `lab.go` and `zebuild.go`, `rfc-conformance-gates.md` on `discriminate_observe.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `rfc-conformance-gates.md` overlay prose; `interop.md` progress line examples |

## Implementation Steps

1. **Phase: Wiring** - register `le test redwalk` and `le test attribution` with failing stubs; add `ScenarioResult.Phase` with a failing phase test
   - Tests: `TestScenarioFailureNamesItsPhase`, `TestRedwalkCheckUnitRoute`, `TestAttributionClassifiesEachFailingTest`
   - Files: `lab.go`, the two new packages, `internal/le/register.go`
   - Verify: actions reachable, tests red on the stubs
2. **Phase: Phase label and recorder reader** - AC-1, AC-2
   - Files: `lab.go`, `internal/le/rfc/discriminate_interop.go`
3. **Phase: Start probe** - AC-3, AC-4; then fail-fast per D1
   - Files: `zebuild.go`, `kernelcheck.go`, each lab's `LabBinaries`
4. **Phase: Record and replay** - AC-5 to AC-8
   - Files: `record.go`, `replay.go`, `lab.go`, the local-pref test and its recording
   - Verify: one lab run of `bgp-local-pref-strip-gobgp` (main thread) writes the recording; the replay test is red on the pre-fix observer through an overlay
5. **Phase: Overlay move, then red walk** - AC-16, then AC-10 to AC-14
   - Files: `internal/le/test/overlay/`, `internal/le/rfc/*`, `internal/le/test/redwalk/`
6. **Phase: Attribution** - AC-15
7. **Phase: Stale runner notice** - AC-9
8. **Phase: Test shape in normal development** - AC-18 to AC-23
   - First the rule point (AC-19), then `le test cost report` (AC-18), then the six skill steps, each pointing at the rule
   - Files: `ai/rules/points/testing/`, `internal/le/test/cost/`, `ai/skills/*.md`
   - Verify: `./le ai rules update condensed` regenerates `testing.md` with the point; a `/ze-review` over a diff that appends a second behaviour to an existing test reports the ISSUE
9. **Docs** - AC-17, written in each phase as it changes behaviour, checked at the end

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Replay never answers a call it did not record; a setup red never reads as a checker red anywhere |
| Naming | le actions follow the closed verb list and the path rule |
| Data flow | No tracked file is written by redwalk or attribution at any point |
| Rule: no-layering | No overlay writer or carrier runner remains in `internal/le/rfc` |
| Rule: testing | No existing assertion removed; hand-built frames in the local-pref test stay |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Phase in every failed scenario | `TestScenarioFailureNamesItsPhase` |
| Start probe | `TestStageBinariesRefusesABinaryThatCannotStart` |
| Record and replay | `TestReplayGivesTheLabVerdict`, `TestLocalPrefStripReplaysTheLabRecording` |
| Red walk | `TestRedwalkCheckUnitRoute`, `test/ui/le-test-redwalk.ci` |
| Attribution | `TestAttributionClassifiesEachFailingTest` |
| Stale notice | `test/runner/stale-runner-notice.ci` |
| No layering | `grep -n "func writeOverlay" internal/le/rfc/*.go` finds nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A break patch applies only to tracked files inside the checkout; a path escaping it is refused |
| Secrets | Recordings carry environment names only; a recording is reviewed before it is committed |
| Shared state | No tracked file, Docker tag or other session's scratch is written |

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

- The brief said functional `.ci` runs execute fixtures from the committed le build. Source shows this holds only for the direct `le test <suite>` route: `./le test functional` builds le from the tree (`buildCommands`). The fix is narrowed to the direct route.
- The brief framed offline checker tests as new. The predicate split and `TestBespokeCheckerBranches` already cover five checkers; the gap is real inputs, and checkers without the split. Record/replay closes both without editing a checker.
- Named-test runs already exist through `./le job run`; the missing piece is attribution, not a launcher.
- A new le action must keep its Summary and every Help line within le's 60-character start check (`checkRegistrations`), or every le process built from it panics at start. A scoped unit run of the new package does not catch it; only the `internal/le` root tests (`go test ./internal/le/`) or starting a session le do, so run one of them before landing a new action.
- A non-harness `le` step in a `.ci` runs at the checkout root, so it cannot read a `tmpfs=` file by its bare name; three drafts in this spec hit it (`docs/architecture/testing/ci-format.md`).

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Record and replay at `CheckerLab` | Rewrite every checker into body plus pure predicate; replay only the relay jsonl | The interface exists and is documented as implementable without Docker; no checker edit; covers checkers that read daemon CLI output, not only relay ones |
| Start probe on `LabBinary` in preflight | A canary scenario before every run | The probe costs seconds and catches the start class exactly; a canary costs a scenario per run and is D1's question for setup classes the probe cannot see |
| Move the overlay code to `internal/le/test/overlay` | Add a non-RFC mode to `rfc record`; copy the code into redwalk | `rfc record` writes RFC records and should not answer untagged walks; a copy is layering |
| Attribution by overlay to HEAD on named files | Compare against a run at HEAD in a worktree; use the changed-set selector | Testing in a worktree or tree copy is banned; the selector cannot separate sessions' edits |
| Phase after the elapsed time on the FAIL line | A new line; a prefix before the scenario name | Keeps both recorder markers and the whole-name scenario match unchanged |
| Test shape judged by the skills at the moment a test is planned, written, grown, reviewed and run, with the criteria in one rule point | A lint that counts test lines or assertions; a periodic clean-up spec | Length is not the defect, mixing questions is, and no count tells a whole-path `.ci` from a bundle of unrelated checks. Catching it while the test is being written costs nothing; a clean-up pass is the deferred work this goal exists to avoid |
| Cost ranked by share of the run, never gated | An absolute time budget per test that refuses a commit | A red never holds a commit (`ai/META-DECISIONS.md`); a share within one run on one host ranks candidates without claiming a performance figure |
| Review finds only on tests the diff adds or grows | Flag every long test the diff's package holds | A review judges its own change; the cost report surfaces old long tests to whoever runs the suite |

## Known Limitations
- Load-sensitive tests: owned by `plan/post-release/engineering/spec-test-verdicts-independent-of-load.md`.
- The inventory proof's cost: owned by `plan/post-release/engineering/spec-inventory-fast-registry-checks.md`.
- Replay judges checker logic only; Ze's behaviour is still proven by its producer's unit tests and one lab pass.

## Open Decisions for Thomas
Implementation started 2026-11-10 under the recommendation of each row; Thomas may override any of them. Thomas authorised claiming this spec above the WIP cap (2026-11-10, "Ignore the cap and do them"): a one-off permission for this spec, not a cap raise.

| ID | Question | Options | Recommendation | Decided |
|----|----------|---------|----------------|---------|
| D1 | Setup failures repeated across scenarios (the four-in-a-row case, for causes the start probe does not catch) | (a) run a declared canary scenario before every multi-scenario run, about one scenario's cost each time; (b) stop the run after N consecutive scenarios fail in setup or readiness, listing the rest as not run; (c) phase label only | (b) with N=3: free on a healthy lab, bounded on a broken one | (b), N=3, on implementation start under the recommendation; Thomas may override |
| D2 | Red walks of non-Go files (`.ci`, scenario configs) and of tests that read source at run time, which a Go overlay cannot reach | (a) in scope here: the walk copies the named files into a private root and points the run there (needs the runner to take that root, and every source-reading test to honour a root variable); (b) refuse here with the reason, and home the private-root route in its own spec | (b): the private root touches the functional runner and every source-scanning guard, a separate mechanism | (b), on implementation start under the recommendation; Thomas may override |
| D3 | Whether a checker change must carry a replay test when its scenario has a recording | (a) a rule in `ai/rules/testing.md` checked at review; (b) a tool only, documented in `interop.md` | (a): it is the step that would have made `7081cfdfa7` a one-run fix | (a), on implementation start under the recommendation; Thomas may override |

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
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
