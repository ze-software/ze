# Verification debt -- commit session 1d9dec9c

Gates that had not run green over these commits when they were made.
Each row is work owed, not a defect: a defect is fixed, never recorded
(`ai/rules/completion.md`). Clear a row with `make ze-verify-debt-clear`:
it re-runs the gate the row names and writes `cleared` only when that
run exits 0. Most of those gates judge the WORKING TREE, so a cleared
row says the gate was green in this checkout, other sessions'
uncommitted files included. It does not say the gate is green over the
commit alone: `discovery-index freshness` and
`ze-repository-tracked-build-check` are the two gates re-judged over
what git holds. A human MAY delete the
shard once every row is cleared.
`scripts/dev/commit_helper.py create --push` refuses while any row here
is open.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-22 | 1d9dec9c | ipsec: prove a responder-role rekey against strongSwan | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | ipsec: prove a responder-role rekey against strongSwan | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-22 | 1d9dec9c | plan: close spec-fixit-child-sa-rekey-policy | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | fix(status): the spec inventory counts every spec it reads | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | fix(status): the spec inventory counts every spec it reads | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-22 | 1d9dec9c | chore(plan): close fixit-spec-status-metadata-window | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | iface: close the selector-ignored-by-apply spec | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | iface: close the selector-ignored-by-apply spec | ze-precommit-verify (not FRESH-green) | no verify has run against this tree; the commit carries no code, so no gate reads anything it changes. The five code commits carry their own debt rows | open |
| 2026-08-22 | 1d9dec9c | plan: close spec-fixit-iface-selector-ignored-by-apply | ze-precommit-verify (not FRESH-green) | the commit removes one markdown file and no gate reads it | open |
