# Verification debt -- commit session 02aedf0f

Gates that had not run green over these commits when they were made.
Each row is work owed, not a defect: a defect is fixed, never recorded
(`ai/rules/completion.md`). Clear a row by running the gate over the
committed code and setting Status to `cleared`, or delete the shard once
every row is cleared. the retired `scripts/dev/commit_helper.py create --push` (current producer: `internal/le/commit/prepare.go`) refuses
while any row here is open.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-18 | 02aedf0f | build: let a commit land with an unrun gate recorded as debt | ./le verify current mode full structural gates (red) | All reds are other sessions': ./le verify lint run is Go-only and this commit carries no Go; ./le doc check links fails on 16 plan/spec-ipsec-* citations from an in-flight test/ipsec-interop rename; ./le doc wiring fails on a retired root Makefile (current producers: internal/le/ native action tables) suite derivation broken by another session's uncommitted the retired Makefile (current producers: `internal/le/` native action tables) hunks; ./le repository generated-check fails on mk/test-fuzz-targets.mk (retired; current producer: `internal/le/fuzz/discover.go`), stale from another session's uncommitted Go. | cleared |
| 2026-08-18 | 02aedf0f | build: let a commit land with an unrun gate recorded as debt | discovery-index freshness | ai/PACKAGE-MAP.md still carries three rows for internal/plugins/anomaly/observe*, packages another session has uncommitted; committing it would put rows in HEAD for packages HEAD lacks. | cleared |
| 2026-08-18 | 02aedf0f | fix: restore the rules corpus floor and HEAD's rule render | ./le verify current mode full structural gates (red) | Same other-session reds as commit c2e3e17fe, plus one this commit narrows rather than clears: check_doc_links still refuses ze-run.sh as a dead name because that script is untracked in another session's work. This commit removes the render/point disagreement; only their commit of the script can clear the name. | cleared |
| 2026-08-18 | 02aedf0f | fix: restore the rules corpus floor and HEAD's rule render | discovery-index freshness | ai/PACKAGE-MAP.md still carries three rows for internal/plugins/anomaly/observe*, packages another session has uncommitted. | cleared |
