# 687 -- web-3-foundation

## Context

The workbench shell from spec-web-2 provided the layout frame (left nav, top bar, content area) but every nav link rendered the same Finder detail fragment. This spec replaced the hollow shell with working building blocks: two-level navigation, reusable table component, detail panel, singleton form, dashboard overview, and consistent CSS. Every subsequent web page (interfaces, BGP, firewall, system, tools, logs) builds on these components.

## Decisions

- **Two-level navigation with path-based selection** over flat nav or YANG-driven nav. Sections contain sub-pages; selection is computed from the URL path segments, not stored state. `selectChild` / `selectBestChild` do longest-prefix matching so deep paths (e.g., `/show/bgp/peer/AS65001`) still highlight the correct parent section and child.
- **Data model structs (`WorkbenchTableData`, `WorkbenchDetailData`, `WorkbenchFormData`)** over template-level construction. Page files build typed Go structs; templates receive them and render. This separates data assembly (testable in Go) from HTML rendering (testable via template output assertions).
- **Flag color classes as constants** (`flagClassGreen`, `flagClassRed`, `flagClassGrey`, `flagClassYellow`) over inline strings. Prevents typos and makes color semantics greppable.
- **Dashboard with placeholder operational data** over blocking on live BGP/interface state. System panel uses real data (hostname, uptime, version, memory). BGP and interface counts use config-derived totals with session-state breakdowns populated by later specs.
- **Empty state with hint links** over blank panels. Dashboard panels with no data show "No BGP peers configured" with a link to the relevant page, guiding new operators to the right place.
- **CSS in one file** (`style.css`) over component-scoped stylesheets. Keeps specificity predictable and avoids cascade surprises across workbench components.

## Consequences

- All web pages share the same component vocabulary. Adding a new page means: write a `page_*.go` that builds table/detail/form data, register it in `workbench_pages.go`, and the nav/rendering/CSS comes for free.
- The two-level nav supports 11 sections with 3-7 sub-pages each, covering the full design document taxonomy.
- Dashboard is the landing page (`/show/` in workbench mode), giving operators an immediate system overview.
- Finder mode remains completely unchanged (AC-13), gated by `UIMode`.

## Gotchas

- Path matching for the Policy section requires checking the second path segment (`bgp/policy`, `bgp/community`, `bgp/prefix-list`) because both Routing and Policy sections share the `bgp` first segment. The `selectChild` switch handles this explicitly.
- The `?type=ethernet` query parameter on interface sub-pages is stripped before path matching (`IndexByte(childPath, '?')`) so type-filtered views still highlight the parent "All Interfaces" or the specific type child.
- Dashboard `processStart` is a package-level `time.Now()` call, not the actual process start time. Uptime is approximate.
- `WorkbenchFormField.Type` is a string enum ("text", "number", "dropdown", "toggle", "ip", "list", "password") validated only by template rendering, not by Go code. Invalid types render as empty.

## Files

- `internal/component/web/workbench_sections.go` -- two-level nav model, section definitions, path-based selection
- `internal/component/web/workbench_table.go` -- WorkbenchTableData, column, row, action types
- `internal/component/web/workbench_detail.go` -- WorkbenchDetailData, tab, tool types
- `internal/component/web/workbench_form.go` -- WorkbenchFormData, field types
- `internal/component/web/workbench_dashboard.go` -- DashboardData builder, system/BGP/iface panels
- `internal/component/web/workbench_pages.go` -- page registry mapping paths to page builders
- `internal/component/web/templates/component/workbench_nav.html` -- two-level nav template
- `internal/component/web/templates/component/workbench_table.html` -- reusable table template
- `internal/component/web/templates/component/workbench_detail.html` -- detail panel template
- `internal/component/web/templates/component/workbench_form.html` -- singleton form template
- `internal/component/web/templates/component/workbench_dashboard.html` -- dashboard overview template
- `internal/component/web/assets/style.css` -- workbench CSS (lines ~2250-2487)
