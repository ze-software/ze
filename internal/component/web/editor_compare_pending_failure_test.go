// Design: docs/architecture/web-interface.md -- review failure refuses publication.
package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestPendingReviewSnapshotFailureRefusesHTTP uses the existing real-store
// lock fault through GET review and POST commit. Failed review must reach the
// error response, leave intent and committed bytes intact, and emit no success
// audit. Removing the fault must restore a useful review over the same intent.
func TestPendingReviewSnapshotFailureRefusesHTTP(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
			store := &graphicalCommitCleanupStore{Storage: mgr.store}
			mgr.store = store
			mgr.SetCommitHook(func() error {
				store.promotions++
				return storage.PromoteCandidate(store, mgr.configPath)
			})
			require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.10"))
			committedBefore, err := mgr.committedConfig()
			require.NoError(t, err)
			pendingBefore, err := store.ReadFile(cli.ChangePath(mgr.configPath, "testuser"))
			require.NoError(t, err)
			renderer, err := NewRenderer()
			require.NoError(t, err)
			recorder, err := audit.NewMemory(100)
			require.NoError(t, err)
			authorizer := adminWebAuthorizer()
			authorizer.Store.AssignProfiles("testuser", []string{"admin"})
			handler := HandleConfigCommitWithAuthorizerAndAudit(mgr, renderer, nil, authorizer, recorder)
			store.failNextLock = true
			request := authedRequest(t, method, "/config/commit/", url.Values{})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			assert.Contains(t, response.Body.String(), "could not be compared")
			assert.NotContains(t, response.Body.String(), "+ bgp router-id 192.0.2.10")
			assert.Equal(t, 1, store.failures, "the review must observe the real storage refusal")
			assert.Zero(t, store.promotions)
			assert.Empty(t, recorder.Query(audit.Filter{Action: audit.ActionConfigCommit}))
			store.failNextLock = false
			committedAfter, err := mgr.committedConfig()
			require.NoError(t, err)
			assert.Equal(t, committedBefore, committedAfter)
			pendingAfter, err := store.ReadFile(cli.ChangePath(mgr.configPath, "testuser"))
			require.NoError(t, err)
			assert.Equal(t, pendingBefore, pendingAfter)

			recovered := httptest.NewRecorder()
			handler.ServeHTTP(recovered, authedRequest(t, http.MethodGet, "/config/commit/", nil))
			assert.Equal(t, http.StatusOK, recovered.Code)
			assert.Contains(t, recovered.Body.String(), "+ bgp router-id 192.0.2.10")
		})
	}
}
