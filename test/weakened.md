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
| TestListenerDefaultsAgreeWithMCPExtraction | What left were the assertions about a REFUSAL that no longer exists: an `assert.False` on `ExtractMCPConfig` returning ok, and a loop of `assert.NotContains` proving no doctor endpoint was invented for a config that started no listener. Thomas ruled on 2026-08-23 that the daemon applies the YANG `refine port { default 8080; }` instead of refusing, so there is no refusal left to assert. The replacement asserts MORE, not less: it pins the exact port of every server entry in config order with one `assert.Equal` over a slice, where the old test proved only an absence. The multi-entry case survives and is stronger, `{"9090", ""}` must yield `{"9090", "8080"}`, so a named port is never overwritten and only the silent entry takes the default. The count falls 5 to 4 because a `False` plus a loop of `NotContains` became one `require.True` plus one `Equal` over a slice, and the checker counts calls rather than what they cover. Both halves are still asserted together on purpose: removing the default from `extractMCPBlock`, or the registration from `listener_defaults.go`, each breaks one half. Re-run after the change: PASS |
| TestMCPMissingPortIsNotReportedForADisabledBlock | Deleted, because its subject was deleted. It asserted that the missing-port DIAGNOSTIC stays silent for a disabled mcp block, and that diagnostic no longer exists: `MCPMissingPortAdvice` and `MCPServersMissingPort` are removed in this commit along with the refusal they reported, since `MCPServersMissingPort` read the config tree and would have rejected exactly the configuration the default now makes valid. The property it existed to protect is not lost, it is asserted positively instead: `TestMCPPortDefaultFollowsTheEnabledGate`, in the same file, pins that a disabled block starts no listener and takes no default. A test that the removed diagnostic stays quiet would assert nothing once nothing can emit it |
| mcp-listener-missing-port | RENAMED to `test/parse/mcp-listener-port-default.ci`, not removed. Its six expectations asserted the parse-time refusal of a server entry that names no port; the file now asserts that such an entry takes 8080. The rename is the point: a file called `missing-port` that tests the default states the opposite of what it asserts, and a name that lies is the defect this commit exists to fix, one layer up. Re-run under the new name: the parse suite is 6 of 6 PASS |
| mcp-listener-missing-port-disabled | RENAMED to `test/parse/mcp-listener-port-default-disabled.ci`, same reason. Its three expectations covered the disabled block, where the behaviour is unchanged: no listener, and now no default either. Only the header and the name moved. Re-run under the new name: PASS |
| doctor-mcp-missing-port | RENAMED to `test/ui/doctor-mcp-port-default.ci`, same reason. It asserted that `ze doctor` reported a missing mcp port; it now asserts that doctor probes the endpoint the daemon binds, which is the half `RegisterListenerDefault` adds in this commit. The embedded config moved with it, `mcp-missing-port.conf` to `mcp-port-default.conf`, and a grep for `missing-port` under `test/` now returns nothing. Re-run under the new name: the ui suite is 2 of 2 PASS |
