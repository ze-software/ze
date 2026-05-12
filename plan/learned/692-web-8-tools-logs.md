# 692 -- web-8-tools-logs

## Context

The operator workbench (web-2/web-3) provided the shell and reusable components but had no interactive diagnostic pages. Operators needed purpose-built tool forms (Ping, BGP Decode, Metrics Query, Capture), real-time log streaming, warning/error tables, and dashboard sub-pages (Health, Events) to complete the workbench as a self-contained operator workstation. All data sources already existed as registered show-command RPCs; this spec wired them into workbench pages with validation, rendering, and streaming.

## Decisions

- **Per-tool handler functions** (`HandleToolPingPage`, `HandleToolBGPDecodePage`, etc.) over a generic "tool page" that switches on a parameter. Each tool has distinct form fields, validation rules, and command construction, so shared code would be an abstraction over one use case.
- **GET renders form, POST validates and dispatches** over separate form/result endpoints. Single URL per tool keeps navigation simple and enables HTMX partial replacement of the result area without a full page reload.
- **CommandDispatcher for all data access** over direct RPC calls. Every show command goes through the same `func(command, username, remoteAddr string) (string, error)` path, preserving authz parity with CLI/API. No handler bypasses the dispatch layer.
- **EventBroker delegation for Live Log SSE** over a custom SSE handler. `HandleLogLiveStream` delegates directly to `broker.ServeHTTP`, reusing the existing subscribe/unsubscribe/broadcast/heartbeat infrastructure. Client connection limits (100) and per-client buffering (16 events) are inherited.
- **Structured JSON parsing with plain-text fallback** for warnings, errors, and events. Handlers attempt `json.Unmarshal` into typed structs; if the output is not JSON (or the format changes), they fall back to line-per-row rendering. This avoids coupling to a single RPC response format.
- **Config-tree presence for health status (v1)** over dispatching "show health". Component health is derived from whether the component has config entries, showing "Configured" (green) or "Not configured" (grey). Real operational state dispatch is deferred to when health RPCs return structured per-component status.
- **Input validation before dispatch** with strict bounds: ping count 1-100, timeout 1-30s, capture count 1-10000, hex-only regex for BGP decode, Prometheus metric name pattern for metrics. Invalid input returns an error without touching the dispatcher.
- **ANSI stripping + HTML escaping + 4 MiB cap** via shared `dispatchToolCommand` helper. All tool output passes through `normalizeOutput` (strips ANSI escapes, enforces size limit) then `template.HTMLEscapeString` before rendering.

## Consequences

- Four tool pages, three log pages, and two dashboard sub-pages are accessible from the workbench left nav under Tools, Logs, and Dashboard sections.
- All pages render inside the workbench shell with consistent navigation highlighting.
- The `ToolPageData` struct is intentionally minimal (Error + Output strings), keeping tool pages stateless and easy to test.
- Dashboard health is placeholder-quality (config presence, not runtime health). This is documented and acceptable for v1.

## Gotchas

- `parseIssueJSON` handles the `report.Issue` JSON shape; if the show warnings/errors RPC format changes, the JSON envelope keys ("warnings", "errors") must match or the fallback plain-text parser takes over silently.
- `formatDuration` rounds to the nearest unit (second, minute, hour), which can show "0s" for sub-second durations. This is acceptable for warning duration display.
- `splitLines` handles both `\n` and `\r\n` but not bare `\r`. Command output from Unix systems will not contain bare `\r`, but output from VPP or external tools might.
- The `renderLogPageContent` function accepts an `*EventBroker` parameter (for future SSE route integration at the page dispatch level) but currently ignores it; Live Log SSE streaming is wired separately in route registration.
- `knownComponents` in `page_dashboard.go` is a hardcoded list. Adding a new component to the health table requires editing this slice. The spec deferred deriving components from the registry.

## Files

- `internal/component/web/page_tools.go` -- tool page handlers (Ping, BGP Decode, Metrics, Capture), input validation, shared dispatch
- `internal/component/web/page_logs.go` -- log page handlers (Live, Warnings, Errors), SSE delegation, issue JSON parsing
- `internal/component/web/page_dashboard.go` -- dashboard sub-pages (Health, Events), config-tree health, event JSON parsing
- `internal/component/web/handler_tool_pages_test.go` -- 20 tests covering form rendering, dispatch, validation, truncation
- `internal/component/web/handler_log_pages_test.go` -- 9 tests covering table rendering, empty states, SSE streaming
- `internal/component/web/handler_dashboard_test.go` -- 8 tests covering panel rendering, empty states, namespace filtering
- `internal/component/web/templates/component/tool_ping.html` -- ping form template
- `internal/component/web/templates/component/tool_bgp_decode.html` -- BGP decode form template
- `internal/component/web/templates/component/tool_metrics.html` -- metrics query form template
- `internal/component/web/templates/component/tool_capture.html` -- capture form template
- `internal/component/web/templates/component/log_live.html` -- live log SSE toolbar and streaming area
- `internal/component/web/templates/component/log_table.html` -- warnings/errors table template
- `internal/component/web/templates/component/dashboard_health.html` -- component health table
- `internal/component/web/templates/component/dashboard_events.html` -- recent events table
