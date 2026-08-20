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
| TestChangedPkgsFailedLastVerifyIgnoresBaseline | Replaced, not dropped. It asserted the OLD answer: a red last verify meant the committed-since term was skipped and the change set was EMPTY. That is the hole this commit closes, because without a proven green commit every commit in history is unverified, so the honest answer is everything. TestChangedPkgsWidensWithNoTrustedGreenBaseline drives the same condition and asserts ./... instead. |
| TestChangedPkgsInvalidBaselineShaIgnored | Replaced by the same table. Its condition, a recorded SHA that is not a commit here, is the second subtest of TestChangedPkgsWidensWithNoTrustedGreenBaseline, which asserts the widened answer rather than an empty one. |
| TestChangedPkgsExcludesIgnoreOnlyDirs | Replaced. It pinned that a directory go list cannot report is EXCLUDED from the set. The selector now widens there instead, because a path nobody can classify is a path whose blast radius is unknown. TestChangedPkgsWidensForADirectoryGoListCannotReport asserts the new answer over the same fixture. |
| TestChangedPkgsDeletedPackageFilteredOut | Replaced. It pinned that a package a commit deleted is filtered out of the set. It is now the `a package a commit deleted` subtest of TestChangedPkgsWidensForADirectoryGoListCannotReport, which asserts the widened answer: the importers of a deleted package still owe a retest and no package name is left to name them. |
| writeStatus | A helper, not a test. It gained a root parameter so it writes into the fixture repository rather than a shared location, and the assertion it lost was a check of its own temp path, which the new signature makes unreachable. Every caller asserts the same behaviour through the tests above. |
| missingStatus | A helper, not a test. It existed to hand runChangedPkgs an environment naming a status file that does not exist. The status file is now found from the repository root, so no caller can name a missing one and the helper has no body left to keep. |
| TestClassifyFunctionalInstallFallbackUsesSuiteCommand | Replaced by a stronger test. It asserted one special case, that the install suite reruns as `bin/ze-test install --all`. functionalSuiteRerun now returns a make target for EVERY suite, and the new test asserts that each one is a make command whose target is declared in the Makefile or under mk/. That covers install and the other suites the old test said nothing about. |
| readStatusField | A helper, not a test. The verify status file is now read through the same reader the freshness check uses, so this local parser has no caller and no assertion of its own left. |
