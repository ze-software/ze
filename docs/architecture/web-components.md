# Web Component Architecture

<!-- source: internal/component/web/fragment.go -- HandleFragment, FragmentData -->
<!-- source: internal/component/web/render.go -- Renderer, renderComponent -->

## Design Principles

The web interface follows three rules:

1. **Server renders HTML, HTMX handles interaction.** No custom JavaScript creates UI elements. All HTML comes from templ components. HTMX attributes on elements handle save, navigation, and error display. The only JS file (`cli.js`) handles Tab/? key interception for CLI autocomplete, which has no HTMX equivalent.

2. **One component per visual concern.** Each `.templ` file renders exactly one thing. Adding a new input type means adding one file and one line in the `fieldInputs` registry. The file names mirror the page structure.

3. **One HTTP request updates multiple components.** HTMX out-of-band (OOB) swaps let a single response update the detail panel, Finder columns, breadcrumb, commit bar, and error panel.

## Page Layout

```
+--------------------------------------------------+
| #breadcrumb   / > bgp > peer > london     [CLI] |
+----------+---------------------------------------+
| #finder  | #detail                               |
|          |                                       |
| PEER  3  |  router-id: [1.2.3.4______]          |
| GROUP 1  |  listen:    [0.0.0.0:179___]          |
| -------- |  hold-time: [90_____________]         |
| local    |                                       |
| timer    |                                       |
| rib      |                                       |
+----------+---------------------------------------+
| #commit-bar   3 pending changes [Review] [Discard]|
+--------------------------------------------------+
| /> set bgp peer london remote as 65001           |
+--------------------------------------------------+
```

<!-- source: internal/component/web/fragment.go -- FinderColumn NamedItems/UnnamedItems -->

Named containers (lists with YANG keys) appear above the separator line. Unnamed containers and settings appear below. When a list has YANG `unique` constraints, it renders as an interactive table:

```
+-----------------------------------------------+
| peer                                          |
+-----------------------------------------------+
| [E] name    | remote/ip    |                  |
+-----------------------------------------------+
| [E] london  | 10.0.0.1     | [D]             |
| [E] paris   | 10.0.0.2     | [D]             |
+-----------------------------------------------+
| [+ new]                                       |
+-----------------------------------------------+
```

<!-- source: internal/component/web/fragment.go -- buildListTable, ListTableView -->

Hidden overlays (shown on demand):
- `#diff-modal` -- diff review with Confirm Commit / Cancel
- `#error-panel` -- collapsible right-side panel for validation errors

## Per-page asset imports

Each page loads the vendored assets its own markup needs, and no others. The set
is derived at build time, never at request time.

`scripts/codegen/web_assets.go` reads the `.templ` sources of
`internal/component/web` and `internal/component/lg`, and the Go string literals
of `internal/chaos/web`. From each page it walks the `@component(...)`
invocations transitively and collects the `hx-*` and `sse-*` attributes every
component it reaches names. It writes one `page_assets.go` per package, and the
head block renders that set:

```
for _, src := range pageAssets(pgPageLayout) {
	<script src={ src }></script>
}
```

A page is one of two shapes. A SHELL renders `<head>` and names itself, as
`pageAssets(pgL2tpList)`. A BODY carries the `//ze:page` marker and renders
inside a shell that names its page at render time, as `pageAssets(v.Page)`. The
looking glass is why the second shape exists: one layout serves every page, and
only the peers page opens an SSE stream, so the other pages stopped loading the
extension.

An unknown page gets every asset the package renders. One file too many costs
bytes; one missing gives a page that renders correctly and does nothing in the
browser.

<!-- source: scripts/codegen/web_assets.go -- templPages, closure, render -->
<!-- source: internal/component/lg/layout.templ -- pageAssets(v.Page), the one shell -->

Two checks hold the set honest, from opposite directions, and neither is relaxed
to make the other pass.

| Check | Reads | Error direction |
|-------|-------|-----------------|
| `make ze-web-assets-check` | the sources | OVER-approximates: a branch no request takes still contributes its asset |
| `TestPageImportsCoverRenderedAttributes` (web), `TestLGPageImportsCoverRenderedAttributes`, `TestChaosPageImportsCoverRenderedAttributes` | the captured fixtures | UNDER-approximates: a branch no fixture exercises is invisible |

<!-- source: internal/test/markupcheck/head.go -- HeadCoverageFindings, the fixture side -->

## Component Filesystem

Every unit below is a templ component in `internal/component/web`. The file name
carries the visual concern, and `make generate` writes a `*_templ.go` beside
each source.

```
page_layout.templ                -- pageLayout: Finder grid layout
page_workbench.templ             -- pageWorkbench: workbench shell (default)
page_login.templ                 -- pageLogin, loginOverlay, loginFullPage, loginForm
page_snapshot.templ              -- snapshotPage: one command's JSON, live over SSE

component_breadcrumb.templ       -- breadcrumbNav, topbarActions, breadcrumbInner
component_detail.templ           -- detail: leaf fields, or the panel the path resolves to
component_detail_kv.templ        -- detailKVTable, detailKVSection: label and value rows
component_finder.templ           -- finder, finderOOB, finderItem
component_list_table.templ       -- listTable, pendingMarker
component_commit_bar.templ       -- commitBar: change count, Review and Discard
component_error_panel.templ      -- errorPanel: collapsible panel with the error list
component_diff_modal.templ       -- diffModal (closed), diffModalOpen (with content)
component_oob_response.templ     -- oobResponse, fullContent, mainSplit
component_oob_save.templ         -- oobSaveOK: OOB commit bar after a save
component_oob_error.templ        -- oobError, errorItem
component_notification_error.templ -- notificationError: OOB toast
component_add_form_overlay.templ -- addFormOverlay: create one list entry
component_path_bar.templ         -- pathBarInner: CLI path bar
component_command_form.templ     -- commandForm
component_command_result.templ   -- commandResult
component_workbench_topbar.templ -- workbenchTopbar
component_workbench_nav.templ    -- workbenchNav
component_workbench_table.templ  -- workbenchTable
component_workbench_form.templ   -- workbenchForm
component_workbench_detail.templ -- workbenchDetail
component_workbench_dashboard.templ, component_dashboard_health.templ,
component_dashboard_events.templ
component_log_live.templ, component_log_table.templ
component_tool_ping.templ        -- toolPing, toolResult
component_tool_bgp_decode.templ, component_tool_capture.templ,
component_tool_metrics.templ
component_tool_overlay.templ     -- toolOverlay: related-tool result, error or prompt
component_system_panels.templ    -- systemResources, hostHardware
component_iface_detail.templ     -- ifaceDetailConfig, ifaceDetailCounters, ifaceDetailMissing
component_peer_detail.templ      -- peerDetailStatus, peerDetailActions
component_page_shells.templ      -- pollPanel, portalFrame, notificationPre, featureDisabled
component_cli_terminal.templ     -- cliPage, terminalContent, cliResponse, cliShowResponse,
                                    configViewBody, configChildList, configKeyList,
                                    configLeafTable, breadcrumbList, and the five
                                    cli*OOB swaps

input_wrapper.templ              -- fieldWrapper: label, tooltip, decoration, frame
input_bool.templ                 -- inputBool: tristate toggle
input_enum.templ                 -- inputEnum: select dropdown
input_number.templ               -- inputNumber
input_text.templ                 -- inputText, fieldValueTag

config_container.templ, config_list.templ, config_inline_list.templ,
config_freeform.templ, config_leaf_input.templ
config_breadcrumb.templ, config_commit.templ, config_notification.templ,
config_command.templ, config_command_form.templ

l2tp_list.templ, l2tp_detail.templ
notification_banner.templ        -- notificationBanner (SSE)
```

`TestNoGoFileBuildsMarkup` (`internal/component/web/markup_check_test.go`) holds
this list to its claim. It reads every Go source in the package and reports a
string literal that builds a tag. A panel written in Go is therefore a red test
rather than a file nobody added above.

`fieldWrapper` takes the editor as a component, so the frame is one balanced
element rather than the start and end pair it replaced.

## Decorators

Leaves with the `ze:decorate` YANG extension show enriched display text alongside their value. The decorator name in the YANG schema (e.g., `ze:decorate "asn-name"`) maps to a registered `Decorator` implementation that resolves the annotation at render time.

The `DecoratorRegistry` is set on the `Renderer` via `SetDecorators()`. When `RenderField()` or `ResolveDecorations()` runs, each field with a `DecoratorName` is resolved and its `Decoration` is set. `fieldWrapper` renders the decoration in a `ze-field-decoration` span next to the label.

Currently registered: `asn-name` (resolves AS numbers to organization names via Team Cymru DNS TXT queries). Errors are silently ignored for graceful degradation.

<!-- source: internal/component/web/decorator.go -- DecoratorRegistry, Decorator interface -->
<!-- source: internal/component/web/decorator_asn.go -- ASN name decorator, Team Cymru parsing -->
<!-- source: internal/component/config/yang/modules/ze-extensions.yang -- ze:decorate extension -->

## Navigation Flow

All navigation uses HTMX. No full page reloads after initial load.

```
User clicks a Finder column entry
  Browser: hx-get="/fragment/detail?path=bgp/peer" hx-target="#detail"
  Server:  HandleFragment builds FragmentData for path ["bgp","peer"]
  Response: detail HTML (fields)
            + <div class="finder-columns" id="finder" hx-swap-oob="outerHTML">
            + <nav id="breadcrumb" hx-swap-oob="innerHTML"> (updated breadcrumb)
  HTMX:    replaces #detail content, OOB-swaps the Finder columns and breadcrumb
```

The left sidebar is gone. The Finder columns carry navigation now, and
`FragmentData.Sidebar` survives only as the count `detail` reads to choose an
empty state.

## Field Save Flow

Fields save automatically. No submit button. No custom JavaScript.

```
User blurs a text input (or clicks a toggle, or changes a select)
  Browser: hx-post="/config/set/bgp" with leaf=router-id&value=1.2.3.4
  Server:  HandleConfigSet calls EditorManager.SetValue
           Returns OOB commit bar with updated change count (oob_save_ok template)
  HTMX:    OOB-swaps #commit-bar to show "N pending changes"

On error:
  Server:  Returns OOB error item appended to #error-list (oob_error template)
           Opens #error-panel by swapping its class to remove "collapsed"
  HTMX:    OOB-swaps error panel content
```

## Commit Flow

```
User clicks "Review & Commit" in commit bar
  Browser: hx-get="/config/diff" hx-target="#diff-modal" hx-swap="outerHTML"
  Server:  Returns diff_modal_open template (modal with class="open", diff content)
  HTMX:    replaces #diff-modal with open version

User clicks "Confirm Commit" in diff modal
  Browser: hx-post="/config/commit"
  Server:  Calls EditorManager.Commit, returns OOB closed commit bar + closed modal

User clicks "Cancel"
  Browser: hx-get="/config/diff-close" hx-target="#diff-modal" hx-swap="outerHTML"
  Server:  Returns diff_modal template (closed, no content)
```

## Template Dispatch (fieldFor)

The `fieldFor` template function renders a field by dispatching to the correct input template based on the YANG type. No if/else chain in templates.

```
Go field_input.go:
  fieldComponent(FieldMeta{Type:"bool", ...})
    -> fieldInputFor reads the fieldInputs registry and picks inputBool
    -> fieldWrapper draws the label and tooltip around it

Adding a new type:
  1. Create input_<type>.templ with `templ input<Type>(f FieldMeta)`
  2. Add one line to the fieldInputs registry in field_input.go
  3. Add case to valueTypeToFieldType() in fragment.go
```

## Data Types

<!-- source: internal/component/web/fragment.go -- FieldMeta, SidebarSection, FragmentData -->

| Type | Purpose | Used by |
|------|---------|---------|
| `FragmentData` | All data for rendering any page state | HandleFragment |
| `FieldMeta` | YANG metadata for one leaf field | fieldComponent, the input_*.templ editors |
| `FinderColumn` | One column in the Finder navigation (NamedItems, UnnamedItems, or Table) | finder template |
| `ListTableView` | Table view for lists with YANG unique constraints | finder template |
| `ListTableRow` | One entry row in a list table (key + editable cells) | finder template |
| `SidebarSection` | One level of the config hierarchy, with its list entries | `detail`, which reads the count alone |
| `SidebarEntry` | One key in a list section | `SidebarSection.Entries`, which no component reads |
| `ChildEntry` | One navigation link | detail template (legacy) |
| `ErrorData` | One error item | oob_error template |

## Starting the Web Server

Two ways:

| Method | What happens |
|--------|-------------|
| `ze start --web <port>` | Starts web server alongside BGP engine (requires config) |
| `ze start --web-only` | Starts web UI only, no daemon (config editing, default port 3443) |
| `environment { web { } }` in config | Detected during config load, enables web server |

Both paths call `startWebServer()` in `cmd/ze/hub/service_web.go` which wires all routes, creates the EditorManager, CLI completer, and session store.

The caller gives `startWebServer` the local credentials. It does not read them
for itself. `liveLocalUsers` (`cmd/ze/hub/main_servers.go`) is the one producer:
it merges the zefs power user with the users the running configuration declares,
and the AAA chain, the web login and the session check all answer from it. A
second reader could disagree with the first, and a login the chain granted would
then be revoked by the session check on the next request.

## Security

| Aspect | Implementation |
|--------|---------------|
| TLS | Self-signed ECDSA P-256, persisted in zefs, includes all interface IPs as SANs |
| CSP | `script-src 'self'` -- no inline scripts, no unsafe-eval |
| Auth | Session cookie (Secure, HttpOnly, SameSite=Strict) or Basic Auth for API |
| Sessions | 32-byte random token, 24h TTL, one per user, bcrypt password check |
| Session revocation | A session the LOCAL backend granted is re-checked against the running configuration on every request (`SessionStore.ValidateToken`, `internal/component/web/auth.go`). A user an operator deletes and reloads loses the session at once: the TTL is a ceiling, never the only test. A session a RADIUS or TACACS+ backend granted is not anchored to the local list and survives, because that list never declared the operator |
| Paths | YANG identifier validation, path traversal rejected |
