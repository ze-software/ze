# Committing

`./le commit create` is the only staging and commit route in this repository.
It writes a message file and an executable script; you run the script. There is
no hand-written fallback.

What you OWE around a commit is `ai/rules/git-safety.md`. This page is how the
command works.

## Why one command owns it

Several sessions share this checkout, so they share one git index and one
working tree. A loose `git add` followed by a `git commit` therefore carries
whatever another session staged in between, and `git add` reads the working
tree, so it also carries whatever another session wrote into a file your commit
names.

The generated script answers both. It commits from an index of its OWN, seeded
from HEAD when the script runs, so the commit's population is exactly the paths
the block names. And it commits the blob each path held when `create` READ it,
so an edit that arrives afterwards stays in the working tree for whoever wrote
it to commit under their own subject.

Neither property depends on you noticing anything. Eight rows in
`plan/journal/concurrent-session-corruption.md` are one session's unfinished
work published under another session's message, and the last of them happened
to an author who had been warned about those exact files minutes beforehand.

## The keywords

`./le commit create` takes keywords, not flags. Every one takes a value except
the switches `append`, `replace` and `dry-run`. `./le commit create --help`
prints the same table from the grammar the command reads.

A value never begins with a dash (`ai/rules/cli.md`): `body "- item"` is
refused, because `- item` stands where a value goes and reads as an option.
Write a bullet as `body "* item"`, or put a word before the dash, as in
`body "Changes: - item"`.

| Keyword | Repeats | Meaning |
|---------|---------|---------|
| `subject` | no | The one-line commit subject. At most 72 characters, and a longer one is refused with the count and the overage |
| `body` | yes | One body chunk, wrapped to 72 characters without breaking a word. Two chunks run together, so a paragraph break is an empty `body ""` between them |
| `file` | yes | One explicit file to stage. Never a directory |
| `file-list` | yes | A file holding one path to stage per line. Blank lines and `#` comments are skipped |
| `hunks` | yes | Two values, `hunks <path> <patch-file>`: commit HEAD's blob of `<path>` with only the hunks in `<patch-file>` applied, and leave the working-tree file as it is. See "Committing some hunks of a shared file" below. Refused with `attest`, and for a path also named by `file` or by `hunks` twice |
| `remove` | yes | One tracked file to delete, named exactly: a directory or a glob is refused at create time, because the script removes one index entry by its literal name. The commit removes it from Git, then the script deletes the working-tree copy when that copy is what the commit removed. Leave the file in place: no `rm` beforehand |
| `remove-list` | yes | A file holding one tracked path to delete per line, each handled as `remove` handles one |
| `rfc-approval` | yes | Two values, `rfc-approval <revision> <unit>`: publish the judge's pending approval of that exact historical change and canonical unit. Revisions resolve once. Missing, replayed and ambiguous same-unit selections refuse |
| `replace` | no | Start a fresh script. Use it for the first commit of a session |
| `dry-run` | no | Print the message and the script it would write, and write neither |
| `append` | no | Add another commit block to a script that already exists. A script an older `./le` prepared, whose blocks do not record the commits they publish, is refused: prepare a fresh one with `replace` |
| `script` | no | The script to append to. `create` with no `script` always gets a distinct path |
| `session` | no | The eight-hex commit namespace. `./le commit session` creates or reuses this harness session's |
| `tag` | no | The block tag inside the script |
| `attest` | no | Two values: a commit and the owner's words, `attest <commit> "<owner words>"`. Prepares an EMPTY commit (its parent's tree) whose message carries `Owner-attested: <stable patch-id>: <owner words>`, which answers every person act the named commit owes. Takes no `file` or `remove`: an attestation that changed the tree would carry code the owner never judged. Takes no `no-test`, `review-override`, `rfc-change-ok` or `ledger-carried` either: each acts on a commit that changes files, and an attestation changes none and runs no gate, so the value would be recorded nowhere. The refusal is derived from the create grammar (`createParameters`) and its policy (`createPolicy`), so a keyword added to the grammar is refused with `attest` until its policy declares it allowed. Refused for a commit already on the upstream, one not on this branch, and one carrying no owed trailer |
| `ledger-carried` | yes | Only on the ledger-deletion commit (`remove` paths, no `file`): `"<sha> <gate key>: <reason>"`, the gate key `review-override` or `rfc-change-ok`. Writes one `Ledger-carried:` trailer. A row on a commit already on the upstream carries no patch-id and is audit only; a row on an unpushed commit carries that commit's stable patch-id, computed here and never given, and holds the push until it is answered |
| `no-test` | no | The reason a commit carries no test evidence |
| `no-news` | no | The reason a spec closure carries no news fragment, because operators see no change: `no-news "tooling: no operator-visible change"`. Given on commit B, the commit that removes the spec, it writes the `No-news: <reason>` trailer just before `Spec-work:`. Outside the Ze checkout it records nothing. "What the command refuses" names the fragment a closure owes otherwise |
| `rfc-change-ok` | no | The reason a commit changes an RFC-tagged test unit the owner has not approved yet. Instead of refusing, the message carries one `RFC-approval-owed: <package>.<TestName>: <reason>` trailer per unapproved unit, and the push derivation holds the push until the owner answers |
<!-- source: internal/le/commit/actions.go -- createParameters, createPolicy -->
<!-- source: internal/le/commit/prepare.go -- refuseAttestAdmission -->

Issuing an `attest` requires the owner's interactive terminal on both stdin and
stdout, including for `dry-run`. A pipe or an unanswered terminal check refuses
before a session directory, message or script is written. Reading an existing
`Owner-attested:` trailer remains noninteractive.
<!-- source: internal/le/commit/prepare.go -- Create -->

`Message` in `internal/le/commit/input.go` enforces the subject limit and the
body wrap, and appends one `RFC-approved: <package>.<TestName>: <reason>`
trailer line, never wrapped, for each `ze-test-judge` approval the commit uses. The
approval is written once, by `./le rfc approve unit <package>.<TestName> reason
"<the judge's verdict>"`, into `tmp/commit-rfc-approved-<session>.md`; the
generated script drops the used rows from that file after `git commit`
succeeds, so the trailer in the commit is the only record
(`docs/contributing/rfc-implementation-guide.md`).

An approval-only publication names `rfc-approval` without files and uses the same
private-index publication route without changing the tree. It cannot combine
`attest`, `no-test`, `review-override` or `rfc-change-ok`. Selected targeted rows
carry their original judge reason as `RFC-approved: <unit>: <reason>` and a
same-carrier `RFC-approved-for: <fullSHA> <unit>` companion. Unselected rows stay
pending. Preparation and failed publication prune nothing; only successful
publication prunes the exact selected target rows.
Every CAS attempt checks the actual parent's reachable history for an already
published exact target/unit binding, so two independently prepared scripts cannot
spend one approval twice.

A judge-approved test weakening is carried the same way, after the RFC
approvals: one `Weakening-approved: <package>.<TestName>: <the judge's verdict>`
line per weakened test, never wrapped. `./le test weakened approve unit
<package>.<TestName> reason "<the judge's verdict>"` writes the approval into
`tmp/commit-weakening-approved-<session>.md`, and the script drops each used row
after `git commit` succeeds (`docs/contributing/testing.md`, "When a test must be
weakened"). A weakening no approval names refuses the commit, and no keyword
admits it.

Before every publication attempt, the script checks the exact ordinary RFC and
weakening verdict lines it captured against its actual parent's history. An
earlier block of the same script spends a verdict just like any other commit;
pruning its pending row does not renew it. This read-only check issues no
approval and does not reread pending rows as authority. Refusal leaves the
unpublished block's message and unrelated pending approvals intact.
The native read-only consumer is `./le commit approval-check parent <fullSHA>
verdict "<whole verdict line>"`; `verdict` repeats when the block selected
several ordinary approvals.
<!-- source: internal/le/commit/script.go -- renderPublish, renderEmptyPublish -->
<!-- source: internal/le/commit/prepare.go -- checkSourceGates, mergeRFCApprovals -->

A live weakening or RFC approval also enters the message when its existing
selected test unit changed without a structural finding, including a comment-only
edit. This selection tries weakening approvals first, then RFC approvals. An RFC
row selected here and by the RFC behavior gate produces one trailer and is
removed once. An edit to `//` comments in Go, or to whole-line `#` comments in
`.ci`, `.et` and `.py` files, that keeps every requirement tag ID leaves the RFC
behavior fingerprints unchanged, so only this selection carries its row. An
edit to a Go `/* */` comment or a trailing `#` comment, or one that drops or
renames a tag ID, is an RFC change. Either way the row is spent: a later change
to the same unit carries a verdict only through a fresh judge row, and needs
one when it weakens the unit or changes its RFC behavior.
An unchanged unit, an excluded hunk or another session's approval contributes
nothing. Preparation keeps the row. The successful script removes it, and a
verdict already carried by a commit cannot be used again. A new test function
or an entirely new test file does not spend a semantic approval.

Go units end at their matching body brace, including one-line functions. Adding
a separator or another declaration cannot spend an unchanged unit's approval.
Valid bodyless declarations end at their signature and do not hide later tests.
If an existing unit's boundary is incomplete, preparation refuses to guess it.
<!-- source: internal/le/test/weakened/testweakened.go -- Check, comparePath, claimFinding -->
<!-- source: internal/le/commit/script.go -- renderApprovalPrune -->

Each admission the gates grant is recorded the same way, as one trailer line
after the approvals (`Prepared.Trailers`):

| Admission | Trailer | Read by the push derivation |
|-----------|---------|-----------------------------|
| `review-override "<reason>"` on a closure with no clean review | `Review-owed: <spec stem>: <reason>` | yes, until a clean review, a recorded Review Gate or the owner's attestation answers it |
| `rfc-change-ok "<reason>"` | `RFC-approval-owed: <package>.<TestName>: <reason>`, one per unit | yes, until a unique same-carrier `RFC-approved:` plus `RFC-approved-for: <fullSHA> <unit>` binds the verdict to that exact change, or the owner's attestation answers it. An unbound `RFC-approved:` answers only its own carrier |
| Automatic when a selected summary adds or rewords `{linux-limit: LL-ID; reason}` | `Linux-limit-owed: <RID>: <LL-id>`, one per changed annotation | yes, until the owner's attestation of that commit; an RFC approval or review does not answer it |

Two trailers record a fact rather than an admission. A spec closure given
`no-news "<reason>"` carries `No-news: <reason>`, which `Create` writes from
`newsFragmentRefusal`'s answer directly before `Spec-work:`. The push
derivation owes nothing for it (`OwesNothing` in `declaredTrailers`), and a
hand-written `No-news:` line is refused like any declared key.

The other is the attribution. A native commit that
changes files, prepared while the session claims a spec (`./le spec claim`, or
the parent's claim for an agent that claimed none), ends with
`Spec-work: <spec stem>`. The action layer reads the claim through
`spec.ClaimedSpec`, keyed by the harness session and agent as the marker is,
and an unreadable claim refuses the commit rather than attributing nothing. An
attestation, a commit with no claim, and a commit outside the Ze checkout carry
none. The push derivation reads the line and owes nothing for it
(`OwesNothing` in `declaredTrailers`), so it never holds a push. A spec
closure reads it: the review artifact must cover every code file of every
commit carrying the closed spec's `Spec-work:` line, matched as a whole line,
plus the earlier blocks of the closure's own script (commit A, not yet
landed), plus the closure commit itself
(`docs/contributing/spec-workflow.md`, "What each closure gate reads"). A
commit made before the trailer existed carries none and is not searched for.
<!-- source: internal/le/commit/actions.go -- createAdmitted, declaredTrailers -->
<!-- source: internal/le/commit/review.go -- specWorkPopulation, scriptPaths -->

An admission no gate used records nothing. A reason spanning lines is refused,
because a trailer is one line. `Message` refuses a subject or a body line that
starts with any declared trailer key (`declaredTrailers` in
`internal/le/commit/actions.go`, which also lists `Owner-attested:`,
`Ledger-carried:`, `No-news:` and `Spec-work:`), naming the line and the one route that writes it: a
trailer is a record the derivation reads back, never the author's prose.

Linux-limit approval needs no admission keyword. `create` compares the selected
summary blobs with its recorded HEAD through the RFC summary parser and writes
the obligation automatically. An unchanged annotation in an edited summary owes
nothing; a new annotation, a different register ID or a reworded reason owes
the owner's approval. Removing the annotation creates no new obligation.
Whole-file snapshots and `hunks` selections are judged from the same immutable
blobs the script commits, not the shared index or unselected working-tree text.
Before parsing, `create` checks the Git modes of every `rfc/short/*.md` entry
the prospective commit retains, including untouched summaries from recorded
HEAD. Each must be a regular file (executable files are regular too), and its
`rfc` and `rfc/short` ancestors must be Git trees, not links or other files.
A link at either ancestor or at a summary is refused even when only its target
was selected: following it would let an annotation change outside the approval
selector. Remove the linked entry explicitly; a replacement summary must be a
regular file under real directories. Selected descendants establish their
prospective tree ancestors without consulting checkout modes. A regular
replacement owes approval for every Linux-limit annotation it carries; the
old link's text is not an approved summary baseline. Unselected checkout and
shared-index modes do not change this judgement.
The commit can land before approval, but its push waits. After the owner
approves the instance, prepare an empty attestation with
`./le commit create subject "<subject>" attest <commit> "<owner words>"`.
The attestation must be in the range the push publishes; one after the selected
push target cannot answer that push.
<!-- source: internal/le/commit/linux_limit.go -- linuxLimitTrailers, linuxLimitHead -->
<!-- source: internal/le/commit/owed_trailers.go -- readPersonActs, answering.answered -->

No admission exists for a red. A red prevents a release, never a commit
(`ai/META-DECISIONS.md`, "Ze is pre-release"), so `create` reads no failure
index and refuses nothing a test, lint, a structural check or a generated-file
check found, whether or not the commit caused it. Name each red you know of in
the body; it is owed at the release check. The two admissions the deleted
structural red check took, `structural-red-ok` and `broken-head-fix`, are
refused as retired keywords naming that replacement.
<!-- source: internal/le/commit/actions.go -- createParameters, redsInTheBody -->

## Committing some hunks of a shared file

Sessions share one checkout, so one file can hold hunks from two sessions, each
of which compiles only with its own session's other files. Neither session can
commit the whole file, and neither may take the other's hunks out of the
working tree. `hunks` commits yours alone:

```bash
dir=$(./le session scratch ensure)
git diff -- internal/le/verify/lifecycle.go > "$dir/lifecycle.patch"
# Delete the other session's hunks from the patch, then:
./le commit create subject "<subject>" file <your other files> \
  hunks internal/le/verify/lifecycle.go "$dir/lifecycle.patch"
```

`create` reads HEAD once, before it takes any content, and applies the patch to
that HEAD's blob in a private index with `git apply --cached`. The working tree
and the shared index are never written. That HEAD is the block's recorded
`PreparedHead`, so the script's existing check refuses the commit when a peer
changed the path after the blob was built. The blob is in the object database
from then on, and the script commits its `git ls-files -s` line as it commits a
`file` snapshot.

The patch is refused, naming the path and the fix, when it is empty, does not
apply to HEAD's blob, changes binary content (commit it whole with `file`),
changes nothing, changes another path as well, or deletes the path (use
`remove`). A hunk with no context line on a side where the file goes on, as
`git diff -U0` writes, is refused too, because `git apply` anchors a hunk with
no trailing context at the end of the file: save the patch with git's default
context. Git paths are read and written NUL-delimited. Spaces, quotes,
backslashes, non-ASCII and leading or trailing spaces retain their exact
identity; NUL, tab, carriage return and newline are refused. A `hunks` path is
validated with the `file` and `remove` paths, before the session directory or
anything else is written, so a refused path records nothing under `tmp/`.
<!-- source: internal/le/commit/hunks_select.go -- normalizeHunkPaths -->

A selected path whose entry is not a `git ls-files -s` line for that path is an
error at every gate that reads it, never a fall back to the working-tree file,
which holds hunks the commit does not carry. `no-test` does not waive it, and
a test file selected that way does not count as the commit's test: the
test-coverage gate reads it first.

These gates judge the selected blob, never the working-tree file, because each
reads a hunks path through `hunks.Selection.Read`: the weakening gate and the
prospective index it renames from, the RFC-tagged change gate, the automatic
Linux-limit approval obligation, the test-coverage gate, the journal shape check
and the owned-row guard (an owner entry or an open row that only the working tree
holds counts as absent), the spec-closure sides, and the review pin.

The review pin compares like with like. Record the review of a hunks code file
with the same pair, `./le spec review record spec <stem> verdict clean rounds
<n> hunks <path> <patch-file>`, which pins HEAD's blob with the patch applied.
A pin taken over the working-tree file is stale, and a hunks path the artifact
does not pin is unreviewed. Either way no review saw the content the commit
carries, and the only fix is a new review of that content. The refusal says
so, and names the verb the reviewer records the new verdict with as its usage
shape, this commit's stem and each stale or unreviewed hunks path with its
patch filled in, the reviewer's own words left as `...`:

```
./le spec review record spec <stem> ... hunks <path> <patch-file>
```

A record replaces the artifact rather than adding to it
(`internal/le/spec/review.go`, `recordReview`), so the new record names every
code file the commit carries, not only the paths the refusal named: each hunks
path with `hunks <path> <patch-file>` and every other one with `file <path>`.
A path the record leaves out is unreviewed at the next `./le commit create`.
Which paths count as code is `IsReviewCode`'s answer, the one list both the
recorder's check and this gate read. Markdown is never pinned, so a spec edited
under `plan/` after the review neither needs a pin nor makes one stale
(`docs/contributing/spec-workflow.md`, "What each closure gate reads").
<!-- source: internal/le/spec/review.go -- recordReview, IsReviewCode -->
<!-- source: internal/le/commit/review.go -- hunksReviewRefusal -->

The refusal prints no command that pins anything by itself. A printed re-pin
command was tried and removed: whatever it carried, pasting it vouched for
content no reviewer had read, which cleared the gate's own refusal.

Whole-file snapshots and hunk blobs are captured before content gates. Every
content gate reads those immutable blobs. The closure review reads one
prospective tree built from the recorded HEAD, earlier script blocks in order,
the current snapshots and hunks, then removals; historical `Spec-work` paths
never read unrelated concurrent checkout bytes.

The comparison baseline is the same retained tree after earlier blocks, not
necessarily a commit object. A readable tree with no such path is a known
absence; a missing revision or blob is an unreadable baseline and refuses
preparation, never a reason to treat an existing file as newly added.

The generated review check receives the originating artifact path and the actual
proposed tree inside every compare-and-swap attempt, including retries. It checks
hunks, modes and removals as well as whole-file content. An unavailable blob
refuses publication. Only the drift note leaves hunk paths out, because their
working-tree content differs intentionally. Verification is reported stale for
hunks because its manifest hashes working-tree files.

After the commit lands, the script's shared-index repair
(`renderSharedIndexRepair`) writes HEAD's entry for every named path, hunks
paths included, into the shared index, so the other session's hunks show as
unstaged changes against the new commit. `./le repo compiles check` after the
script judges what the commit holds, as for any commit.
<!-- source: internal/le/commit/hunks/hunks.go -- Apply, Selection -->
<!-- source: internal/le/commit/hunks_select.go -- selectHunks -->
<!-- source: internal/le/commit/review.go -- hunksReviewRefusal -->
<!-- source: internal/le/commit/prepare.go -- Create, checkVerificationGates -->
<!-- source: internal/le/commit/review.go -- closureStemSelected, checkReviewSelected -->
<!-- source: internal/le/spec/review.go -- recordedFile, ReviewFile -->
<!-- source: internal/le/commit/script.go -- renderSharedIndexRepair -->

## What the command refuses

A refusal's exit code says which kind it is. `create`, `message` and `session`
answer 2 when they refuse the keywords the line gave (`append` with `replace`,
no file named, a malformed session id, a subject the message rules refuse), 3
when the operation could not run (no HEAD to prepare against, a file a gate
must read is unreadable, the checkout is not found), and 1 when it ran and a
gate or git refused it. `check` answers 1 for a review that is not
clean. The generated script runs under `set -e`, so any non-zero code stops it.

<!-- source: internal/le/commit/exitcode.go -- exitCodeOf, refusedKeywords, couldNotRun -->
<!-- source: internal/le/commit/actions.go -- prepareCreate, createAdmitted, answerSession, answerReviewCheck -->

`normalizePath` and `validateAddPath` (`internal/le/commit/input.go`) refuse a
path before the script is written:

- A path outside the repository, or a `..` component.
- Anything under `.git/`.
- A root agent instruction file: the generated `AGENTS.md`, and `CLAUDE.md`, which must not exist.
- A path `git check-ignore` matches. The index is consulted, so a TRACKED file
  that matches an ignore pattern is committable: git already carries it, and the
  pattern governs what is added under that path next.
- A path that does not exist. Use `remove` for a tracked deletion. For a
  relocation, write the destination and leave the source in place; name the old
  path with `remove` and the new one with `file`, because `git mv` stages.
  The same spec filename carried into a different category is a relocation,
  not closure, including migration from the old root, `immediate`,
  `pre-release`, or `to-review` layout into the current categories.
  A changed filename is a rename, not closure, only when exactly one added
  spec absent at HEAD has text equal to the old spec at HEAD with every
  occurrence of the old stem replaced by the new stem. Pairing is one to one.
  Other edits in a stem rename make the removal a closure: rename first and
  edit later (`renamedSpecs`, `docs/contributing/spec-workflow.md`).
<!-- source: internal/le/commit/review.go -- renamedSpecs -->

- A directory. Scripts stage explicit files.

`validateRemovePath` refuses a `remove` path that is not tracked, so you never
have to run `git ls-files --error-unmatch` yourself.

`rfcChangeProblems` (`internal/le/commit/rfcchange.go`) refuses a commit that
changes an RFC-tagged test unit no row in this session's
`tmp/commit-rfc-approved-<session>.md` names, and prints the `./le rfc approve`
command for each unit. `rfc-change-ok "<reason>"` admits the commit with one
`RFC-approval-owed:` trailer per unit instead; a gate that could not name the
units it judged is refused even then, because it has nothing to record. A row a landed commit already carries as a trailer approves
nothing further, and the next successful commit drops it from the file.

`dependencyRefusal` (`internal/le/commit/dependencies.go`) refuses a commit
whose Go adds a third-party import `dependency-ledger.txt` does not approve for
the importing directory, a ledger row no `./le deps approve` recorded, or the
ledger's removal. It names the package, the module, the importer and the
command Thomas runs (`docs/contributing/dependency-approval.md`).
<!-- source: internal/le/commit/dependencies.go -- dependencyRefusal -->

`changeCheckRefusal` (`internal/le/commit/changechecks.go`) runs right after
the dependency gate and runs every change-set check registered in the
`changecheck` seam (`internal/le/commit/changecheck`). The commit code names
no check: a check package calls `changecheck.Register` from its own `init()`
with its command name and a `Judge` over a `changecheck.Change`, and the
composition root links it. `Change.After` reads the carried selection, so a
`hunks` commit is judged on the blob it carries, and `Change.Before` reads the
tree the commit starts from. A check judges only what the commit adds and
never an instance its parent already held: a refusal names the check
(`./le arch operator-error check refused what this commit adds`) and carries
the check's own text, and a check that cannot judge answers exit 3, never a
pass. Two checks register today:

| Check | What it refuses in what the commit adds | Page |
|-------|------------------------------------------|------|
| `arch operator-error` | a plain `errors.New` or `fmt.Errorf` returned from a registered command handler, RPC handler or config verifier, and a `plugin.StatusError` answer whose `Error` text the handler wrote rather than built with `plugin.OperatorErrorResponse` | `docs/architecture/cli/error-surface.md`, "The operator error" |
| `arch doctor-coverage` | a `Warn` or `Error` log line or report-bus raise under `internal/component`, `internal/plugins` or `cmd/ze` that names no doctor code a registered doctor check declares, and a `diagnostic.Diagnostic` finding in shipped Go with no `Fix` | `docs/architecture/doctor-and-health-checks.md`, "Every problem site names a doctor code" |

<!-- source: internal/le/arch/operatorerror/report.go -- judgeChange -->
<!-- source: internal/le/arch/doctorcoverage/report.go -- judgeChange -->
<!-- source: internal/le/commit/changechecks.go -- changeCheckRefusal -->
<!-- source: internal/le/commit/changecheck/changecheck.go -- Register, Run -->

`newsFragmentRefusal` (`internal/le/commit/newsfragment.go`) runs right after
the Existing Mechanisms gate, in the Ze checkout only. A spec closure (the
commit that removes the spec, as `closedSpecStem` decides) is refused unless
the tree the commit produces holds `website/changes/fragments/<spec stem>.md`
written by this spec's work, or the closure gives `no-news "<reason>"`. The
refusal names the file to write, its front matter (`spec`, `topic`, optional
`feature` and `found-by`) and the six headings `## Before`, `## After`,
`## Corrects`, `## Exposure`, `## Upstream` and `## Asked by`, which
`newsHeadings` declares once; `website/changes/fragments/README.md` is the
format page and is never judged as a fragment. Carry the fragment in commit A
of the closure script: the gate reads the tree the commit produces, earlier
script blocks included, never the working tree, so a fragment left
uncommitted does not count. A fragment the gate cannot read answers exit 3,
never a pass.

"Written by this spec's work" means a commit carrying `Spec-work: <stem>`
touched the fragment, an earlier block of the script the closure appends to
carries it, or the closure commit itself does: the same population the closure
review covers. The closure's fragment must also name the stem in its `spec`
key. Four more refusals follow from that rule. A fragment at the stem's path
that no work of the spec touched is stale, left by an earlier spec of the same
name; the refusal names the path, and a closure that gives `no-news` instead
is prepared, since that text is not its news. A closure that carries its own
fragment and also gives `no-news` says two opposite things and is refused. A
fragment whose `spec` key names another spec is refused, naming the key to
set. `no-news` on a commit that closes no spec states nothing and is refused,
naming the keyword to drop; a spec relocation or rename is not a closure.

Every commit that carries a file under `website/changes/fragments/`, a closure
or not, has each one judged, so a fix outside a spec owes a valid fragment
too. One refusal names every fragment at fault and, for each, what is wrong
and the fix: a file that is not one `<name>.md` directly in the directory; a
missing or unclosed front matter, an unknown, repeated or empty key; neither or
both of `spec` and `journal`; no `topic`, or one that is not a tag family of
`website/data/topics.json`, read from the tree the commit produces, so a family
the same commit adds is accepted; a `found-by` outside the closed list
`newsFoundBy` declares, which the refusal lists; text before the first heading,
a heading that is missing, empty, unknown, repeated or out of order, and
`none` alone under `## Before` or `## After`. `none` is accepted under the
other four.
<!-- source: internal/le/commit/newsfragment.go -- newsFragmentRefusal, newsHeadings, newsFragmentOwed, newsFragmentWrittenBySpec, newsFragmentClosureProblem -->
<!-- source: internal/le/commit/newsfragment.go -- newsFragmentsCarried, parseNewsFragment, newsFoundBy -->
<!-- source: internal/le/site/topics/topics.go -- Parse -->

A list keyword buys one thing: a population too large to type stays explicit. It
broadens nothing else. Every line is validated as its own path, and the script
spells each one in its block marker and in its index entries. Write the list
with a command that answers what changed, then read it before you pass it:

```bash
cd ../gh-pages && git -c core.quotePath=false status --porcelain |
  grep -v '^ D' | sed 's/^...//' > "$dir/add.txt"
```

The command reports the verification certificate for the named file
population and never refuses a commit for a missing verification: no agent runs
any `./le verify` command, and GitHub CI verifies the pushed commit. Its
`verify=` line reads `FRESH (...)` when a record Thomas's own run left covers
those paths, and `CI (...)` otherwise; neither asks for a run. Its output also
carries one `debt=` line, the push debt read from the derivation below. When
nothing is owed it reads `debt=nothing owed`. Otherwise it carries the same
headline as the session-start warning, a count of the refusals and of the open
person acts (`debt=1 person act(s) owed`), followed by each reason the push is
refused, separated by `; `. It never reads as a count of zero and never names a
verify. `debt=cannot answer: <why>` means the derivation could not answer.
Nothing is written: no commit carries a debt file.
<!-- source: internal/le/commit/push.go -- debtReport, headline -->
<!-- source: internal/le/commit/verification.go -- VerificationState.label -->

The keywords `unverified` and `missing-full-verify-ok` were removed and are
refused saying nothing replaces them, because no commit or push owes a local
verify; `push` on `create` was removed and is refused naming
`./le commit push "<owner authorisation>"`. The verbs `debt-list`,
`debt-clear` and `debt-discharge` were removed too, refused naming
`./le commit debt-status`, nothing, or `create ... attest`.

## Pushing

`./le commit push "<owner authorisation>"` is the one push route, and only the
owner orders it (`ai/rules/git-safety.md`). It derives the push debt from git
and the commit trailers, and its push target P is HEAD. It reads no verify
record and never asks for a verify: no agent runs one, and GitHub CI verifies
the pushed commit. It is refused, exit 1, naming what clears each reason, when
nothing is ahead of the upstream, when the upstream does not fast-forward to P
(`git pull --rebase --no-autostash`, then `./le commit push`), when a commit
`P ^@{u}` publishes carries a trailer nobody can read, or while a person act is
open on one of those commits. It answers exit 3 when the derivation cannot
answer, such as a branch with no upstream
(`git branch --set-upstream-to=origin/<branch>`).
<!-- source: internal/le/commit/owed.go -- deriveDebt, chooseTarget, pushRefusal -->

Preparing this script also issues the owner's instruction: stdin and stdout
must both be terminals. A pipe or an unanswered terminal check refuses with
exit 2 before any session-directory or script write, including replacement of
an existing push script. Once prepared, the script remains agent-executable and
noninteractive; its debt re-check and exact push target are unchanged.
<!-- source: internal/le/commit/push.go -- preparePush, renderPushScript -->

When it admits, it writes a push-only script at the session's `script=` path
and prints that line and the commit it pushes. The script makes no commit. It
re-runs `./le commit debt-status commit <P>` and pushes nothing when that
fails (a moved upstream, a rebase that took P off the branch, a person act
now open), then runs `git push <remote> <P>:<upstream branch ref>`. It pushes P
by SHA, never the branch tip, so a commit that lands after the script was
prepared is not published. Outside the Ze checkout P is HEAD and the script
carries no re-check.

| Script already at the session's path | `./le commit push` | `create ... append` |
|--------------------------------------|--------------------|---------------------|
| commit blocks not yet run | refused: run that script first, then push | as before |
| a push-only script | prepared again in place, P chosen again | refused: an appended commit would not be published; run the push script first, or use `replace` |

`./le commit debt-status` answers exit 0 when P is HEAD and nothing is owed, 1
when anything is owed (listing it, P included), and 3 when it cannot answer
because the checkout, its upstream or git could not be read. `./le commit push`
answers 2 when it refuses the authorisation the line gave, 1 when the
derivation or a commit script with a block not yet run refuses the push, and
3 when it cannot prepare the push because the checkout, its upstream, a file
or git could not be read.
`debt-status commit <rev>` answers 0 when `<rev>` is an admissible push target
with no open item in `<rev> ^@{u}`, whatever follows it on the branch.
<!-- source: internal/le/commit/push.go -- preparePush, pushScriptPath, renderPushScript, answerDebtStatus -->

An owner attestation is `./le commit create subject "<subject>" attest <commit>
"<owner words>"` with no file. Its empty commit leaves HEAD's tree the green
tree, so the push that follows needs no new verification. The script builds it
on its parent's tree and refuses it if the diff names any path.
<!-- source: internal/le/commit/attest.go -- attestationTrailer, ledgerCarriedTrailers -->
<!-- source: internal/le/commit/script.go -- renderEmptyPublish -->

## Worked invocations

```bash
# Single commit, the common case:
./le commit create \
  replace \
  subject "hook: allow tee pipe, per-session log paths" \
  body "Explanation of why the change was made." \
  file internal/le/hookruntime/bash.go \
  file ai/rules/points/commands/<section>/<point>.md

# A second commit in the same script:
./le commit create \
  append \
  script tmp/commit-<session>-<tag>-<random>.sh \
  subject "feat: add widget support" \
  body "Implements widget rendering for the dashboard." \
  file internal/component/web/widget.go \
  file internal/component/web/widget_test.go

# Spec closure, removing the spec file:
./le commit create \
  append \
  script tmp/commit-<session>-<tag>-<random>.sh \
  subject "spec: close spec-widget" \
  remove plan/release/fixes/spec-widget.md \
  no-news "tooling: no operator-visible change"
```

Leave `no-news` off when commit A carried the spec's news fragment.

The `script=` line the command prints is the only authoritative path. Its name
carries a random suffix, so no guess reaches another agent's script. Copy it;
never construct it from the session id.

The `message=` line carries a random suffix for the same reason, so a script and
its message are one artifact. A second `create` under the same tag therefore
allocates a second message and cannot write over the first one's. Until
2026-09-05 it could, and the first script then made its commit under the second
one's subject with nothing printed to say so
(`plan/journal/pointer-shared-across-the-names-it-indexes.md`).
<!-- source: internal/le/commit/script.go -- nextTag, allocateMessage, allocateScript -->

## What the generated script contains

`renderBlock` (`internal/le/commit/script.go`) is the only native source that
spells the raw staging and commit verbs, and it emits them rather than running
them. One commit block holds, in order:

1. A `# Commit <tag>: <subject>` comment and a `# ze-commit-block:` marker
   naming the tag and every path.
2. The critical-review gate re-check, when the commit closes a spec.
3. Before the loop, three checks on the message and the signing setting, then
   the starting point. A missing message file is refused first: a block deletes
   its message once its commit lands, so a missing one on a rerun means the
   block already published or the file was cleaned up, and the refusal says to
   check `git log` for the subject and prepare whatever is still uncommitted
   again. The message is then passed through `git stripspace`, the cleanup
   `git commit -F` applies, and an empty result is refused, as `git commit`
   refuses it. `commit.gpgSign` is read with `git config --type=bool`: unset
   means unsigned, as with `git commit`, and a value git cannot read as a
   boolean is refused rather than read as unset. Last, `_ze_parent` is set to
   the block's recorded `PreparedHead`, the HEAD `create` read just before it
   took this block's snapshot. Each block records its own, so an appended block
   prepared later compares with its own moment.
   The loop starts here and runs to the swap in step 6. Each attempt keeps the
   HEAD it compares with in `_ze_seen`, then reads the parent ONCE:
   `_ze_parent=$(git rev-parse --verify 'HEAD^{commit}')`. On the first attempt
   `_ze_seen` is the recorded `PreparedHead`; on a later one it is the parent
   the attempt before it read. Before building on the parent, the block lists
   the commits between `_ze_seen` and the parent which change a path it names,
   leaving out, by exact SHA, the commits this script published itself
   (`_ze_own`), because an appended block may name a path an earlier block
   committed. Any left is a peer's change the snapshot never held, so the block
   refuses, names the paths, and asks for a fresh `create`: building on it would
   write content prepared before the change over it. Then
   `git read-tree "$_ze_parent"` into `<script-path-without-.sh>.index`, the
   private index this block commits from. Steps 4 to 6 run inside the same
   loop, and a refused swap in step 6 whose HEAD moved sends it round again.
4. `git update-index --index-info` with one `git ls-files -s` line per path,
   written by `snapshotIndexEntries` when `create` ran. The line names the blob,
   so the content is fixed at preparation time. A blob a `git gc --prune` has
   collected since is refused here, naming every path whose content is gone.
5. For any `remove` paths, their `git ls-files -s` lines captured from the
   private index into `_ze_removed`, then `git update-index --force-remove`.
   The captured lines are the entries the parent holds, mode included, and step
   8 compares against them.
6. The publish (`renderPublish`). `git write-tree`, then `git commit-tree -p
   "$_ze_parent"` with the stripped message. A failure there, signing as a
   rule, is refused with HEAD untouched. `git commit` is not used, because it reads HEAD a
   second time for the parent: a peer's commit landing between the two reads
   became the parent of a tree that never held the peer's work, and every path
   the block did not name was silently reverted (450f9f1ee0 and d621be6677 on
   2026-10-14). `git commit-tree` ignores `commit.gpgSign`, so the script reads
   it and passes `-S`, which signs with `user.signingKey` as `git commit` does.
   No git hook runs; none is installed. The commit is then checked against its
   parent: it MUST change at least one path, and no path the block does not
   name, or the script refuses with HEAD untouched. Last, `git update-ref -m
   "commit: <subject>" HEAD <commit> <parent>` moves the branch only if it still
   names that parent. When the swap fails, HEAD is read again. If a peer moved
   it, the block goes round again on the new HEAD, up to 5 times, and step 3's
   check refuses if that peer changed a named path. If HEAD did not move, git
   refused for another reason, a stale ref lock or a permission, so the script
   stops with git's own error rather than retrying and reporting a race that
   never happened. When git names a lock file, the refusal names it too, and
   says to run the script again once the other git command finishes, removing
   the lock only when no git process is running: git waited only
   `core.filesRefLockTimeout`, so the holder may be a live git that is slow.
   Every refusal before the branch moves opens `ERROR: commit <tag> (<subject>) was not published`,
   with one exception: a block whose message file is gone opens `ERROR: commit
   <tag> (<subject>) was not run`, because a missing file means the block most
   likely published on an earlier run of the script. Both lines say HEAD is
   unchanged by that block, never by the run, since an earlier block of the same
   script can have published in this run. The subject the refusal prints is
   quoted once as a whole, so no character in it is shell syntax.
   The landed commit is printed as `[<short-sha>] <subject>` with its stat.
7. A drift note, then the message file is deleted. The note stages the working
   tree into a throwaway index and reports any named path, other than a `hunks`
   path, whose content, mode,
   or existence moved since preparation. It never refuses: the commit is safe,
   and the difference is still in the working tree. `git update-index
   --refresh` looks like the tool for this and is not, twice over: it rewrites
   the entry it reports, and it refreshes the whole index rather than the
   pathspec. Git holds the message once the commit lands, and the file is what
   holds the block's tag: an automatic tag is taken while any message carries
   it, so a kept file would never free its letter and a session would run out
   after 26 commits. `set -e` keeps a failed commit's message, whose script
   still runs.
8. For a block with `remove` paths, the working-tree deletion
   (`renderWorkingTreeRemoval`). Each removed path's copy is staged into a
   throwaway index, and the copy is deleted only when its entry line is one of
   the lines step 5 captured. A copy whose content or mode differs, a copy Git
   cannot read or stage, a path HEAD never held, and a directory (a removed
   file replaced by one, or a submodule)
   each leave the path in place and print `NOTE: kept <path>: ...` on stderr,
   because that content is in no commit and somebody has to decide about it. A path already absent is
   skipped in silence. The step runs after the publish under `set -e`, so a
   failed commit deletes nothing, and a deleted copy is always one `git show`
   away. A spec closure therefore leaves no file behind: commit A carries the
   spec's final content with `file`, and commit B's `remove` finds the copy
   equal to it. A commit carrying RFC approvals then drops the rows it used
   from the session's approvals file.
9. `git ls-tree HEAD -- <paths> | git update-index --index-info`, and
   `git update-index --force-remove` for the `remove` paths, which point the
   SHARED index at what was just committed, then the private index is deleted.
   Without it every other session reads those paths as staged changes of yours.
   It is the block's last step because it is the one that waits on another
   process: while git refuses over a `.git/index.lock` that a peer holds, it
   retries once a second for up to 30 seconds. Any other failure, or a lock
   still held after that, stops the script with the commit landed and prints
   `ERROR: commit <sha> landed, but the shared index could not be pointed at
   it.`, the paths left stale, and the exact commands to run from the checkout
   once the lock is released. A later block in the same script has not run.
<!-- source: internal/le/commit/script.go -- renderPublish, renderPrivateIndex, renderLostBlobs, publishAttemptsMax, renderWorkingTreeRemoval, renderSharedIndexRepair, indexLockWaitSecondsMax -->

The script opens with `set -euo pipefail` and a `cd` to the checkout it was
PREPARED for, named as an absolute path, so a failed step stops it and a script
prepared for one tree never stages in another. A commit script never pushes:
a push is a script of its own (Pushing, above).

The message file is read when the SCRIPT runs, not when `create` wrote it. So
repairing the message file in place before you run the script is the correct fix
for a bad message, and it is allowed.

## Never put a backtick in an argument

`./le commit create` is invoked from Bash, so `subject`, `body` and every
override reason are shell words before the command sees them. Inside a
double-quoted argument a backtick opens command substitution. A body reading
``the block declares `encoder json` `` runs `encoder json`, prints
`encoder: command not found` to stderr, and substitutes the EMPTY STRING into
the message.

The failure is silent where it matters: the command still writes its message
file and prints `script=`, so the tail of the output reads as success while the
sentence is already mutilated. Quote code in a commit message with plain double
quotes, or name the thing without quoting it.

Two habits make it self-checking. Read the generated message file before running
the script, at the path on the `message=` line. And treat `command not found`
anywhere in the output as a failed invocation rather than noise.

## Committing a sibling checkout

`ZE_REPO_ROOT` points the command at another tree, which is how the published
site and the wiki are committed from the Ze checkout:

```bash
ZE_REPO_ROOT=../gh-pages ./le commit create replace \
  subject "site: republish the generated tree" \
  file-list "$dir/add.txt" remove-list "$dir/del.txt"
```

The gates that read Ze's own sources do not run there, and `lepath.IsCheckout`
decides it: a tree with no `go.mod` beside `feature-gates.txt` is not the Ze
checkout. It holds no test ledger, no discovery index, no verification
certificate and no `plan/` specs, so each of those gates would refuse the commit
for want of a file that tree never had, or read a path that means something else
in it. `verify=NOT-APPLICABLE` says so in the output, nothing is derived, and
`./le commit push` there prepares a push of HEAD with no re-check.

Path validation, the message contract, the private index the script commits
from, and the push authorisation apply in every tree.

## After the script runs

For a commit carrying Go, module, or vendor paths, run
`./le repo compiles check` immediately afterwards. It judges the
commit you just made, which no run before that commit could see.

Report the commit SHA, the included files, the message file, the script path,
the push status, and the verification evidence or the skip reason.

## When a commit fails

A failed commit leaves the SHARED index exactly as it found it. The staging
happens in the block's private index, and the shared one is written only after
the branch holds the commit, so a failure really is "nothing happened". Fix the
cause and run the script again.

A signing failure is the usual trigger, because it fails LAST, after every gate
has passed.

On `gpg failed to sign` or `cannot open /dev/tty`, ask the user to run
`! echo test | gpg --clearsign` to unlock the agent, then re-run the script.

## Rebasing onto a diverged main

A rebase of local commits onto a diverged `origin/main` can re-conflict on a
generated file git still tracks. The rendered rules and
`docs/features/test-health.md` are the ones to expect. Rewrite the conflicted
file with its own generator at each rebase stop and continue. A DERIVED artifact
is never one of them: it is untracked, so a rebase cannot conflict on it. Which
files those are is `derived.All`'s answer rather than a list here, and
`./le ai hooks session-start` prints each one it builds. The RFC ledger and
its shards left git on 2026-09-11 and joined that set. Finish the rebase before repairing
bookkeeping, never mid-rebase, then recompute any derived ratchet the rebase
loosened.

`git rebase --continue` refuses with a misleading "You must edit all merge
conflicts" whenever there are unstaged tracked changes, not only when index
entries are unmerged: `ACTION_CONTINUE` in git's `builtin/rebase.c` checks
`has_unstaged_changes()`. Read `git status` for the unstaged tracked files and
stage or discard them. The message names conflicts you do not have.

Thomas's standing request, October 1, 2026:

> do a git pull --rebase and make sure we do this before first edit on any new sessions

On October 3, 2026 Thomas narrowed it to its purpose: a session never edits on
top of a branch that is behind its upstream. This checkout is shared and always
dirty with other sessions' work, so demanding a clean pull blocked every session
even when origin had nothing new.

Before its first edit an agent runs `git fetch` and `git rev-list --count
HEAD..@{u}` under `ai/rules/git-safety.md`, "Never edit behind the upstream". A
count of zero needs no pull, and a dirty tree does not stop it. A failed fetch
or a missing upstream stops the session, because the check has no answer. Only a
count above zero owes the same-branch `git pull --rebase --no-autostash`, and
only then do a dirty tree, a merge or rebase in progress, or a failed pull stop
the session. That standing authorization does not authorize a direct `git rebase` or
integrating a worktree branch. If the pull conflicts, stop and report it;
conflict resolution requires the user's direction.
