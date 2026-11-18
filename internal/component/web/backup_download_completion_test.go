// Design: docs/architecture/web-interface.md -- HTTP completion follows the immutable native transfer size.
package web

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestBackupHTTPDownloadDetectsIncompleteTransfer exercises real HTTP framing
// over the registered route and native dispatcher. A native refusal after one
// chunk MUST reach the client as an incomplete download, while complete reads
// and a retry still deliver all retained bytes to their owning account.
func TestBackupHTTPDownloadDetectsIncompleteTransfer(t *testing.T) {
	for _, kind := range []string{"source", "artifact"} {
		t.Run(kind, func(t *testing.T) {
			native := backupCommandDispatcher(t)
			var failSecondChunk, reachedRefusal atomic.Bool
			dispatch := func(ctx context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
				if failSecondChunk.Load() {
					if strings.HasPrefix(line, "show config artifact transfer read ") {
						if strings.HasSuffix(line, fmt.Sprintf(" offset %d", command.ContentOctetsMax)) {
							canceled, cancel := context.WithCancel(ctx)
							cancel()
							response, err := native(canceled, caller, line)
							if response != nil {
								if response.Status == plugin.StatusError {
									reachedRefusal.Store(true)
								}
							}
							return response, err
						}
					}
				}
				return native(ctx, caller, line)
			}
			route, ok := lookupWebRoute(RegisteredWebRoutes(), backupWorkspacePath)
			require.True(t, ok)
			handler := route.Build(RouteDeps{Dispatch: dispatch, Authorizer: fakeAuthorizer{allowEdit: true}})
			owner, ok := handler.(*backupWorkspaces)
			require.True(t, ok)
			t.Cleanup(func() { require.NoError(t, owner.Close()) })
			original := backupCommandArtifact(t)
			before := bytes.Clone(original)
			var upload *httptest.ResponseRecorder
			if kind == "source" {
				upload = backupSourceUpload(t, owner, original, "alice", "https://example.com")
			} else {
				upload = backupHTTPUpload(t, owner, original, "alice", "https://example.com")
			}
			require.Equal(t, http.StatusSeeOther, upload.Code, upload.Body.String())
			path := upload.Header().Get("Location") + "download"
			require.Equal(t, http.StatusNotFound, backupHTTPRequest(t, owner, path, "bob", nil).Code)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				owner.ServeHTTP(w, r.WithContext(withUsername(r.Context(), "alice")))
			}))
			t.Cleanup(server.Close)
			client := server.Client()
			complete := backupHTTPDownload(t, client, server.URL+path)
			require.Greater(t, len(complete), command.ContentOctetsMax, "the control must need a second native read")
			if kind == "source" {
				require.Equal(t, before, complete)
			} else {
				file := filepath.Join(t.TempDir(), "complete.zefs")
				require.NoError(t, os.WriteFile(file, complete, 0o600))
				require.NoError(t, storage.CheckArtifact(file))
				store, err := storage.OpenBlob(file, false)
				require.NoError(t, err)
				opaque, readErr := store.ReadKey("meta/opaque")
				require.NoError(t, readErr)
				require.NoError(t, store.Close())
				require.Equal(t, bytes.Repeat([]byte{0, 1, 7, 255}, command.ContentOctetsMax), opaque)
			}
			failSecondChunk.Store(true)
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+path, http.NoBody)
			require.NoError(t, err)
			response, err := client.Do(request)
			require.NoError(t, err)
			partial, readErr := io.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.True(t, reachedRefusal.Load(), "the failing read must reach the native owner refusal")
			require.Equal(t, complete[:command.ContentOctetsMax], partial, "one native chunk must reach the real socket before refusal")
			require.ErrorIs(t, readErr, io.ErrUnexpectedEOF, "a partial artifact must never complete successfully at the HTTP client")
			require.Equal(t, int64(len(complete)), response.ContentLength)
			failSecondChunk.Store(false)
			require.Equal(t, complete, backupHTTPDownload(t, client, server.URL+path), "a failed download cannot consume or change the original retained source")
			require.Equal(t, before, original)
			require.Equal(t, http.StatusNotFound, backupHTTPRequest(t, owner, path, "bob", nil).Code)
		})
	}
}

// backupHTTPDownload requires a successful real HTTP body, rather than a
// recorder that cannot detect an incomplete transfer on a network connection.
func backupHTTPDownload(t *testing.T, client *http.Client, address string) []byte {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody)
	require.NoError(t, err)
	response, err := client.Do(request)
	require.NoError(t, err)
	content, readErr := io.ReadAll(response.Body)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, readErr)
	return content
}
