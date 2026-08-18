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
| TestRouteStore_InternAttribute | Tested `RouteStore`, deleted in this commit as superseded by `attrpool.Pool.Intern`. The code under test no longer exists, so no live behavior loses coverage. |
| TestRouteStore_InternNLRI | Same deletion. NLRI interning on this path was replaced by trie keys in `internal/component/bgp/plugins/rib/storage`. |
| TestRouteStore_InternRoute | Same deletion. `internRoute` never had a production caller in any commit. |
| TestRouteStore_ReleaseRoute | Same deletion. |
| TestRouteStore_Stats | Same deletion. |
| BenchmarkRouteStore_InternAttribute | Benchmark of the deleted store. `attrpool` carries its own benchmarks for the mechanism that replaced it. |
| BenchmarkRouteStore_InternRoute | Same deletion. |
| TestReleaseRouteCannotDropARouteAnotherInternHolds | Added in `aae53cb1b` to pin the lock ordering in `releaseRoute`. That function is deleted here, so the invariant it asserted has no code left to hold. The defect and its repair stay recorded in `plan/journal/refcount-released-outside-the-lock.md`. |
| Hash | Method of the `hashableAttr` test subject in the deleted `rib/store.go`. |
| Equal | Method of the same deleted type. |
| TestAttributeStore_InternBasic | Tested `internal/component/bgp/store`, deleted in this commit: the package had exactly one importer, the deleted `rib/store.go`. |
| TestAttributeStore_Lookup | Same package deletion. |
| TestAttributeStore_Release | Same package deletion. |
| TestAttributeStore_Concurrent | Same package deletion. |
| TestAttributeStore_InternDirect | Same package deletion. |
| TestHashHelpers | Tested `HashBytes` in the deleted package. |
| BenchmarkAttributeStore_Intern | Benchmark of the deleted package. |
| BenchmarkAttributeStore_InternDirect | Benchmark of the deleted package. |
| BenchmarkAttributeStore_ConcurrentIntern | Benchmark of the deleted package. |
| Key | Method of the `hashableNLRI` test subject in the deleted `rib/store.go`. |
| FamilyKey | Method of the same deleted type. |
| TestFamilyStore_InternBasic | Tested the per-family NLRI store in the deleted package. |
| TestFamilyStore_Release | Same package deletion. |
| TestFamilyStore_Concurrent | Same package deletion. |
| TestNLRIStore_MultipleFamilies | Same package deletion. |
| TestNLRIStore_GetOrCreate | Same package deletion. |
| TestNLRIStore_Release | Same package deletion. |
| TestNLRIStore_ConcurrentFamilies | Same package deletion. |
| BenchmarkFamilyStore_Intern | Benchmark of the deleted package. |
| BenchmarkNLRIStore_Intern | Benchmark of the deleted package. |
| TestPrintFormatted | Tested `printFormatted`, the client's local renderer, deleted in this commit. The daemon now renders every one-shot answer (`internal/component/ssh/ssh.go`, `execMiddleware`), because it is the only process of the pair that holds `environment cli format default`. Leaving the client renderer beside it is the layering `ai/rules/no-layering.md` forbids. Its four cases did not leave the suite: `empty_output` is now `TestPrintDaemonOutputPrintsWhatTheDaemonRendered/empty_answer_with_no_format_pipe_says_OK` and `plain_text` is that test's verbatim-printing case, both in the same file; `json_data_yaml_format` moved to `TestRenderYAMLScalarFields` and `json_data_json_format` is covered by `TestApplyJSON*` in `internal/component/command`. |
| TestRenderCommandOutputFormatPipeBeatsFormatFlag | Tested `renderCommandOutput`, deleted with `printFormatted` for the same reason. The precedence it pinned SURVIVES and moved to the site that now applies it: `TestCLIFormatFlagBecomesAPipe/an_explicit_format_pipe_beats_the_flag` asserts that `commandWithFormat` leaves a command whose chain already names `json compact` alone under `--format yaml`. The defect it names (plan/journal/silent-fall-through.md, 2026-08-14) is quoted in the new test's PREVENTS comment and in `commandWithFormat`'s doc comment. |
| TestRenderCommandOutputFormatFlagAppliesWithoutAPipe | The other half of the same precedence, same deletion. It is now `TestCLIFormatFlagBecomesAPipe/the_flag_becomes_a_format_pipe`, which asserts the stronger fact: the flag does not merely apply, it reaches the daemon as a format pipe so one implementation renders every surface. |
| TestPrintFormattedNestedData | Reached `command.RenderYAML` through the deleted `printFormatted`. Moved unchanged to `TestRenderYAMLNestedData` in `internal/component/command/format_test.go`, the package that produces the behavior, which had no direct test for `RenderYAML` before this commit. Every assertion is carried over. |
| TestPrintFormattedStringList | Same move, to `TestRenderYAMLStringList` in the same new file. Every assertion is carried over. |
| main_test | The assertion count of `internal/component/cli/client/main_test.go` falls 71 -> 63 because the five tests above left this file. Six of the eight land in the two new tests in this same file and in `internal/component/command/format_test.go`; the remaining two are `printFormatted`'s json and table arms, which `internal/component/command/pipe_table_test.go` and `pipe_test.go` already own at the layer that renders them. |
