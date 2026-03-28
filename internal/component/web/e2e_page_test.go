package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

func TestE2E_FragmentHandlerRendersHTMX(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	schema := config.YANGSchema()
	tree := config.NewTree()

	handler := HandleFragment(renderer, schema, tree)

	// Full page request to /show/ (root)
	req := httptest.NewRequest(http.MethodGet, "/show/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	html := rec.Body.String()
	t.Logf("Status: %d, Length: %d", rec.Code, len(html))
	t.Logf("Contains htmx.min.js: %v", strings.Contains(html, "htmx.min.js"))
	t.Logf("Contains sidebar: %v", strings.Contains(html, "sidebar"))
	t.Logf("Contains hx-get: %v", strings.Contains(html, "hx-get"))
	t.Logf("Contains id=detail: %v", strings.Contains(html, `id="detail"`))
	t.Logf("Contains main-split: %v", strings.Contains(html, "main-split"))

	if len(html) > 4000 {
		t.Logf("First 4000 chars:\n%s", html[:4000])
	} else {
		t.Logf("Full HTML:\n%s", html)
	}

	// Test RenderFragment directly to see the error
	data := buildFragmentData(schema, tree, []string{"bgp"})
	t.Logf("FragmentData fields: %d, sidebar: %d, breadcrumbs: %d",
		len(data.Fields), len(data.Sidebar), len(data.Breadcrumbs))

	result := renderer.RenderFragment("oob_response", data)
	t.Logf("oob_response result length: %d", len(result))
	if len(result) == 0 {
		// Try rendering sub-templates individually to find the error
		r1 := renderer.RenderFragment("detail", data)
		t.Logf("detail alone: %d bytes", len(r1))
		r2 := renderer.RenderFragment("sidebar", data)
		t.Logf("sidebar alone: %d bytes", len(r2))
		r3 := renderer.RenderFragment("breadcrumb_inner", data)
		t.Logf("breadcrumb_inner alone: %d bytes (THIS is likely the problem)", len(r3))
	}

	// HTMX partial request (sidebar click)
	req2 := httptest.NewRequest(http.MethodGet, "/fragment/detail?path=bgp", http.NoBody)
	req2.Header.Set("HX-Request", "true")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	html2 := rec2.Body.String()
	t.Logf("\n=== HTMX partial response ===")
	t.Logf("Status: %d, Length: %d", rec2.Code, len(html2))
	if len(html2) > 2000 {
		t.Logf("First 2000 chars:\n%s", html2[:2000])
	} else {
		t.Logf("Full response:\n%s", html2)
	}
}
