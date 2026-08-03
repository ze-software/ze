# Deferrals: fixit-doc-gate-and-refs

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-doc-gate-and-refs functional-proof | the optional local check-doc-drift.sh hook is owned by a separate follow-up | live-server/QEMU constraint, deferred to CI | plan/spec-finish-ci-coverage.md | deferred |

