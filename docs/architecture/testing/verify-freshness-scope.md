# Verification Freshness and Scope

Verification is a statement about one commit and the files the run read. The native verifier records that statement as a certificate and a per-path manifest, then commit preparation asks whether the prospective commit still matches it.

## Certificate

`internal/le/verify/engine.WriteCertificate` writes `tmp/ze-verify.status` and its manifest atomically after a native verification run. The certificate records the mode, commit, time, result, and the tree hash captured for the run. The manifest records each path at the content the stages read.

`./le verify status check` calls `internal/le/verify/engine.CheckCertificate`. With no paths it compares the whole checkout, and any change to it, `HEAD` included, returns STALE. Repeated `path <path>` selectors restrict the answer to a prospective commit's files. A missing manifest, an unreadable one, a scoped path whose content differs from what the stages read, and a scoped path that moved while the run was in progress each return STALE.

Four properties of the narrower question matter to a caller:

- A path that MOVED while the run was in flight is STALE whatever it holds now, because no stage judged the content it holds today. The manifest records that path as `movedDuringRun` rather than voiding the whole run, so this is finer granularity and never leniency.
- The scoped question is about CONTENT, and `scopedChange` asks it that way because the manifest is HEAD-relative while `HEAD` moves under a checkout many sessions commit to. A scoped path the run read as dirty stops being dirty the moment a commit absorbs it unchanged, so each manifest row is compared against the file on disk. A scoped path with no row was identical to the certificate's commit, so the commits made since are asked whether any of them reached it. A commit that reaches nothing in the path list therefore leaves the answer FRESH, which is the same reading the property below gives an uncommitted edit outside the list.
- `CheckCertificate` reads the run's recorded exit code BEFORE it reads any scope, so a run that FAILED is STALE for every path list. Scoping is no route around a red run.
- A certificate whose mode names no stage population is STALE, whatever it holds. A run cut into pieces records the piece it ran (`full-part-2-of-6`), and `StagesForMode` knows no such population, so one piece of a verification can never read as a pass over the tree.
- The answer is qualified by mode, and "full" names the STAGE POPULATION rather than a promise that every stage judged everything: a full run narrows its Staticcheck rows by the change set's feature tags and, once a suite map exists, narrows its functional suites by what they were observed to reach. Both narrow only from recorded evidence and widen on anything unanswerable. `FRESH(full)` therefore covers every stage; `FRESH(changed)` is a weaker pass with no vet evidence, no cached full unit pass, and no allocation benchmarks. Lint is NOT among the omissions: `changedStages` keeps `verify lint/run` at its full identity, and the lint action reads no change scope, so both modes load the whole module. A pass recorded with skipped suites (`ZE_SKIP_SUITES`) reports STALE. Only the full mode writes `tmp/ze-verify-full.json`, so a cheaper run cannot certify a Go-carrying commit.

`verificationState` (`internal/le/commit/verification.go`) is what asks the scoped question on a prospective commit's behalf: it passes the commit's own explicit path list, so an edit another session makes outside that list does not make the evidence STALE.

<!-- source: internal/le/verify/engine/status.go -- WriteCertificate, CheckCertificate, scopedChange, movedDuringRun -->
<!-- source: internal/le/verify/engine/part.go -- Part, deal, Name -->
<!-- source: internal/le/job/treehash.go -- Fingerprint, PathsChangedBetween -->
<!-- source: internal/le/verify/engine/stages.go -- changedStages -->
<!-- source: internal/le/verify/status/answer.go -- Answer -->
<!-- source: internal/le/commit/verification.go -- verificationState -->

## One change-set selection

`./le changed packages` and `./le changed group-packages` derive the package scope from the current change set. Non-Go inputs seed the packages that consume them, and every unresolved case widens to `./...`. An empty answer is never used as a successful narrow selection.

`verify deps/unit-race-changed` sizes its race pass from that selection, and an empty selection is two different answers rather than one. A change set holding no Go file is a SKIP: the stage runs no test, exits 0, and its report carries `skipped` with the reason, because an exit code cannot tell that run from one that raced every changed group. A change set whose every directory the toolchain calls no package is a REFUSAL: Go files changed, `Selection.Unresolved` names each directory `go list` dropped, and the stage exits non-zero instead of certifying a change it never tested. A dropped directory beside a selection that still holds packages is named on stderr and on the report, so a partial population does not read as the whole one.

<!-- source: internal/le/verify/deps/verifydeps.go -- runUnitRaceChanged, skipUnitRaceChanged -->
<!-- source: internal/le/changed/changed.go -- Selection, unresolvedDirs -->

The verify runner resolves the selection once and publishes its package and feature-tag answers to the run's artifact directory. `publishChangeScope` writes `scope-packages.txt` and `scope-tags.txt` beside the run's logs and names each one in `ZE_VERIFY_SCOPE_PACKAGES` and `ZE_VERIFY_SCOPE_TAGS`. `le staticcheck-feature-matrix check` reads the tag answer; no stage reads the package answer today, so `scope-packages.txt` is written and never consulted. This keeps the unit pass and the staticcheck matrix on the same snapshot, and it avoids a second reverse-import walk after another session changes the checkout. A run that cannot select publishes neither name, and unset is the widest reading of both: the stage selects its own packages and the matrix judges every row.

<!-- source: internal/le/verify/engine/scope.go -- publishChangeScope -->

`internal/le/staticcheckfeaturematrix.Answer` retains the all-features and core-only rows, plus the feature-omission rows the selected tags can affect. A negated build constraint counts as a use of the tag because that file compiles in the omission row.

The retained rows are then cut across six stages, `check part 1 of 6` through `check part 6 of 6`. The scope decides WHICH rows a run judges; the cut decides WHICH STAGE judges each of them. `Matrix.Part` deals the rows round robin, so a scoped run of three rows puts one row in each of three pieces and the other three pieces report that they were dealt none.

One producer answers the change set: `Scope.resolveSelector` (`internal/le/changed/selector.go`). `internal/le/changed/changed.go` holds no selection logic and only dispatches between the two routes to it. A direct `./le verify lint run` or `./le test-unit all` outside a verify run has no published answer, so it selects its own (2.4 to 2.9s measured). Both routes reach the same producer. The import graph is built with `ze_core` and every tag in `feature-gates.txt`, so a `//go:build ze_<feature>` importer is selected: one file under `internal/component/ssh` selects `./cmd/ze`, `./cmd/ze/hub` and `./internal/component/ssh`, and the feature answer is `ze_ssh` alone. The reverse walk stops at two levels of importers, and `./le changed scope drop-log FILE` records which packages that bound dropped.

### What each changed path selects

<!-- source: internal/le/changed/selector.go -- nonGoPathRules, packageDirsFor, uncompiledTreeReaders -->

| The change set holds | The scoped stages get |
|----------------------|-----------------------|
| a `.go` file | its package, plus every importer within two levels, with the feature tags on |
| `go.mod`, `go.sum`, or a `vendor/` path | `./...`, and the widening names the path: a dependency moved, so every package that compiles against it is reachable |
| Markdown under `ai/`, `plan/`, or `docs/`, the RFC corpus, `.github/*.yml`, or `.claude/settings*.json` | the native Go packages whose tests read that kind, never the whole tree |
| a `.ci`, `.et`, or `.wb` body under `test/` | the Go packages that walk that corpus. `.ci` selects `internal/test/runner`, `internal/le/docvalid` and `internal/le`; `.et` selects `internal/component/cli/testing`; `.wb` selects `internal/component/web/testing` |
| a path under `examples/plugin/go`, matched BEFORE the `.go` rule | no package. It is a separate module, so `go list ./...` never reports it. Ordering is load-bearing: the `.go` rule would seed a directory no package owns and widen the whole run |
| a path under `gokrazy/modcache/` | no package. A third-party module cache every tree walker names in a skip list |
| a `.go` file the unit tag set never compiles, in the module root | `internal/le`, whose tree-walking tests read it (`treeWalkingPackages`). `./...` does not compile it either, so widening would buy nothing |
| a `.go` file under `cmd/ze-installer` | `./...`. `internal/le/verify/lint/matrix.go` lints that package under a `ze_installer` flavor only when the lint runs over `./...`, so the wide answer is the only one that reports on an edit to the initrd's PID 1 |
| a kind no rule names | the package it sits in when that directory holds Go source, the tooling packages otherwise. The path is NAMED on stderr, which is the evidence for writing it a rule |
| nothing, and `tmp/ze-verify.status` holds no green commit | `./...`, and the widening names the condition. Without a proven commit, every commit in history is unverified, so a clean tree must not select nothing |

<!-- source: internal/le/changed/actions.go -- Answer -->
<!-- source: internal/le/staticcheckfeaturematrix/actions.go -- Answer -->
<!-- source: internal/le/verify/engine/run.go -- RunMode, RunPart -->

## The suite map

The change-set selection above answers which PACKAGES a change reaches. The suite map answers the other half: which packages each functional suite REACHED when it last ran. A gating run consults it before it builds anything, so the denominator every progress line reads and the suites the loop starts are one decision.

The map is a derived artifact at `tmp/ze-suite-map.json`, rewritten by a recording run and never committed. It lives beside the other verification artifacts rather than in a session scratch directory, because the run that records it and the run that reads it are two sessions. `suiteMap` (`internal/le/functional/suitemap.go`) holds two fields. `head` is the commit the recording ran at, which a reader needs to ask which files moved since. `reached` names, for each suite, every package that suite reached, spelled the way the change-set selector spells one (`./internal/component/ssh`), so neither side normalizes the other.

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

`packageOf` (`internal/le/functional/suitemap.go`) spells a touched path the way the change-set selector spells a package, so the two sides compare directly. Any file counts, not only a `.go` one: a package whose testdata moved is a package whose recorded reach was observed on another tree.

### Reading the run list before the run

`le functional select` prints the run list a gating run would start for this checkout, and runs nothing. It is what an operator reads when a run started fewer suites than they expected, and it is the run's own decision rather than a second derivation of one: `planRun` (`internal/le/functional/suitemap.go`) produces the plan, `le functional select` prints it, and `runGating` executes it.

The answer places every gating suite in exactly one of three states, so a suite cannot go missing unnoticed.

| State | Why the suite is there |
|-------|------------------------|
| `running` | The map records it as reaching a changed package, or the map does not name it at all |
| `ruled-out` | The map records it, and it reached none of the changed packages |
| `skipped` | `ZE_SKIP_SUITES` names it. It outranks the map |

<!-- source: internal/le/functional/actions.go -- selectVerb -->

### What a suite REACHED

A gating run under `ZE_COVER=1` records the map. The subjects are built `-cover`, each suite runs with its own `GOCOVERDIR`, and `reduceCoverage` (`internal/le/functional/run.go`) reduces that directory once the suite ends.

**A package is REACHED when the suite covered one block that is neither in `register.go` nor inside a `func init()` body.** Every other covered block is what any process runs on any start, because Ze registers its components by running each package's `init()`. Counting every covered block answers "which packages does this binary link", and the spec measured both definitions over the same profiles: the three-suite intersection is 443 packages of 646 counting executions and 126 counting reaches, and `ze show version` alone counts 435 against 115. `packagesInProfile` (`internal/le/functional/reach.go`) reads the `go tool covdata textfmt` profile and asks `go/ast` for the line range of every `func init()` a covered file declares.

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

**Only a whole gating run may write the artifact**, because a suite the map does not name always runs while a suite it does name can be ruled out. A map written by a run that covered one suite would declare the other 27 unknown and narrow on the one. `publish` (`internal/le/functional/suitemap.go`) loops over `Gating` rather than over what the run recorded, so a partial run is refused rather than trusted.

The recorded `head` is the commit read BEFORE the first suite starts, which is the tree the binaries were compiled from. A full run takes an hour on a checkout several sessions share, so the commit can move under it. Reading it at the end would claim the map describes a tree no suite ran; naming the earlier commit is the conservative direction, because a reader treats every package touched since as unknown and widens. `job.Head` answers `unknown` on a tree Git cannot describe, and that publishes no map.

The map is published through a temporary file in the same directory and one rename, so a session reading `tmp/` meets the old map or the new one and never a half-written one.

<!-- source: internal/le/functional/suitemap.go -- suiteMap, readSuiteMap, suiteSelection, selectSuites, suitesFor, touchedSince, planRun, gatingRunList, suiteRecording, publish -->
<!-- source: internal/le/functional/reach.go -- reachedPackages, packagesInProfile, initLineRanges -->
<!-- source: internal/le/functional/run.go -- runGating, reduceCoverage, publishSuiteMap -->

## Native stage execution

`internal/le/verify/engine.RunMode` executes the ordered stage population and captures each stage result. A red stage does not hide later reds; cancellation stops before another stage starts. Each in-process action runner returns a populated `ActionResult`, so an omitted registration cannot look like exit zero.

Native `./le` actions are the public interface, while Go-to-Go paths call their package functions. Heavy verification admits ITSELF: `verify current` and `verify worktree` claim the `verify` label in the shared job registry before they do any work, so a second verification of the same tree takes the running one's verdict and a third queues. A run that holds a slot names its registry entry to every stage it starts, which is how the `verify lint/run` stage admits without waiting for the slot its own parent holds, and it copies each finished stage to the slot's log, which is the growth the stall breaker reads as liveness and the output a following session replays. `./le job run label <label> command <argv...>` and the `verify lock` action admit anything else heavy.

A run has four outcomes, not three. 0 certifies the tree, 1 says a stage judged it and found it wrong, 2 says the run itself broke, and 3 (`Unjudged`) says the run reached no verdict at all. A stage answers 2 when it could not judge its own subject, which `le staticcheck-feature-matrix check` does for an empty package population, and `runCode` carries that outcome up as `Unjudged` instead of flattening it to 1. A stage that judged the tree and found it wrong outranks it, because that stage did reach a verdict. A full device is the second route to `Unjudged`, recognized by `Defeated` with `errors.Is(err, syscall.ENOSPC)` at each write site that holds the typed error: no stage output reaches a `Report`, so text matching cannot see it at all. `CheckCertificate` stales on any non-zero exit, so an unjudged run can never read as fresh.

A run can be CUT. `internal/le/verify/engine.RunPart` runs one piece of the mode's population, dealt round robin by `Part.deal`, which is the shape `staticcheck-feature-matrix check part <n> of <m>` already uses and for the same reason: the expensive stages sit together in the ordered population. Every stage lands in exactly one piece, so the pieces together run the population exactly once, and a piece that names no part of it (the zero value, an index past the count) is refused with `unknown-part` before a stage starts. The pieces of one commit are separate work in the job registry: `worktreeArgv` carries the piece, so a second piece cannot attach to the first and take its exit code.

The lifecycle prints its verdict from the first deferred call, which makes it the last line of the run. Every branch that can still move `Report.Code`, the deferred cleanup included, has run by then. `verify worktree` also links the extracted worktree's `cache/` to the shared per-user target before any stage starts, so GOCACHE resolves out of tree and the run does not build a private Go build cache it will delete unread.

<!-- source: internal/le/verify/engine/run.go -- ActionResult, RunMode, RunPart, Slot, nameJobParent, Unjudged, Defeated -->
<!-- source: internal/le/verify/current.go -- runCurrent, jobLabel, slotFor -->
<!-- source: internal/le/verify/lifecycle.go -- run, sharedCacheLink, worktreeArgv -->
<!-- source: internal/le/job/answer.go -- Answer -->
<!-- source: internal/le/verify/lock/register.go -- Answer -->

## Failure attribution

A gate knows which files caused its failure. `internal/le/doc/wiring` publishes that fact as a JSON failure group at the point where the failure is decided. A group carries a kind, a summary, a rerun command, and zero or more related paths. Paths are JSON-encoded, so a crafted filename cannot forge a second group.

The commit package reads those groups for the prospective commit's explicit paths. A path-bearing group is foreign only when every related path lies outside that commit. A population-wide group, a malformed group, or a failure with no path remains charged. The default is therefore fail closed.

`structuralGateReds` (`internal/le/commit/verification.go`) reports three sets. `charged` refuses the commit; `foreign` names each gate the file list ruled out; `unattributed` names each group that carries a check name, a suite name, or the stage's own name. A green verify rewrites the artifact, so a fixed-and-reverified gate clears automatically.

| What the red gate's failure groups name | What commit preparation does |
|---|---|
| Files, and one of them is in the commit's file list | Charges the gate and refuses |
| Files, and every one lies outside that list | Drops the charge and prints the gate name |
| A check name, a suite name, or the stage itself | Charges the gate and names it as unattributed |

Which gates the file list can rule out follows from what each one declares:

| Gate | What its groups name | Expect |
|------|----------------------|--------|
| `./le verify lint run`, `./le changed scope` | the `.go` file each finding sits in | a drop when none of them is in the file list |
| `ze-evidence-vet` | the package pattern of each red | a drop when the list holds no file under it |
| `./le doc wiring` | the files each sub-check is about, one declared group per failure (`declareFailureGroup`, `internal/le/doc/wiring/groups.go`) | a drop, except for the ci-sleep ratchet and a delegated target, which name no file |
| Every other stage, `./le repository generated-check`, `./le doc check links` and `./le test-weakened check` among them | the stage's own name, through the `generic` fallback group in `writeRunArtifacts` (`internal/le/verify/engine/artifacts.go`) | a charge, always |

The declared-group protocol is available to EVERY stage, not only `./le doc wiring`: a stage that emits its own groups is read back by `declaredGroups` (`internal/le/verify/engine/artifacts.go`), and the generic fallback applies only when it emits none. Attribution also answers a NARROWER question than the ledger asks: it says the files this commit carries cannot have caused the red. It never says the red is somebody else's work rather than the author's from an earlier session.

### Structural stages

A stage declares whether its red means the tree is BROKEN, on the stage itself: `structural(...)` rather than `stage(...)` in the mode's population, read back through `verifyengine.Structural` (`internal/le/verify/engine/stages.go`). There is no separate list to keep in agreement, and a rename moves the name and its membership together. `TestStructuralStagesAreMembersOfThePopulation` and `TestStructuralIsASubsetOfFull` (`internal/le/verify/engine/stages_structural_test.go`) hold that the set is non-empty in both modes, names only stages that run, and never marks a stage structural in the cheaper mode alone.

### Verification debt

Verification debt records an authorised commit that lacked fresh evidence. It follows the commit until the owed gates pass, and open debt blocks a push. `./le commit debt-list` and `./le commit debt-clear` use the same native ledger as commit preparation.

One row holds ONE gate and ONE reason, and covers every commit the session made under that pair. `recordDebt` (`internal/le/commit/debt.go`) extends the open row it finds rather than appending a copy of it. The freshness gates state one fact about the tree, in a reason string byte-identical for every commit a long verification run overlaps: `verify worktree` pins its subject at launch and takes over an hour, so its verdict describes an ancestor and every commit made during it owed the same gate for the same reason. Appending recorded that one fact thousands of times.

The row keeps the date and the subject of the FIRST commit it covers, and its subject cell then carries `(+N more)` for the N commits that followed. Each of those commits writes the shard, so `git log -- plan/verification-debt/<session>.md` names them all and gives their dates and subjects back. A cleared row is never extended, so a gate owed again after it was cleared opens a row of its own. The shards written before this rule were collapsed once to the same shape: 3587 rows became the 1270 distinct (shard, gate, reason, status) triples they carried, over 216 shards, and no triple was lost.

`clearDebt` (`internal/le/commit/actions.go`) runs ONE verification per pass, whatever the row count, and marks EVERY runnable gate name passed on exit 0. It writes `cleared` only after that exit. Every runnable gate runs inside ONE throwaway worktree at HEAD, so a cleared row says the gate was green over the COMMIT rather than over the several sessions' uncommitted files this checkout holds. When no worktree can be made, NOTHING clears and the pass exits 1: that is a refusal to fall back to the working tree, not a gate failure. A pass whose every row names an unrunnable gate materialises no worktree at all.

`./le commit debt-clear part <n> of <m>` runs ONE piece of that verification. The whole population is 50 stages and it uses the whole machine, so on a box several sessions share it is killed before a single row is reachable: on 2026-09-07 the OOM killer ended a pass with 3302 rows open across 216 shards, and nothing cleared. A piece that exits 0 records itself in `tmp/ze-verify-debt-parts.json`, so a piece killed mid-flight costs that piece rather than the pass.

Nothing clears until EVERY piece of the cut has exited 0 over ONE commit. The record is pinned to the commit the pieces judged and to the count they were dealt into, and either one moving starts it again: a verdict is evidence about the tree it ran on, and two cuts deal the stages differently, so pieces of two commits or of two cuts never add up to a population. The record is dropped once its pieces have cleared their rows, so a row written later at the same HEAD cannot clear on a verification that ran before it existed. Reads and writes take the same advisory lock the ledger shards take.

Which gates a verification can re-run is DECLARED by `debtGates` (`internal/le/commit/debt.go`), on a `Runnable` flag beside each gate's Name. A row naming `independent critical review` or `owner approval for an RFC-tagged test change` prints UNRUNNABLE and stays open, because no command produces either: both are acts a person performs.

A gate string that is neither a declared Name nor a declared alias prints UNRECOGNIZED and is never cleared by a green verify. A name nobody declared says nothing about what ran. The legacy spellings the ledger already holds are declared as ALIASES on the gate they name, so those rows still clear, and `TestEveryLedgerGateNameIsDeclared` (`internal/le/commit/ledger_test.go`) turns a spelling nobody declared into a red test rather than a silent open row. It found the fifth one on 2026-09-08.

A red verification exits non-zero. The pass cleared nothing, so the exit code and the ledger agree.

### Discharging a row no gate can run

`./le commit debt-discharge shard <name> line <n> kind <kind> ...` records HOW an unrunnable row's obligation was met. It writes one row per debt row into `plan/verification-debt/discharged/<session>.md`, and the debt shard is not touched: the on-disk vocabulary stays `open` and `cleared`, and `discharged` exists in memory alone, produced by the overlay in `ListDebt`. That is why the push gate, the clearing verb, the status answer and the session-start hook each need no edit to follow it.

A discharge answers a gate no verification can RUN, and it answers an OPEN row. Both are read from the row itself before any kind runs, in `verifyDischarge` (`internal/le/commit/discharge.go`), so a fifth kind inherits them. Where a verification re-runs the gate, the fact a kind derives says nothing about what that gate would report, and the row clears by running it through `debt-clear`: the check is `debtGates[at].Runnable`, the same declaration `debt-clear` reads. A gate `debtGates` declares neither as a Name nor as an alias is refused too. A row already `cleared` had its gate run green, which answers more than a discharge does, so the overlay leaves it cleared and reports the record rather than reclassifying the row.

One debt row holds at most ONE record. A second discharge of that row replaces the first where it stands, and `replaceDischargeRow` (`internal/le/commit/dischargerecord.go`) drops any further row the file already held for it. An appended attempt stays on disk and re-derives on every read, so evidence the operator has already replaced prints INVALID for ever. That line is the whole tamper signal, and a permanent false one teaches a reader to skim past it.

Every record is judged BEFORE any row is overlaid, in `applyDischarges`. `discharged` is that function's own product and never an on-disk status, so each record is read against what the LEDGER says. Records live one file per session, so two sessions CAN hold a record for one row: the second is redundant, it derives, and it is not reported. Where the two disagree on KIND, the attestation stands whatever order the files were read in. `owner` is the only kind no machine re-derives, and a derivation that replaced it would move the row out of the by-kind split R-3 exists to make visible, on nothing but two file names.

Each kind answers the gates its evidence is about. `closed` and `reviewed` both assert that a REVIEW ran, so they answer `independent critical review` alone: an owner's approval of an RFC-tagged test change is an act no reviewer performs. `not-applicable` re-runs the producer the row's own gate names. `owner` is an attestation and answers any unrunnable gate.

One row covers every commit its session made under the same gate and reason, so `commit <sha>` REPEATS and the row discharges only when every commit it covers is named. `debtCovered` gives the count from the `(+N more)` suffix, and no commit is named twice.

Each named commit is then bound to the row by three conditions together, in `dischargeCommits`. It MUST write the row's ledger shard, which every commit a row covers does. It MUST have WRITTEN this row rather than another row of the same shard, because writing the shard says only that the commit belongs to the session and every commit of that session writes it. And at least one of the named commits MUST carry the row's subject, which the row keeps from the first commit it covers. A subject shorter than 12 characters binds by EQUALITY rather than containment: the ledger holds whole subject cells of `test` and `probe`, and `test` is contained in 1353 of this repository's 8387 commit subjects.

"Wrote this row" has two arms and either one answers it. The commit ADDED a ledger row carrying this row's reason cell, read from `git show <sha> -- <shard>`, or the commit appears in the history of the row's OWN line, read with `git log -L<line>,<line>:<shard>`. The line history was the whole condition until 2026-09-08 and it cannot reach three of this ledger's rows: before the 2026-09-07 dedup a session's second commit under one gate and one reason wrote a row of its OWN, that pass merged each such pair and DELETED the second row's line, and no line history reaches a line that was deleted. The reason cell is the dedup's own merge key, so it is byte-identical across the merged pair and it recovers the second commit.

The arms run in cost order, and the line history is read only for a commit the reason arm left unbound. One commit's diff of one shard is a single object read; the line history is a walk of the whole commit graph filtered to that path, measured at 0.36 s against 0.008 s. `debt-status` over 67 records fell from 23.4 s to 5.3 s, which is what a read on the session-start hook and on every `commit create` costs each session in this checkout.

What that still admits is stated on `dischargeCommits` itself. The reason arm admits a commit that added a DIFFERENT row of this shard whose reason cell happens to be byte-identical, and two rows of one shard under one gate and one reason are the same obligation by the merge key's own definition. Both arms admit a commit that REWROTE the shard wholesale, the 2026-09-07 dedup among them, because it adds every line and is in the history of every line, so it can stand in for one of a multi-commit row's commits. Telling a rewrite from an extension needs a diff analysis the code does not do. A row whose `(+N more)` counts more RECORDINGS than commits, which a re-run of `create` with a reworded subject leaves behind, cannot be answered this way and stays open: over-refusing is the safe direction, because the alternative discharges N commits' obligation on evidence about one.

| Kind | What it asserts | What re-derives it |
|------|-----------------|--------------------|
| `not-applicable` | the gate never bound this commit | today's `closedSpecStem` over the commit's file list for a review row, and the owner-approval gate's own tagged-unit reading for an RFC row |
| `closed` | the spec the commit CLOSED recorded a review that ran | the removed spec's bytes at the closure commit's PARENT: its Status, then its `## Review Gate` rows |
| `reviewed` | a review artifact covers the commit | the artifact's verdict and its hash for each code-bearing path AT that commit, falling back to the committed Review Gate when the artifact is gone |
| `owner` | Thomas ordered or reviewed the commit himself | nothing. It is an ATTESTATION, and `debt-status` splits the discharged count by kind so an attested population is visible beside a derived one |

The record stores the INPUT alone, the kind and its evidence, and never a verdict. Every read re-runs the derivation, so a discharge whose evidence stops holding returns its row to `open` with no file edited. The row's SHA-256 pins the discharge to the exact debt row it answers: an edited row drops its discharge, counts open again, and `debt-status` names the record as invalid.

`tmp/review/` is NOT the durable record of a review. It is untracked and is emptied, so a review recorded months ago has no artifact left. The Review Gate is committed prose and closure removes the spec, so `git show <closure-sha>^:<spec-path>` recovers it for ever. The `closed` and `reviewed` kinds read it there. Its verdict row carries two era spellings, so the predicate reads the section's ROWS: a filled artifact reference and a rounds count, with no row saying the review was not recorded or not run. Filled means the artifact cell NAMES A FILE, because `n/a` and `-` are what an author writes where no review produced one, and a cell test that only refuses the empty string reads both as evidence.

A missing Review Gate has two meanings and they never share a branch. On a `skeleton` or `design` spec it means there was nothing to review, and the discharge is accepted. On an implemented one it means the review is missing, and the discharge is refused. The removed spec's own Status at the parent is what separates them; the absence of the gate is never read as the answer.

### Asking before the run ends

`DeclaredGroups` (`internal/le/verify/engine/artifacts.go`) reads one stage's declaration back out of that stage's own log, and `writeRunArtifacts` calls it once per red stage when the run ends. `./le verify reds file <path>` calls it too, over whatever logs the run has written so far, so an agent gets a per-path answer without starting or waiting for a whole-tree run (`docs/contributing/running-commands.md`).

That query is not a certificate and MUST NOT be read as one. It answers about PATHS, it states how many stages have not reported, and only a whole run with every red attributed elsewhere exits 0. `structuralGateReds` still reads the PUBLISHED index and therefore sees nothing at all until the run ends: the freshness certificate is what keeps that empty answer from reading as a pass, and `TestStructuralRedsSeeNothingUntilARunPublishesItsIndex` (`internal/le/commit/commit_test.go`) pins both halves.

<!-- source: internal/le/doc/wiring/groups.go -- Group, declareFailureGroup -->
<!-- source: internal/le/commit/actions.go -- Answer, clearDebt, clearDebtWith -->
<!-- source: internal/le/commit/debtpart.go -- debtPartFrom, recordDebtPart, debtPartsComplete -->
<!-- source: internal/le/commit/verification.go -- structuralGateReds -->
<!-- source: internal/le/verify/engine/artifacts.go -- writeRunArtifacts, DeclaredGroups -->
<!-- source: internal/le/verify/reds.go -- readReds, verdictOf -->
<!-- source: internal/le/verify/engine/stages.go -- Structural -->
<!-- source: internal/le/commit/debt.go -- Debt, ListDebt, readDebt, recordDebt, extendDebtRow, debtGates -->
<!-- source: internal/le/commit/discharge.go -- dischargeDebt, applyDischarges, verifyDischarge, dischargeCommits, rowLineHistory, commitCarriesSubject, reviewGateRecorded -->
<!-- source: internal/le/commit/dischargerecord.go -- writeDischargeRecords, replaceDischargeRow, readDischargeRecords, parseDischargeRow -->

## Producer contract

A new verification gate returns structured data from its owning `internal/le` package. If the gate can attribute a failure, it emits the group at the failure point; collecting groups at the end loses failures from early returns. The rerun field uses an exact `./le <area> <action>` invocation.

A new scoped input kind joins the `changed` package's native path table and names the packages that read it. A rule that cannot prove a narrow package set widens. It must never select nothing.
