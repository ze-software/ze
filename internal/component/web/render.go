// Design: docs/architecture/web-interface.md -- Template rendering
// Related: handler_config.go -- Config tree view handlers
// Related: handler_admin.go -- Admin command handlers
// Related: sse.go -- SSE event rendering
// Detail: decorator.go -- Decorator registry and interface
// Detail: decorator_asn.go -- ASN name decorator via Team Cymru DNS

// Package web provides the ze web interface with template rendering and static assets.
package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed templates
var templatesFS embed.FS

//go:embed assets
var assetsFS embed.FS

// BreadcrumbSegment represents one segment in the breadcrumb navigation.
type BreadcrumbSegment struct {
	Name   string
	URL    string
	Active bool
}

// LayoutData holds the data passed to the layout template.
type LayoutData struct {
	Title            string
	Content          template.HTML
	Breadcrumbs      []BreadcrumbSegment
	NotificationHTML template.HTML
	CLIPrompt        string
	CLIContextPath   string        // Slash-separated YANG path for hidden context tracking
	CLIPathBar       template.HTML // Pre-built path bar HTML with clickable segments
	HasSession       bool
	Username         string
	Insecure         bool
}

// LoginData holds the data passed to the login template.
type LoginData struct {
	Error   string
	Overlay bool
}

// Renderer loads and renders HTML templates from embedded files.
// Caller MUST use NewRenderer to create an instance; zero value is not usable.
type Renderer struct {
	layout     *template.Template
	login      *template.Template
	config     map[string]*template.Template // keyed by template name (e.g., "container.html")
	fragments  *template.Template            // parsed fragment templates (detail, sidebar, pathbar, oob)
	assets     fs.FS
	decorators *DecoratorRegistry // optional: resolves display-time annotations for decorated leaves
}

// NewRenderer parses all embedded templates and returns a ready Renderer.
// Returns an error if any template fails to parse.
func NewRenderer() (*Renderer, error) {
	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
	}

	layout, err := template.New("layout.html").Funcs(funcMap).ParseFS(templatesFS,
		"templates/page/layout.html",
		"templates/component/breadcrumb.html",
		"templates/component/cli_bar.html",
		"templates/component/commit_bar.html",
		"templates/component/error_panel.html",
		"templates/component/diff_modal.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse layout template: %w", err)
	}

	login, err := template.New("login.html").Funcs(funcMap).ParseFS(templatesFS, "templates/page/login.html")
	if err != nil {
		return nil, fmt.Errorf("parse login template: %w", err)
	}

	// Parse config view templates. Each includes the leaf_input partial.
	configTemplateNames := []string{
		"container.html",
		"list.html",
		"flex.html",
		"freeform.html",
		"inline_list.html",
		"breadcrumb.html",
		"commit.html",
		"notification.html",
		"command.html",
		"command_form.html",
	}

	configTemplates := make(map[string]*template.Template, len(configTemplateNames))

	for _, name := range configTemplateNames {
		t, parseErr := template.New(name).Funcs(funcMap).ParseFS(
			templatesFS,
			"templates/"+name,
			"templates/leaf_input.html",
		)
		if parseErr != nil {
			return nil, fmt.Errorf("parse config template %s: %w", name, parseErr)
		}

		configTemplates[name] = t
	}

	// Parse fragment templates together so they can reference each other.
	// Each input type is a separate file, dispatched by fieldFor() at render time.
	var fragments *template.Template
	fragFuncs := template.FuncMap{
		"joinpath": func(path []string, upTo int) string {
			if upTo >= len(path) {
				return strings.Join(path, "/")
			}
			return strings.Join(path[:upTo+1], "/")
		},
		"splitopts": func(opts string) []string {
			if opts == "" {
				return nil
			}
			return strings.Split(opts, ",")
		},
		"fieldFor": func(f any) template.HTML {
			// Render: wrapper_start + input_<type> + wrapper_end.
			// Dispatches to the right input template based on FieldMeta.Type.
			type typer interface{ GetType() string }
			typeName := "text"
			if ft, ok := f.(typer); ok {
				typeName = ft.GetType()
			}
			var buf bytes.Buffer
			if err := fragments.ExecuteTemplate(&buf, "field_wrapper_start", f); err != nil {
				return ""
			}
			inputName := "input_" + typeName
			if err := fragments.ExecuteTemplate(&buf, inputName, f); err != nil {
				// Fall back to text input for unknown types.
				_ = fragments.ExecuteTemplate(&buf, "input_text", f)
			}
			if err := fragments.ExecuteTemplate(&buf, "field_wrapper_end", f); err != nil {
				return ""
			}
			return template.HTML(buf.String()) //nolint:gosec // trusted template output
		},
	}
	fragments, err = template.New("fragments").Funcs(funcMap).Funcs(fragFuncs).ParseFS(templatesFS,
		"templates/component/*.html",
		"templates/input/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse fragment templates: %w", err)
	}

	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return nil, fmt.Errorf("embedded assets sub-fs: %w", err)
	}

	return &Renderer{
		layout:    layout,
		login:     login,
		config:    configTemplates,
		fragments: fragments,
		assets:    assets,
	}, nil
}

// RenderFragment renders a named fragment template to a string.
// Used for composing page content and HTMX partial responses.
func (r *Renderer) RenderFragment(name string, data any) template.HTML {
	var buf bytes.Buffer
	if err := r.fragments.ExecuteTemplate(&buf, name, data); err != nil {
		return ""
	}
	return template.HTML(buf.String()) //nolint:gosec // trusted template output
}

// SetDecorators sets the decorator registry used to resolve display-time
// annotations for leaves with ze:decorate. Optional; nil disables decoration.
// MUST be called before the HTTP server starts serving (not concurrent-safe).
func (r *Renderer) SetDecorators(reg *DecoratorRegistry) {
	r.decorators = reg
}

// ResolveDecorations resolves display-time annotations for all fields.
// Call after building FieldMeta slices and before passing to templates.
func (r *Renderer) ResolveDecorations(fields []FieldMeta) {
	if r.decorators == nil {
		return
	}

	for i := range fields {
		r.decorators.ResolveField(&fields[i])
	}
}

// RenderField renders a single field (wrapper + input + badge) using the
// fragment templates directly. Returns the full field HTML for HTMX swap.
func (r *Renderer) RenderField(field FieldMeta) template.HTML {
	// Resolve decoration if a registry is available.
	if r.decorators != nil {
		r.decorators.ResolveField(&field)
	}

	var buf bytes.Buffer

	if err := r.fragments.ExecuteTemplate(&buf, "field_wrapper_start", field); err != nil {
		return ""
	}

	inputName := "input_" + field.GetType()
	if err := r.fragments.ExecuteTemplate(&buf, inputName, field); err != nil {
		// Fall back to text input for unknown types.
		if err2 := r.fragments.ExecuteTemplate(&buf, "input_text", field); err2 != nil {
			return ""
		}
	}

	if err := r.fragments.ExecuteTemplate(&buf, "field_wrapper_end", field); err != nil {
		return ""
	}

	return template.HTML(buf.String()) //nolint:gosec // trusted template output
}

// RenderConfigTemplate renders a config view template by name with the given data.
// The name should match a config template (e.g., "container.html", "list.html").
// Renders to a buffer first to avoid partial writes on template errors.
func (r *Renderer) RenderConfigTemplate(w http.ResponseWriter, name string, data any) error {
	t, ok := r.config[name]
	if !ok {
		return fmt.Errorf("unknown config template: %s", name)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("render config template %s: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	_, writeErr := buf.WriteTo(w)

	return writeErr
}

// RenderConfigToHTML renders a config template to an HTML string for embedding
// in the layout's Content field. Returns empty HTML on error.
func (r *Renderer) RenderConfigToHTML(name string, data any) template.HTML {
	t, ok := r.config[name]
	if !ok {
		return ""
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return ""
	}

	return template.HTML(buf.String()) //nolint:gosec // trusted template output
}

// RenderLayout renders the layout template with the given data to the response writer.
// Renders to a buffer first to avoid partial writes on template errors.
func (r *Renderer) RenderLayout(w http.ResponseWriter, data LayoutData) error {
	var buf bytes.Buffer
	if err := r.layout.Execute(&buf, data); err != nil {
		return fmt.Errorf("render layout: %w", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	_, writeErr := buf.WriteTo(w)

	return writeErr
}

// RenderLogin renders the login template with the given data to the response writer.
// Renders to a buffer first to avoid partial writes on template errors.
func (r *Renderer) RenderLogin(w http.ResponseWriter, data LoginData) error {
	var buf bytes.Buffer
	if err := r.login.Execute(&buf, data); err != nil {
		return fmt.Errorf("render login: %w", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	_, writeErr := buf.WriteTo(w)

	return writeErr
}

// AssetHandler returns an http.Handler that serves embedded static assets.
// Mount at /assets/ with http.StripPrefix. Assets use no-cache so browsers
// pick up changes after binary updates without requiring a hard refresh.
func (r *Renderer) AssetHandler() http.Handler {
	fs := http.FileServer(http.FS(r.assets))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		fs.ServeHTTP(w, req)
	})
}
