// Design: docs/architecture/web-interface.md -- authenticated candidate ownership.

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
)

// TestEditorIdentityRefusesInvalidOwnersBeforeFactories proves the web manager
// applies the shared account contract before allocating either editor identity.
func TestEditorIdentityRefusesInvalidOwnersBeforeFactories(t *testing.T) {
	mgr, _, _, _ := setupCLITerminalYANGTest(t)
	editorFactory := mgr.editorFactory
	sessionFactory := mgr.editSessionFactory
	editorCalls, sessionCalls := 0, 0
	mgr.editorFactory = func(store any, path string) (contract.Editor, error) {
		editorCalls++
		return editorFactory(store, path)
	}
	mgr.editSessionFactory = func(username, origin string) contract.EditSession {
		sessionCalls++
		return sessionFactory(username, origin)
	}
	before, err := mgr.store.List(filepath.Dir(mgr.configPath))
	require.NoError(t, err)
	for _, username := range []string{"", ".", "..", "operator@east%team", "../other"} {
		_, err := mgr.GetOrCreate(username)
		assert.Error(t, err, "unsupported owner %q must not open an editor", username)
	}
	assert.Zero(t, editorCalls)
	assert.Zero(t, sessionCalls)
	assert.Empty(t, mgr.sessions)
	after, err := mgr.store.List(filepath.Dir(mgr.configPath))
	require.NoError(t, err)
	assert.Equal(t, before, after)
	valid, err := mgr.GetOrCreate("operator-east.team_2")
	require.NoError(t, err)
	require.NotNil(t, valid)
	assert.Equal(t, 1, editorCalls)
	assert.Equal(t, 1, sessionCalls)
	assert.Contains(t, valid.editor.SessionID(), "operator-east.team_2@web%")
}

// TestEditorIdentityHTTPRefusesInvalidPrincipalBeforeCandidate proves an
// authenticated graphical action cannot create an unrepresentable owner, while
// an account using the permitted punctuation still reaches shared inspection.
func TestEditorIdentityHTTPRefusesInvalidPrincipalBeforeCandidate(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	handler := HandleGraphicalAction(mgr, schema, nil, nil, nil, nil)
	before, err := mgr.store.List(filepath.Dir(mgr.configPath))
	require.NoError(t, err)
	invalid := "operator@east%team"
	request := authedRequest(t, http.MethodPost, "/config/action", url.Values{"action": {"inspect"}, "source": {"candidate"}, "format": {"config"}, "path": {"bgp"}})
	request = request.WithContext(withUsername(request.Context(), invalid))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	requireStatus(t, http.StatusInternalServerError, response)
	assert.Contains(t, response.Body.String(), "username")
	assert.Contains(t, response.Body.String(), "fix:")
	assert.NotContains(t, response.Body.String(), "1.2.3.4")
	assert.Empty(t, mgr.sessions)
	after, err := mgr.store.List(filepath.Dir(mgr.configPath))
	require.NoError(t, err)
	assert.Equal(t, before, after)
	validRequest := authedRequest(t, http.MethodPost, "/config/action", url.Values{"action": {"inspect"}, "source": {"candidate"}, "format": {"config"}, "path": {"bgp"}})
	validRequest = validRequest.WithContext(withUsername(validRequest.Context(), "operator-east.team_2"))
	validResponse := httptest.NewRecorder()
	handler.ServeHTTP(validResponse, validRequest)
	requireStatus(t, http.StatusOK, validResponse)
	assert.Contains(t, validResponse.Body.String(), "1.2.3.4")
}
