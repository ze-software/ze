# Deferrals: fixit-firewall-concurrency-deadlock

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-firewall-concurrency-deadlock functional-proof | sibling slices D-2/D-3/D-4 + core-design note remain (spec stays open) | live-server/QEMU constraint, deferred to CI | plan/spec-fixit-firewall-concurrency-deadlock.md | deferred |

