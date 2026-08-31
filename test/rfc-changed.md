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
| checkNoFamilyEndOfRIB | Thomas approved `plan/spec-restore-bespoke-interop-assertions.md` on 2026-08-31, scope settled at all fifteen bespoke checkers with no trimming, and its AC-2 names this function. The change ADDS assertions and removes none. The body was one line, `checkScenario(ctx, check, "no-family-peer-eor-frr")`, and `checkScenario` (`internal/le/interoplab/bgp/check_engine.go:66`) answers `scenario %s has no typed assertions` for every name absent from `scenarioOperations`. This name is absent by design, because `TestCheckerPopulationMatchesProducer` refuses a bespoke checker that also holds a generic fallback, so RFC4724-4-1 had never executed one line of its interop evidence. The restored body waits for the FRR session, requires FRR to hold 10.10.0.0/24 so the marker is not a barrier over an empty conversation, requires FRR to report the IPv4-unicast Multiprotocol capability as advertised by ITSELF and received from nobody, and then polls FRR's own `rcvd End-of-RIB for IPv4 Unicast from <ze>` decode in the FRR container log file. RFC4724-4-1 is proven for the first time rather than still proven. |
| checkISISDynamicHostname | Thomas approved the same spec on the same day, and its AC-2 names this function. The same always-firing `checkScenario` guard stood in front of the body, so RFC5301-3-4 and RFC5301-3-6 had no executing interop evidence at all. The restored body waits for the point-to-point IS-IS adjacency to reach Up, then requires FRR's IS-IS database to render an LSP whose identity half is ze's dynamic hostname `ze-p2p` rather than its system ID, which FRR prints only after decoding TLV 137 (RFC5301-3-4) and only after reading the whole length-framed value (RFC5301-3-6). Both requirements gain evidence; neither loses any. |
| checkNSSADefault | Thomas approved the same spec on the same day, and its AC-2 names this function. RFC3101-2.4-5 was bound to a body that was one `checkScenario` call, so its interop evidence had never run. The restored body waits for the OSPF adjacency to reach Full, which the N-bit options must match to allow, then requires FRR to install 0.0.0.0/0 via the NSSA border router on the SAME route line, from a `ze.conf` that carries no `default-originate` leaf, and finally requires no Type 5 AS-external LSA in the area with the database heading as positive proof the query ran. RFC3101-2.4-5 gains its first executed evidence. |
