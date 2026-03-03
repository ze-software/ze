# 310 — Chaos Dashboard: Peer Text Search/Filter

## Objective

Add a debounced text search input to the chaos dashboard peer table filter bar, filtering peers server-side by index or status text and combining with the existing status dropdown via AND logic.

## Decisions

- Server-side filtering chosen over client-side JS — all rendering is Go HTML via HTMX; no JS framework.
- `peerMatchesSearch()` helper extracted from `handlePeers` to keep logic testable in isolation.
- `hx-include` on both the search input and the status select ensures each preserves the other's value when either fires.

## Patterns

- Extending an existing HTMX handler with a new query param follows the same pattern as existing `sort`/`dir`/`status` params: `r.URL.Query().Get("name")`, filter in loop.
- The `.filters input` CSS class already existed; no new styles needed for the input element.
- Debounce via HTMX `hx-trigger="keyup changed delay:200ms"` — no JS required.

## Gotchas

- Chaos simulation peers have no meaningful address in `PeerState`; address-based search was dropped from scope.
- No chaos functional test infrastructure (`.ci`) exists — functional test was skipped; unit tests fully cover the feature.

## Files

- `cmd/ze-chaos/web/handlers.go` — `peerMatchesSearch()` + `search` param in `handlePeers`
- `cmd/ze-chaos/web/render.go` — search input element in filter bar, updated `hx-include` on status select
- `cmd/ze-chaos/web/handlers_test.go` — 14 new test cases
