# Verification debt -- commit session 02aedf0f

Gates that had not run green over these commits when they were made.
Each row is work owed, not a defect: a defect is fixed, never recorded
(`ai/rules/completion.md`). Clear a row by running the gate over the
committed code and setting Status to `cleared`, or delete the shard once
every row is cleared. `scripts/dev/commit_helper.py create --push` refuses
while any row here is open.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-18 | 02aedf0f | build: let a commit land with an unrun gate recorded as debt | ze-precommit-verify structural gates (red) | All reds are other sessions': ze-lint is Go-only and this commit carries no Go; ze-doc-links-check fails on 16 plan/spec-ipsec-* citations from an in-flight test/ipsec-interop rename; ze-doc-wiring-check fails on a Makefile suite derivation broken by another session's uncommitted Makefile hunks; ze-generated-files-check fails on mk/test-fuzz-targets.mk, stale from another session's uncommitted Go. | open |
| 2026-08-18 | 02aedf0f | build: let a commit land with an unrun gate recorded as debt | discovery-index freshness | ai/PACKAGE-MAP.md still carries three rows for internal/plugins/anomaly/observe*, packages another session has uncommitted; committing it would put rows in HEAD for packages HEAD lacks. | open |
