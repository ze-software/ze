// Design: docs/architecture/web-interface.md -- Template rendering
// Related: handler_config.go -- Config tree view handlers
// Related: handler_admin.go -- Admin command handlers
// Related: sse.go -- SSE event rendering

// Package web provides the ze web interface with template rendering and static assets.
package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
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
	Breadcrumb       []BreadcrumbSegment
	NotificationHTML template.HTML
	CLIPrompt        string
	HasSession       bool
}

// LoginData holds the data passed to the login template.
type LoginData struct {
	Error   string
	Overlay bool
}

// Renderer loads and renders HTML templates from embedded files.
// Caller MUST use NewRenderer to create an instance; zero value is not usable.
type Renderer struct {
	layout *template.Template
	login  *template.Template
	config map[string]*template.Template // keyed by template name (e.g., "container.html")
	assets fs.FS
}

// NewRenderer parses all embedded templates and returns a ready Renderer.
// Returns an error if any template fails to parse.
func NewRenderer() (*Renderer, error) {
	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
	}

	layout, err := template.New("layout.html").Funcs(funcMap).ParseFS(templatesFS, "templates/layout.html")
	if err != nil {
		return nil, fmt.Errorf("parse layout template: %w", err)
	}

	login, err := template.New("login.html").Funcs(funcMap).ParseFS(templatesFS, "templates/login.html")
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

	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return nil, fmt.Errorf("embedded assets sub-fs: %w", err)
	}

	return &Renderer{
		layout: layout,
		login:  login,
		config: configTemplates,
		assets: assets,
	}, nil
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
// Mount at /assets/ with http.StripPrefix.
func (r *Renderer) AssetHandler() http.Handler {
	return http.FileServer(http.FS(r.assets))
}
