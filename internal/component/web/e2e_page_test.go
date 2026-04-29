package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

func TestE2E_FragmentHandlerRendersHTMX(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	schema, schemaErr := config.YANGSchema()
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	tree := config.NewTree()

	handler := HandleFragment(renderer, schema, tree, nil, false)

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
	require.NotEmpty(t, result, "oob_response must render for HTMX navigation")
	require.Contains(t, string(result), "group")
	require.Contains(t, string(result), "peer")

	// HTMX partial request (sidebar click)
	req2 := httptest.NewRequest(http.MethodGet, "/fragment/detail?path=bgp", http.NoBody)
	req2.Header.Set("HX-Request", "true")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	html2 := rec2.Body.String()
	t.Logf("\n=== HTMX partial response ===")
	t.Logf("Status: %d, Length: %d", rec2.Code, len(html2))
	require.Equal(t, http.StatusOK, rec2.Code)
	require.NotEmpty(t, html2, "HTMX partial response must not be empty")
	require.Contains(t, html2, "group")
	require.Contains(t, html2, "peer")
	if len(html2) > 2000 {
		t.Logf("First 2000 chars:\n%s", html2[:2000])
	} else {
		t.Logf("Full response:\n%s", html2)
	}
}
