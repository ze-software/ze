# Tests this commit weakens

Each row accepts one test weakening in the commit that carries this file. A
weakening removes an assertion, a case, or a test. `ai/rules/testing.md` forbids
every weakening. This file is the record of the ones a reviewer accepted.

Two gates read it. `c_test_weakening` (`.claude/hooks/pretool-writeedit.py`)
refuses the edit until this file names the test the edit weakens.
`scripts/dev/commit_helper.py` recomputes the weakenings of the paths the commit
names, and refuses a commit whose weakenings this file does not cover.

Both gates call one implementation, `scripts/dev/check_weakened_tests.py`, so
neither can disagree with the other about what a diff weakens or which row
covers it. `make ze-test-weakened-check` runs that checker over this file alone. It is
a stage of `ze-precommit-verify` in both modes, so a table this parser cannot read goes red
before any commit needs it.

A commit that weakens nothing carries the table with no rows.

## The commit carries this file

`scripts/dev/commit_helper.py` refuses a commit that weakens a test and does not
name `test/weakened.md` in its own `--file` list. A row that stays in the working
tree records nothing. The reason must sit in git history beside the weakening it
accepts, because that is the only place a later reader can find it.

## The file is replaced per commit

Delete the rows of the last commit. Write the rows of this one. This file never
accumulates.

The reason is the mechanism it replaces. A `test-relax:` comment stayed in the
test file forever, and a ceiling file capped their number. That corpus reached
601 tokens and 2,660 lines of prose across 413 test files. Nobody can read 601
justifications, so nobody read them, so writing one cost nothing.

A justification explains one diff. It is written at edit time and it is read at
review time. After the commit lands, it explains a change the reader of the test
file can no longer see. Storing that record permanently is what built the pile.

Git history holds every past entry. `git log -p -- test/weakened.md` shows the
rows of any commit beside the change they accepted.

## The test name

| Carrier | The name |
|---------|----------|
| Go, inside a top-level func | the enclosing `func TestXxx` |
| Go, outside every top-level func | the file stem |
| `.ci`, `.et` | the file stem, because each such file is one test |

`scripts/dev/rfc_tagged_scope.py` resolves each one. `go_func_units` returns the
top-level functions of a Go file with their names. `scope_reader` treats a file
that is not Go as one unit.

Row two is the case with no enclosing func to name. An `ignore` build tag in the
file header drops the whole file from the build, and a count can fall over
package-level code. The stem is then the only name available, so a weakening in
`a_test.go` is written as `a_test`.

**A stem row is owed only for what no named row already carries.** `weakened_units`
(`scripts/dev/check_weakened_tests.py`) reads a Go file twice, per function and
whole-file, and keeps a file-level finding only when its KIND is one no function
reported. Deleting a test lowers the file's count as well as emptying the func,
and both readings see it, so without that filter one deletion would demand two
rows. Write the stem row when the stem is the ONLY carrier, never as a second
row restating a named one: the commit gate refuses a row it cannot pair with a
weakening, and reports it as naming something "which this commit does not
weaken".

A bare name is accepted when it resolves to exactly one weakened test in the
commit. Write `package.TestName` when it does not.
`TestNoGoFileBuildsMarkup` exists in `internal/component/lg` and in
`internal/component/web`, so a commit that weakens both writes
`lg.TestNoGoFileBuildsMarkup` and `web.TestNoGoFileBuildsMarkup`.

## The reason

Name what left the suite, and say why the commit is correct without it. A reason
that names no lost coverage tells the reviewer that the detector fired on a
change which removed none.

Every weakening kind needs a row at commit time, a falling count included, and
one row carries every kind the gate attributes to that name. The hook is
narrower on purpose. An edit that only lowers a count gets a notice and
lands. Consolidating three cases into one table lowers a count exactly as
deleting a check does. So the commit gate asks for a row the hook did not, and
that row is where you say which of the two happened.

| Test | Reason |
|------|--------|
| TestDispatchShowBGPPeerCapabilities | No coverage left the suite: the assertions moved one call deep, into `firstPeerRow` in `summary_test.go`. `show bgp peer capabilities` used to answer a bare object for one matched peer and an array for several, so the `count` operator answered on a three-peer router and was refused on a one-peer router. It answers a `peers` envelope for either cardinality now, so the two lines that cast `resp.Data` to `plugin.Map` and required the cast cannot stand. `firstPeerRow` replaces them and makes THREE requires where the cast made one: the answer is an envelope, it holds `peers` at the exact type `plugin.Slice[plugin.Map]`, and the row set is non-empty. The detector counts assertion statements in the function body, which fall 4 to 3; what the test checks rises. Re-run: `make ze-unit-pkg-test PKG=./internal/component/bgp/plugins/cmd/peer/` ok. |
| TestDispatchShowBGPPeerStatistics | Same change, same helper, same commit: `show bgp peer statistics` answers the `peers` envelope for one matched peer as for several, so the `plugin.Map` cast and its require are replaced by `firstPeerRow`, whose three requires are stricter than the one they replace. Body statements 4 to 3. Re-run: as above, ok. |
| TestBgpPeerCapabilitiesHandler | Same change and same helper. Body statements 10 to 9; the removed pair is the `plugin.Map` cast and its require, and `firstPeerRow` asserts the envelope, the rows key at its exact type, and a non-empty row set in their place. Re-run: as above, ok. |
| TestPeerCapabilitiesHandler | Same change and same helper. Body statements 11 to 10, and the same pair is what left. Re-run: as above, ok. |
| TestPeerCapabilitiesNotEstablished | Same change and same helper. The case still asserts `negotiation-complete` is false for a peer that completed no OPEN exchange; only the route to the row changed. Re-run: as above, ok. |
| TestPeerShowStatistics | Same change and same helper. Every counter and rate this case asserted is still asserted, read from the row `firstPeerRow` returns. Re-run: as above, ok. |
| TestPeerShowStatisticsZeroUptime | Same change and same helper. The case still asserts all four rates are zero when uptime is zero. Re-run: as above, ok. |
| TestBgpPeerStatisticsUptimeTruncatedRatesExact | Same change and same helper. The case still asserts the exact rate arithmetic against a truncated uptime. Re-run: as above, ok. |
