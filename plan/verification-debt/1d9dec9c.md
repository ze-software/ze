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
| 2026-08-22 | 1d9dec9c | test(flowspec): prove the one protocol table, from wire to kernel | ze-precommit-verify (not FRESH-green) | no verify record exists in this shared checkout (STALE: never verified); this commit adds tests, docs and plan files only, changes no production Go, and its own gates are green: ze-repository-check, ze-doc-wiring-check, ze-test-weakened-check, ze-spec-citation-check, and lint 0 issues over both changed packages host and GOOS=linux | open |
| 2026-08-22 | 1d9dec9c | test(flowspec): prove the one protocol table, from wire to kernel | full ze-precommit-verify over this commit's Go | the Go in this commit is two test files; make ze-unit-pkg-test over the five affected packages is ok, and lint is 0 issues host and GOOS=linux. A full ze-precommit-verify would judge another session's in-flight pkg/plugin/rpc edits, which currently do not compile | open |
| 2026-08-22 | 1d9dec9c | chore(plan): close fixit-flowspec-protocol-name-drift | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | fix(ike): install the selectors a child rekey answered, on both roles | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | fix(ike): install the selectors a child rekey answered, on both roles | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-22 | 1d9dec9c | chore(plan): close fixit-child-rekey-answer-vs-installed-selectors | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | test(ci): remove a reject that outlived the refusal it named | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | chore(plan): close fixit-vacuous-functional-tests | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | build: derive GOTOOLCHAIN from the go.mod toolchain line | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | fix(ze-run): key an admitted job on its parameters, separator or not | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | firewall-irr: pin the empty-answer guard at every consumer | ze-precommit-verify (not FRESH-green) | verify-status reports never verified on this checkout; this closure ran lint (0 issues, host and GOOS=linux integration), ze-repository-check, ze-doc-verify, ze-doc-links-check, the weakened-test and relaxation audits, and the four IRR packages plus internal/test/runner. The worktree gate covers it on the next cadence run; nothing is pushed. | open |
| 2026-08-22 | 1d9dec9c | firewall-irr: pin the empty-answer guard at every consumer | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-22 | 1d9dec9c | spec: close fixit-irr-empty-answer-clears-set | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | docs: repair the citations a closed spec left dangling | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
