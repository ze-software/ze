// Design: docs/architecture/web-interface.md -- explicit discard owns durable intent.
package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestWebDiscardWithoutCachedEditorClearsOnlyItsUserIntent includes an old
// session identity, so opening a new cache entry alone cannot hide missing
// audit detail behind a coincidentally equal second-resolution session id.
func TestWebDiscardWithoutCachedEditorClearsOnlyItsUserIntent(t *testing.T) {
	for _, surface := range []string{"button", "terminal"} {
		t.Run(surface, func(t *testing.T) {
			mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
			us, err := mgr.GetOrCreate("testuser")
			require.NoError(t, err)
			oldTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
			adapter, ok := us.editor.(*testEditorAdapter)
			require.True(t, ok)
			adapter.ed.SetSession(&cli.EditSession{User: "testuser", Origin: "web", ID: "testuser@web%2000-01-01T00:00:00Z", StartTime: oldTime})
			require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", "mine"}, "description", "discard-old"))
			require.NoError(t, mgr.CreateEntry("testuser", []string{"bgp", "group", "old-empty"}))
			require.NoError(t, mgr.SetValue("testuser-other", []string{"bgp", "group", "theirs"}, "description", "keep-other"))
			require.NoError(t, mgr.CreateEntry("testuser-other", []string{"bgp", "group", "foreign-empty"}))
			ownPath := cli.ChangePath(mgr.configPath, "testuser")
			otherPath := cli.ChangePath(mgr.configPath, "testuser-other")
			require.True(t, mgr.store.Exists(ownPath))
			otherBefore, err := mgr.store.ReadFile(otherPath)
			require.NoError(t, err)
			committed, err := mgr.committedConfig()
			require.NoError(t, err)
			// Model process restart or idle eviction: durable user files survive.
			mgr.mu.Lock()
			delete(mgr.sessions, "testuser")
			mgr.mu.Unlock()
			recorder, err := audit.NewMemory(100)
			require.NoError(t, err)
			response := httptest.NewRecorder()
			if surface == "button" {
				HandleConfigDiscardWithAuthorizerAndAudit(mgr, fakeAuthorizer{allowEdit: true}, recorder).ServeHTTP(response, authedRequest(t, http.MethodPost, "/config/discard/", url.Values{}))
				require.Equal(t, http.StatusSeeOther, response.Code, response.Body.String())
			} else {
				HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, nil, nil, fakeAuthorizer{allowEdit: true}, recorder).ServeHTTP(response, postConfigRequest(t, "/cli/terminal", url.Values{"command": {"discard all"}, "mode": {terminalModeConfig}}, "testuser"))
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			}
			assert.False(t, mgr.store.Exists(ownPath), "explicit discard must remove old persisted intent")
			otherAfter, err := mgr.store.ReadFile(otherPath)
			require.NoError(t, err)
			assert.Equal(t, otherBefore, otherAfter, "another user's intent must stay byte-identical")
			after, err := mgr.committedConfig()
			require.NoError(t, err)
			assert.Equal(t, committed, after, "discard applies no configuration")
			rows := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
			require.Len(t, rows, 1)
			assert.Equal(t, "testuser", rows[0].Actor)
			assert.Contains(t, rows[0].Detail, "discard-old")
			assert.Contains(t, rows[0].Detail, "old-empty")
			assert.NotContains(t, rows[0].Detail, "keep-other")
			assert.NotContains(t, rows[0].Detail, "foreign-empty")
		})
	}
}
