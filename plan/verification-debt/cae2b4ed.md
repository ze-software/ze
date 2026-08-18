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
