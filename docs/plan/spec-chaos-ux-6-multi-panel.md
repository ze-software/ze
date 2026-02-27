# Spec: Multi-Panel Viz Layout

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research and constraints
4. `cmd/ze-chaos/web/viz.go` - existing 7 viz tab handlers and renderers
5. `cmd/ze-chaos/web/render.go` - writeLayout tab-bar and viz-content area
6. `cmd/ze-chaos/web/handlers.go` - route registration for viz endpoints
7. `cmd/ze-chaos/web/assets/style.css` - tab-bar and viz-panel styles

## Task

Allow 2-4 viz panels to be displayed simultaneously in a grid layout. Currently the viz area shows one tab at a time; multi-panel mode shows multiple panels side by side. Panel selection is done via checkboxes or toggle buttons that let users pick which viz tabs appear as panels. CSS Grid handles panel arrangement: 2x2 default for 4 panels, 1x2 for 2 panels, full-width for 1 panel. Each panel gets its own HTMX polling endpoint so panels update independently. This is additive to the existing single-tab mode -- users toggle between "Tabs" (existing) and "Panels" (new multi-panel grid) via a mode switch in the tab bar area. Each existing viz tab becomes a panel option. No new RPCs are needed.

**Parent spec:** `docs/plan/spec-chaos-ux-0-umbrella.md`

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research, constraints, source file survey
  → Constraint: All rendering is server-side Go HTML; SSE broadcast every 200ms; ProcessEvent() must be fast
  → Constraint: viz.go is 1349 lines -- new viz features MUST go in separate files
  → Constraint: HTMX + SSE architecture, no JS framework
  → Decision: Each viz renders a full panel div with HTMX polling attributes for updates
- [ ] `docs/architecture/chaos-web-dashboard.md` - dashboard architecture
  → Constraint: writeLayout() is the single entry point for full-page structural HTML
  → Decision: Tab bar renders buttons that swap content into #viz-content via hx-get

**Key insights:**
- The tab bar in writeLayout() renders 7 buttons (Families, Convergence, Route Matrix, Timeline, Events, All Peers, + 2 Chaos tabs) that each target #viz-content
- Each viz handler (handleVizEvents, handleVizConvergence, etc.) returns a self-contained panel div with its own hx-trigger for polling
- Polling attributes like hx-trigger="every 500ms [!window._frozen]" are already per-panel in the viz renderers
- Panel mode needs unique swap target IDs per panel slot (viz-panel-0 through viz-panel-3) so updates do not interfere
- Existing viz render functions can be reused directly -- the panel wrapper just changes the surrounding container and HTMX target
- SSE events like "convergence" target specific DOM IDs; in panel mode each panel's content needs its own target ID
- The tab bar already has a "Freeze" toggle at the end; the Tabs/Panels mode switch can go next to it

## Current Behavior (MANDATORY)

**Source files read:** (see umbrella for full survey)
- [ ] `cmd/ze-chaos/web/viz.go` (1349L) - 7 viz tab handlers: handleVizEvents, handleVizConvergence, handleVizPeerTimeline, handleVizChaosEvents, handleVizChaosTimeline, handleVizRouteMatrix, handleVizFamilies, handleVizAllPeers; each calls a write* function that produces a self-contained panel div with HTMX polling
  → Constraint: Already over 1000-line threshold -- new viz features MUST go in separate files
  → Decision: Each write* function (writeEventStream, writeConvergenceHistogram, writePeerTimeline, etc.) takes an io.Writer and state, produces complete panel HTML
- [ ] `cmd/ze-chaos/web/render.go` (470L) - writeLayout renders tab-bar div with buttons and a single div id="viz-content" that receives swapped content; tab buttons use hx-get="/viz/{name}" hx-target="#viz-content" hx-swap="innerHTML"
  → Constraint: writeLayout is single entry point for full page render
  → Decision: Tab bar uses onclick to manage active class (client-side class toggle)
- [ ] `cmd/ze-chaos/web/handlers.go` (330L) - registerRoutes maps GET /viz/{name} to handler methods; 8 viz routes total plus /viz/route-matrix/cell
  → Constraint: Existing viz routes serve both tab mode and will serve panel mode (with panel query param)
- [ ] `cmd/ze-chaos/web/assets/style.css` (1066L) - .tab-bar flex layout with buttons; .viz-panel base styling for panel divs; #viz-content is the swap target
  → Constraint: CSS custom properties for theming; responsive at 900px breakpoint

**Behavior to preserve:**
- All 7 existing viz tab handlers and their render output unchanged
- Tab bar buttons and their hx-get swap behavior in single-tab mode
- SSE event swap targets for existing viz tabs (convergence SSE event)
- HTMX polling intervals per viz tab
- Freeze toggle functionality
- Responsive behavior at 900px breakpoint

**Behavior to change:**
- Add "Panels" toggle button next to "Freeze" toggle in tab bar area
- When panel mode is active, #viz-content is replaced with a CSS Grid container holding 2-4 panel slots
- Each panel slot contains a dropdown to select which viz to display and a content area with a unique target ID
- Each panel's content area independently polls its selected viz endpoint
- Panel selection state is passed as query params (panel=N) so the server renders panel-specific HTMX targets
- Default 4-panel layout shows Families, Convergence, Timeline, Events

## Data Flow (MANDATORY - see rules/data-flow-tracing.md)

### Entry Point
- User clicks "Panels" toggle in the tab bar area
- HTMX request fetches panel grid layout and inserts it into #viz-content
- Each panel's dropdown change triggers a per-panel content fetch

### Transformation Path
1. User clicks "Panels" mode toggle; HTMX GET /viz/panels fetches the multi-panel grid HTML
2. Response contains CSS Grid container with N panel slots (default 4), each with a dropdown and a content div
3. Each panel's dropdown sends hx-get="/viz/{selected}?panel=N" targeting that panel's unique content div (viz-panel-content-N)
4. Existing viz render functions are called with a panel-specific wrapper that adjusts the HTMX target ID
5. Each panel's content has hx-trigger="every Xs" for independent polling, also targeting its unique content div
6. User switches back to "Tabs" mode via the toggle; HTMX swaps #viz-content back to empty (next tab click loads single viz)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser to Go | HTMX GET /viz/panels for grid layout; GET /viz/{name}?panel=N for individual panel content | [ ] |
| Go to Browser | HTML fragment: CSS Grid with panel slots, or individual panel content | [ ] |
| Panel to Viz | Each panel reuses existing viz render functions with panel-specific HTMX target wrapper | [ ] |

### Integration Points
- `writeLayout()` in render.go - add Tabs/Panels mode toggle button near freeze toggle
- `registerRoutes()` in handlers.go - add GET /viz/panels route for panel grid layout
- Existing viz handlers in viz.go - add panel query param support to adjust HTMX target IDs
- New file viz_panels.go - panel grid renderer, per-panel content wrapper, panel dropdown
- style.css - CSS Grid rules for panel layout (2x2, 1x2, full-width)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| GET /viz/panels | → | handleVizPanels returns CSS Grid with 4 panel slots | TestHandleVizPanels |
| GET /viz/convergence?panel=1 | → | handleVizConvergence returns panel-wrapped content with target viz-panel-content-1 | TestHandleVizPanelContent |
| GET / full page load | → | writeLayout includes Tabs/Panels mode toggle | TestLayoutIncludesPanelToggle |
| Panel dropdown selects "events" | → | hx-get="/viz/events?panel=2" swaps content into viz-panel-content-2 | TestPanelDropdownSwap |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Click "Panels" toggle in tab bar area | viz-content replaced with CSS Grid containing 4 panel slots, each with a dropdown and content area |
| AC-2 | Panel dropdown changed to "Convergence" in panel slot 1 | Panel 1 content area shows convergence histogram; other panels unchanged |
| AC-3 | 4 panels active, wait for polling interval | Each panel updates independently via its own HTMX polling (no interference between panels) |
| AC-4 | Click "Tabs" toggle while in panel mode | View switches back to single-tab mode; existing tab behavior restored |
| AC-5 | 2 panels selected (2 dropdowns set, 2 empty) | Layout adapts: 1x2 for 2 panels, full-width for 1 |
| AC-6 | 4 panels selected | Layout is 2x2 CSS Grid |
| AC-7 | Browser width below 900px in panel mode | Panels stack to single column |
| AC-8 | Default panel mode load | 4 default panels shown: Families, Convergence, Timeline, Events |
| AC-9 | Freeze toggle active in panel mode | All panels stop polling (existing freeze mechanism applies) |
| AC-10 | GET /viz/panels | Response contains 4 panel divs with unique IDs (viz-panel-0 through viz-panel-3) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWritePanelGrid` | `cmd/ze-chaos/web/viz_panels_test.go` | writePanelGrid produces CSS Grid container with 4 panel slots, each having a dropdown and content div | |
| `TestWritePanelGridDefaultSelections` | `cmd/ze-chaos/web/viz_panels_test.go` | Default panel selections are Families, Convergence, Timeline, Events | |
| `TestWritePanelSlot` | `cmd/ze-chaos/web/viz_panels_test.go` | Individual panel slot has unique ID (viz-panel-N), dropdown with all viz options, content div (viz-panel-content-N) | |
| `TestPanelDropdownOptions` | `cmd/ze-chaos/web/viz_panels_test.go` | Dropdown lists all 7 viz tab names as options | |
| `TestPanelContentWrapper` | `cmd/ze-chaos/web/viz_panels_test.go` | Viz content wrapped with panel-specific HTMX target (hx-target="#viz-panel-content-N") and polling | |
| `TestHandleVizPanels` | `cmd/ze-chaos/web/viz_panels_test.go` | GET /viz/panels handler returns grid HTML with 4 panel slots | |
| `TestHandleVizPanelContent` | `cmd/ze-chaos/web/viz_panels_test.go` | GET /viz/convergence?panel=1 returns convergence content with panel-specific swap target | |
| `TestLayoutIncludesPanelToggle` | `cmd/ze-chaos/web/render_test.go` | writeLayout output includes Tabs/Panels mode toggle in tab bar area | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| panel query param | 0-3 | 3 | N/A (defaults to tab mode if absent) | 4 (ignored, treated as no panel) |
| Number of active panels | 1-4 | 4 | 0 (show empty grid or revert to tabs) | N/A (max 4 slots) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-panel-layout` | `test/chaos/panel-layout.ci` | Load dashboard, switch to panels mode, verify 4 panels rendered, switch panel dropdown, verify content changes | |

### Future (if deferring any tests)
- Panel resize/drag: not in scope, deferred to potential future spec
- Custom grid dimensions beyond 2x2: not in scope
- Saving panel layout preferences (cookie/localStorage): not in scope

## Files to Modify
- `cmd/ze-chaos/web/render.go` - add Tabs/Panels mode toggle button in tab bar area (next to freeze toggle)
- `cmd/ze-chaos/web/handlers.go` - register GET /viz/panels route; add panel query param handling to existing viz handlers (adjust HTMX target IDs when panel=N is present)
- `cmd/ze-chaos/web/assets/style.css` - CSS Grid rules for .panel-grid (grid-template-columns: 1fr 1fr, gap, responsive single-column at 900px); panel slot styling; dropdown styling within panels

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
- `cmd/ze-chaos/web/viz_panels.go` - handleVizPanels handler, writePanelGrid function, writePanelSlot function, panelContentWrapper function, vizTabNames list
- `cmd/ze-chaos/web/viz_panels_test.go` - unit tests for panel rendering
- `test/chaos/panel-layout.ci` - functional test for multi-panel mode

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for writePanelGrid and writePanelSlot** - Review: tests cover 4 panel slots, unique IDs, dropdown options, default selections?
2. **Run tests** - Verify FAIL (paste output). Fail for RIGHT reason (missing functions)?
3. **Create viz_panels.go** - Implement writePanelGrid (CSS Grid container with 4 panel slots), writePanelSlot (dropdown + content div with unique ID), panelContentWrapper (wraps existing viz output with panel-specific HTMX target), vizTabNames (list of 7 viz tab names and their endpoints). Add // Design: reference.
4. **Run tests** - Verify PASS (paste output). All pass? Any flaky?
5. **Write unit test for handleVizPanels** - Review: returns grid HTML? Handler wired correctly?
6. **Run tests** - Verify FAIL.
7. **Implement handleVizPanels in viz_panels.go** - Handler that renders the full panel grid.
8. **Register GET /viz/panels route in handlers.go** - Add route in registerRoutes.
9. **Add panel query param handling to existing viz handlers** - When panel=N is present, wrap response with panel-specific HTMX target. This can be done in a helper that checks the param.
10. **Run tests** - Verify PASS.
11. **Add Tabs/Panels mode toggle to writeLayout in render.go** - Near freeze toggle in tab bar area.
12. **Add CSS Grid styles to style.css** - .panel-grid: display grid, grid-template-columns 1fr 1fr, gap 12px. Panel slot styling. Responsive: single column at 900px.
13. **Write functional test** - Create test/chaos/panel-layout.ci.
14. **Verify all** - make ze-lint and make ze-chaos-test.
15. **Critical Review** - All 6 checks from rules/quality.md must pass.
16. **Complete spec** - Fill audit tables, move to done/.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 (fix syntax/types in viz_panels.go) |
| Test fails wrong reason | Step 1 or 5 (fix test expectations) |
| Test fails behavior mismatch | Re-read render.go tab bar and viz.go handler pattern |
| Lint failure | Fix inline; if architectural, revisit panel design |
| Functional test fails | Check AC; if AC wrong, revisit design; if AC correct, fix implementation |
| Audit finds missing AC | Back to implementation for that criterion |
| Panel polling conflicts | Each panel must have unique target ID; verify no ID collisions |
| Existing tab tests break | Tab mode must remain default; panel mode is additive |

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
- (pending)

### Bugs Found/Fixed
- (pending)

### Documentation Updates
- (pending)

### Deviations from Plan
- (pending)

## Implementation Audit

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
