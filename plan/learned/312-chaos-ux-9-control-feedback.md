# 312 — Chaos Dashboard: Control Panel Rate and Speed Feedback

## Objective

Show live chaos event rate (events/sec, color-coded) and current speed multiplier in the stats SSE fragment, using an EMA computed from `TotalChaos` deltas at the existing broadcast interval.

## Decisions

- EMA chaos rate integrated into existing `UpdateThroughput()` rather than a standalone method — keeps all EMA logic co-located and avoids a second iteration over state.
- Color thresholds: green < 1/s, yellow 1-5/s, red > 5/s — expressed as `ChaosRateColorClass()` helper returning CSS class names, keeping logic unit-testable.
- Speed factor readback via `speedStat()` helper, shown only when `ControlState.SpeedAvailable` is true — mirrors how the control panel hides the speed section.
- Functional test skipped — feature is purely visual; unit tests prove the values appear in rendered HTML.

## Patterns

- EMA pattern: `prevTotalChaos` + `chaosRate float64` fields on `DashboardState`; compute delta per broadcast tick.
- Color-coded stat: return a CSS class name from a helper, apply as `class="%s"` in HTML span.

## Gotchas

- None.

## Files

- `cmd/ze-chaos/web/state.go` — `prevTotalChaos`, `chaosRate`, `ChaosRate()`, `ChaosRateColorClass()`
- `cmd/ze-chaos/web/dashboard.go` — chaos rate EMA in `UpdateThroughput`, rate/speed spans in `renderStats`, `speedStat()` helper
- `cmd/ze-chaos/web/assets/style.css` — `.rate-green`, `.rate-yellow`, `.rate-red`
