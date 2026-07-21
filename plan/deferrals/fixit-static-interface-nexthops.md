# Deferrals: fixit-static-interface-nexthops

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-static-interface-nexthops functional-proof | test/static/005+006 .ci are needs-linux QEMU, deferred to CI (AC-7) | live-server/QEMU constraint, deferred to CI | plan/spec-finish-appliance-qemu-evidence.md | deferred |

