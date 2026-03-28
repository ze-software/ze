# 454 -- Web HTMX Component Architecture

## Context

The web interface had been built across six specs (web-1 through web-6) but the server started with an empty mux -- no routes registered. The page returned 404. Once routes were wired, the rendering was monolithic: full-page server renders, Go code building HTML via `buf.WriteString` and `fmt.Fprintf`, all field types rendered as plain text inputs, and a large `field.js` handling DOM creation, save logic, error display, and commit workflow. The user wanted a properly composable web interface driven by YANG schema with type-appropriate inputs, sidebar navigation, auto-save, and no unnecessary JavaScript.

## Decisions

- **HTMX for all interactions, not custom JS** over the initial approach of JS fetch + DOM manipulation. HTMX attributes on server-rendered inputs handle save-on-blur, toggle-on-click, and change events. Server returns OOB swaps for commit bar and error panel. Eliminated `field.js` entirely. Only `cli.js` remains (Tab/? key interception has no HTMX equivalent).

- **One template file per input type** over a single template with if/else dispatch. `fieldFor()` Go function dispatches to `input_<type>` template at render time. Adding a new type = one file + one case in `valueTypeToFieldType()`.

- **Template FS mirrors page structure** (`page/`, `component/`, `input/`) over flat directory. Each visible UI region is exactly one template file.

- **Sidebar-only navigation** over duplicate tiles in both sidebar and detail panel. Detail panel shows only leaf fields. Sidebar shows current node's children (containers as headings, lists with entries and add forms).

- **Server-driven commit workflow** over JS-managed state. Commit bar appears via OOB swap after saves. Diff modal opened/closed by server returning HTML with/without `open` CSS class. No JS state tracking for pending changes.

- **Bool toggle sends the new value** over checkbox presence/absence pattern. The old `HandleConfigSet` checked if `r.Form["value"]` key existed (checkbox idiom). Toggle buttons send `value=true` or `value=false` explicitly.

- **`ze start --web` works without config** over requiring a BGP config. Enables initial setup from the browser. Empty config file created if needed.

- **TLS cert persisted in zefs** over regenerating on every start. Browsers don't need to re-accept the cert after restart.

## Consequences

- No custom JavaScript for field rendering or save behavior. All interaction is declarative via HTMX attributes. This means CSP can be strict (`script-src 'self'`, no `unsafe-eval`).

- YANG schema changes automatically appear in the web UI. No hardcoded field lists. New containers, lists, leaves, and enums show up when the schema is updated.

- The `Description` field added to `ContainerNode` and `ListNode` (plus existing `LeafNode.Description`) means any YANG node with a description shows an (i) tooltip. This affects schema memory usage slightly but the descriptions were already parsed and discarded.

- `LeafNode.Enums` stores enum values extracted during YANG conversion. This enables `<select>` dropdowns. Previously enum values were only available through the validator, not the schema.

- The `fieldFor()` dispatch pattern means the template engine executes three sub-templates per field (wrapper_start + input_type + wrapper_end). Negligible cost but worth knowing.

## Gotchas

- **Empty mux was invisible.** The web server started, accepted TLS, returned Go's default 404 for every request. No error logged. The server appeared healthy from the outside. Always wire at least one route or add a startup check.

- **HTMX filter expressions in `hx-trigger` use eval internally.** This requires `unsafe-eval` in CSP. We replaced the filter with a plain JS event listener in `cli.js` to keep CSP strict. Any HTMX trigger with a bracket expression (e.g., `keydown[key=='Enter']`) triggers this.

- **`hx-swap-oob` on `<aside>` elements works but the OOB element's tag must match the page element's tag.** The response had `<aside id="sidebar">` targeting `<aside class="sidebar-panel" id="sidebar">`. HTMX matches by `id`, ignores tag mismatch, but class differences can cause style loss if using `outerHTML` swap mode. Use `innerHTML` to preserve the container.

- **SSE attribute without endpoint causes browser error.** `<body hx-ext="sse" sse-connect="/events">` tried to connect to a non-existent endpoint, got HTML back instead of `text/event-stream`, logged a MIME type error on every page load.

- **`go:embed` with subdirectories just works.** `ParseFS(fs, "templates/component/*.html")` finds files in subdirectories of the embedded FS. No extra embed directives needed beyond the top-level `//go:embed templates`.

- **Bool save bug: checkbox vs toggle semantics.** `HandleConfigSet` used `_, present := r.Form["value"]` (HTML checkbox pattern: key present = checked). Toggle buttons always send the key with a value. The presence check returned true for both "true" and "false", so it always set "true". Fix: read the actual value string.

- **Browser keeps connection open during shutdown.** `http.Server.Shutdown` with a timeout waits for active connections. If a browser has a keep-alive connection, shutdown stalls until timeout. Fix: second Ctrl+C calls `os.Exit(1)` immediately.

- **Self-signed cert SANs for 0.0.0.0.** `GenerateWebCertWithAddr("0.0.0.0:8443")` added `0.0.0.0` as a SAN, but browsers connecting via `10.x.x.x` rejected the cert. Fix: when listen address is unspecified, enumerate all interface IPs and add them as SANs.

## Files

- `internal/component/web/fragment.go` -- HTMX fragment handler, FragmentData, FieldMeta, sidebar builder, OOB error writer
- `internal/component/web/render.go` -- template loading, RenderFragment, fieldFor dispatch
- `internal/component/web/templates/` -- reorganized: page/, component/, input/
- `internal/component/web/assets/cli.js` -- CLI autocomplete (only remaining JS)
- `internal/component/web/handler_config.go` -- config set with OOB commit bar response
- `internal/component/web/server.go` -- TLS cert with interface IPs, error log suppression
- `internal/component/web/auth.go` -- GetUsernameFromRequest (exported), CSP without unsafe-eval
- `internal/component/config/schema.go` -- Description on ContainerNode/ListNode, Enums on LeafNode
- `internal/component/config/yang_schema.go` -- populate Description/Enums during YANG conversion
- `cmd/ze/hub/main.go` -- startWebServer with full route wiring, RunWebOnly, signal handling
- `cmd/ze/main.go` -- --web flag on ze start
- `cmd/ze/init/main.go` -- piped input + /dev/tty confirmation
