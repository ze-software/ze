# Deferrals: fixit-fuzz-target-discovery

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-fuzz-target-discovery functional-proof | bounded ze-fuzz-test mutation run over newly-enabled ISIS/OSPF deferred to CI | live-server/QEMU constraint, deferred to CI | plan/spec-finish-ci-coverage.md | deferred |

