// Design: docs/architecture/web-components.md -- graphical configuration actions.

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestGraphicalActionsCopyRenameAndActivation follows the graphical HTTP
// adapter through the shared editor, checking the candidate after each action.
func TestGraphicalActionsCopyRenameAndActivation(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "source"}, "description", "original"))
	handler := HandleGraphicalAction(mgr, schema, nil, nil, nil, nil)
	copy := graphicalActionRequest(t, handler, "copy", "bgp/group/source", url.Values{"target": {"copied"}})
	requireStatus(t, http.StatusOK, copy)
	assert.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp", "group", "copied"}), "original")
	rename := graphicalActionRequest(t, handler, "rename", "bgp/group/copied", url.Values{"target": {"renamed"}})
	requireStatus(t, http.StatusOK, rename)
	assert.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp", "group", "renamed"}), "original")
	assert.NotContains(t, mgr.ContentAtPath("testuser", []string{"bgp"}), "group copied")
	deactivated := graphicalActionRequest(t, handler, "deactivate", "bgp/group/renamed", nil)
	requireStatus(t, http.StatusOK, deactivated)
	assert.True(t, walkTree(mgr.snapshotTree("testuser"), schema, []string{"bgp", "group", "renamed"}).IsInactive())
	reactivated := graphicalActionRequest(t, handler, "activate", "bgp/group/renamed", nil)
	requireStatus(t, http.StatusOK, reactivated)
	assert.False(t, walkTree(mgr.snapshotTree("testuser"), schema, []string{"bgp", "group", "renamed"}).IsInactive())
	assert.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp", "group", "source"}), "original")
}

// TestGraphicalActionsRespectPermissionsAndRejectUnknownActions proves neither
// a refused mutation nor a forged operation changes the candidate.
func TestGraphicalActionsRespectPermissionsAndRejectUnknownActions(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "source"}, "description", "original"))
	before := config.Serialize(mgr.snapshotTree("testuser"), schema)
	denied := HandleGraphicalAction(mgr, schema, nil, fakeAuthorizer{allowEdit: false}, nil, nil)
	requireStatus(t, http.StatusForbidden, graphicalActionRequest(t, denied, "copy", "bgp/group/source", url.Values{"target": {"denied"}}))
	allowed := HandleGraphicalAction(mgr, schema, nil, nil, nil, nil)
	requireStatus(t, http.StatusBadRequest, graphicalActionRequest(t, allowed, "run", "bgp/group/source", nil))
	requireStatus(t, http.StatusBadRequest, graphicalActionRequest(t, allowed, "copy", "bgp/../source", url.Values{"target": {"bad"}}))
	assert.Equal(t, before, config.Serialize(mgr.snapshotTree("testuser"), schema))
}

// TestGraphicalActionsConfirmedCommitUsesDaemonWindow verifies the buttons use
// the shared window, including ownership refusal, explicit acceptance and abort.
func TestGraphicalActionsConfirmedCommitUsesDaemonWindow(t *testing.T) {
	mgr, schema, window := newWindowEditorManager(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "10.0.0.10"))
	handler := HandleGraphicalAction(mgr, schema, nil, nil, nil, nil)
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "verify", "", nil))
	_, open := window.Status()
	assert.False(t, open, "validation must apply nothing")
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "confirmed", "", url.Values{"seconds": {"60"}}))
	status, open := window.Status()
	require.True(t, open)
	assert.Equal(t, "testuser", status.User)
	req := authedRequest(t, http.MethodPost, "/config/action", url.Values{"action": {"accept"}})
	req = req.WithContext(withUsername(req.Context(), "other-user"))
	other := httptest.NewRecorder()
	handler.ServeHTTP(other, req)
	assert.Equal(t, http.StatusBadRequest, other.Code)
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "abort", "", nil))
	_, open = window.Status()
	assert.False(t, open)
	assert.NotContains(t, mgr.ContentAtPath("testuser", []string{"bgp"}), "10.0.0.10")
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "10.0.0.11"))
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "confirmed", "", url.Values{"seconds": {"60"}}))
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "accept", "", nil))
	_, open = window.Status()
	assert.False(t, open)
	assert.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp"}), "10.0.0.11")
}

// TestGraphicalSavedStructuralChangesOpenConfirmedCommit follows the browser's
// copy/rename/save/activation sequence and requires each HTTP action to succeed.
func TestGraphicalSavedStructuralChangesOpenConfirmedCommit(t *testing.T) {
	mgr, schema, window := newWindowEditorManager(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "actions-source"}, "description", "original"))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleGraphicalAction(mgr, schema, renderer, nil, nil, nil)
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "now", "", nil))
	for _, step := range []struct {
		action string
		path   string
		fields url.Values
	}{
		{"copy", "bgp/group/actions-source", url.Values{"target": {"actions-copy"}}},
		{"rename", "bgp/group/actions-copy", url.Values{"target": {"actions-final"}}},
		{"verify", "bgp/group/actions-final", nil},
		{"save", "bgp/group/actions-final", nil},
		{"deactivate", "bgp/group/actions-final", nil},
		{"activate", "bgp/group/actions-final", nil},
		{"confirmed", "bgp/group/actions-final", url.Values{"seconds": {"60"}}},
	} {
		response := graphicalActionRequest(t, handler, step.action, step.path, step.fields)
		require.Equal(t, http.StatusOK, response.Code, "%s: %s", step.action, response.Body.String())
	}
	status, open := window.Status()
	require.True(t, open)
	assert.Equal(t, "testuser", status.User)
	assert.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp", "group", "actions-final"}), "original")
	assert.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp", "group", "actions-source"}), "original")
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "abort", "", nil))
	assert.NotContains(t, mgr.ContentAtPath("testuser", []string{"bgp", "group"}), "actions-final")
}

func graphicalActionRequest(t *testing.T, handler http.Handler, action, path string, fields url.Values) *httptest.ResponseRecorder {
	t.Helper()
	if fields == nil {
		fields = make(url.Values)
	}
	fields.Set("action", action)
	fields.Set("path", path)
	req := authedRequest(t, http.MethodPost, "/config/action", fields)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// TestGraphicalActionsRestoreAStableRevision pins a selected history object,
// then adds another revision before restoring. Only the candidate changes.
func TestGraphicalActionsRestoreAStableRevision(t *testing.T) {
	mgr, schema, _ := newWindowEditorManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleGraphicalAction(mgr, schema, renderer, nil, nil, nil)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "10.0.0.10"))
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "now", "", nil))
	backups, err := mgr.ListBackups("testuser")
	require.NoError(t, err)
	require.NotEmpty(t, backups)
	selected := backups[0].Path
	history := graphicalActionRequest(t, handler, "history", "", nil)
	requireStatus(t, http.StatusOK, history)
	assert.Contains(t, history.Body.String(), `name="revision" value="`+selected+`"`)
	body, err := storage.ReadVersionEntry(mgr.store, mgr.configPath, selected)
	require.NoError(t, err)
	content := string(body)
	format := config.DetectFormat(content)
	var expected *config.Tree
	if format == config.FormatHierarchical {
		expected, err = config.NewParser(schema).Parse(content)
	} else {
		expected, err = config.ParseSetFormat(schema, content, format)
	}
	require.NoError(t, err)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "10.0.0.11"))
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "now", "", nil))
	committed, err := mgr.committedConfig()
	require.NoError(t, err)
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "rollback", "", url.Values{"revision": {selected}}))
	assert.Equal(t, config.Serialize(expected, schema), config.Serialize(mgr.snapshotTree("testuser"), schema))
	unchanged, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, committed, unchanged)
	before := config.Serialize(mgr.snapshotTree("testuser"), schema)
	requireStatus(t, http.StatusBadRequest, graphicalActionRequest(t, handler, "rollback", "", url.Values{"revision": {"/not-in-this-history"}}))
	assert.Equal(t, before, config.Serialize(mgr.snapshotTree("testuser"), schema))
}

// TestGraphicalActionsRefreshEntryNavigation proves successful structural
// changes replace the old controls and move the URL to the parent collection.
func TestGraphicalActionsRefreshEntryNavigation(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "source"}, "description", "original"))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleGraphicalAction(mgr, schema, renderer, nil, nil, nil)
	for _, action := range []string{"copy", "rename"} {
		rec := graphicalActionRequest(t, handler, action, "bgp/group/source", url.Values{"target": {action + "-destination"}})
		requireStatus(t, http.StatusOK, rec)
		assert.Equal(t, "/show/bgp/group/", rec.Header().Get("HX-Push-Url"))
		assert.Contains(t, rec.Body.String(), action+"-destination")
		assert.NotContains(t, rec.Body.String(), `id="graphical-entry-target"`)
		assert.NotContains(t, rec.Body.String(), `id="graphical-deactivate"`, "a collection is not a selected configurable item")
	}
}

// TestGraphicalActionsAppearWithoutPendingChanges checks the real Finder page
// and its configured read-only fallback, not only a component fixture.
func TestGraphicalActionsAppearWithoutPendingChanges(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	for _, writable := range []bool{false, true} {
		handler := HandleFragment(renderer, schema, tree, mgr, false, fakeAuthorizer{allowEdit: writable})
		req := authedRequest(t, http.MethodGet, "/show/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		requireStatus(t, http.StatusOK, rec)
		assert.Contains(t, rec.Body.String(), "Configuration actions")
		if writable {
			assert.Contains(t, rec.Body.String(), `id="graphical-validate"`)
		} else {
			assert.NotContains(t, rec.Body.String(), `id="graphical-validate"`)
			assert.Contains(t, rec.Body.String(), "read-only access")
		}
	}
}
