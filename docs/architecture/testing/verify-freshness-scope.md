# Scoped verify freshness

`make ze-precommit-verify` writes a fingerprint of the tree it judged, and the
commit helper reads that fingerprint before it writes a commit script. This page
describes what the fingerprint answers about, and what it refuses to answer
about.

## Why the answer is scoped

The fingerprint started as one number over the whole tree: `HEAD`, the whole
`git diff HEAD`, and every untracked file. Several sessions share this checkout,
and it routinely carries more than 300 uncommitted files. Any byte written by
any session flips that number, so a whole-tree answer goes STALE within seconds
of a pass, and it goes STALE for a file the asking session never wrote.

A commit is scoped to a file list. Its evidence must be scopeable to the same
list, or a session can never hold evidence about its own code.

## The two artifacts

| File | Content |
|------|---------|
| `tmp/ze-verify.status` | One record per run: `exit`, `timestamp`, `mode`, `skipped`, `git_sha`, `tree_hash` |
| `tmp/ze-verify-manifest.txt` | One row per path that differs from `HEAD`, as `<fingerprint> <path>`, sorted bytewise |

`writeVerifyStatus` (`scripts/status/verify_run.go`) writes both when a run ends.
The manifest records the fingerprint the STAGES read, which is the one taken at
run start, not the one the tree holds when the run ends.

Only paths that differ from `HEAD` are recorded, a few hundred rather than all
7600 tracked files. A path in neither manifest is identical to `HEAD` on both
sides, which is the same answer as two matching fingerprints.

## Asking the question

`scripts/dev/verify-status.sh check` with no argument keeps its whole-tree
meaning, and `hook-parity-check.py` pins its exit code. `check <PATH>...` asks
about the named paths alone. A directory argument scopes to everything under it.

`verify_status` (`scripts/dev/commit_helper.py`) passes the commit's own file
list, so `commit_helper.py create --file <path>` asks about the files that
commit will carry. `scopeable_paths` drops the whole list when a path holds a
space or a backslash, and the whole-tree question is asked instead: the manifest
row format cannot express such a path, and a scope that matched no row on either
side would compare two empty sets and read FRESH.

## What the scope never widens

Scoping decides WHICH paths the answer covers. It never turns a red into a
green, and four rules hold whatever paths the caller names.

| Condition | Answer |
|-----------|--------|
| The last run exited non-zero | STALE, for every path list. The `check` arm reads `exit=` before it reads any scope argument |
| The last pass skipped suites (`ZE_SKIP_SUITES`) | STALE, for every path list |
| `HEAD` moved since the pass | STALE. A commit landing under a path rewrites what that path's content means, even when the working file is untouched |
| A structural gate is red inside the commit's own file list | The commit is refused. `STRUCTURAL_GATES` (`scripts/dev/commit_helper.py`) is the set `--unverified` cannot wave through |

## Which packages a scoped run judges

Freshness says which paths an answer covers. The change-set selector says which
packages a scoped run judges in the first place, and one program answers it:
`runSelector` (`scripts/checks/verify_scope_selector.go`), reached by
`make ze-verify-scope-selector`.

It takes the changed paths (unstaged, staged, untracked, and committed since the
last green commit) and gives back two answers on stdout: the `./`-prefixed
package list, and the feature tags the change can reach.

| Property | Value |
|----------|-------|
| Reverse-dependency bound | depth 2. Depth 3 lands within 3% of the full closure, and the closure selects a third of the tree for one edit under `internal/core` |
| Import graph | one `go list` with `ze_core` and every `feature-gates.txt` tag on, so a `//go:build ze_ssh` importer is visible |
| A non-Go path | seeds the packages that READ it, never the directory it sits in. `nonGoPathRules` is that map |
| A path no rule names | is named on stderr, and seeds the package it sits in when that directory holds Go source, the tooling packages otherwise |

`expandPackages` states on stderr what the depth-2 bound dropped from the
closure, so a later change to the depth is a measured decision.

Four inputs widen the answer to `./...`, and each one is said on stderr:

| Input | Why |
|-------|-----|
| `go.mod`, `go.sum`, `vendor/` | a dependency moved, so every package that compiles against it is reachable |
| No green commit in `tmp/ze-verify.status` | with no proven commit, every commit in history is unverified. A clean tree would otherwise select nothing and the scoped gate would report green over code no stage ran |
| A changed path that make cannot carry as one unquoted word | the recipes expand the list without quoting |
| A seed directory `go list ./...` does not report and no rule names | its importers cannot be found |

`ZE_VERIFY_STATUS_FILE` names the status file the green commit is read from
(default `tmp/ze-verify.status`).

A verify run selects ONCE, before its first stage: `verify_run.go` writes the
answer into the run's own artifact directory and exports
`ZE_VERIFY_SCOPE_PACKAGES` naming that file. `scripts/dev/changed-pkgs.sh`, which
every scoped recipe calls, reads that file when it is set and runs the selector
itself when it is not. So every stage of one run scopes to the same tree, and no
stage pays for a second graph walk.

## Which matrix rows a scoped run judges

`make ze-staticcheck-feature-matrix-check` runs ONE Staticcheck process over a
matrix of build configurations. `deriveFeatureMatrix`
(`scripts/checks/staticcheck_feature_matrix.go`) builds that matrix from
`feature-gates.txt`: `all_features`, `core_only`, and one `without_ze_<tag>` row
for each declared tag, 38 rows for the 36 tags declared today. Staticcheck
analyzes the configurations serially, so 38 rows is 38 full-module analyses.

`scopeFeatureMatrix` keeps a row when it omits no tag, or when it omits a tag
the change reached. A row that omits tag T differs from `all_features` only in
the packages T gates, so a change confined to the packages the scope names moves
that row only when T is one of them. A change local to `ze_ssh` judges
`all_features`, `core_only` and `without_ze_ssh`.

**The scope answer is the tags a change REACHES, and a file reaches a tag by
negating it as well as by being gated on it.** That second half is not a detail:
a file constrained `ze_web && !ze_ssh` compiles in `without_ze_ssh` and in no
other retained row, so an answer naming `ze_web` alone would subtract the only
row that can see a break in it. `reachedTags` therefore unions the tags a
changed file NEGATES into its answer, parsing the real constraint with
`go/build/constraint` rather than matching text. An argument for this filter
that reasons only about which packages a tag GATES is incomplete, and was the
form this page carried until the negation case was found.

A changed file the selector cannot read a constraint from -- a DELETED `.go`
file is the ordinary case -- widens the answer to every feature, because a
narrow answer there would be a guess.

The rows are SUBTRACTED from the derived set, never named. A tag added to
`feature-gates.txt` gains its row in `matrixRowsForTags` and is scoped with no
second file to edit, where a list of row names would stop covering it in silence.

Measured on this checkout: 3 rows in 17.7s against 38 rows in 215s with a warm
cache, 12.1x. Read that pair as an order of magnitude and not as a benchmark.
The box is shared and was carrying other sessions, and all three timed runs
exited 1 on a compile error already in the tree, so each timing includes a real
compile failure.

### The floor, and what widens

`validateScopedMatrix` refuses a scoped set of fewer than 2 rows, and refuses any
set whose count of rows omitting no tag is not exactly 2. Those two rows are
`all_features` and `core_only`. They judge the combinations Ze ships, and no
scope subtracts them.

`readChangeScope` widens to every row on any doubt, and each widening is a reason
rather than a failure.

| Input | Rows judged |
|-------|-------------|
| `ZE_VERIFY_SCOPE_TAGS` unset, which is a developer running the target alone | 38 |
| The named answer cannot be read | 38 |
| The answer names a tag `feature-gates.txt` does not declare | 38 |
| The answer names every declared tag | 38 |
| The answer is EMPTY | 2 |

An empty answer is a real answer: no gated package changed, so only the two rows
that omit no tag can move.

`runVerify` (`scripts/status/verify_run.go`) publishes the feature-tag half of
the selector's answer into the run's own directory and exports
`ZE_VERIFY_SCOPE_TAGS` naming that file. It publishes only when the selection
SUCCEEDED. A failed selection leaves the variable absent, which the matrix reads
as 38 rows. An empty file written there would be a 2-row fail-closed gate wearing
a valid answer's shape.

### The functional suite is not scoped by this

The functional stage runs every suite, and it is 1472s of the full run. No static
signal attributes a `.ci` file to a Go package: `go list -deps ./cmd/ze` links
562 of the module's 646 packages, so every suite reaches almost everything.
Deriving that map by OBSERVING what a suite executes is
`plan/spec-verify-scope-5-suite-coverage-map.md`, and until it lands a scoped run
judges fewer matrix rows and the same 24 suites.

## A path that moved while the run was in flight

A concurrent edit used to void the whole run: the runner compared the tree hash
before and after the stages and wrote a sentinel no tree could ever match, so
the record read STALE for ever.

`writeVerifyManifest` (`scripts/status/verify_run.go`) records the marker
`MOVED-DURING-RUN` for one path instead. Some stages read that path at one
content and the rest read it at another, so no stage judged what the checkout
now holds. No file content hashes to that word, so the path stays STALE even
after the edit is reverted, while every path that held still keeps a real
fingerprint and still answers FRESH.

The record compares the START snapshot with the END snapshot, so an edit that
begins and ends between them is invisible to it. No acceptance criterion covers
that window, and it has two shapes.

| Shape | Why the comparison misses it |
|-------|------------------------------|
| A path identical to `HEAD` at run start, written during the run, restored before the END snapshot | it differs from `HEAD` in neither snapshot, so it has no row in the record at all |
| A path ALREADY dirty at run start, written during the run, restored to its start content before the END snapshot | it has a row in both snapshots carrying the same fingerprint, so it keeps a real fingerprint and answers FRESH |

An edit reverted AFTER the run ended is a different case and IS caught: the END
snapshot saw the edit, so the path carries `MOVED-DURING-RUN` and stays STALE.
The whole-tree hash has the identical hole for the identical reason, and closing
either needs a third observation of the tree rather than a different marker.
`TestWriteVerifyStatusRecordsMovedPathsNotSentinel`
(`scripts/status/verify_run_test.go`) drives the second shape, so the limitation
is executable rather than only written down here.

## Attributing a structural red

`structural_gate_reds` (`scripts/dev/commit_helper.py`) returns `GateReds` with
three fields: `charged`, `unattributed`, and `foreign`. A gate is dropped only
when every one of its failure groups named files and every one of those files
lies outside the commit. A group that named a check name, a suite name, or the
stage's own name charges the gate and is reported by name, so an unattributable
red is visible rather than silent.

`failureGroup.Related` (`scripts/status/verify_run.go`) carries a file path for
lint and a package pattern for vet. A classifier that reads the stage's prose
carries a check name, a suite name, or the stage name instead, and a gate whose
groups look like that always charges. The section below is how a producer stops
being read that way.

**Which arm of that union a group holds is decided by its `kind`, against an
ALLOWLIST.** `PATH_BEARING_GROUP_KINDS` (`scripts/dev/commit_helper.py`) holds
`files`, `lint` and `package`, and a group of any other kind names no path, is
unattributable, and charges its gate. The polarity is the whole guarantee:
`kind` arrives as JSON a producer wrote, so the values this gate can meet are
not the values anybody enumerated. A denylist of the kinds that name NO path
made each new kind attributable by default, which is how `unparsed`
(`unparsedGroup`) came to be held back only by no checkout owning an entry
called `unparsed-group`. A producer that adds a kind must therefore teach this
set to read it, and until then its reds are charged rather than dropped as
somebody else's.

`classifyLint` writes the constant word `lint` as the kind for that reason, and
keeps the linter in the group id (`lint:<dir>:<linter>`). golangci-lint owns the
linter names, so a kind carrying one would hand the consumer a set it cannot
enumerate.

## The declared-group protocol

A classifier reads a stage's prose and guesses which files the failure is about.
The producer already knows. So any verify stage MAY declare its own failure
groups, and `classifyStage` (`scripts/status/verify_run.go`) asks for them FIRST,
before it dispatches to any classifier.

`ze-functional-test` is the one stage that is NOT asked first, and it loses
nothing by it: `classifyFunctional` reads the declared groups with the same
parser, then reconciles them against the FAIL summary, which names every suite
that failed. That is a stronger completeness statement than the count, so taking
the shortcut for that stage would replace it with the weaker one the day a
functional producer starts printing a terminator.

A producer prints two kinds of line.

| Line | When | Content |
|------|------|---------|
| `VERIFY FAILURE GROUP: <json>` | at the point of each failure | one `failureGroup` as JSON: `group-id`, `kind`, `related`, `summary`, `rerun` |
| `VERIFY FAILURE GROUPS COMPLETE: <n>` | once, when the run ends | the number of group lines the producer printed |

`parseDeclaredGroups` (`scripts/status/verify_run.go`) is the one reader of both,
and `classifyFunctional` and `classifyStage` share it.

**Adoption is all-or-nothing, and two mechanisms hold it there.**
`classifyStage` replaces its groups with `genericGroup` only when the slice is
EMPTY, so a run that declared groups for some of its failures and not for the
rest would fill the slice and take the missed failures out of the failure index
with it.

The count is the first mechanism, and it detects a group set that reached the
reader DAMAGED. The declared groups are preferred only when `<n>` equals the
number of group lines read, so a log truncated before the count, a producer that
died between a group and its count, a child relaying a group line of its own into
the stage log, and a count that is absent, printed twice or unreadable all send
the stage back to its classifier, which is the answer it would have had anyway.

**The count does NOT detect a failure that declared no group**, and that is the
second mechanism's job. `<n>` is the number of declarations the producer MADE, so
a check that fails in silence raises neither side of the comparison and publishes
a count agreeing with itself. `run_check` (`scripts/dev/verify_wiring_docs.py`)
closes it at the producer: every check runs through it, and a check that returns
non-zero or raises having declared nothing is given an unattributable group
naming the check and no file. "Declared nothing" becomes "declared one no-file
group", which the commit helper charges. The count is then honest by
construction, rather than by every future check remembering the convention.

**A group is printed where the failure is decided, never collected for an
end-of-run dump.** `main` (`scripts/dev/verify_wiring_docs.py`) returns early
when the wiring check fails, and `run_make_target` raises past the delegated
targets it has not reached, so a dump would print the groups of whichever
failures come last and none of the others. Only the count is at the end, and
losing it costs a fallback rather than a failure.

**A group that names no file is CHARGED, not dropped.** `declare_failure_group`
derives the kind from `related`: a group with paths gets `files`, which
`PATH_BEARING_GROUP_KINDS` (`scripts/dev/commit_helper.py`) holds, and a group
with none gets `subcheck`, which it does not. So the ci-sleep ratchet, which sums every
`.ci` in the tree against one committed ceiling, and a delegated target, which
relays another program's stdout, both charge the session that is committing. The
empty answer must never be the valid-looking one (`ai/rules/evidence.md`).

Four more edges hold the protocol closed.

| Edge | Behavior |
|------|----------|
| A group line whose JSON does not parse | becomes a group of its own (`unparsedGroup`), of a kind outside `PATH_BEARING_GROUP_KINDS`, so the gate is charged whatever the checkout holds. Skipping it would delete the failure the producer printed the line for |
| A path holding a quote, a newline, or the prefix itself | is escaped into its JSON string by `json.dumps`, so it cannot forge a second group |
| A check naming hundreds of files | is chunked at `RELATED_PER_GROUP` paths per group, the rest going to `<kind>:<name>#N`. A truncated `related` would hide the file that makes the red this session's |
| A stage log too long for one scanner token | ends the read, and `splitLines` appends `logTruncatedMarker` rather than returning a short slice in silence. `classifyStage` turns that marker into a group of its own (`truncatedLogGroup`), of a kind the commit helper charges, because what the unread tail reported can be attributed to nobody. The lost group lines usually make the count disagree on top of that, so the stage also falls back |

`ze-doc-wiring-check` is the only stage that declares groups today. The protocol
is available to every stage, and adopting it in another producer is separable
work: each one has to be right about which files its failure is about, and each
one deserves its own evidence.

## Clearing verification debt

`record_debt` (`scripts/dev/commit_helper.py`) writes one row per override into
`plan/verification-debt/<session>.md`, and `open_debt_rows` refuses `--push`
while one row is open.

`make ze-verify-debt-clear` re-runs the gate each open row names and writes
`cleared` only when that run exits 0. Each distinct gate runs once per pass,
however many rows name it. A row whose gate no command can produce, such as
`independent critical review`, is reported UNRUNNABLE and stays open, and so
does a row whose gate string no runner is registered for.

## What proves this

| Behavior | Test |
|----------|------|
| The scoped compare reads only the named paths | `scripts/dev/verify_status_test.py` |
| The helper scopes to its own commit file list | `TestVerifyStatusScope` (`scripts/dev/commit_helper_test.py`) |
| A moved path is recorded, not a whole-run sentinel | `TestWriteVerifyStatusRecordsMovedPathsNotSentinel` (`scripts/status/verify_run_test.go`) |
| A red is charged unless every group names a foreign file | `TestStructuralRedAttribution`, `TestDebtNotChargedForForeignRed` (`scripts/dev/commit_helper_test.py`) |
| The wiring gate's declared groups drop a foreign red and charge a blind one | `TestWiringGateAttribution` (`scripts/dev/commit_helper_test.py`) |
| Declared groups beat the classifier, and only when the count agrees | `TestClassifyWiringDocsPrefersDeclaredGroups`, `TestClassifyWiringDocsFallsBackToProse` (`scripts/status/verify_run_test.go`) |
| A stage with no classifier can still declare its own groups | `TestAStageWithNoClassifierCanDeclareItsOwnGroups` (`scripts/status/verify_run_test.go`) |
| An unreadable group line becomes a group, and a path cannot forge one | `TestMalformedGroupLineIsReported`, `TestAPathologicalPathCannotForgeAGroup` (`scripts/status/verify_run_test.go`) |
| The Python producer and the Go reader agree on the prefix and the keys | `TestTheWiringGateSpeaksTheProtocolThisRunnerReads` (`scripts/status/verify_run_test.go`) |
| Every failure path of the wiring gate declares a group | `TestEveryWiringSubcheckDeclaresItsFiles`, `DeclaredGroupProtocolTest` (`scripts/dev/verify_wiring_docs_test.py`) |
| A truncated stage log says so, rather than reading as a whole one | `TestSplitLinesReportsATruncatedRead` (`scripts/status/verify_run_test.go`) |
| A truncated read becomes a charged group, so the marker reaches a reader | `TestATruncatedStageLogBecomesItsOwnGroup` (`scripts/status/verify_run_test.go`) |
| The functional stage keeps its FAIL-summary reconciliation | `TestTheFunctionalStageKeepsItsSummaryReconciliation` (`scripts/status/verify_run_test.go`) |
| A check that fails without declaring is charged anyway, dispatch included | `ASilentFailureIsChargedTest` (`scripts/dev/verify_wiring_docs_test.py`) |
| Clearing runs the gate and honors its exit code | `TestDebtClear` (`scripts/dev/commit_helper_test.py`) |
| The shell path end to end, over a throwaway git repo | `test/runner/verify-scope-freshness-scoped.ci` |
| The wiring gate's attribution end to end, over a throwaway git repo | `test/runner/verify-scope-wiring-attribution.ci` |
| The clearing entry point end to end, over throwaway git repos | `test/runner/verify-scope-debt-clear.ci` |
| A gated importer is visible, and an unclassified path narrows | `test/runner/verify-scope-selector.ci` |
| Each changed path kind seeds the packages that read it | `TestSelectorMapsToolingInputKinds`, `TestSelectorMapsTheFunctionalCorpusToItsWalkers` (`scripts/checks/verify_scope_selector_test.go`) |
| No green commit widens instead of selecting nothing | `TestChangedPkgsWidensWithNoTrustedGreenBaseline` (`scripts/dev/changed_pkgs_test.go`) |
| The matrix subtracts the rows a change cannot move | `TestMatrixRowsScopeToChangedTags` (`scripts/checks/staticcheck_feature_matrix_test.go`) |
| A break only one retained row can catch is still caught | `TestMatrixRowFilterCatchesAGatedBreak` (`scripts/checks/staticcheck_feature_matrix_test.go`) |
| One selector run names the feature scope to every stage | `TestVerifyRunNamesTheFeatureScopeToEveryStage` (`scripts/status/verify_run_test.go`) |
| A failed selection leaves the feature answer absent, never empty | `TestVerifyRunWidensWhenTheChangeSetCannotBeSelected` (`scripts/status/verify_run_test.go`) |
| The runner and the matrix spell the variable the same way | `TestTheStaticcheckMatrixReadsTheFeatureScopeVariable` (`scripts/status/verify_run_test.go`) |

Every `.ci` above builds its own git repo and runs each command inside it, so
none of them touches the `tmp/ze-verify.status`, `tmp/ze-verify-manifest.txt` and
`tmp/ze-verify-failures.json` that the other sessions in this checkout read.

<!-- source: scripts/dev/verify-status.sh -- tree_hash, dirty_manifest, manifest_scoped, the check command -->
<!-- source: scripts/status/verify_run.go -- writeVerifyStatus, writeVerifyManifest, movedDuringRun, classifyStage, classifiedGroups, truncatedLogGroup, parseDeclaredGroups, unparsedGroup, splitLines, selectChangeSet, parseChangeSetAnswer -->
<!-- source: scripts/checks/staticcheck_feature_matrix.go -- deriveFeatureMatrix, readChangeScope, scopeFeatureMatrix, validateScopedMatrix, matrixRowsForTags -->
<!-- source: scripts/dev/verify_wiring_docs.py -- declare_failure_group, declare_groups_complete, run_check, child_finding_paths, run_gate -->
<!-- source: scripts/dev/commit_helper.py -- verify_status, scopeable_paths, structural_gate_reds, group_related_paths, clear_debt -->
<!-- source: scripts/checks/verify_scope_selector.go -- runSelector, nonGoPathRules, greenBaseline, expandPackages -->
<!-- source: scripts/dev/changed-pkgs.sh -- widen, EVERY_PACKAGE -->

Related: [`docs/functional-tests.md`](../../functional-tests.md),
[`ai/rules/precommit-verify.md`](../../../ai/rules/precommit-verify.md)
