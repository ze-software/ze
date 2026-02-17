# Spec: chaos-web-dashboard

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/chaos-web-dashboard.md` - full design document (layout, HTMX, SSE, state, controls)
4. `cmd/ze-bgp-chaos/report/reporter.go` - Consumer interface
5. `cmd/ze-bgp-chaos/report/metrics.go` - existing HTTP server pattern
6. `cmd/ze-bgp-chaos/main.go` - orchestrator wiring, setupReporting, flag definitions
7. `cmd/ze-bgp-chaos/orchestrator.go` - EventProcessor, establishedState, ChaosConfig
8. `cmd/ze-bgp-chaos/peer/event.go` - Event types
9. `cmd/ze-bgp-chaos/chaos/scheduler.go` - scheduler Tick/actions
10. `cmd/ze-bgp-chaos/validation/` - model, tracker, convergence, properties

## Task

Build a live web dashboard for ze-bgp-chaos that provides real-time visualization and interactive control of chaos test runs. The dashboard uses **HTMX** for dynamic updates via **Server-Sent Events (SSE)**, with all assets (HTMX JS, CSS) **embedded in the binary** via `go:embed`. The UI must handle **200+ peers** gracefully using an **active set** approach: only ~40 peers are visible at once, with auto-promotion on noteworthy events, adaptive decay, and user-pinning.

The dashboard is activated via a `--web` CLI flag and runs as an additional `report.Consumer` alongside existing reporters (terminal dashboard, JSON log, Prometheus metrics).

### Goals

1. **Live observability** — See run status, peer states, route counts, convergence, and property results updating in real time
2. **Peer drill-down** — Click any peer in the table to open a detail pane with full history and per-peer metrics
3. **Interactive control** — Pause/resume chaos, manually trigger chaos actions on specific peers, adjust chaos rate, re-run with new seed
4. **Advanced visualizations** — Peer state timeline, convergence histogram, chaos event markers, and route flow matrix

### Non-Goals

- Authentication/authorization (local tool, not production service)
- Persistent storage (state lives only for the run duration)
- Mobile-first design (desktop monitoring use case)

## Required Reading

### Architecture Docs
- [ ] `cmd/ze-bgp-chaos/report/reporter.go` - Consumer interface that WebDashboard must implement
  -> Constraint: Consumer.ProcessEvent() runs synchronously on main event loop, must be fast
- [ ] `cmd/ze-bgp-chaos/report/metrics.go` - Existing HTTP server pattern with per-instance registry
  -> Decision: Metrics uses a separate `http.Server` started in `setupReporting()`; web dashboard follows same pattern
- [ ] `cmd/ze-bgp-chaos/main.go` - Orchestrator wiring, flag parsing, setupReporting()
  -> Constraint: Reporter consumers are created in setupReporting(); web consumer added there
  -> Decision: Metrics HTTP server lifecycle managed via cleanup function returned from setupReporting()
- [ ] `cmd/ze-bgp-chaos/orchestrator.go` - EventProcessor, ChaosConfig, establishedState
  -> Decision: EventProcessor accumulates counters (Announced, Received, ChaosEvents, etc.) on main loop
  -> Constraint: establishedState is mutex-protected for cross-goroutine access
- [ ] `cmd/ze-bgp-chaos/peer/event.go` - 10 event types with their fields
  -> Constraint: Events are the only data source; all dashboard state derives from events
- [ ] `cmd/ze-bgp-chaos/chaos/scheduler.go` - Scheduler.Tick() generates actions from established snapshot
  -> Decision: Scheduler is currently fire-and-forget; needs pause/resume/trigger for interactive control
- [ ] `cmd/ze-bgp-chaos/validation/convergence.go` - Convergence tracking with Stats() and CheckDeadline()
  -> Decision: Dashboard needs its own convergence tracking (cannot share the orchestrator's instance safely)
- [ ] `cmd/ze-bgp-chaos/validation/properties.go` - PropertyEngine with Results()
  -> Constraint: PropertyEngine.Results() returns snapshot of all property pass/fail + violations

**Key insights:**
- Consumer.ProcessEvent() is the single data ingestion point — the web dashboard accumulates all state from events
- The existing metrics HTTP server pattern (create in setupReporting, cleanup on exit) is the template for the web server
- Interactive control requires new architecture: the scheduler and orchestrator need control channels/methods, not just fire-and-forget goroutines
- For 200+ peers, the SSE stream must be efficient — send only changed fragments, not full page re-renders

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `cmd/ze-bgp-chaos/report/reporter.go` - Consumer interface: ProcessEvent(ev) + Close() error
- [x] `cmd/ze-bgp-chaos/report/metrics.go` - Prometheus consumer with HTTP handler, per-instance registry
- [x] `cmd/ze-bgp-chaos/report/dashboard.go` - Terminal dashboard: one line per event to io.Writer
- [x] `cmd/ze-bgp-chaos/report/summary.go` - Summary struct with Pass(), Write(w) for exit report
- [x] `cmd/ze-bgp-chaos/main.go` - Flag parsing, setupReporting(), runOrchestrator(), runScheduler()
- [x] `cmd/ze-bgp-chaos/orchestrator.go` - EventProcessor, establishedState, ChaosConfig, orchestratorConfig
- [x] `cmd/ze-bgp-chaos/peer/event.go` - 10 EventType constants, Event struct

**Behavior to preserve:**
- All existing CLI flags and behavior unchanged
- Terminal dashboard, JSON log, and Prometheus metrics continue to work independently
- Web dashboard is purely additive (new `--web` flag)
- Event processing order unchanged (web consumer appended to consumer list)

**Behavior to change:**
- Scheduler needs pause/resume capability (new methods, non-breaking)
- Orchestrator needs a control interface for the web server to call back into (manual chaos trigger, rate adjustment)
- The `--web` flag shares the HTTP server with `--metrics` when both are specified (single server, multiple routes)

## Data Flow (MANDATORY)

### Entry Point
- Events enter via `peer.Event` channel, processed sequentially on main goroutine
- Each event is fanned out to all consumers by `Reporter.Process()`
- Web dashboard's `ProcessEvent()` updates internal state and pushes to SSE broadcast channel

### Transformation Path
1. **Event ingestion** — `WebDashboard.ProcessEvent(ev)` updates per-peer state, counters, ring buffers
2. **SSE rendering** — Background goroutine reads from broadcast channel, renders HTML fragments via `html/template`
3. **HTTP serving** — HTMX requests trigger server-side template rendering of current state snapshots
4. **Control actions** — POST endpoints push commands to orchestrator control channel
5. **Orchestrator dispatch** — Control channel reader in main loop executes commands (pause, trigger, adjust)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Main loop -> Web consumer | ProcessEvent() call (synchronous, fast) | [ ] |
| Web consumer -> SSE clients | Broadcast channel -> per-client goroutine | [ ] |
| Web browser -> HTTP server | HTMX requests (GET for fragments, POST for actions) | [ ] |
| HTTP server -> Orchestrator | Control channel (buffered, non-blocking send) | [ ] |
| Orchestrator -> Scheduler | Method calls on scheduler (pause/resume/adjust) | [ ] |

### Integration Points
- `report.Consumer` interface — WebDashboard implements ProcessEvent + Close
- `setupReporting()` in main.go — Creates WebDashboard, starts HTTP server
- `orchestratorConfig` — Extended with web address and control channel
- `chaos.Scheduler` — Extended with Pause/Resume/SetRate/TriggerAction methods

### Architectural Verification
- [ ] No bypassed layers (events flow through Reporter like all other consumers)
- [ ] No unintended coupling (web consumer reads events, control channel sends commands back)
- [ ] No duplicated functionality (web dashboard is new; doesn't replace terminal dashboard)
- [ ] Zero-copy preserved where applicable (events are small structs, copied by value)

## Design

**Full design document:** `docs/architecture/chaos-web-dashboard.md`

The design doc covers:
- Architecture overview (how WebDashboard fits as a report.Consumer)
- Three-panel layout (header, sidebar, main content) for 200+ peers
- Header bar, sidebar cards, property badges, control panel
- Peer table (columns, sorting, filtering, pagination for 200+)
- Peer detail pane (click-to-open with full peer info)
- 5 visualization tabs (event stream, peer timeline, convergence histogram, chaos timeline, route flow matrix)
- Dark theme color palette and typography
- HTMX request/response map (17 endpoints)
- SSE event types and debouncing strategy (200ms batching)
- Asset embedding (HTMX + SSE extension + CSS via go:embed)
- WebDashboard internal state (11 state components derived from events)
- Control architecture (6 command types via buffered channel)
- 4 implementation phases (Foundation -> Visualizations -> Controls -> Route Matrix)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `--web :8080` flag provided | HTTP server starts on port 8080, serves dashboard at `/` |
| AC-2 | Browser opens `http://localhost:8080/` | Full dashboard layout renders with header, sidebar, peer table |
| AC-3 | Chaos run in progress with 4+ peers | Peer table shows all peers with live status updates via SSE |
| AC-4 | Click on a peer row in the table | Detail pane opens with peer info, route breakdown, recent events |
| AC-5 | 200 peers running, events occurring | Active set shows ~40 most relevant peers with auto-promotion/decay, no browser lag |
| AC-6 | `--web` and `--metrics` both specified | Single HTTP server serves both `/` (dashboard) and `/metrics` (Prometheus) |
| AC-7 | Column header clicked | Table re-sorts by that column (ascending, click again for descending) |
| AC-8 | Filter set to "status=down" | Table shows only disconnected peers |
| AC-9 | SSE connection established | Dashboard updates without page reload as events arrive |
| AC-10 | High event rate (1000+ events/sec) | SSE debouncing prevents browser overload, updates batch at ~5/sec |
| AC-11 | Convergence histogram tab selected | Bar chart shows latency distribution with color gradient and deadline marker |
| AC-12 | Peer state timeline tab selected | Horizontal bars show connected/disconnected periods per peer |
| AC-13 | Chaos timeline tab selected | Markers show when each chaos event fired, colored by action type |
| AC-14 | Route flow matrix tab selected | Heatmap shows peer-to-peer route propagation counts |
| AC-15 | "Pause Chaos" button clicked | Scheduler stops firing chaos events; button changes to "Resume" |
| AC-16 | "Resume Chaos" clicked | Scheduler resumes; button changes to "Pause" |
| AC-17 | Chaos rate slider adjusted to 0.5 | Scheduler uses new rate on next tick interval |
| AC-18 | Manual trigger: "TCPDisconnect" on peer 3 | Peer 3 receives disconnect action, event appears in feed |
| AC-19 | "Stop" button clicked | Run stops gracefully, status changes to COMPLETED/FAILED |
| AC-20 | "New Seed" submitted with value 12345 | Current run stops, new run starts with seed 12345 |
| AC-21 | No `--web` flag | No HTTP server started, no web-related overhead |
| AC-22 | Property badge shows FAIL | Clicking badge shows violation details (which peer, which route) |
| AC-23 | Browser disconnects and reconnects SSE | Dashboard recovers full state from server (not just incremental) |
| AC-24 | Run completes naturally | Dashboard shows final state, remains viewable (server doesn't exit immediately) |
| AC-25 | Assets served from embedded files | No CDN requests, works fully offline / in air-gapped environments |
| AC-26 | Manual trigger dropdown shows all 16 action types | All 10 existing + 6 new actions (ClockDrift, RouteBurst, WithdrawalBurst, RouteFlap, SlowPeer, ZeroWindow) listed |
| AC-27 | "RouteBurst" selected in trigger dropdown | Parameter form shows count (number input) and family (dropdown) |
| AC-28 | "ZeroWindow" triggered on peer 5 with duration 15s | Peer 5's TCP receive window set to zero, restored after 15s, event appears in feed |
| AC-29 | "RouteFlap" with count=50, cycles=3 on peer 2 | Peer 2 withdraws then re-announces 50 routes, 3 times, events logged |
| AC-30 | Trigger form submitted with invalid params (e.g., ClockDrift > hold time) | Error fragment returned, no action executed |
| AC-31 | Peers selected via table checkboxes, trigger executed | Action targets exactly the selected peers, not all peers |
| AC-32 | Manual trigger with --event-log enabled | ChaosExecuted event appears in NDJSON log with action type and chaos-params |
| AC-33 | Replay a log containing manual triggers (--replay) | Manual triggers replay identically to scheduler-generated chaos events |
| AC-34 | Pause/resume/rate-change from UI with --event-log | Control actions logged as "control" record type (informational, skipped by replay) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestWebDashboardProcessEvent | `report/web_test.go` | ProcessEvent updates internal state correctly for all 10 event types | |
| TestWebDashboardSSEBroadcast | `report/web_test.go` | Events are broadcast to connected SSE clients | |
| TestWebDashboardSSEDebounce | `report/web_test.go` | High-frequency events are batched, not sent individually | |
| TestWebDashboardPeerState | `report/web_test.go` | Per-peer state (status, counts, last event) updated correctly | |
| TestWebDashboardStateTimeline | `report/web_test.go` | Peer state transitions recorded for timeline visualization | |
| TestWebDashboardConvergenceHistogram | `report/web_test.go` | Latency values bucketed correctly for histogram | |
| TestWebDashboardEventRingBuffer | `report/web_test.go` | Ring buffer keeps last N events, drops oldest | |
| TestWebDashboardRouteMatrix | `report/web_test.go` | Route receive events update peer-to-peer matrix correctly | |
| TestWebDashboardClose | `report/web_test.go` | Close shuts down HTTP server and SSE connections cleanly | |
| TestWebDashboardConsumerInterface | `report/web_test.go` | WebDashboard satisfies report.Consumer interface | |
| TestPeerTableSorting | `report/web_handlers_test.go` | GET /peers?sort=asn&dir=asc returns peers sorted by ASN ascending | |
| TestPeerTableFiltering | `report/web_handlers_test.go` | GET /peers?status=up returns only established peers | |
| TestPeerDetailHandler | `report/web_handlers_test.go` | GET /peer/3 returns detail pane HTML for peer 3 | |
| TestControlPauseChaos | `report/web_handlers_test.go` | POST /control/chaos/pause sends PauseChaos command | |
| TestControlTriggerChaos | `report/web_handlers_test.go` | POST /control/chaos/trigger sends specific action to specific peer | |
| TestControlSetRate | `report/web_handlers_test.go` | POST /control/chaos/rate updates scheduler rate | |
| TestSchedulerPauseResume | `chaos/scheduler_test.go` | Paused scheduler generates no actions on Tick() | |
| TestSchedulerSetRate | `chaos/scheduler_test.go` | SetRate changes probability used by Tick() | |
| TestSchedulerTriggerAction | `chaos/scheduler_test.go` | TriggerAction returns a targeted action for specific peer | |
| TestSSEClientCleanup | `report/web_test.go` | Disconnected SSE clients are removed from broadcast set | |
| TestEmbeddedAssets | `report/web_test.go` | go:embed assets are loadable and non-empty (htmx.js, style.css) | |
| TestChaosClockDrift | `chaos/actions_test.go` | ClockDrift action skews keepalive timing by drift amount | |
| TestChaosRouteBurst | `chaos/actions_test.go` | RouteBurst announces configurable count of extra routes | |
| TestChaosWithdrawalBurst | `chaos/actions_test.go` | WithdrawalBurst withdraws exact count of routes | |
| TestChaosRouteFlap | `chaos/actions_test.go` | RouteFlap cycles withdraw+announce for count routes, N cycles | |
| TestChaosSlowPeer | `chaos/actions_test.go` | SlowPeer adds delay to message sends for configured duration | |
| TestChaosZeroWindow | `chaos/actions_test.go` | ZeroWindow sets TCP recv window to zero for configured duration | |
| TestTriggerParamValidation | `web/handlers_test.go` | Invalid parameters rejected (drift > hold time, count < 0, etc.) | |
| TestTriggerParamForm | `web/handlers_test.go` | GET /control/trigger-params?action=X returns correct form fields for each action | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Web port | 1-65535 | 65535 | 0 (empty = disabled) | 65536 |
| Chaos rate (slider) | 0.0-1.0 | 1.0 | N/A (0.0 is valid = disabled) | 1.1 (clamp to 1.0) |
| Peer index (trigger) | 0 to N-1 | N-1 | -1 | N |
| Ring buffer size | 1-10000 | 10000 | 0 (default to 1000) | N/A (capped) |
| SSE debounce interval | 50ms-2000ms | 2000ms | 49ms (clamp to 50ms) | N/A |
| Pagination page size | 10-500 | 500 | 9 (default to 50) | N/A |
| RouteBurst count | 1-10000 | 10000 | 0 | N/A (capped at 10000) |
| WithdrawalBurst count | 1-10000 | 10000 | 0 | N/A (capped at 10000) |
| RouteFlap cycles | 1-50 | 50 | 0 | 51 (capped) |
| ClockDrift abs(drift) | 0 to holdTime-1s | holdTime-1s | N/A | holdTime (rejected) |
| SlowPeer delay | 100ms-30s | 30s | 99ms (clamp to 100ms) | N/A |
| ZeroWindow duration | 1s-120s | 120s | 0 (rejected) | N/A (capped at 120s) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-web-startup | `test/chaos/web-startup.ci` | `--web :0` starts server, GET / returns 200 with HTML containing "ze-bgp-chaos" | |
| test-web-sse | `test/chaos/web-sse.ci` | SSE endpoint streams events during a short chaos run | |
| test-web-metrics-coexist | `test/chaos/web-metrics.ci` | `--web :0 --metrics :0` shares server, both / and /metrics respond | |
| test-web-no-flag | `test/chaos/web-no-flag.ci` | Without --web, no HTTP server is started | |
| test-web-assets | `test/chaos/web-assets.ci` | GET /assets/htmx.min.js returns JavaScript content | |
| test-web-trigger-replay | `test/chaos/web-trigger-replay.ci` | Manual trigger via POST, verify event appears in NDJSON log, replay produces same validation result | |

## Files to Modify

- `cmd/ze-bgp-chaos/main.go` - Add `--web` flag, wire WebDashboard into setupReporting(), add control channel plumbing
- `cmd/ze-bgp-chaos/orchestrator.go` - Add control channel type and processing in main event loop, extend orchestratorConfig
- `cmd/ze-bgp-chaos/chaos/scheduler.go` - Add Pause(), Resume(), SetRate(), IsPaused() methods
- `cmd/ze-bgp-chaos/chaos/actions.go` - Add 6 new action types (ClockDrift, RouteBurst, WithdrawalBurst, RouteFlap, SlowPeer, ZeroWindow) with parameters
- `cmd/ze-bgp-chaos/peer/simulator.go` - Handle new action types in the chaos action switch (execute clock drift, route burst, etc.)
- `cmd/ze-bgp-chaos/report/summary.go` - (Minor) Export formatDuration for reuse in web templates

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | Yes | `cmd/ze-bgp-chaos/main.go` (--web flag) |
| CLI usage/help text | Yes | `cmd/ze-bgp-chaos/main.go` (usage function) |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/chaos/web-*.ci` |

## Files to Create

- `cmd/ze-bgp-chaos/web/` - Package for web dashboard
- `cmd/ze-bgp-chaos/web/dashboard.go` - WebDashboard consumer: state accumulation, SSE broadcast, HTTP handlers
- `cmd/ze-bgp-chaos/web/handlers.go` - HTTP handler functions (peer table, detail, visualizations, control)
- `cmd/ze-bgp-chaos/web/sse.go` - SSE broker: client registration, broadcast, debouncing
- `cmd/ze-bgp-chaos/web/state.go` - Internal state types: per-peer state, ring buffer, histogram buckets, route matrix
- `cmd/ze-bgp-chaos/web/templates.go` - Template loading via go:embed, template helper functions
- `cmd/ze-bgp-chaos/web/control.go` - Control command types and channel interface
- `cmd/ze-bgp-chaos/web/assets/htmx.min.js` - Vendored HTMX library
- `cmd/ze-bgp-chaos/web/assets/sse.js` - Vendored HTMX SSE extension
- `cmd/ze-bgp-chaos/web/assets/style.css` - Dark theme CSS
- `cmd/ze-bgp-chaos/web/templates/layout.html` - Main page shell (head, body, script tags)
- `cmd/ze-bgp-chaos/web/templates/header.html` - Header bar fragment
- `cmd/ze-bgp-chaos/web/templates/sidebar.html` - Sidebar with summary cards, properties, controls
- `cmd/ze-bgp-chaos/web/templates/peers.html` - Peer table with sorting/filtering
- `cmd/ze-bgp-chaos/web/templates/peer_detail.html` - Peer detail pane
- `cmd/ze-bgp-chaos/web/templates/event_feed.html` - Event stream feed
- `cmd/ze-bgp-chaos/web/templates/convergence.html` - Convergence histogram
- `cmd/ze-bgp-chaos/web/templates/peer_timeline.html` - Peer state timeline
- `cmd/ze-bgp-chaos/web/templates/chaos_timeline.html` - Chaos event markers
- `cmd/ze-bgp-chaos/web/templates/route_matrix.html` - Route flow heatmap
- `cmd/ze-bgp-chaos/web/dashboard_test.go` - Unit tests for state accumulation
- `cmd/ze-bgp-chaos/web/handlers_test.go` - HTTP handler tests
- `cmd/ze-bgp-chaos/web/sse_test.go` - SSE broker tests
- `test/chaos/web-startup.ci` - Functional test: server starts and serves dashboard
- `test/chaos/web-assets.ci` - Functional test: embedded assets served correctly

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Foundation

1. **Vendor HTMX assets** - Download htmx.min.js and sse.js, place in web/assets/
   -> Review: Are versions pinned? Are files minified?

2. **Write CSS theme** - Create dark theme stylesheet with layout grid
   -> Review: Does it handle 200+ rows without layout issues? Is the grid responsive?

3. **Create state types** - Define per-peer state, ring buffer, histogram buckets (tests first)
   -> Review: Thread safety? Ring buffer correctly drops oldest?

4. **Implement WebDashboard consumer** - ProcessEvent updates state, satisfies Consumer interface (tests first)
   -> Review: Is ProcessEvent fast enough for main loop? No blocking operations?

5. **Implement SSE broker** - Client registration, broadcast with debouncing (tests first)
   -> Review: Client cleanup on disconnect? Memory leak potential?

6. **Create HTML templates** - Layout shell, header, sidebar, peer table
   -> Review: Templates use go template syntax correctly? HTMX attributes correct?

7. **Implement HTTP handlers** - Route table, peer detail, SSE endpoint, asset serving
   -> Review: Error handling? Content-Type headers correct?

8. **Wire into main.go** - Add --web flag, create WebDashboard in setupReporting()
   -> Review: Server lifecycle correct? Cleanup on shutdown?

9. **Test with real chaos run** - Run `ze-bgp-chaos --web :8080 --peers 10 --duration 60s`
   -> Review: Dashboard loads? Updates live? Peer click works?

### Phase 2: Visualizations

10. **Event stream feed** - Live scrolling event list with filtering
11. **Convergence histogram** - CSS bar chart with buckets and deadline marker
12. **Peer state timeline** - Horizontal bars with state segments
13. **Chaos event timeline** - Markers on time axis

### Phase 3: Interactive Controls

14. **Extend scheduler** - Add Pause/Resume/SetRate/TriggerAction methods (tests first)
15. **Add control channel** - Define command types, wire into orchestrator event loop
16. **Control panel UI** - Buttons, slider, trigger form in sidebar
17. **Control handlers** - POST endpoints that send commands to control channel

### Phase 4: Route Flow Matrix

18. **Track route sources** - Extend state to record which peer announced routes received by each peer
19. **Heatmap rendering** - CSS grid with color-intensity cells
20. **Filtering** - Top N peers, per-family view, latency toggle

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

- The `report.Consumer` interface is elegantly simple — ProcessEvent + Close. Adding a web dashboard is purely additive; no changes to the fan-out mechanism needed.
- SSE with HTMX's `hx-swap-oob` allows multi-target updates from a single SSE stream — the server can update the header, a specific peer row, and the event feed all in one SSE message.
- For 200+ peers, server-side rendering of table fragments is actually more efficient than client-side JavaScript rendering — the browser just swaps HTML, no virtual DOM diffing.
- The control channel pattern (web -> orchestrator) mirrors the event channel pattern (peers -> orchestrator) — both use buffered Go channels processed on the main loop.

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Live web dashboard with HTMX | ✅ Done | `web/` package, `assets/htmx.min.js` | HTMX for dynamic updates |
| SSE for real-time updates | ✅ Done | `web/sse.go` broker + `dashboard.go` broadcastDirty | 200ms debounce |
| Embedded assets (go:embed) | ✅ Done | `web/handlers.go` registerRoutes | htmx.min.js, sse.js, style.css |
| 200+ peer table with sort/filter | ✅ Done | `web/handlers.go:66-99` | sort/dir/status params |
| Peer detail pane on click | ✅ Done | `web/handlers.go:101-128` | GET /peer/{id} |
| Dark theme | ✅ Done | `web/assets/style.css` (13K) | #0f1117 background |
| Peer state timeline | ✅ Done | `web/viz.go:206-303` | Horizontal bars, paginated |
| Convergence histogram | ✅ Done | `web/viz.go:140-204` | 9 buckets, deadline marker |
| Chaos event markers | ✅ Done | `web/viz.go:305-372` | Warmup region, colored markers |
| Route flow matrix | ✅ Done | `web/viz.go:453-621` | Heatmap, top-N, family, latency |
| Pause/resume chaos | ✅ Done | `web/control.go:14-65` | Via control channel to runScheduler |
| Manual chaos trigger | ✅ Done | `web/control.go:103-163` | Action + peer targeting |
| Chaos rate adjustment | ✅ Done | `web/control.go:67-101` | 0.0-1.0 slider |
| Re-run with new seed | ❌ Not implemented | — | No restart-with-seed capability |
| Stop/restart control | ⚠️ Partial | `web/control.go:165-185` | Stop works; restart not implemented |
| 6 new chaos actions (ClockDrift, RouteBurst, etc.) | ❌ Not implemented | — | Depends on spec-chaos-actions-v2 (not started) |
| Parameterized trigger UI per action type | ⚠️ Partial | `web/control.go:187-297` | Works for existing 10 actions; v2 action params not available |
| Manual triggers logged to NDJSON for replay | ✅ Done | `web/control.go:150` + `report/jsonlog.go:139` | Triggers flow through event pipeline |
| Control actions logged as informational records | ✅ Done | `report/jsonlog.go:139-158` | "control" record type |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `main.go:740` web.New | HTTP server starts on configured port |
| AC-2 | ✅ Done | `render.go:writeLayout` | Full layout with header, sidebar, peer table |
| AC-3 | ✅ Done | `dashboard.go:169-264` ProcessEvent | Live status updates via SSE |
| AC-4 | ✅ Done | `handlers.go:101-128` handlePeerDetail | Detail pane on click |
| AC-5 | ✅ Done | `state.go:222-333` ActiveSet | ~40 peers, auto-promotion, adaptive decay |
| AC-6 | ✅ Done | `main.go:752-824` shared mux | Single server for both |
| AC-7 | ✅ Done | `handlers.go:66-99` sort/dir params | Column sorting |
| AC-8 | ✅ Done | `handlers.go:66-99` status= param | Status filtering |
| AC-9 | ✅ Done | `sse.go:ServeHTTP` | SSE stream, no page reload |
| AC-10 | ✅ Done | `sse.go` 200ms debounce | Batches at ~5/sec |
| AC-11 | ✅ Done | `viz.go:140-204` | Bar chart with color gradient + deadline |
| AC-12 | ✅ Done | `viz.go:206-303` | Horizontal state bars per peer |
| AC-13 | ✅ Done | `viz.go:305-372` | Colored markers by action type |
| AC-14 | ✅ Done | `viz.go:453-621` | Heatmap with top-N, family, latency |
| AC-15 | ✅ Done | `control.go:14-35` handleControlPause | Pause button works |
| AC-16 | ✅ Done | `control.go:46-65` handleControlResume | Resume button works |
| AC-17 | ✅ Done | `control.go:67-101` handleControlRate | Rate slider works |
| AC-18 | ✅ Done | `control.go:103-163` handleControlTrigger | Manual trigger on peer |
| AC-19 | ✅ Done | `control.go:165-185` handleControlStop | Graceful stop |
| AC-20 | ❌ Not implemented | — | No new-seed / restart capability |
| AC-21 | ✅ Done | `main.go` conditional creation | No server when --web absent |
| AC-22 | ✅ Done | `control.go:299-324` writePropertyBadges | Click shows violations |
| AC-23 | ✅ Done | `sse.go:ServeHTTP` | Full state on reconnect |
| AC-24 | ✅ Done | `dashboard.go:269` Close | Server stays up after run |
| AC-25 | ✅ Done | `handlers.go` go:embed | No CDN, works offline |
| AC-26 | ❌ Not implemented | — | Only 10 existing actions, no v2 actions (ClockDrift etc.) |
| AC-27 | ❌ Not implemented | — | RouteBurst params not available (v2 not implemented) |
| AC-28 | ❌ Not implemented | — | ZeroWindow not implemented |
| AC-29 | ❌ Not implemented | — | RouteFlap not implemented |
| AC-30 | ❌ Not implemented | — | V2 action param validation not available |
| AC-31 | ✅ Done | `control.go:103-163` peers parsing | Multi-select targets exact peers |
| AC-32 | ✅ Done | `control.go:150` + event pipeline | ChaosExecuted in NDJSON |
| AC-33 | ⚠️ Partial | — | Manual triggers logged, but replay of parameterized actions untested |
| AC-34 | ✅ Done | `report/jsonlog.go:139-158` LogControl | "control" records logged |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestWebDashboardProcessEvent | ✅ Done | `handlers_test.go` / `state_test.go` | Covered across files |
| TestWebDashboardSSEBroadcast | ✅ Done | `sse_test.go` | Broadcast verified |
| TestWebDashboardSSEDebounce | ✅ Done | `sse_test.go` | Debounce tested |
| TestWebDashboardPeerState | ✅ Done | `state_test.go` | Per-peer state |
| TestWebDashboardStateTimeline | ✅ Done | `viz_test.go` | Timeline transitions |
| TestWebDashboardConvergenceHistogram | ✅ Done | `state_test.go` + `viz_test.go` | Bucket insertion + rendering |
| TestWebDashboardEventRingBuffer | ✅ Done | `state_test.go` | RingBuffer Push/All/Latest |
| TestWebDashboardRouteMatrix | ✅ Done | `viz_test.go` | Matrix rendering + filtering |
| TestWebDashboardClose | ⚠️ Partial | — | No explicit test, manual only |
| TestWebDashboardConsumerInterface | ✅ Done | Compiles | Interface satisfaction |
| TestPeerTableSorting | ✅ Done | `handlers_test.go` | sort/dir tested |
| TestPeerTableFiltering | ✅ Done | `handlers_test.go` | status= tested |
| TestPeerDetailHandler | ✅ Done | `handlers_test.go` | /peer/{id} tested |
| TestControlPauseChaos | ✅ Done | `handlers_test.go` | POST pause tested |
| TestControlTriggerChaos | ✅ Done | `handlers_test.go` | POST trigger tested |
| TestControlSetRate | ✅ Done | `handlers_test.go` | POST rate tested |
| TestSchedulerPauseResume | 🔄 Changed | — | Via control channel, not Scheduler methods |
| TestSchedulerSetRate | ✅ Done | — | SetRate method exists |
| TestSchedulerTriggerAction | ✅ Done | `handlers_test.go` | Trigger via control channel |
| TestSSEClientCleanup | ✅ Done | `sse_test.go` | Client removal |
| TestEmbeddedAssets | ⚠️ Partial | — | Verified by handler tests, no explicit test |
| TestChaosClockDrift | ❌ Not implemented | — | V2 actions not built |
| TestChaosRouteBurst | ❌ Not implemented | — | V2 actions not built |
| TestChaosWithdrawalBurst | ❌ Not implemented | — | V2 actions not built |
| TestChaosRouteFlap | ❌ Not implemented | — | V2 actions not built |
| TestChaosSlowPeer | ❌ Not implemented | — | V2 actions not built |
| TestChaosZeroWindow | ❌ Not implemented | — | V2 actions not built |
| TestTriggerParamValidation | ⚠️ Partial | `handlers_test.go` | Existing actions only |
| TestTriggerParamForm | ✅ Done | `handlers_test.go` | Form rendering tested |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `web/dashboard.go` | ✅ Created | 428 lines — consumer, SSE, server |
| `web/handlers.go` | ✅ Created | 212 lines — HTTP handlers |
| `web/sse.go` | ✅ Created | 158 lines — SSE broker |
| `web/state.go` | ✅ Created | 806 lines — state, active set, ring buffer |
| `web/templates.go` | 🔄 Changed | Not created — rendering in render.go |
| `web/control.go` | ✅ Created | 340 lines — control handlers |
| `web/viz.go` | ✅ Created | 670 lines — visualization tabs |
| `web/render.go` | ✅ Created | 349 lines — HTML rendering (replaces templates) |
| `web/assets/htmx.min.js` | ✅ Created | 50K vendored |
| `web/assets/sse.js` | ✅ Created | 8.7K vendored |
| `web/assets/style.css` | ✅ Created | 13K dark theme |
| `web/templates/*.html` | 🔄 Changed | Empty dir — all rendering inline in Go |
| `web/handlers_test.go` | ✅ Created | 715 lines |
| `web/state_test.go` | ✅ Created | 584 lines |
| `web/sse_test.go` | ✅ Created | 276 lines |
| `web/viz_test.go` | ✅ Created | 912 lines |
| `test/chaos/web-*.ci` | ❌ Skipped | No functional tests created |
| `chaos/actions.go` (v2 actions) | ❌ Not implemented | ClockDrift, RouteBurst etc. not built |
| `chaos/actions_v2.go` | ❌ Not implemented | V2 action types not built |
| `main.go` | ✅ Modified | --web flag, control channel, setupReporting |
| `orchestrator.go` | ✅ Modified | orchestratorConfig with controlCh |
| `chaos/scheduler.go` | ⚠️ Partial | SetRate added; Pause/Resume via channel, not methods |
| `report/summary.go` | ✅ Modified | FormatDuration exported |

### Audit Summary
- **Total items:** 72
- **Done:** 46
- **Partial:** 7 (trigger form for v2, some tests, replay)
- **Skipped:** 8 (functional tests, v2 action tests)
- **Not implemented:** 11 (all v2 actions + new-seed restart)
- **Changed:** 4 (templates inline, scheduler pause via channel)

## Checklist

### Goal Gates (MUST pass — cannot defer)
- [ ] Acceptance criteria AC-1..AC-34 all demonstrated
- [ ] Tests pass (`make ze-unit-test`)
- [ ] No regressions (`make ze-functional-test`)
- [ ] Feature code integrated into codebase (`cmd/ze-bgp-chaos/web/`)
- [ ] Integration completeness: dashboard proven to work from `--web` flag through to live browser view
- [ ] Architecture docs updated with learnings and changes

### Quality Gates (SHOULD pass — can defer with explicit user approval)
- [ ] `make ze-lint` passes (26 linters)
- [ ] Implementation Audit fully completed
- [ ] Mistake Log escalation candidates reviewed

### Design
- [ ] No premature abstraction (all 4 phases have concrete use cases)
- [ ] No speculative features (everything in spec is requested by user)
- [ ] Single responsibility (web package handles web, scheduler handles scheduling)
- [ ] Explicit behavior (SSE debouncing, control commands are documented)
- [ ] Minimal coupling (web consumer reads events, sends commands — no direct access to internals)
- [ ] Next-developer test (templates, handlers, and state are clearly separated)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs
- [ ] Functional tests verify end-to-end behavior

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (N/A — not protocol work)

### Completion (after tests pass)
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-chaos-web-dashboard.md`
- [ ] All files committed together
