# Deferrals: fixit-ipsec-clear-reestablish

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-ipsec-clear-reestablish functional-proof | strongSwan interop scenarios 10+11 need Docker+charon, deferred to CI (AC-5) | live-server/QEMU constraint, deferred to CI | plan/spec-finish-ci-coverage.md | deferred |

