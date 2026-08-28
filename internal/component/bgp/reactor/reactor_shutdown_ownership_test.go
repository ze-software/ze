// Design: docs/architecture/api/process-protocol.md -- who owns a plugin server's lifecycle
// Detail: reactor.go -- Reactor.cleanup, the ownership guard these tests pin
// Related: internal/component/plugin/process/manager_test.go -- the process-layer half

package reactor

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// newBorrowedPluginServer builds and starts a plugin server the way the hub does
// (cmd/ze/hub/main.go, runHub), hosting the named internal plugins, and returns it
// once every one of them is spawned. The caller injects it into a borrow-mode
// reactor, which is the production wiring: pluginserver.NewServer ->
// registry.SetPluginServer -> registry.GetPluginServer -> Reactor.SetPluginServerAny.
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
// ze-chaos in-process runner (internal/chaos/inprocess/runner.go) ends its simulation
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
