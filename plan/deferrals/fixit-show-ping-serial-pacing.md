# Deferrals: fixit-show-ping-serial-pacing

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-show-ping-serial-pacing functional-proof | privileged CAP_NET_RAW/QEMU batch-shape proof deferred to CI | live-server/QEMU constraint, deferred to CI | plan/spec-finish-appliance-qemu-evidence.md | deferred |

