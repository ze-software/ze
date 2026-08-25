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
| functional_suite_test | The file is deleted because its SUBJECT moved. It tested the per-suite wall-clock budget by extracting `run_suite` out of `mk/test-functional.mk` as text and driving the resulting shell. That recipe is now `scripts/le/application/functional.py`, and its tests are `scripts/le/functional_test.py`: 51 cases against 33 here. Every behavioural case has a counterpart. The cap expiry report and its `VERIFY FAILURE GROUP:` line, the runtime record, the creep warning, the duration units, the per-suite override on all three paths, the concurrency floor held equal to `runner.SuiteConcurrencyFloor`, the serial suites, and the rerun target for every suite. ONE case has no counterpart and is not replaced. `test_the_expanded_recipe_records_the_runtime` re-ran the runtime assertion against `make --dry-run` output, to catch a `$$` make ate. There is no expansion step left for it to catch anything in. `test_runtime_is_reported_against_the_budget` already makes that assertion against what runs. The file-shape cases lose their subject rather than their coverage: one property answers all three budget questions now. |
| fuzz_targets_test | The file is deleted because its SUBJECT is deleted. It tested `scripts/dev/fuzz-targets.py`, the generator that wrote `mk/test-fuzz-targets.mk` with one recipe line per `func Fuzz`. There is no generated fragment now. `scripts/le/application/fuzz.py` finds the fuzzers at run time by reading the packages, so nothing can go stale and nothing needs a freshness gate. Coverage of the DISCOVERY moved rather than went. `scripts/le/fuzz_test.py` asserts the same enumeration against the same tree. It adds two cases the generator never had: that `FUZZ` and `PKG` reach `go test` unaltered, so a regexp and a `/...` wildcard both still work, and that a failing fuzzer stops the sweep. What is genuinely gone is the freshness assertion. A file that is never written cannot be out of date. |
| TestVerifyWiringDocsRoutesFuzzTargetChanges | The whole function goes. It pinned that editing a `func Fuzz` test file, or `mk/test-fuzz-targets.mk`, scheduled `ze-fuzz-targets-check`. Both that gate and the generated file it guarded are deleted in this commit. The routing has no destination left, so `verify_wiring_docs.py` names `ze-fuzz-targets-check` nowhere. |
| verify_wiring_docs_test | Two of the four assertions the function above carried were negative: a non-fuzz internal test file, and a non-internal path, must NOT route the fuzz gate. They are satisfied by the absence rather than lost. `TestVerifyWiringDocsRoutesGoChanges` still drives the read-and-reject branch they shared. |
| gatingFunctionalSuites | `allSuitesRE` and its three checks go, replaced rather than removed. The guard read the gating suite list by regexp out of `mk/test-functional.mk` as text. That recipe is now `scripts/le/application/functional.py`, so the list comes from `le functional --list --json`. The assertion is the same one, over the same 24 gating suites. The vacuity guard is the reason to look twice, and it survived in a stronger form. The old test failed when the regexp matched fewer suites than expected. The new one fails on that, and on the listing command erroring, and on an empty list. A `le` that will not run can no longer be read as a tree with no suites. |
