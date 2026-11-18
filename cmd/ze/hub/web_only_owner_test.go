//go:build ze_web

// Design: docs/architecture/web-interface.md -- standalone commands share their native owner.
package hub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/pkg/zefs"
)

func standaloneCommandFixture(t *testing.T) (*pluginserver.Server, zeweb.CommandDispatcher) {
	t.Helper()
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	return server, webOnlyDispatcher(server)
}

func standaloneCommandData(t *testing.T, dispatch zeweb.CommandDispatcher, user, line string) map[string]any {
	t.Helper()
	response, err := dispatch(t.Context(), plugin.CallerIdentity{Username: user}, line)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	content, err := json.Marshal(response.Data)
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, json.Unmarshal(content, &data))
	return data
}

// TestWebOnlyCommandsUseTheSharedArtifactOwner completes an upload and reads it
// through the same dispatcher that standalone web receives, with no running
// daemon. Another server and another account cannot borrow its identity.
func TestWebOnlyCommandsUseTheSharedArtifactOwner(t *testing.T) {
	server, dispatch := standaloneCommandFixture(t)
	path := filepath.Join(t.TempDir(), "standalone.zefs")
	store, err := storage.CreateBlobPopulated(path, func(seed storage.Storage) error {
		return seed.WriteFile("router.conf", []byte("environment {}\n"), 0o600)
	}, false, zefs.Spare(0))
	require.NoError(t, err)
	require.NoError(t, store.Close())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	begin := standaloneCommandData(t, dispatch, "alice", "request config artifact upload begin name standalone.zefs total "+strconv.Itoa(len(content)))
	transfer, ok := begin["transfer"].(string)
	require.True(t, ok)
	require.NotEmpty(t, transfer)
	appended := standaloneCommandData(t, dispatch, "alice", "request config artifact upload append transfer "+transfer+" offset 0 content "+base64.StdEncoding.EncodeToString(content))
	require.Equal(t, float64(len(content)), appended["next"])
	finished := standaloneCommandData(t, dispatch, "alice", "request config artifact upload finish transfer "+transfer)
	id, ok := finished["artifact"].(string)
	require.True(t, ok)
	require.NotEmpty(t, id)
	info := standaloneCommandData(t, dispatch, "alice", "show config artifact info artifact "+id)
	require.Equal(t, id, info["artifact"])
	require.Equal(t, []any{"router.conf"}, info["names"])
	require.Equal(t, float64(len(content)), info["bytes"])
	require.False(t, server.Running(), "constructing standalone commands must not start a daemon")
	_, other := standaloneCommandFixture(t)
	for _, refused := range []struct {
		dispatch zeweb.CommandDispatcher
		user     string
	}{{dispatch, "bob"}, {other, "alice"}} {
		response, err := refused.dispatch(t.Context(), plugin.CallerIdentity{Username: refused.user}, "show config artifact info artifact "+id)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusError, response.Status)
	}
	server.Stop()
	response, err := dispatch(t.Context(), plugin.CallerIdentity{Username: "alice"}, "show config artifact list")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, response.Status, "standalone stop must seal the shared owner")
}

// TestWebOnlyShutdownJoinsHistoryBeforeCommandOwner proves standalone cleanup
// keeps the native command owner alive until every admitted history write ends.
func TestWebOnlyShutdownJoinsHistoryBeforeCommandOwner(t *testing.T) {
	cert, key, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)
	underlying, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, underlying.Close()) })
	synctest.Test(t, func(t *testing.T) {
		commands, _ := standaloneCommandFixture(t)
		store := &webHistoryStore{Storage: underlying, entered: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
		manager := zeweb.NewEditorManager(store, "router.conf", nil, nil, nil)
		web, err := zeweb.NewWebServer(zeweb.WebConfig{ListenAddrs: []string{"127.0.0.1:0"}, CertPEM: cert, KeyPEM: key})
		require.NoError(t, err)
		service := newWebService(web, nil, manager)
		stopped := false
		_, err = commands.OnStop(func() error {
			stopped = true
			content, readErr := underlying.ReadFile(zefs.KeyHistory.Prefix() + "alice/operational")
			require.NoError(t, readErr)
			require.Equal(t, "show version", string(content))
			return nil
		})
		require.NoError(t, err)
		require.NoError(t, manager.RecordHistory("alice", cli.ModeOperational, nil, "show version", "standalone-owned"))
		<-store.entered
		ended := make(chan error, 1)
		go func() { ended <- shutdownWebOnly(context.Background(), service, commands) }()
		synctest.Wait()
		require.False(t, stopped)
		require.NoError(t, commands.CheckAdmission())
		close(store.release)
		require.NoError(t, <-ended)
		require.True(t, stopped, "standalone cleanup must stop its actual command owner after history drains")
		require.Error(t, commands.CheckAdmission())
	})
}

// TestWebOnlyShutdownRetainsCommandCleanupFailure keeps a native OnStop failure
// visible to the standalone exit verdict after the web service has stopped.
func TestWebOnlyShutdownRetainsCommandCleanupFailure(t *testing.T) {
	commands, _ := standaloneCommandFixture(t)
	_, err := commands.OnStop(func() error { return os.ErrPermission })
	require.NoError(t, err)
	err = shutdownWebOnly(t.Context(), nil, commands)
	require.ErrorIs(t, err, os.ErrPermission)
}
