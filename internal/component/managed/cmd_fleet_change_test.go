// Design: docs/architecture/fleet-config.md -- what `show fleet status` says of the freeze, and what a rejoin answers when its client did not start.
// Related: cmd_fleet.go -- readFleetStatus, frozenHandle, reportOf, clientStartRefusal.

package managed

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestFleetStatusFrozenFollowsTheHandle proves that `frozen` is the freeze's
// own answer for the handle the status is asked on, not a copy of `managed`:
// it reads one managed store through a stopped tool's handle and through a
// daemon's handle bound to each source a daemon starts from.
//
// VALIDATES: a managed store bound to a file or to stdin reports managed true
// and frozen false; bound to its stored configuration, or unbound, frozen true.
// PREVENTS: a status that tells the operator a change is refused on a daemon
// whose store admits it.
func TestFleetStatusFrozenFollowsTheHandle(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte(managedFlagTrue)))
	require.NoError(t, store.WriteFile(fleetStatusConfigName, []byte(fleetStatusConfig), 0))

	for name, want := range map[string]struct {
		handle storage.Storage
		frozen bool
	}{
		"a stopped tool's handle":              {store, true},
		"a daemon started from its store":      {storage.BindConfigSource(store, fleetStatusConfigName, storage.ConfigSourceStored), true},
		"a daemon started from a file":         {storage.BindConfigSource(store, "/etc/ze/router.conf", storage.ConfigSourceFile), false},
		"a daemon started from standard input": {storage.BindConfigSource(store, "-", storage.ConfigSourceStdin), false},
	} {
		t.Run(name, func(t *testing.T) {
			status, err := readFleetStatus(store, want.handle)

			require.NoError(t, err)
			assert.True(t, status.Managed)
			assert.Equal(t, want.frozen, status.Frozen)
		})
	}
}

// TestFleetStatusDaemonRouteAsksTheDaemonsHandle proves that the daemon route
// asks the freeze on the handle the daemon published, with its binding: the
// daemon here started from a file, which a fresh handle cannot know.
//
// VALIDATES: `show fleet status` answered by a daemon started from a file on a
// managed store reports managed true and frozen false.
// PREVENTS: frozenHandle reading the binding from the handle the status opened.
func TestFleetStatusDaemonRouteAsksTheDaemonsHandle(t *testing.T) {
	fleetStatusStore(t,
		map[string]string{zefs.KeyInstanceManaged.Pattern: "true"},
		map[string]string{fleetStatusConfigName: fleetStatusConfig})
	folder := resolve.StoreDir("")
	own, err := storage.OpenReadOnly(folder, folder)
	require.NoError(t, err)
	t.Cleanup(func() { own.Close() }) //nolint:errcheck // test cleanup
	statestore.SetStore(storage.BindConfigSource(own, "/etc/ze/router.conf", storage.ConfigSourceFile))
	t.Cleanup(func() { statestore.SetStore(nil) })

	got := fleetStatusOn(t, command.AnswerRemote, func() (fleetStatus, error) {
		t.Error("the daemon route asked a daemon")
		return fleetStatus{}, errNoDaemon
	})

	assert.True(t, got.Managed)
	assert.False(t, got.Frozen, "a daemon started from a file reported its configuration frozen")
}

// TestFleetStatusReportsStopping proves the word of a client a leave is
// joining: it words the state the owner answers while a stop waits.
//
// VALIDATES: `client` is `stopping`, and the connection the client still holds
// is reported.
// PREVENTS: a status that calls a client still connected "stopped".
func TestFleetStatusReportsStopping(t *testing.T) {
	report := reportOf(ClientState{Stopping: true, Connected: true})

	assert.Equal(t, clientReport{
		client: clientStopping, connection: connectionUp, holding: holdNo,
		heldByHub: hubAnswerNotFetched, revertOffered: hubAnswerNotFetched,
	}, report)
}

// TestFleetEnableAnswersWhatFailed proves that a rejoin whose hub client did
// not start says what failed and gives that failure's fix: the target stores
// the flag and then answers each failure a start can have.
//
// VALIDATES: the answer names the flag stored as true, carries the cause's
// fix, and does not promise a connection at the next start.
// PREVENTS: "the hub connection starts at the next ze start" on a device whose
// hub block cannot be read, which no restart repairs.
func TestFleetEnableAnswersWhatFailed(t *testing.T) {
	unreadableBlock := opererr.New(
		"the plugin hub block of this device's configuration could not be read",
		"it names the hub this device follows",
		"ze config validate")
	for name, want := range map[string]struct {
		failure error
		fix     string
	}{
		"a hub block that does not parse":         {unreadableBlock, "ze config validate"},
		"a configuration that lost its hub block": {ErrNoHubClient, hubClientMissingFix},
	} {
		t.Run(name, func(t *testing.T) {
			store := fleetOpenStore(t,
				map[string]string{zefs.KeyInstanceManaged.Pattern: "false\n"},
				map[string]string{fleetStatusConfigName: fleetStatusConfig})
			target := &FleetTarget{
				StartClient: func(storeFlag func() error) (bool, error) {
					require.NoError(t, storeFlag())
					return false, want.failure
				},
				StopClient:  stopsWith(),
				ClientState: func() ClientState { return ClientState{} },
			}

			_, refusal := enableOnDaemon(store, store.WriteKey, target)

			require.NotNil(t, refusal)
			assert.Equal(t, want.fix, refusal.Fix())
			assert.Contains(t, refusal.Why(), "the managed flag is stored as true")
			assert.NotContains(t, refusal.Error(), "next ze start")
			assert.Equal(t, "true\n", storedManagedFlag(t, store))
		})
	}
}
