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
| flowspec-fw-protocol-sctp | Lost a reject on a digits spelling of l4proto 132 and a bare expect matching l4proto alone, so the count falls from 10 to 9. The reject could never fire. With the defect present `protocolMatches` yields `MatchProtocol{"132"}`. `lowerProtoMatch` (`internal/plugins/firewall/nft/lower_linux.go`) then refuses a name outside the canonical table, and `Apply` (`internal/plugins/firewall/nft/backend_linux.go`) returns that error before its single `conn.Flush`. No rule of any spelling reaches the kernel for `nft list ruleset` to print. The removed pattern also carried literal double quotes, which nft never writes around an l4proto value. What replaces both is one full rule line, matching the destination, the port, the protocol and the verdict together. It implies the removed protocol assertion. It also reddens when a match is DROPPED and the rule is enforced wider than the peer announced. |
| flowspec-fw-untranslatable-keeps-others | Lost a reject on a digits spelling of l4proto 253, so the negative count falls from 2 to 1. Unfireable for the same reason: with the defect present nothing reaches the kernel, so no ruleset dump can carry protocol 253 under any spelling. The surviving reject on `daddr 10.2.0.0/24` is the one that denies an enforced refused route, because `daddr` appears in a programmed rule and never in the NLRI spelling the refusal WARN quotes. Expectations GREW: the same full-rule pattern as above, plus the `ze_flowspec_rules_refused_total` series with reason `unknown-protocol`. |
