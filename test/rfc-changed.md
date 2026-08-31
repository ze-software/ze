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
| checkRFC7999Blackhole | Thomas approved `plan/spec-restore-bespoke-interop-assertions.md` on 2026-08-31, scope settled at all fifteen bespoke checkers with no trimming, and its AC-4 names this function. The change ADDS assertions and removes none. The body was one line, `checkScenario(ctx, check, "bgp-rfc7999-blackhole-frr")`, and `checkScenario` (`internal/le/interoplab/bgp/check_engine.go:66`) answers `scenario %s has no typed assertions` for every name absent from `scenarioOperations`. This name is absent by design, because `TestCheckerPopulationMatchesProducer` refuses a bespoke checker that also holds a generic fallback, so both polarities of `RFC7999-3.3-1` and of `RFC7999-3.3-2` had never executed one line of their interop evidence, while `rfc/enrolled.txt` cited this scenario at the rfc7999 row. The restored body waits for the FRR session and the BIRD session, polls `ip -4 route show` INSIDE the ze container until the Linux FIB names all three announced host routes, and then reads one verdict per destination: 10.100.0.1 is a discard route (both conditions of the single MUST sentence hold on the FRR session), 198.51.100.1 is forwarded with a gateway (the 3.3-1 negative, the community agreed and the covering authorization absent), and 10.200.0.1 is forwarded with a gateway (the 3.3-2 negative, the authorization present on the BIRD peer and the community not agreed on that session). Both requirements GAIN evidence for the first time rather than still being proven. |
| checkRouteServerASPath | Thomas approved the same spec on the same day, and its AC-4 names this function. The same always-firing `checkScenario` guard stood in front of the body, so `RFC7947-x-1` had no executing interop evidence at all. The restored body waits for the FRR session and the BIRD session, waits for BIRD to learn the 10.99.0.0/24 FRR originated, and requires BIRD's own `BGP.as_path` for that prefix to be exactly `65002`: ze's 65001 is absent, which is the recommendation of Section 2.2.2.1 ("the route server SHOULD NOT prepend its own AS number to the AS_PATH segment nor modify the AS_PATH segment in any other way"), and no other AS was added or dropped either. The AS_PATH line is required to sit beside a route line naming the prefix as a whole field, so BIRD's longest-match answer about a covering route cannot pass as the answer about this one. Both sessions are re-read at the end. `RFC7947-x-1` gains its first executed interop evidence. |
| checkNoExportBoundary | Thomas approved the same spec on the same day, and its AC-4 names this function. Both polarities of `RFC1997-Well-1` were bound to a body that was one `checkScenario` call, so its interop evidence had never run, while `rfc/enrolled.txt` cited this scenario at the rfc1997 row. The restored body first requires the lab to hold the base network, because the injected NEXT_HOP is raw hex that `renderScenario` does not rewrite and 172.30.0.9 names the injector on that network alone; it then waits for both observer sessions, waits for FRR and BIRD to learn the control 10.11.0.0/24 that carries an ordinary community, waits for the internal observer BIRD to learn the NO_EXPORT-tagged 10.10.0.0/24 (the negative: ze is a stand-alone AS, which RFC 1997 says to consider a confederation itself, so the prohibition is scoped and a receiver inside the boundary still gets the route), and requires FRR's whole `show bgp ipv4 unicast json` table to hold the control prefix and NOT the NO_EXPORT prefix (the positive: "All routes received carrying a communities attribute containing this value MUST NOT be advertised outside a BGP confederation boundary"). The control's presence in that same answer is the proof the query ran. The injector sends the control AFTER the tagged route, so FRR holding the control is proof ze had already decided what to do with the tagged one. Both sessions are re-read at the end. `RFC1997-Well-1` gains its first executed interop evidence. |
