# Verification Freshness and Scope

Verification is a statement about one commit and the files the run read. The native verifier records that statement as a certificate and a per-path manifest, then commit preparation asks whether the prospective commit still matches it.

## Certificate

`internal/le/verify/engine.WriteCertificate` writes `tmp/ze-verify.status` and its manifest atomically after a native verification run. The certificate records the mode, commit, time, result, and the tree hash captured for the run. The manifest records each path at the content the stages read.

`./le verify status check` calls `internal/le/verify/engine.CheckCertificate`. With no paths it compares the whole checkout, and any change to it, `HEAD` included, returns STALE. Repeated `path <path>` selectors restrict the answer to a prospective commit's files. A missing manifest, an unreadable one, a scoped path whose content differs from what the stages read, and a scoped path that moved while the run was in progress each return STALE.

Four properties of the narrower question matter to a caller:

- A path that MOVED while the run was in flight is STALE whatever it holds now, because no stage judged the content it holds today. The manifest records that path as `movedDuringRun` rather than voiding the whole run, so this is finer granularity and never leniency.
- The scoped question is about CONTENT, and `scopedChange` asks it that way because the manifest is HEAD-relative while `HEAD` moves under a checkout many sessions commit to. A scoped path the run read as dirty stops being dirty the moment a commit absorbs it unchanged, so each manifest row is compared against the file on disk. A scoped path with no row was identical to the certificate's commit, so the commits made since are asked whether any of them reached it. A commit that reaches nothing in the path list therefore leaves the answer FRESH, which is the same reading the property below gives an uncommitted edit outside the list.
- `CheckCertificate` reads the run's recorded exit code BEFORE it reads any scope, so a run that FAILED is STALE for every path list. Scoping is no route around a red run.
- The answer is qualified by mode. `FRESH(full)` covers everything; `FRESH(changed)` is a weaker pass with no vet evidence, no cached full unit pass, and no allocation benchmarks. Lint is NOT among the omissions: `changedStages` keeps `verify lint/run` at its full identity, and the lint action reads no change scope, so both modes load the whole module. A pass recorded with skipped suites (`ZE_SKIP_SUITES`) reports STALE. Only the full mode writes `tmp/ze-verify-full.json`, so a cheaper run cannot certify a Go-carrying commit.

`verificationState` (`internal/le/commit/verification.go`) is what asks the scoped question on a prospective commit's behalf: it passes the commit's own explicit path list, so an edit another session makes outside that list does not make the evidence STALE.

<!-- source: internal/le/verify/engine/status.go -- WriteCertificate, CheckCertificate, scopedChange, movedDuringRun -->
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
<!-- source: internal/le/verify/engine/run.go -- Run, RunMode -->

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

Today `selectSuites` answers `verdictEverySuite` on every path. It reads and validates the map, and no change set is compared against it, so a recorded map subtracts no suite and an absent one costs nothing. Nothing writes the artifact yet.

<!-- source: internal/le/functional/suitemap.go -- suiteMap, readSuiteMap, suiteSelection, selectSuites, gatingRunList -->
<!-- source: internal/le/functional/run.go -- runGating -->

## Native stage execution

`internal/le/verify/engine.RunMode` executes the ordered stage population and captures each stage result. A red stage does not hide later reds; cancellation stops before another stage starts. Each in-process action runner returns a populated `ActionResult`, so an omitted registration cannot look like exit zero.

Native `./le` actions are the public interface, while Go-to-Go paths call their package functions. Heavy verification admits ITSELF: `verify current` and `verify worktree` claim the `verify` label in the shared job registry before they do any work, so a second verification of the same tree takes the running one's verdict and a third queues. A run that holds a slot names its registry entry to every stage it starts, which is how the `verify lint/run` stage admits without waiting for the slot its own parent holds, and it copies each finished stage to the slot's log, which is the growth the stall breaker reads as liveness and the output a following session replays. `./le job run label <label> command <argv...>` and the `verify lock` action admit anything else heavy.

A run has four outcomes, not three. 0 certifies the tree, 1 says a stage judged it and found it wrong, 2 says the run itself broke, and 3 (`Unjudged`) says the run reached no verdict at all. A stage answers 2 when it could not judge its own subject, which `le staticcheck-feature-matrix check` does for an empty package population, and `runCode` carries that outcome up as `Unjudged` instead of flattening it to 1. A stage that judged the tree and found it wrong outranks it, because that stage did reach a verdict. A full device is the second route to `Unjudged`, recognized by `Defeated` with `errors.Is(err, syscall.ENOSPC)` at each write site that holds the typed error: no stage output reaches a `Report`, so text matching cannot see it at all. `CheckCertificate` stales on any non-zero exit, so an unjudged run can never read as fresh.

The lifecycle prints its verdict from the first deferred call, which makes it the last line of the run. Every branch that can still move `Report.Code`, the deferred cleanup included, has run by then. `verify worktree` also links the extracted worktree's `cache/` to the shared per-user target before any stage starts, so GOCACHE resolves out of tree and the run does not build a private Go build cache it will delete unread.

<!-- source: internal/le/verify/engine/run.go -- ActionResult, RunMode, Slot, nameJobParent, Unjudged, Defeated -->
<!-- source: internal/le/verify/current.go -- runCurrent, jobLabel, slotFor -->
<!-- source: internal/le/verify/lifecycle.go -- run, sharedCacheLink -->
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

`clearDebt` (`internal/le/commit/actions.go`) re-runs each DISTINCT gate the open rows name, once per pass whatever the row count, and writes `cleared` only on exit 0. Every runnable gate runs inside ONE throwaway worktree at HEAD, so a cleared row says the gate was green over the COMMIT rather than over the several sessions' uncommitted files this checkout holds. When no worktree can be made, NOTHING clears and the pass exits 1: that is a refusal to fall back to the working tree, not a gate failure. A pass whose every row names an unrunnable gate materialises no worktree at all.

### Asking before the run ends

`DeclaredGroups` (`internal/le/verify/engine/artifacts.go`) reads one stage's declaration back out of that stage's own log, and `writeRunArtifacts` calls it once per red stage when the run ends. `./le verify reds file <path>` calls it too, over whatever logs the run has written so far, so an agent gets a per-path answer without starting or waiting for a whole-tree run (`docs/contributing/running-commands.md`).

That query is not a certificate and MUST NOT be read as one. It answers about PATHS, it states how many stages have not reported, and only a whole run with every red attributed elsewhere exits 0. `structuralGateReds` still reads the PUBLISHED index and therefore sees nothing at all until the run ends: the freshness certificate is what keeps that empty answer from reading as a pass, and `TestStructuralRedsSeeNothingUntilARunPublishesItsIndex` (`internal/le/commit/commit_test.go`) pins both halves.

The pass clears no row whose gate no command produces. A row naming `independent critical review` prints UNRUNNABLE and stays open, and so does a row naming a gate string the runner table does not hold. Those are answered by doing the work the row names, which for a review is `/ze-review` recorded through `internal/le/spec/session/review.go`.

<!-- source: internal/le/doc/wiring/groups.go -- Group, declareFailureGroup -->
<!-- source: internal/le/commit/actions.go -- Answer, clearDebt -->
<!-- source: internal/le/commit/verification.go -- structuralGateReds -->
<!-- source: internal/le/verify/engine/artifacts.go -- writeRunArtifacts, DeclaredGroups -->
<!-- source: internal/le/verify/reds.go -- readReds, verdictOf -->
<!-- source: internal/le/verify/engine/stages.go -- Structural -->
<!-- source: internal/le/commit/debt.go -- Debt, ListDebt -->

## Producer contract

A new verification gate returns structured data from its owning `internal/le` package. If the gate can attribute a failure, it emits the group at the failure point; collecting groups at the end loses failures from early returns. The rerun field uses an exact `./le <area> <action>` invocation.

A new scoped input kind joins the `changed` package's native path table and names the packages that read it. A rule that cannot prove a narrow package set widens. It must never select nothing.
