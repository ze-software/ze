package cli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
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

// keyArgs answers tokens judged against the leaf ze-data-cmd.yang declares for
// `delete data key`, as the dispatcher judges them.
func keyArgs(t *testing.T, tokens ...string) command.ValidatedArgs {
	t.Helper()
	defs := []command.ArgDef{commandtest.Must(command.NewStringArg(keywordKey,
		[]command.UintRange{{Min: 1, Max: 512}}, nil, command.ArgOptions{Mandatory: true}))}
	validated, err := command.ValidateArgs(tokens, defs, nil)
	require.NoError(t, err)
	return validated
}

// keyCall runs the key removal handler on tokens judged by keyArgs.
func keyCall(t *testing.T, tokens ...string) *zePlugin.Response {
	t.Helper()
	response, err := handleDataKeyDelete(&pluginserver.CommandContext{}, keyArgs(t, tokens...))
	require.NoError(t, err)
	return response
}

// TestDeleteKeyRemovesRuntimeKey verifies the daemon removal through its
// handler: a runtime-state key written to the live store is gone after
// `delete data key`, typed with and without the key keyword, and the answer
// names the key it removed.
// VALIDATES: AC-3 (daemon half).
// PREVENTS: a removal that answers success and leaves the key, or opens a second writer.
func TestDeleteKeyRemovesRuntimeKey(t *testing.T) {
	store, _ := dataRPCStore(t, func(context.Context) error { return nil })
	for _, tokens := range [][]string{{"meta/test/state"}, {keywordKey, "meta/test/state"}} {
		require.NoError(t, store.WriteKey("meta/test/state", []byte("stale")))

		response := keyCall(t, tokens...)
		require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
		data, ok := response.Data.(zePlugin.Map)
		require.True(t, ok)
		assert.Equal(t, "meta/test/state", data[keywordKey], tokens)
		_, err := store.ReadKey("meta/test/state")
		assert.ErrorIs(t, err, fs.ErrNotExist, "the key is gone from the live store")
	}
}

// TestDeleteKeyAbsentIsAnError verifies a key the store does not hold is
// refused, as the local ze data rm reports it, and the refusal names the key
// and the command that lists the keys.
// VALIDATES: AC-3 (survivor: an absent key is an error), R-4.
// PREVENTS: the daemon answering success for a key it never removed.
func TestDeleteKeyAbsentIsAnError(t *testing.T) {
	dataRPCStore(t, func(context.Context) error { return nil })

	response := keyCall(t, "meta/test/never-written")
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "holds no key meta/test/never-written")
	assert.Contains(t, response.Error, "fix: show data list")
}

// TestKeyCommandsRefuseConfigurationKey verifies a key that is not runtime
// state (under file/, under meta/config/, or outside both namespaces) is
// refused before the store is touched: every key the store held is still
// there. The refusal says the key is configuration, what the command reaches,
// and names the command that manages a config.
// VALIDATES: AC-4 (delete).
// PREVENTS: a raw removal of a config file or pointer that bypasses versions and history.
func TestKeyCommandsRefuseConfigurationKey(t *testing.T) {
	store, _ := dataRPCStore(t, func(context.Context) error { return nil })
	before, err := store.ListKeys("")
	require.NoError(t, err)
	require.NotEmpty(t, before, "the store holds the daemon's config")

	for _, key := range []string{"file/active/ze.conf", "meta/config/ze.conf", "ze.conf"} {
		response := keyCall(t, key)
		assert.Equal(t, zePlugin.StatusError, response.Status, key)
		assert.Contains(t, response.Error, "delete data key: "+key+" is configuration", key)
		assert.Contains(t, response.Error, "a key under meta/ outside meta/config/", key)
		assert.Contains(t, response.Error, "fix: update data config name <name> content <base64>", key)
	}

	after, err := store.ListKeys("")
	require.NoError(t, err)
	assert.Equal(t, before, after, "no key was removed")
}

// standInPluginState replaces the two readers the running-owner guard reads,
// the key grants and the plugin snapshot, with fixed answers, and restores the
// real readers when the test ends. owners maps a key to the plugin process
// names granted it; a key it does not name has no owner.
func standInPluginState(t *testing.T, owners map[string][]string, snapshot pluginserver.DoctorSnapshot, snapshotErr error) {
	t.Helper()
	savedOwners, savedStates := keyOwners, pluginStates
	keyOwners = func(key string) []string { return owners[key] }
	pluginStates = func() (pluginserver.DoctorSnapshot, error) { return snapshot, snapshotErr }
	t.Cleanup(func() { keyOwners, pluginStates = savedOwners, savedStates })
}

// TestKeyCommandsRefuseKeyOfRunningPlugin verifies the running-owner guard of
// `delete data key`. The grants and the plugin snapshot are stood in, so each
// case fixes who owns the key and whether that owner runs, and the live store
// is read back after the command. A key whose granted plugin runs is refused
// and kept, and the refusal names the plugin and its restart. A key whose
// owner does not run, or that no plugin is granted, is removed; a key with no
// owner is removed even while the snapshot is unavailable, because the guard
// never asks for it. A key with an owner is refused while the snapshot is
// unavailable, saying it cannot tell yet whether the owner runs.
// VALIDATES: AC-5 (delete).
// PREVENTS: a removal that takes runtime state from under a running plugin.
func TestKeyCommandsRefuseKeyOfRunningPlugin(t *testing.T) {
	const key = "meta/test/owned"
	granted := map[string][]string{key: {"test-owner"}}
	ownerRuns := pluginserver.DoctorSnapshot{Plugins: []pluginserver.PluginDoctorState{
		{Name: "test-other", Configured: true, HasProcess: true, Running: true},
		{Name: "test-owner", Configured: true, HasProcess: true, Running: true},
	}}
	ownerStopped := pluginserver.DoctorSnapshot{Plugins: []pluginserver.PluginDoctorState{
		{Name: "test-other", Configured: true, HasProcess: true, Running: true},
		{Name: "test-owner", Configured: true, HasProcess: true, Running: false, Down: true},
	}}
	ownerAbsent := pluginserver.DoctorSnapshot{Plugins: []pluginserver.PluginDoctorState{
		{Name: "test-other", Configured: true, HasProcess: true, Running: true},
	}}

	refused := []struct {
		name        string
		owners      map[string][]string
		snapshot    pluginserver.DoctorSnapshot
		snapshotErr error
		want        []string
	}{
		{"owner runs", granted, ownerRuns, nil, []string{
			"delete data key: plugin test-owner manages " + key + " while it runs",
			"so the key is unchanged",
			"fix: ze request plugin restart test-owner",
		}},
		{"snapshot not available", granted, pluginserver.DoctorSnapshot{}, pluginserver.ErrDoctorSnapshotNotStarted, []string{
			"delete data key: cannot tell yet whether the plugin that owns " + key + " runs",
			"the key is granted to test-owner",
			pluginserver.ErrDoctorSnapshotNotStarted.Error(),
			"fix: ze show health",
		}},
	}
	for i := range refused {
		tc := &refused[i]
		t.Run(tc.name, func(t *testing.T) {
			store, _ := dataRPCStore(t, func(context.Context) error { return nil })
			standInPluginState(t, tc.owners, tc.snapshot, tc.snapshotErr)
			require.NoError(t, store.WriteKey(key, []byte("live state")))

			response := keyCall(t, key)
			assert.Equal(t, zePlugin.StatusError, response.Status)
			for _, want := range tc.want {
				assert.Contains(t, response.Error, want)
			}
			assert.NotContains(t, response.Error, "test-other", "a running plugin that holds no grant is not named")
			data, err := store.ReadKey(key)
			require.NoError(t, err, "the key is still in the live store")
			assert.Equal(t, "live state", string(data), "the key is unchanged")
		})
	}

	removed := []struct {
		name        string
		owners      map[string][]string
		snapshot    pluginserver.DoctorSnapshot
		snapshotErr error
	}{
		{"owner stopped", granted, ownerStopped, nil},
		{"owner not in the snapshot", granted, ownerAbsent, nil},
		{"no plugin holds a grant", nil, ownerRuns, nil},
		{"no grant and no snapshot", nil, pluginserver.DoctorSnapshot{}, pluginserver.ErrDoctorSnapshotNotPublished},
	}
	for i := range removed {
		tc := &removed[i]
		t.Run(tc.name, func(t *testing.T) {
			store, _ := dataRPCStore(t, func(context.Context) error { return nil })
			standInPluginState(t, tc.owners, tc.snapshot, tc.snapshotErr)
			require.NoError(t, store.WriteKey(key, []byte("stale state")))

			response := keyCall(t, key)
			require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
			_, err := store.ReadKey(key)
			assert.ErrorIs(t, err, fs.ErrNotExist, "the key is gone from the live store")
		})
	}
}

// TestDataCommandsRefuseWithNoStore verifies every data RPC refuses while the
// daemon publishes no store (starting, stopping, or running with none),
// instead of reaching a nil or closed store.
// VALIDATES: R-10.
// PREVENTS: a nil dereference for a command that arrives during shutdown.
func TestDataCommandsRefuseWithNoStore(t *testing.T) {
	InstallDataRPC(nil)

	response := keyCall(t, "meta/test/state")
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "delete data key: this daemon serves no store")
	assert.Contains(t, response.Error, "fix: ze data rm <key>")

	for _, handler := range []struct {
		name    string
		handler pluginserver.Handler
	}{
		{"request data backup", handleDataBackup},
		{"request data restore", handleDataRestore},
	} {
		response := dataCall(t, handler.handler, "path", "/tmp/backup.zefs", "config")
		assert.Equal(t, zePlugin.StatusError, response.Status, handler.name)
		assert.Contains(t, response.Error, handler.name+": this daemon serves no store", handler.name)
	}
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
