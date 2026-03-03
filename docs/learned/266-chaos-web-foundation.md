# 266 — Chaos Web Dashboard Foundation

## Objective

Implement Phase 1 of a real-time web dashboard for ze-bgp-chaos: HTTP server with embedded assets, Server-Sent Events (SSE) broker, peer table with active set management (auto-promotion, adaptive decay, pinning), and a peer detail pane.

## Decisions

- HTMX + SSE over WebSockets — SSE is server-push only (no client messages needed for live updates); simpler than WebSockets and naturally handles reconnect. HTMX handles fragment swapping without a JS framework.
- `hx-swap-oob` for multi-target SSE updates — a single SSE event can update header counters, sidebar, and peer table simultaneously without separate channels.
- 200ms SSE debounce (5 updates/sec max) — ProcessEvent() only sets dirty flags synchronously; a background goroutine reads flags every 200ms and sends SSE fragments. Prevents the HTTP layer from blocking the main event loop.
- Inline Go-rendered HTML in `render.go` instead of `html/template` files — eliminated the template parsing/caching complexity; Go string construction is simpler for this volume of HTML.
- Adaptive TTL for the active set (5s at >80% fill, 30s mid-range, 120s at <50% fill) — makes the ~40-peer view self-managing across runs with very different peer counts and event rates.
- Shared HTTP mux when `--web` and `--metrics` are both specified — avoids binding two ports; `/` serves the dashboard, `/metrics` serves Prometheus.

## Patterns

- ProcessEvent() writes state under a write lock and sets dirty flags only — never does I/O. HTTP handlers and SSE goroutine take read locks. Clean separation prevents deadlock.
- Embedded assets via `go:embed` — self-contained binary, no CDN dependency, works offline.
- Active set pinning: pinned peers are exempt from decay TTL; unpin re-enables normal decay rules.

## Gotchas

- Functional .ci tests skipped entirely — the chaos web dashboard requires a running ze-bgp-chaos process with live events; the current `.ci` test framework cannot drive interactive HTTP sessions.
- `web/templates.go` and individual template HTML files were not created — rendering was done inline in `render.go`, which was discovered to be simpler during implementation.

## Files

- `cmd/ze-bgp-chaos/web/dashboard.go` — WebDashboard consumer, SSE integration, HTTP server
- `cmd/ze-bgp-chaos/web/state.go` — Per-peer state, ring buffer, ActiveSet with adaptive TTL
- `cmd/ze-bgp-chaos/web/sse.go` — SSE broker: client registration, broadcast, 200ms debounce
- `cmd/ze-bgp-chaos/web/handlers.go` — HTTP handlers: peers, peer detail, pin, promote
- `cmd/ze-bgp-chaos/web/render.go` — Inline Go HTML rendering (replaces template files)
- `cmd/ze-bgp-chaos/web/assets/` — Vendored htmx.min.js, sse.js, style.css (dark theme)
