// Design: docs/architecture/fleet-config.md -- the owner of the daemon's hub client: leave and rejoin with no restart.
// Related: managed_owner.go -- the owner under test.
// Related: managed_lifetime_test.go -- the same join proven through the actual daemon start and stop.
package hub

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// ownerHubAddress is the hub a stand-in client is built for. No test dials it.
const ownerHubAddress = "192.0.2.1:1791"

// runUntilCanceled is a stand-in for managed.RunManagedClient: it hands the
// configuration it was started with to started, and returns when its context
// is canceled, as the client does.
func runUntilCanceled(started chan<- managed.ClientConfig) func(context.Context, managed.ClientConfig) {
	return func(ctx context.Context, client managed.ClientConfig) {
		started <- client
		<-ctx.Done()
	}
}

// newStandInOwner answers an owner whose client is run, with no commit to
// wire. The test's cleanup ends it.
func newStandInOwner(t *testing.T, build ManagedClientBuilder, run func(context.Context, managed.ClientConfig)) *managedClientOwner {
	t.Helper()
	owner := newManagedClientOwner(build, func(*managed.ClientConfig) {})
	owner.run = run
	t.Cleanup(owner.shutdown)
	return owner
}

// awaitOwnerRun answers the configuration the next stand-in client was
// started with.
func awaitOwnerRun(t *testing.T, started <-chan managed.ClientConfig) managed.ClientConfig {
	t.Helper()
	select {
	case client := <-started:
		return client
	case <-time.After(3 * time.Second):
		t.Fatal("the owner started no hub client")
		return managed.ClientConfig{}
	}
}

// TestOwnerStopEndsLiveConnection proves that stop ends a connected client and
// joins it: the actual client connects to an actual managed server and applies
// the configuration it serves, then stop MUST return, with the connection it
// closed in its answer, and leave no client running.
//
// VALIDATES: AC-10 and A-6: canceling the client's own context ends a live
// connection promptly, and stop returns only after the client has.
// PREVENTS: a leave that stores the flag and leaves the device connected until
// its next reconnect.
func TestOwnerStopEndsLiveConnection(t *testing.T) {
	const name = "edge-01"
	const secret = "edge01-secret-that-is-at-least-32ch"
	store := newTestStore(t)
	require.NoError(t, store.WriteFile(pluginserver.ClientConfigKey(name), []byte("system {}"), 0o600))
	srv := startManagedServer(t.Context(), store, &plugin.HubConfig{Servers: []plugin.HubServerConfig{
		{Name: "local", Host: "127.0.0.1", Port: 0},
		{Name: "central", Host: "127.0.0.2", Port: 0, Clients: map[string]string{name: secret}},
	}})
	require.NotNil(t, srv)
	t.Cleanup(srv.Stop)
	require.Len(t, srv.Addrs(), 1)

	applied := make(chan struct{}, 1)
	owner := newManagedClientOwner(func() (*managed.ClientConfig, error) {
		return &managed.ClientConfig{
			Name: name, Server: srv.Addrs()[0].String(), Token: secret, TLSInsecure: true,
			Handler: &managed.Handler{Validate: func([]byte) error { return nil }},
		}, nil
	}, func(client *managed.ClientConfig) {
		client.OnCommit = func([]byte) error {
			applied <- struct{}{}
			return nil
		}
	})
	t.Cleanup(owner.shutdown)

	started, err := owner.startWith(noFlagWrite)
	require.NoError(t, err)
	require.True(t, started)
	awaitManagedLifetime(t, applied, "the hub client did not apply the configuration its hub serves")
	require.Equal(t, managed.ClientState{Running: true, Connected: true}, owner.state(),
		"a client that applied a fetched configuration is connected")

	stopped := make(chan managed.ClientState, 1)
	go func() {
		before, stopErr := owner.stopWith(noFlagWrite)
		assert.NoError(t, stopErr)
		stopped <- before
	}()
	select {
	case before := <-stopped:
		assert.Equal(t, managed.ClientState{Running: true, Connected: true}, before,
			"stop answers the state of the client it ended")
	case <-time.After(3 * time.Second):
		t.Fatal("stop did not end a live hub connection")
	}
	assert.Equal(t, managed.ClientState{}, owner.state())
}

// TestOwnerRefusesSecondRun proves that a start while a client runs starts
// nothing and says so: the second start answers false, builds no
// configuration and starts no second client.
//
// VALIDATES: R-10: the owner holds one running slot.
// PREVENTS: two hub clients under one name, which the hub refuses and which
// then reconnect against each other.
func TestOwnerRefusesSecondRun(t *testing.T) {
	builds := 0
	started := make(chan managed.ClientConfig, 2)
	owner := newStandInOwner(t, func() (*managed.ClientConfig, error) {
		builds++
		return &managed.ClientConfig{Server: ownerHubAddress}, nil
	}, runUntilCanceled(started))

	first, err := owner.startWith(noFlagWrite)
	require.NoError(t, err)
	require.True(t, first)
	awaitOwnerRun(t, started)

	second, err := owner.startWith(noFlagWrite)

	require.NoError(t, err)
	assert.False(t, second, "a start while a client runs must say it started nothing")
	assert.Equal(t, 1, builds)
	assert.Empty(t, started, "a second client was started")
	assert.True(t, owner.state().Running)
}

// TestOwnerStartAfterStop proves that a stopped client starts again on a
// configuration built for that start: the builder answers another hub address
// each time, and each client MUST run on the address of its own build.
//
// VALIDATES: AC-11, after `request fleet disable` in the same run.
// PREVENTS: a rejoin that reuses the client configuration of the previous run,
// and so the hub block the daemon read before the operator changed it.
func TestOwnerStartAfterStop(t *testing.T) {
	servers := []string{ownerHubAddress, "192.0.2.2:1791"}
	builds := 0
	started := make(chan managed.ClientConfig, 2)
	owner := newStandInOwner(t, func() (*managed.ClientConfig, error) {
		client := &managed.ClientConfig{Server: servers[builds]}
		builds++
		return client, nil
	}, runUntilCanceled(started))

	first, err := owner.startWith(noFlagWrite)
	require.NoError(t, err)
	require.True(t, first)
	assert.Equal(t, servers[0], awaitOwnerRun(t, started).Server)
	before, err := owner.stopWith(noFlagWrite)
	require.NoError(t, err)
	assert.True(t, before.Running, "stop answers the state of the client it ended")
	require.False(t, owner.state().Running)

	again, err := owner.startWith(noFlagWrite)

	require.NoError(t, err)
	assert.True(t, again)
	assert.Equal(t, servers[1], awaitOwnerRun(t, started).Server)
	assert.Equal(t, 2, builds)
}

// TestOwnerStartsWhenBootHadNoHubBlock proves that a managed start whose
// configuration named no hub still has an owner that can start a client: the
// builder answers ErrNoHubClient at boot, then a configuration, as it does
// once the active configuration has gained a hub block.
//
// VALIDATES: AC-11, after a start whose configuration named no hub, and Data
// Flow step 8: the client is built from the configuration at the moment it is
// started.
// PREVENTS: a device that must be restarted to follow a hub block added after
// its start.
func TestOwnerStartsWhenBootHadNoHubBlock(t *testing.T) {
	declared := false
	started := make(chan managed.ClientConfig, 1)
	owner := newStandInOwner(t, func() (*managed.ClientConfig, error) {
		if !declared {
			return nil, managed.ErrNoHubClient
		}
		return &managed.ClientConfig{Server: ownerHubAddress}, nil
	}, runUntilCanceled(started))

	owner.startAtBoot()
	require.Equal(t, managed.ClientState{}, owner.state(), "a boot with no hub block starts no client")
	require.Empty(t, started)
	_, err := owner.startWith(noFlagWrite)
	require.ErrorIs(t, err, managed.ErrNoHubClient, "a start with no hub block says why it started nothing")

	declared = true
	startedNow, err := owner.startWith(noFlagWrite)

	require.NoError(t, err)
	assert.True(t, startedNow)
	assert.Equal(t, ownerHubAddress, awaitOwnerRun(t, started).Server)
	assert.True(t, owner.state().Running)
}

// TestOwnerJoinedBeforeStoreCloses proves that shutdown returns only after the
// client has, and that nothing starts after it: the stand-in client ignores
// its cancellation until the test releases it, as a client inside a store
// write does.
//
// VALIDATES: R-10 and the goroutine rule: the owner is joined before the store
// is returned, and a fleet command that still held the target starts no client
// on a returned store.
// PREVENTS: a hub client that outlives the store its callbacks read.
func TestOwnerJoinedBeforeStoreCloses(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	owner := newManagedClientOwner(func() (*managed.ClientConfig, error) {
		return &managed.ClientConfig{Server: ownerHubAddress}, nil
	}, func(*managed.ClientConfig) {})
	owner.run = func(ctx context.Context, _ managed.ClientConfig) {
		defer close(finished)
		close(entered)
		<-ctx.Done()
		<-release
	}
	t.Cleanup(unblock)

	started, err := owner.startWith(noFlagWrite)
	require.NoError(t, err)
	require.True(t, started)
	awaitManagedLifetime(t, entered, "the hub client did not start")

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		owner.shutdown()
	}()
	select {
	case <-stopped:
		t.Fatal("shutdown returned while the hub client still borrowed the store")
	case <-time.After(200 * time.Millisecond):
		// Only a negative liveness bound: the client cannot end before
		// unblock, whatever the scheduling or the machine speed.
	}
	unblock()
	awaitManagedLifetime(t, stopped, "shutdown did not return after the hub client ended")
	select {
	case <-finished:
	default:
		t.Fatal("shutdown returned before the hub client's goroutine ended")
	}

	_, err = owner.startWith(noFlagWrite)
	require.ErrorIs(t, err, errManagedOwnerEnded)
	assert.Equal(t, managed.ClientState{}, owner.state())
}

// TestOwnerFlagAndClientAgree proves that a leave and a rejoin arriving
// together end with the managed flag and the client agreeing: each round
// starts a client with the flag true, then runs a stop that stores false and a
// start that stores true at the same time, and compares the flag stored last
// with the client's state.
//
// VALIDATES: the owner runs each flag write and its start or stop as one
// operation (op), so the two never interleave.
// PREVENTS: a disable's stop that runs after an enable stored true, which
// leaves the device managed, frozen and with no hub client.
func TestOwnerFlagAndClientAgree(t *testing.T) {
	const rounds = 200
	for round := range rounds {
		started := make(chan managed.ClientConfig, 2)
		owner := newStandInOwner(t, func() (*managed.ClientConfig, error) {
			return &managed.ClientConfig{Server: ownerHubAddress}, nil
		}, runUntilCanceled(started))
		_, err := owner.startWith(noFlagWrite)
		require.NoError(t, err)

		var flagMu sync.Mutex
		flag := true
		storeFlag := func(value bool) func() error {
			return func() error {
				runtime.Gosched()
				flagMu.Lock()
				flag = value
				flagMu.Unlock()
				runtime.Gosched()
				return nil
			}
		}
		var both sync.WaitGroup
		both.Go(func() {
			_, stopErr := owner.stopWith(storeFlag(false))
			assert.NoError(t, stopErr)
		})
		both.Go(func() {
			_, startErr := owner.startWith(storeFlag(true))
			assert.NoError(t, startErr)
		})
		both.Wait()

		flagMu.Lock()
		stored := flag
		flagMu.Unlock()
		require.Equal(t, stored, owner.state().Running,
			"round %d: the flag stored last is %v and the client's state disagrees", round, stored)
		owner.shutdown()
	}
}

// TestOwnerStateAnswersWhileStopWaits proves that the state of the hub client
// is answered while a stop waits for a client that does not return: the
// stand-in client keeps running after its cancel until the test releases it,
// as a client applying a configuration does.
//
// VALIDATES: state answers at once with Stopping while the stop joins, and
// the zero state once the client returned.
// PREVENTS: `show fleet status` blocked behind a `request fleet disable`
// that waits for a push to be applied.
func TestOwnerStateAnswersWhileStopWaits(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseClient := func() { releaseOnce.Do(func() { close(release) }) }
	canceled := make(chan struct{})
	owner := newStandInOwner(t, func() (*managed.ClientConfig, error) {
		return &managed.ClientConfig{Server: ownerHubAddress}, nil
	}, func(ctx context.Context, _ managed.ClientConfig) {
		<-ctx.Done()
		close(canceled)
		<-release
	})
	t.Cleanup(releaseClient)
	_, err := owner.startWith(noFlagWrite)
	require.NoError(t, err)

	stopped := make(chan managed.ClientState, 1)
	go func() {
		before, stopErr := owner.stopWith(noFlagWrite)
		assert.NoError(t, stopErr)
		stopped <- before
	}()
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("the stop did not cancel the client")
	}

	answered := make(chan managed.ClientState, 1)
	go func() { answered <- owner.state() }()
	select {
	case state := <-answered:
		assert.Equal(t, managed.ClientState{Stopping: true}, state)
	case <-time.After(3 * time.Second):
		releaseClient()
		t.Fatal("state waited for the stop's join")
	}

	releaseClient()
	select {
	case before := <-stopped:
		assert.True(t, before.Running, "stop answers the state of the client it ended")
	case <-time.After(3 * time.Second):
		t.Fatal("the stop did not return once the client returned")
	}
	assert.Equal(t, managed.ClientState{}, owner.state())
}
