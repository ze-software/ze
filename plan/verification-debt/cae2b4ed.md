# Verification debt -- commit session cae2b4ed

Gates that had not run green over these commits when they were made.
Each row is work owed, not a defect: a defect is fixed, never recorded
(`ai/rules/completion.md`). Clear a row by running the gate over the
committed code and setting Status to `cleared`, or delete the shard once
every row is cleared. `scripts/dev/commit_helper.py create --push` refuses
while any row here is open.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-18 | cae2b4ed | rules(cli): one point per CLI directive, and drop an empty tracker | ze-precommit-verify (not FRESH-green) | markdown-only under ai/, the does-not-apply row of ai/rules/precommit-verify.md; the structural gates owning this surface are green: ze-rules-lint, ze-rules-points-roundtrip-check, ze-rules-gate-map-report, ze-doc-verify | open |
| 2026-08-18 | cae2b4ed | rules(cli): one point per CLI directive, and drop an empty tracker | ze-precommit-verify structural gates (red) | ze-doc-wiring-check is red because ai/rules/planning.md is unrendered against a point another session edited and has not committed; ai/rules/rule-format.md rules that I MUST NOT render another session's uncommitted points, and this commit touches no planning point | open |
| 2026-08-19 | cae2b4ed | test(cli): hold the structured-payload rule over the show surface | ze-precommit-verify (not FRESH-green) | owner ordered the commit without waiting for a full run; the changed surface is verified: ze-unit-pkg-test on internal/component/plugin/server green and ze-lint-changed 0 issues | open |
| 2026-08-19 | cae2b4ed | test(cli): hold the structured-payload rule over the show surface | ze-precommit-verify structural gates (red) | every red in the record is another session's, from a ze-precommit-verify-changed run at 23:02 that predates this work: ze-lint-changed names internal/component/ssh/ssh.go gofmt, ze-doc-wiring-check is the Hook-to-Rule Mapping row missing for style_guide_reminder in an uncommitted .claude/hooks/pretool-agent-skill.py, and this commit adds one test file that cannot affect either | open |
| 2026-08-19 | cae2b4ed | test(cli): hold the structured-payload rule over the show surface | full ze-precommit-verify over this commit's Go | owner ordered the commit rather than waiting for make ze-precommit-verify, which was terminated at stage 13 of 30; the commit adds one test file and changes no production code | open |
| 2026-08-19 | cae2b4ed | fix(cli): RawJSON refuses a payload that is not JSON | ze-precommit-verify (not FRESH-green) | owner-ordered change verified by package: api, api/rest, api/grpc, mcp, web, lg, plugin, plugin/server and cmd/ze/hub all green, golangci-lint 0 issues over those packages | open |
| 2026-08-19 | cae2b4ed | fix(cli): RawJSON refuses a payload that is not JSON | ze-precommit-verify structural gates (red) | the record's reds are other sessions': ze-lint-changed names internal/component/ssh/ssh.go gofmt and internal/plugins/flowspec-firewall/bridge_test.go typecheck, ze-doc-wiring-check the missing Hook-to-Rule row for style_guide_reminder. golangci-lint over every package this commit touches is 0 issues | open |
| 2026-08-19 | cae2b4ed | fix(cli): RawJSON refuses a payload that is not JSON | full ze-precommit-verify over this commit's Go | make ze-precommit-verify cannot run green in this shared tree: ze-lint-changed is red on internal/plugins/flowspec-firewall/bridge_test.go, another session's mid-edit file | open |
| 2026-08-19 | cae2b4ed | docs(journal): close the RawJSON row the owner decided | ze-precommit-verify (not FRESH-green) | one journal row edited; ai/rules/precommit-verify.md puts plan/**/*.md in the does-not-apply row, and make ze-journal-report is green | open |
| 2026-08-19 | cae2b4ed | docs(journal): close the RawJSON row the owner decided | ze-precommit-verify structural gates (red) | the reds are another session's mid-edit internal/plugins/flowspec-firewall/bridge_test.go typecheck and internal/component/ssh/ssh.go gofmt; this commit edits one markdown row and can affect neither | open |
