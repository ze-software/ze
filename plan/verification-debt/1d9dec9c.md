# Verification debt -- commit session 1d9dec9c

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
| 2026-08-22 | 1d9dec9c | ipsec: prove a responder-role rekey against strongSwan | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | ipsec: prove a responder-role rekey against strongSwan | full ./le verify current mode full over this commit's Go | no full ./le verify current mode full recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-22 | 1d9dec9c | plan: close spec-fixit-child-sa-rekey-policy | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | fix(status): the spec inventory counts every spec it reads | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | fix(status): the spec inventory counts every spec it reads | full ./le verify current mode full over this commit's Go | no full ./le verify current mode full recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-22 | 1d9dec9c | chore(plan): close fixit-spec-status-metadata-window | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | iface: close the selector-ignored-by-apply spec | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | iface: close the selector-ignored-by-apply spec | ./le verify current mode full (not FRESH-green) | no verify has run against this tree; the commit carries no code, so no gate reads anything it changes. The five code commits carry their own debt rows | open |
| 2026-08-22 | 1d9dec9c | plan: close spec-fixit-iface-selector-ignored-by-apply | ./le verify current mode full (not FRESH-green) | the commit removes one markdown file and no gate reads it | open |
| 2026-08-22 | 1d9dec9c | test(flowspec): prove the one protocol table, from wire to kernel | ./le verify current mode full (not FRESH-green) | no verify record exists in this shared checkout (STALE: never verified); this commit adds tests, docs and plan files only, changes no production Go, and its own gates are green: ./le repository check, ./le doc wiring, ./le test-weakened check, ./le spec citation anchors, and lint 0 issues over both changed packages host and GOOS=linux | open |
| 2026-08-22 | 1d9dec9c | test(flowspec): prove the one protocol table, from wire to kernel | full ./le verify current mode full over this commit's Go | the Go in this commit is two test files; the retired make ze-unit-pkg-test (current: direct go test -race <package>) over the five affected packages is ok, and lint is 0 issues host and GOOS=linux. A full ./le verify current mode full would judge another session's in-flight pkg/plugin/rpc edits, which currently do not compile | open |
| 2026-08-22 | 1d9dec9c | chore(plan): close fixit-flowspec-protocol-name-drift | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | fix(ike): install the selectors a child rekey answered, on both roles | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | fix(ike): install the selectors a child rekey answered, on both roles | full ./le verify current mode full over this commit's Go | no full ./le verify current mode full recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-22 | 1d9dec9c | chore(plan): close fixit-child-rekey-answer-vs-installed-selectors | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | test(ci): remove a reject that outlived the refusal it named | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | chore(plan): close fixit-vacuous-functional-tests | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | build: derive GOTOOLCHAIN from the go.mod toolchain line | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | fix(ze-run): key an admitted job on its parameters, separator or not | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | firewall-irr: pin the empty-answer guard at every consumer | ./le verify current mode full (not FRESH-green) | verify-status reports never verified on this checkout; this closure ran lint (0 issues, host and GOOS=linux integration), ./le repository check, ./le doc check verify, ./le doc check links, the weakened-test and relaxation audits, and the four IRR packages plus internal/test/runner. The worktree gate covers it on the next cadence run; nothing is pushed. | open |
| 2026-08-22 | 1d9dec9c | firewall-irr: pin the empty-answer guard at every consumer | full ./le verify current mode full over this commit's Go | no full ./le verify current mode full recorded (tmp/ze-verify-full.json is missing) | open |
| 2026-08-22 | 1d9dec9c | spec: close fixit-irr-empty-answer-clears-set | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-22 | 1d9dec9c | docs: repair the citations a closed spec left dangling | ./le verify current mode full (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
