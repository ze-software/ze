# 311 — Chaos Dashboard: Convergence Trend Rolling Percentile Chart

## Objective

Add a rolling percentile chart (p50/p90/p99) showing convergence latency trends over the last N events, as a CSS-only panel updated via a new `convergence-trend` SSE event alongside the existing histogram.

## Decisions

- New `RingBuffer[time.Duration]` field on `DashboardState` (capacity 1000) — reuses the existing generic `RingBuffer[T]` from `state.go` rather than introducing a new collection type.
- Percentile computation (`sort copy + index`) at render time under read lock, not in the hot `ProcessEvent` path — keeps the push O(1).
- New file `viz_convergence_trend.go` created rather than adding to `viz.go` (already 1349 lines, above the 1000-line split threshold).
- `convergence-trend` SSE event added as a new event type at the same ~2s convergence broadcast interval rather than extending the existing `convergence` event — keeps swap targets independent.

## Patterns

- New viz panels go in separate files when `viz.go` exceeds threshold.
- Rolling buffer push in `ProcessEvent` alongside existing recording keeps the hot path at a single function call.
- CSS-only charts: inline `style="width:X%"` proportional to value vs. max, CSS class per percentile for color.

## Gotchas

- Trend chart was not added as a standalone navigation tab — available via Panels mode and direct URL only.
- No `.ci` functional test — chaos functional test infrastructure does not exist.

## Files

- `cmd/ze-chaos/web/state.go` — `ConvergenceTrend` field, `ComputeConvergencePercentiles()`
- `cmd/ze-chaos/web/dashboard.go` — push in `ProcessEvent`, broadcast in `broadcastDirty`
- `cmd/ze-chaos/web/viz_convergence_trend.go` — render function + HTTP handler (created)
- `cmd/ze-chaos/web/viz_convergence_trend_test.go` — 11 unit tests (created)
- `cmd/ze-chaos/web/assets/style.css` — `.trend-bars`, `.trend-row`, `.trend-p50/p90/p99`
