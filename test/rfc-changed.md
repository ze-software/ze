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
| TestAnnounceStripsLocalPrefTowardExternalPeer | Thomas approved this on 2026-09-05, in answer to the change put to him with the measurement beside it: "ok". The RFC 8669 Section 8 fix for the announce rail needs one bool on `announceBuildKey` and one on the parameter list of `buildBatchAnnounceUpdate`, and no signature-preserving shape is honest: a variadic or a defaulted wrapper makes an RFC decision silently, and a post-build strip re-adds the memmove the announce writer exists to remove. The edit to this test is TWO call sites and NO assertion: `..., 65000)` becomes `..., 65000, false /*propagatePrefixSID*/)` at the `batchRail` helper and at the builder-supplied subtest. `RFC4271-5.1.5-1 negative` and `RFC4271-5.1.5-2 positive` are still proven, and the reason is that neither is asserted by the lines that changed: both rest on `findPathAttr(..., AttrLocalPref)` and on the `attrCodes` equality, which are byte-identical in the diff, and on the byte equality against the queued rail, which is untouched. The new argument is `false` at every existing call site, so no frame in this test changes: `false` is the default of `propagate-srv6-prefix-sid`, and none of these announces carries a Prefix-SID for it to act on. MEASURED BOTH WAYS on this tree, with the widened call in place: green as committed, and RED when `localPrefAllowed := localPrefAllowedTo(isIBGP)` is replaced by `localPrefAllowed := true` (`--- FAIL: TestAnnounceStripsLocalPrefTowardExternalPeer/external-peer-must-not-carry-local-pref`, message "RFC 4271 Section 5.1.5: an external peer MUST NOT be sent LOCAL_PREF"). The break was reverted immediately. |
| zzprobe_prefixsid_announce_test | Thomas approved the rename of this probe file on 2026-09-05, in the same answer. The file is EMPTIED here, not deleted: the pretool-bash test-deletion gate refuses `rm` on a `_test.go` path and has no environment escape, and two copies of one test function do not compile. Its whole content, `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`, moved intact to `internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go`, which names the code it covers, and two positive subtests were ADDED there: a configured external peer keeps attribute 40 byte for byte, and an internal peer's frame does not depend on the leaf. No assertion was dropped. The gate reads this file as a carrier of `RFC4271-5.1.5-1` because its header PROSE quotes that id while explaining why the fix was blocked; the file carried no `RFC requirement:` tag of its own and proved nothing about RFC 4271, which its own header stated. `TestAnnounceStripsLocalPrefTowardExternalPeer` (`reactor_api_origin_test.go`) is where RFC4271-5.1.5-1 and -2 are proven, and the row above records what happened to it. The owner deletes the empty file with `rm internal/component/bgp/reactor/zzprobe_prefixsid_announce_test.go`. |
| reactor_api_batch_attr_order_test | Thomas approved this on 2026-09-05, in answer to the change put to him with the measurement beside it: "ok". The approval is for threading one bool onto announceBuildKey and onto the parameter list of buildBatchAnnounceUpdate; he was shown TestAnnounceStripsLocalPrefTowardExternalPeer as the tagged test it edits, and the same mechanical widening reaches every call site of that function, which is what this row covers. The edit here is one trailing argument per call, `..., 65000)` becoming `..., 65000, false /*propagatePrefixSID*/)`, and NO assertion, name, table or expected value changed. The requirement this file carries is `RFC4271-5-7 positive`, ascending attribute type-code order, and it is still proven because what asserts it is untouched: the `assert.Equal` over the emitted code sequence in each ordering case, and the byte equality between the batch rail and buildRIBRouteUpdate. The new argument is `false` at every call, the default of propagate-srv6-prefix-sid, and no announce in this file carries a Prefix-SID for the new drop to act on, so every frame it compares is byte-identical to the one it compared at HEAD. Measured: the file's tests pass here, and the whole reactor package was run under `./le test-unit bgp`. |
