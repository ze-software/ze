# 357 — Chaos Web Dashboard

## Objective

Build a live web dashboard for ze-chaos providing real-time visualization, peer drill-down, interactive controls, and advanced visualizations (convergence histogram, peer timeline, chaos timeline, route flow matrix). Activated via `--web` flag, runs as a `report.Consumer` alongside terminal dashboard and JSON log.

## Decisions

- HTMX + SSE over WebSockets — server-push only, `hx-swap-oob` for multi-target updates from a single SSE event
- Inline Go HTML rendering (`render.go`) instead of `html/template` files — simpler for this volume of HTML
- Active set (~40 peers visible) with adaptive TTL decay (5s at >80% fill, 120s at <50%) and user pinning
- 200ms SSE debounce — ProcessEvent() sets dirty flags only (no I/O under write lock), background goroutine broadcasts
- Shared HTTP mux when `--web` and `--metrics` both specified — single port serves dashboard + Prometheus
- Controls (pause/resume/rate/trigger/stop/restart) implemented via buffered control channel to orchestrator event loop
- 6 new parameterized chaos actions (ClockDrift, RouteBurst, etc.) deferred to spec-chaos-actions-v2

## Patterns

- ProcessEvent() writes state under write lock, sets dirty flags — HTTP handlers take read locks. No I/O under write lock.
- Control channel mirrors event channel — both are buffered Go channels processed in the same select loop
- `go:embed` for self-contained binary — htmx.min.js, sse.js, style.css bundled, works offline

## Gotchas

- Functional .ci tests require running ze-chaos with live events — current framework cannot drive interactive HTTP sessions; used unit tests + manual verification instead
- Template files were planned but inline Go rendering proved simpler — `web/templates/` directory empty
- Scheduler pause/resume implemented via control channel rather than direct Scheduler methods — avoids mutex complexity

## Files

- `cmd/ze-chaos/web/` — dashboard.go, handlers.go, sse.go, state.go, control.go, viz.go, render.go
- `cmd/ze-chaos/web/assets/` — htmx.min.js, sse.js, style.css (dark theme)
- `cmd/ze-chaos/main.go` — `--web` flag, control channel, setupReporting wiring
- Sub-summaries: 266 (foundation), 267 (viz), 268 (route-matrix), 307-314 (UX iterations)
