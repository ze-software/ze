# RFC-tagged tests this commit changes

Each row records one owner approval for a change to a test that carries an
`RFC requirement:` tag. Such a test is the proof behind a public compliance
claim in `docs/features/rfc-status.md`, and `./le rfc check` counts it as
that proof. `ai/rules/testing.md` refuses every behavior change to one that the
owner did not approve. This file is the record of the approvals the owner gave.

Two native gates read it. The write hook in
`internal/le/hookruntime/writeedit.go` calls `weakened.Proposed` and refuses an
edit until this file names the test it changes. The commit gate in
`internal/le/commit/rfcchange.go` recomputes the tagged tests changed by the
prospective commit and refuses a missing or stale row.

Both use `internal/le/rfc/goscope.go` for tag carriers, function units, and
file-scope fallback, so they cannot disagree about what a row covers. A
reformat, a comment edit, and a Go import-only edit are not behavior changes. A
rename is.

The hook reads this file from disk, so the row is written BEFORE the edit. A row
written after the refusal buys nothing until the edit is made again.

A commit that changes no tagged test carries the table with no rows.

## Who writes the row

This is the one difference from `test/weakened.md`, and it decides everything
else. A weakening row is the author's own justification, and a reviewer reads it
to judge the author. A row here is the OWNER's decision, written down by the
author who asked for it.

An author cannot approve their own change. `ai/rules/testing.md` says it in one
line: a row in `test/weakened.md` does not authorize changing a tagged test,
because self-service justification is not user approval. A row here with no
answer from the owner behind it is a forgery, not a shortcut.

So the Reason column holds what the owner approved, not what the author wanted.

## The commit carries this file

`internal/le/commit/actions.go` refuses a commit that changes a tagged test and
does not name `test/rfc-changed.md` in its own `--file` list. A row that stays in
the working tree records nothing. The approval must sit in git history beside the
change it authorizes, because that is the only place a later reader can find it.

## The file is replaced per commit

Delete the rows of the last commit. Write the rows of this one. This file never
accumulates.

The reason is the mechanism it replaces, and `test/weakened.md` states it in
full: a justification explains one diff, and storing that record permanently is
what built the pile. The pile here is 255 `rfc-test-change-approved:` comments
across 120 test files. Nobody can read 255 approvals, so nobody reads them, so
writing one costs nothing.

Git history holds every past entry. `git log -p -- test/rfc-changed.md` shows the
rows of any commit beside the change they approved.

## The in-file marker is the old mechanism

`// rfc-test-change-approved: <date> <what and why>` is the record this file
replaces. It is a comment, so it stays in the test file after the diff it
explains is gone.

**No gate reads one.** The hook demanded a marker until 2026-08-19, and the
commit gate accepted one while it did, because a gate refusing it would have
refused every author for obeying the other gate. The hook reads this file now,
so both acceptances are gone: a marker approves nothing, and writing a new one
records nothing.

The sweep landed on 2026-08-19: 268 markers across 125 files, and 27 `test-relax:`
comments beside them, the mechanism `test/weakened.md` had already replaced. No
test carrier holds either token now.

The sweep is worth one paragraph, because deleting a retired token is not the
same as deleting what it said. Those 268 markers were 1475 lines of prose, and
about one block in six stated a fact about its own test that exists nowhere else:
a measured vacuity finding, a fixture precondition, a pointer to where coverage
moved. 57 of them survive as ordinary comments with the approval framing removed.
A mechanism can be retired without throwing away what people wrote under it.

## What this gate cannot see

The comparison is textual, and it is made against HEAD after comments and
whitespace are removed. Read the "what this gate saw" line in a refusal before
you write the row. The gate can be wrong, and the row is where you say so.

| The change | What the gate reports | Why |
|------------|-----------------------|-----|
| An assertion moved into a `t.Helper()` outside the tagged func | the tagged test changed | the scope cannot follow a call, which `tag_scope` records as its own known limit |
| The tagged test moved to a sibling file in the same commit | the test is gone from the old file | each path is compared against its own HEAD text, and no rename detection runs |
| The tagged test renamed | the old name is gone and nothing replaces it | a name is code, and telling a rename from a rewrite needs a Go parser |
| An assertion ADDED to a tagged test | the tagged test changed | any behavior change to the evidence needs the owner, and stronger is still different |
| A reformat, a comment edit, a Go import-only edit | nothing | the detector removes comments and whitespace before it compares |

The first three cost an author one row explaining that the gate was wrong. That
is the price of a gate that fails toward asking. `internal/le/testweakened/actions.go`
reported two such false findings on 2026-08-19, one for a helper moved to a
sibling file and one for an error check extracted into a `t.Helper()` that also
added an assertion. Both are the first row of this table.

## The test name

| Carrier | The name |
|---------|----------|
| Go, inside a top-level func | the enclosing `func TestXxx` |
| Go, with a tag no top-level func encloses | the file stem |
| `.ci` or `.et` | the file stem, because each such file is one test |

`internal/le/rfc/goscope.go` resolves each one, and the same resolver serves
`test/weakened.md`. `FunctionUnits` returns top-level Go functions with their
names, while `ScopeReader` treats every non-Go carrier as one file-scoped unit.

Row two is `tag_scope`'s own fallback. A tag can sit outside every function span:
a hoisted table, or a tag separated from its func by a blank line. The gate then
treats the whole file as the unit, because a narrower answer would be a guess.
The stem is the only name available, so a change in `a_test.go` is written as
`a_test`.

A bare name is accepted when it resolves to exactly one changed test in the
commit. Write `package.TestName` when it does not, where the package is the
directory holding the file.

## The reason

Name what the owner approved, and say why the tagged requirement is still
proven after the change. A reason that does not answer the second question
approves a compliance claim losing its evidence.

Every requirement id the gate attributes to a name is covered by that name's one
row. Quote the id, so a reader can open `rfc/short/` beside it.

| Test | Reason |
|------|--------|
| TestAuthX509UsesMethod14 | Thomas approved this change on 2026-08-30, in answer to a question naming this test and this violation. The test ran with `sa.RemoteHashAlgos` nil and asserted AUTH method 14, which is the exact state RFC 7427 Section 3 forbids: "the peer is only allowed to use this authentication method if the Notify payload of type SIGNATURE_HASH_ALGORITHMS has been sent and received by each peer." `computeX509Auth` now refuses the method on an empty `sa.RemoteHashAlgos`, so the old fixture pinned the defect rather than the requirement. The change is one added line, `sa.RemoteHashAlgos = []uint16{hashAlgoSHA2256}`, which puts the SA in the state the RFC permits; the assertion is untouched. The tag moved from RFC7427-4-1 to RFC7427-3-4 in the same edit, because the summary row this test proves was itself corrected: RFC7427-4-1 had restated Section 3 as an obligation and cited Section 4, and it now carries Section 4's own MUST ("a peer MUST pick one algorithm sent by the other peer"), while Section 3's conditional permission became the new row RFC7427-3-4. RFC7427-3-4 is therefore proven positive by the same assertion this test always made, that certificate authentication emits `wire.AuthMethodDigitalSig`, now reached through a permitted path instead of a forbidden one. Its negative polarity is carried by `TestRFC7427DigitalSignatureNeedsTheNotify`, which asserts the refusal the old fixture made impossible to state. |
| TestAuthPSKCompute | Thomas approved the correction of the RFC7427-4-1 summary row on 2026-08-30, in the same answer that approved `TestAuthX509UsesMethod14`: it commissioned correcting the row that misstates the RFC and extracting Section 4's own MUST. Re-homing this test's tag is the mechanical consequence of that correction, not a separate decision. `internal/le/rfc` anchors a requirement id to the section it cites and refuses `RFC7427-4-1` beside a `(§3)` citation, so Section 3's conditional permission had to become a new id, `RFC7427-3-4`, and `RFC7427-4-1` had to take Section 4's text ("a peer MUST pick one hash algorithm sent by the other peer"). This test asserts that pre-shared-key authentication emits AUTH method 2, which is evidence about WHICH method may be used and not about which hash algorithm a signer picks, so its tag moved with the requirement it was always proving. The edit is the tag id alone, `RFC7427-4-1 negative` to `RFC7427-3-4 negative`: no assertion, fixture, or line of test code changed, and the gate reports the function because a tag id is compliance evidence rather than a comment. RFC7427-3-4 keeps this negative and gains `TestRFC7427DigitalSignatureNeedsTheNotify`, which asserts the refusal directly. RFC7427-4-1 loses a carrier that never exercised it and gains two that do, `TestRFC7427SignatureAlgorithmIsOneThePeerSent` and `TestRFC7427AuthRefusesAnAlgorithmThePeerDidNotSend`, both polarities. |
