package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestWorkbench_InterfacesPageDispatch verifies the workbench handler renders
// the interface table page for /show/iface/ instead of the YANG detail view.
// No iface backend is loaded, so the page shows an empty table gracefully.
func TestWorkbench_InterfacesPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/iface/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "wb-table", "must render workbench table component")
	assert.Contains(t, html, "No interfaces found", "empty table must show empty message")
	assert.Contains(t, html, `id="workbench-shell"`, "must be inside workbench shell")
}

// TestWorkbench_InterfacesFilteredDispatch verifies the filtered interface view.
// With no backend loaded, the table is empty but the empty message reflects
// the filter type. The page renders inside the workbench shell with the
// Interfaces section selected in the left nav.
func TestWorkbench_InterfacesFilteredDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/iface/?type=ethernet", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	// Empty message mentions the filter type.
	assert.Contains(t, html, "ethernet", "must mention filtered type in empty message")
	// Must still be inside workbench shell.
	assert.Contains(t, html, `id="workbench-shell"`)
	// Interfaces section must be selected in nav.
	assert.Contains(t, html, `data-section="interfaces"`)
}

// TestWorkbench_TrafficPageDispatch verifies the traffic page renders.
func TestWorkbench_TrafficPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/iface/traffic/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "No interfaces to monitor", "must render traffic empty message")
	assert.Contains(t, html, "wb-table", "must render workbench table component")
}

// TestWorkbench_AddressesPageDispatch verifies the addresses page renders.
func TestWorkbench_AddressesPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/ip/addresses/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "No IP addresses", "must render addresses empty message")
	assert.Contains(t, html, "wb-table", "must render workbench table component")
}

// TestWorkbench_RoutesPageDispatch verifies the routes page renders.
func TestWorkbench_RoutesPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/ip/routes/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "No routes", "must render routes empty message")
	assert.Contains(t, html, "wb-table", "must render workbench table component")
}

// TestWorkbench_DNSPageDispatch verifies the DNS form page renders.
func TestWorkbench_DNSPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/ip/dns/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "DNS Configuration", "must render DNS form")
	assert.Contains(t, html, "wb-form", "must use workbench form component")
}

// TestWorkbench_HTMXPartialForInterfacesPage verifies HTMX partial returns
// just the content fragment, not the full workbench shell.
func TestWorkbench_HTMXPartialForInterfacesPage(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/iface/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "wb-table", "HTMX partial must contain table")
	assert.NotContains(t, html, `id="workbench-shell"`, "HTMX partial must not contain shell")
}

// TestWorkbench_ExistingYANGPathStillWorks verifies that non-page paths
// (e.g., /show/bgp/) still fall through to the YANG detail view.
func TestWorkbench_ExistingYANGPathStillWorks(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/bgp/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, `id="workbench-shell"`, "must be in workbench shell")
	// Must render the YANG detail view (contains ze-field for BGP config).
	assert.Contains(t, html, "ze-field", "must render YANG detail fields")
	// Must NOT contain workbench table/form components from page handlers.
	assert.NotContains(t, html, "wb-table-container", "must not render interface table")
	assert.NotContains(t, html, "wb-form", "must not render form page")
}
