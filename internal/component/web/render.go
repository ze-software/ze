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
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/a-h/templ"
)

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

// workbenchData holds the data passed to the workbench page component. It
// embeds LayoutData so the chrome components (topbar, commit bar, diff modal,
// error panel) render unchanged inside the workbench shell.
type workbenchData struct {
	LayoutData
	// Sections drives the workbench left navigation rendering.
	Sections []WorkbenchSection
}

func newWorkbenchData(layout LayoutData, sections []WorkbenchSection) workbenchData {
	return workbenchData{LayoutData: layout, Sections: sections}
}

// Renderer renders the web interface.
//
// Every page, fragment and editor is a templ component. A component is a Go
// function, so a view-model field the markup misspells is a build failure.
//
// The Renderer holds no parsed template. It carries the static assets and the
// decorator registry, and it turns a render error into a response.
//
// Caller MUST use NewRenderer to create an instance; zero value is not usable.
type Renderer struct {
	assets     fs.FS
	decorators *DecoratorRegistry // optional: resolves display-time annotations for decorated leaves
}

// NewRenderer returns a ready Renderer. It returns an error if the embedded
// asset tree cannot be opened.
func NewRenderer() (*Renderer, error) {
	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return nil, fmt.Errorf("embedded assets sub-fs: %w", err)
	}

	return &Renderer{assets: assets}, nil
}

// renderComponent renders a component for a caller that must hold the markup as
// a string. A render error is logged and yields empty markup. The callers are
// page builders that compose HTML into LayoutData.Content, and none of them
// carries an error path.
//
// It is a method because the Renderer is what a page asks to render. That is
// the shape every one of these call sites already had. The receiver carries no
// state the components need: a templ component is a Go function and resolves
// its own markup at compile time.
//
// Prefer RenderWorkbench, RenderLayout or RenderLogin, which return the error.
// This exists for the call sites that cannot carry one.
func (r *Renderer) renderComponent(what string, c templ.Component) template.HTML {
	var buf bytes.Buffer

	if err := c.Render(context.Background(), &buf); err != nil {
		serverLogger.Warn("component render failed", "component", what, "error", err)

		return ""
	}

	return template.HTML(buf.String()) //nolint:gosec // trusted component output
}

// writeComponent renders a component to a buffer and then to the response, so a
// render error leaves no partial page behind.
func writeComponent(w http.ResponseWriter, what string, c templ.Component) error {
	var buf bytes.Buffer

	if err := c.Render(context.Background(), &buf); err != nil {
		return fmt.Errorf("render %s: %w", what, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	_, writeErr := buf.WriteTo(w)

	return writeErr
}

// RenderWorkbench renders the workbench page with the given data.
func (r *Renderer) RenderWorkbench(w http.ResponseWriter, data workbenchData) error {
	data.Services = PortalServices()

	return writeComponent(w, "workbench", pageWorkbench(data))
}

// SetDecorators sets the decorator registry used to resolve display-time
// annotations for leaves with ze:decorate. Optional; nil disables decoration.
// MUST be called before the HTTP server starts serving (not concurrent-safe).
func (r *Renderer) SetDecorators(reg *DecoratorRegistry) {
	r.decorators = reg
}

// ResolveDecorations resolves display-time annotations for all fields.
// Call after building FieldMeta slices and before passing to components.
func (r *Renderer) ResolveDecorations(fields []FieldMeta) {
	if r.decorators == nil {
		return
	}

	for i := range fields {
		r.decorators.resolveField(&fields[i])
	}
}

// RenderField renders a single field (label frame plus editor) for an HTMX
// swap. The editor comes from the input registry (field_input.go), which is the
// one place a field type resolves.
func (r *Renderer) RenderField(field FieldMeta) template.HTML {
	// Resolve decoration if a registry is available.
	if r.decorators != nil {
		r.decorators.resolveField(&field)
	}

	return r.renderComponent("field", fieldComponent(field))
}

// RenderLayout renders the Finder and CLI page chrome to the response writer.
func (r *Renderer) RenderLayout(w http.ResponseWriter, data LayoutData) error {
	data.Services = PortalServices()

	return writeComponent(w, "layout", pageLayout(data))
}

// RenderLogin renders the sign-in page to the response writer.
func (r *Renderer) RenderLogin(w http.ResponseWriter, data LoginData) error {
	return writeComponent(w, "login", pageLogin(data))
}

// RenderDiffModal renders the closed review overlay. The commit bar swaps it in
// when a review ends, so the page keeps its target for the next one.
func (r *Renderer) RenderDiffModal() (template.HTML, error) {
	var buf bytes.Buffer

	if err := diffModal().Render(context.Background(), &buf); err != nil {
		return "", fmt.Errorf("render diff modal: %w", err)
	}

	return template.HTML(buf.String()), nil //nolint:gosec // trusted component output
}

// RenderDiffModalOpen renders the review overlay with the pending diff inside
// it. diff is written into a <pre>, so its line breaks reach the reader.
func (r *Renderer) RenderDiffModalOpen(diff string, changeCount int) (template.HTML, error) {
	var buf bytes.Buffer

	data := commitModalData{Diff: diff, ChangeCount: changeCount}
	if err := diffModalOpen(data).Render(context.Background(), &buf); err != nil {
		return "", fmt.Errorf("render diff modal: %w", err)
	}

	return template.HTML(buf.String()), nil //nolint:gosec // trusted component output
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
