# Test-Health Reporting and the Sensitivity Ratchets

How ze answers "would a regression actually be caught?" about its own test
suite, as opposed to "how many tests are there?". Those are different questions:
a suite can grow without limit while the share of behaviour a change would break
red keeps falling, and no test count shows that.

The user-facing report is [`../../features/test-health.md`](../../features/test-health.md)
(generated). This document is the architecture behind it: what is measured, what
is enforced, and where each number comes from.

## Two independent mechanisms

The feature is two things that share collectors but not enforcement.

| | Ratchets | Report |
|---|---|---|
| Question | "Did this commit make sensitivity worse?" | "What is the state of the suite?" |
| Enforced by | `./le test sensitivity check`, stage 10 of `./le verify current` | `./le test health check`, inside `./le repo check generated` |
| Reads | `test/health/sensitivity-baseline.json` + the working tree | the committed report vs the tree |
| Source | `internal/le/test/sensitivity.Answer` | `internal/le/test/health.Answer` |
<!-- source: internal/le/verify/engine/run.go -- RunMode, RunPart -->

The ratchets do NOT depend on the report. `./le test sensitivity check` reads only
the baseline and the tree, so a stale or wrong report cannot weaken the
guarantee that you cannot add an inert test or strand a test file.

## The sensitivity detectors

`./le test sensitivity check` runs the native AST detectors in
`internal/le/test/sensitivity`. Its `selftest` action proves each detector on
known-bad fixtures before the live-tree check is trusted. The selftest answers
one row per case, so row operators such as `| count` apply to those cases.
The scanning actions still answer one document containing both finding sets;
row operators cannot select a set implicitly.
<!-- source: internal/le/test/sensitivity/register.go -- init -->
<!-- source: internal/le/le/root/leroot.go -- Run -->

### assert-nothing

A `Test` function with no reachable failure call: no `t.Error`/`Fatal`/`Fail` on
a testing receiver, no assertion-library call, no compile-time `var _ T = ...`
assertion, no `panic`. It executes code, moves coverage, and passes
unconditionally. Deleting the body of the function under test would not turn it
red.

Benchmarks and fuzz targets are exempt: a benchmark measures, and a fuzz target
delegates its oracle to the engine. Assertion helpers are followed at most two
calls deep, so a scenario helper can delegate to an assertion helper. Recursive
or deeper chains stay bounded. Import aliases are resolved so `require` under
a different name still counts.
A helper is judged with the imports of the file that declares it, so a test
whose own file imports only `testing` is credited when a helper in a sibling
file asserts through testify. Testing-receiver names come from the helper's own
parameters, never from its caller.
Testing receivers include the `testing.TB` interface as well as pointers to
`testing.T`, `testing.B` and `testing.F`. Taking that interface alone is not an
assertion: a helper still needs a failure call. A different package's `TB` type
does not count.

### tag-orphan

A `_test.go` file whose `//go:build` constraint is unsatisfiable under every
native test action is a tag orphan. The tag universe is derived from the Go
action tables and feature manifest, so a new feature gate cannot silently
orphan its tests. The satisfiability search handles negated and compile-out
constraints such as `!linux` and `ze_core && !ze_web`.
<!-- source: internal/le/test/sensitivity/tags.go -- tagUniverse -->

### Detector pitfalls

Four traps hit while building these detectors, recorded because each produced a
green or a plausible number that was wrong:

- **A name-shaped heuristic is porous.** Matching `"/is"` as a substring for an
  assertion-library import exempted all of `internal/plugins/isis/**`, which is
  143 tests. Match the final path element. A method-name match with no receiver
  check credited `fmt.Errorf(...)` as a failure call. Discriminate on the
  receiver.
- **A correction can over-shoot into the opposite defect.** Narrowing a
  negative-test regular expression that over-matched setup guards narrowed it
  past the house style `if err == nil { t.Fatal }`, which halved the figure
  (418 against 828) and published `0/31` for subsystems holding 13 files of
  rejection tests.
- **A guard driven only through its helper is not tested.** Several
  fail-closed tests called the helper directly and passed, while the real entry
  point crashed on the same input because a different collector read the file
  first. Drive a fail-closed guard from its entry point.
- **A scripted edit is not done until it is grepped.** Two string replacements
  silently matched nothing after a formatter reflowed their targets, and the
  review record said "fixed". A false review record stops the next reviewer
  looking.

### The floors

Committed in `test/health/sensitivity-baseline.json`. Counts may only go DOWN,
following the `test/.ci-sleep-baseline` convention: lower the floor in the same
change that improves the number. A missing file or key is an error, not a fresh
floor, so deleting the baseline cannot launder a regression; first-time creation
is the explicit `--bootstrap-baseline`.

The ratchet scans the WORKING TREE, so an inert test is caught by the
`./le verify current` run that precedes its commit, not blamed on the next one.

### The question these detectors do not ask

`assert-nothing` asks whether a test CAN fail. The RFC claim-discrimination
ratchet asks whether one test fails for the reason its tag CLAIMS. A test that
asserts something real, and something narrower than its `RFC requirement:` tag
advertises, passes every detector on this page. `./le rfc check` refuses it once
a record exists for that tag under `rfc/discrimination/`, by replaying the
recorded break. The record and its refusals are in
`docs/contributing/rfc-conformance-gates.md`, "The discrimination record".

The two gates take opposite shapes, for a reason worth reading before you copy
either one. A floor works here because the detectors read the whole tree in one
AST pass, so the count is cheap and total. A discrimination proof costs one test
run per tag, over a corpus of about 3,900 tags, and a floor set low enough to
reach on day one forbids only going below zero. That obligation is
change-scoped instead: a tagged unit that is new against git HEAD owes its
record in the change that adds it, and the standing corpus is grandfathered.
<!-- source: internal/le/rfc/check_ratchets.go -- checkDiscriminationRatchet -->

## The per-commit weakening record

A third gate, on a different failure: a test that stopped proving something.

The compiled weakening detector in `internal/le/test/weakened` refuses an edit that
deletes assertions, adds a `t.Skip`, drops an `expect=`, or introduces an
assertion that cannot fail. Only an independent agent running the
`ze-test-judge` skill (`ai/skills/ze-test-judge.md`) can let such an edit
through, before the edit; the agent that wrote the change never judges it, and
the question is never put to Thomas. The judge records its verdict with
`./le test weakened approve unit <package>.<TestName> reason "<the judge's verdict>"`,
which writes one row to `tmp/commit-weakening-approved-<session>.md`. git never
reads that file. The commit that uses the row carries it as a
`Weakening-approved: <package>.<TestName>: <the judge's verdict>` trailer, and that commit
is the record. The judge's approval of an RFC-tagged test change works the same
way through `./le rfc approve` and an `RFC-approved:` trailer
(`docs/contributing/rfc-implementation-guide.md`), and an `RFC-approved:` row for
a unit also accepts that unit's weakening, so one test never needs two answers.

| | |
|---|---|
| Refused at edit time by | `internal/le/hookruntime` calling `internal/le/test/weakened` |
| Refused at commit time by | `checkSourceGates` (`internal/le/commit/prepare.go`), which runs `testweakened.Check` |
| Reads | `tmp/commit-weakening-approved-<session>.md`, `tmp/commit-rfc-approved-<session>.md`, and immutable baseline/selected blobs for the paths the commit names |
| Source | `internal/le/test/weakened.Check`, called by both gates |
| Reviewed afterwards by | `./le test weakened check`, which reads the checked commit's own `Weakening-approved:` and unbound `RFC-approved:` lines. Either admits its unit's weakening; only an unbound `RFC-approved:` admits its carrier's RFC-tagged change. A bound historical verdict is not reused for a different change in its publication carrier |
<!-- source: internal/le/test/weakened/testweakened.go -- Check, loadApprovalFile, claimFinding -->

### One approval names one test

The unit is `<package>.<TestName>`: the name of the directory holding the file,
then the enclosing top-level `func TestXxx`, or the file stem for a `.ci`, an
`.et`, or a Go weakening sitting outside every func. `internal/le/rfc/tags.go` resolves each
one, so the edit-time hook and the commit gate name the same unit. A bare name, a
path, a tree or a file scope is refused by the writer: one approval covers one
test, and nothing wider. A row of another shape written by hand into an approval
file is refused by every reader of that file: the hook, the commit gate and the
audit's worktree pass all read it through one loader, which fails closed on it
and prints the approve command. A commit message line of another shape approves
nothing: the audit skips it and still reports every weakening it might have
named, because a pushed message cannot be corrected and a prose sentence that
opens with `Weakening-approved:` must not block every later audit. The same
loader marks a row whose trailer a landed commit already carries, so the hook
refuses a second edit under it and names that commit, as the commit gate does.

The writer resolves the unit to its one carrier before it writes anything,
for an ordinary row as for a targeted one (`rfc.ApproveInto` calling
`rfc.ResolveApprovalUnitContent`). An unknown or ambiguous unit is refused.
So is a unit spelled with a Go package clause that its directory does not
carry (`repoinventory.TestX` for a test in `internal/le/repo/inventory`): every
reader keys the unit by the directory, so such a row would be written and never
matched, and the refusal names the directory key to record instead.
<!-- source: internal/le/rfc/approvalunits.go -- ResolveApprovalUnitContent, noApprovalCarrier -->

The registered action `./le test weakened approve` resolves the unit and
checks the reason in a read-only preparation that runs before startup can
write. A refused request leaves no commit namespace and no approval file
behind. Only its admitted answer creates the main thread's commit namespace
and writes the row, through the same `Approve` writer.
<!-- source: internal/le/test/weakened/approve.go -- Approve, approveAnswer, approveAdmitted -->

Every approval file holds one table, `| Test | Reason | Target | Content |`.
`Content` is `<base>..<current>`: the SHA-256 digests of the unit's text at
HEAD and in the working tree when the verdict was recorded (`none` for a side
with no carrier). The verdict judged that change, not the unit's name, so the
loader marks an ordinary row stale once the unit no longer matches it, and a
stale row approves nothing at the hook, the commit gate or the audit's
worktree pass. A row recorded before the edit (its two digests equal) admits
a change from the base the judge read, and none once HEAD's unit moves on. A
row recorded after the edit admits only the very text the judge read; a later
different change of the same unit needs a verdict of its own. Re-recording
replaces the row. A table without the `Content` column, or a row without a
digest pair, is refused, because only the approve commands write rows.
<!-- source: internal/le/rfc/approvalunits.go -- ApprovalContentCell, ApprovalContentAdmits -->
<!-- source: internal/le/test/weakened/ledger.go -- parseLedger, staleApproval -->

The shared table also reads targeted RFC rows through its `Target` column.
Those rows approve only their exact historical change and are never used by the
worktree weakening gate, edit hook or ordinary current-change selection. Their
publication is explicit (`rfc-approval <revision> <unit>`), and landed detection
requires the ordinary verdict and exact target binding in the same commit.
The historical audit reads companions through the same strict full-SHA/unit
parser as push debt. A bound RFC verdict does not also approve a separate
weakening or tagged change of the same unit in the publication carrier;
ordinary weakening verdicts in that carrier remain independent.
<!-- source: internal/le/rfc/approve.go -- ParseApprovalForTrailer -->
<!-- source: internal/le/test/weakened/audit.go -- acceptedRows, acceptedBy -->

The unit keeps only the base name of its directory, so a test `TestX` in
`a/cmd` and one in `b/cmd` are both `cmd.TestX`. When one commit changes that
unit in more than one directory, whether as a weakening or as an RFC-tagged
change, no approval of it can say which test the judge approved. The commit gate
refuses the commit before it judges any approval or `rfc-change-ok` admission,
naming the directories and telling the author to split the commit so each
carries one of them. The audit lets no approval line admit either test, and each
finding names the directories. The edit hook judges one file per edit, so the
case never arises there.
<!-- source: internal/le/test/weakened/approve.go -- Approve, ApprovalPath -->
<!-- source: internal/le/test/weakened/ledger.go -- LoadApprovals, SharedUnits, SharedUnitProblems -->
<!-- source: internal/le/test/weakened/audit.go -- acceptedRows, auditFile -->
<!-- source: internal/le/commit/prepare.go -- checkSourceGates -->

The file is keyed on the main thread's commit namespace (`./le commit session`
in the main thread), not on the agent that makes the edit or the judge that
records the verdict, so a subagent's edit sees an approval the judge recorded
under the same main thread.

The edit hook judges only files inside the checkout. A path outside it after
symlinks are resolved, such as the user's `~/.claude/CLAUDE.md` or another
checkout's test, passes unjudged, because no test of this repository lives
there.
<!-- source: internal/le/hookruntime/writeedit.go -- writeWeakening -->

### A row is used once

A semantic test change can keep or add assertions, so the structural detector
can report no finding. `Check` also compares selected unit text with its baseline
and carries a live approval for that changed existing unit, trying weakening
rows first and RFC rows second, as it does for structural findings. This includes
comment-only edits (`//` in Go, whole-line `#` in `.ci`, `.et` and `.py`
files) that keep every requirement tag ID, which leave the RFC behavior
fingerprints unchanged. It uses the
detector's unit inventory and the same selected blobs and rename pairs. Shared
row tracking and the commit gate's RFC merge prevent duplicate trailers and
duplicate pruning when the selectors overlap.
Unchanged units, excluded hunks and foreign-session approvals contribute no
verdict. New functions and entirely new test files spend no semantic approval.
The commit gate checks these approved units for ambiguous directory
names together with structural findings and RFC changes.

Commit preparation records HEAD once and applies earlier captured script blocks
to form its immutable comparison baseline. It derives rename pairs by comparing
that baseline with the selected prospective tree used by closure review. Whole
files and selected hunks are read from captured blobs; the shared index and
concurrent checkout edits are not alternate sources. An unavailable baseline or
blob refuses the comparison, rather than becoming an absent or empty file.
<!-- source: internal/le/test/weakened/prospective.go -- ProspectiveTree -->

Go units come from the one inventory the RFC gate reads too,
`rfc.DeclaredUnits`, so the two gates cannot name different units. It parses
the whole file with `go/parser`. A unit runs from its contiguous `//` doc
comment through its closing brace, or the last signature token of a bodyless
declaration, plus the one line ending after it; a block comment above it and
separators after that line ending stay outside. Comments and literals cannot
introduce declarations. An edit is judged on the whole proposed file, so a
fragment with no package clause never reaches it, and one that did would be
an unparsed file.

A file that does not parse keeps the declarations the parser read whole
before its first syntax error. Every declaration from that error on is
incomplete, a declaration the parser's recovery skipped is absent rather than
guessed, and a truncated result type cannot become a complete declaration. A
bodyless declaration in such a file is incomplete too, because the parser ends
it on the next token, which can be the error itself (`func TestA() )`). The
RFC gate's readers still receive their conservative line-walk spans for such a
file, so the partial syntax tree never moves an RFC unit.

Incomplete boundaries cannot spend semantic approvals: preparation reports that
it cannot judge them. Structural checks still judge the whole text and attribute
unresolved findings to the file stem. Residual evidence is reported even when
additions in a complete unit cancel the whole-file count. A complete unit's
approval cannot cover that evidence. File-stem details are deduplicated only
when their full text is identical.
Deleting a complete named function keeps its assertion-count reduction under
that function, even though its replacement is empty. A whole-file count drop
already accounted for by that deletion does not require a second file-stem
approval; genuinely unresolved or outside-function assertion loss still does.
At edit time, any blocking named or residual verdict opens the existing
per-unit approval lookup, even when compensating sibling changes leave
whole-file assertion or skip counts unchanged. An advisory-only count reduction
remains a notice at edit time; the commit check independently requires approval
for every weakened unit.
<!-- source: internal/le/test/weakened/scope.go -- weakenedUnits -->
<!-- source: internal/le/test/weakened/proposed.go -- Proposed, proposedFindings -->

A syntax error after a closed unit does not invalidate that unit's boundary.
<!-- source: internal/le/test/weakened/scope.go -- changedUnitNames, unitsByName, weakenedUnits -->
<!-- source: internal/le/test/weakened/scope_go.go -- goUnits, goUnresolvedText -->
<!-- source: internal/le/rfc/goscope.go -- DeclaredUnits, readFunctionUnits -->
<!-- source: internal/le/test/weakened/testweakened.go -- Check, comparePath -->
<!-- source: internal/le/commit/prepare.go -- checkSourceGates -->

`./le commit create` admits each weakened unit against a live row, writes that
row's trailer into the message, and stages nothing for it. The generated script
removes every used row from the approval file only after `git commit` succeeds,
so a failed commit keeps the approval for the retry, and a row nothing used stays
because it is still the judge's live verdict. A row whose trailer git history
already carries approves nothing further: a second weakening of the same test is
refused, naming the commit that used the first approval.
<!-- source: internal/le/commit/script.go -- renderApprovalPrune -->
<!-- source: internal/le/rfc/approve.go -- TrailerLanded -->

Until 2026-10-24 the row lived in a tracked ledger, one shard file per commit
session. The agent wrote the row, made the edit and committed
both, so the owner never saw the weakening, and the shard recorded nothing the
commit did not already show.

**The commit gate judges the paths the commit NAMES, never the working tree**,
which is where it differs from the sensitivity ratchet above. Several sessions
share this checkout, so a tree-wide count moves under whoever reads it. It went
751 to 752 to 755 within an hour on 2026-08-10, on edits by three sessions that
had never touched this gate. A gate that reds on another session's half-finished
work is a gate that gets switched off (`ai/rules/repo-maintenance.md`, "the two
changed-file checks"). Keying on the commit's own paths makes the BLOCK tier safe
by construction.

Until 2026-08-16 the justification was a `// test-relax:` comment in the test
file, capped by a ceiling that a census counted at HEAD. Two properties of that
corpus are why the storage moved, and both are recorded in
[`TEST-RELAX-AUDIT.md`](TEST-RELAX-AUDIT.md).

Volume destroyed the mechanism. At 751 tokens across 466 files, reading them was
no longer possible. So nobody read them, writing one cost nothing, and more got
written.

A token never expired. Three `redistribute-l2tp-*.ci` tests carried a
`KNOWN ENGINE ISSUE` justification whose claim was refuted in-place on
2026-07-16, and whose tracking spec had been closed and deleted. Both statements
sat in the file and contradicted each other for four months.

## The report

`internal/le/test/health.Answer` aggregates ten metrics, each assigned to one of
three questions (sensitivity / intent / integrity) and each stating the action
its degradation implies. Every number is derived from committed state; which
files count comes from `git ls-files`, so an untracked scratch test does not
move the figures.

The RFC annotation split uses `rfc.AnnotationKinds` for its vocabulary and
`rfc.AnnotationTraitsOf` for denominator membership. Derived rollup rows remain
visible in the split but outside its accounting total; every other current
kind stays in that total. An unknown kind cannot shrink it. These are the same
traits `rfc.CoverageRows` reads, not a health-specific list of exceptions.

The headline beside that split is not computed here. `implementedSections`
renders the corpus once and sums `rfc.StemFiguresOf` through
`rfc.ImplementedSectionsOf`, the same two functions the site's home page reads;
the RFC compliance page draws the same per-RFC sections. It is taken over the
RFCs that are enrolled, render a public row and declare an Implementation of
`ze` or `mixed`. The density is Ze proven over the MUST rows Ze and the OS owe
(Ze proven, OS verified, OS testing gap and Ze correctness gap), and it counts a
row as Ze proven only where a discrimination record still verifies on a walked
summary. A tag pair no record proves adds nothing. A row not owed (not
applicable, declined, OS blocked) is on neither side, and the detail states how
many there are. A row counts not owed only when its evidence passes the
exclusion evidence check; one lacking it stays owed
(`docs/contributing/rfc-conformance-gates.md`, "The exclusion evidence ratchet"). Only walked RFCs add to any section, and the detail names how
many of those RFCs are not walked and left out.
<!-- source: internal/le/test/health/collect_rfc.go -- annotationCounts.total, implementedSections, densityMetric -->
<!-- source: internal/le/rfc/implemented.go -- ImplementedSectionsOf, Implements -->
<!-- source: internal/le/rfc/rfc.go -- AnnotationTraitsOf -->

A Linux-limit annotation earns no proof credit. Once its register entry
verifies, the row is not owed (OS blocked) and leaves that denominator, because
the kernel, not Ze's code, is what blocks it. One whose entry does not verify
stays owed as a Ze gap. The density detail counts the OS-blocked rows apart from
the not-applicable and declined ones. The annotation split discovers the kind
from the same registry; it never treats a Linux limit as a proof.
<!-- source: internal/le/test/health/collect_rfc.go -- densityMetric, annotationCounts.total -->

### What is gated, and what is only published

`./le test health check` compares only the STRUCTURAL facts against the tree: the
orphaned-test-file list, the unproven-RFC list, and every metric's status. Each
of those changing is an event, and a status flipping to `unknown` means a
collector stopped measuring, which is the sensor rot the report exists to
surface.

The volume counters (test counts, ratios, adoption buckets) are published, not
gated. Byte-comparing the whole report charged a regenerate-and-commit to ~60%
of commits, because every added test moves a denominator, and a check that fires
that often for cosmetic reasons gets routed around rather than read: the
"advisory gate permanently red" failure the report is built to expose. The
counters are refreshed by `./le repo update` and the page discloses that they may
lag.
<!-- source: internal/le/test/health/actions.go -- Answer -->

### The KPI series

`./le test health record` appends one row to `test/health/history.ndjson`.
`./le test mutation record` maintains the per-package mutation series. Both
skip an identical sample at the same commit, so trends do not overstate the
number of runs.

## Publication

The native `./le site build` action renders the website health page from
`internal/le/test/health.Render`, which answers the metric record and the page
Markdown for the tree being built and writes neither file. The site builder
computes no new test-health facts: it draws the same metrics, under the same
three questions, in the same worst-first order. It carries every figure the
page prints into the `health` sections of the site's facts snapshot, keyed by
each metric's own `key`, and the page prints each figure from there as a marked
element, so a metric or a payload column the collectors gain reaches the
snapshot and the page with no edit to the site
(`docs/architecture/site-facts.md`).

It reads the tree rather than the committed `test/health/latest.json`, because
the volume counters are published rather than gated (above) and are refreshed by
hand. A page sourced from the committed snapshot would state the tree as of the
last refresh while claiming to describe the tree it was built from. The KPI
trends still come from the committed `test/health/history.ndjson`, which is the
only record of how the numbers moved.
<!-- source: internal/le/test/health/modes.go -- Render -->
<!-- source: internal/le/site/health.go -- renderHealth -->
<!-- source: internal/le/site/healthfacts.go -- healthFacts -->
<!-- source: internal/le/test/health/actions.go -- Actions -->

## Full operator reference

`test/health/README.md`.
