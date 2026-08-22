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
| TestFormatResult | Deleted with `FormatResult`, the function it covered. That function had no non-test caller: it wrapped `AppendResult` in a fresh allocation for a test to read. `TestFormatRequest`, `TestFormatOK` and `TestFormatError` still cover the three siblings that DO have callers, and `TestParseLine`'s ok round-trip now builds its line with `AppendResult`, so the `#<id> ok <json>` shape is still asserted. |
| TestUnderstandsNeedsTheNameDeclared | Deleted with `DeclareCapabilitiesInput.Understands` and the `Protocol` list it read. Nothing negotiates a wire shape any more, so there is no name to declare and no fail-closed direction left to prove. What it guarded, an answer frame reaching a peer that did not ask for it, cannot happen when one encoding is the only encoding. |
| TestDeclareCapabilitiesCarriesTheProtocolList | Deleted for the same reason: it asserted that the `protocol` JSON key crossed the wire in both directions. The key is gone from the struct. `TestPluginAnswersRecordsWithoutDeclaringAShape` replaces the claim that matters, that a plugin whose Stage 3 names nothing still writes and is read as a record sequence. |
| TestUndeclaredPeerReadsAsUnnegotiated | Deleted with `Process.RecordAnswers`. It asserted the zero value of the flag this commit removes. `TestDispatchCommandAlwaysAnswersRecords` already drives the same undeclared peer through both dispatch ops and reads the record answer it receives, which is the behavior the flag's zero value used to deny. |
| TestHubStartupSinkRecordsTheProtocolDeclaration | Deleted with `hubStartupSink.onCapabilities`'s only statement. The hub records nothing at Stage 3 now. `TestSubsystemRPCCommand` and `TestSubsystemHandler` cover what the deleted test protected: both drive `SubsystemHandler.Handle` against a subsystem whose Stage 3 declares nothing, and both now require the answer to arrive as a head, an item and a terminator. |
| TestPluginRunDeclaresRecordAnswers | Replaced by `TestPluginAnswersRecordsWithoutDeclaringAShape` in the same file. The old test read the protocol list off the Stage 3 message; there is no list. The replacement asserts the stronger fact in its place: the Stage 3 message carries no `protocol` key AND the same plugin answers a later execute-command with the record sequence. |
| message_test | Assertion count fell from 101 to 100 with `TestFormatResult`. The file's other three format tests are untouched, and `TestParseLine`'s ok round-trip still asserts the `#<id> ok <json>` shape, now built with `AppendResult`. |
| types_test | Assertion count fell from 85 to 76 with the two `Understands` tests above. Nothing else in the file moved. What the nine assertions covered was the negotiation's fail-closed direction, and a negotiation that does not exist cannot fail open. |
| dispatch_registry_test | Assertion count fell from 54 to 53 with `TestUndeclaredPeerReadsAsUnnegotiated`, which was one assertion on the deleted flag's zero value. `TestDispatchCommandAlwaysAnswersRecords` in the same file drives the same undeclared peer end to end. |
| subsystem_test | Assertion count fell from 44 to 40 with `TestHubStartupSinkRecordsTheProtocolDeclaration`. Two of the four were `require.NoError` on the sink call and two read the deleted flag. The hub's real obligation moved into `TestSubsystemRPCCommand`, whose mock now writes the record answer the hub must read. |
