package web

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewRenderer verifies that NewRenderer returns a non-nil renderer with no error.
// VALIDATES: All embedded templates are valid html/template syntax.
// PREVENTS: Typo in template causes runtime crash.
func TestNewRenderer(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	if r == nil {
		t.Fatal("NewRenderer() returned nil renderer")
	}

	if r.layout == nil {
		t.Error("renderer layout template is nil")
	}

	if r.login == nil {
		t.Error("renderer login template is nil")
	}

	if r.assets == nil {
		t.Error("renderer assets filesystem is nil")
	}
}

// TestRenderLayout verifies that the layout template renders all four areas:
// breadcrumb navigation, content, notification area, and CLI bar with prompt.
// VALIDATES: AC-11 (layout has four areas).
func TestRenderLayout(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := LayoutData{
		Title: "Test Page",
		Breadcrumbs: []BreadcrumbSegment{
			{Name: "Home", URL: "/", Active: false},
			{Name: "BGP", URL: "/bgp", Active: false},
			{Name: "Peers", URL: "/bgp/peers", Active: true},
		},
		Content:          template.HTML("<p>test-content-marker</p>"),
		NotificationHTML: template.HTML("<span>notification-marker</span>"),
		CLIPrompt:        "ze>",
		HasSession:       true,
	}

	rec := httptest.NewRecorder()

	if err := r.RenderLayout(rec, data); err != nil {
		t.Fatalf("RenderLayout() error: %v", err)
	}

	body := rec.Body.String()

	// Breadcrumb links
	if !strings.Contains(body, `class="breadcrumb-bar"`) {
		t.Error("output missing breadcrumb-bar")
	}

	if !strings.Contains(body, `<a href="/">Home</a>`) {
		t.Error("output missing breadcrumb link to Home")
	}

	if !strings.Contains(body, `<a href="/bgp">BGP</a>`) {
		t.Error("output missing breadcrumb link to BGP")
	}

	if !strings.Contains(body, `<span>Peers</span>`) {
		t.Error("output missing active breadcrumb segment Peers")
	}

	// Back arrow (present when >1 breadcrumb segment)
	if !strings.Contains(body, `class="breadcrumb-back"`) {
		t.Error("output missing breadcrumb back arrow")
	}

	// Content area
	if !strings.Contains(body, "test-content-marker") {
		t.Error("output missing content")
	}

	if !strings.Contains(body, `class="content-area"`) {
		t.Error("output missing content-area class")
	}

	// Notification area
	if !strings.Contains(body, "notification-marker") {
		t.Error("output missing notification content")
	}

	if !strings.Contains(body, `class="notification-bar"`) {
		t.Error("output missing notification-bar class")
	}

	// CLI bar with prompt
	if !strings.Contains(body, `class="cli-bar"`) {
		t.Error("output missing cli-bar class")
	}

	// html/template escapes ">" to "&gt;" inside text nodes
	if !strings.Contains(body, "ze&gt;") {
		t.Error("output missing CLI prompt text")
	}

	if !strings.Contains(body, `class="cli-input"`) {
		t.Error("output missing cli-input")
	}

	// Theme toggle
	if !strings.Contains(body, `id="theme-toggle"`) {
		t.Error("output missing theme toggle button")
	}

	// Content-Type header
	ct := rec.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/html; charset=utf-8")
	}
}

// TestRenderLogin verifies that the login page renders a form with username and
// password fields and a POST action to /login.
// VALIDATES: Login page renders correctly.
func TestRenderLogin(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := LoginData{
		Error:   "",
		Overlay: false,
	}

	rec := httptest.NewRecorder()

	if err := r.RenderLogin(rec, data); err != nil {
		t.Fatalf("RenderLogin() error: %v", err)
	}

	body := rec.Body.String()

	// Full page login (not overlay)
	if !strings.Contains(body, `class="login-page"`) {
		t.Error("output missing login-page class for non-overlay login")
	}

	// Form with POST to /login
	if !strings.Contains(body, `method="POST"`) {
		t.Error("output missing POST method on form")
	}

	if !strings.Contains(body, `action="/login"`) {
		t.Error("output missing form action /login")
	}

	// Username field
	if !strings.Contains(body, `name="username"`) {
		t.Error("output missing username input field")
	}

	// Password field
	if !strings.Contains(body, `name="password"`) {
		t.Error("output missing password input field")
	}

	// Submit button
	if !strings.Contains(body, `class="login-submit"`) {
		t.Error("output missing login submit button")
	}

	// No overlay elements
	if strings.Contains(body, `class="login-overlay"`) {
		t.Error("non-overlay login should not contain login-overlay class")
	}

	// Content-Type header
	ct := rec.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/html; charset=utf-8")
	}
}

// TestRenderLoginOverlay verifies that the login overlay includes a dismiss button
// and the overlay container class.
// VALIDATES: AC-10 (login overlay is dismissible).
func TestRenderLoginOverlay(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := LoginData{
		Error:   "",
		Overlay: true,
	}

	rec := httptest.NewRecorder()

	if err := r.RenderLogin(rec, data); err != nil {
		t.Fatalf("RenderLogin() error: %v", err)
	}

	body := rec.Body.String()

	// Overlay container
	if !strings.Contains(body, `class="login-overlay"`) {
		t.Error("overlay login missing login-overlay class")
	}

	// Dismiss/close button
	if !strings.Contains(body, `class="login-close"`) {
		t.Error("overlay login missing dismiss button (login-close)")
	}

	// Form still present inside overlay
	if !strings.Contains(body, `action="/login"`) {
		t.Error("overlay login missing form action /login")
	}

	if !strings.Contains(body, `name="username"`) {
		t.Error("overlay login missing username field")
	}

	if !strings.Contains(body, `name="password"`) {
		t.Error("overlay login missing password field")
	}

	// Should NOT contain full-page login class
	if strings.Contains(body, `class="login-page"`) {
		t.Error("overlay login should not contain login-page class")
	}
}

// TestAssetHandler verifies that embedded static assets are served correctly.
// VALIDATES: Embedded assets are served.
func TestAssetHandler(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	handler := r.AssetHandler()

	req := httptest.NewRequest(http.MethodGet, "/style.css", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /style.css status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("GET /style.css returned empty body")
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Errorf("GET /style.css Content-Type = %q, want text/css", ct)
	}
}
