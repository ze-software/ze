// Design: docs/architecture/web-interface.md -- authorized recovery of corrupt pending changes.
package web

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestConfigRecoveryRoutesRegistered proves that an operator can reach recovery
// without first opening the broken candidate through another handler.
func TestConfigRecoveryRoutesRegistered(t *testing.T) {
	patterns := make(map[string]WrapKind)
	for _, route := range RegisteredWebRoutes() {
		patterns[route.Pattern] = route.Wrap
	}
	getWrap, ok := patterns["GET /config/recover"]
	assert.True(t, ok, "the recovery form must be reachable without a candidate")
	assert.Equal(t, WrapAuth, getWrap)
	postWrap, ok := patterns["POST /config/recover"]
	assert.True(t, ok, "the confirmed recovery action must have its own mutation route")
	assert.Equal(t, WrapMutation, postWrap)
}

const corruptRecoveryContent = "set bgp { private-unparseable-secret"

func recoveryFingerprint(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func recoveryHandlerForTest(t *testing.T, mgr *EditorManager) http.HandlerFunc {
	t.Helper()
	renderer, err := NewRenderer()
	require.NoError(t, err)
	return HandleConfigRecovery(mgr, renderer, fakeAuthorizer{allowEdit: true}, nil)
}

// TestConfigRecoveryFormNeedsNoCandidate opens the independent form while the
// normal editor is unavailable, without displaying secrets or changing storage.
func TestConfigRecoveryFormNeedsNoCandidate(t *testing.T) {
	mgr := newTestEditorManager(t)
	path := cli.ChangePath(mgr.configPath, "testuser")
	require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
	mgr.editorFactory = func(any, string) (contract.Editor, error) {
		t.Fatal("the recovery form must not open an editor")
		return nil, nil
	}
	response := httptest.NewRecorder()
	recoveryHandlerForTest(t, mgr)(response, authedRequest(t, http.MethodGet, "/config/recover", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	body := response.Body.String()
	assert.Contains(t, body, `action="/config/recover"`)
	assert.Contains(t, body, `id="config-recovery-confirm"`)
	assert.Contains(t, body, recoveryFingerprint(corruptRecoveryContent))
	assert.Contains(t, body, "SSH and console")
	assert.Contains(t, body, "Saved draft changes")
	assert.NotContains(t, body, "private-unparseable-secret")
	data, err := mgr.store.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, corruptRecoveryContent, string(data))
	assert.Empty(t, mgr.sessions)
}

// TestConfigRecoveryFormExplainsSavedStructuralInstructions keeps recovery's
// warning honest when Save draft retained operations in the pending file.
func TestConfigRecoveryFormExplainsSavedStructuralInstructions(t *testing.T) {
	mgr := newTestEditorManager(t)
	require.NoError(t, mgr.store.WriteFile(cli.ChangePath(mgr.configPath, "testuser"), []byte(corruptRecoveryContent), 0o600))
	response := httptest.NewRecorder()
	recoveryHandlerForTest(t, mgr)(response, authedRequest(t, http.MethodGet, "/config/recover", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "structural instructions retained after Save draft")
	assert.Contains(t, response.Body.String(), "The separate saved draft file is kept, and running configuration stays unchanged")
	assert.Contains(t, response.Body.String(), "those instructions will be discarded")
	assert.Contains(t, response.Body.String(), "all changes and structural instructions in this unreadable file will be lost")
}

// TestConfigRecoveryRequiresExplicitConfirmation refuses an incomplete,
// ambiguous or redirected form before it removes any stored intent.
func TestConfigRecoveryRequiresExplicitConfirmation(t *testing.T) {
	for _, form := range []url.Values{
		{"fingerprint": {recoveryFingerprint(corruptRecoveryContent)}},
		{"confirm": {"discard"}},
		{"confirm": {"discard", "discard"}, "fingerprint": {recoveryFingerprint(corruptRecoveryContent)}},
		{"confirm": {"discard"}, "fingerprint": {recoveryFingerprint(corruptRecoveryContent)}, "username": {"other"}},
	} {
		t.Run(form.Encode(), func(t *testing.T) {
			mgr := newTestEditorManager(t)
			path := cli.ChangePath(mgr.configPath, "testuser")
			require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
			response := httptest.NewRecorder()
			recoveryHandlerForTest(t, mgr)(response, authedRequest(t, http.MethodPost, "/config/recover", form))
			assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			data, err := mgr.store.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, corruptRecoveryContent, string(data))
		})
	}
}

// TestConfigRecoveryDiscardsOnlyConfirmedAccountFile preserves the running
// configuration, saved draft and other account while auditing the real action.
func TestConfigRecoveryDiscardsOnlyConfirmedAccountFile(t *testing.T) {
	mgr := newTestEditorManager(t)
	path := cli.ChangePath(mgr.configPath, "testuser")
	otherPath := cli.ChangePath(mgr.configPath, "other")
	require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
	require.NoError(t, mgr.store.WriteFile(otherPath, []byte("set bgp router-id 192.0.2.9\n"), 0o600))
	draftPath := cli.DraftPath(mgr.configPath)
	require.NoError(t, mgr.store.WriteFile(draftPath, []byte("set bgp router-id 192.0.2.8\n"), 0o600))
	draftBefore, err := mgr.store.ReadFile(draftPath)
	require.NoError(t, err)
	committedBefore, err := mgr.committedConfig()
	require.NoError(t, err)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	response := httptest.NewRecorder()
	HandleConfigRecovery(mgr, renderer, fakeAuthorizer{allowEdit: true}, recorder)(response,
		authedRequest(t, http.MethodPost, "/config/recover", url.Values{
			"confirm": {"discard"}, "fingerprint": {recoveryFingerprint(corruptRecoveryContent)},
		}))
	require.Equal(t, http.StatusSeeOther, response.Code, response.Body.String())
	assert.Equal(t, "/show/", response.Header().Get("Location"))
	assert.False(t, mgr.store.Exists(path))
	other, err := mgr.store.ReadFile(otherPath)
	require.NoError(t, err)
	assert.Equal(t, "set bgp router-id 192.0.2.9\n", string(other))
	draftAfter, err := mgr.store.ReadFile(draftPath)
	require.NoError(t, err)
	assert.Equal(t, draftBefore, draftAfter)
	committedAfter, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, committedBefore, committedAfter)
	rows := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
	require.Len(t, rows, 1)
	assert.Equal(t, "testuser", rows[0].Actor)
	assert.Equal(t, audit.Web, rows[0].Surface)
	assert.Contains(t, rows[0].Detail, "unreadable")
	assert.Contains(t, rows[0].Detail, recoveryFingerprint(corruptRecoveryContent))
	assert.NotContains(t, rows[0].Detail, "private-unparseable-secret")
}

// TestConfigRecoveryRefusesReplacement requires a fresh confirmation when a
// different corrupt file replaced the one the administrator reviewed.
func TestConfigRecoveryRefusesReplacement(t *testing.T) {
	mgr := newTestEditorManager(t)
	path := cli.ChangePath(mgr.configPath, "testuser")
	const replacement = "set bgp { replacement-secret"
	require.NoError(t, mgr.store.WriteFile(path, []byte(replacement), 0o600))
	response := httptest.NewRecorder()
	recoveryHandlerForTest(t, mgr)(response, authedRequest(t, http.MethodPost, "/config/recover", url.Values{
		"confirm": {"discard"}, "fingerprint": {recoveryFingerprint(corruptRecoveryContent)},
	}))
	assert.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	data, err := mgr.store.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, replacement, string(data))
}

// TestConfigRecoveryAuthorization refuses readers, anonymous callers and
// cross-origin mutation without removing the file or emitting a success audit.
func TestConfigRecoveryAuthorization(t *testing.T) {
	for _, test := range []struct {
		name, username, origin string
		allow                  bool
		status                 int
	}{
		{"reader", "alice", "", false, http.StatusForbidden},
		{"anonymous", "", "", true, http.StatusUnauthorized},
		{"cross-origin", "alice", "https://other.example", true, http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			mgr := newTestEditorManager(t)
			path := cli.ChangePath(mgr.configPath, "alice")
			require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
			renderer, err := NewRenderer()
			require.NoError(t, err)
			recorder, err := audit.NewMemory(100)
			require.NoError(t, err)
			handler := RequireSameOrigin(HandleConfigRecovery(mgr, renderer, fakeAuthorizer{allowEdit: test.allow}, recorder))
			request := postConfigRequest(t, "/config/recover", url.Values{
				"confirm": {"discard"}, "fingerprint": {recoveryFingerprint(corruptRecoveryContent)},
			}, test.username)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			assert.Equal(t, test.status, response.Code, response.Body.String())
			assert.True(t, mgr.store.Exists(path))
			assert.Empty(t, recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard}))
		})
	}
}

// TestConfigRecoveryRefreshesCachedEditor keeps already admitted editor
// references valid while removing the lost unsaved value from their candidate.
func TestConfigRecoveryRefreshesCachedEditor(t *testing.T) {
	mgr := newTestEditorManager(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.99"))
	before, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	path := cli.ChangePath(mgr.configPath, "testuser")
	require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
	response := httptest.NewRecorder()
	recoveryHandlerForTest(t, mgr)(response, authedRequest(t, http.MethodPost, "/config/recover", url.Values{
		"confirm": {"discard"}, "fingerprint": {recoveryFingerprint(corruptRecoveryContent)},
	}))
	require.Equal(t, http.StatusSeeOther, response.Code, response.Body.String())
	after, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	assert.Same(t, before, after)
	assert.Zero(t, mgr.ChangeCount("testuser"))
	assert.NotContains(t, after.editor.WorkingContent(), "192.0.2.99")
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.100"))
	assert.Equal(t, 1, mgr.ChangeCount("testuser"))
}

// TestConfigRecoveryBusyEditorRefusesWithoutWaiting proves recovery cannot
// deadlock a commit that holds the user lock before reading the manager.
func TestConfigRecoveryBusyEditorRefusesWithoutWaiting(t *testing.T) {
	mgr := newTestEditorManager(t)
	us, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	path := cli.ChangePath(mgr.configPath, "testuser")
	require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
	us.mu.Lock()
	defer us.mu.Unlock()
	response := httptest.NewRecorder()
	recoveryHandlerForTest(t, mgr)(response, authedRequest(t, http.MethodPost, "/config/recover", url.Values{
		"confirm": {"discard"}, "fingerprint": {recoveryFingerprint(corruptRecoveryContent)},
	}))
	require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "busy")
	data, err := mgr.store.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, corruptRecoveryContent, string(data))
}

// TestConfigRecoveryKeepsSavedOwners rebuilds the cached candidate from valid
// saved ownership while dropping only the unreadable unsaved file.
func TestConfigRecoveryKeepsSavedOwners(t *testing.T) {
	mgr := newTestEditorManager(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.80"))
	us, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	require.NoError(t, us.editor.SaveDraft())
	require.NoError(t, mgr.SetValue("other", []string{"bgp", "group", "other"}, "description", "keep-other-saved"))
	other, err := mgr.GetOrCreate("other")
	require.NoError(t, err)
	require.NoError(t, other.editor.SaveDraft())
	require.NoError(t, mgr.SetValue("other", []string{"bgp", "group", "other"}, "description", "keep-other-unsaved"))
	otherPath := cli.ChangePath(mgr.configPath, "other")
	otherBefore, err := mgr.store.ReadFile(otherPath)
	require.NoError(t, err)
	draftPath := cli.DraftPath(mgr.configPath)
	draftBefore, err := mgr.store.ReadFile(draftPath)
	require.NoError(t, err)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "session", "asn"}, "local", "65001"))
	path := cli.ChangePath(mgr.configPath, "testuser")
	require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
	response := httptest.NewRecorder()
	recoveryHandlerForTest(t, mgr)(response, authedRequest(t, http.MethodPost, "/config/recover", url.Values{
		"confirm": {"discard"}, "fingerprint": {recoveryFingerprint(corruptRecoveryContent)},
	}))
	require.Equal(t, http.StatusSeeOther, response.Code, response.Body.String())
	assert.Contains(t, us.editor.WorkingContent(), "192.0.2.80")
	assert.NotContains(t, us.editor.WorkingContent(), "65001")
	assert.NotContains(t, us.editor.WorkingContent(), "keep-other")
	assert.Equal(t, 1, mgr.ChangeCount("testuser"), "saved ownership remains pending")
	otherAfter, err := mgr.store.ReadFile(otherPath)
	require.NoError(t, err)
	assert.Equal(t, otherBefore, otherAfter)
	draftAfter, err := mgr.store.ReadFile(draftPath)
	require.NoError(t, err)
	assert.Equal(t, draftBefore, draftAfter)
}

// TestConfigRecoveryAuditsRemovalBeforeRefreshFailure distinguishes a
// completed recovery from a remaining malformed saved draft.
func TestConfigRecoveryAuditsRemovalBeforeRefreshFailure(t *testing.T) {
	mgr := newTestEditorManager(t)
	_, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	path := cli.ChangePath(mgr.configPath, "testuser")
	require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
	require.NoError(t, mgr.store.WriteFile(cli.DraftPath(mgr.configPath), []byte("set bgp { malformed-saved-draft"), 0o600))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	response := httptest.NewRecorder()
	HandleConfigRecovery(mgr, renderer, fakeAuthorizer{allowEdit: true}, recorder)(response,
		authedRequest(t, http.MethodPost, "/config/recover", url.Values{
			"confirm": {"discard"}, "fingerprint": {recoveryFingerprint(corruptRecoveryContent)},
		}))
	require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "was discarded")
	assert.False(t, mgr.store.Exists(path))
	assert.Len(t, recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard}), 1)
}

// TestConfigRecoveryErrorPagesLinkToIndependentForm checks both graphical
// shells through their normal GET entry points without exposing corrupt text.
func TestConfigRecoveryErrorPagesLinkToIndependentForm(t *testing.T) {
	for _, shell := range []string{"finder", "workbench"} {
		t.Run(shell, func(t *testing.T) {
			mgr := newTestEditorManager(t)
			path := cli.ChangePath(mgr.configPath, "testuser")
			require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
			renderer, err := NewRenderer()
			require.NoError(t, err)
			handler := HandleFragment(renderer, mgr.schema, config.NewTree(), mgr, false, fakeAuthorizer{allowEdit: true})
			if shell == "workbench" {
				handler = HandleWorkbench(renderer, mgr.schema, config.NewTree(), mgr, false, WithAuthorizer(fakeAuthorizer{allowEdit: true}))
			}
			response := httptest.NewRecorder()
			handler(response, authedRequest(t, http.MethodGet, "/show/", nil))
			assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			assert.Contains(t, response.Body.String(), `href="/config/recover"`)
			assert.NotContains(t, response.Body.String(), "private-unparseable-secret")
		})
	}
}
