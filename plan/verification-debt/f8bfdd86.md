# Verification debt -- commit session f8bfdd86

Gates that had not run green over these commits when they were made.
Each row is work owed, not a defect: a defect is fixed, never recorded
(`ai/rules/completion.md`). Clear a row with `./le commit debt-clear`:
it re-runs the gate the row names and writes `cleared` only when that
run exits 0. Most of those gates judge the WORKING TREE, so a cleared
row says the gate was green in this checkout, other sessions'
uncommitted files included. It does not say the gate is green over the
commit alone: `discovery-index freshness` and
`./le repository-tracked-build check` are the two gates re-judged over
what git holds. A human MAY delete the
shard once every row is cleared.
the retired `scripts/dev/commit_helper.py create --push` (current producer: `internal/le/commit/prepare.go`) refuses while any row here
is open.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-19 | f8bfdd86 | fix(hooks): the test-first gate blocks, and earns a commit-time half | ./le verify current mode full (not FRESH-green) | owner ordered commit with no checks; hook suite green this session (parity 206/206, fixtures 562/562) | open |
| 2026-08-19 | f8bfdd86 | fix(hooks): the test-first gate blocks, and earns a commit-time half | ./le verify current mode full structural gates (red) | owner ordered commit with no checks. The red verify record ended 2026-08-19T19:48:47Z; every file in this commit was written 20:19Z or later, so ./le doc-wiring and ./le repository generated-check cannot be charged to it | cleared |
| 2026-08-19 | f8bfdd86 | fix(commit-helper): record_debt appends a row the shard already owes | ./le verify current mode full (not FRESH-green) | owner ordered the previous commit with no checks and this follows it; commit_helper 226/226, parity 206/206, fixtures 562/562 green this session | open |
| 2026-08-19 | f8bfdd86 | fix(commit-helper): record_debt appends a row the shard already owes | ./le verify current mode full structural gates (red) | the red verify record ended 2026-08-19T19:48:47Z and names ./le doc-wiring and ./le repository generated-check, neither charged to any file in this commit | cleared |
| 2026-08-19 | f8bfdd86 | feat(commit-helper): arm the test-coverage gate behind --no-test | ./le verify current mode full (not FRESH-green) | owner ordered the commit; a full verify is mid-run in this checkout and has not rewritten the record | open |
| 2026-08-19 | f8bfdd86 | feat(commit-helper): arm the test-coverage gate behind --no-test | ./le verify current mode full structural gates (red) | ./le doc-wiring is red on ApplyPipesRecords (internal/component/command/pipe.go) and WriteAnswer (internal/component/plugin/dispatch.go), both another session's uncommitted work that this commit does not touch. ./le repository generated-check is now green | cleared |
| 2026-08-19 | f8bfdd86 | fix(gates): mark the fixture paths doc-links reads as citations | ./le verify current mode full (not FRESH-green) | owner ordered the commit; a full verify is mid-run in this checkout and has not rewritten the record | open |
| 2026-08-19 | f8bfdd86 | fix(gates): mark the fixture paths doc-links reads as citations | ./le verify current mode full structural gates (red) | ./le doc-wiring is red on ApplyPipesRecords (internal/component/command/pipe.go) and WriteAnswer (internal/component/plugin/dispatch.go), both another session's uncommitted work that this commit does not touch | cleared |
