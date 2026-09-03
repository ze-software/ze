package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

// testCommandTree builds a static command tree for admin handler tests. One
// leaf carries both help texts, so a test tells a summary from an explanation
// with no YANG load.
func testCommandTree() *command.Node {
	return &command.Node{
		Children: map[string]*command.Node{
			"peer": {
				Name: "peer",
				Children: map[string]*command.Node{
					"raw": {Name: "raw"},
					"teardown": {
						Name:        "teardown",
						Description: testTeardownSummary,
						LongHelp:    testTeardownHelp,
					},
				},
			},
			"bgp rib": {Name: "bgp rib"},
		},
	}
}

// The two halves one admin command declares, quoted by the tests that assert
// which of them reaches which part of the page.
const (
	testTeardownSummary = "Close the session with one peer."
	testTeardownHelp    = "The peer is torn down at once.\nA configured peer reconnects on its own timer."
)

// testDispatcher returns a CommandDispatcher that echoes the command string
// as output. If the command contains "fail", it returns an error.
func testDispatcher() CommandDispatcher {
	return func(_ context.Context, _ plugin.CallerIdentity, command string) (*plugin.Response, error) {
		if strings.Contains(command, "fail") {
			return nil, fmt.Errorf("command failed: %s", command)
		}

		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "executed", "message": command}), nil
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

	tree := testCommandTree()
	handler := HandleAdminView(renderer, tree)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/peer/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	fragData := buildAdminFragmentData([]string{"peer"}, tree)
	assert.Nil(t, fragData.CommandForm, "peer is a container, not a leaf")
	// Finder columns: root column + peer column.
	require.GreaterOrEqual(t, len(fragData.Columns), 2, "root + peer columns")

	// Last column should show peer's sub-commands.
	lastCol := fragData.Columns[len(fragData.Columns)-1]
	itemNames := make(map[string]bool)
	for _, item := range lastCol.UnnamedItems {
		itemNames[item.Name] = true
	}

	assert.True(t, itemNames["raw"], "raw must be in finder column")
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
	tree := testCommandTree()

	fragData := buildAdminFragmentData([]string{"peer", "teardown"}, tree)

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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/peer/192.168.1.1/teardown", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "peer 192.168.1.1 teardown", "result card must contain the command name")
	assert.Contains(t, body, "peer 192.168.1.1 teardown", "result card must contain the output")
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
	req1 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/peer/192.168.1.1/teardown", http.NoBody)
	rec1 := httptest.NewRecorder()

	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	body1 := rec1.Body.String()

	// Second command.
	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/bgp/rib/clear", http.NoBody)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/fail/command", http.NoBody)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/peer/192.168.1.1/teardown?format=json", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var data map[string]any
	err = json.NewDecoder(rec.Body).Decode(&data)
	require.NoError(t, err, "response must be valid JSON")

	assert.Equal(t, "peer 192.168.1.1 teardown", data["command"])
	output, ok := data["output"].(string)
	require.True(t, ok, "output must be the rendered payload")
	assert.JSONEq(t, `{"result":"executed","message":"peer 192.168.1.1 teardown"}`, output)
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

	tree := testCommandTree()
	handler := HandleAdminView(renderer, tree)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/peer/?format=json", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var data map[string]any
	err = json.NewDecoder(rec.Body).Decode(&data)
	require.NoError(t, err, "response must be valid JSON")

	kids, ok := data["children"].([]any)
	require.True(t, ok, "children must be an array")
	assert.Equal(t, []any{"raw", "teardown"}, kids, "children are the node's own, sorted")
}

// VALIDATES: web command completion follows the response writer.
// PREVENTS: accepted lifecycle teardown running during text rendering.
func TestAdminExecuteCompletesAfterResponseWrite(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	completed := false
	dispatch := CommandDispatcher(func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		resp := plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "accepted"})
		resp.OnTransportComplete(func() {
			assert.Contains(t, recorder.Body.String(), "accepted")
			completed = true
		})
		return resp, nil
	})
	handler := HandleAdminExecute(renderer, dispatch)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/request/shutdown?format=json", http.NoBody)

	handler(recorder, req)

	assert.True(t, completed)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/peer/192.168.1.1/teardown", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/peer/teardown", http.NoBody)
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
	tree := testCommandTree()

	fragData := buildAdminFragmentData(nil, tree)

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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/fail/command?format=json", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var data map[string]any
	err = json.NewDecoder(rec.Body).Decode(&data)
	require.NoError(t, err)

	assert.Equal(t, true, data["error"])
	assert.Contains(t, data["output"], "command failed")
}

// TestAdminCommandFormShowsHelp drives GET /admin/peer/teardown/ and reads the
// rendered page. It is the wiring test for AC-8 and user story 4. The form for
// one command shows the summary the YANG node declares, then the explanation
// its ze:help declares.
//
// The page rendered neither before this spec. CommandFormData.Description was
// documented as the YANG description, and no producer set it. The template's
// non-empty guard was therefore never true (plan/journal/unwired-feature.md).
//
// PREVENTS: the form losing its help text again, and the explanation reaching
// the page unescaped.
func TestAdminCommandFormShowsHelp(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	tree := testCommandTree()
	handler := HandleAdminView(renderer, tree)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/peer/teardown/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Contains(t, body, testTeardownSummary, "the summary is the lede of the command form")
	assert.Contains(t, body, "The peer is torn down at once.", "the explanation is the body")
	assert.Contains(t, body, "command-form-help", "the explanation has its own class, so CSS can keep its line breaks")

	summaryAt := strings.Index(body, testTeardownSummary)
	helpAt := strings.Index(body, "The peer is torn down at once.")
	assert.Less(t, summaryAt, helpAt, "the summary comes before the explanation")

	// The fragment builder is the producer both halves come from.
	fragData := buildAdminFragmentData([]string{"peer", "teardown"}, tree)
	require.NotNil(t, fragData.CommandForm)
	assert.Equal(t, testTeardownSummary, fragData.CommandForm.Description)
	assert.Equal(t, testTeardownHelp, fragData.CommandForm.LongHelp)

	// A path no node holds still renders a form, and shows no text it cannot
	// read. An absent command must not borrow its parent's help.
	unknown := buildAdminFragmentData([]string{"peer", "nosuchcommand"}, tree)
	require.NotNil(t, unknown.CommandForm)
	assert.Empty(t, unknown.CommandForm.Description)
	assert.Empty(t, unknown.CommandForm.LongHelp)
}

// TestAdminCommandFormEscapesHelp proves the security row of the spec's review
// checklist: a plugin-supplied help string reaches an HTML page, so the templ
// layer must escape it. The assertion is on the rendered bytes, not on the
// library's reputation.
//
// PREVENTS: a plugin injecting markup or script into the admin console through
// a command description or explanation.
func TestAdminCommandFormEscapesHelp(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	tree := &command.Node{Children: map[string]*command.Node{
		"peer": {Name: "peer", Children: map[string]*command.Node{
			"teardown": {
				Name:        "teardown",
				Description: `Close <b>one</b> session.`,
				LongHelp:    `<script>alert("x")</script>`,
			},
		}},
	}}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/peer/teardown/", http.NoBody)
	rec := httptest.NewRecorder()
	HandleAdminView(renderer, tree).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.NotContains(t, body, "<script>alert", "the explanation must reach the page escaped")
	assert.NotContains(t, body, "<b>one</b>", "the summary must reach the page escaped")
	assert.Contains(t, body, "&lt;script&gt;", "the explanation is still shown, as text")
}
