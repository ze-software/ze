# Pattern: Web Endpoint

Structural template for adding web pages and endpoints to Ze.
Architecture: `docs/architecture/web-interface.md`, `docs/architecture/web-components.md`.

## Also Read

| Rule | When it applies |
|------|----------------|
| `ai/rules/cli.md` | Any API endpoint returning JSON |
| `ai/rules/goroutine-lifecycle.md` | SSE streaming, background workers |
| `ai/rules/evidence.md` | Pages that list or enumerate items |
| `ai/rules/config.md` (Listeners) | If adding a new web listener endpoint |
| Full navigation: `ai/INDEX.md` | |

## Three Web Interfaces

| Interface | Location | Auth | Purpose |
|-----------|----------|------|---------|
| **Config UI** | `internal/component/web/` | Yes (login) | YANG-driven config editor |
| **Looking Glass** | `internal/component/lg/` | Optional (`token`) | Public read-only BGP view |
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

## Component Hierarchy

Every unit is a templ component in `internal/component/web`. A component is a Go
function, so a view-model field the markup misspells is a build failure.

```
page_layout.templ            Document shell (CSS grid, scripts, layout)
page_workbench.templ         Workbench shell
page_login.templ             Login form

component_breadcrumb.templ   Breadcrumb trail + shell toggles
component_sidebar.templ      Left navigation panel
component_detail.templ       Right panel (dispatches to content)
component_cli_bar.templ      CLI prompt + autocomplete
component_commit_bar.templ   Change counter + Review/Discard
component_error_panel.templ  Collapsible error list
component_diff_modal.templ   Commit preview modal
component_oob_response.templ HTMX partial with OOB swaps
component_oob_save.templ     OOB commit bar after save
component_oob_error.templ    OOB error item
component_finder.templ       3-column finder navigation
component_list_table.templ   Multi-row table for lists
component_command_result.templ Admin command result card
component_command_form.templ Admin command parameter form

input_wrapper.templ          Field container + label + tooltip
input_text.templ             input type=text
input_bool.templ             Tristate toggle (yes/default/no)
input_enum.templ             select dropdown
input_number.templ           input type=number
```

**One file = one visual concern.** Adding a new input type is one new `input_<type>.templ` and one line in the `fieldInputs` registry (`field_input.go`).

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

Writes the error fragment (`errorfragment.Render`, `internal/core/errorfragment`)
for the request's target, then appends the same error to `#error-list` through
`component_oob_error.templ` and its `hx-swap-oob`.

A handler with no renderer answers `http.Error` instead, and
`errorfragment.Middleware` turns that plain-text body into the same fragment when
the request carries `HX-Request`. So an endpoint needs no renderer to answer a
refusal the browser can swap. Only a text/plain body is converted: an answer the
handler wrote as html or JSON reaches the client untouched. The middleware is
wrapped around the mux in ONE place per interface: `ServerHandler` (`auth.go`)
here, `NewLGServer` in the looking glass, and `fragmentMux` in the chaos
dashboard.

A message built from an operator value goes through `maskSecretInMessage`
(`secret.go`) when the leaf is in hand. `config.LeafHoldsSecret` is the one
predicate for that question.

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

Third-party assets synced from `third_party/web/` (htmx.min.js v4.0.0-beta6, hx-sse.min.js, ze.svg)
via `scripts/vendor/sync_web.go` (run `make ze-sync-vendor-web`). Never write custom JS shims.

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
| Read-only handler | `internal/component/web/handler_l2tp.go` | Read-only operational view |
| Renderer | `internal/component/web/render.go` | Template loading + fieldFor() |
| Fragment data | `internal/component/web/fragment.go` | FragmentData, FieldMeta structs |
| Looking Glass | `internal/component/lg/handler_ui.go` | Public, read-only, optional bearer token (`auth.go`) |
| Chaos dashboard | `internal/chaos/web/render.go` | No YANG, direct HTML, SSE stream |

## Checklist

```
[ ] Handler file: handler_<concern>.go
[ ] Handler follows 6-step sequence (auth, parse URL, validate, build data, negotiate, render)
[ ] Component(s) in component_<concern>.templ, one visual region each
[ ] If new input type: input_<type>.templ + one line in the fieldInputs registry (field_input.go)
[ ] HTMX OOB swaps for mutation responses (breadcrumb, CLI bar, commit bar)
[ ] Content-Type headers set before writing
[ ] JSON format supported (?format=json)
[ ] Route registered in startup code
[ ] Functional tests in test/web/
```
