# Verification Freshness and Scope

Verification is a statement about one commit and the files the run read. The native verifier records that statement as a certificate and a per-path manifest, then commit preparation asks whether the prospective commit still matches it.

## Certificate

`internal/le/verify/engine.WriteCertificate` writes `tmp/ze-verify.status` and its manifest atomically after a native verification run. The certificate records the mode, commit, time, result, and the tree hash captured for the run. The manifest records each path at the content the stages read.

The engine writes both under the root it ran in. `./le verify worktree` runs in a throwaway worktree, so the pair lands THERE, and the lifecycle publishes it to the checkout that asked for the run before the worktree is removed (`CopyCertificate`, called from `run` in `internal/le/verify/lifecycle.go`). The copy is byte for byte: the certificate describes the commit and the clean tree the worktree held, and re-deriving it at the checkout would snapshot that checkout's dirty tree against the worktree's start and record every path as moved during the run. A cut run publishes its piece, spelled the way the engine spelled it, so the checkout never holds a piece that reads as a whole verification. A run the engine refused before its first stage wrote no certificate, and the lifecycle reports that absence rather than charging it: the engine's own failure already names the cause. Until 2026-09-16 nothing published the pair, so `./le verify status check` at the checkout kept answering from the last in-place run (`plan/journal/unwired-feature.md`). The failure index (`tmp/ze-verify-failures.json` and `.log`, plus `tmp/ze-verify-full.json` for a whole run) follows the same way, after the red-run log save (`PublishFailureIndex`, `internal/le/verify/engine/artifacts.go`). On a red run every `run-dir`, `combined-log` and `detail-log` path in it is moved from `tmp/verify/` to the saved copy under `tmp/verify-worktree-logs/<name>/`, so each one opens; a green run saves no logs and its paths stay as written. Commit preparation charges structural reds from this file (`structuralGateReds`, `internal/le/commit/verification.go`), and until 2026-10-21 no worktree run published it: the 2026-10-18 run went red in lint and the gate read nothing. A run with no index to publish is reported; one whose publication fails is defeated, as a failed certificate copy is.

What the published certificate answers at the checkout follows from the two questions below. The scoped question, which is the one a commit asks, is FRESH for every path the verified commit holds unchanged. The whole-tree question is FRESH only while the checkout is clean at that commit, because an uncommitted edit or an untracked file was judged by no stage.

`./le verify status check` calls `internal/le/verify/engine.CheckCertificate`. With no paths it compares the whole checkout, and any change to it, `HEAD` included, returns STALE. Repeated `path <path>` selectors restrict the answer to a prospective commit's files. A missing manifest, an unreadable one, a scoped path whose content differs from what the stages read, and a scoped path that moved while the run was in progress each return STALE.

Four properties of the narrower question matter to a caller:

- A path that MOVED while the run was in flight is STALE whatever it holds now, because no stage judged the content it holds today. The manifest records that path as `movedDuringRun` rather than voiding the whole run, so this is finer granularity and never leniency.
- The scoped question is about CONTENT, and `scopedChange` asks it that way because the manifest is HEAD-relative while `HEAD` moves under a checkout many sessions commit to. A scoped path the run read as dirty stops being dirty the moment a commit absorbs it unchanged, so each manifest row is compared against the file on disk. A scoped path with no row was identical to the certificate's commit, so the commits made since are asked whether any of them reached it. A commit that reaches nothing in the path list therefore leaves the answer FRESH, which is the same reading the property below gives an uncommitted edit outside the list.
- `CheckCertificate` reads the run's recorded exit code BEFORE it reads any scope, so a run that FAILED is STALE for every path list. Scoping is no route around a red run.
- A certificate whose mode names no stage population is STALE, whatever it holds. A run cut into pieces records the piece it ran (`full-part-2-of-6`), and `StagesForMode` knows no such population, so one piece of a verification can never read as a pass over the tree.
- The answer is qualified by mode, and "full" names the STAGE POPULATION rather than a promise that every stage judged everything: a full run narrows its Staticcheck rows by the change set's feature tags and, once a suite map exists, narrows its functional suites by what they were observed to reach. Both narrow only from recorded evidence and widen on anything unanswerable. `FRESH(full)` therefore covers every stage; `FRESH(fast)` is a weaker pass with no vet evidence, no cached full unit pass, and no allocation benchmarks. Lint is NOT among the omissions: `fastStages` keeps `verify lint/run` at its full identity, and the lint action reads no change scope, so both modes load the whole module. `go patch-stamps` is not among them either: `fastStages` drops only those three whole-tree stages, so both modes check every standard-library and vendor patch's `Ze-Reviewed-Against` stamp against the toolchain and `vendor/modules.txt` it is applied to. A pass recorded with skipped suites (`ZE_SKIP_SUITES`) reports STALE. Only the full mode writes `tmp/ze-verify-full.json`, so a cheaper run cannot certify a Go-carrying commit.

`verificationState` (`internal/le/commit/verification.go`) is what asks the scoped question on a prospective commit's behalf: it passes the commit's own explicit path list, so an edit another session makes outside that list does not make the evidence STALE.

<!-- source: internal/le/verify/engine/status.go -- WriteCertificate, CopyCertificate, CheckCertificate, scopedChange, movedDuringRun -->
<!-- source: internal/le/verify/engine/artifacts.go -- PublishFailureIndex -->
<!-- source: internal/le/verify/engine/part.go -- Part, deal, Name -->
<!-- source: internal/le/job/treehash.go -- Fingerprint, PathsChangedBetween -->
<!-- source: internal/le/verify/engine/stages.go -- fastStages -->
<!-- source: internal/le/verify/status/answer.go -- Answer -->
<!-- source: internal/le/commit/verification.go -- verificationState -->

## One change-set selection

`./le repo changed packages` derives the package scope from the current change set, including the packages that consume changed non-Go inputs. `./le repo changed group-packages` groups the Go-only changes for the race pass. Both widen without a proven baseline; with one, an empty answer is permitted only when the relevant change set selects no package.

`verify deps/unit-race-changed` sizes its race pass from the same freshness boundary: the working tree plus Go files committed since the last green commit. `Selector.Select` uses the shared `greenBaseline` reader, and honors a published `./...` widening for its checkout. Without a readable green commit, the race population is `./...`, even when the latest commit changes only documentation and the working tree is clean. A green baseline with no intervening Go edit remains a legitimate SKIP: the stage executes no test, exits 0, and records `skipped` with a reason. A directory that `go list` cannot resolve is a REFUSAL rather than a covered skip; `Selection.Unresolved` names the missing population.

The full population runs the race pass as three stages, `verify deps/unit-race-changed/part/<i>/of/3` (`raceParts`, `internal/le/verify/engine/stages.go`), because with no green baseline the pass races the whole tree and the undivided stage outran the 120-minute job bound of `.github/workflows/verify.yml`. A piece lists its changed patterns with `go list -e` under the race command's tag set and race constraint (`Toolchain.GoList`), and races every package whose FNV-1a import-path hash modulo the piece count is its own (`dealRacePackages`, `racePieceOf`). Each piece derives the selection at its own moment, so the deal depends on the path alone: a package every piece saw is raced exactly once even when another session changed the tree between two pieces. A green certificate another session writes between two pieces can narrow a later piece's selection, and the packages hashed to that piece then go unraced in this run; that is accepted, because they are unchanged since a commit that certificate proved green. The `-e` keeps a package that fails to load in the list, so `go test` reports it as a FAIL rather than every piece refusing. Piece 1 alone runs the bare `ze_core` check, and runs it even when the deal gives piece 1 no package. Any other piece the deal leaves empty records `skipped` with its reason, and a list that fails is a refusal with its exit code. `./le verify deps unit-race-changed` with no keywords is part 1 of 1, the whole population; `part` without `of` is refused, and only this verb accepts a cut.
<!-- source: internal/le/verify/deps/verifydeps.go -- dealRacePackages, raceCommandCount, skipEmptyRacePiece -->
<!-- source: internal/le/verify/engine/stages.go -- raceParts, raceStages -->

<!-- source: internal/le/verify/deps/verifydeps.go -- runUnitRaceChanged, skipUnitRaceChanged -->
<!-- source: internal/le/repo/changed/changed.go -- Select -->
<!-- source: internal/le/repo/changed/selector.go -- greenBaseline -->

Both unit stages execute their feature-tag population and the bare `ze_core` compile-out population even when the first test command fails. The first non-zero exit code remains the stage verdict; a later pass cannot erase it, and an earlier red cannot hide the second population.

<!-- source: internal/le/verify/deps/verifydeps.go -- runUnitCached, runUnitRaceChanged -->
<!-- source: internal/le/repo/changed/changed.go -- Selection, unresolvedDirs -->

The verify runner resolves the selection once and publishes its package and feature-tag answers to the run's artifact directory. `publishChangeScope` writes `scope-packages.txt` and `scope-tags.txt` beside the run's logs and names each one in `ZE_VERIFY_SCOPE_PACKAGES` and `ZE_VERIFY_SCOPE_TAGS`. `le go staticcheck check` reads the tag answer, and the functional stage reads the package answer: `selectSuites` intersects it with the recorded suite map to decide which suites run. `scope-packages.txt` opens with a `root <path>` line naming the checkout the answer was selected for, and a reader asking about a different checkout selects its own change set instead of taking this one. Every child process of a verify run inherits the two variable names, `go test` among them, so without that line a unit test driving the selector over a fixture directory was handed the run's own package list. This keeps the unit pass and the staticcheck matrix on the same snapshot, and it avoids a second reverse-import walk after another session changes the checkout. A run that cannot select publishes neither name, and unset is the widest reading of both: the stage selects its own packages and the matrix judges every row.

<!-- source: internal/le/verify/engine/scope.go -- publishChangeScope -->

Tests overriding a published scope must remove every equivalent case and
separator spelling of that exact key before assigning the fixture value and
resetting the environment cache. A dotted spelling wins over a nondotted one
regardless of inherited order, even when the dotted value is empty. Cleanup
restores the original environment before resetting the cache again.
<!-- source: internal/core/env/env.go -- ensureCache -->
<!-- source: internal/le/repo/changed/baseline_test.go -- TestRaceSelectionHonorsPublishedFullScope -->

`internal/le/go/staticcheck.Answer` retains the all-features and core-only rows, plus the feature-omission rows the selected tags can affect. A negated build constraint counts as a use of the tag because that file compiles in the omission row.

The retained rows are then cut across six stages, `check part 1 of 6` through `check part 6 of 6`. The scope decides WHICH rows a run judges; the cut decides WHICH STAGE judges each of them. `Matrix.Part` deals the rows round robin, so a scoped run of three rows puts one row in each of three pieces and the other three pieces report that they were dealt none.

`Scope.resolveSelector` (`internal/le/repo/changed/selector.go`) produces the package and feature-tag scope. `Selector.Select` (`internal/le/repo/changed/changed.go`) produces the Go-only race groups; both use `greenBaseline` to decide what is proven. The grouped race selection keeps its narrower Go-only classification after a green baseline, rather than racing tooling groups for every documentation edit. A published whole-tree scope widens it too. The scoped import graph is built with `ze_core` and every tag in `feature-gates.txt`, so a `//go:build ze_<feature>` importer is selected: one file under `internal/component/ssh` selects `./cmd/ze`, `./cmd/ze/hub`, `./internal/appliance` and `./internal/component/ssh`, and the feature answer is `ze_ssh` alone. The graph carries test imports, which is how `./internal/appliance` arrives: its `appliance_test` package blank-imports `cmd/ze/hub` to link every kernelcap enrolment. That import sits in `kernelreq_enrolled_linux_test.go`, and the graph is built for the host's GOOS, so a Darwin host selects the other three packages and not `./internal/appliance`. The reverse walk stops at two levels of importers, and `./le repo changed scope drop-log FILE` names what the bound dropped.

The graph query selects package identity, direct and test imports, and error
fields from `go list -e` JSON. Package errors, incomplete records, malformed
JSON, absent directories, and deadlines retain their cause and widen to
`./...` plus every feature. A partial graph is never published. The query runs
under the patched GOROOT that `gotoolchain.New` activates for the checkout le
runs in, the tree le's root handler already selected, so a refused activation
(a stale review stamp, a patch that no longer applies) widens the same way and
names its cause. It disables persisted Go settings, ambient `GOFLAGS`, and
enclosing workspaces. Selected JSON fields avoid unrelated
VCS/build-info and embed-file scans without dropping import edges.
<!-- source: internal/le/repo/changed/selector.go -- loadPackageGraphContext, parsePackageGraph, resolveSelector -->
<!-- source: internal/le/go/toolchain/gotoolchain.go -- New -->

### What each changed path selects

<!-- source: internal/le/repo/changed/selector.go -- nonGoPathRules, packageDirsFor, uncompiledTreeReaders -->

| The change set holds | The scoped stages get |
|----------------------|-----------------------|
| a `.go` file | its package, plus every importer within two levels, with the feature tags on |
| `go.mod`, `go.sum`, or a `vendor/` path | `./...`, and the widening names the path: a dependency moved, so every package that compiles against it is reachable |
| Markdown under `ai/`, `plan/`, or `docs/`, the RFC corpus, `.github/*.yml`, or `.claude/settings*.json` | the native Go packages whose tests read that kind, never the whole tree |
| a `.ci`, `.et`, or `.wb` body under `test/` | the Go packages that walk that corpus. `.ci` selects `internal/test/runner`, `internal/le/doc/yangcontract` and `internal/le`; `.et` selects `internal/component/cli/testing`; `.wb` selects `internal/component/web/testing` |
| a path under `examples/plugin/go`, matched BEFORE the `.go` rule | no package. It is a separate module, so `go list ./...` never reports it. Ordering is load-bearing: the `.go` rule would seed a directory no package owns and widen the whole run |
| a path under `gokrazy/modcache/` | no package. A third-party module cache every tree walker names in a skip list |
| a `.go` file the unit tag set never compiles, in the module root | `internal/le`, whose tree-walking tests read it (`treeWalkingPackages`). `./...` does not compile it either, so widening would buy nothing |
| a `.go` file under `cmd/ze-installer` | `./...`. `internal/le/go/lint/matrix.go` lints that package under a `ze_installer` flavor only when the lint runs over `./...`, so the wide answer is the only one that reports on an edit to the initrd's PID 1 |
| a kind no rule names | the package it sits in when that directory holds Go source, the tooling packages otherwise. The path is NAMED on stderr, which is the evidence for writing it a rule |
| any change set, and `tmp/ze-verify.status` holds no green commit | `./...`, for both package scope and grouped race selection, and the widening names the condition. Without a proven commit, every commit in history is unverified; a clean tree or docs-only closure must not select nothing |

<!-- source: internal/le/repo/changed/actions.go -- Answer -->
<!-- source: internal/le/go/staticcheck/actions.go -- Answer -->
<!-- source: internal/le/verify/engine/run.go -- RunMode, RunPart -->

## The suite map

The change-set selection above answers which PACKAGES a change reaches. The suite map answers the other half: which packages each functional suite REACHED when it last ran. A gating run consults it before it builds anything, so the denominator every progress line reads and the suites the loop starts are one decision.

The map is a derived artifact at `tmp/ze-suite-map.json`, rewritten by a recording run and never committed. It lives beside the other verification artifacts rather than in a session scratch directory, because the run that records it and the run that reads it are two sessions. `suiteMap` (`internal/le/test/functional/suitemap.go`) holds two fields. `head` is the commit the recording ran at, which a reader needs to ask which files moved since. `reached` names, for each suite, every package that suite reached, spelled the way the change-set selector spells one (`./internal/component/ssh`), so neither side normalizes the other.

**Every route that cannot answer WIDENS to every suite.** The file is under `tmp/`, which several sessions share, so a malformed map must widen and must never narrow. `readSuiteMap` refuses rather than answering thinly, and the caller's response to each refusal is the same widening:

| What the reader meets | Why it is a refusal |
|-----------------------|---------------------|
| No file at the path | The ordinary state of a fresh checkout and of every CI shard |
| A read error | A directory or a half-written file at the path is not a map that records nothing |
| JSON that does not parse | A truncated write is not a narrower answer |
| No `head` | A map with no commit cannot be asked what moved since, so nothing it records is answerable |
| No suite under `reached` | A recording that produced no suite broke |
| A suite whose recorded set is EMPTY | The recording broke for that suite. Reading it as "this suite covers nothing" would skip that suite for ever |

Zero suites selected is a valid answer, because a docs-only change reaches none. Zero packages under one suite is not.

A caller cannot mistake a widening for an empty selection. `suiteSelection` carries a `suiteVerdict` whose zero value is `verdictUnspecified`, so a selection nobody filled in is neither answer, and `runs` reports true under every verdict except `verdictSelected`. A widening therefore names no suite and subtracts none.

`ZE_SKIP_SUITES` outranks the map: `gatingRunList` reads the operator's skip set first, so a recorded map can only ever subtract a suite and never add a skipped one back. The closing report names the operator's skips.

### How a package becomes answerable

`selectSuites` needs three answers before it can rule one suite out, and a route that cannot produce one of them widens.

**A package is ANSWERABLE only when the map records a suite reaching it AND no commit since the recording touched it.** One unanswerable package widens the whole run and is NAMED, because the map cannot say which suites a change to that package could break.

| The condition | What the run does |
|---------------|-------------------|
| The map records a suite reaching the package, and no commit has touched it | The suites recorded as reaching it run |
| The map records no suite reaching the package | Every suite runs, and the answer names the package |
| A commit since the map's `head` touched the package | Every suite runs, and the answer names the package and the commit |
| The map's `head` cannot be compared with `HEAD` | Every suite runs. `job.PathsChangedBetween` answers an error rather than an empty list, so a commit a rebase dropped never reads as "no package moved" |
| The change-set selector refused the checkout, or widened to `./...` | Every suite runs |
| This run records the map (`ZE_COVER`) | Every suite runs. Only a run of every gating suite may publish, so a recording run that narrowed would refuse its own map and the artifact could never be refreshed |

**A gating suite the map does not name is UNKNOWN, and every unknown suite runs.** A suite that recorded nothing is omitted rather than written empty, so nothing rules it out: `editor`, `web`, `runner` and `policy` are in that state on every run and always run. A change set holding no package at all is a valid narrow answer, and those four still run.

The touched-package test is deliberately per-package rather than per-tree. The stale-map risk this bounds is a suite that newly reaches a package, and a run that widened on any commit at all would widen on every commit this shared checkout takes, which is the same as having no map.

`packageOf` (`internal/le/test/functional/suitemap.go`) spells a touched path the way the change-set selector spells a package, so the two sides compare directly. Any file counts, not only a `.go` one: a package whose testdata moved is a package whose recorded reach was observed on another tree.

### Reading the run list before the run

`le test functional select` prints the run list a gating run would start for this checkout, and runs nothing. It is what an operator reads when a run started fewer suites than they expected, and it is the run's own decision rather than a second derivation of one: `planRun` (`internal/le/test/functional/suitemap.go`) produces the plan, `le test functional select` prints it, and `runGating` executes it.

The answer places every gating suite in exactly one of three states, so a suite cannot go missing unnoticed.

| State | Why the suite is there |
|-------|------------------------|
| `running` | The map records it as reaching a changed package, or the map does not name it at all |
| `ruled-out` | The map records it, and it reached none of the changed packages |
| `skipped` | `ZE_SKIP_SUITES` names it. It outranks the map |

<!-- source: internal/le/test/functional/actions.go -- selectVerb -->

### What a suite REACHED

A gating run under `ZE_COVER=1` records the map. The subjects are built `-cover`, each suite runs with its own `GOCOVERDIR`, and `reduceCoverage` (`internal/le/test/functional/run.go`) reduces that directory once the suite ends.

**A package is REACHED when the suite covered one block that is neither in `register.go` nor inside a `func init()` body.** Every other covered block is what any process runs on any start, because Ze registers its components by running each package's `init()`. Counting every covered block answers "which packages does this binary link", and the spec measured both definitions over the same profiles: the three-suite intersection is 443 packages of 646 counting executions and 126 counting reaches, and `ze show version` alone counts 435 against 115. `packagesInProfile` (`internal/le/test/functional/reach.go`) reads the `go tool covdata textfmt` profile and asks `go/ast` for the line range of every `func init()` a covered file declares.

Every uncertainty in the reduction WIDENS. A file that will not parse contributes its blocks, so its package reads as reached and the suite runs on a change to it. A profile row the reduction cannot read is a refusal rather than a skipped line, because a silently shorter set is a map that narrows more. A suite that FAILED records what it reached before it failed, and a recorded set is therefore a lower bound: a package the run missed is one the map cannot answer for, so the suite runs on a change to it.

The raw coverage directory is removed as soon as it is reduced, and the text profile it reduces to is a temporary the reduction removes when it has read it. One suite's profile exists at a time, which is what bounds the disk an instrumented run costs: the largest measured is 13.7 MB.

### What the writer refuses

| What the run holds | What it publishes |
|--------------------|-------------------|
| Every gating suite reduced, some with packages | The map, naming the suites that recorded something |
| A suite that recorded nothing | Nothing for that suite: it is OMITTED, so the next reader knows nothing about it and runs it |
| A gating suite this run did not run | No map at all. A single-suite run and a run under `ZE_SKIP_SUITES` are both partial |
| No suite recorded anything | No map. `validate` refuses a map that records no suite |
| A commit Git could not name | No map. A map with no commit can be asked nothing about what moved since |

**An omitted suite and an empty recorded set are different answers, and only one of them is writable.** Omitted means "this run learned nothing about that suite", which widens. An empty set read back would mean "this suite covers nothing", which would skip that suite for ever, and `readSuiteMap` refuses one for that reason. `editor`, `web`, `runner` and `policy` record nothing on every run: the first three run the harness rather than an instrumented `ze`, and `policy` skips its tests unprivileged. They are omitted every time, and they always run.

**Only a whole gating run may write the artifact**, because a suite the map does not name always runs while a suite it does name can be ruled out. A map written by a run that covered one suite would declare the other 27 unknown and narrow on the one. `publish` (`internal/le/test/functional/suitemap.go`) loops over `Gating` rather than over what the run recorded, so a partial run is refused rather than trusted.

The recorded `head` is the commit read BEFORE the first suite starts, which is the tree the binaries were compiled from. A full run takes an hour on a checkout several sessions share, so the commit can move under it. Reading it at the end would claim the map describes a tree no suite ran; naming the earlier commit is the conservative direction, because a reader treats every package touched since as unknown and widens. `job.Head` answers `unknown` on a tree Git cannot describe, and that publishes no map.

The map is published through a temporary file in the same directory and one rename, so a session reading `tmp/` meets the old map or the new one and never a half-written one.

<!-- source: internal/le/test/functional/suitemap.go -- suiteMap, readSuiteMap, suiteSelection, selectSuites, suitesFor, touchedSince, planRun, gatingRunList, suiteRecording, publish -->
<!-- source: internal/le/test/functional/reach.go -- reachedPackages, packagesInProfile, initLineRanges -->
<!-- source: internal/le/test/functional/run.go -- runGating, reduceCoverage, publishSuiteMap -->

## Native stage execution

`internal/le/verify/engine.RunMode` executes the ordered stage population and captures each stage result. A red stage does not hide later reds, unless the run was asked for `first-red` (below). An interrupt ends the run with 130, whether it lands between stages or during one, and whether the run's context saw it or only the stage process did (a SIGINT or SIGTERM straight to the child); no later stage starts. The stage is marked interrupted, with no verdict, only when the interrupt cut it short: the dispatcher's cancel signal reached the running process (`cancelStage`, `internal/le/verify/dispatch/dispatch.go`; a signal that finds the process already reaped answers `os.ErrProcessDone` and leaves the stage's exit as its verdict), the child died of SIGINT or SIGTERM, or the stage exited 130 itself after the interrupt landed, as a child that handles SIGINT does. Any other exit answered before the interrupt stays a verdict. The action runner returns a populated `ActionResult`, so an omitted registration cannot look like exit zero, and when the runner refuses a stage with a reason of its own (`killed`, `interrupted`, `spawn`) that reason is the report's failure rather than a generic one.

`internal/le/verify/dispatch.RunAction` refuses a command le does not own or has no handler for, then runs the stage as its own le process: on Linux the running image itself (`/proc/self/exe`), so a `bin/le` replaced mid-run by `./le --update` cannot put different code under later stages, and on Darwin, which has no such handle, the binary's path. The child is started under the name this process was, in the requested root with `ze.repo.root` set to it. One pipe takes both of the child's streams, and what it carries is the stage log. A handler that writes its reason straight to `os.Stderr`, a helper or library that does, and a program the stage starts all land in the log, so a failing stage cannot leave a log with no cause in it, and the log reads as what an operator who typed the `Command:` line would have seen.

Every stage log ends with two lines the engine writes (`runMode`, `internal/le/verify/engine/run.go`): `### Stage timing: <name> start=<RFC3339 UTC> seconds=<decimal>`, then `### Stage result: <name> exit=N`. The result line stays last and keeps its spelling, because `stageResult` (`internal/le/verify/reds.go`) reads the last result marker and three fixtures write it. The same start and seconds are on each stage of the report and of the failure index (`start`, `seconds`).
<!-- source: internal/le/verify/engine/run.go -- runMode, StageReport -->

A stage whose process died of a Go panic or fatal error is a red stage. The stage's own Go runtime says so: dispatch passes it an empty file as descriptor 3, a before-dispatch hook in the child hands that descriptor to `debug.SetCrashOutput`, and only a crash writes to it, so a stage that merely relays another program's panic text and answers 2 stays unjudged. A crashed stage is refused as `crashed` with status 1, never the "could not judge" its raw status reads as, and the stages after it still run. A canceled stage is sent SIGTERM so it can release its job slot, port lock, temporary directory or namespace and stop its own children, and is killed 10 seconds later if it has not exited. It stays in verify's process group, so the stall breaker's group kill and a terminal's Ctrl-C reach it too. A stage process that dies of any other signal is refused as `killed`, which breaks the run with 2. Once the stage exits, the log waits at most 10 seconds for a process the stage left running to close the output it inherited, and says so in the log when it stops waiting, whatever the stage's status.

<!-- source: internal/le/verify/dispatch/dispatch.go -- RunAction, dispatch, stageExecutable, runStage, crashReported -->
<!-- source: internal/le/verify/dispatch/register_crashoutput.go -- armStageCrashOutput -->
<!-- source: internal/le/verify/engine/run.go -- runMode, validateResult, redStage -->

Native `./le` actions are the public interface, while Go-to-Go paths call their package functions. Heavy verification admits ITSELF: `verify current` and `verify worktree` claim the `verify` label in the shared job registry before they do any work, so a second verification of the same tree takes the running one's verdict and a third queues. A run that holds a slot names its registry entry to every stage it starts, which is how the `verify lint/run` stage admits without waiting for the slot its own parent holds, and it copies each finished stage to the slot's log, which is the growth the stall breaker reads as liveness and the output a following session replays. `./le job run label <label> command <argv...>` and the `verify lock` action admit anything else heavy.

`verify current` takes one of two bare words. `full`, which is also the bare form, runs the full population through the worktree lifecycle `verify worktree` runs (`runWorktree`), at HEAD, so the full population never runs over the working tree. `fast` runs the changed-files population in place (`runFast`), with the certificate mode `fast`. Naming both is refused, and so is any other word, with every spelling the action takes. `verify list full` and `verify list fast` print each population; CI shards `verify list full`.
<!-- source: internal/le/verify/current.go -- currentHere, runFast, population -->

A run has four outcomes, not three. 0 certifies the tree, 1 says a stage judged it and found it wrong, 2 says the run itself broke, and 3 (`Unjudged`) says the run reached no verdict at all. A stage answers 2 when it could not judge its own subject, which `le go staticcheck check` does for an empty package population, and `runCode` carries that outcome up as `Unjudged` instead of flattening it to 1. A stage that judged the tree and found it wrong outranks it, because that stage did reach a verdict. A full device is the second route to `Unjudged`, recognized by `Defeated` with `errors.Is(err, syscall.ENOSPC)` at each write site that holds the typed error: no stage output reaches a `Report`, so text matching cannot see it at all. `CheckCertificate` stales on any non-zero exit, so an unjudged run can never read as fresh.

A run can ATTACH. A second run of the same work over the same tree takes the holder's exit code rather than judging twice, and the certificate on disk is the one the holder wrote. Such a report SAYS so: `Report.Attached` names the holder's registry entry and its progress log, because the run judged nothing and therefore carries no stages and no log directory of its own. Without that field a zero code reads as a green nobody earned, which is the reading `admissionFailure` refuses to publish for the other path that reaches no verdict.

<!-- source: internal/le/verify/current.go -- attachedReport -->

A run can be ORDERED BY COST, and it can STOP AT THE FIRST RED. Every run is handed a `Plan` (`internal/le/verify/engine/stagetimes.go`): `History` names the checkout whose `tmp/ze-verify-stage-times.tsv` the run reads its order from and appends one line per reached stage to (stage, seconds, exit, mode, commit, start), and `FirstRed` asks it to stop. Both entry points pass the checkout, so a worktree run's times outlive the worktree. An uncut run with a history runs every stage with no recorded time first, in declared order, then the rest by ascending latest seconds, ties in declared order (`planOrder`, `costOrder`); a cut run keeps the dealt declared order, and `verify list` prints the declared order whatever the history says, because CI deals its shards from it. With `FirstRed`, no stage starts after the first non-zero stage: the report's `StoppedAt` names it and `Unreached` lists every stage not started, the combined log ends its stages with `### Run stopped at first red: <stage>, <n> stages not run` (`StoppedMarker`), and the failure index carries `stopped-at` and one `reached: false` entry per unreached stage. A red at the last stage stops nothing, so that run reports as one that went to its end: no `StoppedAt`, no stop line, `Completed` true. The run's exit is the stopping stage's, so `CheckCertificate` answers STALE for every path list. A first-red run is separate work: `worktreeArgv` and `fastArgv` carry the keyword, so it never attaches to a run to the end of the same commit, nor that run to it. `first-red` with `part` is refused before any work (`firstRedWithPart`), because a piece is dealt for debt clearing, which needs every stage of the piece.

<!-- source: internal/le/verify/engine/stagetimes.go -- Plan, ReadStageCosts, costOrder, AppendStageTimes -->
<!-- source: internal/le/verify/engine/run.go -- planOrder, recordStageTimes, StoppedMarker -->

A run can be CUT. `internal/le/verify/engine.RunPart` runs one piece of the mode's population, dealt round robin by `Part.deal`, which is the shape `staticcheck-feature-matrix check part <n> of <m>` already uses and for the same reason: the expensive stages sit together in the ordered population. Every stage lands in exactly one piece, so the pieces together run the population exactly once, and a piece that names no part of it (the zero value, an index past the count) is refused with `unknown-part` before a stage starts. The pieces of one commit are separate work in the job registry: `worktreeArgv` carries the piece, so a second piece cannot attach to the first and take its exit code.

The lifecycle prints its verdict from the first deferred call, which makes it the last line of the run. Every branch that can still move `Report.Code`, the deferred cleanup included, has run by then. `verify worktree` also links the extracted worktree's `cache/` to the shared per-user target before any stage starts, so GOCACHE resolves out of tree and the run does not build a private Go build cache it will delete unread. It also links into the worktree's `gokrazy/modcache` every entry of the checkout's that the worktree lacks, because the appliance build resolves modules offline and git carries only the tracked gokrazy source. Third, it links the worktree's `tmp/golangci-lint-cache` to the checkout's (`sharedLintCacheLink`): golangci-lint keys its cache on the module-relative path, so the checkout's cache serves any worktree, and the removal deletes only the link. Each link that cannot be made is printed and the run continues cold. The Go build cache is reused across worktrees only because every le-driven Go command runs with `-trimpath` (`gotoolchain.TrimpathGoFlags`); without it each compile action ID carries the worktree's absolute directory. Tests that open or stat files inside the module still miss the Go test cache in a fresh worktree, because the test cache hashes those files' absolute names and modification times. A test that reads a variable through `os.Environ` (every `env.Get` does) is not keyed on it, so the test environment of two runs of one commit must not differ: every cache path a stage hands its test children is named where the worktree's link lands (`gen.Landing`), and a test child inherits neither the checkout root nor the change scope. `docs/contributing/running-commands.md`, after the `-trimpath` paragraph, names what is pinned, the one entry that is not (`ze.run.job`), and the tests that prove it.

Above that verdict the lifecycle names every stage that exited non-zero, one line each, with the stage's own log file. The verdict states an exit code, and the stage behind it is otherwise a line inside a log of tens of thousands: a debt sweep printed `full-part-5-of-6 exit=1` and the reader opened a 49K log to reach `doc check/verify exit=1` (`plan/journal/failing-gate-prints-no-cause.md`). `./le verify worktree all of <m>` prints those lines too, because the sweep carries each piece's diagnostics into its own answer.

<!-- source: internal/le/verify/engine/run.go -- ActionResult, RunMode, RunPart, Slot, nameJobParent, Unjudged, Defeated -->
<!-- source: internal/le/verify/current.go -- runFast, jobLabel, slotFor -->
<!-- source: internal/le/verify/lifecycle.go -- run, sharedCacheLink, sharedLintCacheLink, worktreeArgv -->
<!-- source: internal/le/verify/modcache.go -- sharedModcacheLink, linkModcache -->
<!-- source: internal/le/job/answer.go -- Answer -->

## Failure attribution

A gate knows which files caused its failure. `internal/le/doc/wiring` publishes that fact as a JSON failure group at the point where the failure is decided. A group carries a kind, a summary, a rerun command, and zero or more related paths. Paths are JSON-encoded, so a crafted filename cannot forge a second group.

The commit package reads those groups for the prospective commit's explicit paths. A path-bearing group is foreign only when every related path lies outside that commit. A population-wide group, a malformed group, or a failure with no path remains charged. The default is therefore fail closed.

`structuralGateReds` (`internal/le/commit/verification.go`) reports four sets. `charged` refuses the commit; `foreign` names each gate the file list ruled out; `unattributed` names each group that carries a check name, a suite name, or the stage's own name; `unreached` names each structural stage a first-red run held and stopped before (`reached: false` with no `outside-run`), which is also charged, because the run set out to judge it and did not finish (AC-11). Every index names the whole full population (`carryForward`, `internal/le/verify/engine/artifacts.go`): a stage the run did not hold (a piece of a cut run, the fast population, a run that broke) takes the verdict the checkout's earlier index recorded for it, marked `outside-run` with `carried-from` naming the commit and run that judged it, so a carried red is charged as a red and the refusal names `red at <sha> in <run>`. Without it, a piece or a fast run that replaced a full run's index erased that run's lint red. A stage no run judged is `outside-run` and not reached, and is not charged: not running a stage is owed by the derived verification debt until a full verify judges it, and charging it refused every commit after a green fast run or a green cut sweep. A stage an interrupt cut short is not held either, because its exit is what its process died with: `RunMode` marks it `Interrupted` on its `StageReport`, and `writeRunArtifacts` hands it to `carryForward` as not held, so it takes the earlier verdict for the stage, or, with none, is written `interrupted: true`, not reached, with no exit, no group and its own `rerun`. `judged` refuses to carry such an entry as a verdict, and `structuralGateReds` lists it in `Unjudged` rather than `Charged`; `verificationState` appends each one's cause (`unjudgedCause`: never judged, why, and the command that judges it) to the stale verify detail the commit report prints. An index written before the marker existed records the run's interrupt status (`Interrupted`, 130) as the stage's exit, and both `judged` and the gate read that the same way: a killed stage once blocked every commit until a full verify reached it again. A first-red stop's own unreached stages are the exception, and they stay charged until a later run judges them or supersedes them: once a run that did not hold such a stage replaces the index, the stage is `outside-run` with no verdict, so it is owed as not run by the unverified-debt row and is no longer charged as red. `writeFailureIndex` does the read, merge and write under a lock on `tmp/ze-verify-failures.lock`, and treats a missing or malformed earlier index as empty, saying so in `carry-note`; an earlier index that recorded no commit is named `an earlier run (commit not recorded)` there, in the text index and in the refusal. It writes the JSON index first, because that is what the gate reads. `PublishFailureIndex`, which carries a worktree run's index into the checkout, always writes the current format (`FailureIndexFormat`): a worktree index with no format has each of its exit-130 entries rewritten as interrupted first, so a current-format exit-130 red carried from the checkout's index is never read back as an interrupt. The gate reads a missing index as empty, but an index or a stage entry it cannot read or parse holds no verdict, charges nothing, and is reported in `Unreadable`, which `verificationState` appends to the stale verify detail with the fix, `le verify current`. A lock or JSON write that fails says the earlier index was left unchanged and this run's reds were not published, and names the fix: correct what refused the write, then rerun the verify (`unpublished`). A text or full-mode copy that fails after it says the JSON index holds this run and the copy still shows the earlier one (`copyBehind`). Each refusal line names one charged stage, its cause, the command that diagnoses it and the runs that clear it (`redCause`, `clearingRuns`). The two differ: the diagnosing command is the stage's own rerun from its group, which runs the stage directly and publishes no failure index, so a stage fixed and rerun green that way stays charged. Only a verify run that reaches the stage rewrites its entry, so the clearing line names `le verify worktree`, the `le verify worktree part <i> of <n>` piece that holds the stage, and `le verify current fast` when the fast population (read from `StagesForMode(FastMode)`) holds it. An index with no `reached` key predates first-red and reads every stage as run. A green verify rewrites the artifact, so a fixed-and-reverified gate clears automatically.

| What the red gate's failure groups name | What commit preparation does |
|---|---|
| Files, and one of them is in the commit's file list | Charges the gate and refuses |
| Files, and every one lies outside that list | Drops the charge and prints the gate name |
| A check name, a suite name, or the stage itself | Charges the gate and names it as unattributed |

A test run names no file: `go test` prints the failing PACKAGE, and an assertion under it prints a base name with no directory. `verify deps` therefore declares the `package` kind, whose related list holds checkout-relative directories rather than files. `Covers` reads a red about a directory as belonging to whoever changed a file inside it, so the attribution is exact without guessing which file held the assertion.

<!-- source: internal/le/verify/failuregroup/failuregroup.go -- Packages, the go test scanner -->
<!-- source: internal/le/verify/deps/verifydeps.go -- failingPackages, Report.Text -->

Which gates the file list can rule out follows from what each one declares:

| Gate | What its groups name | Expect |
|------|----------------------|--------|
| `./le go lint run`, `./le repo changed scope` | the `.go` file each finding sits in | a drop when none of them is in the file list |
| `ze-evidence-vet` | the package pattern of each red | a drop when the list holds no file under it |
| `./le doc wiring` | the files each sub-check is about, one declared group per failure (`declareFailureGroup`, `internal/le/doc/wiring/groups.go`) | a drop, except for the ci-sleep ratchet and a delegated target, which name no file |
| Every other stage, `./le repo generated-check`, `./le doc check links` and `./le test weakened check` among them | the stage's own name, through the `generic` fallback group in `writeRunArtifacts` (`internal/le/verify/engine/artifacts.go`) | a charge, always |

The declared-group protocol is available to EVERY stage, not only `./le doc wiring`: a stage that emits its own groups is read back by `declaredGroups` (`internal/le/verify/engine/artifacts.go`), and the generic fallback applies only when it emits none. Attribution also answers a NARROWER question than the ledger asks: it says the files this commit carries cannot have caused the red. It never says the red is somebody else's work rather than the author's from an earlier session.

### Structural stages

A stage declares whether its red means the tree is BROKEN, on the stage itself: `structural(...)` rather than `stage(...)` in the mode's population, read back through `verifyengine.Structural` (`internal/le/verify/engine/stages.go`). There is no separate list to keep in agreement, and a rename moves the name and its membership together. `TestStructuralStagesAreMembersOfThePopulation` and `TestStructuralIsASubsetOfFull` (`internal/le/verify/engine/stages_structural_test.go`) hold that the set is non-empty in both modes, names only stages that run, and never marks a stage structural in the cheaper mode alone.

### The green record and the cut

`tmp/ze-verify-green.status` records the last green, full, unskipped worktree verification of one commit. It is not the certificate: every run overwrites `tmp/ze-verify.status`, red or green, so the last green verification is gone from the certificate the moment a later run fails, while a red run leaves the green record alone. Its fields are `commit`, `tree` (`<commit>^{tree}`, the git tree id a push target is compared with), `mode` (always `full`), `skipped` (always empty), `cut` (`uncut`, or the piece count `m`) and `timestamp`. `ReadGreenRecord` (`internal/le/verify/green.go`) answers an absent record as `os.ErrNotExist` and a record that proves nothing (another mode, a skipped suite, a malformed field) as `ErrNotGreen`.

Two writers write it, both at the checkout that asked and both through `writeGreen` (`internal/le/verify/green.go`). `recordGreen`, called from `run` in `internal/le/verify/lifecycle.go`, writes it after a finished run, once the certificate is published. `Sweep` writes it from a parts record that holds every piece of the commit and cut, each piece recorded only after its run passed `recordGreen`'s guards (the sweep is below). `writeGreen` applies no guard of its own and relies on its caller to have established them. `recordGreen`'s guards judge a verdict that is the run's OWN: `Report.Verify` is present and not attached to another run's verdict, the exit is 0, every stage of the population completed, the mode is the full population for the run's own cut, and `ZE_SKIP_SUITES` is empty. The published certificate is read for one fact only, whether the tree moved during the run. When its `git_sha` is not the commit this run verified, another run rewrote it between the publish and the read, and nothing is written; the run prints `verify-worktree: green record not written: certificate names <sha>, this run verified <sha>`. `./le verify status write` writes the certificate alone and never the green record, because a declared exit code proves no run. Neither record is written inside the worktree, so `CopyCertificate` copies only the certificate and its manifest.

`./le verify worktree part <i> of <m>` runs one piece of the population dealt into `m`. A green piece adds `i` to `tmp/ze-verify-parts.json`, pinned to the commit and to `m` under an advisory lock, and either one moving starts the record again: pieces of two commits describe two trees, and two cuts deal the stages differently. The piece that completes 1..m writes the green record with `cut` = `m`, then takes the lock and drops the parts record, only while that record is still pinned to that commit and that `m`. The green write itself is not under the lock, so a piece can pin the record between the two: a record pinned since to another commit, or to another cut of the same commit by any piece or sweep, holds that work's progress and stays. A record that cannot be read is dropped. `part` without `of`, `of` without `part`, a piece number that is not a positive whole number and `i > m` are refused with exit 2 before any work.

`./le verify worktree all of <m>` is the sweep (`Sweep`, `internal/le/verify/sweep.go`). It resolves `commit <revision>` (default HEAD) once, so every piece judges the same SHA even when HEAD moves during it. It skips the pieces the parts record holds for that commit and `m`, and runs nothing when the green record already names the commit. When the parts record already holds every piece for the commit and `m` but no green record names it, the green write after the last piece failed: the sweep runs no piece and writes the green record from the parts record, dropping that record as a completing piece does, because `recordPart` records a piece only after every guard the green writer applies (a completed full run, no skipped suite, a certificate naming the commit, an unmoved tree) held for that piece's run. It runs the host preflight for every piece it will run before the first starts, so a host that cannot run the last piece starts none. A red piece does not end the sweep, and the exit is the first red piece's. When every piece passed but no green record was written, the sweep exits 1 and its verdict line says so, because a zero exit would read as a covered push that nothing records. `all` with `part`, `all` without `of`, and `all` with `first-red` are refused naming `all of <m>`.
<!-- source: internal/le/verify/green.go -- recordGreen, writeGreen, recordPart, forgetParts, ReadGreenRecord -->
<!-- source: internal/le/verify/sweep.go -- Sweep -->
<!-- source: internal/le/verify/actions.go -- worktreeFrom -->

### Verification debt, derived

Nothing records verification debt. `deriveDebt` (`internal/le/commit/owed.go`) derives it on every read from git and the green record above, and the push verb, `debt-status`, the `create` report and the session-start hook all read that one answer. An answer it cannot give (no upstream, a git call that failed, a green record it cannot read) is an error naming its fix, never zero owed. Outside the Ze checkout it answers not-applicable and derives nothing.

The session-start hook prints one line, `verification debt: <what is owed>, push refused until cleared`, and at most five items under it (the commits left behind, the reasons the push is refused, then the open person acts; a longer list ends with `... and N more: ./le commit debt-status`). What is owed is the first of these that holds: `N commit(s) owe a full verify since <G>` when commits are left behind; `HEAD's tree is not covered by a green verify (no green record): run ./le verify worktree`, or `(the green record names <G>)`, when the push target is not HEAD, even with no commit listed and no refusal; `the push of HEAD is refused`; `N person act(s) owed`. It never prints a count of zero. It prints nothing only when nothing is owed: the push target is HEAD, so the green record covers HEAD's tree, no commit is left behind, nothing refuses the push and no person act is open. That is one predicate, `owed` (`internal/le/commit/owed.go`), which `debt-status` and the `create` report read too, so no surface says nothing is owed while another refuses. When the derivation cannot answer it says so on stderr with the fix, never as zero owed, except in a linked worktree, where a branch with no upstream is the normal state of an agent that never pushes.
<!-- source: internal/le/commit/push.go -- DebtWarning, debtReport, answerDebtStatus, headline -->
<!-- source: internal/le/commit/owed.go -- owed -->
<!-- source: internal/le/hookruntime/lifecycle.go -- reportDerivedDebt, linkedWorktree -->

The derivation chooses the push target P, the commit a push publishes:

| Condition, in order | P |
|---------------------|---|
| the green tree covers `HEAD^{tree}` | HEAD |
| the green record's commit G is an ancestor of HEAD | the last commit of the walk from G |
| otherwise (no record, or G left the branch) | none: a full verify is owed |

The green tree COVERS a tree when every path `git diff --no-renames --name-only <green tree> <tree>` names is prose, and an empty diff is covered. Prose is declared once, on the `ai/`, `plan/` and `docs/` Markdown entries of `nonGoPathRules`, and read through `IsProse` (`internal/le/repo/changed/selector.go`); the commit package holds no path list of its own. A root-level page, the RFC corpus, a test body and any non-Markdown file under those trees are not prose. A diff git cannot compute is NOT covered and the refusal names git's error: it is never read as an empty diff. Renames are off, so a file moved out of a prose tree names its old path too.

The walk starts at G and follows `git rev-list --reverse --first-parent G..HEAD`, advancing while the green tree covers each commit and stopping at the first it does not. G is judged too, so a record whose tree does not cover its own commit proves nothing. Prose-only commits after G are therefore published with it, and the first commit carrying anything else ends the walk. `debt-status commit <rev>` judges `<rev>` alone: admissible when the green tree covers it and it is on HEAD's branch. Whatever P is, the upstream must fast-forward to it, or the answer names `git pull --rebase --no-autostash` then `./le verify worktree`.

`Published` is `P ^@{u}`. `LeftBehind` is `HEAD ^P`, or `HEAD ^@{u} ^G` with no P (`HEAD ^@{u}` when G is not in the repository), and lists only the commits that change a non-prose path. `pushRefusal` refuses on any refusal, on an empty `Published` ("nothing verified is ahead of the upstream"), and on any open person act.

### Person acts, read from trailers

A review or an owner approval is an act no verification performs, so it is carried by the commit that owes it, as a trailer `create` writes when it grants the admission (`docs/contributing/committing.md`). `readPersonActs` (`internal/le/commit/owed_trailers.go`) reads every trailer over `P ^@{u}` (`HEAD ^@{u}` with no P) in one `git log`, and keeps each obligation no answer holds. Answers are read over the same range, so an attestation committed after P does not count for a push of P. A malformed declared trailer is a refusal, never skipped.

| Answer | `Review-owed: <stem>: <reason>` | `RFC-approval-owed: <package>.<Test>: <reason>` |
|--------|---------------------------------|-------------------------------------------------|
| not applicable | the commit closes no spec (`closedSpecStem`) | the commit does not change that unit (`changedTaggedUnits`) |
| closed | the removed spec, read at the commit's parent, records its Review Gate, or its Status is design or skeleton (`answerClosed`) | - |
| reviewed | a clean `tmp/review/<stem>-*.md` artifact whose hashes match the commit's own blobs (`answerReviewed`) | - |
| approved | - | an `RFC-approved:` trailer for the same unit in range |
| owner | an `Owner-attested:` trailer in range naming the commit's stable patch-id | the same |

The producers live in `internal/le/commit/owed_answers.go`. `tmp/review/` is NOT the durable record of a review: it is untracked and is emptied. The Review Gate is committed prose and closure removes the spec, so `git show <closure-sha>^:<spec-path>` recovers it for ever, which is where `closed` reads it. A missing Review Gate has two meanings and they never share a branch: on a `skeleton` or `design` spec there was nothing to review, and on an implemented one the review is missing. The removed spec's own Status at the parent separates them.

The owner answer is keyed by `git patch-id --stable` (`stablePatchIDs`), which a rebase keeps unless conflict resolution changes the diff, and then the commit is owed again. A `Ledger-carried: <sha> <patch-id> <gate key>: <reason>` trailer gates the same way: it is matched to the commit in range with that patch-id and answered over it, and with no match it stays open under the carried SHA until the owner attests the carrying commit. A carried row with no patch-id names a commit already published and is audit only.

<!-- source: internal/le/commit/owed.go -- deriveDebt, pushRefusal -->
<!-- source: internal/le/commit/owed_trailers.go -- readPersonActs, stablePatchIDs -->
<!-- source: internal/le/repo/changed/selector.go -- IsProse -->

### Asking before the run ends

`DeclaredGroups` (`internal/le/verify/engine/artifacts.go`) reads one stage's declaration back out of that stage's own log, and `writeRunArtifacts` calls it once per red stage when the run ends. `./le verify reds file <path>` calls it too, over whatever logs the run has written so far, so an agent gets a per-path answer without starting or waiting for a whole-tree run (`docs/contributing/running-commands.md`).

That query is not a certificate and MUST NOT be read as one. It answers about PATHS, it states how many stages have not reported, and only a whole run with every red attributed elsewhere exits 0. `structuralGateReds` still reads the PUBLISHED index and therefore sees nothing at all until the run ends: the freshness certificate is what keeps that empty answer from reading as a pass, and `TestStructuralRedsSeeNothingUntilARunPublishesItsIndex` (`internal/le/commit/commit_test.go`) pins both halves.

<!-- source: internal/le/doc/wiring/groups.go -- Group, declareFailureGroup -->
<!-- source: internal/le/verify/green.go -- recordPart, partsComplete -->
<!-- source: internal/le/commit/verification.go -- structuralGateReds -->
<!-- source: internal/le/verify/engine/artifacts.go -- writeRunArtifacts, DeclaredGroups -->
<!-- source: internal/le/verify/reds.go -- readReds, verdictOf -->
<!-- source: internal/le/verify/engine/stages.go -- Structural -->

## Producer contract

A new verification gate returns structured data from its owning `internal/le` package. If the gate can attribute a failure, it emits the group at the failure point; collecting groups at the end loses failures from early returns. The rerun field uses an exact `./le <area> <action>` invocation.

A new scoped input kind joins the `changed` package's native path table and names the packages that read it. A rule that cannot prove a narrow package set widens. It must never select nothing.
