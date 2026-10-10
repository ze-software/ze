package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/zefs"
)

// dataRPCStore installs a live tree store as the RPC target with reload, and
// removes it when the test ends.
func dataRPCStore(t *testing.T, reload func(context.Context) error) (storage.Storage, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Create(dir)
	require.NoError(t, err)
	configPath := filepath.Join(dir, "ze.conf")
	_, _, err = storage.EnsureActiveVersion(store, configPath, []byte("current"), time.Now().Add(-time.Second))
	require.NoError(t, err)
	InstallDataRPC(&DataRPCTarget{Store: store, ConfigPath: configPath, Reload: reload})
	t.Cleanup(func() {
		InstallDataRPC(nil)
		require.NoError(t, store.Close())
	})
	return store, configPath
}

// dataArtifact writes a blob artifact holding one config.
func dataArtifact(t *testing.T, name, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "backup.zefs")
	artifact, err := storage.CreateBlob(path, zefs.Spare(0))
	require.NoError(t, err)
	_, err = storage.RestoreConfig(artifact, name, []byte(text))
	require.NoError(t, err)
	require.NoError(t, artifact.Close())
	return path
}

func dataCall(t *testing.T, handler pluginserver.Handler, args ...string) *zePlugin.Response {
	t.Helper()
	response, err := handler(&pluginserver.CommandContext{}, commandtest.Args(args...))
	require.NoError(t, err)
	return response
}

// TestBackupRPCPath verifies AC-4's refusals through the RPC entry point:
// relative, "..", symlink, existing without force, and a store-owned name
// with or without force, each naming its rule; an absolute path is written
// and answered with path, key count and size.
// VALIDATES: AC-4, R-10.
// PREVENTS: the RPC writing a file the operator did not mean, or one the store owns.
func TestBackupRPCPath(t *testing.T) {
	store, configPath := dataRPCStore(t, func(context.Context) error { return nil })
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.zefs")
	require.NoError(t, os.WriteFile(existing, []byte("old"), 0o600))
	link := filepath.Join(dir, "link.zefs")
	require.NoError(t, os.Symlink(existing, link))
	owned := filepath.Join(filepath.Dir(configPath), "database.zefs")
	for _, refused := range []struct {
		args []string
		rule string
	}{
		{[]string{"path", "out.zefs"}, "absolute"},
		{[]string{"path", dir + "/a/../out.zefs"}, `".."`},
		{[]string{"path", link}, "symlink"},
		{[]string{"path", existing}, "exists"},
		{[]string{"path", owned}, "canonical blob"},
		{[]string{"path", owned, "force"}, "canonical blob"},
		{[]string{"path", filepath.Join(dir, "x.zefs"), "spare", "101"}, "0 to 100"},
		{[]string{"spare", "0"}, "path <absolute-file> is required"},
	} {
		response := dataCall(t, handleDataBackup, refused.args...)
		assert.Equal(t, zePlugin.StatusError, response.Status, refused.args)
		assert.Contains(t, response.Error, refused.rule, refused.args)
	}

	response := dataCall(t, handleDataBackup, "path", existing, "force")
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	data, ok := response.Data.(zePlugin.Map)
	require.True(t, ok)
	assert.Equal(t, existing, data[keywordPath])
	keys, err := store.ListKeys("")
	require.NoError(t, err)
	assert.Equal(t, len(keys), data["keys"])
	info, err := os.Stat(existing)
	require.NoError(t, err)
	assert.Equal(t, info.Size(), data["bytes"])
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// TestRestoreConfigLive verifies AC-8's order through the RPC: the artifact's
// config is staged as the candidate, the reload runs with it staged, and the
// answer names the source and the device's config.
// VALIDATES: AC-8, A-5.
// PREVENTS: a live restore writing the active config before the reload accepts it.
func TestRestoreConfigLive(t *testing.T) {
	var store storage.Storage
	var configPath string
	reloaded := false
	store, configPath = dataRPCStore(t, func(context.Context) error {
		candidate, _, ok, err := storage.ReadCandidateConfig(store, configPath)
		require.NoError(t, err)
		require.True(t, ok, "the reload runs with the candidate staged")
		assert.Equal(t, "restored", string(candidate))
		active, err := storage.ReadActiveConfig(store, configPath)
		require.NoError(t, err)
		assert.Equal(t, "current", string(active), "nothing is active before the reload accepts")
		reloaded = true
		return storage.PromoteCandidate(store, configPath)
	})
	source := dataArtifact(t, "other.conf", "restored")

	response := dataCall(t, handleDataRestore, "path", source, "config")
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	assert.True(t, reloaded)
	data, ok := response.Data.(zePlugin.Map)
	require.True(t, ok)
	assert.Equal(t, "other.conf", data["source-name"])
	assert.Equal(t, "ze.conf", data["config-name"])
	active, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "restored", string(active))
}

// TestRestoreConfigRejectedLeavesPointers verifies a refused reload leaves the
// active config as it was and the answer says so; a staged change, a missing
// config keyword and a name the artifact lacks are refused before staging.
// VALIDATES: AC-8.
// PREVENTS: a rejected restore reported as success, or staged over another edit.
func TestRestoreConfigRejectedLeavesPointers(t *testing.T) {
	var store storage.Storage
	var configPath string
	store, configPath = dataRPCStore(t, func(context.Context) error {
		require.NoError(t, storage.ClearCandidate(store, configPath))
		return errors.New("verify refused")
	})
	source := dataArtifact(t, "ze.conf", "restored")

	response := dataCall(t, handleDataRestore, "path", source, "config")
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "active config is unchanged")
	active, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "current", string(active))

	for _, refused := range []struct {
		args []string
		rule string
	}{
		{[]string{"path", source}, "config keyword is required"},
		{[]string{"path", source, "config", "name", "missing.conf"}, "holds: ze.conf"},
		{[]string{"path", source, "full"}, "unknown keyword"},
	} {
		response := dataCall(t, handleDataRestore, refused.args...)
		assert.Equal(t, zePlugin.StatusError, response.Status, refused.args)
		assert.Contains(t, response.Error, refused.rule, refused.args)
	}

	_, err = storage.WriteCandidateVersion(store, configPath, []byte("staged"), time.Now())
	require.NoError(t, err)
	response = dataCall(t, handleDataRestore, "path", source, "config")
	assert.Contains(t, response.Error, "already staged")
}

// TestRestoreClientConfigLive verifies AC-8's client target through the RPC
// entry point: a hub that serves the client writes the artifact's config as
// the client's served config, a new active version, without
// running its own reload; the write reaches the observer that pushes
// config-changed under the client's name. A daemon serving no client, and a
// hub without that client entry, refuse naming why and write nothing.
// VALIDATES: AC-8 (client target), R-4 (selection unchanged).
// PREVENTS: a client restore that reloads the hub, or writes a key no client is served from.
func TestRestoreClientConfigLive(t *testing.T) {
	reloads := 0
	store, configPath := dataRPCStore(t, func(context.Context) error {
		reloads++
		return nil
	})
	clientKey := zefs.KeyFileActive.Key(pluginserver.ClientConfigKey("edge"))
	_, _, err := storage.EnsureActiveVersion(store, pluginserver.ClientConfigKey("edge"), []byte("served before"), time.Now().Add(-time.Second))
	require.NoError(t, err)
	artifact := dataArtifact(t, "yesterday.conf", "served after")

	response := dataCall(t, handleDataRestore, "path", artifact, "config", "client", "edge")
	require.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "serves no managed client")

	var pushed []string
	store.SetWriteObserver(func(key string) {
		if name, ok := pluginserver.ClientNameFromConfigKey(key); ok {
			pushed = append(pushed, name)
		}
	})
	InstallDataRPC(&DataRPCTarget{Store: store, ConfigPath: configPath, Reload: func(context.Context) error {
		reloads++
		return nil
	}, ServesClient: func(name string) bool { return name == "edge" }})

	response = dataCall(t, handleDataRestore, "path", artifact, "config", "client", "core")
	require.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "no client entry named core")
	assert.Empty(t, pushed)

	response = dataCall(t, handleDataRestore, "path", artifact, "config", "client", "edge")
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	data, ok := response.Data.(zePlugin.Map)
	require.True(t, ok)
	assert.Equal(t, "client-edge.conf", data["config-name"])
	assert.Equal(t, "yesterday.conf", data["source-name"])
	served, err := store.ReadKey(clientKey)
	require.NoError(t, err)
	assert.Equal(t, "served after", string(served))
	active, err := storage.ReadActiveConfig(store, "client-edge.conf")
	require.NoError(t, err)
	assert.Equal(t, "served after", string(active), "the served config is a promoted version")
	own, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "current", string(own), "the hub's own config is untouched")
	assert.Zero(t, reloads, "a client restore MUST NOT reload the hub")
	assert.Contains(t, pushed, "edge")
}
