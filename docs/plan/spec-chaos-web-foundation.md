# Spec: chaos-web-foundation

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/architecture/chaos-web-dashboard.md` - full design (layout, SSE, theme, state)
3. `docs/plan/spec-chaos-web-dashboard.md` - parent spec (overall vision)
4. `cmd/ze-bgp-chaos/report/reporter.go` - Consumer interface
5. `cmd/ze-bgp-chaos/report/metrics.go` - existing HTTP server pattern
6. `cmd/ze-bgp-chaos/main.go` - setupReporting(), flag parsing

## Task

Implement the foundation layer of the chaos web dashboard: HTTP server with embedded assets, WebDashboard consumer, SSE broker, main layout with dark theme, peer table with active set (auto-promotion, adaptive decay, pinning), and peer detail pane.

This is **Phase 1** of the chaos web dashboard. It delivers a fully functional view-only dashboard. All subsequent sub-specs (visualizations, controls, route matrix) build on this foundation.

**Parent spec:** `docs/plan/spec-chaos-web-dashboard.md`
**Design doc:** `docs/architecture/chaos-web-dashboard.md`

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/chaos-web-dashboard.md` - Full design document
  -> Constraint: Three-panel layout (header, sidebar 320px, main content)
  -> Decision: SSE with hx-swap-oob for multi-target updates, 200ms debounce
  -> Decision: Dark theme (#0f1117 background, system monospace for data)
- [ ] `cmd/ze-bgp-chaos/report/reporter.go` - Consumer interface
  -> Constraint: ProcessEvent() runs synchronously on main loop, must be fast
- [ ] `cmd/ze-bgp-chaos/report/metrics.go` - HTTP server lifecycle pattern
  -> Decision: Server created in setupReporting(), cleanup via returned function
- [ ] `cmd/ze-bgp-chaos/main.go` - Flag parsing, setupReporting()
  -> Constraint: --web flag added alongside existing flags
  -> Decision: Share HTTP server with --metrics when both specified

**Key insights:**
- ProcessEvent() must only update state and signal dirty flags — never block on I/O
- SSE broker runs in background goroutine, reads dirty flags every 200ms
- HTTP handlers take read lock on state, ProcessEvent takes write lock
- Embedded assets via go:embed — self-contained binary

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze-bgp-chaos/report/reporter.go` - Consumer: ProcessEvent(ev) + Close() error
- [ ] `cmd/ze-bgp-chaos/report/metrics.go` - Metrics consumer with HTTP handler
- [ ] `cmd/ze-bgp-chaos/report/dashboard.go` - Terminal dashboard (one line per event)
- [ ] `cmd/ze-bgp-chaos/main.go` - setupReporting() creates consumers, starts HTTP servers
- [ ] `cmd/ze-bgp-chaos/orchestrator.go` - EventProcessor, orchestratorConfig
- [ ] `cmd/ze-bgp-chaos/peer/event.go` - 10 EventType constants, Event struct

**Behavior to preserve:**
- All existing CLI flags and reporters unchanged
- --web is purely additive (no flag = no overhead)

**Behavior to change:**
- setupReporting() extended to create WebDashboard when --web is set
- When both --web and --metrics are specified, share one HTTP server with multiple routes

## Data Flow (MANDATORY)

### Entry Point
- Events from peer.Event channel -> Reporter.Process() -> WebDashboard.ProcessEvent()

### Transformation Path
1. ProcessEvent() updates per-peer state, counters, ring buffers (write lock)
2. ProcessEvent() sets dirty flags for changed components
3. SSE goroutine wakes every 200ms, renders changed fragments, broadcasts to clients
4. HTTP GET handlers read current state snapshot (read lock), render templates

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Main loop -> WebDashboard | ProcessEvent() call (synchronous) | [ ] |
| WebDashboard -> SSE clients | Broadcast channel per client | [ ] |
| Browser -> HTTP server | HTMX GET requests for fragments | [ ] |

### Integration Points
- `report.Consumer` interface — WebDashboard implements ProcessEvent + Close
- `setupReporting()` in main.go — Creates WebDashboard, starts HTTP server, returns cleanup
- `orchestratorConfig` — Extended with webAddr string field
- `report.FormatDuration()` — Exported for reuse in web templates

### Architectural Verification
- [ ] No bypassed layers (events flow through Reporter)
- [ ] No unintended coupling (web package only depends on peer.Event)
- [ ] No duplicated functionality (new consumer, doesn't replace existing ones)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `--web :8080` flag provided | HTTP server starts on port 8080, serves dashboard at `/` |
| AC-2 | Browser opens `http://localhost:8080/` | Full dashboard layout renders with header, sidebar, peer table |
| AC-3 | Chaos run in progress with 4+ peers | Active set shows peers with recent events, live SSE updates |
| AC-4 | Click on a peer row in the table | Detail pane opens with peer info, route breakdown, recent events |
| AC-5 | 200 peers running, events occurring | Active set shows ~40 most relevant peers, no browser lag |
| AC-6 | `--web` and `--metrics` both specified | Single HTTP server serves both `/` and `/metrics` |
| AC-7 | Column header clicked | Table re-sorts by that column (toggle ascending/descending) |
| AC-8 | Filter set to "status=down" | Table shows only disconnected peers within active set |
| AC-11 | Pin icon clicked on a peer row | Peer becomes pinned, survives decay, pin icon filled |
| AC-12 | Pinned peer unpinned | Peer subject to normal decay rules again |
| AC-13 | High churn, active set at capacity | Oldest non-pinned peers decay within 5s to make room |
| AC-14 | Low churn, active set below 50% | Auto-promoted peers remain visible for up to 120s |
| AC-15 | Peer added via peer picker dropdown | Peer appears in table at its natural index position |
| AC-9 | SSE connection established | Dashboard updates without page reload as events arrive |
| AC-10 | High event rate (1000+ events/sec) | SSE debouncing batches updates at ~5/sec |
| AC-21 | No `--web` flag | No HTTP server started, no overhead |
| AC-23 | Browser disconnects and reconnects SSE | Full state recovered from server |
| AC-24 | Run completes naturally | Dashboard shows final state, remains viewable |
| AC-25 | Assets served from embedded files | No CDN requests, works offline |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestWebDashboardProcessEvent | `web/dashboard_test.go` | State updated for all 10 event types | |
| TestWebDashboardConsumerInterface | `web/dashboard_test.go` | Satisfies report.Consumer | |
| TestWebDashboardPeerState | `web/state_test.go` | Per-peer status, counts, last event | |
| TestWebDashboardEventRingBuffer | `web/state_test.go` | Ring buffer keeps last N, drops oldest | |
| TestWebDashboardClose | `web/dashboard_test.go` | Shuts down HTTP server and SSE connections | |
| TestSSEBroadcast | `web/sse_test.go` | Events broadcast to connected clients | |
| TestSSEDebounce | `web/sse_test.go` | High-frequency events batched | |
| TestSSEClientCleanup | `web/sse_test.go` | Disconnected clients removed | |
| TestPeerTableSorting | `web/handlers_test.go` | /peers?sort=asn&dir=asc returns sorted rows | |
| TestPeerTableFiltering | `web/handlers_test.go` | /peers?status=up returns only established | |
| TestActiveSetPromotion | `web/state_test.go` | Noteworthy event promotes peer into active set | |
| TestActiveSetDecay | `web/state_test.go` | Non-pinned peer decays after adaptive TTL | |
| TestActiveSetAdaptiveTTL | `web/state_test.go` | TTL shortens as active set fills up | |
| TestActiveSetPinning | `web/state_test.go` | Pinned peer survives decay, unpin re-enables decay | |
| TestActiveSetCapacity | `web/state_test.go` | Active set never exceeds max-visible | |
| TestActiveSetStableOrder | `web/state_test.go` | Peer positions don't change when others appear/disappear | |
| TestPeerDetailHandler | `web/handlers_test.go` | /peer/3 returns detail pane for peer 3 | |
| TestPeerPinHandler | `web/handlers_test.go` | POST /peers/3/pin toggles pin state | |
| TestEmbeddedAssets | `web/dashboard_test.go` | go:embed assets non-empty | |
| TestSharedHTTPServer | `web/dashboard_test.go` | Web + metrics share single server | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Web port | 1-65535 | 65535 | 0 (disabled) | 65536 |
| Max visible peers | 10-200 | 200 | 9 (default 40) | N/A |
| Ring buffer size | 1-10000 | 10000 | 0 (default 1000) | N/A |
| SSE debounce interval | 50ms-2000ms | 2000ms | 49ms (clamp) | N/A |
| Decay TTL minimum | 1s-120s | 120s | 0 (clamp to 1s) | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-web-startup | `test/chaos/web-startup.ci` | --web :0 starts server, GET / returns 200 | |
| test-web-sse | `test/chaos/web-sse.ci` | SSE streams events during short run | |
| test-web-metrics-coexist | `test/chaos/web-metrics.ci` | Both / and /metrics respond | |
| test-web-no-flag | `test/chaos/web-no-flag.ci` | No --web = no server | |
| test-web-assets | `test/chaos/web-assets.ci` | /assets/htmx.min.js returns JS | |

## Files to Modify

- `cmd/ze-bgp-chaos/main.go` - Add --web flag, wire WebDashboard in setupReporting(), share HTTP server with metrics
- `cmd/ze-bgp-chaos/orchestrator.go` - (Minor) Extend orchestratorConfig with webAddr field
- `cmd/ze-bgp-chaos/report/summary.go` - Export FormatDuration for web templates

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | Yes | `cmd/ze-bgp-chaos/main.go` (--web flag + usage) |
| Functional tests | Yes | `test/chaos/web-*.ci` |

## Files to Create

- `cmd/ze-bgp-chaos/web/dashboard.go` - WebDashboard consumer: state, SSE integration, HTTP server setup
- `cmd/ze-bgp-chaos/web/state.go` - Per-peer state, ring buffer, status enum
- `cmd/ze-bgp-chaos/web/sse.go` - SSE broker: client registration, broadcast, debouncing
- `cmd/ze-bgp-chaos/web/handlers.go` - HTTP handlers: index, peers (sort/filter/page), peer detail, assets
- `cmd/ze-bgp-chaos/web/templates.go` - Template loading via go:embed
- `cmd/ze-bgp-chaos/web/assets/htmx.min.js` - Vendored HTMX library
- `cmd/ze-bgp-chaos/web/assets/sse.js` - Vendored HTMX SSE extension
- `cmd/ze-bgp-chaos/web/assets/style.css` - Dark theme CSS
- `cmd/ze-bgp-chaos/web/templates/layout.html` - Page shell
- `cmd/ze-bgp-chaos/web/templates/header.html` - Header bar fragment
- `cmd/ze-bgp-chaos/web/templates/sidebar.html` - Summary cards, property badges
- `cmd/ze-bgp-chaos/web/templates/peers.html` - Peer table
- `cmd/ze-bgp-chaos/web/templates/peer_detail.html` - Peer detail pane
- `cmd/ze-bgp-chaos/web/dashboard_test.go` - Unit tests
- `cmd/ze-bgp-chaos/web/state_test.go` - State tests
- `cmd/ze-bgp-chaos/web/sse_test.go` - SSE broker tests
- `cmd/ze-bgp-chaos/web/handlers_test.go` - Handler tests
- `test/chaos/web-startup.ci` - Functional test
- `test/chaos/web-assets.ci` - Functional test

## Implementation Steps

1. **Vendor HTMX assets** - Download htmx.min.js + sse.js, place in web/assets/
2. **Write state types (TDD)** - Per-peer state, ring buffer, status enum
3. **Write SSE broker (TDD)** - Client registration, broadcast, debouncing
4. **Write WebDashboard consumer (TDD)** - ProcessEvent updates state, Close shuts down
5. **Write dark theme CSS** - Layout grid, cards, table styles, color palette
6. **Write HTML templates** - Layout shell, header, sidebar, peer table, detail pane
7. **Write HTTP handlers** - Index, peers (sort/filter/page), peer detail, SSE, assets
8. **Wire into main.go** - --web flag, setupReporting(), shared HTTP server
9. **Functional tests** - Startup, SSE, metrics coexistence, assets
10. **Manual test** - `ze-bgp-chaos --web :8080 --peers 10 --duration 60s`, verify in browser

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| HTTP server with --web flag | | | |
| WebDashboard Consumer | | | |
| SSE broker with debouncing | | | |
| Three-panel layout | | | |
| Dark theme | | | |
| Peer table with active set (promotion/decay/pinning) | | | |
| Peer detail pane | | | |
| Embedded assets (go:embed) | | | |
| Shared server with --metrics | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |
| AC-10 | | | |
| AC-11 | | | |
| AC-12 | | | |
| AC-13 | | | |
| AC-14 | | | |
| AC-15 | | | |
| AC-21 | | | |
| AC-23 | | | |
| AC-24 | | | |
| AC-25 | | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**

## Checklist

### Goal Gates
- [ ] AC-1..AC-15, AC-21, AC-23..AC-25 demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] `make ze-lint` passes
- [ ] Dashboard loads in browser with live SSE updates

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
