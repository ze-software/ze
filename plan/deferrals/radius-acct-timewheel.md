# Deferrals: radius-acct-timewheel

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-radius-acct-timewheel (Known Limitations) | RADIUS accounting packet content (which attributes are emitted) | The timewheel spec covers interim-update scheduling only; packet content is the sibling spec's concern | `plan/spec-radius-subscriber-attributes.md` | deferred |

