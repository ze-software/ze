// Design: docs/architecture/web-interface.md -- native admission before rendering the backup upload page.
package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestBackupInitialPageUsesNativeListAdmission proves the registered route
// renders over both empty and populated real native list answers, while an
// actual native admission refusal still prevents either upload control.
func TestBackupInitialPageUsesNativeListAdmission(t *testing.T) {
	dispatch := backupCommandDispatcher(t)
	route, ok := lookupWebRoute(RegisteredWebRoutes(), backupWorkspacePath)
	require.True(t, ok)
	require.Equal(t, WrapAuth, route.Wrap)
	handler := route.Build(RouteDeps{Dispatch: dispatch, Authorizer: fakeAuthorizer{allowEdit: true}})
	owner, ok := handler.(*backupWorkspaces)
	require.True(t, ok)
	closer, ok := handler.(io.Closer)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, closer.Close()) })
	for _, population := range []string{"empty", "populated"} {
		if population == "populated" {
			_, content := backupHTTPFixture(t)
			upload := backupHTTPUpload(t, owner, content, "alice", "https://example.com")
			require.Equal(t, http.StatusSeeOther, upload.Code, upload.Body.String())
		}
		list := backupCommand(t, dispatch, "alice", "show config artifact list")
		if population == "empty" {
			require.Equal(t, "[]", backupJSON(t, list.Data))
		} else {
			require.Contains(t, backupJSON(t, list.Data), "original.zefs")
		}
		response := backupHTTPRequest(t, owner, backupWorkspacePath, "alice", nil)
		require.Equal(t, http.StatusOK, response.Code, "%s: %s", population, response.Body.String())
		require.Contains(t, response.Body.String(), `id="backup-upload-file"`)
		require.Contains(t, response.Body.String(), `id="backup-source-file"`)
		require.NotContains(t, response.Body.String(), "credential-to-preserve")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	refused, err := dispatch(ctx, plugin.CallerIdentity{Username: "alice"}, "show config artifact list")
	require.NoError(t, err)
	require.NotNil(t, refused)
	require.Equal(t, plugin.StatusError, refused.Status, "the negative control must reach native owner admission")
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/backups/", http.NoBody)
	request = request.WithContext(withUsername(request.Context(), "alice"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.NotContains(t, response.Body.String(), `id="backup-upload-file"`)
	require.NotContains(t, response.Body.String(), `id="backup-source-file"`)
}
