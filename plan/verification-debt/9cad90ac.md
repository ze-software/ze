# Verification debt -- commit session 9cad90ac

Gates that had not run green over these commits when they were made.
Each row is work owed, not a defect: a defect is fixed, never recorded
(`ai/rules/completion.md`). Clear a row with `./le commit debt-clear`:
it re-runs the gate the row names and writes `cleared` only when that
run exits 0. Most of those gates judge the WORKING TREE, so a cleared
row says the gate was green in this checkout, other sessions'
uncommitted files included. It does not say the gate is green over the
commit alone: `discovery-index freshness` and
`./le repository tracked-build check` are the two gates re-judged over
what git holds. A human MAY delete the
shard once every row is cleared.
the retired `scripts/dev/commit_helper.py create --push` (current producer: `internal/le/commit/prepare.go`) refuses while any row here
is open.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-20 | 9cad90ac | fix(journal): a Spec cell names spec stems, never prose | ./le verify current mode full (not FRESH-green) | owner said commit it; the changed scripts own suites are green | open |
| 2026-08-20 | 9cad90ac | fix(journal): a Spec cell names spec stems, never prose | ./le verify current mode full structural gates (red) | owner said commit it; the reds are another session uncommitted Go lint under scripts/evidence/l2tp-diag (retired; current producer: `internal/le/deployment/`), this commit is python and markdown only | cleared |
