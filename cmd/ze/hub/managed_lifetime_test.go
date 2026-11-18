// Design: docs/architecture/fleet-config.md -- managed borrowers end before the daemon returns.
// Related: main.go -- actual managed worker ownership and daemon teardown.
package hub

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/fleet"
)

// managedHeldRead holds exactly the named managed configuration read. All other
// startup storage operations retain the real store's behavior.
type managedHeldRead struct {
	storage.Storage
	name     string
	entered  chan struct{}
	release  <-chan struct{}
	finished chan struct{}
}

func (s *managedHeldRead) ReadFile(name string) ([]byte, error) {
	if name != s.name {
		return s.Storage.ReadFile(name)
	}
	close(s.entered)
	defer close(s.finished)
	<-s.release
	return s.Storage.ReadFile(name)
}

// TestHubManagedClientJoinsStoredRead holds the actual outbound runner inside
// CheckManaged's store read, then accepts a real stop or restart. The daemon
// must join that borrower and preserve the requested clean exit after release.
func TestHubManagedClientJoinsStoredRead(t *testing.T) {
	for _, test := range []struct {
		command string
		exit    int
	}{{command: "request shutdown", exit: 0}, {command: "request restart", exit: 75}} {
		t.Run(test.command, func(t *testing.T) {
			base := newTestStore(t)
			const name = "managed-lifetime.conf"
			require.NoError(t, base.WriteFile(name, []byte("managed read survives shutdown"), 0o600))
			release := make(chan struct{})
			held := &managedHeldRead{Storage: base, name: name, entered: make(chan struct{}),
				release: release, finished: make(chan struct{})}
			readResult := make(chan error, 1)
			client := &managed.ClientConfig{CheckManaged: func() bool {
				_, err := held.ReadFile(name)
				readResult <- err
				return false
			}}
			hub := startManagedLifetimeHub(t, held, client, "")
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			awaitManagedLifetime(t, held.entered, "outbound managed client did not enter its stored read")
			hub.request(t, test.command)
			select {
			case <-hub.done:
				t.Fatal("daemon returned while the outbound managed client still borrowed storage")
			case <-time.After(200 * time.Millisecond):
				// Only a negative liveness bound: the controlled read cannot end
				// before unblock, regardless of scheduling or machine speed.
			}
			unblock()
			awaitManagedLifetime(t, hub.done, "daemon did not finish after its managed read was released")
			require.Equal(t, test.exit, hub.exit)
			awaitManagedLifetime(t, held.finished, "daemon did not join its managed storage borrower")
			require.NoError(t, <-readResult)
		})
	}
}

// TestHubManagedServerStopsItsOwner boots the actual configured managed server.
// An outbound runner observes it after startup; an authorized restart must
// withdraw that same server's doctor record through its synchronous Stop.
func TestHubManagedServerStopsItsOwner(t *testing.T) {
	observed := make(chan error, 1)
	client := &managed.ClientConfig{CheckManaged: func() bool {
		_, err := pluginserver.ManagedDoctorSnapshot()
		observed <- err
		return false
	}}
	hub := startManagedLifetimeHub(t, newTestStore(t), client, `plugin {
	hub {
		server local { ip 127.0.0.1; port 0; }
		server central {
			ip 127.0.0.2; port 0;
			client edge-01 { secret "edge01-secret-that-is-at-least-32ch"; }
		}
	}
}`)
	select {
	case err := <-observed:
		require.NoError(t, err, "actual hub must start its configured managed server")
	case <-time.After(3 * time.Second):
		t.Fatal("actual hub did not reach its post-managed-server startup worker")
	}
	hub.request(t, "request restart")
	awaitManagedLifetime(t, hub.done, "actual hub did not finish its authorized restart")
	require.Equal(t, 75, hub.exit)
	_, err := pluginserver.ManagedDoctorSnapshot()
	require.ErrorIs(t, err, pluginserver.ErrManagedNotServing)
}

// TestManagedServerStopJoinsStoredRead authenticates a real managed TLS client
// and holds config-fetch inside the storage callback. Stop must join that read
// before returning; this proves the owner retained by the hub actually drains.
func TestManagedServerStopJoinsStoredRead(t *testing.T) {
	const name = "edge-01"
	base := newTestStore(t)
	key := pluginserver.ClientConfigKey(name)
	require.NoError(t, base.WriteFile(key, []byte("system {}"), 0o600))
	release := make(chan struct{})
	held := &managedHeldRead{Storage: base, name: key, entered: make(chan struct{}),
		release: release, finished: make(chan struct{})}
	srv := startManagedServer(t.Context(), held, &plugin.HubConfig{Servers: []plugin.HubServerConfig{
		{Name: "local", Host: "127.0.0.1", Port: 0},
		{Name: "central", Host: "127.0.0.2", Port: 0,
			Clients: map[string]string{name: "edge01-secret-that-is-at-least-32ch"}},
	}})
	require.NotNil(t, srv)
	t.Cleanup(srv.Stop)
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	require.Len(t, srv.Addrs(), 1)
	mc := dialHubClient(t, srv.Addrs()[0].String(), name, "edge01-secret-that-is-at-least-32ch")
	fetchDone := make(chan struct{})
	go func() {
		defer close(fetchDone)
		_, err := mc.CallRPC(t.Context(), fleet.VerbConfigFetch, fleet.ConfigFetchRequest{})
		// Cancellation may close the connection before it sends the reply.
		if err != nil {
			return
		}
	}()
	awaitManagedLifetime(t, held.entered, "managed config-fetch did not reach its stored read")
	stopped := make(chan struct{})
	go func() { srv.Stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("managed server Stop returned while config-fetch still borrowed storage")
	case <-time.After(200 * time.Millisecond):
		// The callback is held by the test, so only release can end it.
	}
	unblock()
	awaitManagedLifetime(t, stopped, "managed server did not stop after its stored read finished")
	awaitManagedLifetime(t, held.finished, "managed server Stop did not join its storage borrower")
	awaitManagedLifetime(t, fetchDone, "managed client RPC did not end after server stop")
}

// managedLifetimeHub owns one actual hub Run in a test. done publishes exit;
// the test MUST release any controlled borrower before cleanup joins this Run.
type managedLifetimeHub struct {
	dispatch plugin.CommandDispatcher
	done     chan struct{}
	exit     int
}

func startManagedLifetimeHub(t *testing.T, store storage.Storage, client *managed.ClientConfig, extra string) *managedLifetimeHub {
	t.Helper()
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	withCleanRegistry(t)
	stopRestartCause.Store(nil)
	rebootRequested.Store(false)
	t.Cleanup(func() { stopRestartCause.Store(nil); rebootRequested.Store(false) })
	oldDir := env.Get("ze.config.dir")
	require.NoError(t, env.Set("ze.config.dir", t.TempDir()))
	t.Cleanup(func() { require.NoError(t, env.Set("ze.config.dir", oldDir)) })
	oldREST, oldGRPC, oldBNG := restBuild, grpcBuild, bngRegister
	t.Cleanup(func() { restBuild, grpcBuild, bngRegister = oldREST, oldGRPC, oldBNG })
	restBuild, bngRegister = nil, nil
	dispatchReady := make(chan plugin.CommandDispatcher, 1)
	grpcBuild = func(in *apiBuildInputs, _ *apiShared) (apiServerHandle, error) {
		dispatchReady <- serverDispatcher(in.Server, audit.GRPC)
		return apiServerHandle{}, nil
	}
	config := []byte(`environment {
	api-server { grpc { enabled true; server main { ip 127.0.0.1; port 0; } } }
}
` + extra)
	hub := &managedLifetimeHub{done: make(chan struct{})}
	go func() {
		defer close(hub.done)
		hub.exit = runYANGConfig(store, "-", config, nil, 0, -1,
			false, false, "", false, "", "", false, client)
	}()
	select {
	case hub.dispatch = <-dispatchReady:
	case <-hub.done:
		t.Fatalf("hub exited before constructing its API: %d", hub.exit)
	case <-time.After(3 * time.Second):
		t.Fatal("hub did not construct its API")
	}
	t.Cleanup(func() {
		select {
		case <-hub.done:
			return
		default:
			hub.request(t, "request shutdown")
			awaitManagedLifetime(t, hub.done, "test-owned hub did not stop during cleanup")
		}
	})
	return hub
}

func (h *managedLifetimeHub) request(t *testing.T, command string) {
	t.Helper()
	response, err := h.dispatch(context.Background(), plugin.CallerIdentity{
		Username: "admin", Authorizer: authz.StoreAuthorizer{Store: daemonRestartSurfacePolicy()},
	}, command)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusDone, response.Status)
	response.TransportComplete()
}

func awaitManagedLifetime(t *testing.T, done <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal(failure)
	}
}
