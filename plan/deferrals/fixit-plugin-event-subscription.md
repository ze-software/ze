# Deferrals: fixit-plugin-event-subscription

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-plugin-event-subscription functional-proof | functional .ci for the SDK-fork end-to-end path not authored (unrunnable here) | live-server/QEMU constraint, deferred to CI | plan/spec-finish-ci-coverage.md | deferred |

