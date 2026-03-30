# 486 -- CLI Navigation Sync

## Context

The web config editor's CLI bar showed `/>` as its prompt on page load, losing the current path context. Navigation commands (`edit`, `show`, `top`, `up`) in the CLI bar did not pass the current context path to the server, so relative navigation was broken. GUI navigation (clicking finder links) did not update the CLI prompt or context, leaving the two interfaces out of sync. There was no visual path indicator above the CLI input.

## Decisions

- Chose a hidden `<span id="cli-context-path">` as the single source of truth for CLI context in the browser, over tracking state in a JS variable (DOM element is naturally updated by HTMX OOB swaps).
- Path bar segments are pre-computed in Go as `PathBarSegment` structs (Name, URL, HxPath) passed to a template, over using the `joinpath` template function. This avoids needing `joinpath` in the layout template set where it's unavailable.
- CLI prompt uses `formatCLIPrompt()` (`ze[bgp peer X]# `) on all code paths, replacing the ad-hoc `/>` format in `HandleFragment`.
- `adjustListContext()` is applied to CLI fields in `buildFragmentData` so the CLI prompt/path bar correctly reflects that the CLI can't be "at" a named list node (e.g., web shows `/bgp/peer/` table, CLI stays at `ze[bgp]# `).
- URL syncs via `history.pushState` only for CLI-initiated navigation (detected by checking `e.detail.requestConfig.path === '/cli'`), not for finder clicks which use `hx-push-url`.

## Consequences

- CLI and GUI navigation are now bidirectionally synced: typing `edit bgp` updates the web view, breadcrumbs, path bar, prompt, and URL; clicking a finder link updates the CLI prompt and context.
- Tab completion and all CLI commands now use the tracked context path instead of parsing it from the URL, which was fragile and didn't account for CLI-only navigation.
- Admin pages explicitly set CLI fields to root state to prevent OOB swaps from clearing the CLI bar with empty values.

## Gotchas

- `cli_bar.html` is parsed in both the layout template set (no `joinpath`) and the fragments template set (has `joinpath`). Cannot use fragment-only template functions in `cli_bar.html`. Solved by pre-rendering the path bar HTML in Go and passing as `template.HTML`.
- The `handleCLIShow` function needed the `renderer` parameter added to its signature for path bar OOB rendering, requiring updates to the dispatch call site.
- The `adjustListContext` call is critical for CLI fields: without it, navigating to a list view in the GUI would set the CLI context to an impossible state (e.g., `ze[bgp peer]# ` with no entry key).

## Files

- `internal/component/web/cli.go` -- PathBarSegment type, buildPathBarSegments, buildPathBarOOB, buildContextOOB, updated writeCLIResponse/handleCLIShow
- `internal/component/web/fragment.go` -- CLIPrompt/CLIContextPath/CLIPathSegments on FragmentData, adjustListContext for CLI fields
- `internal/component/web/render.go` -- CLIContextPath/CLIPathBar on LayoutData
- `internal/component/web/handler_admin.go` -- CLI fields in buildAdminFragmentData
- `internal/component/web/assets/cli.js` -- getContextPath(), navigation sync, URL pushState
- `internal/component/web/assets/style.css` -- path bar styling
- `internal/component/web/templates/component/path_bar.html` -- new path bar template
- `internal/component/web/templates/component/cli_bar.html` -- restructured with path bar
- `internal/component/web/templates/component/oob_response.html` -- CLI OOB swaps
