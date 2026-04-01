# Pattern: Web Endpoint

Structural template for adding web pages and endpoints to Ze.
Architecture: `docs/architecture/web-interface.md`, `docs/architecture/web-components.md`.

## Three Web Interfaces

| Interface | Location | Auth | Purpose |
|-----------|----------|------|---------|
| **Config UI** | `internal/component/web/` | Yes (login) | YANG-driven config editor |
| **Looking Glass** | `internal/component/lg/` | No | Public read-only BGP view |
| **Chaos Dashboard** | `internal/chaos/web/` | No | Test simulator UI |

All three use the same pattern: Go HTTP handlers + Go templates + HTMX.

## URL Routing Scheme (Config UI)

```
/show/<yang-path>           GET   Read-only view
/monitor/<yang-path>        GET   Auto-refresh view (5s poll)
/config/<verb>/<yang-path>  POST  Config mutation (requires auth)
/admin/<yang-path>          GET/POST  Admin commands
/cli                        POST  CLI bar command execution
/login                      POST  Authentication
/assets/                    GET   Static files (no auth)
/                           GET   Redirects to /show/
```

Config verbs: `edit`, `set`, `add`, `add-form`, `changes`, `delete`, `commit`, `discard`, `compare`.

Content negotiation: `?format=json` > `Accept: application/json` > HTML.

## Handler Pattern (6 Steps)

**Every handler follows this exact sequence. No exceptions.**

```go
func HandleMyFeature(renderer *Renderer, schema *config.Schema) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Extract username (auth check)
        username := GetUsernameFromRequest(r)
        if username == "" {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        // 2. Parse URL
        parsed, err := ParseURL(r)
        if err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }

        // 3. Validate path segments
        if err := ValidatePathSegments(parsed.Path); err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }

        // 4. Build domain-specific data
        data := buildMyFeatureData(parsed.Path, schema)

        // 5a. JSON response
        if parsed.Format == "json" {
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(data)
            return
        }

        // 5b. HTMX fragment (partial page update)
        if r.Header.Get("HX-Request") == "true" {
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            html := renderer.RenderFragment("myfeature_fragment", data)
            w.Write([]byte(html))
            return
        }

        // 5c. Full page
        content := renderer.RenderFragment("myfeature_content", data)
        renderer.RenderLayout(w, LayoutData{
            Title: "My Feature", Content: content,
            HasSession: true, Username: username,
        })
    }
}
```

## Template Hierarchy

```
templates/
  page/
    layout.html              Document shell (CSS grid, scripts, layout)
    login.html               Login form

  component/
    breadcrumb.html          Breadcrumb trail + CLI toggle
    sidebar.html             Left navigation panel
    detail.html              Right panel (dispatches to content)
    cli_bar.html             CLI prompt + autocomplete
    commit_bar.html          Change counter + Review/Discard
    error_panel.html         Collapsible error list
    diff_modal.html          Commit preview modal
    oob_response.html        HTMX partial with OOB swaps
    oob_save.html            OOB commit bar after save
    oob_error.html           OOB error item
    finder.html              3-column finder navigation
    list_table.html          Multi-row table for lists
    command_result.html      Admin command result card
    command_form.html        Admin command parameter form

  input/
    wrapper.html             Field container + label + tooltip
    text.html                input type=text
    bool.html                Tristate toggle (yes/default/no)
    enum.html                <select> dropdown
    number.html              input type=number
```

**One file = one visual concern.** Adding a new input type = one new file in `input/`.

### Template Naming Convention

| Suffix | Purpose |
|--------|---------|
| `_fragment` | HTMX response (partial + OOB swaps) |
| `_content` | Full page content |
| `_detail` | Nested reusable component |

### Input Type Dispatch

`fieldFor()` in `render.go` dispatches dynamically -- no if/else chains:

```go
inputName := "input_" + field.GetType()  // "input_text", "input_bool", etc.
fragments.ExecuteTemplate(&buf, inputName, field)
```

## HTMX Conventions

### OOB Swaps Are Mandatory

Every mutation response updates multiple DOM elements simultaneously:

```html
<!-- Primary swap target (detail panel) -->
<div class="main-split" hx-target="#content-area">
  {{template "detail" .}}
</div>
<!-- OOB updates (outside primary target) -->
<nav id="breadcrumb" hx-swap-oob="innerHTML">{{template "breadcrumb_inner" .}}</nav>
<div id="cli-path-bar" hx-swap-oob="innerHTML">{{template "path_bar_inner" .}}</div>
```

**Invariant:** When the user clicks a field, 5 things update in one request:
detail panel, breadcrumb, CLI path bar, sidebar (sometimes), commit bar.

### Field Save Pattern

```html
<input type="text" class="ze-field-input"
       hx-post="/config/set/{{.Path}}"
       hx-trigger="blur changed, keyup[key=='Enter'], input changed delay:1s"
       hx-target="closest .ze-field"
       hx-swap="outerHTML"
       hx-vals='{"leaf":"{{.Leaf}}"}'>
```

### Error Handling

```go
WriteOOBError(w, renderer, errPath, err.Error(), http.StatusBadRequest)
```

Renders `oob_error.html` with `hx-swap-oob="true"` to append to `#error-list`.

### Monitor Pages

`/monitor/` URLs auto-refresh. Set `data.Monitor = true` and the template adds:
`hx-get="/monitor/{{.CurrentPath}}" hx-trigger="every 5s"`.

## No Custom JavaScript

The only JS file is `cli.js` (Tab/? key interception in CLI bar).
Everything else uses HTMX attributes. No inline JS in templates.

## Asset Embedding

```go
//go:embed assets
var assetsFS embed.FS
```

Third-party assets synced from `third_party/web/` (htmx.min.js v2.0.4, sse.js, ze.svg)
via `scripts/sync-vendor-web.sh`. Never write custom JS shims.

## Route Registration

Routes are registered in startup code (`cmd/ze/hub/main.go`):

```go
mux.HandleFunc("/myfeature/", authWrap(HandleMyFeature(renderer, schema)))
```

All routes go through the auth dispatcher which calls `ParseURL()` to route by prefix.

## Reference Implementations

| Variant | File | Notes |
|---------|------|-------|
| Config handler | `internal/component/web/handler_config.go` | Full YANG-driven, mutations, OOB |
| Admin handler | `internal/component/web/handler_admin.go` | Simpler: command forms + results |
| Show handler | `internal/component/web/handler_show.go` | Read-only view |
| Renderer | `internal/component/web/render.go` | Template loading + fieldFor() |
| Fragment data | `internal/component/web/fragment.go` | FragmentData, FieldMeta structs |
| Looking Glass | `internal/component/lg/handler_ui.go` | Public, no auth, read-only |
| Chaos dashboard | `internal/chaos/web/render.go` | No YANG, direct HTML, SSE stream |

## Checklist

```
[ ] Handler file: handler_<concern>.go
[ ] Handler follows 6-step sequence (auth, parse URL, validate, build data, negotiate, render)
[ ] Template(s) in templates/component/ with _fragment/_content naming
[ ] If new input type: templates/input/<type>.html + fieldFor() picks it up automatically
[ ] HTMX OOB swaps for mutation responses (breadcrumb, CLI bar, commit bar)
[ ] Content-Type headers set before writing
[ ] JSON format supported (?format=json)
[ ] Route registered in startup code
[ ] Functional tests in test/web/
```
