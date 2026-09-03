# Deferrals: radius-subscriber-attributes

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-radius-subscriber-attributes (Known Limitations) | Interim-update accounting scheduling (timer wheel) | The subscriber-attributes spec covers packet content only; scheduling is the sibling spec's concern **MEASURED 2026-09-03, and the row's two halves have separated.** The BEHAVIOUR exists: `(*radiusAcct).interimLoop` (`internal/component/l2tp/plugins/authradius/acct.go`) schedules an interim per session on its own ticker, and re-reads client, NAS id and source address each tick so a reload takes effect without restarting the loop. What does not exist is the SHAPE the row names, and that is not a stylistic preference: one goroutine and one `time.Ticker` per subscriber is a per-session cost a BNG pays at subscriber scale, which is what a timer wheel exists to remove. So this is no longer missing scheduling, it is a scale question about the scheduling ze has, and it wants a measurement at a realistic session count before anyone builds a wheel. | `plan/future/spec-radius-acct-timewheel.md` | deferred |
| 2026-07-10 | spec-radius-subscriber-attributes | Adjacent verified RADIUS gaps: Calling-Station-Id (31), Event-Timestamp (55), Acct-Delay-Time (41), Acct-Terminate-Cause (49), present in the dictionary but not emitted | DESIGN decides whether these join the subscriber-attributes scope or stay out; flagged so the decision is not lost | `plan/spec-radius-subscriber-attributes.md` (DESIGN-phase decision) | deferred |

