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
| TestRSVPSenderTemplateReservedZeroOnSend | Thomas commissioned this repair on 2026-08-29, after an audit mutation-proved the test vacuous: it read `buf[8]` and `buf[9]` out of a freshly zeroed `make([]byte, 32)`, so deleting `buf[8] = 0; buf[9] = 0` from `encodeSenderTemplate` (`internal/plugins/rsvpte/wire.go`) left it green. The change ADDS assertions and REMOVES none: the buffer is pre-filled with 0xAA before the call, the same reserved-field check now runs against a buffer that held non-zero bytes, and the full 12-octet object plus the untouched tail are pinned. RFC3209-4.6.2-1 keeps its one positive tag and its `{single-polarity}` annotation; the requirement is now proven rather than assumed, because the deletion above reddens the test. |
| TestLSPTableAllocateSkipsReservedLabels | Thomas commissioned this repair on 2026-08-29, after an audit mutation-proved the test vacuous for the line its annotation cites: 100 allocations from a fresh table never reach the wrap, so rewriting `t.nextLabel = firstDynamicLabel` to `t.nextLabel = 3` in `AllocateLabel` (`internal/plugins/rsvpte/fsm.go`) left it green. The change ADDS assertions and REMOVES none: the original 100-allocation floor check is unchanged, and a second table is driven to `MaxLabel` so the wrap itself executes and the label it returns after the wrap is checked. RFC3209-4.1-3 keeps its one positive tag and its `{single-polarity}` annotation, whose justification names that wrap; the mutation above now reddens the test. |
| TestEncodeUserPasswordMultiBlock | Thomas commissioned this repair on 2026-08-29, after an audit mutation-proved the test vacuous: it asserted only `len(encoded) == 32` while its tag claimed the RFC 2865 Section 5.2 chain, so keying every block on MD5(S+RA) instead of MD5(S+c(i-1)) in `EncodeUserPassword` (`internal/component/radius/attr.go`) left it green. The change ADDS assertions and REMOVES none: the length check stays, and the test now computes b1 = MD5(S+RA) and b2 = MD5(S+c(1)) itself and compares the whole 32-octet ciphertext byte for byte, so the second block's key is pinned to the first block's ciphertext. RFC2865-5.2-1 keeps both polarities and RFC2865-5.2-2 keeps the padding tag on the same function; the mutation above now reddens the test. |
| TestRFC2328ZeroLSChecksumRejected | Thomas commissioned this repair on 2026-08-29, after an audit found the negative weak: zeroing the LS Checksum field also breaks the Fletcher sums, so the LSA was rejected for mismatch and deleting the explicit zero guard in `FletcherVerify` (`internal/plugins/ospf/types/checksum.go`) left the package green. The change ADDS assertions and REMOVES none: the original zeroed-checksum case is unchanged, and a second LSA is crafted whose LS Checksum field is zero AND whose covered Fletcher sums are both zero, verified by a sum the test computes itself, so only the zero rule can reject it. RFC2328-12.1.7-1 keeps its negative tag; deleting the zero guard now reddens the test. |
| TestOSPFLSAChecksum | Thomas commissioned this repair on 2026-08-29. The positive proved the checksum only through `VerifyLSAChecksum`, which shares `fletcherModulus` with the generator, so a modulus wrong in both would pass while the wire bytes disagreed with every other OSPF implementation. The change ADDS assertions and REMOVES none: the backfilled-non-zero and verify checks are unchanged, and the test now sums the covered region mod 255 itself, from the RFC 905 Annex B definition, and requires both sums zero. RFC2328-12.1.7-1 and RFC905-x-6 and RFC905-x-7 keep the same positive tags on this function; the oracle no longer comes from the code under test. Measured on 2026-08-29: setting `fletcherModulus` to 254, which leaves the generator and the verifier agreeing with each other, left every earlier assertion in this function green and reddens the new one. |
| TestRFC2865RequestAuthenticatorRandom | Thomas commissioned this repair on 2026-08-29, after an audit found the positive weak: it proved 16 octets that are not a fixed constant, so replacing `rand.Read` with a 1-byte counter in `RandomAuthenticator` (`internal/component/radius/packet.go`) left the package green. The change ADDS assertions and REMOVES none: the width check and the two-values-differ check are unchanged, and the test then substitutes `crypto/rand.Reader` with a recognisable stub and requires the returned authenticator to be exactly the stub's 16 octets, which is the structural half of "cryptographically random" a unit test can honestly prove. RFC2865-3-2 keeps its one positive tag and its `{single-polarity}` annotation; the counter mutation now reddens the test. |
