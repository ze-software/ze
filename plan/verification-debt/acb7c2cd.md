# Verification debt -- commit session acb7c2cd

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
| 2026-08-22 | acb7c2cd | fix(commit-helper): read a closure signal as the committing session's own | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | fix(commit-helper): a closure signal is the committing session's own | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | plan: three defects found while draining the fixit backlog | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | docs(storage): the path-to-key docs name both resolvers | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | spec: close fixit-zefs-diff-structural-ops | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | ike: close the resource-lifetime spec and pair the SA events in docs | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | plan: close spec-fixit-ike-resource-lifetime-leaks | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | plan: the functional suite runs a backend Ze does not ship | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | rules: three points so today's lessons bind the next session | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | plan: repair the citations closing specs left dangling | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | test(rib): the best-path walk .ci reads back what it asserts | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | test(runner): the debt-clear fixture repos carry a commit | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | docs(journal): the row names the file, not the file minus its suffix | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | rules: a discrimination proof verifies its mutation applied | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | fix(tools): a spec move is not a spec closure | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | test(tools): the go twin asserts the refusal the code emits | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | test(tools): the go twin asserts the refusal the code emits | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-22 | acb7c2cd | plan: the plugin-concurrency spec waits for the first release | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | fix(plugin): every spawn clears the stop re-entry guard | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | fix(plugin): every spawn clears the stop re-entry guard | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-22 | acb7c2cd | plan: a parked spec reads ready, not in-progress | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | fix(firewall-irr): a term needs both family sets or neither | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | acb7c2cd | fix(firewall-irr): a term needs both family sets or neither | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-23 | acb7c2cd | fix(firewall-irr): a term needs both family sets or neither | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-23 | acb7c2cd | fix(firewall-irr): a term needs both family sets or neither | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-23 | acb7c2cd | fix(plugin): every spawn clears the stop re-entry guard | ze-precommit-verify (not FRESH-green) | concurrent-session interference in a shared checkout. ze-precommit-verify-changed reached stage 13 of 27 with two reds, and neither is in this commit's files: ze-lint-changed fails on internal/component/firewall/plugins/irr/irr_test.go with 'undefined: assertTermSets', and ze-fs-persistence-check fails on an os.OpenFile of a loose ze-dhcpmu-hold log under the system temp directory in internal/component/iface/register.go. Both files are another session's uncommitted work. This commit's own packages lint clean: make ze-lint-changed standalone reported '0 issues.' on both the host and the GOOS=linux pass. ze-tier-check, ze-iface-resolution-check, ze-plugin-boundary-check, ze-rfc-check, ze-config-coercion-check, ze-dash-stdio-check, ze-port-defaults-check, ze-config-claims-check, ze-test-sensitivity-check and ze-test-weakened-check all exit 0. The full gate over this commit is owed and stays owed; nothing is pushed. | open |
| 2026-08-23 | acb7c2cd | fix(plugin): every spawn clears the stop re-entry guard | full ze-precommit-verify over this commit's Go | no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-23 | acb7c2cd | plan: close spec-fixit-shutdown-waits-out-a-deadlock | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-23 | acb7c2cd | plan: close spec-fixit-firewall-irr-term-fails-validation | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-23 | acb7c2cd | rules: restore a mutated file by copy, never by a banned verb | ze-precommit-verify (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
