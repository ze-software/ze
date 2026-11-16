// Design: docs/architecture/web-components.md -- field edits after a reverted structural window.
package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGraphicalFieldSaveAfterAbortedStructuralWindow isolates the field POST
// from browser Enter dispatch after the saved copy/rename window is reverted.
func TestGraphicalFieldSaveAfterAbortedStructuralWindow(t *testing.T) {
	mgr, schema, _ := newWindowEditorManager(t)
	path := []string{"bgp", "group", "actions-source"}
	require.NoError(t, mgr.SetValue("testuser", path, "description", "original"))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	actions := HandleGraphicalAction(mgr, schema, renderer, nil, nil, nil)
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, actions, "now", "", nil))
	for _, step := range []struct {
		action, path string
		fields       url.Values
	}{
		{"copy", "bgp/group/actions-source", url.Values{"target": {"actions-copy"}}},
		{"rename", "bgp/group/actions-copy", url.Values{"target": {"actions-final"}}},
		{"verify", "bgp/group/actions-final", nil},
		{"save", "bgp/group/actions-final", nil},
		{"deactivate", "bgp/group/actions-final", nil},
		{"activate", "bgp/group/actions-final", nil},
		{"confirmed", "bgp/group/actions-final", url.Values{"seconds": {"60"}}},
		{"abort", "bgp/group/actions-final", nil},
	} {
		response := graphicalActionRequest(t, actions, step.action, step.path, step.fields)
		require.Equal(t, http.StatusOK, response.Code, "%s: %s", step.action, response.Body.String())
	}
	request := authedRequest(t, http.MethodPost, "/config/set/bgp/group/actions-source", url.Values{
		"leaf": {"description"}, "value": {"accepted"},
	})
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	HandleConfigSetWithAuthorizer(mgr, schema, renderer, nil).ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `value="accepted"`)
	assert.Contains(t, mgr.ContentAtPath("testuser", path), "description accepted")
	answer := terminalAsUser(t, mgr, "testuser", nil, "show bgp group actions-source")
	assert.Contains(t, answer.Output, "description accepted")
	before, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Contains(t, string(before), "description original")
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, actions, "confirmed", "bgp/group/actions-source", url.Values{"seconds": {"60"}}))
	requireStatus(t, http.StatusOK, graphicalActionRequest(t, actions, "accept", "bgp/group/actions-source", nil))
	after, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Contains(t, string(after), "description accepted")
	assert.Contains(t, mgr.ContentAtPath("testuser", path), "description accepted")
}
