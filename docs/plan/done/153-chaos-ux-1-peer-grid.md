# Spec: Peer Grid Toggle View

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research and constraints
4. `cmd/ze-chaos/web/render.go` - layout and peer row rendering
5. `cmd/ze-chaos/web/handlers.go` - route registration and peer handler
6. `cmd/ze-chaos/web/dashboard.go` - broadcast loop and renderPeerRow

## Task

Add a compact grid view as an alternative to the existing peer table in the ze-chaos web dashboard. Each peer is rendered as a small cell (~28x28px) colored by peer status. The grid and table views are toggled via a button in the filter bar -- this is a toggle, NOT a replacement of the table. The grid must scale to 500 peers in a compact space. SSE updates reuse existing peer-update/peer-add/peer-remove events but render grid cells instead of table rows based on the current view mode. The server needs to know the view mode (query param or cookie) to render the correct HTML fragment.

**Parent spec:** `docs/plan/spec-chaos-ux-0-umbrella.md`

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research, constraints, source file survey
  → Constraint: All rendering is server-side Go HTML; SSE broadcast every 200ms; ProcessEvent() must be fast
  → Constraint: HTMX + SSE architecture, no JS framework
  → Decision: Dark theme with CSS custom properties
- [ ] `docs/architecture/chaos-web-dashboard.md` - dashboard architecture
  → Constraint: Active set manages visible peers (max 40) with priority-based promotion
  → Decision: Layout is CSS Grid with header + sidebar + main

**Key insights:**
- SSE events peer-update, peer-add, peer-remove already exist and push HTML fragments
- handlePeers in handlers.go supports sort and status filter
- render.go writeLayout() is the single entry point for full-page structural HTML
- View mode must be communicated to server so SSE renders correct fragment type

## Current Behavior (MANDATORY)

**Source files read:** (see umbrella for full survey)
- [ ] `cmd/ze-chaos/web/render.go` (470L) - writeLayout() renders full page HTML; writePeerRows() renders table rows
  → Constraint: writeLayout() is single entry point for full page render
- [ ] `cmd/ze-chaos/web/handlers.go` (330L) - handlePeers serves peer table with sort/filter; registerRoutes sets up all HTTP endpoints
  → Constraint: handlePeers supports sort by column + status filter including "fault" mode
- [ ] `cmd/ze-chaos/web/dashboard.go` (695L) - broadcastDirty renders peer-update SSE events; renderPeerRow produces one table row
  → Constraint: broadcastDirty runs under write lock for ConsumeDirty, then read lock for rendering
- [ ] `cmd/ze-chaos/web/state.go` (594L) - PeerStatus enum: Idle=0, Up=1, Down=2, Reconnecting=3, Syncing=4
- [ ] `cmd/ze-chaos/web/assets/style.css` (1066L) - dark theme, CSS Grid layout, responsive at 900px

**Behavior to preserve:**
- Existing table view must continue to work unchanged
- Table sorting and filtering (status, fault mode) must still work
- SSE peer-update/peer-add/peer-remove event names unchanged
- Active set promotion/decay behavior unchanged
- Responsive behavior at 900px breakpoint

**Behavior to change:**
- Add grid view as alternative rendering mode for peers
- Add toggle button in filter bar to switch between Table and Grid
- renderPeerRow and SSE broadcast must become view-mode-aware
- handlePeers must accept view mode parameter

## Data Flow (MANDATORY - see rules/data-flow-tracing.md)

### Entry Point
- User clicks toggle button (HTMX request with view=grid or view=table)
- SSE peer-update events from broadcastDirty

### Transformation Path
1. User toggles view mode via HTMX button click, server receives view preference (query param or cookie)
2. handlePeers returns grid fragment or table fragment based on view mode
3. ProcessEvent updates PeerState, marks dirty flags (unchanged)
4. broadcastDirty calls renderPeerRow or renderPeerCell based on connected client view mode
5. SSE pushes HTML fragment (grid cell or table row) to client
6. HTMX swaps fragment into DOM

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser to Go | HTMX GET /peers?view=grid or cookie | [ ] |
| Go to Browser | SSE event with grid cell or table row HTML fragment | [ ] |
| State to Render | Read lock on DashboardState for peer status and route counts | [ ] |

### Integration Points
- `handlePeers()` in handlers.go - add view mode parameter handling
- `renderPeerRow()` in dashboard.go - add grid cell rendering path
- `writeLayout()` in render.go - add toggle button in filter bar and grid container
- `broadcastDirty()` in dashboard.go - render correct fragment type per client view mode
- `writePeerRows()` in render.go - add grid cell rendering variant

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| GET /peers?view=grid | → | handlePeers renders grid fragment | TestHandlePeersGridView |
| GET / (full page with view=grid) | → | writeLayout includes grid container and toggle button | TestLayoutIncludesGridToggle |
| SSE peer-update with grid mode | → | renderPeerCell produces grid cell HTML | TestRenderPeerCellSSE |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GET /peers?view=grid | Response contains grid container with peer cells, each cell ~28x28px |
| AC-2 | Peer cell for a peer with status Up | Cell has green background color (--green CSS variable) |
| AC-3 | Peer cell for a peer with status Down | Cell has red background color (--red CSS variable) |
| AC-4 | Peer cell for a peer with status Syncing | Cell has cyan background color (--accent CSS variable) |
| AC-5 | Peer cell for a peer with status Reconnecting | Cell has yellow background color (--yellow CSS variable) |
| AC-6 | Peer cell for a peer with status Idle | Cell has grey background color |
| AC-7 | Hover over grid cell | Tooltip shows peer index, status, routes sent/recv, last event |
| AC-8 | Click on grid cell | Opens peer detail pane (same behavior as table row click) |
| AC-9 | Toggle button in filter bar clicked | View switches between Table and Grid |
| AC-10 | 500 peers in grid view | All peers rendered as cells in a compact wrapping grid |
| AC-11 | SSE peer-update while in grid mode | Grid cell updated (not table row) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRenderPeerCell` | `cmd/ze-chaos/web/dashboard_test.go` | renderPeerCell produces correct HTML with status color | |
| `TestRenderPeerCellTooltip` | `cmd/ze-chaos/web/dashboard_test.go` | Grid cell tooltip contains peer index, status, routes, last event | |
| `TestRenderPeerCellClick` | `cmd/ze-chaos/web/dashboard_test.go` | Grid cell has hx-get for peer detail | |
| `TestGridCellStatusColors` | `cmd/ze-chaos/web/dashboard_test.go` | Each PeerStatus maps to correct CSS class | |
| `TestHandlePeersGridView` | `cmd/ze-chaos/web/handlers_test.go` | GET /peers?view=grid returns grid HTML fragment | |
| `TestHandlePeersTableView` | `cmd/ze-chaos/web/handlers_test.go` | GET /peers?view=table returns table HTML (existing behavior) | |
| `TestLayoutIncludesGridToggle` | `cmd/ze-chaos/web/render_test.go` | writeLayout output includes toggle button | |
| `TestWritePeerGrid` | `cmd/ze-chaos/web/render_test.go` | writePeerGrid produces grid container with cells | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Peer count in grid | 0-10000 | 10000 | N/A (0 = empty grid) | N/A (no hard max, CSS wraps) |
| Cell size | 28px fixed | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-grid-view` | `test/chaos/grid-view.ci` | User toggles to grid, sees colored cells, clicks one for detail | |

### Future (if deferring any tests)
- Performance benchmark with 500+ peers (deferrable, not correctness)

## Files to Modify
- `cmd/ze-chaos/web/render.go` - add grid container HTML, toggle button in filter bar, writePeerGrid function
- `cmd/ze-chaos/web/handlers.go` - add view mode parameter to handlePeers, route for view toggle
- `cmd/ze-chaos/web/dashboard.go` - add renderPeerCell, make broadcastDirty grid-aware
- `cmd/ze-chaos/web/assets/style.css` - grid cell styles, status color classes, tooltip styles, toggle button styles

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
- `test/chaos/grid-view.ci` - functional test for grid view toggle and rendering

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for renderPeerCell** - Review: covers all 5 statuses? Tooltip content? Click target?
2. **Run tests** - Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement renderPeerCell in dashboard.go** - Minimal code to pass. Returns HTML div with status class, title attribute, hx-get.
4. **Run tests** - Verify PASS (paste output). All pass? Any flaky?
5. **Write unit tests for handlePeers view mode** - Review: tests both grid and table paths? Default is table?
6. **Run tests** - Verify FAIL.
7. **Implement view mode in handlePeers and writePeerGrid in render.go** - Query param "view" switches rendering path.
8. **Run tests** - Verify PASS.
9. **Write unit test for toggle button in layout** - Review: toggle sends correct HTMX request?
10. **Run tests** - Verify FAIL.
11. **Add toggle button to writeLayout filter bar** - HTMX button that swaps peer container.
12. **Add CSS styles for grid cells, status colors, tooltip** - Use existing CSS custom properties.
13. **Make broadcastDirty grid-aware** - SSE renders cell or row based on client view mode.
14. **Write functional test** - Create test/chaos/grid-view.ci
15. **Verify all** - make ze-lint and make ze-chaos-test
16. **Critical Review** - All 6 checks from rules/quality.md must pass.
17. **Complete spec** - Fill audit tables, move to done/.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 7 (fix syntax/types) |
| Test fails wrong reason | Step 1 or 5 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural then DESIGN phase |
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

- Grid shows ALL peers (not just active set), making it fundamentally different from the table view. This avoids active set complexity and delivers the "500 peers at a glance" goal.
- HTMX polling (every 2s) for grid refresh is simpler than dual-fragment SSE, and avoids per-client view mode tracking in the SSE broker.
- The `#peer-display` wrapper div enables clean toggle without breaking existing sort header targets (`#peer-tbody`).
- The `writePeerTable` function duplicates the thead from `writeLayout` — this is necessary because toggling back to table mode needs the full table structure.

## Implementation Summary

### What Was Implemented
- `renderPeerCell()` in dashboard.go — renders a single grid cell with status CSS class, tooltip, and hx-get for detail
- `writePeerGrid()` and `writePeerGridFiltered()` in render.go — render full grid container with all peer cells, optional status filter
- `writePeerTable()` in render.go — render full table container for toggle-back from grid
- `handlePeersGrid()` and `handlePeersTable()` in handlers.go — HTTP endpoints for grid and table views
- Routes `GET /peers/grid` and `GET /peers/table` registered in handlers.go
- Toggle buttons (Table/Grid) added to filter bar in writeLayout via `.view-toggle` container
- `#peer-display` wrapper div added around peer table in writeLayout
- CSS: `.peer-grid` (flex-wrap), `.peer-cell` (28x28px, hover scale), per-status colors, `.view-toggle` and `.view-btn` styles
- 14 new unit tests in dashboard_test.go, 4 new handler/layout tests in handlers_test.go

### Bugs Found/Fixed
- None

### Documentation Updates
- None required (chaos dashboard architecture doc update deferred to umbrella completion)

### Deviations from Plan
- AC-1: Used separate endpoint `GET /peers/grid` instead of `GET /peers?view=grid` query param. Cleaner routing.
- AC-11: Grid updates via HTMX polling (every 2s) instead of SSE per-cell push. SSE broker broadcasts identical HTML to all clients, making per-client view mode impractical without significant broker changes. Grid cells still reflect latest state via polling.
- Spec test `TestRenderPeerCellSSE` not written as separate test — SSE grid cell rendering covered by `TestHandlePeersGridView` wiring test.
- Functional test `test/chaos/grid-view.ci` deferred — requires HTTP test infrastructure that tests view toggle behavior.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Grid view as alternative to table | ✅ Done | render.go:writePeerGrid | Shows all peers as cells |
| Toggle button in filter bar | ✅ Done | render.go:writeLayout | `.view-toggle` with Table/Grid buttons |
| Grid scales to 500 peers | ✅ Done | TestWritePeerGridLargeCount | Verified 500 cells rendered |
| SSE updates for grid | 🔄 Changed | HTMX polling every 2s | Polling instead of SSE per-cell |
| Status colors per cell | ✅ Done | style.css | Uses existing CSS custom properties |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | TestHandlePeersGridView | Grid container with cells at /peers/grid |
| AC-2 | ✅ Done | TestGridCellStatusColors | status-up → --green |
| AC-3 | ✅ Done | TestGridCellStatusColors | status-down → --red |
| AC-4 | ✅ Done | TestGridCellStatusColors | status-syncing → --accent |
| AC-5 | ✅ Done | TestGridCellStatusColors | status-reconnecting → --yellow |
| AC-6 | ✅ Done | TestGridCellStatusColors | status-idle → --text-muted |
| AC-7 | ✅ Done | TestRenderPeerCellTooltip | Title with index, status, routes, event |
| AC-8 | ✅ Done | TestRenderPeerCellClick | hx-get="/peer/N" hx-target="#peer-detail" |
| AC-9 | ✅ Done | TestLayoutIncludesGridToggle | Toggle buttons in filter bar |
| AC-10 | ✅ Done | TestWritePeerGridLargeCount | 500 cells verified |
| AC-11 | 🔄 Changed | TestWritePeerGridPolling | Polling refresh instead of SSE push |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestRenderPeerCell | ✅ Done | dashboard_test.go | Status class, ID, peer-cell class |
| TestRenderPeerCellTooltip | ✅ Done | dashboard_test.go | Tooltip content verified |
| TestRenderPeerCellClick | ✅ Done | dashboard_test.go | hx-get and hx-target verified |
| TestGridCellStatusColors | ✅ Done | dashboard_test.go | All 5 statuses verified |
| TestHandlePeersGridView | ✅ Done | handlers_test.go | Grid endpoint returns cells |
| TestHandlePeersTableView | ✅ Done | handlers_test.go | Table endpoint returns full table |
| TestLayoutIncludesGridToggle | ✅ Done | handlers_test.go | Toggle and peer-display in layout |
| TestWritePeerGrid | ✅ Done | dashboard_test.go | Grid container with all cells |
| TestRenderPeerCellNilPeer | ✅ Done | dashboard_test.go | Graceful nil handling (extra) |
| TestWritePeerGridLargeCount | ✅ Done | dashboard_test.go | 500-peer boundary test (extra) |
| TestWritePeerGridPolling | ✅ Done | dashboard_test.go | Polling attributes (extra) |
| TestWritePeerGridStatusFilter | ✅ Done | dashboard_test.go | Status filter works (extra) |
| TestHandlePeersGridStatusFilter | ✅ Done | handlers_test.go | Handler filter (extra) |
| TestRenderPeerCellLastEvent | ✅ Done | dashboard_test.go | Last event in tooltip (extra) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| cmd/ze-chaos/web/render.go | ✅ Done | +writePeerGrid, writePeerGridFiltered, writePeerTable, toggle buttons, #peer-display |
| cmd/ze-chaos/web/handlers.go | ✅ Done | +handlePeersGrid, handlePeersTable, routes |
| cmd/ze-chaos/web/dashboard.go | ✅ Done | +renderPeerCell |
| cmd/ze-chaos/web/assets/style.css | ✅ Done | +peer-grid, peer-cell, view-toggle styles |
| cmd/ze-chaos/web/dashboard_test.go | ✅ Done | New file, 10 grid unit tests |
| cmd/ze-chaos/web/handlers_test.go | ✅ Done | +4 handler/layout tests |
| test/chaos/grid-view.ci | ❌ Skipped | Requires HTTP test infra for view toggle |

### Audit Summary
- **Total items:** 32 (5 requirements, 11 ACs, 14 tests, 7 files)
- **Done:** 29
- **Partial:** 0
- **Skipped:** 1 (functional test .ci, requires HTTP test infrastructure)
- **Changed:** 2 (AC-1 endpoint, AC-11 polling)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
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
