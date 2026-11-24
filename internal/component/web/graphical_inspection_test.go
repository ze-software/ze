// Design: docs/architecture/web-components.md -- graphical configuration inspection.

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestGraphicalInspectionReadOnlyControlsReachFinder checks the real default
// editor response, including inspection controls outside the mutation branch.
func TestGraphicalInspectionReadOnlyControlsReachFinder(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleFragment(renderer, schema, tree, mgr, false, fakeAuthorizer{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authedRequest(t, http.MethodGet, "/show/bgp/", nil))
	requireStatus(t, http.StatusOK, response)
	for _, id := range []string{"graphical-inspect-source", "graphical-inspect-format", "graphical-inspect", "graphical-inspect-metadata", "graphical-compare", "graphical-sessions", "graphical-preference"} {
		assert.Contains(t, response.Body.String(), `id="`+id+`"`)
	}
	assert.NotContains(t, response.Body.String(), `id="graphical-delete-selected"`)
	assert.NotContains(t, response.Body.String(), `id="graphical-commit-now"`)
}

// TestGraphicalInspectionSourcesUseSharedProducer distinguishes the candidate,
// saved draft and running configuration at one selected schema path.
func TestGraphicalInspectionSourcesUseSharedProducer(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.21"))
	require.NoError(t, mgr.SaveDraft("testuser"))
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.22"))
	before := config.Serialize(mgr.snapshotTree("testuser"), schema)
	committed, err := mgr.committedConfig()
	require.NoError(t, err)
	handler := HandleGraphicalAction(mgr, schema, nil, fakeAuthorizer{}, nil, nil)
	for _, source := range []struct {
		form string
		word string
		want string
	}{
		{"candidate", "", "192.0.2.22"},
		{"confirmed", " confirmed", "1.2.3.4"},
		{"saved", " saved", "192.0.2.21"},
	} {
		for _, format := range []string{"tree", "config"} {
			shared, err := mgr.ConfigCommand("testuser", contract.ConfigCommandRequest{Line: "show" + source.word + " | format " + format, Path: []string{"bgp"}})
			require.NoError(t, err)
			require.Contains(t, shared.Output, source.want)
			response := graphicalActionRequest(t, handler, "inspect", "bgp", url.Values{"source": {source.form}, "format": {format}})
			requireStatus(t, http.StatusOK, response)
			want := shared.Output
			if shared.Feedback != "" {
				want = shared.Feedback + "\n" + shared.Output
			}
			assert.Equal(t, want, response.Body.String())
		}
	}
	assert.Equal(t, before, config.Serialize(mgr.snapshotTree("testuser"), schema))
	unchanged, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, committed, unchanged)
}

// TestGraphicalInspectionMetadataUsesCandidateOnly exposes precisely the
// shared metadata pipes and refuses source selections those pipes ignore.
func TestGraphicalInspectionMetadataUsesCandidateOnly(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "metadata"}, "description", "candidate-metadata"))
	_, err := mgr.ConfigCommand("testuser", contract.ConfigCommandRequest{Line: "deactivate bgp group metadata"})
	require.NoError(t, err)
	handler := HandleGraphicalAction(mgr, schema, nil, fakeAuthorizer{}, nil, nil)
	for _, view := range []struct {
		name string
		pipe string
	}{
		{"blame", "blame"}, {"active", "active"}, {"inactive", "inactive"},
		{"changes", "changes"}, {"allchanges", "changes all"}, {"errors", "errors"},
	} {
		shared, err := mgr.ConfigCommand("testuser", contract.ConfigCommandRequest{Line: "show | " + view.pipe, Path: []string{"bgp"}})
		require.NoError(t, err)
		response := graphicalActionRequest(t, handler, "inspect-metadata", "bgp", url.Values{"view": {view.name}})
		requireStatus(t, http.StatusOK, response)
		want := shared.Output
		if shared.Feedback != "" {
			want = shared.Feedback + "\n" + shared.Output
		}
		assert.Equal(t, want, response.Body.String())
	}
	forged := graphicalActionRequest(t, handler, "inspect-metadata", "bgp", url.Values{"view": {"blame"}, "source": {"saved"}})
	requireStatus(t, http.StatusBadRequest, forged)
	assert.NotContains(t, forged.Body.String(), "candidate-metadata")
}

// TestGraphicalInspectionCompareTargetsUseSharedProducer proves the normal
// running and draft comparisons do not change the candidate or committed data.
func TestGraphicalInspectionCompareTargetsUseSharedProducer(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.31"))
	require.NoError(t, mgr.SaveDraft("testuser"))
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.32"))
	before := config.Serialize(mgr.snapshotTree("testuser"), schema)
	committed, err := mgr.committedConfig()
	require.NoError(t, err)
	handler := HandleGraphicalAction(mgr, schema, nil, fakeAuthorizer{}, nil, nil)
	for _, target := range []string{"confirmed", "saved"} {
		shared, err := mgr.ConfigCommand("testuser", contract.ConfigCommandRequest{Line: "show | compare " + target + " | format config", Path: []string{"bgp"}})
		require.NoError(t, err)
		require.Contains(t, shared.Output, "192.0.2.32")
		response := graphicalActionRequest(t, handler, "compare", "bgp", url.Values{"target": {target}, "format": {"config"}})
		requireStatus(t, http.StatusOK, response)
		assert.Equal(t, shared.Output, response.Body.String())
	}
	assert.Equal(t, before, config.Serialize(mgr.snapshotTree("testuser"), schema))
	unchanged, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, committed, unchanged)
}

// TestGraphicalInspectionStableRevisionCompare retains the chosen saved key
// while another commit moves its ordinal in the history list.
func TestGraphicalInspectionStableRevisionCompare(t *testing.T) {
	mgr, schema, _ := newWindowEditorManager(t)
	handler := HandleGraphicalAction(mgr, schema, nil, nil, nil, nil)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.41"))
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "stable-revision"}, "description", "selected-baseline-only"))
	require.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp"}), "selected-baseline-only", "candidate must contain the distinguishing baseline before commit")
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "now", "", nil))
	committed, err := mgr.committedConfig()
	require.NoError(t, err)
	require.Contains(t, string(committed), "selected-baseline-only", "first commit must publish the distinguishing baseline")
	backups, err := mgr.ListBackups("testuser")
	require.NoError(t, err)
	require.NotEmpty(t, backups)
	selected := backups[0].Path
	selectedBody, err := storage.ReadVersionEntry(mgr.store, mgr.configPath, selected)
	require.NoError(t, err)
	t.Logf("selected history key %s contains:\n%s", selected, selectedBody)
	require.Contains(t, string(selectedBody), "selected-baseline-only", "selected archive must carry the baseline used by this test")
	require.NoError(t, mgr.DeleteByPath("testuser", []string{"bgp", "group"}, "stable-revision"))
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.42"))
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, handler, "now", "", nil))
	archivedAgain, err := storage.ReadVersionEntry(mgr.store, mgr.configPath, selected)
	require.NoError(t, err)
	require.Equal(t, selectedBody, archivedAgain, "newer commits must preserve the bytes behind the selected stable key")
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.43"))
	response := graphicalActionRequest(t, handler, "compare", "bgp", url.Values{"target": {"revision"}, "revision": {selected}, "format": {"config"}})
	requireStatus(t, http.StatusOK, response)
	assert.Contains(t, response.Body.String(), "selected-baseline-only")
	assert.Contains(t, response.Body.String(), "192.0.2.43")
	assert.NotContains(t, response.Body.String(), "192.0.2.42")
	refused := graphicalActionRequest(t, handler, "compare", "bgp", url.Values{"target": {"revision"}, "revision": {"not-a-current-revision"}, "format": {"config"}})
	requireStatus(t, http.StatusBadRequest, refused)
	assert.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp"}), "192.0.2.43")
}

// TestGraphicalInspectionSessionsRetainExactID renders the typed session ID
// supplied by the editor; read-only users can inspect but cannot disconnect.
func TestGraphicalInspectionSessionsRetainExactID(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.51"))
	us, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	sid := us.editor.SessionID()
	require.NotEmpty(t, sid)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	readOnly := HandleGraphicalAction(mgr, schema, renderer, fakeAuthorizer{}, nil, nil)
	response := graphicalActionRequest(t, readOnly, "sessions", "bgp", nil)
	requireStatus(t, http.StatusOK, response)
	assert.Contains(t, response.Body.String(), sid)
	assert.NotContains(t, response.Body.String(), `id="graphical-disconnect-session"`)
	denied := graphicalActionRequest(t, readOnly, "disconnect", "bgp", url.Values{"session": {sid}})
	requireStatus(t, http.StatusForbidden, denied)
	writable := HandleGraphicalAction(mgr, schema, nil, nil, nil, nil)
	own := graphicalActionRequest(t, writable, "disconnect", "bgp", url.Values{"session": {sid}})
	requireStatus(t, http.StatusBadRequest, own)
	assert.Contains(t, own.Body.String(), "own session")
	assert.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp"}), "192.0.2.51")
}

// TestGraphicalInspectionPreferencesArePerUser changes the shared editor
// preference through the graphical form without granting config mutation.
func TestGraphicalInspectionPreferencesArePerUser(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	_, err := mgr.GetOrCreate("other-user")
	require.NoError(t, err)
	otherBefore, err := mgr.ConfigCommand("other-user", contract.ConfigCommandRequest{Line: "option author"})
	require.NoError(t, err)
	handler := HandleGraphicalAction(mgr, schema, nil, fakeAuthorizer{}, nil, nil)
	response := graphicalActionRequest(t, handler, "preference", "bgp", url.Values{"option": {"author"}, "value": {"enable"}})
	requireStatus(t, http.StatusOK, response)
	own, err := mgr.ConfigCommand("testuser", contract.ConfigCommandRequest{Line: "option author"})
	require.NoError(t, err)
	assert.Contains(t, own.Feedback, "enabled")
	otherAfter, err := mgr.ConfigCommand("other-user", contract.ConfigCommandRequest{Line: "option author"})
	require.NoError(t, err)
	assert.Equal(t, otherBefore.Feedback, otherAfter.Feedback)
	refused := graphicalActionRequest(t, handler, "preference", "bgp", url.Values{"option": {"author | delete bgp"}, "value": {"enable"}})
	requireStatus(t, http.StatusBadRequest, refused)
	assert.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp"}), "1.2.3.4")
}

// TestGraphicalInspectionUserCompareKeepsExactMetadataOwner uses a username
// using the permitted account punctuation and compares through the actual CLI owner.
func TestGraphicalInspectionUserCompareKeepsExactMetadataOwner(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	username := "operator-east.team"
	require.NoError(t, mgr.SetValue(username, []string{"bgp"}, "router-id", "192.0.2.61"))
	shared, err := mgr.ConfigCommand(username, contract.ConfigCommandRequest{Line: `show | compare "operator-east.team" | format config`, Path: []string{"bgp"}})
	require.NoError(t, err)
	require.Contains(t, shared.Output, "192.0.2.61")
	renderer, err := NewRenderer()
	require.NoError(t, err)
	view := HandleFragment(renderer, schema, mgr.snapshotTree(username), mgr, false, fakeAuthorizer{})
	viewRequest := authedRequest(t, http.MethodGet, "/show/bgp/", nil)
	viewRequest = viewRequest.WithContext(withUsername(viewRequest.Context(), username))
	page := httptest.NewRecorder()
	view.ServeHTTP(page, viewRequest)
	requireStatus(t, http.StatusOK, page)
	assert.Contains(t, page.Body.String(), `value="operator-east.team"`)
	handler := HandleGraphicalAction(mgr, schema, nil, fakeAuthorizer{}, nil, nil)
	for _, owner := range []string{username, "operator"} {
		request := authedRequest(t, http.MethodPost, "/config/action", url.Values{"action": {"compare"}, "path": {"bgp"}, "target": {"user"}, "username": {owner}, "format": {"config"}})
		request = request.WithContext(withUsername(request.Context(), username))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if owner == username {
			requireStatus(t, http.StatusOK, response)
			assert.Equal(t, shared.Output, response.Body.String())
		} else {
			requireStatus(t, http.StatusBadRequest, response)
		}
	}
	assert.Contains(t, mgr.ContentAtPath(username, []string{"bgp"}), "192.0.2.61")
}

// TestGraphicalInspectionDisconnectsExactOtherSession verifies the posted
// identifier removes only the selected other session's pending changes.
func TestGraphicalInspectionDisconnectsExactOtherSession(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.71"))
	require.NoError(t, mgr.SetValue("other-account", []string{"bgp", "group", "other"}, "description", "other-pending"))
	other, err := mgr.GetOrCreate("other-account")
	require.NoError(t, err)
	sid := other.editor.SessionID()
	current, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	require.NotEmpty(t, current.editor.PendingChanges(sid))
	committed, err := mgr.committedConfig()
	require.NoError(t, err)
	handler := HandleGraphicalAction(mgr, schema, nil, nil, nil, nil)
	response := graphicalActionRequest(t, handler, "disconnect", "bgp", url.Values{"session": {sid}})
	requireStatus(t, http.StatusOK, response)
	assert.Contains(t, response.Body.String(), sid)
	assert.Empty(t, current.editor.PendingChanges(sid))
	assert.Contains(t, mgr.ContentAtPath("testuser", []string{"bgp"}), "192.0.2.71")
	unchanged, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, committed, unchanged)
}
