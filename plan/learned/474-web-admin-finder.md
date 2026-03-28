# 474 -- Web Admin Finder Navigation

## Context
The web interface originally designed admin commands (D-21) with card stacking: each command execution produced a result card, with new cards appearing above previous ones. Config navigation already used a macOS Finder-style column browser. The two areas had different UX patterns for navigating hierarchical trees, creating inconsistency.

## Decisions
- Chose unified Finder navigation for both config and admin over card stacking, because the command tree is hierarchical (peer > teardown, cache > list) and Finder columns are already proven for this in the config view.
- Reused `FragmentData`, `FinderColumn`, `ColumnItem` types and `oob_response` template over creating separate admin-specific templates. Added `CommandForm` and `CommandResult` fields to `FragmentData` so the detail panel renders either config fields or admin forms/results depending on context.
- Removed `AdminViewData`, `containerFromAdmin`, and the separate `container.html`/`command_form.html` rendering paths. Admin now flows through the same fragment renderer as config.
- Command execution results replace the detail panel content (the form), not stack. User re-navigates to re-execute.

## Consequences
- Single layout pattern for all tree navigation (config, admin). Any future tree-structured views (monitoring, diagnostics) can reuse the same Finder infrastructure.
- `detail.html` template now has a priority chain: `CommandResult` > `CommandForm` > `Fields` > hint. Adding new detail types means extending this chain.
- Card stacking is gone. If a user wants to compare results of two commands, they must execute them separately. This trades history for simplicity.

## Gotchas
- The admin Finder columns use `/admin/` URL prefix in `HxPath`, not `/show/`. The Finder template's `hx-get` points to `/fragment/detail?path=...` which only handles config paths. Admin finder items link directly via `href` and full page navigation, not HTMX partial loads. Wiring admin into the fragment endpoint would require path-prefix awareness in `extractPath`.
- `HandleAdminExecute` now returns `FragmentData` rendered via the `detail` template for HTMX requests, but the detail panel's `hx-post` target is `#detail` with `innerHTML` swap. After execution, the form is replaced by the result -- there's no "back to form" button yet.

## Files
- `internal/component/web/handler_admin.go` -- `buildAdminFragmentData`, `buildAdminFinderColumns`, updated handlers
- `internal/component/web/fragment.go` -- `CommandForm`, `CommandResult`, `Monitor` fields on `FragmentData`
- `internal/component/web/templates/component/detail.html` -- priority chain for admin rendering
- `internal/component/web/handler_admin_test.go` -- tests updated for Finder data model
