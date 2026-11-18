// Design: docs/architecture/storage-backends.md -- daemon-owned complete config rename.

package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
)

func configRenameDispatcher(t *testing.T) *pluginserver.Dispatcher {
	t.Helper()
	resolved, err := configyang.DefaultLoader()
	require.NoError(t, err)
	dispatcher := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(dispatcher, configyang.WireMethodToPath(resolved),
		configyang.PathToDescription(resolved), configyang.PathToHelp(resolved), configyang.PathToArgDefs(resolved), configyang.BuildCommandTree(resolved)))
	require.NotNil(t, dispatcher.Lookup("request data config rename"), "the daemon exposes no stored-config rename")
	return dispatcher
}

// TestConfigRenameRPCUsesTheOwnedStore proves the actual daemon dispatcher
// renames a stored configuration's version and draft without reloading the box.
func TestConfigRenameRPCUsesTheOwnedStore(t *testing.T) {
	reloads := 0
	store, _ := dataRPCStore(t, func(context.Context) error { reloads++; return nil })
	stamp, _, err := storage.EnsureActiveVersion(store, "other.conf", []byte("other active"), time.Now())
	require.NoError(t, err)
	require.NoError(t, store.WriteFile("other.conf.draft", []byte("saved work"), 0o600))
	response, err := configRenameDispatcher(t).Dispatch(&pluginserver.CommandContext{
		Username: "config-operator", Sender: zePlugin.OperatorSender(), RequestContext: context.Background(),
	}, "request data config rename old other.conf new renamed.conf")
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	require.Equal(t, zePlugin.Map{"old": "other.conf", "new": "renamed.conf"}, response.Data)
	active, err := storage.ReadActiveConfig(store, "renamed.conf")
	require.NoError(t, err)
	require.Equal(t, "other active", string(active))
	version, err := store.ReadVersion("renamed.conf", stamp)
	require.NoError(t, err)
	require.Equal(t, active, version)
	draft, err := store.ReadFile("renamed.conf.draft")
	require.NoError(t, err)
	require.Equal(t, "saved work", string(draft))
	require.False(t, store.Exists("other.conf"))
	require.Zero(t, reloads)
}

// TestConfigRenameRPCRefusesTheRunningIdentity proves a rename cannot leave
// the daemon or its next startup referring to an absent source configuration.
func TestConfigRenameRPCRefusesTheRunningIdentity(t *testing.T) {
	store, path := dataRPCStore(t, func(context.Context) error { return nil })
	dispatcher := configRenameDispatcher(t)
	before, err := storage.ReadActiveConfig(store, path)
	require.NoError(t, err)
	response, err := dispatcher.Dispatch(&pluginserver.CommandContext{
		Username: "config-operator", Sender: zePlugin.OperatorSender(),
	}, "request data config rename old "+filepath.Base(path)+" new renamed.conf")
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusError, response.Status)
	require.Contains(t, response.Error, "running configuration")
	after, err := storage.ReadActiveConfig(store, path)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.False(t, store.Exists("renamed.conf"))
}

// TestConfigRenameRPCRefusesServedManagedIdentity proves a client's configured
// fetch name cannot be silently changed by renaming its stored configuration.
func TestConfigRenameRPCRefusesServedManagedIdentity(t *testing.T) {
	store, path := dataRPCStore(t, func(context.Context) error { return nil })
	_, _, err := storage.EnsureActiveVersion(store, "client-branch.conf", []byte("client configuration"), time.Now())
	require.NoError(t, err)
	InstallDataRPC(&DataRPCTarget{
		Store: store, ConfigPath: path, ServesClient: func(name string) bool { return name == "branch" },
	})
	response, err := configRenameDispatcher(t).Dispatch(&pluginserver.CommandContext{
		Username: "config-operator", Sender: zePlugin.OperatorSender(),
	}, "request data config rename old client-branch.conf new renamed.conf")
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, zePlugin.StatusError, response.Status)
	require.Contains(t, response.Error, "managed client")
	data, err := storage.ReadActiveConfig(store, "client-branch.conf")
	require.NoError(t, err)
	require.Equal(t, "client configuration", string(data))
	require.False(t, store.Exists("renamed.conf"))
}

// configRenameAudit records one dispatcher's entries in a serial test; no
// background writer or external sink is involved.
type configRenameAudit struct{ entries []audit.Entry }

func (r *configRenameAudit) Record(entry audit.Entry) error {
	r.entries = append(r.entries, entry)
	return nil
}

// TestConfigRenameRPCRecordsItsOwnAuditAction proves successful identity
// changes record the actor and command, while a refused rename records none.
func TestConfigRenameRPCRecordsItsOwnAuditAction(t *testing.T) {
	store, configPath := dataRPCStore(t, func(context.Context) error { return nil })
	require.NoError(t, store.WriteFile("other.conf", []byte("other config"), 0o600))
	dispatcher := configRenameDispatcher(t)
	recorder := &configRenameAudit{}
	dispatcher.SetAuditRecorder(recorder)
	const line = "request data config rename old other.conf new renamed.conf"
	response, err := dispatcher.Dispatch(&pluginserver.CommandContext{
		Username: "rename-operator", Surface: audit.Web, Sender: zePlugin.OperatorSender(),
	}, line)
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusDone, response.Status)
	require.Len(t, recorder.entries, 1)
	require.Equal(t, "data-config-rename", recorder.entries[0].Action)
	require.Equal(t, "rename-operator", recorder.entries[0].Actor)
	require.Equal(t, audit.Web, recorder.entries[0].Surface)
	require.Equal(t, line, recorder.entries[0].Detail)
	response, err = dispatcher.Dispatch(&pluginserver.CommandContext{
		Username: "rename-operator", Surface: audit.Web, Sender: zePlugin.OperatorSender(),
	}, "request data config rename old "+filepath.Base(configPath)+" new forbidden.conf")
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusError, response.Status)
	require.Len(t, recorder.entries, 1, "a refused rename recorded a mutation")
}
