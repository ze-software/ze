// Design: docs/architecture/web-interface.md -- Template rendering

// Package web provides the ze web interface with template rendering and static assets.
package web

import (
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

	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return nil, fmt.Errorf("embedded assets sub-fs: %w", err)
	}

	return &Renderer{
		layout: layout,
		login:  login,
		assets: assets,
	}, nil
}

// RenderLayout renders the layout template with the given data to the response writer.
func (r *Renderer) RenderLayout(w http.ResponseWriter, data LayoutData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := r.layout.Execute(w, data); err != nil {
		return fmt.Errorf("render layout: %w", err)
	}

	return nil
}

// RenderLogin renders the login template with the given data to the response writer.
func (r *Renderer) RenderLogin(w http.ResponseWriter, data LoginData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := r.login.Execute(w, data); err != nil {
		return fmt.Errorf("render login: %w", err)
	}

	return nil
}

// AssetHandler returns an http.Handler that serves embedded static assets.
// Mount at /assets/ with http.StripPrefix.
func (r *Renderer) AssetHandler() http.Handler {
	return http.FileServer(http.FS(r.assets))
}
