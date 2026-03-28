# 455 -- Web Interface Session 1

## Context

Completed the remaining work: wiring SSE, admin commands, CLI handlers, insecure mode, and iterating on the UI based on live user feedback. The web interface provides YANG schema-driven config viewing and editing via HTMX.

## Decisions

- Chose HTMX 2.x for all dynamic UI over custom JS. Requires `unsafe-eval` in CSP (HTMX uses `Function()` internally). Tried `allowEval:false` first but it silently broke all HTMX functionality.
- Chose server-side field re-rendering over JS DOM manipulation. After a config set, the server returns the full re-rendered field HTML (border color, badge, value) via `hx-swap="outerHTML"` on `closest .ze-field`. Removed all JS field/tristate state cycling code.
- Chose `--insecure-web` flag restricted to `127.0.0.1` over any other testing auth bypass. `InsecureMiddleware` injects username "insecure" into request context so all handlers work.
- Removed standalone `ze web` subcommand. `ze start --web` is the only entry point, adding web alongside the BGP daemon. Web-only fallback when no config exists.
- Chose Finder-style column navigation (macOS Finder) over single sidebar. Up to 3 columns visible, mixed behavior: lists get a column for entry selection, leaf-only containers go to edit panel.
- Built `.wb` browser test framework wrapping `agent-browser` CLI for declarative web UI tests, analogous to `.ci` (BGP) and `.et` (editor TUI).

## Consequences

- Fragment handler now takes `EditorManager` to read the user's working tree. Config changes (set values, created entries) are visible when navigating. Without this, the static empty tree made all edits invisible.
- SSE moved from JS `EventSource` to HTMX SSE extension (`hx-ext="sse"`). View toggle moved from 50-line JS DOM clone to HTMX `hx-post="/cli/mode"`. `cli.js` reduced from ~350 to ~280 lines.
- Tristate boolean slider (yes/default/no) with YANG default color hint on the center position. Server re-renders the correct state on each click.
- Light/dark theme toggle with full CSS variable override. Username and insecure warning in breadcrumb bar.
- `.wb` tests run via `ze-test web` with `make ze-web-test`. Runner starts server, opens headless browser, executes declarative steps.

## Gotchas

- **FragmentData missing HasSession**: The `breadcrumb_inner` template accessed `.HasSession` which didn't exist on `FragmentData`. Go templates fail silently on missing struct fields, and `RenderFragment` returned empty string. Every HTMX click got an empty 200 response. Root cause took 3 attempts to find because the error was swallowed.
- **CSP blocks HTMX silently**: HTMX 2.x uses `Function()` internally. Without `unsafe-eval`, HTMX degrades silently (no console errors). Buttons appear to do nothing. Spent 2 rounds of changes before identifying this.
- **Static tree vs editor tree**: The fragment handler was initialized with `config.NewTree()` (empty) and never updated. All config edits went to the editor's session tree, invisible to the fragment handler. Had to wire `EditorManager` into the fragment handler.
- **Collapsed error panel clipping**: A `position: fixed` error panel with `translateX(calc(100% - 2rem))` left a 2rem bar on the right edge, clipping the CLI button. Changed to `translateX(100%)`.
- **SSE banner template had OOB wrapper**: When switching from JS `EventSource` to HTMX SSE extension, the banner template still had `hx-swap-oob` wrapper div. HTMX SSE extension swaps directly into the target, no OOB needed.
- **Terminal mode output as raw HTML**: The CLI terminal handler returned HTML fragments but `textContent +=` escaped them. Changed to `insertAdjacentHTML('beforeend', html)`.
- **Commit redirect changed page**: `handleCommitPost` redirected to `/config/edit/` after commit, changing the breadcrumb and CLI prompt under the overlay modal. Fixed to return closed modal + OOB empty commit bar.
- **Breadcrumb corruption on back**: `extractPath` treated `?path=` (empty value) as "no path param" and fell through to URL path parsing, producing `["fragment", "detail"]`. Fixed with `Query().Has("path")`.

## Files

Key files modified/created:
- `internal/component/web/fragment.go` -- FinderColumn types, buildFinderColumns, editor tree wiring
- `internal/component/web/handler_config.go` -- server-side field re-rendering, __default__ delete
- `internal/component/web/handler_admin.go` -- BuildAdminCommandTree, nil dispatcher guard
- `internal/component/web/render.go` -- RenderField for HTMX field swap
- `internal/component/web/auth.go` -- InsecureMiddleware, CSP headers
- `internal/component/web/sse.go` -- simplified banner template for HTMX SSE
- `internal/component/web/editor.go` -- session-aware Diff
- `internal/component/web/assets/cli.js` -- command history, theme toggle, flyout, delegated actions
- `internal/component/web/assets/style.css` -- Finder columns, tristate, light mode, tooltips
- `internal/component/web/testing/` -- .wb parser, runner, expectations
- `cmd/ze-test/web.go` -- ze-test web CLI
- `test/web/*.wb` -- browser test files
- `cmd/ze/main.go` -- removed ze web, added --insecure-web to ze start
