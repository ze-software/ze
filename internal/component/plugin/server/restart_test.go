package server

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// restartExclusiveRole is the claim token the restart tests move between two
// plugins. Its spelling is arbitrary; the engine treats a claim as opaque.
const restartExclusiveRole = "restart-test-peer-up-replay"

// restartProbe counts what a restarted plugin must go through again.
type restartProbe struct {
	configures atomic.Int64 // Stage 2 configure callbacks received
	claimSeen  atomic.Int64 // configure callbacks that saw the role claimed
	commands   atomic.Int64 // commands executed
}

// registerRestartPlugins registers a claimant that declares the exclusive role
// and a plugin that stands down for it, and returns the probe the stand-down
// plugin writes on every handshake it completes.
func registerRestartPlugins(t *testing.T, claimant, standDown, commandName string) *restartProbe {
	t.Helper()
	probe := &restartProbe{}

	require.NoError(t, registry.Register(registry.Registration{
		Name:        claimant,
		Description: "restart test claimant",
		Claims:      []string{restartExclusiveRole},
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(claimant, conn)
			if err := p.Run(context.Background(), sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	require.NoError(t, registry.Register(registry.Registration{
		Name:        standDown,
		Description: "restart test stand-down plugin",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(standDown, conn)
			p.OnConfigure(func([]sdk.ConfigSection) error {
				probe.configures.Add(1)
				if p.ClaimActive(restartExclusiveRole) {
					probe.claimSeen.Add(1)
				}
				return nil
			})
			p.OnExecuteCommand(func(_, _ string, _ []string, _ string) (string, any, error) {
				probe.commands.Add(1)
				return "done", map[string]any{"served": true}, nil
			})
			err := p.Run(context.Background(), sdk.Registration{
				Commands: []sdk.CommandDecl{{Name: commandName}},
			})
			if err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	return probe
}

// TestRestartPluginReRunsTheStartupHandshake verifies that restarting a plugin
// brings the replacement process through the 5-stage handshake, so it is
// configured, is told which exclusive roles other plugins hold, and has its
// commands registered against itself.
//
// VALIDATES: AC-5 -- a plugin that stood its default behavior down for another
// plugin's claim is told about that claim again after a restart, before it can
// receive any event.
// PREVENTS: ProcessManager.Respawn spawning a replacement the engine never
// speaks to. Before this, Respawn called StartWithContext and nothing else, so
// the replacement held no registration, no delivered config, no subscriptions,
// no commands and no claim set: it resumed its own default behavior, which is
// the duplicate announce the claim exists to prevent.
func TestRestartPluginReRunsTheStartupHandshake(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const claimant = "restart-claimant"
	const standDown = "restart-stand-down"
	const commandName = "show restart standdown"

	probe := registerRestartPlugins(t, claimant, standDown, commandName)

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: claimant, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: standDown, Internal: true, Encoder: plugin.EncodingJSON},
	}))

	require.Equal(t, int64(1), probe.configures.Load(), "startup must configure the stand-down plugin once")
	require.Equal(t, int64(1), probe.claimSeen.Load(), "startup must tell it the role is claimed")

	// Startup is over: the command registry is frozen, exactly as
	// signalStartupComplete leaves it before any reload can run.
	s.dispatcher.Registry().Freeze()

	pm := s.procManager.Load()
	require.NotNil(t, pm)
	before := pm.GetProcess(standDown)
	require.NotNil(t, before)

	require.NoError(t, s.restartPlugin(standDown))

	after := pm.GetProcess(standDown)
	require.NotNil(t, after)
	assert.NotSame(t, before, after, "restart must replace the process")

	assert.Equal(t, int64(2), probe.configures.Load(),
		"the replacement must be configured again")
	assert.Equal(t, int64(2), probe.claimSeen.Load(),
		"the replacement must be told the role is still claimed")

	cmd := s.dispatcher.Registry().Lookup(commandName)
	require.NotNil(t, cmd, "the replacement's command must be resolvable after the restart")
	assert.Same(t, after, cmd.Process, "the command must resolve to the replacement process")

	assert.NotNil(t, s.registry.LookupCommand(commandName),
		"the plugin registry row must be rebuilt for the replacement")
}

// TestRestartPluginRestartsAPluginThatDeclaredNothing verifies that a rollback
// restart is carried out for a plugin that made no declaration about its own
// failure.
//
// VALIDATES: AC-11 -- the config-reload rollback restart succeeds.
// PREVENTS: the state this spec found. ProcessManager.Respawn refused every
// plugin whose PluginConfig.Respawn was false, and no producer ever set that
// field, so the rollback path could not restart anything: an ack reporting a
// BROKEN plugin was answered with "respawn not enabled for plugin" every time.
//
// A rollback restart is an ORDER, not a policy question. The orchestrator has
// already been told the plugin is broken, and it is repairing a config
// transaction rather than reacting to a failure, so it does not consult the
// plugin's failure policy.
func TestRestartPluginRestartsAPluginThatDeclaredNothing(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const claimant = "restart-noenable-claimant"
	const standDown = "restart-noenable-stand-down"
	const commandName = "show restart noenable"

	probe := registerRestartPlugins(t, claimant, standDown, commandName)

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: claimant, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: standDown, Internal: true, Encoder: plugin.EncodingJSON},
	}))

	s.dispatcher.Registry().Freeze()

	pm := s.procManager.Load()
	require.NotNil(t, pm)
	before := pm.GetProcess(standDown)
	require.NotNil(t, before)

	require.NoError(t, s.restartPlugin(standDown))

	after := pm.GetProcess(standDown)
	require.NotNil(t, after)
	assert.NotSame(t, before, after, "the rollback restart must replace the process")
	assert.Equal(t, int64(2), probe.configures.Load(), "the replacement must be configured again")

	cmd := s.dispatcher.Registry().Lookup(commandName)
	require.NotNil(t, cmd, "the replacement's command must be resolvable")
	assert.Same(t, after, cmd.Process, "the command must resolve to the replacement process")
}

// TestRestartBeforeStartupCompletesLeavesTheFanOutToDeliverOnce verifies that a
// plugin restarted while startup phases are still running receives the
// post-startup callback exactly once, from the fan-out that owes it.
//
// VALIDATES: AC-13 -- a restarted plugin runs its OnAllPluginsReady handler, and
// runs it once. The handler is where a plugin takes an exclusive role over from
// another one, and those handlers are written for a single call
// (sendPostStartupToNames, poststartup.go).
// PREVENTS: a second delivery. A plugin's own exit reaches restartHandshake
// through applyFailurePolicy, whose supervisors start at the end of each
// runPluginPhase, so a plugin that exits between two phases is restarted BEFORE
// signalStartupComplete. Delivering there and again from sendPostStartupToAll
// runs the handler twice, and the first of the two runs before the registries
// are frozen, which is the state the callback's contract promises the handler.
func TestRestartBeforeStartupCompletesLeavesTheFanOutToDeliverOnce(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "restart-early-ready"
	ready := &atomic.Int64{}

	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "restart test plugin that counts its post-startup callbacks",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			p.OnAllPluginsReady(func() error {
				ready.Add(1)
				return nil
			})
			if err := p.Run(context.Background(), sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
	}))

	// No signalStartupComplete yet: this is the window a later phase would still
	// be running in.
	require.NoError(t, s.restartPlugin(name))
	require.Never(t, func() bool { return ready.Load() > 0 }, failureSettle, 10*time.Millisecond,
		"a restart before startup completed must leave the callback to the fan-out")

	s.signalStartupComplete()
	require.Eventually(t, func() bool { return ready.Load() == 1 }, failureWait, 10*time.Millisecond,
		"the fan-out must deliver the callback to the replacement")
	assert.Never(t, func() bool { return ready.Load() > 1 }, failureSettle, 10*time.Millisecond,
		"the replacement must not run its OnAllPluginsReady handler twice")
}
