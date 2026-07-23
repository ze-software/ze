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

	"codeberg.org/thomas-mangin/ze/internal/core/stringsx"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
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
	Services         []PortalService
	ActiveUI         string // "workbench", "finder", or "cli" — controls which nav buttons appear
	RouterIdentity   string // Resolved display name: system/host > bgp/router-id > "ze"
	FleetPeers       []FleetPeer
	ChangeCount      int
	// ReadOnly is true when the authenticated user may NOT edit configuration
	// (aaa authorizer denies the edit section). Templates hide edit controls
	// (commit bar, save/add/delete) when set. Default false keeps every
	// existing render path (insecure, single-admin, Finder) fully editable.
	ReadOnly bool
}

// FleetPeer is one entry in the fleet selector dropdown.
type FleetPeer struct {
	Name   string
	URL    string
	Active bool
}

// LoginData holds the data passed to the login template.
type LoginData struct {
	Error    string
	Overlay  bool
	ReturnTo string // URL to redirect to after successful login
	// Locale is the UI locale for translated strings ("en", "fr", ...). Empty
	// renders English (Translate falls back to the English base).
	Locale string
}

// WorkbenchData holds the data passed to the workbench page template. It
// embeds LayoutData so existing fragments (cli_bar, commit_bar, breadcrumb,
// diff_modal, error_panel) render unchanged inside the workbench shell.
type WorkbenchData struct {
	LayoutData
	// Sections drives the workbench left navigation rendering.
	Sections []WorkbenchSection
}

// Renderer loads and renders HTML templates from embedded files.
// Caller MUST use NewRenderer to create an instance; zero value is not usable.
type Renderer struct {
	layout     *template.Template
	workbench  *template.Template
	login      *template.Template
	config     map[string]*template.Template // keyed by template name (e.g., "container.html")
	fragments  *template.Template            // parsed fragment templates (detail, sidebar, pathbar, oob)
	l2tp       map[string]*template.Template // L2TP page templates (list.html, detail.html)
	assets     fs.FS
	decorators *DecoratorRegistry // optional: resolves display-time annotations for decorated leaves
}

// NewRenderer parses all embedded templates and returns a ready Renderer.
// Returns an error if any template fails to parse.
func NewRenderer() (*Renderer, error) {
	funcMap := template.FuncMap{
		"sub":          func(a, b int) int { return a - b },
		"fieldid":      formFieldID,
		"fieldname":    formFieldName,
		"fieldchecked": formFieldChecked,
		"fieldlabel":   humanizeFieldLabel,
		// t translates a key for the given locale with English fallback (i18n.go).
		"t": Translate,
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

	workbench, err := template.New("workbench.html").Funcs(funcMap).ParseFS(templatesFS,
		"templates/page/workbench.html",
		"templates/component/breadcrumb.html",
		"templates/component/cli_bar.html",
		"templates/component/commit_bar.html",
		"templates/component/error_panel.html",
		"templates/component/diff_modal.html",
		"templates/component/workbench_topbar.html",
		"templates/component/workbench_nav.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse workbench template: %w", err)
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
				return textbuf.Join(path, "/")
			}
			return textbuf.Join(path[:upTo+1], "/")
		},
		"splitopts": func(opts string) []string {
			if opts == "" {
				return nil
			}
			parts, _ := stringsx.SplitCount(opts, ",")
			return parts
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
			var tb textbuf.Buffer
			inputName := tb.Str("input_").Str(typeName).String()
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

	// Parse L2TP page templates.
	l2tpTemplateNames := []string{"list.html", "detail.html"}
	l2tpTemplates := make(map[string]*template.Template, len(l2tpTemplateNames))
	for _, name := range l2tpTemplateNames {
		t, parseErr := template.New(name).Funcs(funcMap).ParseFS(
			templatesFS,
			"templates/l2tp/"+name,
		)
		if parseErr != nil {
			return nil, fmt.Errorf("parse l2tp template %s: %w", name, parseErr)
		}
		l2tpTemplates[name] = t
	}

	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return nil, fmt.Errorf("embedded assets sub-fs: %w", err)
	}

	return &Renderer{
		layout:    layout,
		workbench: workbench,
		login:     login,
		config:    configTemplates,
		fragments: fragments,
		l2tp:      l2tpTemplates,
		assets:    assets,
	}, nil
}

// RenderWorkbench renders the workbench page template with the given data.
// Renders to a buffer first to avoid partial writes on template errors.
func (r *Renderer) RenderWorkbench(w http.ResponseWriter, data WorkbenchData) error {
	data.Services = PortalServices()
	var buf bytes.Buffer
	if err := r.workbench.Execute(&buf, data); err != nil {
		return fmt.Errorf("render workbench: %w", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	_, writeErr := buf.WriteTo(w)

	return writeErr
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

// RenderL2TPTemplate renders a named L2TP template to a string.
func (r *Renderer) RenderL2TPTemplate(name string, data any) template.HTML {
	t, ok := r.l2tp[name]
	if !ok {
		serverLogger.Warn("l2tp template not found", "name", name)
		return ""
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		serverLogger.Warn("l2tp template execute", "name", name, "error", err)
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
	data.Services = PortalServices()
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
	fileServer := http.FileServer(http.FS(r.assets))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		fileServer.ServeHTTP(w, req)
	})
}

// FaviconHandler serves the Ze logo as the site favicon. Browsers request
// /favicon.ico automatically on every page; without this route the catch-all
// treated it as a bad /show path and bounced to /show/?error=... on every view
// (F14). Mount unauthenticated at /favicon.ico, like /assets/.
func (r *Renderer) FaviconHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data, err := fs.ReadFile(r.assets, "ze.svg")
		if err != nil {
			http.Error(w, "favicon not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		if _, writeErr := w.Write(data); writeErr != nil {
			return
		}
	})
}
