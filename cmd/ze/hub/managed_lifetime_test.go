// Design: docs/architecture/fleet-config.md -- managed borrowers end before the daemon returns.
// Related: main.go -- actual managed worker ownership and daemon teardown.
package hub

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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

func awaitManagedLifetime(t *testing.T, done <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal(failure)
	}
}
