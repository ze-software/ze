// Design: docs/architecture/fleet-config.md -- managed borrowers end before the daemon returns.
// Related: managed_lifetime_test.go -- the managed server's own Stop proof, which needs no API surface.
//
// These tests drive an actual hub Run and stop or restart it through a command
// dispatcher, which they take from the gRPC API surface: its api-server grpc
// block parses only in a build carrying ze_grpc.
//
//go:build ze_grpc

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
)

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
			client := &managed.ClientConfig{Baseline: managed.NewBaselineAccess(base), CheckManaged: func() bool {
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
	client := &managed.ClientConfig{Baseline: managed.NewBaselineAccess(newTestStore(t)), CheckManaged: func() bool {
		_, err := pluginserver.ManagedDoctorSnapshot()
		observed <- err
		return false
	}}
	hub := startManagedLifetimeHub(t, newTestStore(t), client, `bus {
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
		// The daemon builds its hub client through a builder; this one hands
		// it the client the test controls.
		hub.exit = runYANGConfig(store, "-", config, nil, 0, -1,
			false, false, "", false, "", "", false,
			func() (*managed.ClientConfig, error) { return client, nil })
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
