// Design: docs/architecture/web-interface.md -- full current-system backup downloads.
package web

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/zefs"
)

const systemBackupPath = "/backups/system-download"

// systemBackupFixture installs a synthetic daemon-owned tree containing real
// committed history, a candidate, saved editing intent and nonconfiguration keys.
// Tests using the process-wide data RPC target MUST NOT run in parallel.
func systemBackupFixture(t *testing.T) storage.Storage {
	t.Helper()
	directory := t.TempDir()
	store, err := storage.Create(directory)
	require.NoError(t, err)
	configPath := filepath.Join(directory, "router.conf")
	_, _, err = storage.EnsureActiveVersion(store, configPath, []byte(validWebConfig), time.Now().Add(-2*time.Second))
	require.NoError(t, err)
	updated := strings.Replace(validWebConfig, "1.2.3.4", "192.0.2.42", 1)
	_, err = storage.RestoreConfig(store, "router.conf", []byte(updated))
	require.NoError(t, err)
	_, err = storage.WriteCandidateVersion(store, "router.conf", []byte(validWebConfig), time.Now().Add(time.Second))
	require.NoError(t, err)
	require.NoError(t, store.WriteFile(cli.DraftPath("router.conf"), []byte(validWebConfig), 0o600))
	require.NoError(t, store.WriteFile(cli.ChangePath("router.conf", "alice"), []byte("set bgp router-id 192.0.2.99\n"), 0o600))
	require.NoError(t, store.WriteKey(zefs.KeyLocalAdminPassword.Key(), []byte("system-backup-private-credential")))
	require.NoError(t, store.WriteKey("test/system-backup/binary", bytes.Repeat([]byte{0, 255, 128, 7}, 65536)))
	storagecli.InstallDataRPC(&storagecli.DataRPCTarget{Store: store, ConfigPath: configPath})
	t.Cleanup(func() {
		storagecli.InstallDataRPC(nil)
		require.NoError(t, store.Close())
	})
	return store
}

func systemBackupKeys(t *testing.T, store storage.Storage) map[string][]byte {
	t.Helper()
	keys, err := store.ListKeys("")
	require.NoError(t, err)
	values := make(map[string][]byte, len(keys))
	// The fixed, test-owned key inventory bounds this walk.
	for _, key := range keys {
		content, err := store.ReadKey(key)
		require.NoError(t, err, key)
		values[key] = bytes.Clone(content)
	}
	return values
}

// TestBackupSystemDownloadUsesRegisteredCommands requires the production route
// to return a complete binary snapshot without publishing a live change. Every
// raw key, including history, pending intent, credentials and binary data, must
// survive a normal ZeFS reopen; an artifact transfer must be aborted afterward.
func TestBackupSystemDownloadUsesRegisteredCommands(t *testing.T) {
	store := systemBackupFixture(t)
	before := systemBackupKeys(t, store)
	native := backupCommandDispatcher(t)
	var transfer string
	dispatch := func(ctx context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
		answer, err := native(ctx, caller, line)
		if line == "request config artifact system snapshot" {
			if answer != nil {
				if fields, ok := answer.Data.(plugin.Map); ok {
					var found bool
					transfer, found = fields["transfer"].(string)
					require.True(t, found)
				}
			}
		}
		return answer, err
	}
	route, ok := lookupWebRoute(RegisteredWebRoutes(), backupWorkspacePath)
	require.True(t, ok)
	handler, ok := route.Build(RouteDeps{Dispatch: dispatch, Authorizer: fakeAuthorizer{allowEdit: true}}).(*backupWorkspaces)
	require.True(t, ok)
	other := backupHTTPUpload(t, handler, backupCommandArtifact(t), "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, other.Code, other.Body.String())
	otherPath := other.Header().Get("Location") + "download"
	otherBefore := backupHTTPRequest(t, handler, otherPath, "alice", nil)
	require.Equal(t, http.StatusOK, otherBefore.Code)
	response := backupHTTPRequest(t, handler, systemBackupPath, "alice", url.Values{})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, "application/octet-stream", response.Header().Get("Content-Type"))
	require.Equal(t, `attachment; filename="system-backup.zefs"`, response.Header().Get("Content-Disposition"))
	require.Equal(t, strconv.Itoa(response.Body.Len()), response.Header().Get("Content-Length"))
	require.NotEmpty(t, transfer, "the HTTP path must use the registered private snapshot operation")
	path := filepath.Join(t.TempDir(), "system-backup.zefs")
	require.NoError(t, os.WriteFile(path, response.Body.Bytes(), 0o600))
	require.NoError(t, storage.CheckArtifact(path))
	require.Equal(t, before, backupSourceKeys(t, path), "a backup must preserve every source key")
	require.Equal(t, before, systemBackupKeys(t, store), "creating a backup must not mutate the live store")
	otherAfter := backupHTTPRequest(t, handler, otherPath, "alice", nil)
	require.Equal(t, http.StatusOK, otherAfter.Code)
	require.Equal(t, otherBefore.Body.Bytes(), otherAfter.Body.Bytes(), "creating a system snapshot must preserve existing uploaded workspaces")
	refused, err := native(t.Context(), plugin.CallerIdentity{Username: "alice"}, "show config artifact transfer read transfer "+transfer+" offset 0")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, refused.Status, "HTTP completion must remove its private transfer")
}

// TestBackupSystemDownloadRefusesUnauthorizedCreation checks the same origin,
// edit authorization and method boundaries before any snapshot command runs.
func TestBackupSystemDownloadRefusesUnauthorizedCreation(t *testing.T) {
	for _, test := range []struct {
		name, method, user, origin string
		edit                       bool
		status                     int
	}{
		{name: "anonymous", method: http.MethodPost, origin: "https://example.com", edit: true, status: http.StatusUnauthorized},
		{name: "read only", method: http.MethodPost, user: "alice", origin: "https://example.com", status: http.StatusForbidden},
		{name: "cross origin", method: http.MethodPost, user: "alice", origin: "https://untrusted.example", edit: true, status: http.StatusForbidden},
		{name: "get", method: http.MethodGet, user: "alice", origin: "https://example.com", edit: true, status: http.StatusMethodNotAllowed},
		{name: "put", method: http.MethodPut, user: "alice", origin: "https://example.com", edit: true, status: http.StatusMethodNotAllowed},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			handler := HandleBackupWorkspace(RouteDeps{Authorizer: fakeAuthorizer{allowEdit: test.edit}, Dispatch: func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
				calls++
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}})
			request := httptest.NewRequestWithContext(t.Context(), test.method, "https://example.com"+systemBackupPath, http.NoBody)
			request.Header.Set("Origin", test.origin)
			if test.user != "" {
				request = request.WithContext(withUsername(request.Context(), test.user))
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			require.Equal(t, test.status, response.Code, response.Body.String())
			if test.status == http.StatusMethodNotAllowed {
				require.Equal(t, http.MethodPost, response.Header().Get("Allow"))
			}
			require.Zero(t, calls, "a refused creation must not reach the dispatcher")
			require.Empty(t, response.Header().Get("Content-Disposition"))
		})
	}
}

// TestBackupSystemDownloadNeedsLiveStore refuses a fresh backup when the daemon
// serves no source store, rather than returning an empty or fabricated artifact.
func TestBackupSystemDownloadNeedsLiveStore(t *testing.T) {
	storagecli.InstallDataRPC(nil)
	handler, ok := HandleBackupWorkspace(RouteDeps{Dispatch: backupCommandDispatcher(t), Authorizer: fakeAuthorizer{allowEdit: true}}).(*backupWorkspaces)
	require.True(t, ok)
	response := backupHTTPRequest(t, handler, systemBackupPath, "alice", url.Values{})
	require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
	require.Empty(t, response.Header().Get("Content-Disposition"))
	require.Contains(t, response.Body.String(), "store")
}

type systemBackupFailedWriter struct{ *httptest.ResponseRecorder }

func (w systemBackupFailedWriter) Write([]byte) (int, error) {
	return 0, errors.New("injected system-backup response write failure")
}

// TestBackupSystemDownloadInterruptionAbortsSnapshot forces cancellation and a
// response-write failure through the new path. Its account-owned source must be
// gone even though the ordinary request context cannot perform cleanup anymore.
func TestBackupSystemDownloadInterruptionAbortsSnapshot(t *testing.T) {
	for _, kind := range []string{"canceled", "write failure"} {
		t.Run(kind, func(t *testing.T) {
			store := systemBackupFixture(t)
			before := systemBackupKeys(t, store)
			native := backupCommandDispatcher(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var transfer string
			aborts := 0
			dispatch := func(ctx context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
				if kind == "canceled" {
					if strings.HasPrefix(line, "show config artifact transfer read ") {
						cancel()
					}
				}
				if strings.HasPrefix(line, "request config artifact upload abort ") {
					aborts++
				}
				answer, err := native(ctx, caller, line)
				if line == "request config artifact system snapshot" {
					require.NotNil(t, answer)
					require.Equal(t, plugin.StatusDone, answer.Status, answer.Error)
					fields, ok := answer.Data.(plugin.Map)
					require.True(t, ok)
					transfer, ok = fields["transfer"].(string)
					require.True(t, ok)
				}
				return answer, err
			}
			handler := HandleBackupWorkspace(RouteDeps{Dispatch: dispatch, Authorizer: fakeAuthorizer{allowEdit: true}})
			request := httptest.NewRequestWithContext(ctx, http.MethodPost, "https://example.com"+systemBackupPath, http.NoBody)
			request.Header.Set("Origin", "https://example.com")
			request = request.WithContext(withUsername(request.Context(), "alice"))
			recorder := httptest.NewRecorder()
			var output http.ResponseWriter = recorder
			if kind == "write failure" {
				output = systemBackupFailedWriter{recorder}
			}
			handler.ServeHTTP(output, request)
			require.NotEmpty(t, transfer)
			require.Equal(t, 1, aborts)
			require.Empty(t, recorder.Body.Bytes(), "the injected failure must prevent a successful download body")
			require.NotEqual(t, "0", recorder.Header().Get("Content-Length"))
			refused, err := native(t.Context(), plugin.CallerIdentity{Username: "alice"}, "show config artifact transfer read transfer "+transfer+" offset 0")
			require.NoError(t, err)
			require.Equal(t, plugin.StatusError, refused.Status)
			require.Equal(t, before, systemBackupKeys(t, store))
		})
	}
}
