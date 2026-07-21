# Deferrals: radius-subscriber-attributes

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-radius-subscriber-attributes (Known Limitations) | Interim-update accounting scheduling (timer wheel) | The subscriber-attributes spec covers packet content only; scheduling is the sibling spec's concern | `plan/spec-radius-acct-timewheel.md` | deferred |
| 2026-07-10 | spec-radius-subscriber-attributes | Adjacent verified RADIUS gaps: Calling-Station-Id (31), Event-Timestamp (55), Acct-Delay-Time (41), Acct-Terminate-Cause (49), present in the dictionary but not emitted | DESIGN decides whether these join the subscriber-attributes scope or stay out; flagged so the decision is not lost | `plan/spec-radius-subscriber-attributes.md` (DESIGN-phase decision) | deferred |

