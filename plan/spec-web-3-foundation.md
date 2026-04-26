# Spec: web-3-foundation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-web-2-operator-workbench |
| Phase | 7/7 |
| Updated | 2026-04-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/web-interface.md` - URL routing, content negotiation
4. `docs/architecture/web-components.md` - template structure, fragment rendering
5. `plan/design/web-interface-design.md` - full UI design (navigation structure, page designs, common patterns)
6. `internal/component/web/workbench_sections.go` - current flat nav model
7. `internal/component/web/render.go` - WorkbenchData, WorkbenchSection structs
8. `internal/component/web/handler_workbench.go` - workbench handler
9. `internal/component/web/templates/component/workbench_nav.html` - nav template
10. `internal/component/web/templates/page/workbench.html` - page shell template
11. `internal/component/web/assets/style.css` - all CSS (lines 2250-2487: workbench section)

## Task

Build the foundation components that every subsequent web page will reuse. The current workbench (from spec-web-2) has a hollow shell: a flat left nav with 9 links that all render the same Finder detail fragment. This spec replaces that hollow shell with working building blocks.

Six deliverables:

1. **Two-level left navigation** with expand/collapse sections and sub-page links, matching the design doc's "Left Navigation (Sidebar)" structure.

2. **Reusable table component** (`workbench_table.html`) with standard toolbar (Add, filter, search), sortable column headers, flag column with color coding, action buttons per row, and an empty-state pattern.

3. **Reusable detail panel component** (`workbench_detail.html`) that opens when a table row is clicked: right-side drawer or below-row expansion, tabs (Config/Status/Actions), close button, related tools area.

4. **Reusable form component** (`workbench_form.html`) for singleton configuration pages with standard field types (text, number, dropdown, toggle, IP address, list add/remove), Save/Discard buttons.

5. **Updated dashboard overview page** showing real system information: System panel (hostname, uptime, version), BGP Summary (peer counts by state), Interfaces (interface counts by state), Active Warnings, Recent Errors.

6. **CSS for all workbench components** in style.css.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/web-interface.md` - URL routing, content negotiation, handler structure
  -> Decision: URLs follow verb-first three-tier scheme: /show/, /monitor/, /config/, /admin/
  -> Constraint: Both UIs (Finder and Workbench) share the same OOB swap path for HTMX requests
- [ ] `docs/architecture/web-components.md` - template rendering, fragment composition, decorator registry
  -> Decision: Templates are embedded via go:embed; fragments render via RenderFragment()
  -> Constraint: Component templates go in templates/component/; page templates in templates/page/
- [ ] `plan/design/web-interface-design.md` - full UI design blueprint
  -> Decision: Tables are the primary view; every table follows the same pattern (toolbar, flag column, actions, empty state)
  -> Decision: Navigation follows network stack order: Interfaces, IP, Routing, Policy, Firewall, L2TP, Services, System, Tools, Logs
  -> Constraint: Every page must be useful; if empty, show column headers + "No {items} configured" + Add button
  -> Constraint: Detail panel has tabs (Config/Status/Actions) and opens as right-side drawer or below-row expansion
  -> Constraint: Form pattern for singletons: field types + Save/Discard buttons

### RFC Summaries (MUST for protocol work)
(Not applicable: this is UI component work, no protocol wire formats.)

**Key insights:**
- The workbench shell (grid layout, topbar, nav, workspace, commit bar, CLI bar) is already built and tested (spec-web-2). This spec adds the inner components that populate the workspace area.
- The existing `WorkbenchSection` struct is flat: `{Key, Label, URL, Selected}`. It must gain a Children field for two-level navigation.
- The existing `list_table.html` template renders YANG-backed list tables with inline editing. The new `workbench_table.html` is a higher-level component that wraps a styled table with toolbar, empty state, and color coding, independent of YANG schema.
- The workbench handler already runs through the shared fragment data builder (`buildFragmentData`), then enrichment (`enrichWorkbenchTable`). New components plug into this pipeline or add new handler functions.
- `ze.web.ui=finder` must continue to work; Finder never sees the new components.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/component/web/workbench_sections.go` - flat list of 9 WorkbenchSection entries with per-path selection. `WorkbenchSections(currentPath)` returns `[]WorkbenchSection`.
  -> Constraint: WorkbenchSection struct currently has Key, Label, URL, Selected only; no Children
  -> Constraint: Selection logic uses switch on first path segment; adding sub-pages requires matching on deeper segments
- [ ] `internal/component/web/render.go` - WorkbenchData embeds LayoutData and adds `Sections []WorkbenchSection`. Renderer parses templates including `workbench_nav.html` and `workbench_topbar.html`.
  -> Constraint: NewRenderer() parses all component templates via `templates/component/*.html` glob; new templates added to that directory are automatically included in the fragment template set
  -> Constraint: WorkbenchData.Sections is the only data the nav template receives; extending the struct affects all callers
- [ ] `internal/component/web/handler_workbench.go` - HandleWorkbench builds FragmentData, enriches it, renders the workbench page. HTMX partial requests return OOB fragment. Full-page renders the workspace from the detail fragment.
  -> Constraint: Dashboard (root path, empty path) currently renders via `buildFragmentData(schema, viewTree, nil)` which returns just root-level YANG children; no system info
- [ ] `internal/component/web/fragment.go` - FragmentData struct and buildFragmentData(). ListTableView, FieldMeta, etc. ChildEntry is the tile data for YANG containers.
  -> Constraint: buildFragmentData at root (empty path) returns only Sidebar, Columns, and Breadcrumbs; no Fields or ListTable
- [ ] `internal/component/web/templates/component/workbench_nav.html` - iterates Sections as flat `<ul>` with `<li>` per section; no nesting
  -> Constraint: Must be replaced or extended to support two-level structure
- [ ] `internal/component/web/templates/page/workbench.html` - page shell: topbar, nav, workspace, notifications, commit bar, cli bar. Content goes into `workbench-workspace`.
  -> Constraint: The shell structure is stable from spec-web-2; this spec only adds content inside the workspace area and restructures the nav
- [ ] `internal/component/web/templates/component/list_table.html` - YANG-aware list table with inline editing, rename, delete, row tools, table tools, pending-change markers
  -> Constraint: This template serves both Finder and Workbench; do not break it. The new workbench_table component is separate.
- [ ] `internal/component/web/assets/style.css` - 2487 lines; lines 2250-2487 are workbench CSS. Uses CSS variables for theming.
  -> Constraint: Append new CSS to the workbench section; do not reorganize existing CSS
- [ ] `internal/component/web/ui_mode.go` - UIMode selector reads `ze.web.ui` env var; default is Finder, "workbench" is opt-in
  -> Constraint: Finder default must be preserved; all new components are workbench-only
- [ ] `internal/component/web/workbench_enrich.go` - enrichWorkbenchTable() adds row/table tools and pending markers to ListTableView. Only runs in workbench mode.
  -> Constraint: Enrichment only affects lists at their list-level path (not entry-level); do not extend to non-list views
- [ ] `cmd/ze/hub/main.go` (lines 1060-1078) - Hub wires up both handlers; showHandler switches on UIMode. Fragment handler serves /fragment/detail regardless of mode.
  -> Constraint: New routes for dashboard data or component endpoints must be registered in the hub, behind auth middleware

**Behavior to preserve:**
- Finder UI renders unchanged when `ze.web.ui=finder` (or unset)
- HTMX partial requests (HX-Request header) continue to return OOB fragments
- CLI bar, commit bar, diff/discard workflow in the workbench shell
- Existing list_table.html behavior for YANG-backed tables
- Tool overlay container and overlay rendering
- Path validation (ValidatePathSegments) on all URL inputs
- Breadcrumb rendering in the workbench topbar
- All existing workbench tests pass without modification

**Behavior to change:**
- `WorkbenchSection` struct: add Children field for sub-pages
- `WorkbenchSections()` function: return two-level structure per design doc
- `workbench_nav.html` template: render two-level nav with expand/collapse
- Dashboard page (root path in workbench mode): show real system panels instead of YANG root children
- CSS: add styles for table component, detail panel, form component, two-level nav, dashboard panels

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- HTTP GET `/show/` (workbench mode) for dashboard overview
- HTTP GET `/show/<yang-path>/` (workbench mode) for page content with table/detail/form components
- All requests enter via `HandleWorkbench` in `handler_workbench.go`

### Transformation Path
1. **URL routing** in `handler_workbench.go`: `extractPath(r)` extracts YANG path segments
2. **Fragment data building** in `fragment.go`: `buildFragmentData(schema, viewTree, path)` assembles YANG-based data
3. **Workbench enrichment** in `workbench_enrich.go`: `enrichWorkbenchTable()` adds V2-specific row/table tools
4. **Dashboard data assembly** (NEW): when path is empty (dashboard), `buildDashboardData()` fetches system info, BGP summary, interface counts, warnings, errors
5. **Navigation model** (CHANGED): `WorkbenchSections(path)` returns two-level sections with children and expansion state
6. **Template rendering** in `render.go`: `RenderWorkbench(w, data)` renders the page shell; `RenderFragment("detail", data)` renders workspace content; new templates render table/detail/form components

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Handler -> Fragment builder | Direct function call: `buildFragmentData()` | [ ] |
| Handler -> Dashboard builder | Direct function call: `buildDashboardData()` (new) | [ ] |
| Handler -> Enrichment | Direct function call: `enrichWorkbenchTable()` | [ ] |
| Handler -> Renderer | `RenderWorkbench()` and `RenderFragment()` calls | [ ] |
| Template -> CSS | Class names in templates match CSS selectors in style.css | [ ] |
| Hub -> Handlers | `HandleWorkbench()` registered on mux behind auth middleware | [ ] |

### Integration Points
- `WorkbenchSections()` - called by `HandleWorkbench` to populate nav data; must return the new two-level structure
- `WorkbenchData.Sections` - consumed by `workbench_nav.html` template; struct change propagates to template
- `RenderFragment("detail", data)` - still used for workspace content; new table/detail/form components supplement this
- `enrichWorkbenchTable()` - continues to enrich list table views; no change to its contract
- Dashboard data sources: hub internal state (hostname, uptime, version, peer state counts, interface counts)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| GET /show/ (workbench mode) | -> | Two-level nav renders with sections and sub-pages | `test/web/workbench-two-level-nav.wb` |
| GET /show/ (workbench mode) | -> | Dashboard panels render system info | `test/web/workbench-dashboard-overview.wb` |
| GET /show/ (finder mode) | -> | Finder layout unchanged, no workbench nav | `test/web/workbench-rollback-finder.wb` (existing) |
| WorkbenchSections(path) | -> | Returns two-level sections with children | `TestWorkbenchSections_TwoLevel` |
| workbench_table.html | -> | Renders toolbar, headers, rows, empty state | `TestRenderWorkbenchTable` |
| workbench_detail.html | -> | Renders tabbed panel with close button | `TestRenderWorkbenchDetail` |
| workbench_form.html | -> | Renders field types with Save/Discard | `TestRenderWorkbenchForm` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GET /show/ in workbench mode | Left nav renders two-level structure: sections (Dashboard, Interfaces, IP, Routing, etc.) each with sub-pages |
| AC-2 | Click a section label in left nav | Section expands/collapses its sub-pages (CSS/JS toggle, no server round-trip) |
| AC-3 | Navigate to /show/bgp/peer/ | Active sub-page "Peers" is highlighted; parent section "Routing" is expanded |
| AC-4 | Render a workbench table with rows | Table shows toolbar (Add button, filter dropdown, search field), column headers, rows with flag column and action buttons |
| AC-5 | Render a workbench table with zero rows | Table shows column headers + centered "No {items} configured" message + Add button |
| AC-6 | Table row with status "down" | Flag column cell has red color coding; row with status "disabled" has grey; "healthy" has green |
| AC-7 | Click a table row | Detail panel opens (right-side drawer at wide viewports, or below-row expansion) |
| AC-8 | Detail panel is open | Panel shows tabs (Config / Status / Actions), close button, and related tools area |
| AC-9 | Render a singleton form page | Form shows standard field types (text, number, dropdown, toggle, IP address, list add/remove) with Save and Discard buttons |
| AC-10 | GET /show/ in workbench mode (dashboard) | Dashboard shows panels: System (hostname, uptime, version), BGP Summary (peer counts by state), Interfaces (counts by state), Active Warnings, Recent Errors; placeholder data acceptable where real data is not yet available |
| AC-11 | Dashboard with no BGP peers configured and no interfaces | Dashboard panels show empty-state hints with links to relevant pages (e.g., "No BGP peers configured" linking to Routing > BGP > Peers) |
| AC-12 | All workbench pages | Consistent CSS layout: same font, spacing, color scheme, border styles across table, detail, form, nav, dashboard components |
| AC-13 | Hub started with `ze.web.ui=finder` (or unset) | Finder UI renders unchanged; no workbench nav, table, detail, or form components appear |
| AC-14 | Workbench mode | CLI bar at bottom, commit bar with pending count, diff/discard buttons all render and function as in spec-web-2 |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWorkbenchSections_TwoLevel` | `internal/component/web/workbench_sections_test.go` | Two-level nav model returns sections with Children populated | |
| `TestWorkbenchSections_TwoLevel_ChildSelected` | `internal/component/web/workbench_sections_test.go` | When navigating to /show/bgp/peer/, "Peers" child is selected and "Routing" parent is expanded | |
| `TestWorkbenchSections_TwoLevel_SectionCollapse` | `internal/component/web/workbench_sections_test.go` | Sections other than the active one have Expanded=false | |
| `TestWorkbenchSections_TwoLevel_DashboardDefault` | `internal/component/web/workbench_sections_test.go` | Root path selects Dashboard section and Dashboard > Overview sub-page | |
| `TestWorkbenchSections_TwoLevel_AllSectionsPresent` | `internal/component/web/workbench_sections_test.go` | All design-doc sections exist: Dashboard, Interfaces, IP, Routing, Policy, Firewall, L2TP, Services, System, Tools, Logs | |
| `TestRenderWorkbenchNav_TwoLevel` | `internal/component/web/workbench_render_test.go` | Nav template outputs nested `<ul>` with section headings and sub-page links | |
| `TestRenderWorkbenchNav_ActiveHighlight` | `internal/component/web/workbench_render_test.go` | Active sub-page link has `workbench-nav-subitem--active` class | |
| `TestRenderWorkbenchNav_ExpandedSection` | `internal/component/web/workbench_render_test.go` | Active section has `workbench-nav-section--expanded` class | |
| `TestRenderWorkbenchTable` | `internal/component/web/workbench_table_test.go` | Table component renders toolbar, column headers, flag column, action buttons, rows | |
| `TestRenderWorkbenchTable_EmptyState` | `internal/component/web/workbench_table_test.go` | Empty table renders column headers + "No {items} configured" + Add button | |
| `TestRenderWorkbenchTable_FlagColors` | `internal/component/web/workbench_table_test.go` | Flag column cells carry correct CSS classes for red/grey/green status | |
| `TestRenderWorkbenchDetail` | `internal/component/web/workbench_detail_test.go` | Detail panel renders tabs (Config/Status/Actions), close button, related tools area | |
| `TestRenderWorkbenchDetail_TabSwitching` | `internal/component/web/workbench_detail_test.go` | Correct tab content is marked active; inactive tabs are present but hidden | |
| `TestRenderWorkbenchForm` | `internal/component/web/workbench_form_test.go` | Form renders text, number, dropdown, toggle, IP, list fields with Save/Discard buttons | |
| `TestRenderWorkbenchForm_FieldTypes` | `internal/component/web/workbench_form_test.go` | Each field type renders the correct HTML input element (input[type=text], input[type=number], select, checkbox, etc.) | |
| `TestBuildDashboardData` | `internal/component/web/workbench_dashboard_test.go` | Dashboard data builder returns panels with system info, BGP summary, interface counts, warnings, errors | |
| `TestBuildDashboardData_EmptyState` | `internal/component/web/workbench_dashboard_test.go` | With no config, dashboard returns empty-state hints with links | |
| `TestRenderDashboard` | `internal/component/web/workbench_dashboard_test.go` | Dashboard template renders all panel sections with correct HTML structure | |
| `TestHandleWorkbench_DashboardRendersOverview` | `internal/component/web/handler_workbench_test.go` | GET /show/ in workbench mode renders dashboard panels instead of YANG root children | |
| `TestFinderLayout_NoWorkbenchComponents` | `internal/component/web/workbench_render_test.go` | Finder layout contains no workbench table, detail, form, or two-level nav components | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Navigation depth | 1-3 levels | 3 (section > page) | N/A (1 is minimum) | N/A (max 2 levels in this design) |
| Table column count | 0-20 | 20 columns | N/A | Overflow handled by horizontal scroll |
| Form list items | 0-100 | 100 items | N/A (0 is valid empty list) | N/A (no hard limit in UI) |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `workbench-two-level-nav` | `test/web/workbench-two-level-nav.wb` | Workbench page shows two-level nav with Dashboard, Interfaces, Routing sections and sub-pages | |
| `workbench-dashboard-overview` | `test/web/workbench-dashboard-overview.wb` | Dashboard page shows system panels (hostname, version at minimum) | |
| `workbench-nav-active-highlight` | `test/web/workbench-nav-active-highlight.wb` | Navigating to /show/bgp/peer/ highlights Peers sub-page and expands Routing section | |
| `workbench-rollback-finder` | `test/web/workbench-rollback-finder.wb` (existing) | Finder mode shows no workbench components | |

### Future (if deferring any tests)
- SSE auto-refresh for dashboard counters (requires hub running with real BGP peers; Phase 2+ concern)
- Detail panel HTMX-driven tab switching (requires real table rows and a handler; tested in subsequent page-specific specs)

## Files to Modify
- `internal/component/web/workbench_sections.go` - Replace flat WorkbenchSection with two-level model; add WorkbenchSubPage and Expanded/Children fields
- `internal/component/web/render.go` - Update WorkbenchData if needed; add new template parsing for workbench component templates
- `internal/component/web/handler_workbench.go` - Add dashboard data path for root; pass new nav model to WorkbenchData
- `internal/component/web/templates/component/workbench_nav.html` - Replace flat list with two-level expand/collapse nav
- `internal/component/web/templates/page/workbench.html` - Add any new JS for nav expand/collapse (minimal, CSS-driven preferred)
- `internal/component/web/assets/style.css` - Add CSS for two-level nav, table component, detail panel, form component, dashboard panels
- `internal/component/web/workbench_render_test.go` - Add/update tests for two-level nav rendering
- `internal/component/web/handler_workbench_test.go` - Add test for dashboard rendering

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/architecture/web-components.md` - document new workbench components (table, detail, form, dashboard, two-level nav) |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/web-components.md` - add section on workbench foundation components and their data models |

## Files to Create
- `internal/component/web/workbench_dashboard.go` - Dashboard data model and builder: `DashboardData`, `DashboardPanel`, `buildDashboardData()` with system/BGP/interface/warning/error panels
- `internal/component/web/workbench_dashboard_test.go` - Tests for dashboard data builder and empty states
- `internal/component/web/workbench_table_test.go` - Tests for table component rendering (toolbar, empty state, flag colors)
- `internal/component/web/workbench_detail_test.go` - Tests for detail panel rendering (tabs, close, tools area)
- `internal/component/web/workbench_form_test.go` - Tests for form component rendering (field types, Save/Discard)
- `internal/component/web/workbench_sections_test.go` - Tests for two-level nav model (moved from workbench_render_test.go if needed, or new file)
- `internal/component/web/templates/component/workbench_table.html` - Reusable table component template: toolbar, headers, flag column, rows, actions, empty state
- `internal/component/web/templates/component/workbench_detail.html` - Reusable detail panel template: tabs (Config/Status/Actions), close button, tools area
- `internal/component/web/templates/component/workbench_form.html` - Reusable form component template: field types, Save/Discard buttons
- `internal/component/web/templates/component/workbench_dashboard.html` - Dashboard overview template: system panel, BGP summary, interfaces, warnings, errors, empty state hints
- `test/web/workbench-two-level-nav.wb` - Functional test: two-level nav renders with sections and sub-pages
- `test/web/workbench-dashboard-overview.wb` - Functional test: dashboard renders system panels
- `test/web/workbench-nav-active-highlight.wb` - Functional test: active page highlighted, parent section expanded

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

1. **Phase: Two-level navigation model** -- Restructure WorkbenchSection to support sections with children
   - Tests: `TestWorkbenchSections_TwoLevel`, `TestWorkbenchSections_TwoLevel_ChildSelected`, `TestWorkbenchSections_TwoLevel_SectionCollapse`, `TestWorkbenchSections_TwoLevel_DashboardDefault`, `TestWorkbenchSections_TwoLevel_AllSectionsPresent`
   - Files: `internal/component/web/workbench_sections.go`, `internal/component/web/workbench_sections_test.go`
   - Implementation:
     - Add `WorkbenchSubPage` struct: `{Key, Label, URL string; Selected bool}`
     - Extend `WorkbenchSection` with `Children []WorkbenchSubPage` and `Expanded bool`
     - Rewrite `WorkbenchSections()` to build the full two-level structure from the design doc:
       - Dashboard: Overview, Health, Recent Events
       - Interfaces: All Interfaces, Ethernet, Bridge, VLAN, Tunnel, Traffic
       - IP: Addresses, Routes, DNS
       - Routing: BGP > Peers, BGP > Groups, BGP > Families, BGP > Summary, Redistribute
       - Policy: Filters, Communities, Prefix Lists
       - Firewall: Tables, Chains, Rules, Sets, Connections
       - L2TP: Sessions, Configuration, Health
       - Services: SSH, Web, Telemetry, TACACS, MCP, Looking Glass, API
       - System: Identity, Users, Resources, Host Hardware, Sysctl Profiles
       - Tools: Ping, BGP Decode, Metrics Query, Capture
       - Logs: Live Log, Warnings, Errors
     - Implement child selection logic: match URL path to sub-page, set Selected on child and Expanded on parent
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Two-level navigation template and CSS** -- Render the new nav structure
   - Tests: `TestRenderWorkbenchNav_TwoLevel`, `TestRenderWorkbenchNav_ActiveHighlight`, `TestRenderWorkbenchNav_ExpandedSection`
   - Files: `internal/component/web/templates/component/workbench_nav.html`, `internal/component/web/assets/style.css`, `internal/component/web/workbench_render_test.go`
   - Implementation:
     - Replace flat `<ul>` in `workbench_nav.html` with nested structure: section headings that toggle expand/collapse, nested `<ul>` for sub-pages
     - Use HTML `<details>`/`<summary>` elements for expand/collapse (CSS-only, no JS needed for basic toggle). Set `open` attribute on expanded sections.
     - Add CSS: `.workbench-nav-section`, `.workbench-nav-section--expanded`, `.workbench-nav-sublist`, `.workbench-nav-subitem`, `.workbench-nav-subitem--active`, `.workbench-nav-section-label`
     - Update existing tests that check for section labels (they should still pass since labels are preserved)
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Table component** -- Reusable workbench table template and Go data model
   - Tests: `TestRenderWorkbenchTable`, `TestRenderWorkbenchTable_EmptyState`, `TestRenderWorkbenchTable_FlagColors`
   - Files: `internal/component/web/templates/component/workbench_table.html`, `internal/component/web/workbench_table_test.go`, `internal/component/web/assets/style.css`
   - Implementation:
     - Define `WorkbenchTableData` struct in a new or existing file: `Title string`, `AddURL string`, `AddLabel string`, `FilterOptions []string`, `Columns []WorkbenchTableColumn`, `Rows []WorkbenchTableRow`, `EmptyMessage string`, `EmptyHint string`
     - Define `WorkbenchTableColumn`: `Name string`, `Sortable bool`, `Key bool`
     - Define `WorkbenchTableRow`: `Flags string`, `FlagClass string` (red/grey/green), `Cells []string`, `URL string`, `Actions []WorkbenchRowAction`
     - Define `WorkbenchRowAction`: `Label string`, `URL string`, `Class string` (danger for delete), `HxPost string`, `Confirm string`
     - Create `workbench_table.html` template: toolbar div (Add button, filter `<select>`, search `<input>`), `<table>` with `<thead>` (sortable columns), `<tbody>` (rows with flag cell, data cells, action cell), empty state `<tr>` shown when no rows
     - Add CSS: `.wb-table-toolbar`, `.wb-table`, `.wb-table-flag`, `.wb-table-flag--red`, `.wb-table-flag--grey`, `.wb-table-flag--green`, `.wb-table-empty`, `.wb-table-actions`, `.wb-table-search`, `.wb-table-filter`, `.wb-table-add`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Detail panel component** -- Reusable detail drawer/panel template
   - Tests: `TestRenderWorkbenchDetail`, `TestRenderWorkbenchDetail_TabSwitching`
   - Files: `internal/component/web/templates/component/workbench_detail.html`, `internal/component/web/workbench_detail_test.go`, `internal/component/web/assets/style.css`
   - Implementation:
     - Define `WorkbenchDetailData` struct: `Title string`, `Tabs []WorkbenchDetailTab`, `ActiveTab string`, `CloseURL string`, `Tools []RelatedToolButton`
     - Define `WorkbenchDetailTab`: `Key string`, `Label string`, `Content template.HTML`, `Active bool`
     - Create `workbench_detail.html` template: panel wrapper, tab bar (`<div class="wb-detail-tabs">`), tab content areas (show active, hide inactive via CSS class), close button, tools area
     - Add CSS: `.wb-detail-panel`, `.wb-detail-tabs`, `.wb-detail-tab`, `.wb-detail-tab--active`, `.wb-detail-content`, `.wb-detail-close`, `.wb-detail-tools`, `.wb-detail-panel--open` (slide-in animation)
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Form component** -- Reusable form for singleton config pages
   - Tests: `TestRenderWorkbenchForm`, `TestRenderWorkbenchForm_FieldTypes`
   - Files: `internal/component/web/templates/component/workbench_form.html`, `internal/component/web/workbench_form_test.go`, `internal/component/web/assets/style.css`
   - Implementation:
     - Define `WorkbenchFormData` struct: `Title string`, `Fields []WorkbenchFormField`, `SaveURL string`, `DiscardURL string`
     - Define `WorkbenchFormField`: `Name string`, `Label string`, `Type string` (text/number/dropdown/toggle/ip/list), `Value string`, `Options []string` (for dropdown), `Items []string` (for list), `Description string`, `Required bool`
     - Create `workbench_form.html` template: title, field loop (dispatch on Type: `<input type="text">`, `<input type="number">`, `<select>`, `<input type="checkbox">`, IP input with pattern, list with add/remove), Save and Discard buttons
     - Add CSS: `.wb-form`, `.wb-form-field`, `.wb-form-label`, `.wb-form-input`, `.wb-form-toggle`, `.wb-form-list`, `.wb-form-list-item`, `.wb-form-list-add`, `.wb-form-actions`, `.wb-form-save`, `.wb-form-discard`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Dashboard overview page** -- System panels with real/placeholder data
   - Tests: `TestBuildDashboardData`, `TestBuildDashboardData_EmptyState`, `TestRenderDashboard`, `TestHandleWorkbench_DashboardRendersOverview`
   - Files: `internal/component/web/workbench_dashboard.go`, `internal/component/web/workbench_dashboard_test.go`, `internal/component/web/templates/component/workbench_dashboard.html`, `internal/component/web/handler_workbench.go`, `internal/component/web/assets/style.css`
   - Implementation:
     - Define `DashboardData` struct: `System DashboardSystemPanel`, `BGP DashboardBGPPanel`, `Interfaces DashboardIfacePanel`, `Warnings []DashboardWarning`, `Errors []DashboardError`
     - `DashboardSystemPanel`: `Hostname string`, `Uptime string`, `Version string`
     - `DashboardBGPPanel`: `Established int`, `Active int`, `Idle int`, `Down int`, `Total int`, `Empty bool`, `HintURL string`
     - `DashboardIfacePanel`: `Up int`, `Down int`, `AdminDown int`, `Total int`, `Empty bool`, `HintURL string`
     - `DashboardWarning`/`DashboardError`: `Time string`, `Component string`, `Message string`
     - `buildDashboardData()`: gather hostname from config tree (`identity/hostname`), uptime from Go runtime, version from build info; BGP peer counts from config tree walk + stub for runtime state; interface counts from config tree walk; warnings/errors initially empty (populated by SSE in later specs)
     - In `HandleWorkbench`, when path is empty: call `buildDashboardData()`, render `workbench_dashboard` template instead of `detail` fragment
     - Create `workbench_dashboard.html`: grid of panels, each panel has title + content; empty-state panels show hint text with links
     - Add CSS: `.wb-dashboard`, `.wb-dashboard-panel`, `.wb-dashboard-panel-title`, `.wb-dashboard-panel-body`, `.wb-dashboard-empty-hint`, `.wb-dashboard-grid`
   - Verify: tests fail -> implement -> tests pass

7. **Phase: Integration and full verification**
   - Tests: all functional tests (`workbench-two-level-nav.wb`, `workbench-dashboard-overview.wb`, `workbench-nav-active-highlight.wb`), plus existing `workbench-shell.wb` and `workbench-rollback-finder.wb`
   - Files: `test/web/workbench-two-level-nav.wb`, `test/web/workbench-dashboard-overview.wb`, `test/web/workbench-nav-active-highlight.wb`
   - Implementation:
     - Write functional .wb tests
     - Run `make ze-verify` to confirm all unit tests, lint, and functional tests pass
     - Verify existing workbench tests still pass (especially `workbench-shell.wb`, `workbench-rollback-finder.wb`)
   - Verify: `make ze-verify` clean

8. **Functional tests** -> Create after feature works. Cover user-visible behavior.
9. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)
10. **Complete spec** -> Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-14 has implementation with file:line |
| Correctness | Two-level nav matches design doc section structure exactly; no missing or extra sections |
| Naming | CSS classes use `wb-` prefix for new workbench components; Go types use `Workbench` prefix |
| Data flow | Dashboard data assembled in handler, not in template; templates receive fully built data |
| Rule: no-layering | Old flat WorkbenchSections() removed or replaced, not wrapped |
| Rule: no-finder-bleed | Finder layout has zero workbench component classes; workbench has zero Finder column classes |
| Rule: template isolation | New templates are `{{define "name"}}` blocks, parseable by the existing component glob |
| Rule: CSS variable usage | New CSS uses existing CSS variables (--bg, --border, --accent, --text, --text-muted) |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Two-level WorkbenchSection model with Children | `grep 'Children' internal/component/web/workbench_sections.go` |
| WorkbenchSections() returns two-level structure | `go test -run TestWorkbenchSections_TwoLevel ./internal/component/web/` |
| workbench_nav.html renders nested nav | `grep 'workbench-nav-sublist' internal/component/web/templates/component/workbench_nav.html` |
| workbench_table.html exists | `ls internal/component/web/templates/component/workbench_table.html` |
| workbench_detail.html exists | `ls internal/component/web/templates/component/workbench_detail.html` |
| workbench_form.html exists | `ls internal/component/web/templates/component/workbench_form.html` |
| workbench_dashboard.html exists | `ls internal/component/web/templates/component/workbench_dashboard.html` |
| Dashboard handler renders overview | `go test -run TestHandleWorkbench_DashboardRendersOverview ./internal/component/web/` |
| Table empty state works | `go test -run TestRenderWorkbenchTable_EmptyState ./internal/component/web/` |
| Detail panel tabs work | `go test -run TestRenderWorkbenchDetail ./internal/component/web/` |
| Form field types work | `go test -run TestRenderWorkbenchForm_FieldTypes ./internal/component/web/` |
| CSS for all components added | `grep 'wb-table' internal/component/web/assets/style.css` |
| Finder unchanged | `go test -run TestFinderLayout_NoWorkbenchComponents ./internal/component/web/` |
| Existing tests pass | `go test ./internal/component/web/...` |
| Functional test: two-level nav | `ls test/web/workbench-two-level-nav.wb` |
| Functional test: dashboard | `ls test/web/workbench-dashboard-overview.wb` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | All URL path segments validated by existing ValidatePathSegments(); no new URL parsing introduced without validation |
| Template injection | All dynamic content in templates uses `{{.Field}}` (auto-escaped by Go html/template); no raw HTML injection via `template.HTML` for user-supplied data |
| CSS injection | No user-supplied strings used in class names or inline styles |
| Dashboard data exposure | System info (hostname, uptime, version) only shown to authenticated users (behind auth middleware) |
| HTMX target safety | All hx-target values are fixed IDs, not user-controllable |

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
| Existing workbench tests break | Prioritize fixing; two-level nav change must preserve section labels that existing tests check |

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

Not applicable: no protocol work in this spec.

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

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
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
