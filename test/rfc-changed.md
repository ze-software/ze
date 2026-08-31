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
| ------ | ------ |
| TestCoAListenerInvalidAuth | Thomas approved this on 2026-08-31, after asking whether the change reduces RFC compliance and reading the evidence that it does not. Ten tagged tests each gain ONE line, `AllowedSources: coaLoopbackSources()`, in their `newCoAListener` literal. No assertion, expected value, quoted RFC sentence or test name changes. The reason is a fix in the same commit: `coaListener.isAllowedSource` read an EMPTY allow list as "allow every source", and every fixture in the package built a listener with none, so all ten had been reaching the handler THROUGH the hole the commit closes. Naming loopback is what makes them cross the source filter instead of bypassing it. This row's ids are `RFC5176-3.5-1 negative` and `RFC5176-3.5-2 positive`. The risk Thomas asked about is specific and was measured: a source filter can only SUPPRESS a reply, so it can only create a false pass in a test asserting "no response". All four such tests were forced RED by breaking their OWN guard -- this one by disabling `radius.VerifyCoARequestAuth` -- so none passes because of the new filter. Tags go 12 to 14 in these files: `RFC5176-6.1-1` is added in both polarities and no id is removed or reworded. |
| TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired | Same approval, same one-line change, id `RFC5176-3.4-4 negative`. Forced RED by disabling the `!hasMessageAuthenticator && cl.cfg.RequireMessageAuthenticator` branch, so its discard is still the require-gate's and not the source filter's. |
| TestCoAListenerUnknownSession | Same approval, same one-line change, id `RFC5176-3.3-1`. It asserts a CoA-NAK is RETURNED, so the source filter cannot produce its pass: a filter suppresses replies and never manufactures one. |
| TestDisconnectReplayReturnsCachedResponse | Same approval, same one-line change, id `RFC5176-2.3` duplicate detection. It asserts a returned cached response, so the same immunity applies. |
| TestRFC5176NoSessionIdNotActedOn | Same approval, same one-line change, id `RFC5176-3.3-1 negative`. Asserts a returned NAK. |
| TestRFC5176ResponseAuthenticator | Same approval, same one-line change, id `RFC5176-3.5-3 positive`. Asserts a returned response whose authenticator verifies, so a suppressed reply would fail it rather than pass it. |
| TestRFC5176ListenerAcceptsConformantMessageAuthenticator | Same approval, same one-line change, id `RFC5176-3.4-3 positive`. Asserts a returned response. |
| TestRFC5176ListenerDiscardsWrongMessageAuthenticator | Same approval, same one-line change, id `RFC5176-3.4-3`. Forced RED by disabling `radius.VerifyCoAMessageAuthenticator`, so its discard is still that guard's. |
| TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault | Same approval, same one-line change, id `RFC5176-3.4-4 positive`. Asserts a returned response. |
| TestRFC5176WrongMessageAuthenticatorDiscardedWhenNotRequired | Same approval, same one-line change, id `RFC5176-3.4-3 negative`. Forced RED by the same `VerifyCoAMessageAuthenticator` break. |
