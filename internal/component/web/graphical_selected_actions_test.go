// Design: docs/architecture/web-components.md -- graphical selected-item operations.

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestGraphicalSelectedItemOffersDeleteAndDiscard follows a real Finder request
// for a slash-containing entry key and requires both selected-item controls.
func TestGraphicalSelectedItemOffersDeleteAndDiscard(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	path := []string{"bgp", "group", "selected/key"}
	require.NoError(t, mgr.SetValue("testuser", path, "description", "selected"))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleFragment(renderer, schema, tree, mgr, false, fakeAuthorizer{allowEdit: true})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authedRequest(t, http.MethodGet, "/show/"+encodeConfigPath(path)+"/", nil))
	requireStatus(t, http.StatusOK, response)
	assert.Contains(t, response.Body.String(), `id="graphical-delete-selected"`)
	assert.Contains(t, response.Body.String(), `id="graphical-discard-selected"`)
	assert.Contains(t, response.Body.String(), `value="bgp/group/selected%2Fkey"`)
}

// TestGraphicalSelectedWholeListOffersDelete distinguishes a selected list
// collection from an individual entry without losing whole-list deletion.
func TestGraphicalSelectedWholeListOffersDelete(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "one"}, "description", "configured"))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleFragment(renderer, schema, tree, mgr, false, fakeAuthorizer{allowEdit: true})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authedRequest(t, http.MethodGet, "/show/bgp/group/", nil))
	requireStatus(t, http.StatusOK, response)
	assert.Contains(t, response.Body.String(), `id="graphical-delete-selected"`)
	assert.Contains(t, response.Body.String(), `value="bgp/group"`)
	assert.NotContains(t, response.Body.String(), `id="graphical-entry-target"`)
}

// TestGraphicalSelectedPathDiscardPreservesOtherChangesAndAudit drives the
// selected-path action through its HTTP adapter and checks exact dropped intent.
func TestGraphicalSelectedPathDiscardPreservesOtherChangesAndAudit(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	committed, err := mgr.committedConfig()
	require.NoError(t, err)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.123"))
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "retained"}, "description", "keep-this-change"))
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	handler := HandleGraphicalAction(mgr, schema, nil, nil, recorder, nil)
	response := graphicalActionRequest(t, handler, "discard-path", "bgp/router-id", nil)
	requireStatus(t, http.StatusOK, response)
	diff, err := mgr.Diff("testuser")
	require.NoError(t, err)
	assert.NotContains(t, diff, "192.0.2.123")
	diff, err = mgr.Diff("testuser")
	require.NoError(t, err)
	assert.Contains(t, diff, "keep-this-change")
	unchanged, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, committed, unchanged)
	rows := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
	require.Len(t, rows, 1)
	assert.Equal(t, "testuser", rows[0].Actor)
	assert.Equal(t, audit.Web, rows[0].Surface)
	assert.Contains(t, rows[0].Detail, "192.0.2.123")
	assert.NotContains(t, rows[0].Detail, "keep-this-change")
	refused := graphicalActionRequest(t, handler, "discard-path", "bgp/router-id", nil)
	requireStatus(t, http.StatusBadRequest, refused)
	assert.Len(t, recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard}), 1)
}

// TestGraphicalSelectedPathDiscardRefreshesRemovedEntry proves that discarding
// a new entry returns the parent list rather than stale entry controls.
func TestGraphicalSelectedPathDiscardRefreshesRemovedEntry(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	path := []string{"bgp", "group", "discard/key"}
	require.NoError(t, mgr.SetValue("testuser", path, "description", "temporary"))
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "retained"}, "description", "kept"))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleGraphicalAction(mgr, schema, renderer, nil, nil, nil)
	response := graphicalActionRequest(t, handler, "discard-path", encodeConfigPath(path), nil)
	requireStatus(t, http.StatusOK, response)
	assert.Nil(t, walkTree(mgr.Tree("testuser"), schema, path))
	assert.Equal(t, "/show/bgp/group/", response.Header().Get("HX-Push-Url"))
	assert.Contains(t, response.Body.String(), "retained")
	assert.NotContains(t, response.Body.String(), `id="graphical-entry-target"`)
}

// TestGraphicalSelectedItemDeleteStagesOnlyCandidate exercises the already
// shared delete producer while the new button makes it reachable graphically.
func TestGraphicalSelectedItemDeleteStagesOnlyCandidate(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	committed, err := mgr.committedConfig()
	require.NoError(t, err)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "remove"}, "description", "removed"))
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "retained"}, "description", "kept"))
	handler := HandleGraphicalAction(mgr, schema, nil, nil, nil, nil)
	response := graphicalActionRequest(t, handler, "delete", "bgp/group/remove", nil)
	requireStatus(t, http.StatusOK, response)
	assert.Nil(t, walkTree(mgr.Tree("testuser"), schema, []string{"bgp", "group", "remove"}))
	assert.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp", "group", "retained"}), "kept")
	unchanged, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, committed, unchanged)
}

// TestGraphicalSelectedItemReadOnlyHidesAndRefusesMutations pairs hidden
// controls with forged HTTP submissions that must preserve the candidate.
func TestGraphicalSelectedItemReadOnlyHidesAndRefusesMutations(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	path := []string{"bgp", "group", "selected"}
	require.NoError(t, mgr.SetValue("testuser", path, "description", "retained"))
	before := config.Serialize(mgr.Tree("testuser"), schema)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	view := HandleFragment(renderer, schema, tree, mgr, false, fakeAuthorizer{})
	response := httptest.NewRecorder()
	view.ServeHTTP(response, authedRequest(t, http.MethodGet, "/show/"+encodeConfigPath(path)+"/", nil))
	requireStatus(t, http.StatusOK, response)
	assert.NotContains(t, response.Body.String(), `id="graphical-delete-selected"`)
	assert.NotContains(t, response.Body.String(), `id="graphical-discard-selected"`)
	handler := HandleGraphicalAction(mgr, schema, renderer, fakeAuthorizer{}, nil, nil)
	for _, action := range []string{"delete", "discard-path"} {
		forged := graphicalActionRequest(t, handler, action, encodeConfigPath(path), nil)
		requireStatus(t, http.StatusForbidden, forged)
	}
	assert.Equal(t, before, config.Serialize(mgr.Tree("testuser"), schema))
}

// TestGraphicalSelectedRootActionsAreRefused keeps selected-item operations
// from becoming an accidental delete-all or discard-all at the root.
func TestGraphicalSelectedRootActionsAreRefused(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "retained"}, "description", "retained"))
	before := config.Serialize(mgr.Tree("testuser"), schema)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	handler := HandleGraphicalAction(mgr, schema, nil, nil, recorder, nil)
	for _, action := range []string{"delete", "discard-path"} {
		response := graphicalActionRequest(t, handler, action, "", nil)
		requireStatus(t, http.StatusBadRequest, response)
	}
	assert.Equal(t, before, config.Serialize(mgr.Tree("testuser"), schema))
	assert.Empty(t, recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard}))
}
