# 153 — Chaos UX 1: Peer Grid Toggle View

## Objective

Add a compact grid view (28x28px cells per peer, colored by status) as an alternative to the existing table view in the ze-chaos web dashboard, with a toggle button in the filter bar.

## Decisions

- Grid shows ALL peers (not just the active set), making it fundamentally different from the table. Avoids active set promotion complexity and delivers "500 peers at a glance" directly.
- HTMX polling (every 2s at `/peers/grid`) for grid refresh instead of dual-fragment SSE. SSE broker broadcasts identical HTML to all clients, making per-client view mode tracking impractical without significant broker changes.
- Separate endpoints `GET /peers/grid` and `GET /peers/table` instead of a `?view=` query param — cleaner routing, avoids conditional logic inside a single handler.
- `#peer-display` wrapper div enables clean toggle without breaking existing sort header targets (`#peer-tbody`).
- `writePeerTable()` was added to duplicate the `<thead>` from `writeLayout()` — necessary because toggling back to table mode needs the full table structure including headers.

## Gotchas

- Functional test `test/chaos/grid-view.ci` was skipped — requires HTTP test infrastructure for view toggle that didn't exist. SSE per-cell rendering was also changed to polling for the same infrastructure reason.

## Files

- `cmd/ze-chaos/web/render.go` — `writePeerGrid`, `writePeerGridFiltered`, `writePeerTable`, toggle buttons, `#peer-display`
- `cmd/ze-chaos/web/handlers.go` — `handlePeersGrid`, `handlePeersTable`, routes
- `cmd/ze-chaos/web/dashboard.go` — `renderPeerCell`
- `cmd/ze-chaos/web/assets/style.css` — `.peer-grid`, `.peer-cell`, `.view-toggle` styles
