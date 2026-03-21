# 154 — Chaos UX 2: Health Donut Ring Chart

## Objective

Replace the flat "Peers Up/Total" counter in the sidebar Stats card with an SVG donut ring chart showing peer status distribution (Up/Syncing/Down/Reconnecting/Idle as colored segments).

## Decisions

- SVG circle with `stroke-dasharray` for segments — pure CSS/SVG, no charting library. Single SVG element approach is simpler than multiple arc path elements.
- Both `renderStats()` (SSE path) and `writeLayout()` (initial page load) must be updated to include the donut — they must stay structurally identical or the SSE swap will visually break on first update.
- `StatusCounts()` returns `[5]int` indexed by `PeerStatus` iota — avoids a map allocation on every broadcast tick.
- Removed `syncingStatInline()` from `renderStats()` since syncing count is now in the donut legend — no duplication needed.

## Patterns

- CSS custom properties (`--green`, `--red`, `--accent`, `--yellow`, `--text-muted`) referenced from inline SVG `style` attributes — keeps color management in one place.

## Gotchas

- Zero-peer edge case: must show empty grey ring with "0" center, not divide by zero when computing arc lengths. The donut circumference formula uses total=0 as a special case returning the idle-colored full ring.

## Files

- `cmd/ze-chaos/web/render.go` — `writeDonut()`, `writeDonutLegend()`, `donutStatusOrder`, writeLayout update
- `cmd/ze-chaos/web/dashboard.go` — `renderStats()` updated
- `cmd/ze-chaos/web/state.go` — `StatusCounts()` method
- `cmd/ze-chaos/web/assets/style.css` — donut container, legend, center text styles
