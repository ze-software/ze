// VALIDATES: a managed client follows the hub connection of each config it
// applies -- a config naming another hub moves the running client to it, a
// config with no hub client block ends the client, a config that leaves the
// block alone keeps the connection, and a refused config changes nothing.
// PREVENTS: the client reading its hub address, secret and trust anchor once
// at start, so a pushed change to any of them was applied to the daemon and to
// nothing the client did until ze was restarted.

package managed

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/pki"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// endpointAt is the Endpoint reading these tests use: the config is the hub
// address itself, and the word "none" stands for a config with no hub client
// block. The real reading is cmd/ze hubClientEndpoint.
func endpointAt(config []byte) (Endpoint, error) {
	if string(config) == "none" {
		return Endpoint{}, ErrNoHubClient
	}
	return Endpoint{Name: "edge-01", Server: string(config), Token: testHubToken, CA: "fleet-hub"}, nil
}

// applyOnce runs one fetch against a hub that serves config and returns what
// fetchAndProcess answered and the ack the hub received.
func applyOnce(t *testing.T, cfg *ClientConfig, config []byte) (fleet.ConfigAck, error) {
	t.Helper()
	clientEnd, hubEnd := net.Pipe()
	t.Cleanup(func() { clientEnd.Close() }) //nolint:errcheck // test cleanup
	t.Cleanup(func() { hubEnd.Close() })    //nolint:errcheck // test cleanup
	ackCh := make(chan fleet.ConfigAck, 1)
	go mockHubFetchAndAck(t, hubEnd, config, ackCh)

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	t.Cleanup(func() { mc.Close() }) //nolint:errcheck // test cleanup
	err := fetchAndProcess(context.Background(), mc, cfg)
	select {
	case ack := <-ackCh:
		return ack, err
	case <-time.After(3 * time.Second):
		t.Fatal("the hub received no ack")
		return fleet.ConfigAck{}, err
	}
}

// TestAppliedConfigChangesEndpoint: an applied config that names other hub
// settings is acknowledged, then ends the connection with the new settings in
// place for the next one.
func TestAppliedConfigChangesEndpoint(t *testing.T) {
	access := appliedDevice(t)
	cfg := &ClientConfig{
		Name: "edge-01", Server: "192.0.2.1:1791", Token: testHubToken, CA: "fleet-hub",
		Handler:    &Handler{Validate: func([]byte) error { return nil }},
		CommitOver: stagedOver(access, func([]byte) error { return nil }),
		Baseline:   access,
		Endpoint:   endpointAt,
	}

	ack, err := applyOnce(t, cfg, []byte("192.0.2.2:1791"))

	require.ErrorIs(t, err, errEndpointChanged)
	assert.True(t, ack.OK, "the config was applied, so the hub is told so before the connection ends")
	assert.Equal(t, "192.0.2.2:1791", cfg.Server)
	assert.Equal(t, fleet.VersionHash([]byte("192.0.2.2:1791")), ack.Version,
		"the ack names the version of the configuration the hub sent")
}

// TestAppliedConfigKeepsEndpoint: an applied config whose hub settings are
// the ones in use keeps the connection.
func TestAppliedConfigKeepsEndpoint(t *testing.T) {
	access := appliedDevice(t)
	cfg := &ClientConfig{
		Name: "edge-01", Server: "192.0.2.1:1791", Token: testHubToken, CA: "fleet-hub",
		Handler:    &Handler{Validate: func([]byte) error { return nil }},
		CommitOver: stagedOver(access, func([]byte) error { return nil }),
		Baseline:   access,
		Endpoint:   endpointAt,
	}

	ack, err := applyOnce(t, cfg, []byte("192.0.2.1:1791"))

	require.NoError(t, err)
	assert.True(t, ack.OK)
	assert.Equal(t, "192.0.2.1:1791", cfg.Server)
}

// TestAppliedConfigWithoutHubClient: an applied config with no hub client
// block is acknowledged and answers errHubClientRemoved, with the settings in
// use untouched.
func TestAppliedConfigWithoutHubClient(t *testing.T) {
	access := appliedDevice(t)
	cfg := &ClientConfig{
		Name: "edge-01", Server: "192.0.2.1:1791", Token: testHubToken, CA: "fleet-hub",
		Handler:    &Handler{Validate: func([]byte) error { return nil }},
		CommitOver: stagedOver(access, func([]byte) error { return nil }),
		Baseline:   access,
		Endpoint:   endpointAt,
	}

	ack, err := applyOnce(t, cfg, []byte("none"))

	require.ErrorIs(t, err, errHubClientRemoved)
	assert.True(t, ack.OK)
	assert.Equal(t, "192.0.2.1:1791", cfg.Server)
}

// TestRefusedConfigKeepsEndpoint: a config the daemon refused to apply names
// nothing the client follows, whatever hub block it holds.
func TestRefusedConfigKeepsEndpoint(t *testing.T) {
	consulted := false
	access := appliedDevice(t)
	cfg := &ClientConfig{
		Name: "edge-01", Server: "192.0.2.1:1791", Token: testHubToken, CA: "fleet-hub",
		Handler:    &Handler{Validate: func([]byte) error { return nil }},
		CommitOver: stagedOver(access, func([]byte) error { return errors.New("verify rejected") }),
		Baseline:   access,
		Endpoint: func(config []byte) (Endpoint, error) {
			consulted = true
			return endpointAt(config)
		},
	}

	ack, err := applyOnce(t, cfg, []byte("192.0.2.2:1791"))

	require.NoError(t, err)
	assert.False(t, ack.OK)
	assert.False(t, consulted, "the hub block of a refused config was read")
	assert.Equal(t, "192.0.2.1:1791", cfg.Server)
	requireDeviceUnchanged(t, access)
}

// startServingHub starts a real managed server that admits edge-01 and serves
// it config. fetched receives one value for each config-fetch it answers.
func startServingHub(t *testing.T, root *pki.Root, config func() []byte) (addr string, fetched <-chan struct{}) {
	t.Helper()
	seen := make(chan struct{}, 16)
	srv, err := pluginserver.NewManagedServer(pluginserver.ManagedServerConfig{
		Authority: root,
		Listeners: []pluginserver.ManagedListener{
			{Addr: "127.0.0.1:0", Clients: map[string]string{"edge-01": testHubToken}},
		},
		ReadConfig: func(string) ([]byte, error) {
			select {
			case seen <- struct{}{}:
			default: // The test reads only the first few.
			}
			return config(), nil
		},
	})
	require.NoError(t, err)
	require.NoError(t, srv.Start(context.Background()))
	t.Cleanup(srv.Stop)
	return srv.Addrs()[0].String(), seen
}

// runClient runs RunManagedClient until the test ends and returns a channel
// closed when it returns.
func runClient(t *testing.T, cfg ClientConfig) <-chan struct{} {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		RunManagedClient(ctx, cfg)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return done
}

// TestRunningClientMovesToTheHubItsConfigNames: the whole path against two
// real hubs. The first serves a config that names the second; the client
// applies it and MUST then fetch from the second, with no restart.
//
// MUTATION: return nil in place of errEndpointChanged in adoptEndpoint and
// this fails: the client stays on the first hub and the second sees no fetch.
func TestRunningClientMovesToTheHubItsConfigNames(t *testing.T) {
	root := testRoot(t)
	installCA(t, root)

	// The second hub serves a config naming itself, so the client stays on it.
	var own atomic.Pointer[string]
	secondAddr, secondFetched := startServingHub(t, root, func() []byte { return []byte(*own.Load()) })
	own.Store(&secondAddr)
	firstAddr, firstFetched := startServingHub(t, root, func() []byte { return []byte(secondAddr) })
	access := appliedDevice(t)

	runClient(t, ClientConfig{
		Name: "edge-01", Server: firstAddr, Token: testHubToken, CA: "fleet-hub",
		Handler:    &Handler{Validate: func([]byte) error { return nil }},
		CommitOver: stagedOver(access, func([]byte) error { return nil }),
		Baseline:   access,
		Endpoint:   endpointAt,
	})

	select {
	case <-firstFetched:
	case <-time.After(5 * time.Second):
		t.Fatal("the client never fetched from the hub it was started with")
	}
	select {
	case <-secondFetched:
	case <-time.After(5 * time.Second):
		t.Fatal("the client applied a config naming another hub and never connected to it")
	}
}

// TestRunningClientStopsWhenItsConfigNamesNoHub: a client whose applied
// config has no hub client block stops following its hub, as it would after a
// restart on that config.
func TestRunningClientStopsWhenItsConfigNamesNoHub(t *testing.T) {
	root := testRoot(t)
	installCA(t, root)
	addr, _ := startServingHub(t, root, func() []byte { return []byte("none") })
	access := appliedDevice(t)

	done := runClient(t, ClientConfig{
		Name: "edge-01", Server: addr, Token: testHubToken, CA: "fleet-hub",
		Handler:    &Handler{Validate: func([]byte) error { return nil }},
		CommitOver: stagedOver(access, func([]byte) error { return nil }),
		Baseline:   access,
		Endpoint:   endpointAt,
	})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the client kept following a hub its applied config no longer names")
	}
}
