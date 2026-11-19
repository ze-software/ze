// Design: docs/architecture/fleet-config.md -- the freeze: a hub that is itself managed still writes the configurations of the clients it declares
// Related: managed_machine_writers_test.go -- the machine writers on a managed store
// Related: session_factory_test.go -- the session editor on a declared client's configuration

package hub

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/zefs"
)

// The configurations of a hub that is itself managed: its own, and the one it
// serves to the client it declares.
const (
	hubFreezeOwnConfig    = "hub.conf"
	hubFreezeClient       = "edge"
	hubFreezeClientConfig = "client-edge.conf"
	hubFreezeOwnText      = "the hub's own configuration"
	hubFreezeClientText   = "the configuration of edge"
	// The wire method `update data config` dispatches to
	// (internal/component/config/storage/cli/register_data_rpc.go).
	hubFreezeImportMethod = "ze-config:data-config-update"
	// The last line of the freeze's refusal (storage.ErrConfigFrozen).
	hubFreezeFixLine = "fix: request fleet disable"
)

// hubFreezeImport answers the handler the daemon runs for
// `update data config name <name> content <base64>`, found by its wire method
// in the registry the dispatcher reads, as a call that takes the name and the
// text to import.
func hubFreezeImport(t *testing.T) func(name, text string) *zePlugin.Response {
	t.Helper()
	for _, registration := range pluginserver.AllBuiltinRPCs() {
		if registration.WireMethod != hubFreezeImportMethod {
			continue
		}
		handler := registration.Handler
		return func(name, text string) *zePlugin.Response {
			t.Helper()
			// The tokens are judged against a name and a content leaf, as the
			// dispatcher judges them against ze-data-cmd.yang, so the handler
			// finds each value under its keyword.
			defs := []command.ArgDef{
				commandtest.Must(command.NewStringArg("name", nil, nil, command.ArgOptions{})),
				commandtest.Must(command.NewStringArg("content", nil, nil, command.ArgOptions{})),
			}
			validated, err := command.ValidateArgs([]string{"name", name, "content", base64.StdEncoding.EncodeToString([]byte(text))}, defs, nil)
			require.NoError(t, err)
			response, err := handler(&pluginserver.CommandContext{}, validated)
			require.NoError(t, err)
			return response
		}
	}
	t.Fatalf("no handler is registered for %s", hubFreezeImportMethod)
	return nil
}

// TestManagedHubImportsDeclaredClientConfig proves that a hub that is itself
// managed still writes the configuration of a client it declares, while the
// freeze refuses a change to its own configuration.
//
// Method: the hub's store holds its own configuration hub.conf as an active
// version and meta/instance/managed true, and the handle is bound to hub.conf
// as a daemon started from its store binds it (ConfigSourceStored). The data
// RPC target is the one the hub installs once its managed server runs: it
// serves the client edge. `update data config` of client-edge.conf MUST be
// accepted, record a version the active pointer names, reach the write
// observer that pushes config-changed to edge, and reload nothing. The same
// command on hub.conf MUST be refused with the freeze's operator error, its
// fix last, with no reload, no candidate and the active configuration
// unchanged.
//
// VALIDATES: spec-fleet-6 AC-19 -- a managed hub imports the configuration of
// a client it declares, and the client is notified; its own configuration is
// frozen as in AC-16.
// PREVENTS: a freeze that refuses every configuration of a managed store
// rather than the daemon's own (the scope check of refuseFrozenConfigLocked,
// sameConfigName), which would stop a managed hub from serving its fleet.
func TestManagedHubImportsDeclaredClientConfig(t *testing.T) {
	dir := t.TempDir()
	owner, err := storage.Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		storagecli.InstallDataRPC(nil)
		require.NoError(t, owner.Close())
	})
	ownPath := filepath.Join(dir, hubFreezeOwnConfig)
	_, _, err = storage.EnsureActiveVersion(owner, ownPath, []byte(hubFreezeOwnText), time.Now().Add(-time.Second))
	require.NoError(t, err)
	require.NoError(t, owner.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true\n")))
	var notified []string
	owner.SetWriteObserver(func(key string) {
		if name, ok := pluginserver.ClientNameFromConfigKey(key); ok {
			notified = append(notified, name)
		}
	})
	store := storage.BindConfigSource(owner, ownPath, storage.ConfigSourceStored)
	reloads := 0
	storagecli.InstallDataRPC(&storagecli.DataRPCTarget{
		Store:      store,
		ConfigPath: ownPath,
		Reload: func(context.Context) error {
			reloads++
			return nil
		},
		ServesClient: func(name string) bool { return name == hubFreezeClient },
	})
	importConfig := hubFreezeImport(t)

	response := importConfig(hubFreezeClientConfig, hubFreezeClientText)
	require.Equal(t, zePlugin.StatusDone, response.Status, "a managed hub imports a declared client's configuration: %s", response.Error)
	answer, ok := response.Data.(zePlugin.Map)
	require.True(t, ok, "the import answers a map")
	version, ok := answer["version"].(string)
	require.True(t, ok, "the import answers the version it recorded: %v", answer)
	require.NotEmpty(t, version)
	pointer, err := owner.ReadKey(zefs.KeyConfigActive.Key(hubFreezeClientConfig))
	require.NoError(t, err, "the client's configuration is recorded as a version")
	assert.Equal(t, version, strings.TrimSpace(string(pointer)), "the version the answer names is the active one")
	active, err := storage.ReadActiveConfig(store, hubFreezeClientConfig)
	require.NoError(t, err)
	assert.Equal(t, hubFreezeClientText, string(active))
	assert.Contains(t, notified, hubFreezeClient, "the write observer hears the declared client's configuration, which pushes config-changed")
	assert.Zero(t, reloads, "a client's configuration reloads nothing on the hub")

	response = importConfig(hubFreezeOwnConfig, "the hub's own configuration, edited")
	require.Equal(t, zePlugin.StatusError, response.Status, "the managed hub's own configuration is frozen")
	assert.Contains(t, response.Error, "this device is managed by a hub")
	lines := strings.Split(strings.TrimSpace(response.Error), "\n")
	assert.Equal(t, hubFreezeFixLine, lines[len(lines)-1], "the fix is the last line: %s", response.Error)
	assert.Zero(t, reloads, "a frozen import runs no reload")
	own, err := storage.ReadActiveConfig(store, ownPath)
	require.NoError(t, err)
	assert.Equal(t, hubFreezeOwnText, string(own), "the hub's own configuration is unchanged")
	_, _, staged, err := storage.ReadCandidateConfig(store, ownPath)
	require.NoError(t, err)
	assert.False(t, staged, "a frozen import leaves no candidate")
}
