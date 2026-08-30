# Deferrals: fixit-recent-cache-buffer-reclaim

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-recent-cache-buffer-reclaim functional-proof | no privileged pool-pressure QEMU proof; unit-tested via fake pool ratio | live-server/QEMU constraint, deferred to CI. **Triaged 2026-08-30 as an improvement, not a release defect:** the reclaim is unit-proven against a fake pool ratio, so this is added evidence rather than a fix | plan/future/spec-finish-appliance-qemu-evidence.md | deferred |

