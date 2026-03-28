# Web Component Architecture

<!-- source: internal/component/web/fragment.go -- HandleFragment, FragmentData -->
<!-- source: internal/component/web/render.go -- Renderer, RenderFragment, fieldFor -->

## Design Principles

The web interface follows three rules:

1. **Server renders HTML, HTMX handles interaction.** No custom JavaScript creates UI elements. All HTML comes from Go templates. HTMX attributes on elements handle save, navigation, and error display. The only JS file (`cli.js`) handles Tab/? key interception for CLI autocomplete, which has no HTMX equivalent.

2. **One template per visual concern.** Each file in `templates/` renders exactly one thing. Adding a new input type means adding one file. The template filesystem mirrors the page structure.

3. **One HTTP request updates multiple components.** HTMX out-of-band (OOB) swaps let a single response update the detail panel, sidebar, breadcrumb, commit bar, and error panel simultaneously.

## Page Layout

```
+--------------------------------------------------+
| #breadcrumb   / > bgp > peer > 1.2.3.4    [CLI] |
+----------+---------------------------------------+
| #sidebar | #detail                               |
|          |                                       |
| <- BACK  |  router-id: [1.2.3.4______]          |
|          |  listen:    [0.0.0.0:179___]          |
| GROUP    |  hold-time: [90_____________]         |
| LOCAL    |                                       |
| PEER     |                                       |
|  1.2.3.4 |                                       |
|  10.0.0.1|                                       |
|  [+add]  |                                       |
| RIB      |                                       |
| RPKI     |                                       |
+----------+---------------------------------------+
| #commit-bar   3 pending changes [Review] [Discard]|
+--------------------------------------------------+
| /> set bgp peer 1.2.3.4 remote-as 65001         |
+--------------------------------------------------+
```

Hidden overlays (shown on demand):
- `#diff-modal` -- diff review with Confirm Commit / Cancel
- `#error-panel` -- collapsible right-side panel for validation errors

## Template Filesystem

```
templates/
  page/                          -- document shells
    layout.html                  -- grid layout, includes all component templates
    login.html                   -- login form

  component/                     -- page sections (one file = one visual region)
    breadcrumb.html              -- breadcrumb_inner: path trail + CLI/GUI toggle
    sidebar.html                 -- sidebar + sidebar_section: back link, headings, entries, add forms
    detail.html                  -- detail: leaf fields via fieldFor(), hint when empty
    cli_bar.html                 -- cli_bar: prompt + input + completions container
    commit_bar.html              -- commit_bar: change count + Review/Discard buttons
    error_panel.html             -- error_panel: collapsible panel with error list
    diff_modal.html              -- diff_modal (closed) + diff_modal_open (with content)
    oob_response.html            -- oob_response: HTMX partial (detail + OOB sidebar/breadcrumb)
                                    full_content: initial page (sidebar + detail)
    oob_save.html                -- oob_save_ok: OOB commit bar after successful save
    oob_error.html               -- oob_error: OOB error item appended to error list

  input/                         -- one file per YANG value type
    wrapper.html                 -- field_wrapper_start/end: label, (i) tooltip, container div
    bool.html                    -- input_bool: toggle button (on/off), hx-post on click
    enum.html                    -- input_enum: <select> dropdown, hx-post on change
    number.html                  -- input_number: <input type=number>, hx-post on blur
    text.html                    -- input_text: <input type=text>, hx-post on blur

  *.html                         -- legacy config templates (container, list, flex, etc.)
```

## Navigation Flow

All navigation uses HTMX. No full page reloads after initial load.

```
User clicks "peer" in sidebar
  Browser: hx-get="/fragment/detail?path=bgp/peer" hx-target="#detail"
  Server:  HandleFragment builds FragmentData for path ["bgp","peer"]
  Response: detail HTML (fields)
            + <aside id="sidebar" hx-swap-oob="innerHTML"> (new sidebar children)
            + <nav id="breadcrumb" hx-swap-oob="innerHTML"> (updated breadcrumb)
  HTMX:    replaces #detail content, OOB-swaps sidebar and breadcrumb
```

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
Go render.go:
  fieldFor(FieldMeta{Type:"bool", ...})
    -> executes "field_wrapper_start" (label + tooltip)
    -> executes "input_bool" (toggle button with hx-post)
    -> executes "field_wrapper_end" (closing div)

Adding a new type:
  1. Create templates/input/<type>.html with {{define "input_<type>"}}
  2. Add case to valueTypeToFieldType() in fragment.go
  3. Done -- fieldFor dispatches automatically
```

## Data Types

<!-- source: internal/component/web/fragment.go -- FieldMeta, SidebarSection, FragmentData -->

| Type | Purpose | Used by |
|------|---------|---------|
| `FragmentData` | All data for rendering any page state | HandleFragment |
| `FieldMeta` | YANG metadata for one leaf field | fieldFor, input templates |
| `SidebarSection` | One heading in the sidebar (with entries for lists) | sidebar template |
| `SidebarEntry` | One key in a list section | sidebar_section template |
| `ChildEntry` | One navigation link | detail template (legacy) |
| `ErrorData` | One error item | oob_error template |

## Starting the Web Server

Two ways:

| Method | What happens |
|--------|-------------|
| `ze start --web <port>` | Starts web server alongside BGP engine (or standalone if no config) |
| `environment { web { } }` in config | Detected during config load, enables web server |

Both paths call `startWebServer()` in `cmd/ze/hub/main.go` which wires all routes, creates the EditorManager, CLI completer, and session store.

## Security

| Aspect | Implementation |
|--------|---------------|
| TLS | Self-signed ECDSA P-256, persisted in zefs, includes all interface IPs as SANs |
| CSP | `script-src 'self'` -- no inline scripts, no unsafe-eval |
| Auth | Session cookie (Secure, HttpOnly, SameSite=Strict) or Basic Auth for API |
| Sessions | 32-byte random token, 24h TTL, one per user, bcrypt password check |
| Paths | YANG identifier validation, path traversal rejected |
