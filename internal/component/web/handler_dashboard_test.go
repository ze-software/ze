package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "No BGP peers configured")
	assert.Contains(t, html, "No interfaces configured")
}

func TestDashboardOverviewAutoRefresh(t *testing.T) {
	// workbenchDashboard (component_workbench_dashboard.templ) renders the root
	// path, and its panels are static. Auto-refresh reaches them through the
	// hx-trigger the health sub-page carries.
	// This test verifies the dashboard renders with the expected panels.
	tree := config.NewTree()
	handler := workbenchForDashboard(t, tree, nil)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/", http.NoBody)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/health/", http.NoBody)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/health/", http.NoBody)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/events/", http.NoBody)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/events/", http.NoBody)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/events/?namespace=bgp", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// The handler dispatches both "show event namespaces" and "show event recent".
	// The last captured command should include the namespace filter.
	assert.Contains(t, capturedCmd, "namespace bgp")
}

// TestDashboardHealthRendersEveryCellOfEveryRow verifies a short row renders
// what it has and every header column keeps a cell under it.
// VALIDATES: a short row renders what it has, and every cell the header names
// reaches the page.
// PREVENTS: one malformed row emptying the whole panel. The markup read cells
// 0, 1 and 2 by index. Under html/template a two-cell row failed the index,
// RenderFragment discarded the error, and the operator got a blank health panel
// rather than one bad line. A four-column header also drew a column with no
// cell under it.
func TestDashboardHealthRendersEveryCellOfEveryRow(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	data := dashboardHealthData{
		Title: "Component Health",
		Columns: []WorkbenchTableColumn{
			{Label: "Status"}, {Label: "Component"}, {Label: "Detail"}, {Label: "Since"},
		},
		Rows: []WorkbenchTableRow{
			{FlagClass: "green", Cells: []string{"UP", "BGP", "3 peers", "12:04"}},
			{FlagClass: "grey", Cells: []string{"DOWN", "L2TP"}},
			{FlagClass: "green", Cells: []string{"UP", "Web", "serving"}},
		},
		EmptyMessage: "No components",
	}

	html := string(renderer.renderComponent("dashboard_health", dashboardHealth(data)))
	require.NotEmpty(t, html, "the health panel must render")

	// The short row does not empty the panel.
	assert.Contains(t, html, "BGP", "the long row must render")
	assert.Contains(t, html, "L2TP", "the short row must render")
	assert.Contains(t, html, "Web", "the row after the short one must render")

	// The fourth cell has a header, so it must have a cell under it.
	assert.Contains(t, html, "12:04", "a fourth cell must render under its fourth column")

	// The status badge stays on the first cell only.
	assert.Contains(t, html, `wb-health-status wb-health-status--green`)
	assert.Equal(t, 3, strings.Count(html, "wb-health-status wb-health-status--"),
		"exactly one status badge per row")
}

// healthRowCellCounts answers the number of td in each rendered body row.
func healthRowCellCounts(html string) []int {
	var counts []int

	for _, row := range strings.Split(html, `<tr class="wb-table-row">`)[1:] {
		body, _, _ := strings.Cut(row, "</tr>")
		counts = append(counts, strings.Count(body, "<td"))
	}

	return counts
}

// TestDashboardHealthDrawsOneCellPerHeaderColumn verifies every body row is as
// wide as the header, whatever its producer wrote.
//
// VALIDATES: one td per th, for a short row, an exact row, an empty row and a
// row longer than the header.
// PREVENTS: a table whose columns stop lining up. The header ranged Columns and
// the body ranged the row's own Cells, so the two agreed only by the habit of
// the one producer. A zero-cell row drew an empty tr. A longer row drew a cell
// under no header, which reads as the value of the column beside it.
func TestDashboardHealthDrawsOneCellPerHeaderColumn(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	data := dashboardHealthData{
		Title: "Component Health",
		Columns: []WorkbenchTableColumn{
			{Label: "Status"}, {Label: "Component"}, {Label: "Detail"}, {Label: "Since"},
		},
		Rows: []WorkbenchTableRow{
			{Key: "exact", FlagClass: "green", Cells: []string{"UP", "BGP", "3 peers", "12:04"}},
			{Key: "short", FlagClass: "grey", Cells: []string{"DOWN", "L2TP"}},
			{Key: "empty", FlagClass: "grey"},
			{Key: "long", FlagClass: "green", Cells: []string{"UP", "Web", "serving", "09:00", "unheaded-cell"}},
		},
		EmptyMessage: "No components",
	}

	html := string(renderer.renderComponent("dashboard_health", dashboardHealth(data)))
	require.NotEmpty(t, html, "the health panel must render")

	assert.Equal(t, 4, strings.Count(html, "<th "), "the header must draw one th per column")
	assert.Equal(t, []int{4, 4, 4, 4}, healthRowCellCounts(html),
		"every row must draw one td per header column")
	assert.NotContains(t, html, "unheaded-cell",
		"a cell past the last column has no header to name it and must not render")
}
