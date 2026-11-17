// Design: docs/architecture/web-interface.md -- failed recovery refresh fences admitted requests.
package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestConfigRecoveryFailedRefreshFencesAdmittedEditor keeps a reference that a
// request acquired before recovery. A failed reload must leave that reference
// unable to expose or resurrect the removed unsaved candidate.
func TestConfigRecoveryFailedRefreshFencesAdmittedEditor(t *testing.T) {
	mgr := newTestEditorManager(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.99"))
	admitted, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	admitted.editor = failedRecoveryResumeEditor{Editor: admitted.editor}
	path := cli.ChangePath(mgr.configPath, "testuser")
	require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
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
	require.False(t, mgr.store.Exists(path), "recovery removed the corrupt file before the refresh failed")
	require.Len(t, recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard}), 1)
	require.Nil(t, admitted.editor.Tree(), "an admitted reader cannot expose the old unsaved tree")
	require.NotContains(t, admitted.editor.WorkingContent(), "192.0.2.99")
	require.Error(t, admitted.editor.SetValue([]string{"bgp"}, "router-id", "192.0.2.100"))
	_, err = admitted.editor.CommitSession()
	require.Error(t, err)
	_, err = admitted.editor.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: "show"})
	require.Error(t, err)
	require.False(t, mgr.store.Exists(path), "an admitted mutation cannot resurrect discarded intent")
	_, cached := mgr.sessions["testuser"]
	require.False(t, cached, "a later request must retry creation instead of reusing the failed editor")
	reopened, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	require.NotSame(t, admitted, reopened)
	require.NotContains(t, reopened.editor.WorkingContent(), "192.0.2.99")
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.101"))
}

type failedRecoveryResumeEditor struct {
	contract.Editor
}

func (failedRecoveryResumeEditor) ResumeSession() error {
	return errors.New("injected recovery refresh failure")
}
