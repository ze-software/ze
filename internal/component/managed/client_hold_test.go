// VALIDATES: a managed client holds. Before each fetch it compares the
// configuration its store holds as active with the baseline (the last one its
// hub gave it); while they differ it applies nothing the hub sends, tells the
// hub so and keeps its connection, and the hub's "current" answer is what
// records a new baseline.
// PREVENTS: a device that was changed locally taking its hub's configuration
// at the next connection or hub write, and losing the change without a trace.

package managed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	// holdDeviceConfig is what the device runs, and holdHubConfig what its
	// hub holds for it. Neither names a hub: these tests set no Endpoint.
	holdDeviceConfig = "device { changed locally; }\n"
	holdHubConfig    = "device { as the hub holds it; }\n"
	// holdEarlierConfig is the configuration the hub gave before either.
	holdEarlierConfig = "device { as the hub gave it earlier; }\n"
	// holdWait bounds each wait for the client goroutine or the hub.
	holdWait = 5 * time.Second
)

// holdDevice creates a device store that holds active as its configuration
// and, when earlier is not empty, the hash of earlier as its baseline.
func holdDevice(t *testing.T, active, earlier string) storage.Storage {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	require.NoError(t, store.WriteFile(fleetStatusConfigName, []byte(active), 0))
	if earlier != "" {
		require.NoError(t, StoreBaseline(store, versionOf([]byte(earlier))))
	}
	return store
}

// holdClient is a client configuration for edge-01 over access. applied
// receives each configuration the client commits, by a push or a revert; a
// test that expects none reads it empty.
func holdClient(server string, access *BaselineAccess) (cfg ClientConfig, validated *atomic.Bool, applied chan []byte) {
	validated = &atomic.Bool{}
	applied = make(chan []byte, 4)
	cfg = ClientConfig{
		Name: "edge-01", Server: server, Token: testHubToken, CA: "fleet-hub",
		Handler: &Handler{Validate: func([]byte) error {
			validated.Store(true)
			return nil
		}},
		CommitOver: stagedOver(access, func(data []byte) error {
			applied <- data
			return nil
		}),
		Baseline: access,
	}
	return cfg, validated, applied
}

// stagedOver stands for the daemon's stage (cmd/ze/hub wireManagedCommit): it
// hands check the configuration the store behind access holds as active now,
// and commits through commit only when check accepts it
// (storage.StageMachineCandidateOver).
func stagedOver(access *BaselineAccess, commit func(data []byte) error) func(data []byte, check func(active []byte) error) error {
	return func(data []byte, check func(active []byte) error) error {
		held, err := access.held()
		if err != nil {
			return err
		}
		if err := check(held.content); err != nil {
			return err
		}
		return commit(data)
	}
}

// appliedDevice is the BaselineAccess of a device store that runs
// holdEarlierConfig and records it as its baseline: a device that does not
// hold, so a push it fetches is staged and applied over its check.
func appliedDevice(t *testing.T) *BaselineAccess {
	t.Helper()
	return NewBaselineAccess(holdDevice(t, holdEarlierConfig, holdEarlierConfig))
}

// requireDeviceUnchanged asserts that the store behind access still runs
// holdEarlierConfig and records it as its baseline: nothing was applied, and
// the baseline did not move.
func requireDeviceUnchanged(t *testing.T, access *BaselineAccess) {
	t.Helper()
	active, baseline, err := access.read()
	require.NoError(t, err)
	assert.Equal(t, versionOf([]byte(holdEarlierConfig)), active, "the device's active configuration changed")
	assert.Equal(t, versionOf([]byte(holdEarlierConfig)), baseline, "the device's baseline moved")
}

// hubRefusalOf waits for the hub that was started last to record a refused
// config-ack of edge-01 and answers its text.
func hubRefusalOf(t *testing.T) string {
	t.Helper()
	var detail string
	require.Eventually(t, func() bool {
		record, err := pluginserver.ManagedDoctorSnapshot()
		if err != nil {
			return false
		}
		for _, rejection := range record.Rejected {
			if rejection.Client == "edge-01" && rejection.Cause == pluginserver.ManagedRejectConfigRejected {
				detail = rejection.Detail
				return true
			}
		}
		return false
	}, holdWait, 10*time.Millisecond, "the hub recorded no refused config-ack of edge-01")
	return detail
}

// answerHub plays the hub on conn until conn closes: each request is handed
// to the returned channel and answered with what answer returns for it.
// The caller MUST cancel ctx, close conn and wait for done before releasing
// anything answer borrows. The worker MUST return when ctx is canceled.
func answerHub(t *testing.T, ctx context.Context, conn net.Conn, answer func(*rpc.Request) any) (<-chan *rpc.Request, <-chan struct{}) {
	t.Helper()
	seen := make(chan *rpc.Request, 8)
	done := make(chan struct{})
	rc := rpc.NewConn(conn, conn)
	go func() {
		defer close(done)
		for {
			req, err := rc.ReadRequest(ctx)
			if err != nil {
				return // The test closed the pipe.
			}
			select {
			case seen <- req:
			case <-ctx.Done():
				return
			}
			if err := rc.SendResult(ctx, req.ID, answer(req)); err != nil {
				return
			}
		}
	}()
	return seen, done
}

// holdPipe connects a client end to a hub that answers with answer, and
// returns the client end and the requests the hub receives.
// Its cleanup MUST cancel the hub worker, close both pipe ends and join the
// worker before returning, so no callback outlives the test's borrowed state.
func holdPipe(t *testing.T, answer func(*rpc.Request) any) (*rpc.MuxConn, <-chan *rpc.Request) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	clientEnd, hubEnd := net.Pipe()
	seen, done := answerHub(t, ctx, hubEnd, answer)
	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	t.Cleanup(func() {
		cancel()
		_ = clientEnd.Close()
		_ = hubEnd.Close()
		_ = mc.Close()
		select {
		case <-done:
		case <-time.After(holdWait):
			t.Error("the test's hub worker did not join after its pipe closed")
		}
	})
	return mc, seen
}

// nextRequest answers the next request the hub of holdPipe received.
func nextRequest(t *testing.T, seen <-chan *rpc.Request) *rpc.Request {
	t.Helper()
	select {
	case req := <-seen:
		return req
	case <-time.After(holdWait):
		t.Fatal("the hub received no request")
		return nil
	}
}

// answerCurrent is a hub that holds whatever configuration the client names.
func answerCurrent(*rpc.Request) any {
	return fleet.ConfigFetchResponse{Status: fetchStatusCurrent}
}

// TestFetchHoldsWhenActiveDiffersFromBaseline proves the hold end to end: a
// real client over a real store connects to a real hub that holds another
// configuration than the one the device runs, and the device's configuration
// is not the last one the hub gave.
//
// VALIDATES: AC-4 and AC-18. The hub's configuration is neither validated nor
// committed, the device's configuration and baseline are untouched, and the
// client keeps running.
// PREVENTS: a device changed locally being overwritten when it connects.
//
// MUTATION: drop the Diverged branch of fetchAgainstBaseline and this fails:
// the hub's configuration reaches Validate and CommitOver.
func TestFetchHoldsWhenActiveDiffersFromBaseline(t *testing.T) {
	root := testRoot(t)
	installCA(t, root)
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	addr, fetched := startServingHub(t, root, func() []byte { return []byte(holdHubConfig) })
	cfg, validated, applied := holdClient(addr, NewBaselineAccess(store))

	done := runClient(t, cfg)

	select {
	case <-fetched:
	case <-time.After(holdWait):
		t.Fatal("the client never fetched from its hub")
	}
	// The refusal the hub records is sent after the client decided, so it is
	// the point where the fetch is over.
	hubRefusalOf(t)

	assert.False(t, validated.Load(), "a configuration the device holds against was validated")
	assert.Empty(t, applied, "a configuration the device holds against was committed")
	kept, err := store.ReadFile(fleetStatusConfigName)
	require.NoError(t, err)
	assert.Equal(t, holdDeviceConfig, string(kept), "the device's configuration changed while it held")
	baseline, err := BaselineVersion(store)
	require.NoError(t, err)
	assert.Equal(t, versionOf([]byte(holdEarlierConfig)), baseline, "a hold moved the baseline")
	select {
	case <-done:
		t.Fatal("the client stopped when it held")
	default:
	}
}

// TestHoldAcknowledgesRefusalAndKeepsConnection proves what a hold tells the
// hub and what it tells its caller: one fetch against a hub that returns a
// configuration, on a device whose configuration differs from its baseline.
//
// VALIDATES: AC-4. The config-ack is ok:false, names the hold and both
// hashes, carries the version the hub offered and no configuration content,
// and fetchAndProcess answers nil, which keeps the connection. The hub
// answers every request, the copy of the held configuration the device sends
// before its ack included (uploadHeld).
// PREVENTS: a hold that ends the connection, so the device reconnects and
// holds again about once a second, and a refusal that leaks configuration.
func TestHoldAcknowledgesRefusalAndKeepsConnection(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := holdPipe(t, hubHolding("", struct{}{}))

	err := fetchAndProcess(t.Context(), mc, &cfg)

	require.NoError(t, err, "a hold MUST keep the connection")
	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	require.Equal(t, fleet.VerbConfigHeld, nextRequest(t, seen).Method)
	ack := ackOf(t, seen)
	assert.False(t, ack.OK)
	assert.Equal(t, fleet.VersionHash([]byte(holdHubConfig)), ack.Version, "the ack names the version the hub offered")
	assert.Contains(t, ack.Error, holdRefusalPrefix)
	assert.Contains(t, ack.Error, "active "+fleet.VersionHash([]byte(holdDeviceConfig)))
	assert.Contains(t, ack.Error, "baseline "+fleet.VersionHash([]byte(holdEarlierConfig)))
	assert.NotContains(t, ack.Error, "changed locally", "the refusal carries the device's configuration")
	assert.NotContains(t, ack.Error, "as the hub holds it", "the refusal carries the hub's configuration")
}

// TestCurrentAnswerSetsBaseline proves that the hub's "current" answer is
// what records a baseline: a real client connects to a real hub that holds
// the configuration the device runs.
//
// VALIDATES: AC-6 (a device whose baseline is older stops being diverged) and
// the "current" half of AC-7 (a device with no baseline gets one).
// PREVENTS: a hold nothing can end, and a device that never gains a baseline.
func TestCurrentAnswerSetsBaseline(t *testing.T) {
	for _, state := range []struct{ name, earlier string }{
		{"the baseline is an earlier configuration", holdEarlierConfig},
		{"no baseline is recorded", ""},
	} {
		t.Run(state.name, func(t *testing.T) {
			root := testRoot(t)
			installCA(t, root)
			store := holdDevice(t, holdDeviceConfig, state.earlier)
			addr, _ := startServingHub(t, root, func() []byte { return []byte(holdDeviceConfig) })
			access := NewBaselineAccess(store)
			stored := make(chan ConfigVersion, 4)
			storeBaseline := access.store
			access.store = func(version ConfigVersion) error {
				err := storeBaseline(version)
				stored <- version
				return err
			}
			cfg, _, applied := holdClient(addr, access)

			runClient(t, cfg)

			select {
			case <-stored:
			case <-time.After(holdWait):
				t.Fatal("a current answer stored no baseline")
			}
			baseline, err := BaselineVersion(store)
			require.NoError(t, err)
			assert.Equal(t, versionOf([]byte(holdDeviceConfig)), baseline)
			assert.Empty(t, applied, "a current answer committed a configuration")
		})
	}
}

// TestHoldEndedOnCurrentTellsTheHub proves what the hub hears when a hold
// ends: one fetch, on a device whose configuration differs from its baseline,
// against a hub that holds that same configuration.
//
// VALIDATES: AC-9. After the "current" answer the device sends a config-ack
// with ok:true naming the configuration it runs, which is what makes the hub
// drop the refusal it recorded while the device held.
// PREVENTS: a hub whose doctor goes on listing a device as holding after the
// operator gave the hub that device's configuration.
func TestHoldEndedOnCurrentTellsTheHub(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := holdPipe(t, answerCurrent)

	require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))

	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	told := nextRequest(t, seen)
	require.Equal(t, fleet.VerbConfigAck, told.Method)
	var ack fleet.ConfigAck
	require.NoError(t, json.Unmarshal(told.Params, &ack))
	assert.True(t, ack.OK)
	assert.Equal(t, fleet.VersionHash([]byte(holdDeviceConfig)), ack.Version)
	assert.Empty(t, ack.Error)
}

// TestFirstBaselineSendsNoAck proves that recording a first baseline tells
// the hub nothing: one fetch, on a device with no baseline, against a hub that
// holds the configuration the device runs.
//
// VALIDATES: the only request the hub receives is the fetch. The device never
// refused anything, so there is no refusal for an ack to clear.
// PREVENTS: every connection of an in-sync device logging "applied config" on
// the hub for a configuration nobody applied.
func TestFirstBaselineSendsNoAck(t *testing.T) {
	store := holdDevice(t, holdDeviceConfig, "")
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := holdPipe(t, answerCurrent)

	require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))

	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	assert.Empty(t, seen, "the device sent the hub a second request")
}

// TestNoBaselineAppliesAsToday proves that a device with no baseline is
// clean: a real client whose store records none connects to a real hub that
// holds another configuration.
//
// VALIDATES: AC-7 and D-6. The hub's configuration is validated and committed
// as before baselines existed.
// PREVENTS: every device provisioned before this change holding at its first
// hub change.
func TestNoBaselineAppliesAsToday(t *testing.T) {
	root := testRoot(t)
	installCA(t, root)
	store := holdDevice(t, holdDeviceConfig, "")
	addr, _ := startServingHub(t, root, func() []byte { return []byte(holdHubConfig) })
	cfg, validated, applied := holdClient(addr, NewBaselineAccess(store))

	runClient(t, cfg)

	select {
	case data := <-applied:
		assert.Equal(t, holdHubConfig, string(data))
	case <-time.After(holdWait):
		t.Fatal("a device with no baseline did not apply its hub's configuration")
	}
	assert.True(t, validated.Load())
}

// TestFetchSendsActiveHash proves which version a fetch names: two fetches
// over one connection, with the device's configuration replaced in the store
// between them.
//
// VALIDATES: the version a fetch sends is the hash of the active
// configuration, read from the store at fetch time.
// PREVENTS: a client that names the configuration it read at start, so a hub
// that holds the device's present configuration never answers "current" and
// a hold never ends.
func TestFetchSendsActiveHash(t *testing.T) {
	store := holdDevice(t, holdEarlierConfig, "")
	cfg, _, _ := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	mc, seen := holdPipe(t, answerCurrent)

	require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))
	assert.Equal(t, fleet.VersionHash([]byte(holdEarlierConfig)), sentVersion(t, nextRequest(t, seen)))

	require.NoError(t, store.WriteFile(fleetStatusConfigName, []byte(holdDeviceConfig), 0))
	require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))
	assert.Equal(t, fleet.VersionHash([]byte(holdDeviceConfig)), sentVersion(t, nextRequest(t, seen)))
}

// sentVersion is the version a config-fetch request names.
func sentVersion(t *testing.T, req *rpc.Request) string {
	t.Helper()
	require.Equal(t, fleet.VerbConfigFetch, req.Method)
	var fetch fleet.ConfigFetchRequest
	require.NoError(t, json.Unmarshal(req.Params, &fetch))
	return fetch.Version
}

// TestUnreadableBaselineAppliesNothingAndTellsTheHub proves the direction a
// client takes when its store cannot say what the baseline is: one fetch with
// a baseline read that fails.
//
// VALIDATES: nothing is fetched or applied, the hub receives a config-ack
// with ok:false that says why, and the connection is kept.
// PREVENTS: a store failure read as "no baseline", which would apply the
// hub's configuration over a local change, and a device that stops following
// its hub with nothing to show for it.
func TestUnreadableBaselineAppliesNothingAndTellsTheHub(t *testing.T) {
	unreadable := errors.New("the key did not decode")
	access := &BaselineAccess{
		active:   func() (ConfigVersion, error) { return versionOf([]byte(holdDeviceConfig)), nil },
		baseline: func() (ConfigVersion, error) { return ConfigVersion{}, unreadable },
		store:    func(ConfigVersion) error { return nil },
	}
	cfg, validated, applied := holdClient("192.0.2.1:1791", access)
	mc, seen := holdPipe(t, func(*rpc.Request) any {
		return fleet.ConfigFetchResponse{
			Version: fleet.VersionHash([]byte(holdHubConfig)),
			Config:  base64.StdEncoding.EncodeToString([]byte(holdHubConfig)),
		}
	})

	require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))

	req := nextRequest(t, seen)
	require.Equal(t, fleet.VerbConfigAck, req.Method, "the client fetched with a baseline it could not read")
	var ack fleet.ConfigAck
	require.NoError(t, json.Unmarshal(req.Params, &ack))
	assert.False(t, ack.OK)
	assert.Contains(t, ack.Error, "could not compare")
	assert.Contains(t, ack.Error, unreadable.Error())
	assert.False(t, validated.Load())
	assert.Empty(t, applied)
}

// TestCurrentAnswerKeepsConnectionWhenBaselineCannotBeStored proves that a
// baseline write that fails after a "current" answer ends nothing: one fetch
// with a store that refuses the write.
//
// VALIDATES: fetchAndProcess answers nil, so the connection is kept and the
// next fetch settles the baseline again.
// PREVENTS: a store write failure turning into a reconnect loop.
func TestCurrentAnswerKeepsConnectionWhenBaselineCannotBeStored(t *testing.T) {
	attempts := 0
	access := &BaselineAccess{
		active:   func() (ConfigVersion, error) { return versionOf([]byte(holdDeviceConfig)), nil },
		baseline: func() (ConfigVersion, error) { return ConfigVersion{}, nil },
		store: func(ConfigVersion) error {
			attempts++
			return errors.New("the store is read-only")
		},
	}
	cfg, _, _ := holdClient("192.0.2.1:1791", access)
	mc, _ := holdPipe(t, answerCurrent)

	require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))
	require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))

	assert.Equal(t, 2, attempts, "each current answer tries to record the baseline")
}
