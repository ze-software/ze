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
| Event stream feed | ✅ Done | `viz.go:27-38` handleVizEvents, `viz.go:75-137` writeEventStream | Scrollable feed, newest first |
| Event filtering (peer, type) | ✅ Done | `viz.go:27-38` peer= and type= query params | Both filters work |
| Auto-scroll with toggle | ✅ Done | `viz.go:104-106` JS checkbox | `window._autoScroll` toggle |
| Convergence histogram | ✅ Done | `viz.go:140-204` writeConvergenceHistogram | 9 buckets, CSS bar chart, color gradient |
| Deadline marker | ✅ Done | `viz.go:140-204` | Vertical dashed line at deadline position |
| Peer state timeline | ✅ Done | `viz.go:206-303` writePeerTimeline | Horizontal bars, green/red/yellow segments |
| Timeline pagination (200+) | ✅ Done | `viz.go:49-64` page= param | Prev/Next navigation links |
| Chaos event timeline | ✅ Done | `viz.go:305-372` writeChaosTimeline | Markers by action type |
| Warmup region | ✅ Done | `viz.go:318-325` | Shaded warmup period at start |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `render.go:writeLayout` tab bar + `handlers.go` /viz/* routes | 5 tabs (Events, Timeline, Convergence, Chaos, Route Matrix) |
| AC-2 | ✅ Done | `viz.go:75-137` writeEventStream | Scrollable feed, ring buffer backed |
| AC-3 | ✅ Done | `dashboard.go:broadcastDirty` SSE events | New events prepended via SSE |
| AC-4 | ✅ Done | `viz.go:27-38` peer= param | Filters to single peer |
| AC-5 | ✅ Done | `viz.go:27-38` type= param | Filters by event type |
| AC-6 | ✅ Done | `viz.go:104-106` JS auto-scroll checkbox | Toggle button in UI |
| AC-7 | ✅ Done | `viz.go:206-303` writePeerTimeline | Horizontal state bars per peer |
| AC-8 | ✅ Done | `viz.go:49-64` page= param | Paginated at configurable page size |
| AC-9 | ✅ Done | `viz.go:140-204` writeConvergenceHistogram | 9 latency buckets with color gradient |
| AC-10 | ✅ Done | `viz.go:140-204` deadline marker | Dashed vertical line at deadline |
| AC-11 | ✅ Done | `dashboard.go:broadcastDirty` convergence SSE | Updates via SSE on convergence events |
| AC-12 | ✅ Done | `viz.go:305-372` writeChaosTimeline | Markers positioned by time, colored by type |
| AC-13 | ⚠️ Partial | `viz.go:305-372` | Markers rendered but no explicit click-to-highlight peer behavior found |
| AC-14 | ✅ Done | `viz.go:318-325` | Shaded warmup region at start |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestConvergenceHistogramBuckets | ✅ Done | `state_test.go` | Bucket insertion tested |
| TestConvergenceHistogramStats | ✅ Done | `state_test.go` | Min/avg/max stats |
| TestEventStreamFiltering | ✅ Done | `viz_test.go` | peer= filter tested |
| TestEventStreamTypeFilter | ✅ Done | `viz_test.go` | type= filter tested |
| TestPeerTimelinePagination | ✅ Done | `viz_test.go` | page= param tested |
| TestChaosTimelineMarkers | ✅ Done | `viz_test.go` | Markers rendered |
| TestChaosTimelineWarmup | ✅ Done | `viz_test.go` | Warmup region tested |
| TestSSEConvergenceEvent | ⚠️ Partial | — | Convergence broadcasts tested via broadcastDirty, no specific 2s interval test |
| TestSSEEventFeedThrottle | ⚠️ Partial | — | SSE debounce tested but no explicit 10/s throttle test |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `web/state.go` | ✅ Modified | Convergence histogram buckets, chaos history added |
| `web/dashboard.go` | ✅ Modified | ProcessEvent updates histogram and chaos history |
| `web/sse.go` | ✅ Modified | Convergence + event SSE events via broadcastDirty |
| `web/handlers.go` | ✅ Modified | /viz/* endpoint handlers added |
| `web/viz.go` | ✅ Created | All visualization rendering (670 lines) |
| `web/viz_test.go` | ✅ Created | 912 lines of viz tests |
| `web/templates/tabs.html` | 🔄 Changed | Not created — tabs rendered inline in render.go |
| `web/templates/event_feed.html` | 🔄 Changed | Not created — inline in viz.go |
| `web/templates/convergence.html` | 🔄 Changed | Not created — inline in viz.go |
| `web/templates/peer_timeline.html` | 🔄 Changed | Not created — inline in viz.go |
| `web/templates/chaos_timeline.html` | 🔄 Changed | Not created — inline in viz.go |
| `test/chaos/web-viz.ci` | ❌ Skipped | No functional tests created |

### Audit Summary
- **Total items:** 30
- **Done:** 23
- **Partial:** 3 (AC-13 click-to-highlight, SSE convergence interval, SSE throttle test)
- **Skipped:** 1 (functional test)
- **Changed:** 5 (template files replaced by inline rendering in viz.go)

## Checklist

### Goal Gates
- [ ] AC-1..AC-14 demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] `make ze-lint` passes
- [ ] All 4 tabs render correctly in browser

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
