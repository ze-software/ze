# 473 -- SSE Live Updates

## Context
When one user commits config changes, other web sessions need to know without polling. Server-Sent Events (SSE) push notifications to all connected browsers. The chaos dashboard already proved the SSE + HTMX pattern works in Ze.

## Decisions
- SSE with pre-rendered HTML over SSE with raw text + client-side templating -- Go's `html/template` auto-escapes the notification content server-side, preventing XSS via HTMX `innerHTML` insertion
- Non-blocking broadcast (drop if buffer full) over blocking send -- one slow client cannot block notifications to others
- Session cookie auth for SSE over token-in-query-param -- cookies are sent automatically with EventSource connections
- Maximum 100 concurrent SSE clients over unlimited -- prevents file descriptor exhaustion

## Consequences
- SSE notifications are HTML fragments with `hx-swap-oob` for out-of-band insertion into the notification area -- HTMX handles DOM insertion without custom JS
- The commit handler calls `BroadcastConfigChange()` only after `CommitSession()` succeeds -- no notification on failed or conflicted commits
- CLI commits need an `OnCommit` callback on the Editor to reach the SSE broker -- this is the coupling point between CLI and web components

## Gotchas
- HTMX's SSE extension inserts event data via `innerHTML` -- if the data contained unescaped HTML, it would be an XSS vector. Pre-rendering through `html/template` is the mitigation
- The notification banner includes a "Refresh" button targeting `#content-area` -- in terminal mode, this would replace the terminal with GUI content. Documented as accepted behavior
- SSE connections are long-lived -- credential revocation during an active connection is not detected until the client reconnects

## Files
- `internal/component/web/sse.go`, `sse_test.go`
- `internal/component/web/templates/notification_banner.html`
- `internal/component/web/handler_config.go` (commit broadcast hook)
