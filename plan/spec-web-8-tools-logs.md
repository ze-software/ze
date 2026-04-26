# Spec: web-8 -- Tools, Logs, and Dashboard Pages

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-web-3-foundation |
| Phase | 7/7 |
| Updated | 2026-04-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/design/web-interface-design.md` - page layouts and UI patterns for every page in this spec
4. `ai/patterns/web-endpoint.md` - web handler, template, HTMX, and route conventions
5. `docs/architecture/web-interface.md` - current web UI architecture
6. `internal/component/web/sse.go` - existing SSE infrastructure (EventBroker)
7. `internal/component/web/handler_tools.go` - existing related-tool overlay handler
8. `internal/component/web/handler_workbench.go` - workbench shell handler
9. `internal/component/web/workbench_sections.go` - left navigation sections
10. `internal/component/web/render.go` - WorkbenchData, Renderer, template parsing
11. `internal/component/cmd/show/show.go` - show verb RPC registration (warnings, errors, health, metrics-query, event-recent, event-namespaces, capture, bgp-health, ping)

## Task

Build the Tools, Logs, and Dashboard pages for the operator workbench. These are purpose-built pages with forms, streaming output, and auto-refreshing data panels. They do not render YANG tree browsers; each page has a specific layout driven by its underlying show command and the design document.

The Tools pages provide interactive diagnostic forms (Ping, BGP Decode, Metrics Query, Capture). The Logs pages provide real-time event streaming and warning/error tables. The Dashboard pages provide system health overview with panels pulling from multiple data sources.

All pages render inside the existing workbench shell (left nav, top bar, CLI bar). All data comes from existing show-command RPCs dispatched through the standard CommandDispatcher. SSE streaming uses the existing EventBroker infrastructure.

-> Decision: Each page gets its own handler function and template. No generic "tools page" that switches on a parameter.
-> Decision: Ping streaming uses chunked HTTP responses (Transfer-Encoding: chunked) with HTMX hx-swap, not SSE. SSE is reserved for the Live Log page where the stream is open-ended.
-> Constraint: All command execution goes through CommandDispatcher (same authz path as CLI/admin). No direct RPC calls from handlers.
-> Constraint: Output is ANSI-stripped, HTML-escaped, and capped per the existing relatedOverlayMaxBufBytes (4 MiB) limit.

## Required Reading

### Architecture Docs
- [ ] `plan/design/web-interface-design.md` - page-by-page UI design for every page in this spec
  -> Decision: Every page follows the table pattern, form pattern, or overlay pattern defined in the design doc
  -> Constraint: Empty states must be page-specific messages with action hints, never blank
- [ ] `ai/patterns/web-endpoint.md` - handler, template, route, and test conventions
  -> Constraint: Each handler is a standalone function returning http.HandlerFunc; templates live under templates/component/ or templates/page/
- [ ] `docs/architecture/web-interface.md` - overall web architecture and rendering pipeline
  -> Constraint: Server-rendered Go templates + HTMX. No client-side frameworks.
- [ ] `docs/architecture/web-components.md` - workbench component architecture
  -> Decision: Workbench pages render inside the WorkbenchData shell with left-nav sections
- [ ] `docs/architecture/api/commands.md` - command dispatch taxonomy and RPC wire methods
  -> Constraint: All show commands go through the standard dispatch path. Wire methods: ze-show:ping, ze-show:bgp-decode, ze-show:metrics-query, ze-show:capture, ze-show:warnings, ze-show:errors, ze-show:event-recent, ze-show:event-namespaces, ze-show:health, ze-show:bgp-health, ze-show:version, ze-show:uptime, ze-show:system-memory, ze-show:system-cpu

### RFC Summaries (MUST for protocol work)
N/A -- no protocol work in this spec.

**Key insights:**
- SSE infrastructure already exists in sse.go (EventBroker with subscribe/unsubscribe/broadcast)
- Show commands are registered as RPCs in internal/component/cmd/show/show.go via pluginserver.RegisterRPCs
- The CommandDispatcher type (func(command, username, remoteAddr string) (string, error)) is the single dispatch path
- WorkbenchData embeds LayoutData; pages render inside the workbench shell with WorkbenchSections for left nav
- The existing handler_tools.go handles related-tool overlay execution; this spec adds standalone tool pages
- Ping handler already exists at ze-show:ping with count/timeout args
- Capture handler exists at ze-show:capture with tunnel-id/peer/count args
- Metrics query exists at ze-show:metrics-query with name and label=value filter args
- Warnings, errors, health, events, and bgp-health RPCs all exist and return structured JSON

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/web/handler_tools.go` - Related-tool overlay handler for /tools/related/run POST endpoint; renders ToolOverlayData with inline/overflow output regions
  -> Constraint: This spec adds new tool page handlers alongside, not replacing, the existing related-tool overlay handler
- [ ] `internal/component/web/sse.go` - EventBroker with subscribe/unsubscribe/broadcast; sseEvent has eventType and data fields; max 100 concurrent clients; 16-event buffer per client
  -> Constraint: Live Log SSE must use the existing EventBroker pattern, subscribing a new event type for log entries
- [ ] `internal/component/web/handler_workbench.go` - HandleWorkbench serves /show/* with WorkbenchData shell; HTMX partial requests fall through to fragment OOB response
  -> Constraint: Tool/Log/Dashboard pages must integrate with the same HTMX partial navigation pattern
- [ ] `internal/component/web/workbench_sections.go` - WorkbenchSections returns ordered left-nav entries; "tools" maps to /admin/ and "logs" maps to /admin/show/log/
  -> Decision: Tool pages need routes under /tools/ (not /admin/); Log pages under /logs/; Dashboard pages under /show/ (root). Section URLs in workbench_sections.go must be updated.
- [ ] `internal/component/web/render.go` - Renderer parses embedded templates; WorkbenchData struct with LayoutData + Sections; NewRenderer registers all templates
  -> Constraint: New templates must be registered in NewRenderer's ParseFS calls
- [ ] `internal/component/web/handler_admin.go` - CommandDispatcher type and HandleAdminExecute; admin endpoints dispatch commands
  -> Constraint: Tool page handlers accept CommandDispatcher and dispatch show commands through it
- [ ] `internal/component/cmd/show/show.go` - All RPC registrations for ze-show:warnings, ze-show:errors, ze-show:health, ze-show:bgp-health, ze-show:ping, ze-show:metrics-query, ze-show:event-recent, ze-show:event-namespaces, ze-show:capture
  -> Constraint: RPC response format is plugin.Response with Status and Data fields; Data is typically map[string]any with structured keys
- [ ] `internal/component/cmd/show/ping.go` - Ping handler: parsePingArgs extracts dest (netip.Addr), count (1-100, default 5), timeout (1s-30s, default 5s); returns doPing results synchronously
  -> Constraint: Ping currently returns all results at once. Streaming requires either chunked response from the handler or a progress callback pattern.
- [ ] `internal/component/web/handler.go` - RegisterRoutes and RegisterCLIRoutes patterns; NegotiateContentType for json/html response format switching
  -> Decision: New pages will have a RegisterToolRoutes function (or similar) called from the hub startup

**Behavior to preserve:**
- Existing /tools/related/run endpoint for context-aware related tools (ToolOverlayData rendering)
- Existing SSE /events endpoint and EventBroker lifecycle
- WorkbenchSections left-nav rendering
- CommandDispatcher authz path (username + remoteAddr)
- CLI bar, commit bar, breadcrumbs, and error panel in workbench shell
- HTMX partial navigation via HX-Request header detection

**Behavior to change:**
- WorkbenchSections URLs for "tools" and "logs" entries (currently /admin/ and /admin/show/log/) to point to the new purpose-built pages
- NewRenderer template parsing to include new page templates
- Route registration to add new tool/log/dashboard page endpoints

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Browser HTTP request to tool/log/dashboard page URL (GET for page load, POST for form submission)
- SSE connection for Live Log streaming

### Transformation Path

**Tool pages (Ping, BGP Decode, Metrics Query, Capture):**
1. Browser sends GET /tools/ping (page load) or POST /tools/ping (form submission with params)
2. Handler extracts form params, validates input, builds show command string
3. Handler calls CommandDispatcher(command, username, remoteAddr) which routes to the registered RPC handler
4. RPC handler (e.g., handlePing) executes the operation and returns plugin.Response
5. Handler parses the structured response, builds template data struct
6. Renderer renders the result template (inline for HTMX partial, full page for initial load)

**Live Log page:**
1. Browser sends GET /logs/live (full page load with toolbar UI)
2. Browser opens SSE connection to /logs/live/stream
3. EventBroker subscribes the client
4. As events arrive on the bus, broker broadcasts sseEvent with log data
5. Browser-side HTMX/JS appends each event as a table row, applying namespace/level/text filters client-side
6. Pause/Resume toggles the SSE connection (disconnect/reconnect)

**Dashboard page:**
1. Browser sends GET /show/ (dashboard root)
2. Handler dispatches multiple show commands (version, uptime, system-memory, system-cpu, bgp-health, warnings, errors, event-recent)
3. Handler aggregates responses into dashboard panel data
4. Renderer renders the dashboard template with all panels
5. Auto-refresh: HTMX hx-trigger="every 5s" on counter panels, "every 30s" on health panels

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser <-> Web handler | HTTP GET/POST (form params), SSE stream | [ ] |
| Web handler <-> CommandDispatcher | func(command, username, remoteAddr string) (string, error) | [ ] |
| CommandDispatcher <-> RPC handler | pluginserver dispatch to registered handler | [ ] |
| EventBroker <-> SSE client | sseEvent channel with eventType + data | [ ] |

### Integration Points
- `CommandDispatcher` in handler_admin.go - all show commands dispatch through this function type
- `EventBroker` in sse.go - Live Log SSE streaming uses the existing broker infrastructure
- `WorkbenchSections` in workbench_sections.go - left nav highlights the active section
- `Renderer` in render.go - all templates registered and rendered through the central Renderer
- `RegisterRoutes` pattern in handler.go - new routes follow the same mux registration pattern

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| GET /tools/ping | -> | HandleToolPing | `TestToolPingPageRendersForm` |
| POST /tools/ping with dest=192.0.2.1 | -> | HandleToolPing -> CommandDispatcher("show ping 192.0.2.1") | `TestToolPingDispatchesCommand` |
| GET /tools/bgp-decode | -> | HandleToolBGPDecode | `TestToolBGPDecodePageRendersForm` |
| POST /tools/bgp-decode with hex input | -> | HandleToolBGPDecode -> CommandDispatcher("show bgp/decode <hex>") | `TestToolBGPDecodeDispatchesCommand` |
| GET /tools/metrics | -> | HandleToolMetrics | `TestToolMetricsPageRendersForm` |
| POST /tools/metrics with metric name | -> | HandleToolMetrics -> CommandDispatcher("show metrics-query <name>") | `TestToolMetricsDispatchesCommand` |
| GET /tools/capture | -> | HandleToolCapture | `TestToolCapturePageRendersForm` |
| POST /tools/capture with filters | -> | HandleToolCapture -> CommandDispatcher("show capture ...") | `TestToolCaptureDispatchesCommand` |
| GET /logs/live | -> | HandleLogLive | `TestLogLivePageRendersToolbar` |
| GET /logs/live/stream (SSE) | -> | HandleLogLiveStream -> EventBroker | `TestLogLiveSSEStreamsEvents` |
| GET /logs/warnings | -> | HandleLogWarnings -> CommandDispatcher("show warnings") | `TestLogWarningsRendersTable` |
| GET /logs/errors | -> | HandleLogErrors -> CommandDispatcher("show errors") | `TestLogErrorsRendersTable` |
| GET /dashboard/ (overview) | -> | HandleDashboardOverview -> multiple dispatches | `TestDashboardOverviewRendersPanels` |
| GET /dashboard/health | -> | HandleDashboardHealth -> CommandDispatcher("show health") | `TestDashboardHealthRendersTable` |
| GET /dashboard/events | -> | HandleDashboardEvents -> CommandDispatcher("show event/recent") | `TestDashboardEventsRendersTable` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | User visits /tools/ping page, enters destination IP "192.0.2.1", clicks Run | Ping form dispatches "show ping 192.0.2.1 count 5 timeout 5s", result area shows per-probe latency and loss summary |
| AC-2 | Ping results from a 5-count ping | Results appear progressively (chunked response), not all at once after the final probe completes |
| AC-3 | User visits /tools/bgp-decode, pastes hex bytes into textarea, clicks Decode | Handler dispatches "show bgp/decode <hex>", result area shows decoded message fields in structured HTML |
| AC-4 | User visits /tools/metrics, types in the metric name field | A searchable dropdown lists available metric names (populated from a metrics list endpoint or pre-fetched) |
| AC-5 | User selects a metric name and optional label filter, clicks Query | Handler dispatches "show metrics-query <name> [label=value]", results display as a table of matching series |
| AC-6 | User visits /tools/capture, sets tunnel-id and peer filters, clicks Run | Handler dispatches "show capture tunnel-id N peer X count N", table shows captured L2TP/BGP control messages |
| AC-7 | User opens /logs/live | Page opens SSE connection to /logs/live/stream; new log events appear as table rows in real time |
| AC-8 | User selects namespace filter, checks/unchecks level checkboxes, or types search text on Live Log | Displayed entries are filtered accordingly; namespace dropdown populated from show event/namespaces |
| AC-9 | User clicks Pause on Live Log | SSE streaming stops (connection closed); clicking Resume reconnects and resumes streaming |
| AC-10 | Live Log entries arrive with level=error | Entry row has red styling; warning=yellow, info=normal, debug=grey |
| AC-11 | User visits /logs/warnings | Table shows active warnings with columns: Time, Component, Message, Duration (time since raised) |
| AC-12 | No active warnings exist | Warnings page shows empty state: "No active warnings. All systems operating normally." |
| AC-13 | User visits /logs/errors | Table shows recent errors with columns: Time, Component, Message |
| AC-14 | User visits /dashboard/ (overview) | Page shows panels: System (hostname, uptime, version, CPU, memory), BGP Summary (peer counts by state), Active Warnings, Recent Errors, Recent Events |
| AC-15 | Dashboard overview is displayed for 10+ seconds | Counter panels auto-refresh via HTMX polling (5s for counters, 30s for health) without full page reload |
| AC-16 | Ze is freshly installed with no config | Dashboard shows "No interfaces configured" with link, "No BGP peers configured" with link, "System healthy, no warnings" |
| AC-17 | User visits /dashboard/health | Table shows one row per component (BGP, Interfaces, L2TP, DNS, ...) with Status indicator (green/yellow/red) and Summary text |
| AC-18 | User visits /dashboard/events | Table shows recent events with Time, Namespace, Message columns; namespace filter dropdown from show event/namespaces |
| AC-19 | User submits a tool form with invalid input (empty destination for ping, non-hex for BGP decode) | Form shows validation error message without dispatching a command |
| AC-20 | Tool output exceeds 128 KiB | First 128 KiB renders inline; remainder in a collapsible overflow section; hard cap at 4 MiB with truncation notice |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestToolPingPageRendersForm` | `internal/component/web/handler_tool_pages_test.go` | GET /tools/ping returns HTML with form fields (destination, count, timeout) | |
| `TestToolPingDispatchesCommand` | `internal/component/web/handler_tool_pages_test.go` | POST /tools/ping builds correct show command and dispatches through CommandDispatcher | |
| `TestToolPingValidatesInput` | `internal/component/web/handler_tool_pages_test.go` | POST /tools/ping with empty destination returns validation error, no dispatch | |
| `TestToolPingStreamingChunked` | `internal/component/web/handler_tool_pages_test.go` | POST /tools/ping returns chunked response with progressive results | |
| `TestToolBGPDecodePageRendersForm` | `internal/component/web/handler_tool_pages_test.go` | GET /tools/bgp-decode returns HTML with hex textarea | |
| `TestToolBGPDecodeDispatchesCommand` | `internal/component/web/handler_tool_pages_test.go` | POST /tools/bgp-decode dispatches "show bgp/decode <hex>" | |
| `TestToolBGPDecodeValidatesHex` | `internal/component/web/handler_tool_pages_test.go` | POST /tools/bgp-decode with non-hex input returns validation error | |
| `TestToolMetricsPageRendersForm` | `internal/component/web/handler_tool_pages_test.go` | GET /tools/metrics returns HTML with metric name input and label filter fields | |
| `TestToolMetricsDispatchesCommand` | `internal/component/web/handler_tool_pages_test.go` | POST /tools/metrics dispatches "show metrics-query <name>" with optional label filter | |
| `TestToolMetricsValidatesName` | `internal/component/web/handler_tool_pages_test.go` | POST /tools/metrics with empty name returns validation error | |
| `TestToolCapturePageRendersForm` | `internal/component/web/handler_tool_pages_test.go` | GET /tools/capture returns HTML with tunnel-id, peer, count fields | |
| `TestToolCaptureDispatchesCommand` | `internal/component/web/handler_tool_pages_test.go` | POST /tools/capture builds correct show command with optional filters | |
| `TestLogLivePageRendersToolbar` | `internal/component/web/handler_log_pages_test.go` | GET /logs/live returns HTML with namespace dropdown, level checkboxes, search, pause/resume | |
| `TestLogLiveSSEStreamsEvents` | `internal/component/web/handler_log_pages_test.go` | GET /logs/live/stream returns SSE content-type; broadcast events appear as SSE data lines | |
| `TestLogLiveSSEClientDisconnect` | `internal/component/web/handler_log_pages_test.go` | Client closing connection triggers unsubscribe from EventBroker | |
| `TestLogWarningsRendersTable` | `internal/component/web/handler_log_pages_test.go` | GET /logs/warnings dispatches "show warnings" and renders table with Time, Component, Message, Duration | |
| `TestLogWarningsEmptyState` | `internal/component/web/handler_log_pages_test.go` | GET /logs/warnings with no warnings shows "All systems operating normally" | |
| `TestLogErrorsRendersTable` | `internal/component/web/handler_log_pages_test.go` | GET /logs/errors dispatches "show errors" and renders table with Time, Component, Message | |
| `TestLogErrorsEmptyState` | `internal/component/web/handler_log_pages_test.go` | GET /logs/errors with no errors shows "No recent errors" | |
| `TestDashboardOverviewRendersPanels` | `internal/component/web/handler_dashboard_test.go` | GET /dashboard/ dispatches multiple show commands and renders system/BGP/warnings/errors/events panels | |
| `TestDashboardOverviewEmptyState` | `internal/component/web/handler_dashboard_test.go` | Dashboard with no config shows setup hints ("No interfaces configured" with link, "No BGP peers configured" with link) | |
| `TestDashboardOverviewAutoRefresh` | `internal/component/web/handler_dashboard_test.go` | Dashboard panel HTML contains HTMX hx-trigger="every 5s" for counter panels | |
| `TestDashboardHealthRendersTable` | `internal/component/web/handler_dashboard_test.go` | GET /dashboard/health dispatches "show health" and renders component status table | |
| `TestDashboardHealthStatusIndicators` | `internal/component/web/handler_dashboard_test.go` | Health table rows have green/yellow/red CSS classes matching component status | |
| `TestDashboardEventsRendersTable` | `internal/component/web/handler_dashboard_test.go` | GET /dashboard/events dispatches "show event/recent" and renders table with namespace filtering | |
| `TestDashboardEventsNamespaceFilter` | `internal/component/web/handler_dashboard_test.go` | GET /dashboard/events?namespace=bgp dispatches "show event/recent namespace bgp" | |
| `TestToolOutputTruncation` | `internal/component/web/handler_tool_pages_test.go` | Output exceeding 4 MiB is truncated with notice; 128 KiB inline, rest in overflow | |
| `TestWorkbenchSectionsToolsURL` | `internal/component/web/workbench_sections_test.go` | Tools section URL points to /tools/ (not /admin/) | |
| `TestWorkbenchSectionsLogsURL` | `internal/component/web/workbench_sections_test.go` | Logs section URL points to /logs/ (not /admin/show/log/) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Ping count | 1-100 | 100 | 0 | 101 |
| Ping timeout | 1s-30s | 30s | 0s | 31s |
| Capture count | 1-10000 | 10000 | 0 | 10001 |
| Capture tunnel-id | 0-65535 | 65535 | N/A (0 means no filter) | 65536 |
| Hex input length (BGP decode) | 1-65535 bytes | 65535 hex chars | 0 (empty) | N/A (server truncates) |
| Metric name length | 1-256 | 256 chars | 0 (empty) | 257 chars |
| Dashboard refresh interval | 5s/30s | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-web-tools-ping` | `test/plugin/web-tools-ping.ci` | User opens ping page, submits destination, receives latency results | |
| `test-web-tools-decode` | `test/plugin/web-tools-decode.ci` | User opens BGP decode page, pastes hex, sees decoded output | |
| `test-web-tools-metrics` | `test/plugin/web-tools-metrics.ci` | User opens metrics page, queries a metric, sees table results | |
| `test-web-tools-capture` | `test/plugin/web-tools-capture.ci` | User opens capture page, queries with filters, sees captured messages | |
| `test-web-logs-warnings` | `test/plugin/web-logs-warnings.ci` | User opens warnings page, sees active warnings or empty state | |
| `test-web-logs-errors` | `test/plugin/web-logs-errors.ci` | User opens errors page, sees recent errors or empty state | |
| `test-web-dashboard-overview` | `test/plugin/web-dashboard-overview.ci` | User opens dashboard, sees system/BGP/warnings panels with data | |
| `test-web-dashboard-health` | `test/plugin/web-dashboard-health.ci` | User opens health page, sees component status table | |

### Future (if deferring any tests)
- Live Log SSE functional test (requires a running event bus and real-time verification, difficult in .ci framework): deferred to browser-based e2e test with agent-browser
- Ping streaming functional test (requires observing chunked transfer encoding): deferred to browser-based e2e test

## Files to Modify
- `internal/component/web/handler_tool_pages.go` - New handler functions for Ping, BGP Decode, Metrics Query, Capture tool pages
- `internal/component/web/handler_log_pages.go` - New handler functions for Live Log, Warnings, Errors pages
- `internal/component/web/handler_dashboard.go` - New handler functions for Dashboard Overview, Health, Recent Events pages
- `internal/component/web/workbench_sections.go` - Update section URLs for tools and logs; add dashboard sub-sections
- `internal/component/web/render.go` - Register new templates in NewRenderer
- `internal/component/web/handler.go` - Add RegisterToolRoutes, RegisterLogRoutes, RegisterDashboardRoutes functions
- `internal/component/web/sse.go` - Add log-event SSE event type and broadcast integration for Live Log streaming

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A -- uses existing show RPCs |
| CLI commands/flags | No | N/A -- web-only pages |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/plugin/web-tools-*.ci`, `test/plugin/web-logs-*.ci`, `test/plugin/web-dashboard-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add Tools/Logs/Dashboard web pages |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A -- uses existing RPCs |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/web-interface.md` - add Tools, Logs, Dashboard section |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A -- extends existing web architecture |

## Files to Create
- `internal/component/web/handler_tool_pages.go` - Ping, BGP Decode, Metrics Query, Capture page handlers
- `internal/component/web/handler_tool_pages_test.go` - Unit tests for tool page handlers
- `internal/component/web/handler_log_pages.go` - Live Log, Warnings, Errors page handlers
- `internal/component/web/handler_log_pages_test.go` - Unit tests for log page handlers
- `internal/component/web/handler_dashboard.go` - Dashboard Overview, Health, Events page handlers
- `internal/component/web/handler_dashboard_test.go` - Unit tests for dashboard handlers
- `internal/component/web/templates/component/tool_ping.html` - Ping form and result area template
- `internal/component/web/templates/component/tool_bgp_decode.html` - BGP Decode form and result template
- `internal/component/web/templates/component/tool_metrics.html` - Metrics Query form and result template
- `internal/component/web/templates/component/tool_capture.html` - Capture form and result template
- `internal/component/web/templates/component/log_live.html` - Live Log toolbar and streaming area template
- `internal/component/web/templates/component/log_table.html` - Warnings/Errors table template (shared)
- `internal/component/web/templates/component/dashboard_overview.html` - Dashboard multi-panel template
- `internal/component/web/templates/component/dashboard_health.html` - Component health table template
- `internal/component/web/templates/component/dashboard_events.html` - Events table with namespace filter template
- `test/plugin/web-tools-ping.ci` - Functional test: ping tool page
- `test/plugin/web-tools-decode.ci` - Functional test: BGP decode tool page
- `test/plugin/web-tools-metrics.ci` - Functional test: metrics query tool page
- `test/plugin/web-tools-capture.ci` - Functional test: capture tool page
- `test/plugin/web-logs-warnings.ci` - Functional test: warnings page
- `test/plugin/web-logs-errors.ci` - Functional test: errors page
- `test/plugin/web-dashboard-overview.ci` - Functional test: dashboard overview
- `test/plugin/web-dashboard-health.ci` - Functional test: dashboard health

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. /ze-review gate | Review Gate section -- run `/ze-review`; fix every BLOCKER/ISSUE; re-run until only NOTEs remain (BEFORE full verification) |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Tool page handlers and templates** -- Implement Ping, BGP Decode, Metrics Query, and Capture tool pages
   - Tests: `TestToolPingPageRendersForm`, `TestToolPingDispatchesCommand`, `TestToolPingValidatesInput`, `TestToolPingStreamingChunked`, `TestToolBGPDecodePageRendersForm`, `TestToolBGPDecodeDispatchesCommand`, `TestToolBGPDecodeValidatesHex`, `TestToolMetricsPageRendersForm`, `TestToolMetricsDispatchesCommand`, `TestToolMetricsValidatesName`, `TestToolCapturePageRendersForm`, `TestToolCaptureDispatchesCommand`, `TestToolOutputTruncation`
   - Files: `handler_tool_pages.go`, `handler_tool_pages_test.go`, `templates/component/tool_ping.html`, `templates/component/tool_bgp_decode.html`, `templates/component/tool_metrics.html`, `templates/component/tool_capture.html`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Live Log with SSE streaming** -- Implement Live Log page with SSE event streaming, namespace/level filtering, pause/resume
   - Tests: `TestLogLivePageRendersToolbar`, `TestLogLiveSSEStreamsEvents`, `TestLogLiveSSEClientDisconnect`
   - Files: `handler_log_pages.go`, `handler_log_pages_test.go`, `templates/component/log_live.html`, `sse.go` (add log event type)
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Warnings and Errors pages** -- Implement Warnings and Errors read-only table pages with empty states
   - Tests: `TestLogWarningsRendersTable`, `TestLogWarningsEmptyState`, `TestLogErrorsRendersTable`, `TestLogErrorsEmptyState`
   - Files: `handler_log_pages.go`, `handler_log_pages_test.go`, `templates/component/log_table.html`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Dashboard Overview with real data** -- Implement multi-panel dashboard with system, BGP, warnings, errors, events panels
   - Tests: `TestDashboardOverviewRendersPanels`, `TestDashboardOverviewEmptyState`, `TestDashboardOverviewAutoRefresh`
   - Files: `handler_dashboard.go`, `handler_dashboard_test.go`, `templates/component/dashboard_overview.html`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Dashboard Health aggregation** -- Implement component health table with status indicators
   - Tests: `TestDashboardHealthRendersTable`, `TestDashboardHealthStatusIndicators`
   - Files: `handler_dashboard.go`, `handler_dashboard_test.go`, `templates/component/dashboard_health.html`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Dashboard Events with namespace filtering** -- Implement recent events page with namespace filter dropdown
   - Tests: `TestDashboardEventsRendersTable`, `TestDashboardEventsNamespaceFilter`
   - Files: `handler_dashboard.go`, `handler_dashboard_test.go`, `templates/component/dashboard_events.html`
   - Verify: tests fail -> implement -> tests pass

7. **Phase: Route registration and workbench integration** -- Wire all new handlers into route registration, update workbench section URLs, register templates in Renderer
   - Tests: `TestWorkbenchSectionsToolsURL`, `TestWorkbenchSectionsLogsURL`
   - Files: `handler.go`, `workbench_sections.go`, `render.go`
   - Verify: tests fail -> implement -> tests pass

8. **Functional tests** -> Create after feature works. Cover user-visible behavior.
   - Files: `test/plugin/web-tools-ping.ci`, `test/plugin/web-tools-decode.ci`, `test/plugin/web-tools-metrics.ci`, `test/plugin/web-tools-capture.ci`, `test/plugin/web-logs-warnings.ci`, `test/plugin/web-logs-errors.ci`, `test/plugin/web-dashboard-overview.ci`, `test/plugin/web-dashboard-health.ci`

9. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)

10. **Complete spec** -> Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-20 has implementation with file:line |
| Correctness | Show command strings exactly match registered RPC wire methods; form field names match handler extraction; SSE event types match subscriber filters |
| Naming | Route paths use kebab-case (/tools/bgp-decode not /tools/bgpDecode); template names use snake_case; handler funcs use PascalCase (HandleToolPing) |
| Data flow | All show commands go through CommandDispatcher, never direct RPC calls; SSE uses EventBroker, never direct goroutine writes |
| Rule: no-layering | No duplicate dispatch paths; no second SSE broker |
| Rule: html-escape | All command output is HTML-escaped before template rendering; no raw user input in HTML |
| Rule: empty-states | Every page has a specific empty-state message per the design doc, not a blank page |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Ping tool page renders form and results | `go test -run TestToolPingPageRendersForm ./internal/component/web/` |
| BGP Decode page renders form and decoded output | `go test -run TestToolBGPDecodeDispatchesCommand ./internal/component/web/` |
| Metrics Query page renders form and table | `go test -run TestToolMetricsDispatchesCommand ./internal/component/web/` |
| Capture page renders form and results | `go test -run TestToolCaptureDispatchesCommand ./internal/component/web/` |
| Live Log page streams events via SSE | `go test -run TestLogLiveSSEStreamsEvents ./internal/component/web/` |
| Live Log toolbar has namespace/level/search/pause controls | `go test -run TestLogLivePageRendersToolbar ./internal/component/web/` |
| Warnings page renders table or empty state | `go test -run TestLogWarningsRendersTable ./internal/component/web/` and `TestLogWarningsEmptyState` |
| Errors page renders table or empty state | `go test -run TestLogErrorsRendersTable ./internal/component/web/` and `TestLogErrorsEmptyState` |
| Dashboard overview renders all panels | `go test -run TestDashboardOverviewRendersPanels ./internal/component/web/` |
| Dashboard empty state shows setup hints | `go test -run TestDashboardOverviewEmptyState ./internal/component/web/` |
| Dashboard auto-refresh attributes present | `go test -run TestDashboardOverviewAutoRefresh ./internal/component/web/` |
| Health page renders component table | `go test -run TestDashboardHealthRendersTable ./internal/component/web/` |
| Events page renders with namespace filter | `go test -run TestDashboardEventsRendersTable ./internal/component/web/` |
| Tool page templates exist | `ls internal/component/web/templates/component/tool_*.html` |
| Log page templates exist | `ls internal/component/web/templates/component/log_*.html` |
| Dashboard templates exist | `ls internal/component/web/templates/component/dashboard_*.html` |
| Functional test files exist | `ls test/plugin/web-tools-*.ci test/plugin/web-logs-*.ci test/plugin/web-dashboard-*.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Ping destination must be valid IP (netip.Addr parse); hex input must contain only hex chars and whitespace; metric name must be alphanumeric/underscore/colon; tunnel-id must be uint16; count must be positive integer in range |
| Command injection | Form params must never be interpolated into command strings without validation; use structured command building, not string concatenation with raw input |
| XSS prevention | All command output HTML-escaped before rendering; no template.HTML from user-controlled data; hex input textarea content escaped on re-render |
| SSE resource exhaustion | Live Log SSE respects EventBroker max client limit (100); client disconnect triggers cleanup; no unbounded goroutine leak |
| Auth bypass | All tool/log/dashboard routes wrapped in auth middleware; unauthenticated requests redirected to /login |
| Output size | Tool output capped at 4 MiB (relatedOverlayMaxBufBytes); no unbounded buffering of command output in memory |
| CSRF | POST endpoints for tool execution protected by same-origin checks (HTMX sends HX-Request header; handler may verify Origin header) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem -> arch doc, process -> rules, knowledge -> memory.md -->

## RFC Documentation

N/A -- no protocol work in this spec.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

<!-- BLOCKING (rules/planning.md Completion Checklist step 7): -->
<!-- Run /ze-review BEFORE the final testing/verify step. Record the findings here. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns only NOTEs (or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block -- record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->
<!-- Hook pre-commit-spec-audit.sh (exit 2) checks this section exists and is filled. -->

### Files Exist (ls)
<!-- For EVERY file in "Files to Create": ls -la <path> -- paste output. -->
<!-- For EVERY .ci file in Wiring Test and Functional Tests: ls -la <path> -- paste output. -->
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
<!-- For EVERY AC-N: independently verify. Do NOT copy from audit -- re-check. -->
<!-- Acceptable evidence: test name + pass output, grep showing function call, ls showing file. -->
<!-- NOT acceptable: "already checked", "should work", reference to audit table above. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
<!-- For EVERY wiring test row: does the .ci test exist AND does it exercise the full path? -->
<!-- Read the .ci file content. Does it actually test what the wiring table claims? -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-20 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
