# 267 — Chaos Web Visualization Tabs

## Objective

Add four visualization tabs (Event Stream, Peer State Timeline, Convergence Histogram, Chaos Timeline) to the chaos web dashboard using server-side HTML rendering with HTMX and SSE — no JavaScript charting libraries.

## Decisions

- All tab content rendered inline in `viz.go` rather than separate template files — avoids template loading complexity for embedded HTML fragments.
- Tabs loaded lazily via HTMX GET on first click, then updated via SSE; initial state snapshot served synchronously on load.
- Convergence histogram uses 9 fixed buckets with CSS-only bar chart; color gradient encodes latency severity.
- Timeline pagination fixed at configurable page size (default 30) for 200+ peer scale — not virtual scrolling.

## Patterns

- `broadcastDirty` SSE mechanism from foundation drives all live updates (convergence every 2s, event feed on arrival).
- State mutations (histogram buckets, chaos history) happen in `ProcessEvent()` under the same write lock as peer table — no separate locks needed.
- Ring buffer from foundation backs the event stream; filtering applied at render time, not at insertion time.

## Gotchas

- AC-13 (click-to-highlight peer on chaos event marker) was not fully implemented — markers render but clicking does not highlight peer in peer table.
- SSE convergence 2s interval and event-feed 10/s throttle were not tested with dedicated interval tests; only via the `broadcastDirty` mechanism.
- Functional `.ci` test was skipped entirely — no end-to-end HTTP test for visualization endpoints.

## Files

- `cmd/ze-bgp-chaos/web/viz.go` — all visualization rendering (670 lines, created)
- `cmd/ze-bgp-chaos/web/viz_test.go` — 912 lines of tests (created)
- `cmd/ze-bgp-chaos/web/state.go`, `dashboard.go`, `sse.go`, `handlers.go` — extended
