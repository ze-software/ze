# Verification debt -- commit session b6dab65b

Gates that had not run green over these commits when they were made.
Each row is work owed, not a defect: a defect is fixed, never recorded
(`ai/rules/completion.md`). Clear a row with `make ze-verify-debt-clear`:
it re-runs the gate the row names and writes `cleared` only when that
run exits 0. Every gate runs inside one throwaway worktree at HEAD, so
a cleared row says the gate was green over the COMMIT rather than
beside the uncommitted files several sessions keep in this checkout.
When no worktree can be made, nothing clears and the pass exits 1: the
fallback it refuses, judging the working tree, is the whole reason the
worktree is there. A human MAY delete the
shard once every row is cleared.
`scripts/dev/commit_helper.py create --push` refuses while any row here
is open.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-24 | b6dab65b | feat(website): rework the blog, FAQ, and pipe reference pages | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-24 | b6dab65b | plan: spec the asciinema conversion of the terminal demos | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-24 | b6dab65b | fix(tests): make the python runner count what each file ran | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-24 | b6dab65b | fix(tests): make the python runner count what each file ran | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-24 | b6dab65b | fix(tests): stop a duplicate class name shadowing 1 of 821 tests | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
