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

`internal/le/web/assets.Write` reads the `.templ` sources of
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

<!-- source: internal/le/web/assets/actions.go -- Answer -->
<!-- source: internal/component/lg/layout.templ -- pageAssets(v.Page), the one shell -->

Two checks hold the set honest, from opposite directions, and neither is relaxed
to make the other pass.

| Check | Reads | Error direction |
|-------|-------|-----------------|
| `./le web assets check` | the sources | OVER-approximates: a branch no request takes still contributes its asset |
| `TestPageImportsCoverRenderedAttributes` (web), `TestLGPageImportsCoverRenderedAttributes`, `TestChaosPageImportsCoverRenderedAttributes` | the captured fixtures | UNDER-approximates: a branch no fixture exercises is invisible |

<!-- source: internal/test/markupcheck/head.go -- HeadCoverageFindings, the fixture side -->

## Component Filesystem

Every unit below is a templ component in `internal/component/web`. The file name
carries the visual concern, and `./le repo update` writes a `*_templ.go` beside
each source.

```
page_layout.templ                -- pageLayout: Finder grid layout
page_workbench.templ             -- pageWorkbench: experimental workbench shell
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
component_graphical_values.templ -- graphicalValues, graphicalInlineList: collection rows and inline-list entry navigation

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
<!-- source: internal/test/markupcheck/markupcheck.go -- AssertNoMarkup, scan -->

`fieldWrapper` takes the editor as a component, so the frame is one balanced
element rather than the start and end pair it replaced.

Its label names the editor through `for`, and the id it uses is the editor's
own: `fieldInputID` derives one from the whole escaped config path (`view.go`).
The reversible DOM encoding keeps ordinary IDs stable and escapes punctuation
and percent escapes so HTMX can address keys containing slashes, spaces or
Unicode without selector collisions. Related-tool overlays use the same
encoding with their tool and context tokens. The
wrapper `div` carries `field-<leaf>` instead, and that id belongs to no
control. A `for` over the wrapper binds the label to an element that holds no
value.

The binding is what gives the editor an accessible name. An unbound editor
falls back to its placeholder, which is the leaf's default where the schema
supplies one, so the field announces itself as a value.

The name is the leaf name alone. The decorative (i) badge and the description
both sit inside the label, so both are `aria-hidden`, and the editor names the
description through `aria-describedby` instead (`fieldDescribedBy`, `view.go`).
A node a `describedby` directly names is read even when it is hidden
(accname-1.2, step 2A), which is what lets one element be the tooltip a sighted
operator hovers and the description a reader hears. The tooltip is a real
element for that reason: it used to be `::after` content on the badge, and
generated content has no id to name.

Three tests in `markup_contract_test.go` hold the whole contract.
`TestCapturedLabelsNameAnIDTheSameDocumentCarries` walks every capture and holds
each `for` to an id that capture writes. `TestEveryLeafEditorIsNamedByItsLabel`
walks the `fieldInputs` registry, so an editor added later cannot ship without
either binding. `TestALeafEditorIsNamedByItsLeafAlone` strips the hidden spans
and reads what is left of the label.

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
  Server:  Returns the error fragment for the field, then one OOB error item
           appended to #error-list (oob_error template). No htmx swap can set
           a class, so the server does not open the panel
  HTMX:    swaps the fragment into the field's target, appends the item
  cli.js:  syncErrorPanel, on htmx:after:settle over #error-list, removes
           "collapsed" from #error-panel while the list holds an item
```

A text or number field saves on three triggers: blur, Enter (`ze-enter`, sent
by `initEnterSubmit` in `cli.js`) and a one-second debounce after typing. Each
is its own htmx listener, and Enter does not cancel the pending debounce. The
field carries `hx-sync="this:drop"` (`fieldSaveSync`, `view.go`), so a save it
issues while its own save is in flight is dropped rather than queued: one edit
reaches the editor once, and a refused value lists one error, not two. Every
answer replaces the field with what the editor now holds, so the operator sees
the outcome of the save that was sent. Text and number editors, including
inline table cells, use a zero settle delay for these replacements. HTMX still
restores focus and the selection by the stable input id, but does not first copy
old attributes onto the new input: its normal settle step could otherwise leave
the focused input's live value behind its updated value attribute, letting the
next blur submit the old value again.
<!-- source: internal/component/web/input_text.templ -- inputText -->
<!-- source: internal/component/web/input_number.templ -- inputNumber -->
<!-- source: internal/component/web/component_list_table.templ -- listTable -->

With no errors the panel carries `collapsed`, which `style.css` renders fully
off screen (`translateX(100%)`, `pointer-events: none`), so it covers nothing.
Its Errors toggle sits inside the panel: it closes an open panel, and dismissing
the last error collapses it again. While `#error-list` holds an item,
`syncErrorPanel` sets `data-count` on the toggle, which shows the count. A
panel the operator closed with errors still listed keeps that toggle on screen:
`style.css` hangs it off the panel's left edge, so it sits on the viewport's
right edge as a tab at mid height, clear of the top bar's CLI button and
breadcrumb actions, and clicking it reopens the panel. With no errors the toggle
carries no `data-count` and the collapsed panel shows nothing.

## Commit Flow

The graphical YANG editor renders multi-word leaves, bracket leaf-lists and
value-or-array leaf-lists as rows. Each row has an input or schema enum selector;
Add, Remove, Move up, Move down, Save and Clear submit through the existing
authorized `/config/set/` endpoint. Row order, permitted repeated members and
leaf-list inactive markers survive the save. Multi-word leaves support activation
of the whole setting through Configuration actions; their words have no separate
activation controls. An invalid member refuses the entire save and leaves the
current form available for correction. The HTMX refusal disables the main swap
with `HX-Reswap: none`, while the error panel receives the out-of-band error.
Freeform blocks expose entry names and values; inline lists expose entry links
and a typed key form. Read-only views withhold mutation controls, and the normal
set/add authorization gates still decide every submitted request.

`buildGraphicalValues` and `populateInlineList` (`graphical_values.go`) read the
candidate tree. `replaceGraphicalValues` (`handler_graphical_values.go`) holds
the user's editor lock while it clones the candidate, changes the selected
typed node and passes set-format serialization to the shared editor's atomic
`load terminal absolute replace` operation. The configuration parser owns the
text grammar; the web adapter never parses configuration text. An absent parent
is created by parsing one quoted set statement for the first requested row.

<!-- source: internal/component/web/graphical_values.go -- buildGraphicalValues, populateInlineList -->
<!-- source: internal/component/web/handler_graphical_values.go -- replaceGraphicalValues, graphicalSeed -->

Form refusals state the problem, explain why the action did not run, and end
with the browser action that resumes it. Authentication, form decoding, entry
addressing and candidate storage failures retain their HTTP status. A schema
refusal that already supplies a typed correction keeps that correction rather
than being replaced with a general retry instruction. A collection refusal
keeps the editable rows available for correction.
<!-- source: internal/component/web/graphical_actions.go -- HandleGraphicalAction -->
<!-- source: internal/component/web/handler_graphical_values.go -- handleGraphicalValues -->
<!-- source: internal/component/web/handler_inline_list.go -- handleInlineListAdd -->
<!-- source: internal/component/web/handler_config_entry.go -- HandleConfigAddWithAuthorizer, HandleConfigRenameWithAuthorizer -->
<!-- source: internal/component/web/handler_config_commit.go -- HandleConfigDiscardWithAuthorizerAndAudit -->

The YANG editor's **Configuration actions** panel exposes the shared editor's
copy, rename, activation, draft, history, validation and confirmed-commit
operations. Entry tools appear only at a named list entry. A copy or rename
refreshes the parent list so its entries agree with the candidate. History
offers a restore button beside each saved revision; restoration changes the
candidate and still requires commit. Root-level actions remain available when
there are no pending changes. Read-only views omit mutation controls.

<!-- source: internal/component/web/graphical_actions.go -- buildGraphicalActions, HandleGraphicalAction -->

Buttons post structured form fields to `/config/action`. The adapter checks
the path and form bounds, quotes each value through the command lexer, and
passes the resulting configuration command to `executeTerminalCommand` after
the same authorization check as the browser CLI. Commit, rollback and discard
reuse the terminal's audit decision. The confirmed-commit window stays owned
by the daemon; its owner, deadline and refusal are shown through the existing
window notices and the action response. No separate graphical transaction
engine or history store exists.

<!-- source: internal/component/web/graphical_actions.go -- graphicalActionCommand, HandleGraphicalAction -->
<!-- source: internal/component/web/window_notices.go -- WindowNotices -->

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

## Markup guards

`ai/rules/architecture.md` states the obligations: markup lives in a `.templ`
file, a component takes a named struct, and a page carries no inline script,
style attribute or event handler. Each obligation has one guard, and each guard
walks its own directory, so a guard naming one package never covers its
sibling.

| Guard | Where the rules live | What it refuses |
|---|---|---|
| `TestNoGoFileBuildsMarkup` | `internal/test/markupcheck`, `AssertNoMarkup` | A Go string literal that builds a tag. It reads the FORM of a tag, so `usage: set <leaf>` is not a finding, and it knows HTML's void elements, so a bare `<br>` is one. Direct operator text follows the contract below. An exemption that explains nothing is a finding, and so is a table that changed size |
| `TestTemplatesAvoidInlineScriptAndStyle` | `internal/test/markupcheck`, `AssertNoInlineScriptOrStyle` | An inline `<script>` block, an inline `style=`, an `on*` handler, an `hx-on` attribute |
| `TestTemplAssetsResolve` | `internal/test/markupcheck`, `AssertAssetsResolve` | A `src` or `href` the served filesystem does not hold, and one naming an asset tree the package does not serve |
| `TestWebViewDataIsTyped`, `TestLGViewDataIsTyped` | `internal/test/templcheck`, `AssertTyped` | A component parameter that is a map, a named map, a bare `any`, or a struct wrapping any of them |
| `./le doc check templ-output` | `internal/le/doc/check` and `internal/le/doc/wiring` | A `*_templ.go` its `.templ` source no longer produces |
| `go test ./internal/component/web ./internal/component/lg` | `internal/test/golden` and the package capture tests | A rendered byte that moved with no fixture behind it |

<!-- source: internal/test/markupcheck/markupcheck.go -- AssertNoMarkup -->
<!-- source: internal/test/markupcheck/inline.go -- AssertNoInlineScriptOrStyle -->
<!-- source: internal/test/markupcheck/assets.go -- AssertAssetsResolve -->
<!-- source: internal/test/templcheck/templcheck.go -- AssertTyped -->

For candidate review, golden capture tests accept `-update-golden -golden-output
<directory>`. The capture mirrors each fixture path under that directory and
leaves the original fixture unchanged. Without `-update-golden`, comparison
still reads the original fixture. Directory coverage still checks the original
fixtures, so new cases report missing fixtures after their candidate bytes are
written. Run capture through the named `./le job run` action with the package's
feature tags, as described in `docs/contributing/running-commands.md`.
<!-- source: internal/test/golden/golden.go -- Compare -->
<!-- source: internal/test/golden/names.go -- AssertCoversDir -->

Direct string literals and concatenations in `opererr.New` and `opererr.Wrap`
text arguments follow their plain-text contract, so multiword command
placeholders stay text. The guard resolves the canonical import and its alias,
and a local binding cannot use that exception. Other literals, including nested
builders and tags with boolean attributes, keep the ordinary markup check.
If the Go source does not parse, every literal keeps the lexical check. All
literals still count toward the scan floor.
<!-- source: internal/test/markupcheck/markupcheck.go -- operatorTextLiterals, operatorTextArgs, markOperatorText, scan -->
<!-- source: internal/core/opererr/opererr.go -- New, Wrap, Error -->

## Security

| Aspect | Implementation |
|--------|---------------|
| TLS | Self-signed ECDSA P-256, persisted in zefs, includes all interface IPs as SANs |
| CSP | `script-src 'self'` -- no inline scripts, no unsafe-eval |
| Auth | Session cookie (Secure, HttpOnly, SameSite=Strict) or Basic Auth for API |
| Sessions | 32-byte random token, 24h TTL, one per user, bcrypt password check |
| Session revocation | A session the LOCAL backend granted is re-checked against the running configuration on every request (`SessionStore.ValidateToken`, `internal/component/web/auth.go`). A user an operator deletes and reloads loses the session at once: the TTL is a ceiling, never the only test. A session a RADIUS or TACACS+ backend granted is not anchored to the local list and survives, because that list never declared the operator |
| Paths | YANG identifier validation, path traversal rejected |
