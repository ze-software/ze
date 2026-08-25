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
| target | `target` and its recursive helper `withDelegated` are deleted, and the gate reads that as content replaced by an empty string plus a table-driven case going 1 -> 0. Coverage RISES rather than falls. The pair existed to flatten one make target's recipe together with the recipes it delegates to, so a caller could ask "does this target's text contain X". That question is now asked of every QEMU target rather than of two, by a larger set of helpers that replace them: `declaredTargets` and `targetName` enumerate the file, `qemuRunInvocations` and `qemuRunTargets` find the thirteen real `qemu-run.py` calls, `targetsWhoseRecipeContains` answers the containment question, and `isMakeConditional` fixes a real defect in the old flattening -- it stopped at an `ifneq`, which reported `ze-qemu-debug` and `ze-qemu-shell` as unwired while make itself ran their recipes. The suite GAINS a test in the same commit, `TestQemuTargetsDependOnHostBuild`, and the two that survive now assert over thirteen targets each. Discrimination measured on a clean diagonal: removing `--kernel`, the guard, or `ze-host-build` from one target reddens exactly one of the three tests and leaves the other two green. |
| TestQemuTargetsGuardTheStagedKernel | Assertions go 7 -> 5, and neither of the two is lost. One MOVED: `runs the guard but does not declare ": ze-host-build"` is now `TestQemuTargetsDependOnHostBuild`, a test this same commit adds, which asserts it for all thirteen QEMU targets rather than only for those that already had the guard. The other was REPLACED by something stronger that the detector cannot attribute here, because it sits in a shared helper rather than in the test body: a `t.Fatalf` on `len(users) < 2`. That floor said nothing about whether the parser had silently stopped finding targets, because eleven of the thirteen could vanish and it would still pass. `qemuRunTargets` now carries two fatals instead -- one refusing an empty enumeration with "this test must not pass vacuously", and one cross-checking attributed invocations against a raw count of recipe lines in the file, so a parser that loses an invocation is caught without anybody remembering to update a number. The remaining four assertions about the guard's own body are unchanged, and the per-target assertion now runs over thirteen targets rather than six. |
| makeInvocationTargets | Deleted with `target`, whose delegation walk was its only caller: it split a `$(MAKE) ... foo bar` recipe line into the target names being invoked, so `withDelegated` knew what to recurse into. Nothing recurses now. `qemuRunTargets` enumerates every declared target directly, which reaches the same recipes without needing to know which target delegates to which. |
| guardUsers | Deleted for the same reason and replaced by a more general helper. It answered one hard-coded question, "which targets take `$(ze-qemu-kernel-guard)`", by scanning for that one literal. `targetsWhoseRecipeContains` takes the needle as an argument, so the same scan now serves the guard, `--kernel`, and anything a later test needs to ask. The guard assertion it fed is not weaker: it runs over thirteen targets instead of the six that had the guard when it was written. |
| withDelegated | Deleted with `target`, and the same row governs it: it was that function's recursion step, never called from anywhere else, and its job is done by `qemuRunTargets` walking every declared target instead of following delegation from a named one. |
