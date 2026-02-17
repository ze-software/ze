# Spec: chaos-web-viz

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/architecture/chaos-web-dashboard.md` - "Visualization Tabs" section
3. `cmd/ze-bgp-chaos/web/` - foundation package (from spec-chaos-web-foundation)
4. `cmd/ze-bgp-chaos/web/state.go` - per-peer state, ring buffer

## Task

Add 4 visualization tabs to the chaos web dashboard: **Event Stream**, **Peer State Timeline**, **Convergence Histogram**, and **Chaos Event Timeline**.

Each tab is lazily loaded via HTMX GET on first click, and updated via SSE for live data. All visualizations are rendered server-side as HTML fragments — no JavaScript charting libraries.

**Parent spec:** `docs/plan/spec-chaos-web-dashboard.md`
**Design doc:** `docs/architecture/chaos-web-dashboard.md` (section "Visualization Tabs")
**Depends on:** `spec-chaos-web-foundation` (provides WebDashboard, SSE broker, state types, HTTP server)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/chaos-web-dashboard.md` - Visualization tab designs
  -> Decision: Tab 1 = Event Stream (ring buffer, color-coded, filterable)
  -> Decision: Tab 2 = Peer State Timeline (horizontal bars, green/red/yellow segments)
  -> Decision: Tab 3 = Convergence Histogram (CSS bar chart, 9 buckets, deadline marker)
  -> Decision: Tab 4 = Chaos Timeline (horizontal timeline, markers by action type)
- [ ] `cmd/ze-bgp-chaos/web/state.go` - State types from foundation
  -> Constraint: Event ring buffer and per-peer state history already tracked
  -> Decision: Convergence histogram needs bucket state (9 buckets + running stats)

**Key insights:**
- All visualizations are pure CSS — no canvas, SVG, or charting libraries
- Event stream uses SSE prepend for live updates (newest on top)
- Peer timeline paginates at 30 peers per page for 200+ scale
- Convergence histogram updates every 2s via SSE

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze-bgp-chaos/web/state.go` - Foundation state types
- [ ] `cmd/ze-bgp-chaos/web/sse.go` - SSE broker from foundation
- [ ] `cmd/ze-bgp-chaos/web/handlers.go` - Existing handler patterns
- [ ] `cmd/ze-bgp-chaos/web/templates/` - Existing template structure

**Behavior to preserve:**
- Foundation layout, peer table, detail pane unchanged
- SSE event types from foundation (tick, stats, peer-update) still work

**Behavior to change:**
- State types extended with convergence histogram buckets
- SSE extended with event feed and convergence update events
- New tab bar added below peer table/detail pane

## Data Flow (MANDATORY)

### Entry Point
- Events from peer.Event channel -> Reporter.Process() -> WebDashboard.ProcessEvent()
- Same entry point as foundation — visualization state is updated alongside peer state

### Transformation Path
1. ProcessEvent() updates per-peer state (existing from foundation)
2. ProcessEvent() additionally updates: convergence histogram buckets, chaos history list
3. ProcessEvent() sets dirty flags for changed visualization components
4. SSE goroutine renders changed visualization fragments (convergence every 2s, event feed throttled to 10/s)
5. HTTP GET handlers read state snapshot and render full visualization tab on first load

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Main loop -> WebDashboard | ProcessEvent() call (synchronous) | [ ] |
| WebDashboard -> SSE clients | Convergence + event-feed SSE events | [ ] |
| Browser -> HTTP server | HTMX GET for /viz/* tab content | [ ] |

### Integration Points
- `web/state.go` — Extended with ConvergenceHistogram and ChaosHistory types
- `web/dashboard.go` ProcessEvent() — Updates histogram buckets on convergence events
- `web/sse.go` SSE broker — New event types for convergence (2s) and event-feed (throttled)
- `web/handlers.go` — New /viz/* handlers following existing handler patterns
- `web/templates/` — New tab templates following existing template structure

### Architectural Verification
- [ ] No bypassed layers (events flow through Reporter)
- [ ] No unintended coupling (viz handlers read state via same read lock as peer table)
- [ ] No duplicated functionality (extends foundation state and SSE, doesn't recreate)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Tab bar visible below peer area | 4 tabs: Events, Peer Timeline, Convergence, Chaos Timeline |
| AC-2 | Event Stream tab active | Scrollable feed of last 500 events, newest first |
| AC-3 | Events arriving during stream view | New events prepended via SSE without page reload |
| AC-4 | Event stream filter set to peer=3 | Only events for peer 3 shown |
| AC-5 | Event stream filter set to type=ChaosExecuted | Only chaos events shown |
| AC-6 | User scrolls up in event stream | Auto-scroll pauses; toggle button appears |
| AC-7 | Peer State Timeline tab selected | Horizontal bars showing connected/disconnected periods |
| AC-8 | 200+ peers with timeline | Paginated at 30 peers per page, filter controls available |
| AC-9 | Convergence Histogram tab selected | Bar chart with 9 latency buckets, color gradient |
| AC-10 | Convergence deadline configured | Vertical dashed line at deadline position on histogram |
| AC-11 | Routes converging live | Histogram updates every 2s via SSE |
| AC-12 | Chaos Timeline tab selected | Horizontal timeline with markers per chaos event |
| AC-13 | Chaos event marker clicked | Affected peer highlighted, event details shown |
| AC-14 | Warmup period visible | Shaded region at start of chaos timeline |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestConvergenceHistogramBuckets | `web/state_test.go` | Latency values bucketed into correct 9 buckets | |
| TestConvergenceHistogramStats | `web/state_test.go` | Running min/avg/max/p99 updated correctly | |
| TestEventStreamFiltering | `web/handlers_test.go` | /events?peer=3 returns only peer 3 events | |
| TestEventStreamTypeFilter | `web/handlers_test.go` | /events?type=ChaosExecuted filters by type | |
| TestPeerTimelinePagination | `web/handlers_test.go` | /viz/peer-timeline?page=2 returns correct peer subset | |
| TestChaosTimelineMarkers | `web/handlers_test.go` | /viz/chaos-timeline returns markers positioned by time | |
| TestChaosTimelineWarmup | `web/handlers_test.go` | Warmup period included in timeline response | |
| TestSSEConvergenceEvent | `web/sse_test.go` | Convergence SSE event sent every 2s | |
| TestSSEEventFeedThrottle | `web/sse_test.go` | Event feed SSE throttled to 10/s max | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-web-viz-tabs | `test/chaos/web-viz.ci` | GET /viz/convergence, /viz/chaos-timeline return HTML | |

## Files to Modify

- `cmd/ze-bgp-chaos/web/state.go` - Add convergence histogram bucket state, chaos history list
- `cmd/ze-bgp-chaos/web/dashboard.go` - ProcessEvent updates histogram buckets and chaos history
- `cmd/ze-bgp-chaos/web/sse.go` - Add convergence and event-feed SSE event types
- `cmd/ze-bgp-chaos/web/handlers.go` - Add visualization endpoint handlers

## Files to Create

- `cmd/ze-bgp-chaos/web/templates/tabs.html` - Tab bar fragment
- `cmd/ze-bgp-chaos/web/templates/event_feed.html` - Event stream feed
- `cmd/ze-bgp-chaos/web/templates/convergence.html` - Histogram (CSS bars)
- `cmd/ze-bgp-chaos/web/templates/peer_timeline.html` - State timeline bars
- `cmd/ze-bgp-chaos/web/templates/chaos_timeline.html` - Chaos event markers
- `test/chaos/web-viz.ci` - Functional test

## Implementation Steps

1. **Add histogram state (TDD)** - 9 buckets, running stats, bucket insertion
2. **Add chaos history state** - Ordered list of (time, peer, action)
3. **Event stream handler (TDD)** - /events endpoint with peer/type filters
4. **Convergence handler (TDD)** - /viz/convergence renders CSS bar chart
5. **Peer timeline handler (TDD)** - /viz/peer-timeline with pagination
6. **Chaos timeline handler (TDD)** - /viz/chaos-timeline with markers
7. **SSE extensions** - convergence (2s) and event-feed (throttled) events
8. **CSS for visualizations** - Bar chart styles, timeline bars, markers, color gradients
9. **Tab bar template** - Tab switching via HTMX
10. **Functional tests**

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Event stream feed | | | |
| Event filtering (peer, type) | | | |
| Auto-scroll with toggle | | | |
| Convergence histogram | | | |
| Deadline marker | | | |
| Peer state timeline | | | |
| Timeline pagination (200+) | | | |
| Chaos event timeline | | | |
| Warmup region | | | |

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

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**

## Checklist

### Goal Gates
- [ ] AC-1..AC-14 demonstrated
- [ ] `make unit-test` passes
- [ ] `make functional-test` passes
- [ ] `make lint` passes
- [ ] All 4 tabs render correctly in browser

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
