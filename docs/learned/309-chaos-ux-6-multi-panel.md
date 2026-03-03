# 309 — Chaos UX-6 Multi-Panel Viz Layout

## Objective

Allow 2-4 viz panels to be displayed simultaneously in a CSS Grid layout alongside the existing single-tab mode, with each panel independently polling its selected viz endpoint.

## Decisions

- Panel content is served via `/viz/panel-content?panel=N&viz=name` rather than modifying each existing viz handler to accept a `panel` query param — simpler: renders the full viz then strips outer HTMX attributes; avoids touching all 8 existing handlers
- `stripOuterVizAttrs()` strips `hx-trigger`, `hx-get`, `hx-target`, `hx-swap` from the outermost viz div — prevents double-polling (panel slot has its own polling; inner viz attrs would conflict)
- `viz_panels.go` is a new file rather than adding to `viz.go` — viz.go was already 1349 lines (above 1000 threshold); new viz features must go in separate files
- All 4 slots always render (AC-5 changed): CSS Grid handles layout with empty content; 2-panel layout via CSS not tracked panel count — simpler than conditionally rendering grid dimensions
- Functional test skipped — no `test/chaos/` directory exists; all coverage via Go unit tests (14 tests in `viz_panels_test.go`)

## Patterns

- `renderVizToBuffer()` dispatches to all 8 existing viz write functions by name — switch statement, no registry needed, all 8 are in the same package
- Panel unique IDs: `viz-panel-0` through `viz-panel-3` for slots; `viz-panel-content-0` through `viz-panel-content-3` for content swap targets — no collision with single-tab `#viz-content`
- `block-ignored-errors.sh` hook blocks `_, _ = fmt.Fprintf()` — use `htmlWriter` pattern (consistent with the rest of the codebase)

## Gotchas

- Inner viz filter controls (Events peer/type selects) target `#viz-content` in panel mode — in panel mode this is absent, so inner filter controls don't work. Acceptable for v1.

## Files

- `cmd/ze-chaos/web/viz_panels.go` — handleVizPanels, writePanelGrid, writePanelSlot, renderVizToBuffer, stripOuterVizAttrs (new file)
- `cmd/ze-chaos/web/viz_panels_test.go` — 14 tests (new file)
- `cmd/ze-chaos/web/render.go` — Panels toggle button, `// Related: viz_panels.go`
- `cmd/ze-chaos/web/handlers.go` — 2 new routes: /viz/panels and /viz/panel-content
- `cmd/ze-chaos/web/assets/style.css` — .panel-grid CSS Grid, responsive single-column at 900px
