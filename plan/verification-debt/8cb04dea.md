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
| 2026-08-26 | 8cb04dea | feat(le): port the repository tooling to Go under former top-level tool tree/ | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-26 | 8cb04dea | feat(le): port the repository tooling to Go under former top-level tool tree/ | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-26 | 8cb04dea | feat(le): port the repository tooling to Go under former top-level tool tree/ | independent critical review | Independent reviewer requested a commit baseline before review; review will run against this commit. | open |
| 2026-08-26 | 8cb04dea | fix(le): keep registrations within committed source | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-26 | 8cb04dea | fix(le): keep registrations within committed source | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-26 | 8cb04dea | fix(le): keep registrations within committed source | independent critical review | Independent reviewer requested a commit baseline before review; review will cover this repair commit. | open |
| 2026-08-26 | 8cb04dea | fix(le): keep registrations within committed source | ze-repository-tracked-build-check (HEAD does not compile) | This commit removes composition-root imports for packages absent from HEAD. | open |
| 2026-08-26 | 8cb04dea | fix(le): keep ze-le registrations committed | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-26 | 8cb04dea | fix(le): keep ze-le registrations committed | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-26 | 8cb04dea | fix(le): keep ze-le registrations committed | independent critical review | Independent reviewer requested a commit baseline before review; review will cover this repair commit. | open |
| 2026-08-26 | 8cb04dea | fix(le): keep ze-le registrations committed | ze-repository-tracked-build-check (HEAD does not compile) | This commit removes ze_le composition-root imports for packages absent from HEAD. | open |
| 2026-08-27 | 8cb04dea | fix(cli): make pipe contracts truthful across surfaces | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-27 | 8cb04dea | fix(cli): make pipe contracts truthful across surfaces | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-27 | 8cb04dea | fix(cli): make pipe contracts truthful across surfaces | discovery-index freshness | ai/PACKAGE-MAP.md includes unrelated in-flight former top-level tool tree packages and is excluded from this focused CLI commit. | open |
| 2026-08-27 | 8cb04dea | fix(cli): close pipe contract review gaps | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-27 | 8cb04dea | fix(cli): close pipe contract review gaps | full ze-precommit-verify over this commit's Go | Focused package, live functional, documentation, and repository checks cover this review-fix chunk; the periodic full gate has not run. | open |
| 2026-08-27 | 8cb04dea | fix(cli): close pipe contract review gaps | discovery-index freshness | ai/PACKAGE-MAP.md contains concurrent former top-level tool tree discovery work outside this CLI commit. | open |
| 2026-08-27 | 8cb04dea | fix(cli): enforce pipe contracts at final boundaries | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-27 | 8cb04dea | fix(cli): enforce pipe contracts at final boundaries | full ze-precommit-verify over this commit's Go | Focused unit packages, authenticated functional scenarios, documentation drift, and every tracked build flavor cover this review-fix chunk; the periodic full gate has not run. | open |
| 2026-08-27 | 8cb04dea | fix(cli): enforce pipe contracts at final boundaries | discovery-index freshness | ai/PACKAGE-MAP.md and ai/DOCS-TO-CODE.md contain concurrent repository changes outside this CLI commit. | open |
| 2026-08-27 | 8cb04dea | fix(cli): close fourth pipe review findings | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-27 | 8cb04dea | fix(cli): close fourth pipe review findings | full ze-precommit-verify over this commit's Go | Focused command, registry, SSH, docvalid, and authenticated production PTY tests cover this review-fix chunk; the periodic full gate has not run. | open |
| 2026-08-27 | 8cb04dea | fix(cli): close fourth pipe review findings | discovery-index freshness | ai/PACKAGE-MAP.md and ai/DOCS-TO-CODE.md contain concurrent repository changes outside this CLI commit. | open |
| 2026-08-27 | 8cb04dea | fix(cli): close fifth pipe review findings | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-27 | 8cb04dea | fix(cli): close fifth pipe review findings | full ze-precommit-verify over this commit's Go | Focused command, registry, docvalid, and documentation drift tests cover this review-fix chunk; the periodic full gate has not run. | open |
| 2026-08-27 | 8cb04dea | fix(cli): close fifth pipe review findings | discovery-index freshness | ai/PACKAGE-MAP.md and ai/DOCS-TO-CODE.md contain concurrent repository changes outside this CLI commit. | open |
| 2026-08-27 | 8cb04dea | fix(cli): close sixth pipe review findings | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-27 | 8cb04dea | fix(cli): close sixth pipe review findings | full ze-precommit-verify over this commit's Go | Full command package, production-runner coverage tests, structured docvalid mutations, and documentation drift checks cover this review-fix chunk; the periodic full gate has not run. | open |
| 2026-08-27 | 8cb04dea | fix(cli): close sixth pipe review findings | discovery-index freshness | ai/PACKAGE-MAP.md and ai/DOCS-TO-CODE.md contain concurrent repository changes outside this CLI commit. | open |
| 2026-08-27 | 8cb04dea | feat(le): move tooling into the cmd/ze personality | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-27 | 8cb04dea | feat(le): move tooling into the cmd/ze personality | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-27 | 8cb04dea | feat(le): move tooling into the cmd/ze personality | independent critical review | Independent review will run against this committed internal/le and personality cutover. | open |
| 2026-08-27 | 8cb04dea | fix(le): probe committed tree with le basename | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-27 | 8cb04dea | fix(le): probe committed tree with le basename | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-27 | 8cb04dea | fix(le): probe committed tree with le basename | independent critical review | Independent review will cover the final committed le cutover and this inventory fix together. | open |
| 2026-08-27 | 8cb04dea | fix(demo): show health banner and visible commands | ze-precommit-verify (not FRESH-green) | Focused source tests and an isolated health-reports render pass; no fresh repository-wide verify status exists. | open |
| 2026-08-27 | 8cb04dea | fix(demo): arm config response before submit | ze-precommit-verify (not FRESH-green) | An isolated web-config render passed; no fresh repository-wide verify status exists. | open |
| 2026-08-27 | 8cb04dea | fix(cli): validate count followers before folding | ze-precommit-verify (not FRESH-green) | Focused command tests passed; no fresh repository-wide verify status exists. | open |
| 2026-08-27 | 8cb04dea | fix(cli): validate count followers before folding | full ze-precommit-verify over this commit's Go | Folded count-follower and formatted-line tests passed; the periodic full gate has not run. | open |
| 2026-08-27 | 8cb04dea | test(cli): bind coverage to observed completion | ze-precommit-verify (not FRESH-green) | Focused registry coverage tests passed; no fresh repository-wide verify status exists. | open |
| 2026-08-27 | 8cb04dea | test(cli): bind coverage to observed completion | full ze-precommit-verify over this commit's Go | Production-runner, completion-marker, and registration-population tests passed; the periodic full gate has not run. | open |
