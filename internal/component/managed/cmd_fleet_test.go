package managed

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	_ "github.com/ze-software/ze/internal/component/hub/yang"    // the environment root the plugin schema is read beside
	_ "github.com/ze-software/ze/internal/component/plugin/yang" // the plugin hub block DeclaredEndpoint reads
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fleetStatusConfig is a device configuration that declares a hub. It is the
// shape cmd/ze's start-path tests read with the same first pass.
const fleetStatusConfig = `plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-plugin-secret-of-32-characters";
        }
        client edge-01 {
            host 10.0.0.1;
            port 1791;
            secret "edge-01-secret-of-at-least-32-chars";
        }
    }
}
`

// fleetStatusConfigName is the name a store with no instance name gives its
// own configuration (resolve.DefaultConfigName).
const fleetStatusConfigName = "ze.conf"

// errNoDaemon is what a test's ask answers when no daemon runs.
var errNoDaemon = errors.New("no daemon listens")

// fleetStatusStore creates a store in a directory of the test's own, writes
// keys and files into it, closes it, and points ze.config.dir at it, so the
// handler opens the store the way a `ze` process does. Tests using it MUST NOT
// call t.Parallel: ze.config.dir is process state.
func fleetStatusStore(t *testing.T, keys, files map[string]string) {
	t.Helper()
	folder := t.TempDir()
	store, err := storage.Create(folder)
	require.NoError(t, err)
	for key, value := range keys {
		require.NoError(t, store.WriteKey(key, []byte(value)))
	}
	for name, content := range files {
		require.NoError(t, store.WriteFile(name, []byte(content), 0))
	}
	require.NoError(t, store.Close())

	previous := env.Get("ze.config.dir")
	require.NoError(t, env.Set("ze.config.dir", folder))
	t.Cleanup(func() {
		if err := env.Set("ze.config.dir", previous); err != nil {
			t.Errorf("restore ze.config.dir: %v", err)
		}
	})
}

// fleetStatusOn runs the handler on one route over ask and answers the status
// it gave. The answer MUST be a status with exit code 0.
func fleetStatusOn(t *testing.T, answer func(command.LocalDataHandler, command.ValidatedArgs) command.LocalAnswer, ask daemonAsk) fleetStatus {
	t.Helper()
	validated, err := command.ValidateArgs(nil, nil, nil)
	require.NoError(t, err)
	got := answer(func(args command.ValidatedArgs) command.LocalAnswer {
		return answerFleetStatusWith(args, ask)
	}, validated)
	require.Equal(t, 0, got.Code, "diagnostic: %s", got.Diagnostic)
	status, isStatus := got.Payload.(fleetStatus)
	require.True(t, isStatus, "payload is %T, want a fleet status", got.Payload)
	return status
}

// TestFleetStatusStoreOnly proves that `show fleet status` with no daemon
// answers the store facts: it runs the handler on the local route with an ask
// that fails, over stores in each state the answer tells apart.
//
// VALIDATES: AC-15 (the store fields, and the client and connection fields
// saying no daemon answered) and AC-22 (a device that is not managed reports
// it).
// PREVENTS: a status that reads an absent baseline as a hash, reports a
// device with no baseline as diverged, or shows a stopped daemon as a stopped
// client.
func TestFleetStatusStoreOnly(t *testing.T) {
	noDaemon := func() (fleetStatus, error) { return fleetStatus{}, errNoDaemon }
	activeHash := fleet.VersionHash([]byte(fleetStatusConfig))

	t.Run("a store nobody provisioned is not managed", func(t *testing.T) {
		fleetStatusStore(t, nil, nil)
		got := fleetStatusOn(t, command.AnswerLocal, noDaemon)
		assert.Equal(t, fleetStatus{
			Client: runtimeNoDaemon, Connection: runtimeNoDaemon, Holding: runtimeNoDaemon,
		}, got)
	})

	t.Run("managed with a hub block and no baseline is not diverged", func(t *testing.T) {
		fleetStatusStore(t,
			map[string]string{zefs.KeyInstanceManaged.Pattern: "true\n"},
			map[string]string{fleetStatusConfigName: fleetStatusConfig})
		got := fleetStatusOn(t, command.AnswerLocal, noDaemon)
		assert.Equal(t, fleetStatus{
			Managed: true, Frozen: true,
			HubDeclared: true, HubName: "edge-01", HubAddress: "10.0.0.1:1791",
			ActiveRecorded: true, ActiveHash: activeHash,
			Client: runtimeNoDaemon, Connection: runtimeNoDaemon, Holding: runtimeNoDaemon,
		}, got)
	})

	t.Run("a baseline equal to the active hash is in sync", func(t *testing.T) {
		fleetStatusStore(t,
			map[string]string{
				zefs.KeyInstanceManaged.Pattern:    "true",
				zefs.KeyManagedBaseVersion.Pattern: activeHash,
			},
			map[string]string{fleetStatusConfigName: fleetStatusConfig})
		got := fleetStatusOn(t, command.AnswerLocal, noDaemon)
		assert.True(t, got.BaselineRecorded)
		assert.Equal(t, activeHash, got.BaselineHash)
		assert.Equal(t, activeHash, got.ActiveHash)
		assert.False(t, got.Diverged)
	})

	t.Run("a baseline other than the active hash is diverged", func(t *testing.T) {
		baseline := fleet.VersionHash([]byte("the configuration the hub gave"))
		fleetStatusStore(t,
			map[string]string{
				zefs.KeyInstanceManaged.Pattern:    "true",
				zefs.KeyManagedBaseVersion.Pattern: baseline,
			},
			map[string]string{fleetStatusConfigName: fleetStatusConfig})
		got := fleetStatusOn(t, command.AnswerLocal, noDaemon)
		assert.Equal(t, baseline, got.BaselineHash)
		assert.Equal(t, activeHash, got.ActiveHash)
		assert.True(t, got.Diverged)
	})

	t.Run("a flag set to false is not managed and not frozen", func(t *testing.T) {
		fleetStatusStore(t,
			map[string]string{zefs.KeyInstanceManaged.Pattern: "false"},
			map[string]string{fleetStatusConfigName: fleetStatusConfig})
		got := fleetStatusOn(t, command.AnswerLocal, noDaemon)
		assert.False(t, got.Managed)
		assert.False(t, got.Frozen)
		assert.True(t, got.HubDeclared)
	})
}

// TestFleetStatusLocalRouteDrawsDaemonAnswer proves that the local route asks
// a running daemon first and answers what it said: the ask answers a status
// the store does not hold, and ze.config.dir names a folder with no store, so
// an answer read from the store would be a refusal.
//
// VALIDATES: AC-15, "the local command and every daemon surface give the same
// fields": with a daemon running the local answer IS the daemon's.
// PREVENTS: the local route answering from the store beside a running daemon,
// which would hide the state only the daemon holds.
func TestFleetStatusLocalRouteDrawsDaemonAnswer(t *testing.T) {
	previous := env.Get("ze.config.dir")
	require.NoError(t, env.Set("ze.config.dir", t.TempDir()))
	t.Cleanup(func() {
		if err := env.Set("ze.config.dir", previous); err != nil {
			t.Errorf("restore ze.config.dir: %v", err)
		}
	})
	fromDaemon := fleetStatus{
		Managed: true, Frozen: true, DaemonAnswered: true,
		Client: clientRunning, Connection: connectionUp, Holding: holdNo,
	}
	got := fleetStatusOn(t, command.AnswerLocal, func() (fleetStatus, error) { return fromDaemon, nil })
	assert.Equal(t, fromDaemon, got)
}

// TestFleetStatusDaemonRouteAsksNobody proves that the handler, run by the
// daemon for a caller in another process, answers from its store and marks the
// answer as a daemon's: the ask fails the test when it is called. No fleet
// target is installed, which is a daemon that started unmanaged.
//
// VALIDATES: AC-15 on the daemon route: the same store fields, and
// daemon-answered set, which is what the local route recognizes; AC-12, a
// daemon that started unmanaged shows managed with the client not started.
// PREVENTS: a daemon that asks itself over SSH for its own status.
func TestFleetStatusDaemonRouteAsksNobody(t *testing.T) {
	fleetStatusStore(t,
		map[string]string{zefs.KeyInstanceManaged.Pattern: "true"},
		map[string]string{fleetStatusConfigName: fleetStatusConfig})
	got := fleetStatusOn(t, command.AnswerRemote, func() (fleetStatus, error) {
		t.Error("the daemon route asked a daemon")
		return fleetStatus{}, errNoDaemon
	})
	assert.True(t, got.Managed)
	assert.True(t, got.DaemonAnswered)
	assert.Equal(t, clientNotStarted, got.Client)
	assert.Equal(t, connectionDown, got.Connection)
	assert.Equal(t, holdNo, got.Holding)
}

// TestFleetStatusDaemonRouteReportsClientState proves that a daemon that
// started managed fills the client, connection and holding fields from the
// owner of its hub client: it installs a target whose state the test sets and
// runs the handler on the daemon route for each state the answer tells apart.
//
// VALIDATES: AC-15, "from a running daemon the client state, the connection
// state and whether a fetch is being held".
// PREVENTS: a daemon's status that reports a stopped client as running, or
// never reads the owner at all.
func TestFleetStatusDaemonRouteReportsClientState(t *testing.T) {
	fleetStatusStore(t,
		map[string]string{zefs.KeyInstanceManaged.Pattern: "true"},
		map[string]string{fleetStatusConfigName: fleetStatusConfig})
	var state ClientState
	installFleetTarget(t, &FleetTarget{
		StartClient: func(func() error) (bool, error) { return false, nil },
		StopClient:  stopsWith(ClientState{}),
		ClientState: func() ClientState { return state },
	})
	daemonAsksNobody := func() (fleetStatus, error) {
		t.Error("the daemon route asked a daemon")
		return fleetStatus{}, errNoDaemon
	}
	for name, want := range map[string]struct {
		state                       ClientState
		client, connection, holding string
	}{
		"no client runs":           {ClientState{}, clientStopped, connectionDown, holdNo},
		"running, not connected":   {ClientState{Running: true}, clientRunning, connectionDown, holdNo},
		"connected":                {ClientState{Running: true, Connected: true}, clientRunning, connectionUp, holdNo},
		"connected, holding":       {ClientState{Running: true, Connected: true, Holding: true}, clientRunning, connectionUp, holdYes},
		"reconnecting, still held": {ClientState{Running: true, Holding: true}, clientRunning, connectionDown, holdYes},
	} {
		t.Run(name, func(t *testing.T) {
			state = want.state
			got := fleetStatusOn(t, command.AnswerRemote, daemonAsksNobody)
			assert.True(t, got.DaemonAnswered)
			assert.Equal(t, want.client, got.Client)
			assert.Equal(t, want.connection, got.Connection)
			assert.Equal(t, want.holding, got.Holding)
		})
	}
}

// TestFleetStatusRefusesWithoutStore proves that a host with no store gets a
// refusal and no status: ze.config.dir names an empty folder and no daemon
// answers.
//
// VALIDATES: "code that cannot answer refuses": no store means nobody looked.
// PREVENTS: an unmanaged, in-sync status invented for a host with no store.
func TestFleetStatusRefusesWithoutStore(t *testing.T) {
	previous := env.Get("ze.config.dir")
	require.NoError(t, env.Set("ze.config.dir", t.TempDir()))
	t.Cleanup(func() {
		if err := env.Set("ze.config.dir", previous); err != nil {
			t.Errorf("restore ze.config.dir: %v", err)
		}
	})
	validated, err := command.ValidateArgs(nil, nil, nil)
	require.NoError(t, err)
	got := command.AnswerLocal(func(args command.ValidatedArgs) command.LocalAnswer {
		return answerFleetStatusWith(args, func() (fleetStatus, error) { return fleetStatus{}, errNoDaemon })
	}, validated)
	assert.NotEqual(t, 0, got.Code)
	assert.Nil(t, got.Payload)
	assert.NotEmpty(t, got.Diagnostic)
}

// TestDecodeDaemonFleetStatusRefusesAnotherDocument proves that only a
// document a daemon marked as its own is drawn: it decodes JSON of another
// shape, a store-only status and text that is not JSON.
//
// VALIDATES: the guard on daemon-answered in decodeDaemonFleetStatus.
// PREVENTS: an unrelated answer drawn as an unmanaged device.
func TestDecodeDaemonFleetStatusRefusesAnotherDocument(t *testing.T) {
	for name, output := range map[string]string{
		"another document":    `{"status":"ok"}`,
		"a store-only status": `{"managed":true,"daemon-answered":false,"client":"no-daemon"}`,
		"not JSON":            "show fleet status: unknown command",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := decodeDaemonFleetStatus(output)
			assert.Error(t, err)
		})
	}
	status, err := decodeDaemonFleetStatus(`{"managed":true,"daemon-answered":true,"client":"running"}`)
	require.NoError(t, err)
	assert.True(t, status.Managed)
	assert.Equal(t, clientRunning, status.Client)
}

// TestManagedFlagAbsentIsNotManaged proves the one reader of the managed flag
// tells its three answers apart: an absent key, the value true, another value.
//
// VALIDATES: the contract checkHubClientDeclared, the start path and the
// status share (ManagedFlag).
// PREVENTS: an absent flag read as an error, or white space around the value
// read as "not managed".
func TestManagedFlagAbsentIsNotManaged(t *testing.T) {
	for name, want := range map[string]struct {
		keys    map[string]string
		managed bool
	}{
		"absent":           {nil, false},
		"true with a line": {map[string]string{zefs.KeyInstanceManaged.Pattern: "true\n"}, true},
		"false":            {map[string]string{zefs.KeyInstanceManaged.Pattern: "false"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			store, err := storage.Create(t.TempDir())
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, store.Close()) })
			for key, value := range want.keys {
				require.NoError(t, store.WriteKey(key, []byte(value)))
			}
			got, err := ManagedFlag(store)
			require.NoError(t, err)
			assert.Equal(t, want.managed, got)
		})
	}
}

// fleetConfigNoHub is a device configuration that declares no hub: the plugin
// hub block holds the local server and no client block.
const fleetConfigNoHub = `plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-plugin-secret-of-32-characters";
        }
    }
}
`

// installFleetTarget publishes target for the length of the test. Tests using
// it MUST NOT call t.Parallel: the target is process state.
func installFleetTarget(t *testing.T, target *FleetTarget) {
	t.Helper()
	InstallFleetTarget(target)
	t.Cleanup(func() { InstallFleetTarget(nil) })
}

// fleetOpenStore creates a store holding keys and files and keeps it open for
// the length of the test, as a daemon keeps its own handle.
func fleetOpenStore(t *testing.T, keys, files map[string]string) storage.Storage {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	for key, value := range keys {
		require.NoError(t, store.WriteKey(key, []byte(value)))
	}
	for name, content := range files {
		require.NoError(t, store.WriteFile(name, []byte(content), 0))
	}
	return store
}

// storedManagedFlag answers the bytes the store holds under the managed flag.
func storedManagedFlag(t *testing.T, store storage.Storage) string {
	t.Helper()
	flag, err := store.ReadFile(zefs.KeyInstanceManaged.Pattern)
	require.NoError(t, err)
	return string(flag)
}

// neverStarts is a StartClient that fails the test: a leave starts no client.
func neverStarts(t *testing.T) func(func() error) (bool, error) {
	return func(func() error) (bool, error) {
		t.Error("the command started a hub client")
		return false, nil
	}
}

// stopsWith is a StopClient as the owner keeps its contract: it runs the flag
// write first, answers its error and stops nothing when it fails, and
// otherwise answers before as the state of the client it stopped.
func stopsWith(before ClientState) func(func() error) (ClientState, error) {
	return func(storeFlag func() error) (ClientState, error) {
		if err := storeFlag(); err != nil {
			return ClientState{}, err
		}
		return before, nil
	}
}

// TestFleetDisableStoresFlagThenStops proves what a leave hands a daemon that
// started managed: the target's stop runs the flag write it is handed, as the
// owner does first, then reads the managed flag, which MUST then be false.
//
// VALIDATES: AC-10 and Data Flow step 7: the flag is stored as false, then the
// client is stopped, and the answer says the connection was closed.
// PREVENTS: a client stopped while the store still says managed, so a daemon
// that stopped between the two acts would start managed again.
func TestFleetDisableStoresFlagThenStops(t *testing.T) {
	store := fleetOpenStore(t,
		map[string]string{zefs.KeyInstanceManaged.Pattern: "true\n"},
		map[string]string{fleetStatusConfigName: fleetStatusConfig})
	stops := 0
	managedAtStop := true
	target := &FleetTarget{
		StartClient: neverStarts(t),
		StopClient: func(storeFlag func() error) (ClientState, error) {
			require.NoError(t, storeFlag())
			stops++
			flag, err := ManagedFlag(store)
			require.NoError(t, err)
			managedAtStop = flag
			return ClientState{Running: true, Connected: true}, nil
		},
		ClientState: func() ClientState { return ClientState{} },
	}

	left, refusal := disableOnDaemon(store.WriteKey, target)

	require.Nil(t, refusal)
	assert.Equal(t, 1, stops)
	assert.False(t, managedAtStop, "the hub client was stopped while the store still said managed")
	assert.Equal(t, fleetChange{Client: clientStopped, Connection: connectionDown, Note: noteLeftClosed}, left)
	assert.Equal(t, "false\n", storedManagedFlag(t, store))
}

// TestFleetDisableKeepsClientWhenFlagIsNotStored proves that a leave whose
// flag write fails stops nothing: the write answers an error, and the target's
// stop fails the test when it is called.
//
// VALIDATES: the refusal of storeManagedFlag, with its fix, and the order of
// disableOnDaemon from its other side.
// PREVENTS: a device whose hub connection was closed while its store still
// says managed, which reconnects at the next start as if nobody had asked.
func TestFleetDisableKeepsClientWhenFlagIsNotStored(t *testing.T) {
	target := &FleetTarget{
		StartClient: neverStarts(t),
		StopClient: func(storeFlag func() error) (ClientState, error) {
			if err := storeFlag(); err != nil {
				return ClientState{}, err
			}
			t.Error("the hub client was stopped though the flag was not stored")
			return ClientState{}, nil
		},
		ClientState: func() ClientState { return ClientState{} },
	}
	refused := func(string, []byte) error { return errors.New("the store is read-only") }

	_, refusal := disableOnDaemon(refused, target)

	require.NotNil(t, refusal)
	assert.Contains(t, refusal.Problem(), pathFleetDisable)
	assert.Equal(t, fixCheckStore, refusal.Fix())
}

// TestFleetDisableNeedsNoHub proves that the exit works on a device with no
// hub, no connection and no client: the configuration names no hub, and the
// leave runs once against an owner with no client and once in a daemon that
// started unmanaged.
//
// VALIDATES: D-3 and the survivor of AC-13: `request fleet disable` on a
// device whose configuration names no hub succeeds.
// PREVENTS: a managed device with no hub block that can never be unfrozen.
func TestFleetDisableNeedsNoHub(t *testing.T) {
	for name, want := range map[string]struct {
		target *FleetTarget
		client string
	}{
		"a daemon that started managed and runs no client": {
			&FleetTarget{
				StartClient: neverStarts(t),
				StopClient:  stopsWith(ClientState{}),
				ClientState: func() ClientState { return ClientState{} },
			}, clientStopped,
		},
		"a daemon that started unmanaged": {nil, clientNotStarted},
	} {
		t.Run(name, func(t *testing.T) {
			store := fleetOpenStore(t,
				map[string]string{zefs.KeyInstanceManaged.Pattern: "true\n"},
				map[string]string{fleetStatusConfigName: fleetConfigNoHub})

			left, refusal := disableOnDaemon(store.WriteKey, want.target)

			require.Nil(t, refusal)
			assert.Equal(t, fleetChange{Client: want.client, Connection: connectionDown, Note: noteLeftNoClient}, left)
			assert.Equal(t, "false\n", storedManagedFlag(t, store))
		})
	}
}

// TestFleetEnableRefusesWithoutHubBlock proves that a rejoin is refused, and
// changes nothing, while the active configuration names no hub: the store
// holds a configuration with no hub client block, and the target's start fails
// the test when it is called.
//
// VALIDATES: AC-13: an operator error whose fix is the one
// doctor-hub-client-missing names, and the flag left as it was.
// PREVENTS: a device marked managed, and so frozen, with no hub that could
// ever change it.
func TestFleetEnableRefusesWithoutHubBlock(t *testing.T) {
	store := fleetOpenStore(t,
		map[string]string{zefs.KeyInstanceManaged.Pattern: "false\n"},
		map[string]string{fleetStatusConfigName: fleetConfigNoHub})
	target := &FleetTarget{
		StartClient: neverStarts(t),
		StopClient:  stopsWith(ClientState{}),
		ClientState: func() ClientState { return ClientState{} },
	}

	_, refusal := enableOnDaemon(store, store.WriteKey, target)

	require.NotNil(t, refusal)
	assert.Contains(t, refusal.Problem(), pathFleetEnable)
	assert.Equal(t, hubClientMissingFix, refusal.Fix())
	assert.Equal(t, "false\n", storedManagedFlag(t, store))
}

// TestFleetEnableOnUnmanagedStart proves that a rejoin in a daemon that
// started unmanaged stores the flag and says when the connection starts: the
// configuration names a hub and no fleet target exists.
//
// VALIDATES: AC-12 and D-7: the flag is true and the answer states that the
// hub connection starts at the next `ze start`.
// PREVENTS: an answer that reads as a live rejoin on a daemon that owns no hub
// client.
func TestFleetEnableOnUnmanagedStart(t *testing.T) {
	store := fleetOpenStore(t,
		map[string]string{zefs.KeyInstanceManaged.Pattern: "false\n"},
		map[string]string{fleetStatusConfigName: fleetStatusConfig})

	joined, refusal := enableOnDaemon(store, store.WriteKey, nil)

	require.Nil(t, refusal)
	assert.Equal(t, fleetChange{
		Managed: true, Client: clientNotStarted, Connection: connectionDown, Note: noteJoinsAtStart,
	}, joined)
	assert.Contains(t, joined.Note, "next ze start")
	assert.Equal(t, "true\n", storedManagedFlag(t, store))
}

// TestFleetEnableStartsClient proves that a rejoin in a daemon that started
// managed stores the flag as true and then starts the hub client: the
// target's start reads the flag when it is called. A second case answers that
// a client already runs.
//
// VALIDATES: AC-11 and Data Flow step 8: flag first, then the owner is asked
// to build and start a client.
// PREVENTS: a client started while the store says unmanaged, which the client
// reads before its first connection and ends on.
func TestFleetEnableStartsClient(t *testing.T) {
	for name, want := range map[string]struct {
		started bool
		note    string
	}{
		"no client ran":         {true, noteJoinedStarted},
		"a client already runs": {false, noteJoinedRunning},
	} {
		t.Run(name, func(t *testing.T) {
			store := fleetOpenStore(t,
				map[string]string{zefs.KeyInstanceManaged.Pattern: "false\n"},
				map[string]string{fleetStatusConfigName: fleetStatusConfig})
			managedAtStart := false
			target := &FleetTarget{
				StartClient: func(storeFlag func() error) (bool, error) {
					require.NoError(t, storeFlag())
					flag, err := ManagedFlag(store)
					require.NoError(t, err)
					managedAtStart = flag
					return want.started, nil
				},
				StopClient:  stopsWith(ClientState{}),
				ClientState: func() ClientState { return ClientState{Running: true} },
			}

			joined, refusal := enableOnDaemon(store, store.WriteKey, target)

			require.Nil(t, refusal)
			assert.True(t, managedAtStart, "the hub client was started while the store still said unmanaged")
			assert.Equal(t, fleetChange{
				Managed: true, Client: clientRunning, Connection: connectionDown, Note: want.note,
			}, joined)
		})
	}
}

// TestFleetLocalRouteStoresFlagWithoutDaemon proves that the local route, with
// no daemon holding the store, writes the flag itself: it runs each command
// over a closed store, with an ask that fails the test when it is called, and
// reads the flag back.
//
// VALIDATES: AC-14: with the daemon stopped the local route stores the flag
// and answers.
// PREVENTS: a leave or rejoin that needs a running daemon, on a device whose
// daemon is the thing the operator stopped.
func TestFleetLocalRouteStoresFlagWithoutDaemon(t *testing.T) {
	noDaemonAsked := func(string, string) ([]byte, error) {
		t.Error("the local route asked a daemon though the store was free")
		return nil, errNoDaemon
	}
	for name, want := range map[string]struct {
		path    string
		change  stoppedChange
		before  string
		managed bool
		note    string
	}{
		"disable": {pathFleetDisable, disableStopped, "true\n", false, noteLeftStopped},
		"enable":  {pathFleetEnable, enableStopped, "false\n", true, noteJoinsStopped},
	} {
		t.Run(name, func(t *testing.T) {
			fleetStatusStore(t,
				map[string]string{zefs.KeyInstanceManaged.Pattern: want.before},
				map[string]string{fleetStatusConfigName: fleetStatusConfig})
			validated, err := command.ValidateArgs(nil, nil, nil)
			require.NoError(t, err)

			got := changeFleetLocal(want.path, validated, noDaemonAsked, want.change)

			require.Equal(t, 0, got.Code, "diagnostic: %s", got.Diagnostic)
			assert.Equal(t, fleetChange{
				Managed: want.managed, Client: runtimeNoDaemon, Connection: runtimeNoDaemon, Note: want.note,
			}, got.Payload)
			folder := env.Get("ze.config.dir")
			store, err := storage.OpenReadOnly(folder, folder)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, store.Close()) })
			managed, err := ManagedFlag(store)
			require.NoError(t, err)
			assert.Equal(t, want.managed, managed)
		})
	}
}

// TestFleetLocalRouteSendsBusyStoreToDaemon proves that the local route, while
// another handle owns the store, sends the command line to the daemon and
// answers what it answered: the test holds the store open as a daemon does,
// and its ask records the line and answers a daemon's document.
//
// VALIDATES: the survivor of AC-14: with a daemon holding the store the same
// line is sent to that daemon.
// PREVENTS: a local write refused as busy with nothing done, or a flag
// written under a running daemon whose hub client stays connected.
func TestFleetLocalRouteSendsBusyStoreToDaemon(t *testing.T) {
	folder := t.TempDir()
	owner, err := storage.Create(folder)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, owner.Close()) })
	require.NoError(t, owner.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true\n")))
	previous := env.Get("ze.config.dir")
	require.NoError(t, env.Set("ze.config.dir", folder))
	t.Cleanup(func() {
		if err := env.Set("ze.config.dir", previous); err != nil {
			t.Errorf("restore ze.config.dir: %v", err)
		}
	})
	var sentFolder, sentLine string
	daemon := func(askedFolder, askedLine string) ([]byte, error) {
		sentFolder, sentLine = askedFolder, askedLine
		return []byte(`{"managed":false,"client":"stopped","connection":"not-connected","note":"` + noteLeftClosed + `"}`), nil
	}
	validated, err := command.ValidateArgs(nil, nil, nil)
	require.NoError(t, err)

	got := changeFleetLocal(pathFleetDisable, validated, daemon, func(storage.Storage) (fleetChange, *opererr.Error) {
		t.Error("the local route wrote a store a daemon owns")
		return fleetChange{}, nil
	})

	require.Equal(t, 0, got.Code, "diagnostic: %s", got.Diagnostic)
	assert.Equal(t, folder, sentFolder)
	assert.Equal(t, pathFleetDisable, sentLine)
	assert.Equal(t, fleetChange{Client: clientStopped, Connection: connectionDown, Note: noteLeftClosed}, got.Payload)
	assert.Equal(t, "true\n", storedManagedFlag(t, owner), "the test's daemon changed nothing, so the local route must not have either")
}

// TestFleetCommandTakesNoArgument proves that a leave typed with a word after
// it is refused: the model declares no leaf for either command, so the
// validator passes the word through and the handler is the one that refuses.
//
// VALIDATES: noArguments, with the command itself as the fix.
// PREVENTS: `request fleet disable <anything>` read as a leave.
func TestFleetCommandTakesNoArgument(t *testing.T) {
	validated, err := command.ValidateArgs([]string{"edge-01"}, nil, nil)
	require.NoError(t, err)

	refusal := noArguments(pathFleetDisable, validated)

	require.NotNil(t, refusal)
	assert.Equal(t, pathFleetDisable, refusal.Fix())
}
