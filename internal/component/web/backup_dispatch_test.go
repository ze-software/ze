// Design: docs/architecture/web-interface.md -- one artifact owner on every surface.

package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/zefs"
)

func backupCommandDispatcher(t *testing.T) CommandDispatcher {
	t.Helper()
	resolved, err := configyang.DefaultLoader()
	require.NoError(t, err)
	dispatcher := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(dispatcher, configyang.WireMethodToPath(resolved),
		configyang.PathToDescription(resolved), configyang.PathToHelp(resolved), configyang.PathToArgDefs(resolved), configyang.BuildCommandTree(resolved)))
	owner, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(owner.Stop)
	return func(ctx context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
		return dispatcher.Dispatch(&pluginserver.CommandContext{Server: owner, Username: caller.Username,
			Authorizer: caller.Authorizer, RequestContext: ctx, Surface: "web", Sender: plugin.OperatorSender()}, line)
	}
}

func backupJSON(t *testing.T, data plugin.ResponseData) string {
	t.Helper()
	encoded, err := json.Marshal(data)
	require.NoError(t, err)
	return string(encoded)
}

func backupCommand(t *testing.T, dispatch CommandDispatcher, user, line string) *plugin.Response {
	t.Helper()
	response, err := dispatch(t.Context(), plugin.CallerIdentity{Username: user}, line)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusDone, response.Status, "%s: %s", line, response.Error)
	return response
}

func backupCommandArtifact(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cross-surface.zefs")
	artifact, err := storage.CreateBlobPopulated(path, func(seed storage.Storage) error {
		if err := seed.WriteFile("router.conf", []byte(validWebConfig), 0o600); err != nil {
			return err
		}
		return seed.WriteKey("meta/opaque", bytes.Repeat([]byte{0, 1, 7, 255}, command.ContentOctetsMax))
	}, false, zefs.Spare(0))
	require.NoError(t, err)
	require.NoError(t, artifact.Close())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Greater(t, len(content), command.ContentOctetsMax)
	return content
}

// TestBackupHTTPAndDispatcherShareAnAccountBoundWorkspace uploads through the
// native HTTP route and reads/removes that identity through the real dispatcher.
// The other account must fail on both surfaces without learning its contents.
func TestBackupHTTPAndDispatcherShareAnAccountBoundWorkspace(t *testing.T) {
	dispatch := backupCommandDispatcher(t)
	handler := HandleBackupWorkspace(RouteDeps{Dispatch: dispatch, Authorizer: fakeAuthorizer{allowEdit: true}})
	if closer, ok := handler.(io.Closer); ok {
		t.Cleanup(func() { require.NoError(t, closer.Close()) })
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("backup", "device.zefs")
	require.NoError(t, err)
	_, err = file.Write(backupCommandArtifact(t))
	require.NoError(t, err)
	require.NoError(t, form.Close())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.com/backups/", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Origin", "https://example.com")
	request = request.WithContext(withUsername(request.Context(), "alice"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusSeeOther, response.Code, response.Body.String())
	id := strings.Trim(strings.TrimPrefix(response.Header().Get("Location"), backupWorkspacePath), "/")
	require.NotEmpty(t, id)
	listed := backupCommand(t, dispatch, "alice", "show config artifact list")
	require.Contains(t, fmt.Sprint(listed.Data), id)
	read := backupCommand(t, dispatch, "alice", "show config artifact source artifact "+id+" name router.conf")
	require.Contains(t, fmt.Sprint(read.Data), "router-id")
	refused, err := dispatch(t.Context(), plugin.CallerIdentity{Username: "bob"},
		"show config artifact source artifact "+id+" name router.conf")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, refused.Status)
	open := httptest.NewRequestWithContext(t.Context(), http.MethodGet, backupWorkspacePath+id+"/", http.NoBody)
	open = open.WithContext(withUsername(open.Context(), "bob"))
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, open)
	require.Equal(t, http.StatusNotFound, denied.Code)
	backupCommand(t, dispatch, "alice", "delete config artifact "+id)
	open = open.WithContext(withUsername(open.Context(), "alice"))
	removed := httptest.NewRecorder()
	handler.ServeHTTP(removed, open)
	require.Equal(t, http.StatusNotFound, removed.Code)
}

// TestBackupDispatcherUploadSpansContentPackets feeds an actual artifact larger
// than one packet through modeled begin/append/finish and opens it with HTTP.
// An incorrect offset must refuse without advancing the transfer.
func TestBackupDispatcherUploadSpansContentPackets(t *testing.T) {
	dispatch := backupCommandDispatcher(t)
	content := backupCommandArtifact(t)
	begin := backupCommand(t, dispatch, "alice", fmt.Sprintf("request config artifact upload begin name device.zefs total %d", len(content)))
	fields, ok := begin.Data.(plugin.Map)
	require.True(t, ok)
	id, ok := fields["transfer"].(string)
	require.True(t, ok)
	wrong, err := dispatch(t.Context(), plugin.CallerIdentity{Username: "alice"},
		"request config artifact upload append transfer "+id+" offset 1 content AA==")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, wrong.Status)
	for offset := 0; offset < len(content); offset += command.ContentOctetsMax {
		chunk := content[offset:min(offset+command.ContentOctetsMax, len(content))]
		backupCommand(t, dispatch, "alice", fmt.Sprintf("request config artifact upload append transfer %s offset %d content %s",
			id, offset, base64.StdEncoding.EncodeToString(chunk)))
	}
	finished := backupCommand(t, dispatch, "alice", "request config artifact upload finish transfer "+id)
	fields, ok = finished.Data.(plugin.Map)
	require.True(t, ok)
	workspace, ok := fields["artifact"].(string)
	require.True(t, ok)
	handler := HandleBackupWorkspace(RouteDeps{Dispatch: dispatch, Authorizer: fakeAuthorizer{allowEdit: true}})
	if closer, ok := handler.(io.Closer); ok {
		t.Cleanup(func() { require.NoError(t, closer.Close()) })
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, backupWorkspacePath+workspace+"/download", http.NoBody)
	request = request.WithContext(withUsername(request.Context(), "alice"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	path := filepath.Join(t.TempDir(), "download.zefs")
	require.NoError(t, os.WriteFile(path, response.Body.Bytes(), 0o600))
	artifact, err := storage.OpenBlob(path, false)
	require.NoError(t, err)
	defer func() { require.NoError(t, artifact.Close()) }()
	opaque, err := artifact.ReadKey("meta/opaque")
	require.NoError(t, err)
	require.Equal(t, bytes.Repeat([]byte{0, 1, 7, 255}, command.ContentOctetsMax), opaque)
}

// TestBackupDispatcherRefusesCanceledAdmission proves the trusted request
// context fences transfer creation, while a later ordinary request can proceed.
func TestBackupDispatcherRefusesCanceledAdmission(t *testing.T) {
	dispatch := backupCommandDispatcher(t)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	response, err := dispatch(canceled, plugin.CallerIdentity{Username: "alice"},
		"request config artifact upload begin name device.zefs total 1")
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusError, response.Status)
	list := backupCommand(t, dispatch, "alice", "show config artifact list")
	require.NotContains(t, fmt.Sprint(list.Data), "device.zefs")
	begin := backupCommand(t, dispatch, "alice", "request config artifact upload begin name device.zefs total 1")
	fields, ok := begin.Data.(plugin.Map)
	require.True(t, ok)
	transfer, ok := fields["transfer"].(string)
	require.True(t, ok)
	backupCommand(t, dispatch, "alice", "request config artifact upload abort transfer "+transfer)
}

// TestBackupDispatcherCannotAcceptAnAccountFromInput proves only transport
// authentication selects ownership; an account keyword cannot impersonate Alice.
func TestBackupDispatcherCannotAcceptAnAccountFromInput(t *testing.T) {
	dispatch := backupCommandDispatcher(t)
	response, err := dispatch(t.Context(), plugin.CallerIdentity{Username: "bob"},
		"request config artifact upload begin name private.zefs total 1 account alice")
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusError, response.Status)
	for _, user := range []string{"alice", "bob"} {
		list := backupCommand(t, dispatch, user, "show config artifact list")
		require.NotContains(t, fmt.Sprint(list.Data), "private.zefs")
	}
}

// TestBackupHTTPTextSpansContentPackets retains the native 4 MiB form allowance:
// a review larger than one command packet crosses the shared text transfers.
func TestBackupHTTPTextSpansContentPackets(t *testing.T) {
	owner, content := backupHTTPFixture(t)
	upload := backupHTTPUpload(t, owner, content, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, upload.Code, upload.Body.String())
	path := upload.Header().Get("Location")
	opened := backupHTTPRequest(t, owner, path+"?config=router.conf", "alice", nil)
	require.Equal(t, http.StatusOK, opened.Code)
	text := validWebConfig + strings.Repeat("\n# retained comment", 20000) + "\n"
	require.Greater(t, len(text), command.ContentOctetsMax)
	form := url.Values{"config": {"router.conf"}, "base": {backupHTTPField(t, opened.Body.String(), "base")},
		"content": {text}, "action": {"review"}, "review": {""}}
	reviewed := backupHTTPRequest(t, owner, path, "alice", form)
	require.Equal(t, http.StatusOK, reviewed.Code, reviewed.Body.String())
	require.Equal(t, text, backupSourceTextarea(t, reviewed.Body.String()))
	require.NotEmpty(t, backupHTTPField(t, reviewed.Body.String(), "review"))
}
