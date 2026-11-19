// Design: docs/architecture/web-interface.md -- per-user Workbench pending rows
// Related: handler_workbench.go -- authenticated candidate and row enrichment

//go:build ze_bgp

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestWorkbenchHTTPPendingMarkerBelongsToTheEditingAccount posts an edit
// through the authenticated terminal handler, then parses the real Workbench
// HTTP response. Only that account's changed row may carry the accessible
// marker; an unchanged sibling and a second account keep their clean rows.
// The policy list takes the generic YANG route, not an unfinished BGP page.
func TestWorkbenchHTTPPendingMarkerBelongsToTheEditingAccount(t *testing.T) {
	const content = `policy {
    prefix-list alpha {
        entry 192.0.2.0/24 { action accept; }
    }
    prefix-list beta {
        entry 198.51.100.0/24 { action accept; }
    }
}`
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(content)
	require.NoError(t, err)
	configPath := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))
	store := testConfigStore(t, configPath)
	manager := NewEditorManager(store, configPath, schema, testEditorFactory(), testEditSessionFactory())
	t.Cleanup(func() { require.NoError(t, manager.Close()) })
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleWorkbench(renderer, schema, tree, manager, false)

	renderMarkers := func(username string) map[string]bool {
		t.Helper()
		request := authedRequest(t, http.MethodGet, "/show/policy/prefix-list/", nil)
		// Authentication middleware binds this identity before the handler.
		request = request.WithContext(withUsername(request.Context(), username))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		document := parseFinderHTML(t, response.Body.String())
		workspace := finderElement(t, document, "id", "workbench-workspace")
		require.ElementsMatch(t, []string{"alpha", "beta"}, finderAttributeValues(workspace, "data-row-key"))
		marked := make(map[string]bool, 2)
		for _, key := range []string{"alpha", "beta"} {
			row := finderElement(t, workspace, "data-row-key", key)
			require.Equal(t, "tr", row.Data)
			markers := finderElements(row, "aria-label", "Pending changes")
			require.LessOrEqual(t, len(markers), 1, "row %s repeats its pending marker", key)
			marked[key] = len(markers) == 1
			if marked[key] {
				assert.Equal(t, "Uncommitted changes", finderAttr(markers[0], "title"))
				assert.Equal(t, "●", finderText(markers[0]))
			}
		}
		return marked
	}

	clean := map[string]bool{"alpha": false, "beta": false}
	assert.Equal(t, clean, renderMarkers("testuser"), "a fresh editor must render both clean rows")
	terminal := HandleCLITerminalWithDispatchAuthorizerAndAudit(manager, schema, tree, nil, nil, nil)
	answer := runTerminalCommandForm(t, terminal, url.Values{
		"command": {"set policy prefix-list alpha entry 192.0.2.0/24 action reject"},
		"mode":    {terminalModeConfig},
	})
	require.NotContains(t, answer.Output, "error:")
	require.Equal(t, 1, manager.ChangeCount("testuser"))
	entry := walkTree(manager.Tree("testuser"), schema,
		[]string{"policy", "prefix-list", "alpha", "entry", "192.0.2.0/24"})
	require.NotNil(t, entry)
	action, found := entry.Get("action")
	require.True(t, found)
	require.Equal(t, "reject", action)

	pending := map[string]bool{"alpha": true, "beta": false}
	assert.Equal(t, pending, renderMarkers("testuser"), "only the edited list entry may be marked")
	assert.Equal(t, clean, renderMarkers("other-user"), "one account must not inherit another account's marker")
	assert.Zero(t, manager.ChangeCount("other-user"))
	assert.Equal(t, pending, renderMarkers("testuser"), "reading another account must preserve the original pending marker")
}
