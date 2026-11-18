// Design: docs/architecture/storage-backends.md -- stored-config rollback through the daemon.

package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/pkg/zefs"
)

func configRollbackDispatcher(t *testing.T) *pluginserver.Dispatcher {
	t.Helper()
	resolved, err := configyang.DefaultLoader()
	require.NoError(t, err)
	dispatcher := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(dispatcher, configyang.WireMethodToPath(resolved),
		configyang.PathToDescription(resolved), configyang.PathToHelp(resolved), configyang.PathToArgDefs(resolved), configyang.BuildCommandTree(resolved)))
	require.NotNil(t, dispatcher.Lookup("request data config rollback"), "the daemon exposes no indexed stored rollback")
	return dispatcher
}

// TestConfigRollbackRPCPublishesTheInactiveStoredRevision proves actual daemon
// dispatch changes the named config's active version without reloading its own
// config, and records the successful mutation with the original actor.
func TestConfigRollbackRPCPublishesTheInactiveStoredRevision(t *testing.T) {
	reloads := 0
	store, _ := dataRPCStore(t, func(context.Context) error { reloads++; return nil })
	_, _, err := storage.EnsureActiveVersion(store, "other.conf", []byte("other current"), time.Now())
	require.NoError(t, err)
	historical := time.Now().Add(-time.Hour)
	require.NoError(t, store.WriteVersion("other.conf", []byte("other historical"), historical))
	dispatcher := configRollbackDispatcher(t)
	recorder := &configRenameAudit{}
	dispatcher.SetAuditRecorder(recorder)
	const line = "request data config rollback name other.conf revision 2"
	response, err := dispatcher.Dispatch(&pluginserver.CommandContext{
		Username: "rollback-operator", Surface: audit.Web, Sender: zePlugin.OperatorSender(),
	}, line)
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	data, ok := response.Data.(zePlugin.Map)
	require.True(t, ok)
	require.Equal(t, "other.conf", data["name"])
	require.Equal(t, 2, data["revision"])
	require.Equal(t, storage.FormatVersionStamp(historical), data["source-version"])
	require.Equal(t, historical.Truncate(time.Millisecond), data["source-time"])
	require.NotEmpty(t, data["version"])
	active, err := storage.ReadActiveConfig(store, "other.conf")
	require.NoError(t, err)
	require.Equal(t, "other historical", string(active))
	require.Zero(t, reloads)
	require.Len(t, recorder.entries, 1)
	require.Equal(t, "data-config-rollback", recorder.entries[0].Action)
	require.Equal(t, "rollback-operator", recorder.entries[0].Actor)
	require.Equal(t, line, recorder.entries[0].Detail)
}

// TestConfigRollbackRPCStagesTheRunningSourceThroughReload proves the active
// source stays current during staging and changes only through the configured
// reload callback's normal pointer promotion.
func TestConfigRollbackRPCStagesTheRunningSourceThroughReload(t *testing.T) {
	var source storage.Storage
	var sourcePath string
	reloads := 0
	store, path := dataRPCStore(t, func(context.Context) error {
		reloads++
		active, err := storage.ReadActiveConfig(source, sourcePath)
		require.NoError(t, err)
		require.Equal(t, "current", string(active))
		candidate, _, present, err := storage.ReadCandidateConfig(source, sourcePath)
		require.NoError(t, err)
		require.True(t, present)
		require.Equal(t, "historical runtime config", string(candidate))
		return storage.PromoteCandidate(source, sourcePath)
	})
	source, sourcePath = store, path
	require.NoError(t, store.WriteVersion(path, []byte("historical runtime config"), time.Now().Add(-time.Hour)))
	response, err := configRollbackDispatcher(t).Dispatch(&pluginserver.CommandContext{
		Username: "rollback-operator", Sender: zePlugin.OperatorSender(), RequestContext: context.Background(),
	}, "request data config rollback name "+filepath.Base(path)+" revision 2")
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	require.Equal(t, 1, reloads)
	active, err := storage.ReadActiveConfig(store, path)
	require.NoError(t, err)
	require.Equal(t, "historical runtime config", string(active))
}

// TestConfigRollbackRPCRespectsTheLiveConfirmWindow proves an open window
// refuses active rollback before staging or invoking reload.
func TestConfigRollbackRPCRespectsTheLiveConfirmWindow(t *testing.T) {
	reloads := 0
	store, path := dataRPCStore(t, func(context.Context) error { reloads++; return nil })
	require.NoError(t, store.WriteVersion(path, []byte("historical runtime config"), time.Now().Add(-time.Hour)))
	window := confirm.NewWindow(func([]byte) error { return nil }, confirm.NewStoreRecorder(store, path))
	t.Cleanup(window.Stop)
	require.NoError(t, window.Confirmed("alice", time.Minute, false, confirm.Commit{
		Snapshot: func() ([]byte, error) { return storage.ReadActiveConfig(store, path) },
		Apply:    func() error { return nil },
	}))
	InstallDataRPC(&DataRPCTarget{
		Store: store, ConfigPath: path, Reload: func(context.Context) error { reloads++; return nil },
		Window: func() *confirm.Window { return window },
	})
	response, err := configRollbackDispatcher(t).Dispatch(&pluginserver.CommandContext{
		Username: "rollback-operator", Sender: zePlugin.OperatorSender(),
	}, "request data config rollback name "+filepath.Base(path)+" revision 2")
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusError, response.Status)
	require.Contains(t, response.Error, "alice")
	require.Zero(t, reloads)
	_, _, candidate, err := storage.ReadCandidateConfig(store, path)
	require.NoError(t, err)
	require.False(t, candidate)
	active, err := storage.ReadActiveConfig(store, path)
	require.NoError(t, err)
	require.Equal(t, "current", string(active))
}

// TestConfigRollbackRPCPreservesAnotherStagedCandidate proves neither active
// nor inactive rollback can replace an existing candidate's bytes or pointer.
func TestConfigRollbackRPCPreservesAnotherStagedCandidate(t *testing.T) {
	for _, active := range []bool{false, true} {
		name := "inactive"
		if active {
			name = "active"
		}
		t.Run(name, func(t *testing.T) {
			reloads := 0
			store, path := dataRPCStore(t, func(context.Context) error { reloads++; return nil })
			selectedName := filepath.Base(path)
			if !active {
				selectedName = "other.conf"
				_, _, err := storage.EnsureActiveVersion(store, selectedName, []byte("other active"), time.Now().Add(-time.Hour))
				require.NoError(t, err)
			}
			stamp, err := storage.WriteCandidateVersion(store, selectedName, []byte("staged work"), time.Now())
			require.NoError(t, err)
			response, err := configRollbackDispatcher(t).Dispatch(&pluginserver.CommandContext{
				Username: "rollback-operator", Sender: zePlugin.OperatorSender(),
			}, "request data config rollback name "+selectedName+" revision 2")
			require.NoError(t, err)
			require.Equal(t, zePlugin.StatusError, response.Status)
			require.Contains(t, response.Error, "staged")
			candidate, gotStamp, present, err := storage.ReadCandidateConfig(store, selectedName)
			require.NoError(t, err)
			require.True(t, present)
			require.Equal(t, stamp, gotStamp)
			require.Equal(t, "staged work", string(candidate))
			require.Zero(t, reloads)
		})
	}
}

// TestConfigRollbackRPCPreservesAnUnfinishedFileCommit proves both the active
// reload route and inactive publication route refuse a recorded file handoff
// before staging, reloading or replacing its source identity.
func TestConfigRollbackRPCPreservesAnUnfinishedFileCommit(t *testing.T) {
	for _, active := range []bool{false, true} {
		label := "inactive"
		if active {
			label = "active"
		}
		t.Run(label, func(t *testing.T) {
			reloads := 0
			store, path := dataRPCStore(t, func(context.Context) error { reloads++; return nil })
			name := filepath.Base(path)
			if !active {
				name = "other.conf"
				_, _, err := storage.EnsureActiveVersion(store, name, []byte("other active"), time.Now())
				require.NoError(t, err)
			}
			require.NoError(t, store.WriteVersion(name, []byte("historical config"), time.Now().Add(-time.Hour)))
			key := zefs.KeyConfigFileCommit.Key(name)
			require.NoError(t, store.WriteKey(key, []byte("pending file handoff")))
			response, err := configRollbackDispatcher(t).Dispatch(&pluginserver.CommandContext{
				Username: "rollback-operator", Sender: zePlugin.OperatorSender(),
			}, "request data config rollback name "+name+" revision 2")
			require.NoError(t, err)
			require.Equal(t, zePlugin.StatusError, response.Status)
			require.Contains(t, response.Error, "file commit")
			require.Zero(t, reloads)
			_, _, candidate, err := storage.ReadCandidateConfig(store, name)
			require.NoError(t, err)
			require.False(t, candidate)
			intent, err := store.ReadKey(key)
			require.NoError(t, err)
			require.Equal(t, "pending file handoff", string(intent))
		})
	}
}
