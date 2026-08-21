package server

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type lifecycleTestSpawner struct {
	ctx context.Context
	pm  *process.ProcessManager
}

func (s *lifecycleTestSpawner) SpawnMore(configs []plugin.PluginConfig) error {
	if s.pm == nil {
		s.pm = process.NewProcessManager(configs)
		return s.pm.StartWithContext(s.ctx)
	}
	return s.pm.StartMore(configs)
}

func (s *lifecycleTestSpawner) GetProcessManager() any {
	return s.pm
}

func newLifecycleStartupServer(t *testing.T) (*Server, *lifecycleTestSpawner) {
	t.Helper()

	serverCtx, cancel := context.WithCancel(context.Background())
	s, err := NewServer(&ServerConfig{}, &mockReactor{})
	require.NoError(t, err)
	s.ctx, s.cancel = context.WithCancel(serverCtx)
	t.Cleanup(func() {
		s.cancel()
		cancel()
		if pm := s.procManager.Load(); pm != nil {
			pm.Stop()
		}
	})

	spawner := &lifecycleTestSpawner{ctx: s.ctx}
	s.SetProcessSpawner(spawner)
	return s, spawner
}

func registerLifecyclePlugin(t *testing.T, name string, optionalDeps []string, run func(net.Conn) int) {
	t.Helper()
	require.NoError(t, registry.Register(registry.Registration{
		Name:                 name,
		Description:          "lifecycle rollback test plugin",
		OptionalDependencies: optionalDeps,
		RunEngine:            run,
		CLIHandler:           func([]string) int { return 0 },
	}))
}

func registerLifecyclePluginWithDeps(t *testing.T, name string, deps, optionalDeps []string, run func(net.Conn) int) {
	t.Helper()
	require.NoError(t, registry.Register(registry.Registration{
		Name:                 name,
		Description:          "lifecycle rollback test plugin",
		Dependencies:         deps,
		OptionalDependencies: optionalDeps,
		RunEngine:            run,
		CLIHandler:           func([]string) int { return 0 },
	}))
}

// TestPluginStartupRollsBackPartialRegistration verifies a plugin whose family
// declaration fails after earlier declarations does not leave plugin registry or
// family registry state visible.
//
// VALIDATES: AC-1, a later family conflict makes startup fail and rolls back the failed plugin's registry rows.
// PREVENTS: Failed startup leaving commands or earlier family declarations from a rejected plugin.
func TestPluginStartupRollsBackPartialRegistration(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-family-conflict"
	const commandName = "show lifecycle rollback"
	const firstFamily = "lifecycle/one"
	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{Name: commandName}},
			Families: []sdk.FamilyDecl{
				{Name: firstFamily, Mode: "both", AFI: 65000, SAFI: 200},
				{Name: "not-ipv4/test", Mode: "both", AFI: uint16(family.AFIIPv4), SAFI: 201},
			},
		})
		if err != nil {
			return 1
		}
		return 0
	})

	s, _ := newLifecycleStartupServer(t)
	err := s.runPluginPhase([]plugin.PluginConfig{{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), pluginName)
	assert.Empty(t, s.registry.LookupCommand(commandName))
	assert.Empty(t, s.registry.LookupFamily(firstFamily))
	_, ok := family.LookupFamily(firstFamily)
	assert.False(t, ok, "first family from failed batch must not remain registered")
}

// TestPluginStartupRollsBackFamiliesAfterLaterStageFailure verifies families
// committed during declaration are removed if a later startup stage fails.
//
// VALIDATES: AC-1 and AC-3, a plugin failing after family declaration leaves no dynamic family registry state.
// PREVENTS: Capability/config failures leaving runtime families from a plugin that never reached ready.
func TestPluginStartupRollsBackFamiliesAfterLaterStageFailure(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-family-late-fail"
	const commandName = "show lifecycle family late"
	const familyName = "lifecycle/late"
	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		p.OnConfigure(func([]sdk.ConfigSection) error {
			return fmt.Errorf("reject config")
		})
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{Name: commandName}},
			Families: []sdk.FamilyDecl{
				{Name: familyName, Mode: "both", AFI: 65001, SAFI: 202},
			},
		})
		if err != nil {
			return 1
		}
		return 0
	})

	s, _ := newLifecycleStartupServer(t)
	err := s.runPluginPhase([]plugin.PluginConfig{{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), pluginName)
	assert.Empty(t, s.registry.LookupCommand(commandName))
	assert.Empty(t, s.registry.LookupFamily(familyName))
	_, ok := family.LookupFamily(familyName)
	assert.False(t, ok, "dynamic family from later failed startup stage must be unregistered")
}

// TestRunPluginPhaseReturnsStageFailure verifies a pre-ready stage failure is
// returned by runPluginPhase and the failed process is not committed.
//
// VALIDATES: AC-3, a process failing before ready makes the phase return an error and skips runtime startup for that process.
// PREVENTS: Startup goroutine failures being swallowed and failed processes entering runtime handling.
func TestRunPluginPhaseReturnsStageFailure(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-config-fail"
	const commandName = "show lifecycle failed"
	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		p.OnConfigApply(func([]sdk.ConfigDiffSection) error { return nil })
		p.OnConfigure(func([]sdk.ConfigSection) error {
			return fmt.Errorf("reject config")
		})
		err := p.Run(context.Background(), sdk.Registration{Commands: []sdk.CommandDecl{{Name: commandName}}})
		if err != nil {
			return 1
		}
		return 0
	})

	s, spawner := newLifecycleStartupServer(t)
	err := s.runPluginPhase([]plugin.PluginConfig{{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), pluginName)
	assert.Empty(t, s.registry.LookupCommand(commandName))
	if spawner.pm != nil {
		assert.Nil(t, spawner.pm.GetProcess(pluginName), "failed process must be removed instead of entering runtime handling")
	}
}

// TestRunPluginPhaseRollsBackUnprocessedDependencyTier verifies startup failure
// stops later-tier processes that were spawned before the failing tier ran.
//
// VALIDATES: AC-3, spawned processes that never entered the startup handshake
// are removed when an earlier dependency tier fails.
// PREVENTS: failed startup leaving later-tier plugin goroutines unmanaged in the ProcessManager.
func TestRunPluginPhaseRollsBackUnprocessedDependencyTier(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const failingPlugin = "lifecycle-tier-fail"
	const dependentPlugin = "lifecycle-tier-dependent"
	registerLifecyclePlugin(t, failingPlugin, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(failingPlugin, conn)
		p.OnConfigure(func([]sdk.ConfigSection) error {
			return fmt.Errorf("reject config")
		})
		err := p.Run(context.Background(), sdk.Registration{})
		if err != nil {
			return 1
		}
		return 0
	})
	registerLifecyclePluginWithDeps(t, dependentPlugin, []string{failingPlugin}, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(dependentPlugin, conn)
		err := p.Run(context.Background(), sdk.Registration{})
		if err != nil {
			return 1
		}
		return 0
	})

	s, spawner := newLifecycleStartupServer(t)
	err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: failingPlugin, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: dependentPlugin, Internal: true, Encoder: plugin.EncodingJSON},
	})
	require.Error(t, err)
	require.NotNil(t, spawner.pm)
	assert.Nil(t, spawner.pm.GetProcess(failingPlugin), "failed tier process must be removed")
	assert.Nil(t, spawner.pm.GetProcess(dependentPlugin), "unprocessed later-tier process must be removed")
	assert.False(t, s.isPluginLoaded(dependentPlugin), "unprocessed later-tier plugin must not remain marked loaded")
}

// TestRunPluginPhaseAllowsMissingOptionalDependency verifies optional dependency
// absence is still successful.
//
// VALIDATES: AC-5, missing OptionalDependencies do not fail startup.
// PREVENTS: Lifecycle rollback tightening hard-dependency behavior onto optional dependencies.
func TestRunPluginPhaseAllowsMissingOptionalDependency(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-optional"
	registerLifecyclePlugin(t, pluginName, []string{"missing-optional"}, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{})
		if err != nil {
			return 1
		}
		return 0
	})

	s, spawner := newLifecycleStartupServer(t)
	err := s.runPluginPhase([]plugin.PluginConfig{{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON}})
	require.NoError(t, err)
	require.NotNil(t, spawner.pm)
	proc := spawner.pm.GetProcess(pluginName)
	require.NotNil(t, proc)
	assert.Equal(t, plugin.StageRunning, proc.Stage())

	proc.Stop()
	require.Eventually(t, func() bool { return !proc.Running() }, time.Second, 10*time.Millisecond)
}

// TestOnRegistrationRegistersPluginPipes verifies a pipe alias a plugin declares
// in its Stage 1 message reaches the alias registry the pipe resolver reads.
// TestOnRegistrationRefusesMalformedPluginPipe, below, is the refusal half.
//
// VALIDATES: the Wiring Test row "a plugin sends a pipes list in
// declare-registration" -> "onRegistration validates it and writes it to
// aliasRegistry".
// PREVENTS: the declaration type traveling the wire with nothing reading it,
// and a bad declaration reaching the registry.
func TestOnRegistrationRegistersPluginPipes(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-pipe-alias"
	const commandName = "show lifecycle pipes"
	const aliasName = "totals"
	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{Name: commandName}},
			Pipes: []sdk.PipeDecl{{
				Command:     commandName,
				Name:        aliasName,
				Description: "The counters alone",
				Expansion:   "display kind vrp-count",
			}},
		})
		if err != nil {
			return 1
		}
		return 0
	})

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON},
	}))

	aliases := command.AliasesForCommand(commandName)
	found := false
	for _, alias := range aliases {
		if alias.Name != aliasName {
			continue
		}
		found = true
		assert.Equal(t, "The counters alone", alias.Description)
		assert.Equal(t, "display kind vrp-count", alias.Expansion)
	}
	assert.True(t, found, "declared pipe alias missing from the registry: %v", aliases)
}

// TestOnRegistrationRefusesMalformedPluginPipe verifies the pipe declarations are
// validated in the position the doctor checks and enrichers are validated, which
// is before onRegistration converts anything. The plugin fails to start and its
// command never reaches the registry.
//
// VALIDATES: the same Wiring Test row, its refusal half.
// PREVENTS: a malformed declaration being converted and registered.
func TestOnRegistrationRefusesMalformedPluginPipe(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-pipe-alias-malformed"
	const commandName = "show lifecycle badpipe"
	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{Name: commandName}},
			Pipes: []sdk.PipeDecl{{
				Command:   commandName,
				Name:      "",
				Expansion: "display kind",
			}},
		})
		if err != nil {
			return 1
		}
		return 0
	})

	s, _ := newLifecycleStartupServer(t)
	err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), pluginName)
	assert.Empty(t, s.registry.LookupCommand(commandName))
	assert.Empty(t, command.AliasesForCommand(commandName))
}

// TestOnRegistrationRefusesPipeOnUndeclaredCommand verifies a plugin can name
// only a command path it declared in the same message. The alias sits on a
// command path, and a path the plugin did not declare belongs to somebody else.
//
// VALIDATES: AC-7. Stage 1 fails, and the error names the path.
// PREVENTS: a plugin claiming another owner's subtree. The check refuses what it
// cannot confirm, because a check that answers "allowed" on a path it cannot
// resolve hands the whole command tree to whoever asks for it.
func TestOnRegistrationRefusesPipeOnUndeclaredCommand(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-pipe-alias-undeclared"
	const declaredCommand = "show lifecycle owned"
	const claimedCommand = "show lifecycle borrowed"
	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{Name: declaredCommand}},
			Pipes: []sdk.PipeDecl{{
				Command:   claimedCommand,
				Name:      "totals",
				Expansion: "display kind",
			}},
		})
		if err != nil {
			return 1
		}
		return 0
	})

	s, _ := newLifecycleStartupServer(t)
	err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), pluginName)
	assert.Contains(t, err.Error(), claimedCommand)
	assert.Empty(t, s.registry.LookupCommand(declaredCommand))
	assert.Empty(t, command.AliasesForCommand(claimedCommand))
	assert.Empty(t, command.AliasesForCommand(declaredCommand))
}

// TestOnRegistrationRollsBackPipesOnLaterFailure verifies a Stage 1 that fails
// AFTER the pipe aliases are written leaves none of them in the registry. The
// family conflict is the failure, because it is the one that runs after the
// write and under the same lock.
//
// onRegistration is called directly, without the startup driver around it,
// because the driver's own rollback removes the aliases a moment later and
// would answer for this unwind. The two unwinds are not the same: the driver's
// runs once the whole tier has finished its handshake, and the plugins of one
// tier register concurrently, so a name this plugin no longer wants must be
// free for its neighbor before the tier ends.
//
// VALIDATES: A-5. A refused Stage 1 leaves no partial registration behind.
// PREVENTS: a plugin unable to start ever again. The alias registry refuses a
// name the exact command path already carries, so an alias that outlives the
// registration that wrote it refuses that same plugin its own name, and the
// operator reads a collision between a plugin and itself.
func TestOnRegistrationRollsBackPipesOnLaterFailure(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-pipe-alias-rollback"
	const commandName = "show lifecycle rollback pipes"
	t.Cleanup(func() { command.UnregisterPluginAliases(pluginName) })

	s, _ := newLifecycleStartupServer(t)
	proc := process.NewProcess(plugin.PluginConfig{Name: pluginName, Internal: true})
	sink := &engineStartupSink{s: s, proc: proc}

	err := sink.onRegistration(&rpc.DeclareRegistrationInput{
		Commands: []rpc.CommandDecl{{Name: commandName}},
		Pipes: []rpc.PipeDecl{{
			Command:     commandName,
			Name:        "totals",
			Description: "The counters alone",
			Expansion:   "display kind",
		}},
		// The conflict: the AFI number is IPv4's and the name is not.
		Families: []rpc.FamilyDecl{
			{Name: "not-ipv4/pipes", Mode: "both", AFI: uint16(family.AFIIPv4), SAFI: 203},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "family")
	assert.Empty(t, s.registry.LookupCommand(commandName))
	assert.Empty(t, command.AliasesForCommand(commandName),
		"a Stage 1 that failed after the pipe write left its alias behind")
}

// TestPluginPipesRemovedOnPluginStop verifies the aliases a plugin declared
// leave the registry when the plugin stops, and that the same plugin then
// starts again and declares them again. rollbackStartupProcess is the function
// under test: it is what a config reload calls to stop a plugin whose config
// the operator removed, and what a failed startup calls.
//
// VALIDATES: AC-9. The name stops resolving when the plugin stops, and the same
// plugin can start again and register it.
// PREVENTS: a plugin that can start exactly once. Nothing removed an alias when
// a plugin stopped, and the exact-path refusal then made the second start
// collide with the first start's registration.
func TestPluginPipesRemovedOnPluginStop(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-pipe-alias-restart"
	const commandName = "show lifecycle restart pipes"
	const aliasName = "totals"
	t.Cleanup(func() { command.UnregisterPluginAliases(pluginName) })

	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{Name: commandName}},
			Pipes: []sdk.PipeDecl{{
				Command:     commandName,
				Name:        aliasName,
				Description: "The counters alone",
				Expansion:   "display kind",
			}},
		})
		if err != nil {
			return 1
		}
		return 0
	})

	carriesAlias := func(t *testing.T) bool {
		t.Helper()
		for _, alias := range command.AliasesForCommand(commandName) {
			if alias.Name == aliasName {
				return true
			}
		}
		return false
	}

	s, spawner := newLifecycleStartupServer(t)
	configs := []plugin.PluginConfig{{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON}}

	require.NoError(t, s.runPluginPhase(configs))
	require.True(t, carriesAlias(t), "the first start did not register the declared alias")

	require.NotNil(t, spawner.pm)
	proc := spawner.pm.GetProcess(pluginName)
	require.NotNil(t, proc)
	s.rollbackStartupProcess(proc)

	assert.False(t, carriesAlias(t), "the alias outlived the plugin that declared it")
	assert.Empty(t, command.AliasesForCommand(commandName),
		"the command path still carries a declaration nobody serves")

	require.NoError(t, s.runPluginPhase(configs), "the plugin cannot start a second time")
	assert.True(t, carriesAlias(t), "the second start did not register the declared alias")
}
