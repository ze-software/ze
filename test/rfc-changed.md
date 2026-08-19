# RFC-tagged tests this commit changes

Each row records one owner approval for a change to a test that carries an
`RFC requirement:` tag. Such a test is the proof behind a public compliance
claim in `docs/features/rfc-status.md`, and `make ze-rfc-check` counts it as
that proof. `ai/rules/testing.md` refuses every behavior change to one that the
owner did not approve. This file is the record of the approvals the owner gave.

One gate reads it. `scripts/dev/commit_helper.py` recomputes which tagged tests
the commit's own paths change, and refuses a commit whose changes this file does
not name.

That gate calls `_rfc_tagged_change_err` (`.claude/hooks/pretool-writeedit.py`),
which is the function the edit-time hook calls, so the two cannot disagree about
what counts as a behavior change. A reformat, a comment edit and a Go
import-only edit are not one. A rename is.

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

`scripts/dev/commit_helper.py` refuses a commit that changes a tagged test and
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

**The gate still accepts one, and that acceptance is temporary.**
`.claude/hooks/pretool-writeedit.py` still demands the marker at edit time, so a
gate that refused it would refuse every author for obeying the other gate. A
commit whose only record is a marker therefore passes, and says the marker is
superseded.

Write the row as well while both mechanisms are live. When the hook stops
demanding the marker, the marker branch of `rfc_changed_findings`
(`scripts/dev/commit_helper.py`) goes with it, and the markers leave the tree
in one sweep.

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
is the price of a gate that fails toward asking. `scripts/dev/check_weakened_tests.py`
reported two such false findings on 2026-08-19, one for a helper moved to a
sibling file and one for an error check extracted into a `t.Helper()` that also
added an assertion. Both are the first row of this table.

## The test name

| Carrier | The name |
|---------|----------|
| Go, inside a top-level func | the enclosing `func TestXxx` |
| Go, with a tag no top-level func encloses | the file stem |
| `.ci`, `.et`, an interop `check.py` | the file stem, because each such file is one test |

`scripts/dev/rfc_tagged_scope.py` resolves each one, and it is the same resolver
`test/weakened.md` names. `go_func_units` returns the top-level functions of a Go
file with their names. `scope_reader` treats a file that is not Go as one unit.

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
| TestRFC8955TrafficActionUnusedBitsZero | Thomas ruled on 2026-08-19 that `bogus` should not be supported. The test loops over action specs asserting `require.NoError`, then asserts the reserved octets of the Traffic Action Field are zero, which is what `RFC8955-7.3-1` gates. `bogus` sat in that list only because `parseFlowSpecAction` accepted any word: it asserted that a typo produces a real traffic-action community with S and T both clear rather than an error, so an operator writing `action termnial` got a community they never asked for on a live FlowSpec rule. That is the defect, not the requirement, and `ai/rules/rfc-compliance.md` calls a test pinning non-conformant behaviour the violation with a green bar on top. It is replaced by `none`, which `flowSpecTrafficActionFlags` (`internal/core/bgp/attribute/flowspec_encode.go`) accepts deliberately, because the decoder prints `traffic-action:none` when neither bit is set and a rendered community must stay re-configurable. The loop keeps four iterations and gains a fourth distinct bit pattern, `0x00`, the one case where the final octet must be entirely clear. The tag, the assertions, the loop body and the polarity are untouched; only the input word changed. The change also carries an in-file `rfc-test-change-approved:` marker, because the pre-write hook still demands one: that duplication is the transitional state this ledger's own prose describes, and it disappears when the hook message is rewritten. |
