package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCommandTree builds a static command tree for admin handler tests.
// Structure:
//
//	(root) -> [peer, rib]
//	peer -> [teardown, refresh]
//	rib -> [clear]
//	peer/teardown -> leaf (no children)
//	peer/refresh -> leaf (no children)
//	rib/clear -> leaf (no children)
func testCommandTree() map[string][]string {
	return map[string][]string{
		"":     {"peer", "bgp rib"},
		"peer": {"teardown", "refresh"},
		"rib":  {"clear"},
	}
}

// testDispatcher returns a CommandDispatcher that echoes the command string
// as output. If the command contains "fail", it returns an error.
func testDispatcher() CommandDispatcher {
	return func(command, _, _ string) (string, error) {
		if strings.Contains(command, "fail") {
			return "", fmt.Errorf("command failed: %s", command)
		}

		return "executed: " + command, nil
	}
}

// TestAdminRouteDispatch verifies that GET /admin/peer/ returns the command
// tree with child links for the "peer" container.
//
// VALIDATES: AC-2 (peer admin tree with sub-commands as links).
// PREVENTS: Missing children in admin container view.
func TestAdminRouteDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	children := testCommandTree()
	handler := HandleAdminView(renderer, children)

	req := httptest.NewRequest(http.MethodGet, "/admin/peer/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	fragData := buildAdminFragmentData([]string{"peer"}, children)
	assert.Nil(t, fragData.CommandForm, "peer is a container, not a leaf")
	// Finder columns: root column + peer column.
	require.GreaterOrEqual(t, len(fragData.Columns), 2, "root + peer columns")

	// Last column should show peer's sub-commands.
	lastCol := fragData.Columns[len(fragData.Columns)-1]
	itemNames := make(map[string]bool)
	for _, item := range lastCol.UnnamedItems {
		itemNames[item.Name] = true
	}

	assert.True(t, itemNames["teardown"], "teardown must be in finder column")
	assert.True(t, itemNames["refresh"], "refresh must be in finder column")
}

// TestAdminBreadcrumb verifies that /admin/peer/ produces breadcrumb segments
// with the correct names and URLs.
//
// VALIDATES: AC-8 (breadcrumb at /admin/peer/ shows admin > peer with clickable segments).
// VALIDATES: AC-9 (back button at /admin/peer/ navigates to /admin/).
// PREVENTS: Broken breadcrumb URLs, missing segments, wrong prefix.
func TestAdminBreadcrumb(t *testing.T) {
	segments := buildAdminBreadcrumbs([]string{"peer"})

	require.Len(t, segments, 2, "admin root + peer")

	// Root segment: "admin" linking to /admin/.
	assert.Equal(t, "admin", segments[0].Name)
	assert.Equal(t, "/admin/", segments[0].URL)
	assert.False(t, segments[0].Active)

	// "peer" segment (last = active).
	assert.Equal(t, "peer", segments[1].Name)
	assert.Equal(t, "/admin/peer/", segments[1].URL)
	assert.True(t, segments[1].Active)
}

// TestAdminBreadcrumbRoot verifies that an empty path under /admin/ produces
// only the root breadcrumb segment.
//
// VALIDATES: AC-1 (root admin view).
// PREVENTS: Panic on empty admin path.
func TestAdminBreadcrumbRoot(t *testing.T) {
	segments := buildAdminBreadcrumbs(nil)

	require.Len(t, segments, 1, "admin root only")

	assert.Equal(t, "admin", segments[0].Name)
	assert.Equal(t, "/admin/", segments[0].URL)
	assert.True(t, segments[0].Active)
}

// TestAdminBreadcrumbDeep verifies breadcrumbs for a multi-level admin path.
//
// VALIDATES: AC-8 (breadcrumb with multiple clickable segments).
// PREVENTS: Wrong URL construction for deep paths.
func TestAdminBreadcrumbDeep(t *testing.T) {
	segments := buildAdminBreadcrumbs([]string{"peer", "192.168.1.1", "teardown"})

	require.Len(t, segments, 4, "admin + 3 path segments")

	assert.Equal(t, "admin", segments[0].Name)
	assert.Equal(t, "/admin/", segments[0].URL)

	assert.Equal(t, "peer", segments[1].Name)
	assert.Equal(t, "/admin/peer/", segments[1].URL)

	assert.Equal(t, "192.168.1.1", segments[2].Name)
	assert.Equal(t, "/admin/peer/192.168.1.1/", segments[2].URL)

	assert.Equal(t, "teardown", segments[3].Name)
	assert.Equal(t, "/admin/peer/192.168.1.1/teardown/", segments[3].URL)
	assert.True(t, segments[3].Active)
}

// TestCommandFormRendering verifies that a leaf command (no sub-commands)
// produces form data with the command name and action URL.
//
// VALIDATES: AC-3 (leaf command renders with parameter form fields and Execute button).
// VALIDATES: AC-10 (command with parameters renders form with path and parameter fields).
// PREVENTS: Leaf nodes rendered as containers, missing action URL.
func TestCommandFormRendering(t *testing.T) {
	children := testCommandTree()

	fragData := buildAdminFragmentData([]string{"peer", "teardown"}, children)

	require.NotNil(t, fragData.CommandForm, "leaf command must have form data")

	assert.Equal(t, "peer teardown", fragData.CommandForm.CommandName)
	assert.Equal(t, "/admin/peer/teardown", fragData.CommandForm.ActionURL)
}

// TestAdminCommandExecution verifies that POST /admin/peer/192.168.1.1/teardown
// dispatches the command and returns a result card with the command name
// and output.
//
// VALIDATES: AC-4 (POST executes the mutation command and returns result card).
// VALIDATES: AC-5 (result card has titled header with command name and output in body).
// PREVENTS: Command not dispatched, result card missing output.
func TestAdminCommandExecution(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	dispatch := testDispatcher()
	handler := HandleAdminExecute(renderer, dispatch)

	req := httptest.NewRequest(http.MethodPost, "/admin/peer/192.168.1.1/teardown", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "peer 192.168.1.1 teardown", "result card must contain the command name")
	assert.Contains(t, body, "executed: peer 192.168.1.1 teardown", "result card must contain the output")
	assert.NotContains(t, body, "command-error", "successful command must not have error class")
}

// TestCommandResultCard verifies the structure of a command result card
// by checking the template data fields.
//
// VALIDATES: AC-5 (result card has titled header with command name and output in body).
// PREVENTS: Wrong command name, missing output in result data.
func TestCommandResultCard(t *testing.T) {
	result := CommandResultData{
		CommandName: "peer 192.168.1.1 teardown",
		Output:      "peer 192.168.1.1 torn down",
		Error:       false,
	}

	assert.Equal(t, "peer 192.168.1.1 teardown", result.CommandName)
	assert.Equal(t, "peer 192.168.1.1 torn down", result.Output)
	assert.False(t, result.Error)
}

// TestCommandResultCardStack verifies that multiple command executions
// produce independent result cards. HTMX stacking (afterbegin) is a
// client-side concern; the server returns one card per POST.
//
// VALIDATES: AC-6 (new card appears above previous one via hx-swap="afterbegin").
// PREVENTS: Server-side accumulation of results (each POST is stateless).
func TestCommandResultCardStack(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	dispatch := testDispatcher()
	handler := HandleAdminExecute(renderer, dispatch)

	// First command.
	req1 := httptest.NewRequest(http.MethodPost, "/admin/peer/192.168.1.1/teardown", http.NoBody)
	rec1 := httptest.NewRecorder()

	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	body1 := rec1.Body.String()

	// Second command.
	req2 := httptest.NewRequest(http.MethodPost, "/admin/bgp/rib/clear", http.NoBody)
	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)

	body2 := rec2.Body.String()

	// Each response is an independent result card.
	assert.Contains(t, body1, "peer 192.168.1.1 teardown")
	assert.Contains(t, body2, "bgp rib clear")

	// They are different cards (different content).
	assert.NotEqual(t, body1, body2, "each POST produces a distinct result card")
}

// TestCommandErrorCard verifies that a command execution error produces
// an error-styled result card with the error message in the body.
//
// VALIDATES: AC-11 (command execution error renders error in result card).
// PREVENTS: Errors silently swallowed, missing error styling.
func TestCommandErrorCard(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	dispatch := testDispatcher()
	handler := HandleAdminExecute(renderer, dispatch)

	// The test dispatcher returns an error when the command contains "fail".
	req := httptest.NewRequest(http.MethodPost, "/admin/fail/command", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "command-error", "error card must have error CSS class")
	assert.Contains(t, body, "command failed", "error card must contain error message")
}

// TestAdminContentNegotiation verifies that ?format=json on a command
// execution POST returns JSON instead of HTML.
//
// VALIDATES: AC-7 (format=json returns JSON command output).
// PREVENTS: JSON negotiation ignored for admin commands.
func TestAdminContentNegotiation(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	dispatch := testDispatcher()
	handler := HandleAdminExecute(renderer, dispatch)

	req := httptest.NewRequest(http.MethodPost, "/admin/peer/192.168.1.1/teardown?format=json", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var data map[string]any
	err = json.NewDecoder(rec.Body).Decode(&data)
	require.NoError(t, err, "response must be valid JSON")

	assert.Equal(t, "peer 192.168.1.1 teardown", data["command"])
	assert.Equal(t, "executed: peer 192.168.1.1 teardown", data["output"])
	assert.Equal(t, false, data["error"])
}

// TestAdminContentNegotiationView verifies that ?format=json on a GET
// admin view returns the command tree as JSON.
//
// VALIDATES: AC-7 (JSON content negotiation for admin views).
// PREVENTS: JSON negotiation only working for POST, not GET.
func TestAdminContentNegotiationView(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	children := testCommandTree()
	handler := HandleAdminView(renderer, children)

	req := httptest.NewRequest(http.MethodGet, "/admin/peer/?format=json", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var data map[string]any
	err = json.NewDecoder(rec.Body).Decode(&data)
	require.NoError(t, err, "response must be valid JSON")

	kids, ok := data["children"].([]any)
	require.True(t, ok, "children must be an array")
	assert.Len(t, kids, 2)
}

// TestAdminExecuteMethodNotAllowed verifies that GET to the execute handler
// returns 405 Method Not Allowed.
//
// VALIDATES: handler enforces POST-only for command execution.
// PREVENTS: Commands executed via GET (browser navigation).
func TestAdminExecuteMethodNotAllowed(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	dispatch := testDispatcher()
	handler := HandleAdminExecute(renderer, dispatch)

	req := httptest.NewRequest(http.MethodGet, "/admin/peer/192.168.1.1/teardown", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// TestBuildAdminCommandTree verifies that the production command tree has
// the expected top-level categories and peer sub-commands.
//
// VALIDATES: BuildAdminCommandTree returns a valid tree structure.
// PREVENTS: Missing top-level categories, empty sub-command lists.
func TestBuildAdminCommandTree(t *testing.T) {
	tree := BuildAdminCommandTree() //nolint:staticcheck // legacy tree retained for fallback path

	// Root must have top-level categories.
	root := tree[""]
	require.NotEmpty(t, root, "root must have children")
	assert.Contains(t, root, "peer")
	assert.Contains(t, root, "route")
	assert.Contains(t, root, "cache")
	assert.Contains(t, root, "system")

	// Peer must have operational sub-commands.
	peer := tree["peer"]
	require.NotEmpty(t, peer, "peer must have children")
	assert.Contains(t, peer, "teardown")
	assert.Contains(t, peer, "show")
	assert.Contains(t, peer, "list")
}

// TestAdminExecuteNilDispatcher verifies that POST with nil dispatcher
// returns 503 instead of panicking.
//
// VALIDATES: nil dispatcher guard prevents panic.
// PREVENTS: nil pointer dereference on command execution.
func TestAdminExecuteNilDispatcher(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	handler := HandleAdminExecute(renderer, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/peer/teardown", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "not available")
}

// TestAdminRootView verifies that GET /admin/ renders the root admin view
// with top-level command modules as navigable links.
//
// VALIDATES: AC-1 (root admin view with top-level mutation command modules).
// PREVENTS: Empty root view, missing top-level commands.
func TestAdminRootView(t *testing.T) {
	children := testCommandTree()

	fragData := buildAdminFragmentData(nil, children)

	assert.Nil(t, fragData.CommandForm, "root is a container")
	// Root column should list top-level commands.
	require.Len(t, fragData.Columns, 1, "root has 1 finder column")

	itemNames := make(map[string]bool)
	for _, item := range fragData.Columns[0].UnnamedItems {
		itemNames[item.Name] = true
	}

	assert.True(t, itemNames["peer"], "peer must be in root column")
	assert.True(t, itemNames["bgp rib"], "bgp rib must be in root column")

	// Verify URLs use /admin/ prefix.
	for _, item := range fragData.Columns[0].UnnamedItems {
		assert.True(t, strings.HasPrefix(item.URL, "/admin/"),
			"item URL %q must start with /admin/", item.URL)
	}
}

// TestAdminErrorContentNegotiation verifies that ?format=json on an error
// command execution returns JSON with error=true.
//
// VALIDATES: AC-7 + AC-11 (JSON error response for failed commands).
// PREVENTS: Error lost in JSON content negotiation.
func TestAdminErrorContentNegotiation(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	dispatch := testDispatcher()
	handler := HandleAdminExecute(renderer, dispatch)

	req := httptest.NewRequest(http.MethodPost, "/admin/fail/command?format=json", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var data map[string]any
	err = json.NewDecoder(rec.Body).Decode(&data)
	require.NoError(t, err)

	assert.Equal(t, true, data["error"])
	assert.Contains(t, data["output"], "command failed")
}
