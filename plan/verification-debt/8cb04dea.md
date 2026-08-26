# Verification debt -- commit session 8cb04dea

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
| 2026-08-24 | 8cb04dea | fix(zeledon): make weekly publication metadata explicit | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-24 | 8cb04dea | fix(zeledon): correct recovered post date | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-24 | 8cb04dea | docs(zeledon): add 17 August weekly update | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-24 | 8cb04dea | docs(site): reconcile weekly feature and benchmark data | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-26 | 8cb04dea | website: add dark architecture diagram palette | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-26 | 8cb04dea | website: use paired architecture diagram assets | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-26 | 8cb04dea | docs(plan): record independent CLI pipe review findings | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-26 | 8cb04dea | feat(le): port the repository tooling to Go under letools/ | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-26 | 8cb04dea | feat(le): port the repository tooling to Go under letools/ | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-26 | 8cb04dea | feat(le): port the repository tooling to Go under letools/ | independent critical review | Independent reviewer requested a commit baseline before review; review will run against this commit. | open |
| 2026-08-26 | 8cb04dea | fix(le): keep registrations within committed source | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-26 | 8cb04dea | fix(le): keep registrations within committed source | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-26 | 8cb04dea | fix(le): keep registrations within committed source | independent critical review | Independent reviewer requested a commit baseline before review; review will cover this repair commit. | open |
| 2026-08-26 | 8cb04dea | fix(le): keep registrations within committed source | ze-repository-tracked-build-check (HEAD does not compile) | This commit removes composition-root imports for packages absent from HEAD. | open |
| 2026-08-26 | 8cb04dea | fix(le): keep ze-le registrations committed | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-26 | 8cb04dea | fix(le): keep ze-le registrations committed | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-26 | 8cb04dea | fix(le): keep ze-le registrations committed | independent critical review | Independent reviewer requested a commit baseline before review; review will cover this repair commit. | open |
| 2026-08-26 | 8cb04dea | fix(le): keep ze-le registrations committed | ze-repository-tracked-build-check (HEAD does not compile) | This commit removes ze_le composition-root imports for packages absent from HEAD. | open |
