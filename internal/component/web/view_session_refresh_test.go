// Design: docs/architecture/web-interface.md -- request views follow committed configuration.
package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
)

// TestWebNewUserViewsReadCurrentCommit exercises both shells with a startup
// tree older than a commit and a user who has no cached editor.
func TestWebNewUserViewsReadCurrentCommit(t *testing.T) {
	for _, mode := range []string{"finder", "workbench"} {
		for _, readOnly := range []bool{false, true} {
			name := mode + "/editable"
			if readOnly {
				name = mode + "/readonly"
			}
			t.Run(name, func(t *testing.T) {
				mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
				startup, err := config.NewParser(schema).Parse(windowCommittedConfig)
				require.NoError(t, err)
				require.NoError(t, mgr.SetValue("writer", []string{"bgp", "group", "source"}, "description", "accepted"))
				_, err = mgr.CommitNow("writer")
				require.NoError(t, err)
				require.Nil(t, mgr.Tree("reader"))
				renderer, err := NewRenderer()
				require.NoError(t, err)
				auth := fakeAuthorizer{allowEdit: !readOnly}
				view := HandleFragment(renderer, schema, startup, mgr, true, auth)
				if mode == "workbench" {
					view = HandleWorkbench(renderer, schema, startup, mgr, true, WithAuthorizer(auth))
				}
				request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/bgp/group/source/description/", nil)
				request = request.WithContext(withUsername(request.Context(), "reader"))
				response := httptest.NewRecorder()
				view.ServeHTTP(response, request)
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				assert.Contains(t, response.Body.String(), "accepted")
				if readOnly {
					assert.NotContains(t, response.Body.String(), `hx-post="/config/set/`)
					assert.NotContains(t, response.Body.String(), `id="graphical-copy"`)
				}
				assert.Zero(t, mgr.ChangeCount("reader"))
			})
		}
	}
}

// TestWebDiscardHistoryRestorationReopensCurrentView proves the restore is a
// candidate, discard deletes its durable intent, and the next GET reads current
// committed bytes rather than the startup tree captured by either shell.
func TestWebDiscardHistoryRestorationReopensCurrentView(t *testing.T) {
	for _, mode := range []string{"finder", "workbench"} {
		t.Run(mode, func(t *testing.T) {
			mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
			startup, err := config.NewParser(schema).Parse(windowCommittedConfig)
			require.NoError(t, err)
			path := []string{"bgp", "group", "source"}
			require.NoError(t, mgr.SetValue("testuser", path, "description", "original"))
			_, err = mgr.CommitNow("testuser")
			require.NoError(t, err)
			backups, err := mgr.ListBackups("testuser")
			require.NoError(t, err)
			require.NotEmpty(t, backups)
			selected := backups[0].Path
			require.NoError(t, mgr.SetValue("testuser", path, "description", "accepted"))
			_, err = mgr.CommitNow("testuser")
			require.NoError(t, err)
			committed, err := mgr.committedConfig()
			require.NoError(t, err)
			require.Contains(t, string(committed), "description accepted")
			renderer, err := NewRenderer()
			require.NoError(t, err)
			actions := HandleGraphicalAction(mgr, schema, renderer, nil, nil, nil)
			restored := graphicalActionRequest(t, actions, "rollback", "", url.Values{"revision": {selected}})
			require.Equal(t, http.StatusOK, restored.Code, restored.Body.String())
			require.Contains(t, mgr.ContentAtPath("testuser", path), "description original")
			unchanged, err := mgr.committedConfig()
			require.NoError(t, err)
			require.Equal(t, committed, unchanged)
			request := authedRequest(t, http.MethodPost, "/config/discard/", url.Values{})
			response := httptest.NewRecorder()
			HandleConfigDiscardWithAuthorizerAndAudit(mgr, nil, nil).ServeHTTP(response, request)
			require.Equal(t, http.StatusSeeOther, response.Code, response.Body.String())
			assert.False(t, mgr.store.Exists(cli.ChangePath(mgr.configPath, "testuser")), "discard must remove persisted restoration intent")
			assert.Nil(t, mgr.Tree("testuser"), "discard still ends the cached editor")
			unchanged, err = mgr.committedConfig()
			require.NoError(t, err)
			assert.Equal(t, committed, unchanged, "discard must not publish restored content")
			view := HandleFragment(renderer, schema, startup, mgr, true, nil)
			if mode == "workbench" {
				view = HandleWorkbench(renderer, schema, startup, mgr, true)
			}
			response = httptest.NewRecorder()
			view.ServeHTTP(response, authedRequest(t, http.MethodGet, "/show/bgp/group/source/", nil))
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			assert.Contains(t, response.Body.String(), `value="accepted"`)
			require.NotNil(t, mgr.Tree("testuser"))
			assert.Contains(t, mgr.ContentAtPath("testuser", path), "description accepted")
			assert.Zero(t, mgr.ChangeCount("testuser"))
		})
	}
}

// TestWebViewStoreFailureCannotRenderStartupSnapshot refuses a fresh view whose
// actual owning store is closed, rather than showing plausible stale values.
func TestWebViewStoreFailureCannotRenderStartupSnapshot(t *testing.T) {
	for _, mode := range []string{"finder", "workbench"} {
		t.Run(mode, func(t *testing.T) {
			mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
			startup, err := config.NewParser(schema).Parse(windowCommittedConfig)
			require.NoError(t, err)
			renderer, err := NewRenderer()
			require.NoError(t, err)
			view := HandleFragment(renderer, schema, startup, mgr, true, nil)
			if mode == "workbench" {
				view = HandleWorkbench(renderer, schema, startup, mgr, true)
			}
			require.NoError(t, mgr.store.Close())
			response := httptest.NewRecorder()
			view.ServeHTTP(response, authedRequest(t, http.MethodGet, "/show/bgp/", nil))
			assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			assert.NotContains(t, response.Body.String(), `value="10.0.0.250"`)
			assert.Nil(t, mgr.Tree("testuser"))
		})
	}
}
