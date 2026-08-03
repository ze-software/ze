# Deferrals: fixit-perf-alloc-ci-gate

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-perf-alloc-ci-gate functional-proof | make ze-alloc-gate full run (needs bench log) deferred to CI/nightly | live-server/QEMU constraint, deferred to CI | plan/spec-finish-ci-coverage.md | deferred |

