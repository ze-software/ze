// Design: docs/architecture/web-components.md -- graphical candidate import.

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGraphicalImportControlsReachFinder requires native forms in the real
// primary editor, at both the root and a transport-encoded selected entry.
func TestGraphicalImportControlsReachFinder(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "selected/key"}, "description", "selected"))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleFragment(renderer, schema, tree, mgr, false, fakeAuthorizer{allowEdit: true})
	for _, path := range []string{"", "bgp/group/selected%2Fkey"} {
		where := "/show/"
		if path != "" {
			where += path + "/"
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, authedRequest(t, http.MethodGet, where, nil))
		requireStatus(t, http.StatusOK, response)
		body := response.Body.String()
		for _, id := range []string{"graphical-import-file", "graphical-import-file-submit", "graphical-import-paste", "graphical-import-paste-submit"} {
			assert.Contains(t, body, `id="`+id+`"`)
		}
		assert.Contains(t, body, `hx-post="/config/import"`)
		assert.Contains(t, body, `hx-encoding="multipart/form-data"`)
		assert.Contains(t, body, `name="path" value="`+path+`"`)
		assert.Contains(t, body, `name="location"`)
		assert.Contains(t, body, `value="absolute"`)
		assert.Contains(t, body, `value="relative"`)
		assert.Contains(t, body, `name="action"`)
		assert.Contains(t, body, `value="merge"`)
		assert.Contains(t, body, `value="replace"`)
	}
}

// TestGraphicalImportReadOnlyHidesControls leaves inspection available while
// excluding both ways to submit candidate-changing content.
func TestGraphicalImportReadOnlyHidesControls(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleFragment(renderer, schema, tree, mgr, false, fakeAuthorizer{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authedRequest(t, http.MethodGet, "/show/bgp/", nil))
	requireStatus(t, http.StatusOK, response)
	assert.Contains(t, response.Body.String(), `id="graphical-inspect"`)
	assert.NotContains(t, response.Body.String(), `hx-post="/config/import"`)
	assert.NotContains(t, response.Body.String(), `id="graphical-import-file"`)
	assert.NotContains(t, response.Body.String(), `id="graphical-import-paste"`)
}
