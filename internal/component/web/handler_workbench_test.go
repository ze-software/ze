package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config"
)

// TestHandleWorkbench_RootRendersShell verifies that a full-page GET against
// the workbench handler at the root path renders the workbench shell with all four
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

	rec := newGET("/show/").serve(t, handler)
	requireStatus(t, http.StatusOK, rec)

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
// would break HTMX-driven navigation under the workbench.
func TestHandleWorkbench_HTMXPartialReusesOOBResponse(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	rec := newGET("/show/bgp/").htmx().serve(t, handler)
	requireStatus(t, http.StatusOK, rec)

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

// TestWorkbenchWebServiceRoute verifies that /show/web/ renders the Web service
// configuration page. The handler (HandleWebServicePage) already existed but
// renderPageContent never routed segWeb, so the link returned a 400 (F6).
//
// VALIDATES: AC-5 -- Services > Web nav link renders content, not "bad request".
// PREVENTS: a nav entry with a working handler left unrouted.
func TestWorkbenchWebServiceRoute(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)
	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)

	handler := HandleWorkbench(renderer, schema, config.NewTree(), nil, true)
	req := httptest.NewRequest(http.MethodGet, "/show/web/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.NotContains(t, html, "bad request")
	assert.Contains(t, html, "Web Configuration", "Web service link must render the web config page")
}

// TestBGPPeerDetailRoute verifies that /show/bgp/peer/<name>/ renders the single
// peer's detail (generic YANG view), not the whole peers table (F7). The table
// lists every peer; the detail is scoped to one, so peer-b must not appear when
// viewing peer-a.
//
// VALIDATES: AC-6 -- peer row Edit opens that peer, not the table.
// PREVENTS: bgp/peer/<name> matching the table handler regardless of depth.
func TestBGPPeerDetailRoute(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)
	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := buildTestBGPTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	// The table at /show/bgp/peer/ lists all peers.
	tableReq := httptest.NewRequest(http.MethodGet, "/show/bgp/peer/", http.NoBody)
	tableRec := httptest.NewRecorder()
	handler.ServeHTTP(tableRec, tableReq)
	assert.Equal(t, http.StatusOK, tableRec.Code)
	tableHTML := tableRec.Body.String()
	assert.Contains(t, tableHTML, "BGP Peers", "table view shows the peers table title")
	assert.Contains(t, tableHTML, "peer-a")
	assert.Contains(t, tableHTML, "peer-b")

	// The detail at /show/bgp/peer/peer-a/ is scoped to peer-a only.
	detailReq := httptest.NewRequest(http.MethodGet, "/show/bgp/peer/peer-a/", http.NoBody)
	detailRec := httptest.NewRecorder()
	handler.ServeHTTP(detailRec, detailReq)
	assert.Equal(t, http.StatusOK, detailRec.Code)
	detailHTML := detailRec.Body.String()
	assert.Contains(t, detailHTML, "peer-a", "detail view shows the selected peer")
	assert.NotContains(t, detailHTML, "peer-b",
		"detail must be scoped to peer-a; listing peer-b means it rendered the table")
}

// TestWorkbenchNavAllRoutes walks every URL declared in the left-nav sections()
// and asserts each renders 200 with shell content and no "bad request" — the
// regression guard for F6 (dead nav links). Dead entries that mapped to no real
// config area were removed from sections(); the survivors must all resolve.
//
// VALIDATES: AC-5 -- every nav link returns 200 + non-empty content.
// PREVENTS: a nav entry pointing at a path with no page handler or YANG target.
func TestWorkbenchNavAllRoutes(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)
	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := buildTestBGPTree()

	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	for _, def := range sections() {
		for _, child := range def.children {
			t.Run(def.key+"_"+child.Key, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, child.URL, http.NoBody)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusOK, rec.Code,
					"nav %q (%s) must return 200", child.Label, child.URL)
				body := rec.Body.String()
				assert.NotContains(t, body, "bad request",
					"nav %q (%s) must not render a bad-request page", child.Label, child.URL)
				assert.Contains(t, body, "workbench-shell",
					"nav %q (%s) must render inside the workbench shell", child.Label, child.URL)
			})
		}
	}
}

// TestFaviconHandlerServesAsset verifies /favicon.ico is served from assets with
// a 200 and an image content type, instead of falling through the catch-all to
// an error redirect on every page view (F14/AC-15).
func TestFaviconHandlerServesAsset(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", http.NoBody)
	rec := httptest.NewRecorder()
	renderer.FaviconHandler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "image/svg+xml")
	assert.NotEmpty(t, rec.Body.Bytes(), "favicon body must not be empty")
	assert.NotContains(t, rec.Body.String(), "error=", "favicon must not be an error redirect")
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
	rec := newGET("/show/bgp/..").serve(t, handler)
	requireStatus(t, http.StatusBadRequest, rec)
}

// TestWorkbenchHidesCommitBarForReadOnly verifies the commit bar (the primary,
// always-visible edit affordance) is hidden from a read-only user and shown to
// an editor. This proves the authorizer -> readOnly -> LayoutData.ReadOnly ->
// template chain end-to-end through the workbench handler.
//
// VALIDATES: AC-1 tail -- edit controls hidden for read-only users (nav-hiding),
// complementing the route-gate 403 enforcement (already shipped).
// PREVENTS: showing a commit button that would 403 on click.
func TestWorkbenchHidesCommitBarForReadOnly(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)
	schema, schemaErr := config.YANGSchema()
	assert.NoError(t, schemaErr)
	tree := config.NewTree()

	ro := HandleWorkbench(renderer, schema, tree, nil, false, WithAuthorizer(fakeAuthorizer{allowEdit: false}))
	roReq := httptest.NewRequest(http.MethodGet, "/show/", http.NoBody)
	roReq = roReq.WithContext(withUsername(roReq.Context(), "bob"))
	roRec := httptest.NewRecorder()
	ro.ServeHTTP(roRec, roReq)
	assert.Equal(t, http.StatusOK, roRec.Code)
	roHTML := roRec.Body.String()
	assert.Contains(t, roHTML, "workbench-shell", "read-only user still gets the shell")
	assert.NotContains(t, roHTML, "commit-review-btn", "read-only user must not see the commit button")
	assert.NotContains(t, roHTML, `id="commit-bar"`, "read-only user must not see the commit bar")

	admin := HandleWorkbench(renderer, schema, tree, nil, false, WithAuthorizer(fakeAuthorizer{allowEdit: true}))
	adminReq := httptest.NewRequest(http.MethodGet, "/show/", http.NoBody)
	adminReq = adminReq.WithContext(withUsername(adminReq.Context(), "admin"))
	adminRec := httptest.NewRecorder()
	admin.ServeHTTP(adminRec, adminReq)
	assert.Equal(t, http.StatusOK, adminRec.Code)
	assert.Contains(t, adminRec.Body.String(), "commit-review-btn", "editor must see the commit button")
}

// TestListTableHidesEditControlsForReadOnly verifies the generic config list
// table hides its inline-edit inputs, rename, delete and "+ new" controls for a
// read-only user, while still rendering the values as text.
//
// VALIDATES: AC-1 tail -- config-form (list table) edit controls gated on ReadOnly.
// PREVENTS: read-only users seeing set/delete/add affordances that 403 on use.
func TestListTableHidesEditControlsForReadOnly(t *testing.T) {
	renderer, err := NewRenderer()
	assert.NoError(t, err)

	render := func(readOnly bool) string {
		data := FragmentData{
			ActiveUI: "workbench",
			ReadOnly: readOnly,
			ListTable: &ListTableView{
				Name:    "peer",
				Columns: []ListTableColumn{{Name: "peer", Key: true}, {Name: "ip"}},
				Rows: []ListTableRow{{
					KeyValue: "edge1",
					URL:      "/show/bgp/peer/edge1/",
					HxPath:   "bgp/peer/edge1",
					Cells:    []ListTableCell{{Value: "1.1.1.1", Leaf: "ip", Path: "bgp/peer/edge1/remote"}},
				}},
				FormURL: "/config/add-form/bgp/peer/",
			},
		}
		return string(renderer.RenderFragment("list_table", data))
	}

	roHTML := render(true)
	assert.NotContains(t, roHTML, "finder-add", "read-only: no + new button")
	assert.NotContains(t, roHTML, "finder-delete-btn", "read-only: no delete button")
	assert.NotContains(t, roHTML, "finder-rename-btn", "read-only: no rename button")
	assert.NotContains(t, roHTML, "finder-table-input", "read-only: cells are not editable inputs")
	assert.Contains(t, roHTML, "1.1.1.1", "read-only: value still shown as text")

	rwHTML := render(false)
	assert.Contains(t, rwHTML, "finder-add", "editor: + new button present")
	assert.Contains(t, rwHTML, "finder-delete-btn", "editor: delete button present")
	assert.Contains(t, rwHTML, "finder-table-input", "editor: inline-edit inputs present")
}
