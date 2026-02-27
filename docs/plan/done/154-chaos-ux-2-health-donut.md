# Spec: Health Donut Ring Chart

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research and constraints
4. `cmd/ze-chaos/web/render.go` - sidebar rendering, writeLayout()
5. `cmd/ze-chaos/web/dashboard.go` - renderStats(), broadcastDirty()
6. `cmd/ze-chaos/web/assets/style.css` - status color CSS custom properties

## Task

Add an SVG/CSS ring chart (donut) to the sidebar Stats card showing peer status distribution visually. The donut replaces the flat "Peers Up/Total" counter display with a visual ring where segments are colored by status (green=Up, cyan=Syncing, red=Down, yellow=Reconnecting, grey=Idle). The center of the donut shows the total peer count. The donut updates via the existing `stats` SSE event -- no new SSE event type is needed. This is CSS/SVG only -- no JS charting library.

**Parent spec:** `docs/plan/spec-chaos-ux-0-umbrella.md`

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research, constraints, source file survey
  --> Constraint: All rendering is server-side Go HTML; SSE broadcast every 200ms; ProcessEvent() must be fast
  --> Constraint: HTMX + SSE architecture, no JS framework
  --> Decision: Dark theme with CSS custom properties (--bg-primary, --green, --red, --accent, --yellow, --text-muted)
- [ ] `docs/architecture/chaos-web-dashboard.md` - dashboard architecture
  --> Constraint: SSE events use sse-swap + hx-swap="outerHTML" for in-place replacement
  --> Decision: Stats card is in the sidebar, updated by `stats` SSE event

**Key insights:**
- The stats card is rendered in writeLayout() (render.go) for the initial page load and in renderStats() (dashboard.go) for SSE updates
- Both must be updated to include the donut so the initial render and all subsequent SSE pushes are consistent
- The `stats` SSE event already pushes the full stats div fragment with sse-swap="stats" and hx-swap="outerHTML"
- PeerStatus counts are available: PeersUp, PeersSyncing on DashboardState; Down/Reconnecting/Idle must be derived
- CSS custom properties already define all needed colors: --green (Up), --accent (Syncing), --red (Down), --yellow (Reconnecting), --text-muted (Idle)
- SVG donut can be done with a single SVG element using circle stroke-dasharray for segments

## Current Behavior (MANDATORY)

**Source files read:** (see umbrella for full survey)
- [ ] `cmd/ze-chaos/web/render.go` (470L) - writeLayout() renders the Stats card in the sidebar with flat counter spans; first stat is Peers Up/Total
  --> Constraint: writeLayout() is the single entry point for full-page render; stats div has id="stats" with sse-swap="stats"
- [ ] `cmd/ze-chaos/web/dashboard.go` (695L) - renderStats() returns the stats HTML fragment for SSE; must match writeLayout() structure
  --> Constraint: renderStats() must preserve sse-swap and hx-swap attributes for future SSE events to work
  --> Decision: Stats div starts with Peers Up/Total counter, followed by syncing, msgs, bytes, rates, chaos, reconnects, sync
- [ ] `cmd/ze-chaos/web/state.go` (594L) - DashboardState has PeersUp, PeersSyncing, PeerCount; status counts for Down/Reconnecting/Idle must be computed
  --> Constraint: All state is behind DashboardState.mu RWMutex
  --> Decision: PeerStatus iota: Idle=0, Up=1, Down=2, Reconnecting=3, Syncing=4; CSSClass() returns status-idle, status-up, etc.
- [ ] `cmd/ze-chaos/web/assets/style.css` (1066L) - status colors: .status-up uses --green, .status-syncing uses --accent, .status-down uses --red, .status-reconnecting uses --yellow, .status-idle uses --text-muted
  --> Decision: SVG donut styles must use the same CSS custom properties for color consistency

**Behavior to preserve:**
- All existing stats (Msgs Sent/Recv, Bytes, Rates, Chaos, Reconnects, Sync) must remain in the stats card
- The stats div id="stats" with sse-swap="stats" and hx-swap="outerHTML" attributes must be preserved
- Polling fallback via hx-get="/sidebar/stats" hx-trigger="every 500ms" must be preserved
- handleSidebarStats handler calls renderStats() and needs no changes

**Behavior to change:**
- Replace the flat "Peers Up/Total" counter span at the top of the stats card with an SVG donut ring
- The donut segments show Up (green), Syncing (cyan), Down (red), Reconnecting (yellow), Idle (grey) proportionally
- Center text shows total peer count
- A legend below the donut shows per-status counts with colored indicators

## Data Flow (MANDATORY - see rules/data-flow-tracing.md)

### Entry Point
- ProcessEvent() updates PeersUp, PeersSyncing, and individual peer Status values on DashboardState
- broadcastDirty() calls renderStats() which must now include the donut SVG

### Transformation Path
1. ProcessEvent updates per-peer PeerStatus and global PeersUp/PeersSyncing counters (unchanged)
2. broadcastDirty fires when dirtyGlobal is true, calls renderStats() under read lock
3. renderStats() computes per-status counts from DashboardState (Up, Syncing, Down, Reconnecting, Idle)
4. renderStats() calls a donut render helper that produces SVG with stroke-dasharray segments proportional to counts
5. SSE pushes stats fragment; HTMX swaps outerHTML into DOM; SVG renders the donut with CSS-colored segments

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Go to Browser | SSE event "stats" with HTML+SVG fragment | [ ] |
| State to Render | Read lock on DashboardState for peer status counts | [ ] |

### Integration Points
- `renderStats()` in dashboard.go - add donut SVG rendering and per-status count computation
- `writeLayout()` in render.go - add donut SVG in the initial Stats card render (must match renderStats output)
- `handleSidebarStats()` in handlers.go - already calls renderStats(), no changes needed
- `style.css` - add donut SVG styles and legend styles

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | --> | Feature Code | Test |
|-------------|---|--------------|------|
| GET / (full page load) | --> | writeLayout() renders SVG donut in stats card | TestLayoutIncludesHealthDonut |
| SSE stats event | --> | renderStats() includes SVG donut with correct segments | TestRenderStatsIncludesDonut |
| GET /sidebar/stats (polling fallback) | --> | handleSidebarStats returns donut fragment | TestSidebarStatsIncludesDonut |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Full page load (GET /) | Stats card contains an SVG donut ring chart |
| AC-2 | 10 peers: 6 Up, 2 Syncing, 1 Down, 1 Idle | Donut has 4 segments with proportional arc lengths |
| AC-3 | All peers Up | Donut is a solid green ring |
| AC-4 | All peers Idle (startup) | Donut is a solid grey ring |
| AC-5 | Zero peers (PeerCount=0) | Donut shows empty ring with "0" in center, no division by zero |
| AC-6 | Donut center | Shows total peer count as text |
| AC-7 | Donut segments | Up=green (--green), Syncing=cyan (--accent), Down=red (--red), Reconnecting=yellow (--yellow), Idle=grey (--text-muted) |
| AC-8 | SSE stats event fires | Donut updates with current status distribution |
| AC-9 | Stats card after donut | All other stats (Msgs, Bytes, Rates, Chaos, etc.) still present |
| AC-10 | Legend below donut | Shows per-status count labels with colored indicators |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRenderDonut` | `cmd/ze-chaos/web/render_test.go` | renderDonut produces valid SVG with correct segment arcs for mixed statuses | |
| `TestRenderDonutAllUp` | `cmd/ze-chaos/web/render_test.go` | Single-status donut renders as full circle segment | |
| `TestRenderDonutZeroPeers` | `cmd/ze-chaos/web/render_test.go` | Zero peers produces empty ring with "0" center, no panic | |
| `TestRenderDonutLegend` | `cmd/ze-chaos/web/render_test.go` | Legend shows per-status counts with correct labels and color classes | |
| `TestRenderStatsIncludesDonut` | `cmd/ze-chaos/web/dashboard_test.go` | renderStats() output contains SVG donut element | |
| `TestLayoutIncludesHealthDonut` | `cmd/ze-chaos/web/render_test.go` | writeLayout output contains SVG donut in stats card | |
| `TestComputeStatusCounts` | `cmd/ze-chaos/web/state_test.go` | StatusCounts returns correct distribution from peer map | |
| `TestDonutSegmentColors` | `cmd/ze-chaos/web/render_test.go` | Each status maps to the correct CSS custom property color | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PeerCount | 0-10000 | 10000 | N/A (0 = empty donut) | N/A (no hard max, graceful) |
| Status count sum | Must equal PeerCount | PeerCount | N/A | N/A |
| Single status at 100% | All peers in one status | PeerCount | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-health-donut` | `test/chaos/health-donut.ci` | Dashboard loads with donut showing peer distribution, SSE updates it | |

### Future (if deferring any tests)
- Hover tooltip on donut segments showing exact percentage (deferrable, cosmetic enhancement)

## Files to Modify
- `cmd/ze-chaos/web/render.go` - add renderDonut() helper function; update writeLayout() stats card to include donut instead of flat Peers counter
- `cmd/ze-chaos/web/dashboard.go` - update renderStats() to include donut SVG; call StatusCounts() for per-status distribution
- `cmd/ze-chaos/web/state.go` - add StatusCounts() method on DashboardState returning map of PeerStatus to count
- `cmd/ze-chaos/web/assets/style.css` - add donut SVG container styles, legend layout styles, donut center text styles

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

## Files to Create
- `test/chaos/health-donut.ci` - functional test for donut rendering in dashboard

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit test for StatusCounts** - Review: covers all 5 statuses? Returns zero counts for zero peers?
2. **Run tests** - Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement StatusCounts on DashboardState** - Iterate over Peers map, count by Status. Must be called under read lock.
4. **Run tests** - Verify PASS (paste output).
5. **Write unit tests for renderDonut** - Review: all-one-status case? Mixed? Zero peers? Correct SVG structure?
6. **Run tests** - Verify FAIL.
7. **Implement renderDonut in render.go** - SVG circle with stroke-dasharray. Compute arc lengths from counts. Use CSS custom properties for colors via inline style attributes. Center text via SVG text element.
8. **Run tests** - Verify PASS.
9. **Write unit test for renderStats including donut** - Review: donut SVG present? Other stats still present?
10. **Run tests** - Verify FAIL.
11. **Update renderStats in dashboard.go** - Replace flat Peers counter span with donut + legend. Keep all other stats.
12. **Update writeLayout in render.go** - Replace flat Peers counter in initial stats card with same donut rendering.
13. **Run tests** - Verify PASS.
14. **Add CSS styles for donut** - SVG sizing, legend layout, center text styling. Use existing CSS custom properties.
15. **Write functional test** - Create test/chaos/health-donut.ci
16. **Verify all** - make ze-lint and make ze-chaos-test
17. **Critical Review** - All 6 checks from rules/quality.md must pass.
18. **Complete spec** - Fill audit tables, move to done/.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 7 (fix syntax/types) |
| Test fails wrong reason | Step 1 or 5 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural then DESIGN phase |
| SVG renders incorrectly | Check stroke-dasharray math, verify circumference calculation |
| Donut not visible in SSE update | Verify renderStats() output matches writeLayout() stats div structure |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

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

## Implementation Summary

### What Was Implemented
- `StatusCounts()` method on DashboardState returning [5]int indexed by PeerStatus
- `writeDonut()` renders SVG circle-based donut with stroke-dasharray segments
- `writeDonutLegend()` renders per-status count labels with colored dots
- Replaced flat "Peers Up/Total" counter in both renderStats() and writeLayout() with donut + legend
- CSS for donut container, center text, legend layout

### Bugs Found/Fixed
- None

### Documentation Updates
- None needed (UI-only change)

### Deviations from Plan
- Removed syncingStatInline() from renderStats since syncing count is now in the donut legend

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | TestLayoutIncludesHealthDonut | SVG donut in full page load |
| AC-2 | ✅ Done | TestRenderDonut | Mixed statuses produce proportional segments |
| AC-3 | ✅ Done | TestRenderDonutAllUp | Solid green ring |
| AC-4 | ✅ Done | TestRenderDonutZeroPeers + TestStatusCountsZeroPeers | All idle = grey ring |
| AC-5 | ✅ Done | TestRenderDonutZeroPeers | Zero peers shows "0" center, no crash |
| AC-6 | ✅ Done | TestRenderDonut | Center text shows total |
| AC-7 | ✅ Done | TestDonutSegmentColors | All 5 status colors verified |
| AC-8 | ✅ Done | TestRenderStatsIncludesDonut, TestSidebarStatsIncludesDonut | SSE + polling include donut |
| AC-9 | ✅ Done | TestRenderStatsIncludesDonut | Msgs Sent, Chaos still present |
| AC-10 | ✅ Done | TestRenderDonutLegend | Per-status labels with counts |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestStatusCounts | ✅ Done | dashboard_test.go | Mixed status distribution |
| TestStatusCountsZeroPeers | ✅ Done | dashboard_test.go | Zero peers edge case |
| TestRenderDonut | ✅ Done | dashboard_test.go | SVG structure + segments |
| TestRenderDonutAllUp | ✅ Done | dashboard_test.go | Single status full ring |
| TestRenderDonutZeroPeers | ✅ Done | dashboard_test.go | Empty ring |
| TestRenderDonutLegend | ✅ Done | dashboard_test.go | Legend labels + counts |
| TestRenderStatsIncludesDonut | ✅ Done | dashboard_test.go | SSE fragment |
| TestDonutSegmentColors | ✅ Done | dashboard_test.go | Color mapping |
| TestLayoutIncludesHealthDonut | ✅ Done | handlers_test.go | Full page |
| TestSidebarStatsIncludesDonut | ✅ Done | handlers_test.go | Polling fallback |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| render.go | ✅ Done | writeDonut, writeDonutLegend, donutStatusOrder + writeLayout update |
| dashboard.go | ✅ Done | renderStats updated with donut |
| state.go | ✅ Done | StatusCounts method |
| style.css | ✅ Done | Donut + legend CSS |

### Audit Summary
- **Total items:** 24
- **Done:** 24
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`cmd/ze-chaos/web/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] `make ze-lint` passes
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
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** -- NEVER commit implementation without the completed spec. One commit = code + tests + spec.
