// Design: docs/architecture/web-interface.md -- registered backup workspace wiring.
package web

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestBackupWorkspaceRegisteredWorkflow uploads a real binary artifact through
// the route registry that production serves, then opens its configuration list.
// It prevents a helper-only implementation with no reachable browser workflow.
func TestBackupWorkspaceRegisteredWorkflow(t *testing.T) {
	artifactPath := filepath.Join(t.TempDir(), "device.zefs")
	artifact, err := storage.CreateBlobPopulated(artifactPath, func(seed storage.Storage) error {
		return seed.WriteFile("router.conf", []byte(validWebConfig), 0o600)
	}, false, zefs.Spare(0))
	require.NoError(t, err)
	require.NoError(t, artifact.Close())
	content, err := os.ReadFile(artifactPath)
	require.NoError(t, err)

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("backup", "device.zefs")
	require.NoError(t, err)
	_, err = io.Copy(file, bytes.NewReader(content))
	require.NoError(t, err)
	require.NoError(t, form.Close())

	route, ok := lookupWebRoute(RegisteredWebRoutes(), backupWorkspacePath)
	require.True(t, ok, "backup editing must be registered on the production web surface")
	require.Equal(t, WrapAuth, route.Wrap)
	mux := http.NewServeMux()
	handler := route.Build(RouteDeps{Authorizer: fakeAuthorizer{allowEdit: true}})
	closer, ok := handler.(io.Closer)
	require.True(t, ok, "the artifact owner must expose cleanup before route wrapping")
	t.Cleanup(func() { require.NoError(t, closer.Close()) })
	mux.Handle(route.Pattern, handler)
	upload := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.com/backups/", &body)
	upload.Header.Set("Content-Type", form.FormDataContentType())
	upload.Header.Set("Origin", "https://example.com")
	upload = upload.WithContext(withUsername(upload.Context(), "alice"))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, upload)
	require.Equal(t, http.StatusSeeOther, response.Code, "valid upload must open an independent artifact workspace: %s", response.Body.String())
	location := response.Header().Get("Location")
	require.NotEqual(t, backupWorkspacePath, location, "the workspace must have an explicit identity")
	require.Contains(t, location, backupWorkspacePath)

	open := httptest.NewRequestWithContext(t.Context(), http.MethodGet, location, http.NoBody)
	open = open.WithContext(withUsername(open.Context(), "alice"))
	selected := httptest.NewRecorder()
	mux.ServeHTTP(selected, open)
	require.Equal(t, http.StatusOK, selected.Code)
	require.Contains(t, selected.Body.String(), "router.conf", "the uploaded artifact's configuration must be selectable")
}
