package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestHandleWorkbench_RootRendersShell verifies that a full-page GET against
// the workbench handler at the root path renders the V2 shell with all four
// regions (top bar, left nav, workspace, commit bar). This is the
// integration test that proves the whole render chain wires up: route ->
// data builder -> detail fragment -> RenderWorkbench.
//
// VALIDATES: AC-1 (top bar + left nav + workspace + commit + CLI bar render).
// PREVENTS: Silent regressions where the shell template parses but a region
// drops out at runtime (e.g., template name mismatch in {{template ...}}).
func TestHandleWorkbench_RootRendersShell(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	html := rec.Body.String()
	assert.Contains(t, html, `id="workbench-shell"`)
	assert.Contains(t, html, `data-ui-mode="workbench"`)
	assert.Contains(t, html, `id="workbench-topbar"`)
	assert.Contains(t, html, `id="workbench-nav"`)
	assert.Contains(t, html, `id="workbench-workspace"`)
	assert.Contains(t, html, `id="commit-bar"`)
	assert.Contains(t, html, `Routing`, "left nav must contain Routing section")
}

// TestHandleWorkbench_HTMXPartialReusesOOBResponse verifies that an HTMX
// partial request (HX-Request header) returns the existing OOB fragment
// rather than the workbench shell. Both UIs share the same OOB swap protocol
// so that /fragment/detail navigation continues to work transparently.
//
// VALIDATES: HTMX swap behavior is preserved across the UI mode boundary.
// PREVENTS: A workbench-specific OOB protocol diverging from Finder's, which
// would break HTMX-driven navigation under V2.
func TestHandleWorkbench_HTMXPartialReusesOOBResponse(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/bgp/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	html := rec.Body.String()
	// HTMX partial must NOT include the full workbench shell wrappers.
	assert.False(t, strings.Contains(html, `<!DOCTYPE html>`), "HTMX partial must be a fragment, not a full page")
	assert.False(t, strings.Contains(html, `id="workbench-shell"`), "HTMX partial must not include the shell")
}

// TestHandleWorkbench_DashboardRendersOverview verifies that a full-page GET
// at the root path renders the dashboard overview panels instead of the
// detail fragment. The dashboard is the default landing page for the
// workbench shell.
//
// VALIDATES: Phase 6 dashboard integration (root path renders overview).
// PREVENTS: Dashboard template not wired into the handler, or root path
// still rendering the detail fragment.
func TestHandleWorkbench_DashboardRendersOverview(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	html := rec.Body.String()
	// Must contain dashboard panels.
	assert.Contains(t, html, `wb-dashboard`, "root path must render dashboard")
	assert.Contains(t, html, `System`, "dashboard must have System panel")
	assert.Contains(t, html, `BGP Summary`, "dashboard must have BGP panel")
	assert.Contains(t, html, `Interfaces`, "dashboard must have Interfaces panel")
	// Must still be inside the workbench shell.
	assert.Contains(t, html, `id="workbench-shell"`)
}

// TestHandleWorkbench_BadPathReturns400 verifies that a path containing
// invalid YANG-identifier characters is rejected before any rendering work
// happens. The shared ValidatePathSegments helper is the gate.
//
// VALIDATES: Path validation runs in the workbench handler.
// PREVENTS: Workbench paths bypassing the path-traversal/character checks
// that the Finder handler enforces.
func TestHandleWorkbench_BadPathReturns400(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	// `..` is forbidden by ValidatePathSegments.
	req := httptest.NewRequest(http.MethodGet, "/show/bgp/..", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
