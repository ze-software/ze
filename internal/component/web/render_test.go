package web

import (
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

var inlineHandlerPattern = regexp.MustCompile(`\s(?:on[a-z]+|hx-on(?:::|:)[^=]*)=`)

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

// TestTemplatesAvoidInlineScriptAndStyle verifies templates stay compatible with strict CSP.
// VALIDATES: Templates use external scripts/styles and delegated event handlers.
// PREVENTS: Reintroducing inline JS/CSS that requires unsafe-inline CSP.
func TestTemplatesAvoidInlineScriptAndStyle(t *testing.T) {
	err := fs.WalkDir(templatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		contentBytes, readErr := templatesFS.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		content := string(contentBytes)
		if strings.Contains(content, "<script>") {
			t.Errorf("%s contains inline <script> block", path)
		}
		if strings.Contains(content, "style=") {
			t.Errorf("%s contains inline style attribute", path)
		}
		if inlineHandlerPattern.MatchString(content) {
			t.Errorf("%s contains inline event handler or hx-on attribute", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk templates: %v", err)
	}
}

// TestHandleCLIPageAvoidsInlineStyle verifies generated CLI HTML also obeys strict CSP.
// VALIDATES: Generated HTML uses CSS classes instead of inline style attributes.
// PREVENTS: Strict CSP working for templates but failing on generated fragments.
func TestHandleCLIPageAvoidsInlineStyle(t *testing.T) {
	body := string(HandleCLIPage(nil))
	if strings.Contains(body, "style=") {
		t.Fatalf("HandleCLIPage contains inline style attribute: %s", body)
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

	// CLI bar remains part of the Finder shell; /cli is the full terminal page.
	if !strings.Contains(body, `class="cli-bar"`) {
		t.Error("layout missing cli-bar")
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

// TestRenderDecoratedLeaf verifies that a FieldMeta with Decoration renders the annotation.
// VALIDATES: AC-1, AC-9 -- annotation appears wherever the decorated leaf is rendered.
// PREVENTS: Decoration silently dropped during template rendering.
func TestRenderDecoratedLeaf(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	field := FieldMeta{
		Leaf:          "as",
		Path:          "bgp/peer/upstream/remote",
		Type:          "uint32",
		Value:         "13335",
		Description:   "Peer ASN",
		Min:           "0",
		Max:           "4294967295",
		DecoratorName: "asn-name",
		Decoration:    "Cloudflare, Inc.",
	}

	html := r.RenderField(field)
	body := string(html)

	if !strings.Contains(body, "Cloudflare, Inc.") {
		t.Error("rendered output missing decoration text 'Cloudflare, Inc.'")
	}

	if !strings.Contains(body, "ze-field-decoration") {
		t.Error("rendered output missing ze-field-decoration CSS class")
	}

	// Value should still be present.
	if !strings.Contains(body, `value="13335"`) {
		t.Error("rendered output missing input value")
	}
}

// TestRenderUnDecoratedLeaf verifies that a plain leaf renders without decoration markup.
// VALIDATES: AC-2 -- no regression for undecorated leaves.
// PREVENTS: Empty decoration span appearing on all leaves.
func TestRenderUnDecoratedLeaf(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	field := FieldMeta{
		Leaf:  "router-id",
		Path:  "bgp",
		Type:  "ip",
		Value: "1.2.3.4",
	}

	html := r.RenderField(field)
	body := string(html)

	if strings.Contains(body, "ze-field-decoration") {
		t.Error("undecorated leaf should not contain ze-field-decoration class")
	}

	// Value should be present.
	if !strings.Contains(body, `value="1.2.3.4"`) {
		t.Error("rendered output missing input value")
	}
}

// TestDecoratorRegistryResolveField verifies that ResolveField populates Decoration.
// VALIDATES: AC-1 -- decorator resolution populates FieldMeta.Decoration.
// PREVENTS: Decoration not filled even when decorator is registered.
func TestDecoratorRegistryResolveField(t *testing.T) {
	reg := NewDecoratorRegistry()
	reg.Register(DecoratorFunc("asn-name", func(value string) (string, error) {
		return "Test Org", nil
	}))

	field := FieldMeta{
		Leaf:          "as",
		Value:         "64500",
		DecoratorName: "asn-name",
	}

	reg.ResolveField(&field)
	if field.Decoration != "Test Org" {
		t.Errorf("Decoration = %q, want %q", field.Decoration, "Test Org")
	}

	// No decorator name -- should not change.
	field2 := FieldMeta{Leaf: "port", Value: "179"}
	reg.ResolveField(&field2)
	if field2.Decoration != "" {
		t.Errorf("field without decorator should have empty Decoration, got %q", field2.Decoration)
	}

	// Empty value -- should not call decorator.
	field3 := FieldMeta{Leaf: "as", DecoratorName: "asn-name"}
	reg.ResolveField(&field3)
	if field3.Decoration != "" {
		t.Errorf("field with empty value should have empty Decoration, got %q", field3.Decoration)
	}

	// Unknown decorator name -- should not change. (Finding #15)
	field4 := FieldMeta{Leaf: "as", Value: "64500", DecoratorName: "nonexistent"}
	reg.ResolveField(&field4)
	if field4.Decoration != "" {
		t.Errorf("unknown decorator should have empty Decoration, got %q", field4.Decoration)
	}

	// Decorator returns error -- Decoration should stay empty. (Finding #7)
	reg.Register(DecoratorFunc("failing", func(string) (string, error) {
		return "", errors.New("lookup failed")
	}))
	field5 := FieldMeta{Leaf: "as", Value: "64500", DecoratorName: "failing"}
	reg.ResolveField(&field5)
	if field5.Decoration != "" {
		t.Errorf("failing decorator should have empty Decoration, got %q", field5.Decoration)
	}
}

// TestRenderFieldResolvesDecoration verifies that RenderField calls SetDecorators
// to resolve decoration at render time, not relying on pre-set Decoration.
// VALIDATES: AC-1 -- renderer resolves decoration from registry during RenderField.
// PREVENTS: Decoration only working with pre-set values, not via registry lookup.
func TestRenderFieldResolvesDecoration(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	reg := NewDecoratorRegistry()
	reg.Register(DecoratorFunc("asn-name", func(value string) (string, error) {
		if value == "13335" {
			return "Cloudflare, Inc.", nil
		}
		return "", nil
	}))
	r.SetDecorators(reg)

	field := FieldMeta{
		Leaf:          "as",
		Path:          "bgp/peer/upstream/remote",
		Type:          "uint32",
		Value:         "13335",
		Min:           "0",
		Max:           "4294967295",
		DecoratorName: "asn-name",
		// Decoration intentionally NOT pre-set.
	}

	html := r.RenderField(field)
	body := string(html)

	if !strings.Contains(body, "Cloudflare, Inc.") {
		t.Error("RenderField should resolve decoration via registry, but annotation not found")
	}
}

// TestResolveDecorationsOnRenderer verifies ResolveDecorations on Renderer.
// VALIDATES: AC-9 -- renderer resolves decorations for field slices.
// PREVENTS: ResolveDecorations silently broken or panicking on nil registry.
func TestResolveDecorationsOnRenderer(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	// Nil decorators -- should not panic.
	fields := []FieldMeta{{Leaf: "as", Value: "64500", DecoratorName: "asn-name"}}
	r.ResolveDecorations(fields)
	if fields[0].Decoration != "" {
		t.Error("nil decorator registry should leave Decoration empty")
	}

	// With decorators set.
	reg := NewDecoratorRegistry()
	reg.Register(DecoratorFunc("asn-name", func(string) (string, error) {
		return "Test Org", nil
	}))
	r.SetDecorators(reg)

	fields2 := []FieldMeta{{Leaf: "as", Value: "64500", DecoratorName: "asn-name"}}
	r.ResolveDecorations(fields2)
	if fields2[0].Decoration != "Test Org" {
		t.Errorf("Decoration = %q, want %q", fields2[0].Decoration, "Test Org")
	}
}

// TestDecorationHTMLEscaped verifies that decoration text with HTML is escaped.
// VALIDATES: Security -- XSS prevention in decoration output.
// PREVENTS: Malicious DNS TXT records injecting HTML/JS into rendered page.
func TestDecorationHTMLEscaped(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	field := FieldMeta{
		Leaf:       "as",
		Path:       "bgp",
		Type:       "uint32",
		Value:      "64500",
		Decoration: `<script>alert("xss")</script>`,
	}

	html := r.RenderField(field)
	body := string(html)

	if strings.Contains(body, "<script>") {
		t.Error("decoration with <script> tag should be HTML-escaped, found raw <script>")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("decoration should contain escaped &lt;script&gt;")
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
