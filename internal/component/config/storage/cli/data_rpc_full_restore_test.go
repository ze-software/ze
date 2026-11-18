// Design: docs/architecture/hub-architecture.md -- full restore after daemon teardown.
package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func fullRestoreDispatcher(t *testing.T) *pluginserver.Dispatcher {
	t.Helper()
	resolved, err := configyang.DefaultLoader()
	require.NoError(t, err)
	dispatcher := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(dispatcher, configyang.WireMethodToPath(resolved),
		configyang.PathToDescription(resolved), configyang.PathToHelp(resolved), configyang.PathToArgDefs(resolved), configyang.BuildCommandTree(resolved)))
	return dispatcher
}

func fullRestoreOwner(t *testing.T) (*storage.DaemonRecovery, string) {
	t.Helper()
	store, path := dataRPCStore(t, nil)
	bound := storage.BindConfigSource(store, path, storage.ConfigSourceStored)
	owner, err := storage.NewDaemonRecovery(bound)
	require.NoError(t, err)
	InstallDataRPC(&DataRPCTarget{Store: owner.Borrow(), ConfigPath: path})
	t.Cleanup(func() {
		if owner.Borrow() != nil {
			closed, err := owner.Close()
			require.NoError(t, err)
			require.NoError(t, closed.Discard())
		}
	})
	return owner, filepath.Dir(path)
}

// TestFullRestoreRPCRequiresTheOuterStoreOwner proves the modeled full command
// reaches its owner-admission refusal, never attempts a replacement underneath
// the live writer, and keeps its original content.
func TestFullRestoreRPCRequiresTheOuterStoreOwner(t *testing.T) {
	store, _ := dataRPCStore(t, nil)
	path := dataArtifact(t, "router.conf", "replacement")
	resolved, err := configyang.DefaultLoader()
	require.NoError(t, err)
	dispatcher := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(dispatcher, configyang.WireMethodToPath(resolved),
		configyang.PathToDescription(resolved), configyang.PathToHelp(resolved), configyang.PathToArgDefs(resolved), configyang.BuildCommandTree(resolved)))
	response, err := dispatcher.Dispatch(&pluginserver.CommandContext{Username: "operator", Sender: zePlugin.OperatorSender()},
		"request data restore path "+path+" full")
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusError, response.Status)
	require.Contains(t, response.Error, "lifecycle")
	current, err := storage.ReadActiveConfig(store, "ze.conf")
	require.NoError(t, err)
	require.Equal(t, "current", string(current))
}

// TestFullRestoreRPCAcceptsBeforeRestartWithoutReplacingLiveStore proves actual
// dispatch stages a complete source, preserves the live bytes through response
// delivery and changes storage only after the outer owner closes.
func TestFullRestoreRPCAcceptsBeforeRestartWithoutReplacingLiveStore(t *testing.T) {
	owner, dir := fullRestoreOwner(t)
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	restarts := 0
	server.SetRestartFunc(func() { restarts++ })
	path := dataArtifact(t, "router.conf", "replacement")
	response, err := fullRestoreDispatcher(t).Dispatch(&pluginserver.CommandContext{
		Server: server, Username: "operator", Sender: zePlugin.OperatorSender(),
	}, "request data restore path "+path+" full")
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	data, ok := response.Data.(zePlugin.Map)
	require.True(t, ok)
	require.Equal(t, "accepted", data["state"])
	require.NotEmpty(t, data["digest"])
	require.Zero(t, restarts)
	current, err := storage.ReadActiveConfig(owner.Borrow(), "ze.conf")
	require.NoError(t, err)
	require.Equal(t, "current", string(current))
	response.TransportComplete()
	response.TransportComplete()
	require.Equal(t, 1, restarts)
	current, err = storage.ReadActiveConfig(owner.Borrow(), "ze.conf")
	require.NoError(t, err)
	require.Equal(t, "current", string(current))
	closed, err := owner.Close()
	require.NoError(t, err)
	require.NoError(t, closed.Apply())
	restored, err := storage.Open(dir)
	require.NoError(t, err)
	content, err := storage.ReadActiveConfig(restored, "router.conf")
	require.NoError(t, err)
	require.Equal(t, "replacement", string(content))
	require.NoError(t, restored.Close())
}

// TestFullRestoreRPCRefusesCanceledAcceptance proves a canceled caller cannot
// stage a source or trigger its ready restart action through actual dispatch.
func TestFullRestoreRPCRefusesCanceledAcceptance(t *testing.T) {
	owner, dir := fullRestoreOwner(t)
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	server.SetRestartFunc(func() { t.Error("canceled full restore restarted the daemon") })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := dataArtifact(t, "router.conf", "replacement")
	response, err := fullRestoreDispatcher(t).Dispatch(&pluginserver.CommandContext{
		Server: server, Username: "operator", Sender: zePlugin.OperatorSender(), RequestContext: ctx,
	}, "request data restore path "+path+" full")
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusError, response.Status)
	require.Contains(t, response.Error, "canceled")
	response.TransportComplete()
	stages, err := filepath.Glob(filepath.Join(dir, "database.recovery-tmp-*"))
	require.NoError(t, err)
	require.Empty(t, stages)
	current, err := storage.ReadActiveConfig(owner.Borrow(), "ze.conf")
	require.NoError(t, err)
	require.Equal(t, "current", string(current))
}
