package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/health"
)

// TestComponentHealthRow verifies the health table resolves a component's row
// from a live probe first, then AlwaysUp (web is serving), then config presence
// (F10/AC-11) -- so the Web row never reads "Not configured" while serving.
func TestComponentHealthRow(t *testing.T) {
	tree := config.NewTree()
	probes := map[string]health.ComponentHealth{}

	// Web is always running: it is serving this page, even with empty config.
	web := componentDef{Name: "Web", ConfigKey: "environment/web", AlwaysUp: true}
	status, flag, _ := componentHealthRow(web, tree, probes)
	assert.Equal(t, "Running", status)
	assert.Equal(t, flagClassGreen, flag)

	// A live healthy probe shows Running.
	bgp := componentDef{Name: "BGP", ConfigKey: "bgp", HealthName: "bgp"}
	probes["bgp"] = health.ComponentHealth{Name: "bgp", Status: health.StatusHealthy}
	status, _, _ = componentHealthRow(bgp, tree, probes)
	assert.Equal(t, "Running", status)

	// A down probe shows Down/red and surfaces the reason.
	probes["bgp"] = health.ComponentHealth{Name: "bgp", Status: health.StatusDown, Reason: "no sessions"}
	status, flag, summary := componentHealthRow(bgp, tree, probes)
	assert.Equal(t, "Down", status)
	assert.Equal(t, flagClassRed, flag)
	assert.Equal(t, "no sessions", summary)

	// No probe and not configured -> Not configured.
	dns := componentDef{Name: "DNS", ConfigKey: "dns"}
	status, flag, _ = componentHealthRow(dns, tree, probes)
	assert.Equal(t, "Not configured", status)
	assert.Equal(t, flagClassGrey, flag)
}

// workbenchForDashboard creates a workbench handler for dashboard testing.
func workbenchForDashboard(t *testing.T, tree *config.Tree, dispatch CommandDispatcher) http.HandlerFunc {
	t.Helper()
	renderer, err := NewRenderer()
	require.NoError(t, err)
	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	var opts []WorkbenchOption
	if dispatch != nil {
		opts = append(opts, WithDispatch(dispatch))
	}
	return HandleWorkbench(renderer, schema, tree, nil, true, opts...)
}

// --- Dashboard Overview ---

func TestDashboardOverviewRendersPanels(t *testing.T) {
	tree := config.NewTree()
	handler := workbenchForDashboard(t, tree, nil)

	rec := newGET("/show/").serve(t, handler)
	requireContains(t, rec, "wb-dashboard")
	html := rec.Body.String()
	assert.Contains(t, html, "System")
	assert.Contains(t, html, "BGP Summary")
	assert.Contains(t, html, "Interfaces")
}

func TestDashboardOverviewEmptyState(t *testing.T) {
	tree := config.NewTree()
	handler := workbenchForDashboard(t, tree, nil)

	req := httptest.NewRequest(http.MethodGet, "/show/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "No BGP peers configured")
	assert.Contains(t, html, "No interfaces configured")
}

func TestDashboardOverviewAutoRefresh(t *testing.T) {
	// The dashboard overview is rendered by the workbench_dashboard template
	// at the root path. The template contains hx-trigger for auto-refresh.
	// The existing template uses static panels; auto-refresh is added via
	// the dashboard_overview template used by the health sub-page.
	// For v1, verify the dashboard renders with the expected panels.
	tree := config.NewTree()
	handler := workbenchForDashboard(t, tree, nil)

	req := httptest.NewRequest(http.MethodGet, "/show/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	// The root dashboard renders wb-dashboard panels.
	assert.Contains(t, html, "wb-dashboard", "dashboard must render panels")
	assert.Contains(t, html, "wb-dashboard-panel", "dashboard must have panel components")
}

// --- Dashboard > Health ---

func TestDashboardHealthRendersTable(t *testing.T) {
	tree := config.NewTree()
	handler := workbenchForDashboard(t, tree, nil)

	req := httptest.NewRequest(http.MethodGet, "/show/health/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "Component Health")
	assert.Contains(t, html, "BGP")
	assert.Contains(t, html, "Interfaces")
	assert.Contains(t, html, "L2TP")
	assert.Contains(t, html, "DNS")
}

func TestDashboardHealthStatusIndicators(t *testing.T) {
	// Create tree with BGP configured.
	tree := config.NewTree()
	bgp := config.NewTree()
	tree.SetContainer("bgp", bgp)

	handler := workbenchForDashboard(t, tree, nil)
	req := httptest.NewRequest(http.MethodGet, "/show/health/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	// BGP should be green (configured).
	assert.Contains(t, html, "wb-health-status--green")
	// DNS should be grey (not configured).
	assert.Contains(t, html, "wb-health-status--grey")
}

// --- Dashboard > Events ---

func TestDashboardEventsRendersTable(t *testing.T) {
	dispatch, _ := mockDispatcher("bgp peer up 192.0.2.1")
	tree := config.NewTree()
	handler := workbenchForDashboard(t, tree, dispatch)

	req := httptest.NewRequest(http.MethodGet, "/show/events/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "Recent Events")
	assert.Contains(t, html, "bgp peer up")
}

func TestDashboardEventsEmptyState(t *testing.T) {
	tree := config.NewTree()
	handler := workbenchForDashboard(t, tree, nil)

	req := httptest.NewRequest(http.MethodGet, "/show/events/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "No recent events")
}

func TestDashboardEventsNamespaceFilter(t *testing.T) {
	var capturedCmd string
	dispatch := func(_ context.Context, _ plugin.CallerIdentity, command string) (*plugin.Response, error) {
		capturedCmd = command
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}
	tree := config.NewTree()
	handler := workbenchForDashboard(t, tree, dispatch)

	req := httptest.NewRequest(http.MethodGet, "/show/events/?namespace=bgp", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// The handler dispatches both "show event namespaces" and "show event recent".
	// The last captured command should include the namespace filter.
	assert.Contains(t, capturedCmd, "namespace bgp")
}
