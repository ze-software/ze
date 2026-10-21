// Design: docs/architecture/api/process-protocol.md -- who owns a plugin server's lifecycle
// Detail: reactor.go -- Reactor.cleanup, the ownership guard these tests pin
// Related: internal/component/plugin/process/manager_test.go -- the process-layer half

package reactor

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// newBorrowedPluginServer builds and starts a plugin server before the reactor
// starts, as the hub does (cmd/ze/hub/main.go, runHub), hosting the named
// internal plugins, and returns it once every one of them is spawned. The caller
// injects it into a borrow-mode reactor, which is the production ownership:
// pluginserver.NewServer -> registry.SetPluginServer -> registry.GetPluginServer
// -> Reactor.SetPluginServerAny.
//
// One difference from the hub: the server's reactor is a reactorAPIAdapter over
// r, where the hub passes a plugin.Coordinator that forwards to the reactor only
// once the bgp plugin has called Coordinator.SetReactor. Here the server reaches
// r from its first signal, so a startup signal can land while r is still in
// StartWithContext, which is the ordering the startup barrier must survive.
func newBorrowedPluginServer(t *testing.T, r *Reactor, names ...string) *pluginserver.Server {
	t.Helper()
	configs := make([]plugin.PluginConfig, 0, len(names))
	for _, name := range names {
		configs = append(configs, plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"})
	}
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: configs}, &reactorAPIAdapter{r: r})
	require.NoError(t, err)
	if len(configs) > 0 {
		// The hub spawns plugin processes through a PluginManager and hands it to
		// the server (cmd/ze/hub/main.go, apiServer.SetProcessSpawner(pm)).
		mgr := pluginmgr.NewManager()
		require.NoError(t, mgr.StartAll(context.Background(), nil, nil))
		srv.SetProcessSpawner(mgr)
	}
	require.NoError(t, srv.StartWithContext(context.Background()))
	t.Cleanup(srv.Stop)
	for _, name := range names {
		require.Eventually(t, func() bool {
			pm := srv.ProcessManager()
			return pm != nil && pm.GetProcess(name) != nil
		}, 5*time.Second, 10*time.Millisecond, "plugin %s never spawned in the hub's server", name)
	}
	return srv
}

// startBorrowedPeers starts a borrow-mode reactor's peers once the borrowed
// server has finished plugin startup, which is when the daemon starts them: the
// bgp plugin registers StartPeers as the coordinator's post-startup callback
// (internal/component/bgp/plugin/register.go), and that callback runs from
// SignalPluginStartupComplete, after every plugin's startup handshake. A peer
// started earlier validates its OPEN against plugin registrations the startup
// handshake is still writing (Server.PluginsWithPerPeerOpenPolicy reading
// Process.Registration), an ordering the daemon never produces.
//
// The caller MUST have started r with StartWithContext first.
func startBorrowedPeers(t *testing.T, r *Reactor, srv *pluginserver.Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, srv.WaitForStartupComplete(ctx), "the borrowed server never finished plugin startup")
	require.NoError(t, r.StartPeers())
}

// registerIdleEngine registers an internal plugin whose engine runs until its
// connection is closed, which is what the daemon's shutdown signal does.
func registerIdleEngine(t *testing.T, name string) {
	t.Helper()
	// The plugin registry is global to the process and has no unregister, so a
	// name registered here outlives the test that registered it.
	// Repeated package runs execute in one process, so without this restore every
	// run after the first fails on a duplicate plugin
	// name.
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	err := registry.Register(registry.Registration{
		Name:        name,
		Description: "test plugin for reactor shutdown ownership",
		RunEngine: func(conn net.Conn) int {
			buf := make([]byte, 64)
			for {
				if _, err := conn.Read(buf); err != nil {
					return 0
				}
			}
		},
		CLIHandler: func(_ []string) int { return 0 },
	})
	require.NoError(t, err)
}

// stopAndWait runs the shutdown tail of runBGPEngine
// (internal/component/bgp/plugin/register.go): Stop, then an unbounded Wait.
func stopAndWait(t *testing.T, r *Reactor) {
	t.Helper()
	r.Stop()
	waitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, r.Wait(waitCtx), "reactor did not finish its cleanup")
}

// VALIDATES: spec-fixit-shutdown-waits-out-a-deadlock -- candidate A, the ownership
// guard. A borrow-mode reactor does not stop the plugin server the hub constructed
// and still owns.
//
// PREVENTS: the cycle at its cause. Reactor.cleanup Phase 1 stopping r.api reaches
// Server.Stop -> Server.cleanup -> ProcessManager.Stop, which waits for the bgp
// engine that is blocked in Reactor.Wait on the goroutine running this very cleanup.
// The process-layer test measures what that costs; this one names why it happens.
func TestReactorCleanupDoesNotStopWhatItDoesNotOwn(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0"}) // Standalone=false: borrow mode
	srv := newBorrowedPluginServer(t, r)
	r.SetPluginServer(srv)
	require.NoError(t, r.StartWithContext(context.Background()))

	stopAndWait(t, r)

	assert.True(t, srv.Running(),
		"the reactor stopped the hub's plugin server, which it borrowed and does not own")
}

// VALIDATES: spec-fixit-shutdown-waits-out-a-deadlock -- the second defect the same
// line carries. runBGPEngine returns on two occasions, not one: daemon shutdown, and
// bgp being REMOVED at reload (Server.autoStopForRemovedConfigPaths stops the bgp
// process, its engine returns, and its tail calls Reactor.Stop). An unguarded
// Reactor.cleanup then takes the hub's whole plugin server down, killing every OTHER
// plugin, while the daemon keeps running.
//
// PREVENTS: a reload that removes one plugin silently ending all of them. The
// surviving plugin is what makes this the reload case rather than a second spelling
// of the test above: at daemon shutdown nobody would miss it.
func TestBGPRemovedAtReloadLeavesTheHubPluginServerRunning(t *testing.T) {
	const survivor = "test-reactor-reload-survivor"
	registerIdleEngine(t, survivor)

	r := New(&Config{ListenAddr: "127.0.0.1:0"}) // Standalone=false: borrow mode
	srv := newBorrowedPluginServer(t, r, survivor)
	r.SetPluginServer(srv)
	require.NoError(t, r.StartWithContext(context.Background()))

	// The daemon is NOT stopping. Only bgp was removed, so only its engine returns.
	stopAndWait(t, r)

	assert.True(t, srv.Running(),
		"removing bgp at reload stopped the hub's plugin server, so the daemon lost its whole plugin surface")
	pm := srv.ProcessManager()
	require.NotNil(t, pm, "the hub's server lost its process manager")
	assert.NotNil(t, pm.GetProcess(survivor),
		"removing bgp at reload also killed %s, a plugin the reload never touched", survivor)
}

// VALIDATES: spec-fixit-shutdown-waits-out-a-deadlock -- the guard is on OWNERSHIP,
// not on the call site. A standalone reactor builds its own plugin server
// (startAPIServer, the !externalServer branch) and cleanup is its only stop: the
// le chaos run in-process runner (internal/chaos/inprocess/runner.go) ends its simulation
// with reactorCancel plus Reactor.Wait and nothing else ever stops that server.
//
// PREVENTS: fixing the cycle by deleting the calls instead of guarding them, which
// would leave the chaos runner's server and every plugin under it alive after the
// simulation ends.
func TestReactorCleanupStopsTheServerItOwns(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	var owned *pluginserver.Server
	r.pluginServerMaker = func(cfg *pluginserver.ServerConfig, lifecycle plugin.ReactorLifecycle) (*pluginserver.Server, error) {
		srv, err := pluginserver.NewServer(cfg, lifecycle)
		owned = srv
		return srv, err
	}
	require.NoError(t, r.StartWithContext(context.Background()))
	require.NotNil(t, owned, "standalone reactor must self-host a plugin server")
	require.True(t, owned.Running(), "the self-hosted server must be running before the stop")

	stopAndWait(t, r)

	assert.False(t, owned.Running(),
		"the reactor left its OWN plugin server running, so a standalone consumer has no stop for it")
}

// TestReactorStopsWithACongestedForwardWorker proves that a reactor whose
// forward worker is congested at shutdown finishes its cleanup. A worker whose
// queue drains fires onResumed, which reads the peer table under r.mu; Stop
// itself empties the overflow, so a congested worker always fires it on the
// way out. The test parks one worker in its batch handler, fills its queue
// until TryDispatch fails (onCongested), stops the reactor, and releases the
// handler only once the pool is stopping, so the worker's onResumed lands
// while cleanup waits for it.
//
// VALIDATES: cleanup waits for the forward workers without holding r.mu.
// PREVENTS: a daemon whose shutdown hangs because cleanup held r.mu while
// fwdPool.Stop waited for a worker blocked on r.mu.RLock (the reactor Wait has
// no bound in production).
func TestReactorStopsWithACongestedForwardWorker(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	// The handler is replaced before any worker exists, so no worker reads
	// the production one.
	r.fwdPool.handler = func(fwdKey, []fwdItem) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
	}
	srv := newBorrowedPluginServer(t, r)
	r.SetPluginServer(srv)
	require.NoError(t, r.StartWithContext(context.Background()))

	key := fwdKey{peerAddr: netip.MustParseAddrPort("192.0.2.9:179")}
	require.True(t, r.fwdPool.Dispatch(key, &fwdItem{}))
	<-entered
	for range r.fwdPool.cfg.chanSize + 1 {
		if !r.fwdPool.TryDispatch(key, &fwdItem{}) {
			break
		}
	}
	require.False(t, r.fwdPool.TryDispatch(key, &fwdItem{}), "the worker's queue never filled")

	r.Stop()
	stopping := func() bool {
		r.fwdPool.mu.RLock()
		defer r.fwdPool.mu.RUnlock()
		return r.fwdPool.stopped
	}
	lowEventually(t, stopping, "the forward pool is stopping")
	close(release)

	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, r.Wait(waitCtx), "the reactor did not finish its cleanup with a congested forward worker")
}
