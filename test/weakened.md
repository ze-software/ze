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
| assertAnswerShape | Two assertions on the head's `status=` go, because the head no longer writes one: the outcome of an answer is stated once, on its terminator. The helper still checks every line's kind, the head's item type, the record payloads and both terminator counts, and `TestHeadStatesNoStatus` is the named test that the head carries no status at all. |
| TestAnswerForAgreesWithTheWire | The comparison of the in-process head's status against the wire head's goes, because neither head carries one. What the two producers must still agree about is unchanged and still asserted: the item type, the envelope name, the column names, every record in walk order, the verdict and the terminator's message. The message is what now carries a failure, and the last two failing cases of the table drive it. |
| TestSendExecuteCommandReadsRecords | One assertion on `Answer.Status` goes with the field. The answer's outcome is read from `Answer.Verdict` and `Answer.Message`, both already asserted in the same test. |
| TestExecAnswerUnconditional | The assertion that the exec frame's first line states `status=done` goes with the head's status. The kind assertion beside it is what says the first line is the head, and it stays; `TestAFailedCommandCarriesItsMessageOnTheTerminator` covers the failing outcome the status used to carry. |
| TestParseLineCarriesKeyValueTailWhole | Renamed to `TestParseLineCarriesTheAnswerTailWhole`, in place, because the tail it hands whole is no longer bare `key=value` pairs. No coverage left the suite: the new test makes every assertion the old one made, over the same four line kinds, with each fixture rewritten into the positional grammar the writers now produce. The detector reports a deleted func because a rename is a delete plus an add in one hunk. |
| TestTailTokenizerNeedsNoJSONDecoder | The head's `status=` assertion goes with the field. The test's own point is unchanged and still asserted: a reader takes the item type, the envelope name, a record payload that is not JSON at all, and both terminator counts, without a JSON decoder. |
| TestOpenEndedKeyRunsToEndOfLine | Renamed to `TestAnswerValuesNeedNoEscaping`, in place, because only the record payload still runs to the end of the line: the message and the column names are counted fields now. No coverage left the suite. All four subtests survive with the same values, the same round trip and the same newline case, and each is named for what it carries rather than for the key it used to sit behind. |
| TestTerminatorIsTheLineStatingTheEndKind | The head's `status=` assertion goes with the field. The test asks each line what it is with no other line in hand, and that question is still asked of all four kinds. |
| TestTerminatorCarriesNoStatusKey | Replaced by `TestHeadStatesNoStatus`, which is the same question asked of the line that used to answer it wrongly. The terminator never stated a status; the head did, and the head is where the check now sits. The new test writes every head the writers can produce, requires none to carry a status word or an `=` at all, counts the head's fields against the grammar, and refuses a head that states an outcome. |
| TestFieldsRunToEndOfLine | Renamed to `TestColumnNamesAreCounted`, in place, because the column names state their own byte count rather than running to the end of the line. No coverage left the suite: the same schema, holding a space and an `=`, is written after an envelope name and read back name for name, and the suffix assertion is replaced by one that the stated count equals the schema's length. |
| TestAnswerKeyBelongsToOneKind | Deleted with the keys it tested. Every case in it offered a key name to a kind that does not carry it, and no key name reaches the wire any more, so each case now asserts a grammar that no writer can produce. `TestAnswerLineCarriesNoKeyNames` is what replaces it: it requires every kind's line to carry no `key=value` pair, and it feeds each retired tail this test used to accept to the reader and requires a refusal. The misplaced-key rule it covered is now structural, because a positional reader has no key to misplace. |
| TestExecuteCommandRecordResult | One assertion on the head's status goes with the field. The case it covered, a handler that reported a failure, is asserted more strongly than before: the terminator's message is now compared against the handler's own text, and the answer is required to carry no record. |
| TestPluginAnswersRecordsWithoutDeclaringAShape | The assertion that the answer's status is `done` goes with `Answer.Status`. The verdict assertion beside it is the stronger statement of the same fact, and it stays. |
