package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
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

// TestValidateShapeDecls verifies every refusal the answer-shape declarations
// owe, before any of them reaches a registry. The declarations arrive from
// another process, so each one is checked against a closed set or a bound.
//
// VALIDATES: AC-2, AC-3, AC-4, AC-5.
// PREVENTS: a plugin's typo publishing the operators of a document and refusing
// the row operators the command supports, and an unbounded list from another
// process being stored.
func TestValidateShapeDecls(t *testing.T) {
	longName := strings.Repeat("a", 65)
	manyColumns := make([]string, 65)
	for i := range manyColumns {
		manyColumns[i] = "column"
	}
	manyAddressFields := make([]string, 17)
	for i := range manyAddressFields {
		manyAddressFields[i] = "address"
	}

	cases := []struct {
		name     string
		commands []rpc.CommandDecl
		refuses  []string
	}{{
		name:     "no declaration at all",
		commands: []rpc.CommandDecl{{Name: "show plain"}},
	}, {
		name: "a shape, a column order and an address field",
		commands: []rpc.CommandDecl{{
			Name:          "show declared",
			Shape:         "tab",
			Columns:       []string{"address", "state"},
			AddressFields: []string{"address"},
		}},
	}, {
		name:     "the last valid column count",
		commands: []rpc.CommandDecl{{Name: "show declared", Shape: "tab", Columns: manyColumns[:64]}},
	}, {
		name: "the last valid address-field count",
		commands: []rpc.CommandDecl{{
			Name:          "show declared",
			Shape:         "tab",
			AddressFields: manyAddressFields[:16],
		}},
	}, {
		name:     "the last valid name length",
		commands: []rpc.CommandDecl{{Name: "show declared", Shape: "tab", Columns: []string{longName[:64]}}},
	}, {
		name:     "a spelling no shape writes",
		commands: []rpc.CommandDecl{{Name: "show declared", Shape: "table"}},
		refuses:  []string{"show declared", "table"},
	}, {
		name:     "a capitalized spelling",
		commands: []rpc.CommandDecl{{Name: "show declared", Shape: "Doc"}},
		refuses:  []string{"show declared", "Doc"},
	}, {
		name:     "a column order with no shape",
		commands: []rpc.CommandDecl{{Name: "show declared", Columns: []string{"address"}}},
		refuses:  []string{"show declared", "shape"},
	}, {
		name:     "an address-field list with no shape",
		commands: []rpc.CommandDecl{{Name: "show declared", AddressFields: []string{"address"}}},
		refuses:  []string{"show declared", "shape"},
	}, {
		name:     "a shape on a path the message does not declare",
		commands: []rpc.CommandDecl{{Name: "   ", Shape: "tab"}},
		refuses:  []string{"command path"},
	}, {
		name:     "one column past the bound",
		commands: []rpc.CommandDecl{{Name: "show declared", Shape: "tab", Columns: manyColumns}},
		refuses:  []string{"show declared", "65", "64"},
	}, {
		name: "one address field past the bound",
		commands: []rpc.CommandDecl{{
			Name:          "show declared",
			Shape:         "tab",
			AddressFields: manyAddressFields,
		}},
		refuses: []string{"show declared", "17", "16"},
	}, {
		name:     "a column name past the bound",
		commands: []rpc.CommandDecl{{Name: "show declared", Shape: "tab", Columns: []string{longName}}},
		refuses:  []string{"show declared", "64"},
	}, {
		name: "an address-field name past the bound",
		commands: []rpc.CommandDecl{{
			Name:          "show declared",
			Shape:         "tab",
			AddressFields: []string{longName},
		}},
		refuses: []string{"show declared", "64"},
	}, {
		name:     "a column name of nothing",
		commands: []rpc.CommandDecl{{Name: "show declared", Shape: "tab", Columns: []string{" "}}},
		refuses:  []string{"show declared", "column"},
	}, {
		name: "an address-field name of nothing",
		commands: []rpc.CommandDecl{{
			Name:          "show declared",
			Shape:         "tab",
			AddressFields: []string{""},
		}},
		refuses: []string{"show declared", "address field"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateShapeDecls(tc.commands)
			if len(tc.refuses) == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, word := range tc.refuses {
				assert.Contains(t, err.Error(), word)
			}
		})
	}
}

// TestValidateShapeDeclsClampsTheValueItReports verifies a plugin string reaches
// an error message, and the daemon log behind it, clamped.
//
// VALIDATES: the Security Review row on error leakage.
// PREVENTS: an unbounded string from another process being mirrored into the
// daemon log, which is the log the operator reads and the disk it fills.
func TestValidateShapeDeclsClampsTheValueItReports(t *testing.T) {
	err := validateShapeDecls([]rpc.CommandDecl{{
		Name:  strings.Repeat("z", 4096),
		Shape: strings.Repeat("q", 4096),
	}})
	require.Error(t, err)
	assert.Less(t, len(err.Error()), 512, "the refusal mirrors the plugin's string: %q", err.Error())
}

// TestRegisterPluginShapes verifies the three declarations a plugin sends reach
// the three registries, through the whole Stage 1 handshake.
//
// VALIDATES: AC-1.
// PREVENTS: a channel that validates a declaration and stores none of it, which
// leaves every plugin command deriving its shape from the payload in hand.
func TestRegisterPluginShapes(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-shape"
	const commandName = "show lifecycle shape"
	t.Cleanup(func() { command.UnregisterPluginShapes(pluginName) })

	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{
				Name:          commandName,
				Shape:         "tab",
				Columns:       []string{"address", "state"},
				AddressFields: []string{"address"},
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

	shape, declared := command.ShapeForCommand(commandName)
	assert.True(t, declared, "the command declares no shape")
	assert.Equal(t, command.ShapeTab, shape)
	assert.Equal(t, []command.ColumnOrder{{"address", "state"}}, command.ColumnsForCommand(commandName))
	assert.Equal(t, []string{"address"}, command.AddressFieldsForCommand(commandName))
}

// TestPluginShapeDoesNotInheritItsParentsFields verifies a plugin command that
// declares a shape and names no column and no address field resolves to
// neither, rather than to the ones the plugin declared on the path above it.
//
// VALIDATES: AC-1, the half that says a declaration describes ONE command's
// answer.
// PREVENTS: `show x rows` reading its rows against the column order of
// `show x`, and admitting the address operators on an answer holding no
// address, because a command that declares nothing resolves to the nearest
// declared ancestor.
func TestPluginShapeDoesNotInheritItsParentsFields(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-shape-barrier"
	const parentCommand = "show lifecycle barrier"
	const childCommand = "show lifecycle barrier status"
	t.Cleanup(func() { command.UnregisterPluginShapes(pluginName) })

	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{
				Name:          parentCommand,
				Shape:         "tab",
				Columns:       []string{"address", "state"},
				AddressFields: []string{"address"},
			}, {
				Name:  childCommand,
				Shape: "doc",
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

	shape, declared := command.ShapeForCommand(childCommand)
	require.True(t, declared)
	assert.Equal(t, command.ShapeDoc, shape)
	assert.Empty(t, command.ColumnsForCommand(childCommand),
		"the child reads its answer against the column order of its parent")
	assert.Empty(t, command.AddressFieldsForCommand(childCommand),
		"the child admits the address operators on its parent's address field")

	assert.Equal(t, []command.ColumnOrder{{"address", "state"}}, command.ColumnsForCommand(parentCommand))
	assert.Equal(t, []string{"address"}, command.AddressFieldsForCommand(parentCommand))
}

// TestPluginShapeOverridesEmptyDeclaration verifies a plugin's declaration lands
// on a path an in-tree package declared EMPTY, which is what every direct child
// of `show bgp` carries.
//
// VALIDATES: AC-6, and assumption A-1.
// PREVENTS: the eleven `show bgp` plugin commands declaring into a path whose
// empty declaration silently wins, so the channel reaches none of them.
func TestPluginShapeOverridesEmptyDeclaration(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-shape-floor"
	const parentCommand = "show lifecycle floor"
	const commandName = "show lifecycle floor child"
	t.Cleanup(func() { command.UnregisterPluginShapes(pluginName) })

	// The in-tree half, as the BGP command plugin writes it: a shape on the
	// parent, and an empty declaration on the child that stops the child
	// inheriting it.
	command.RegisterShape([]string{parentCommand}, command.ShapeMap)
	command.RegisterShape([]string{commandName})
	command.RegisterColumns([]string{parentCommand}, command.ColumnOrder{"peer", "uptime"})
	command.RegisterColumns([]string{commandName})

	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{
				Name:    commandName,
				Shape:   "tab",
				Columns: []string{"address", "port"},
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

	shape, declared := command.ShapeForCommand(commandName)
	assert.True(t, declared, "the empty declaration kept the plugin's shape out")
	assert.Equal(t, command.ShapeTab, shape)
	assert.Equal(t, []command.ColumnOrder{{"address", "port"}}, command.ColumnsForCommand(commandName))
}

// TestUnregisterPluginShapes verifies a stopped plugin's declarations leave, and
// that each path returns to what it held BEFORE the plugin declared: the empty
// declaration an in-tree package wrote, not nothing.
//
// VALIDATES: AC-7, and assumption A-3.
// PREVENTS: removal restoring inheritance the in-tree declaration exists to
// stop, which would make the child answer its parent's shape and its parent's
// columns once the plugin stops.
func TestUnregisterPluginShapes(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-shape-restart"
	const parentCommand = "show lifecycle restart shape"
	const commandName = "show lifecycle restart shape child"
	t.Cleanup(func() { command.UnregisterPluginShapes(pluginName) })

	command.RegisterShape([]string{parentCommand}, command.ShapeMap)
	command.RegisterShape([]string{commandName})

	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{
				Name:          commandName,
				Shape:         "tab",
				Columns:       []string{"address"},
				AddressFields: []string{"address"},
			}},
		})
		if err != nil {
			return 1
		}
		return 0
	})

	s, spawner := newLifecycleStartupServer(t)
	configs := []plugin.PluginConfig{{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON}}
	require.NoError(t, s.runPluginPhase(configs))

	shape, declared := command.ShapeForCommand(commandName)
	require.True(t, declared)
	require.Equal(t, command.ShapeTab, shape)

	require.NotNil(t, spawner.pm)
	proc := spawner.pm.GetProcess(pluginName)
	require.NotNil(t, proc)
	s.rollbackStartupProcess(proc)

	shape, declared = command.ShapeForCommand(commandName)
	assert.False(t, declared, "the shape outlived the plugin that declared it")
	assert.Equal(t, command.ShapeDoc, shape)
	assert.Empty(t, command.ColumnsForCommand(commandName),
		"the column order outlived the plugin, or the path fell back to its parent")
	assert.Empty(t, command.AddressFieldsForCommand(commandName))

	// The parent still declares, which is what makes the assertions above a
	// statement about the EMPTY declaration rather than about an empty registry.
	parentShape, parentDeclared := command.ShapeForCommand(parentCommand)
	assert.True(t, parentDeclared)
	assert.Equal(t, command.ShapeMap, parentShape)

	require.NoError(t, s.runPluginPhase(configs), "the plugin cannot start a second time")
	shape, declared = command.ShapeForCommand(commandName)
	assert.True(t, declared, "the second start did not declare the shape again")
	assert.Equal(t, command.ShapeTab, shape)
}

// TestShapeWriteUnwindsWithStageOne verifies a Stage 1 that fails AFTER the
// shapes are written leaves none of them behind. The family conflict is the
// failure, because it is the one that runs after the write and under the same
// lock.
//
// onRegistration is called directly, for the reason
// TestOnRegistrationRollsBackPipesOnLaterFailure states: the driver's own
// rollback would answer for this unwind a moment later.
//
// VALIDATES: AC-8.
// PREVENTS: a refused plugin leaving a shape on a command path the daemon
// serves itself, which would publish operators nobody can satisfy.
func TestShapeWriteUnwindsWithStageOne(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-shape-rollback"
	const commandName = "show lifecycle rollback shape"
	t.Cleanup(func() { command.UnregisterPluginShapes(pluginName) })

	s, _ := newLifecycleStartupServer(t)
	proc := process.NewProcess(plugin.PluginConfig{Name: pluginName, Internal: true})
	sink := &engineStartupSink{s: s, proc: proc}

	err := sink.onRegistration(&rpc.DeclareRegistrationInput{
		Commands: []rpc.CommandDecl{{
			Name:          commandName,
			Shape:         "tab",
			Columns:       []string{"address"},
			AddressFields: []string{"address"},
		}},
		// The conflict: the AFI number is IPv4's and the name is not.
		Families: []rpc.FamilyDecl{
			{Name: "not-ipv4/shapes", Mode: "both", AFI: uint16(family.AFIIPv4), SAFI: 204},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "family")

	_, declared := command.ShapeForCommand(commandName)
	assert.False(t, declared, "a Stage 1 that failed after the shape write left the shape behind")
	assert.Empty(t, command.ColumnsForCommand(commandName))
	assert.Empty(t, command.AddressFieldsForCommand(commandName))
}

// TestOnRegistrationRefusesConflictingShapeDeclaration verifies a plugin
// declaring a shape a path already carries is REFUSED, and that the refusal is
// an error rather than a panic.
//
// declarationRegistry.declare panics on that conflict, which is right for a
// table written in init(): only a Ze defect reaches it. This declaration
// arrived over a socket, so the same conflict is an operating error and the
// daemon MUST stay up (docs/contributing/ze-go-style.md).
//
// VALIDATES: the Critical Review row "a bad plugin message is REFUSED and never
// panics".
// PREVENTS: a plugin process taking the daemon down with one string.
func TestOnRegistrationRefusesConflictingShapeDeclaration(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-shape-conflict"
	const commandName = "show lifecycle conflict shape"
	t.Cleanup(func() { command.UnregisterPluginShapes(pluginName) })

	// The in-tree declaration the plugin will contradict.
	command.RegisterShape([]string{commandName}, command.ShapeDoc)

	s, _ := newLifecycleStartupServer(t)
	proc := process.NewProcess(plugin.PluginConfig{Name: pluginName, Internal: true})
	sink := &engineStartupSink{s: s, proc: proc}

	var err error
	require.NotPanics(t, func() {
		err = sink.onRegistration(&rpc.DeclareRegistrationInput{
			Commands: []rpc.CommandDecl{{Name: commandName, Shape: "tab"}},
		})
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), pluginName)
	assert.Contains(t, err.Error(), commandName)

	shape, declared := command.ShapeForCommand(commandName)
	assert.True(t, declared)
	assert.Equal(t, command.ShapeDoc, shape, "the refused declaration replaced the one the path held")
	assert.Empty(t, s.registry.LookupCommand(commandName),
		"the refused plugin left its registry row behind")
}

// TestPluginHelpDeclarationReachesTheCommandTree proves a plugin's two help
// texts cross the process boundary and arrive at the two fields the command
// tree reads.
//
// The method is the whole Stage 1 handshake: a plugin declares a summary and an
// explanation, and the test reads what the registry holds and what
// MergeCommandPaths writes into the tree the completer and the help page read.
//
// VALIDATES: AC-7, the carrying side.
// PREVENTS: a boundary that validates both texts and stores one of them, which
// leaves every plugin command with the explanation the plugin never sent.
func TestPluginHelpDeclarationReachesTheCommandTree(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-help"
	const commandName = "show lifecycle help"
	const summary = "Show the lifecycle help fixture."
	const explanation = "The fixture declares two help texts.\nThis is the second line of the explanation."

	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{
				Name:        commandName,
				Description: summary,
				LongHelp:    explanation,
			}},
		})
		if err != nil {
			return 1
		}
		return 0
	})

	s, _ := newLifecycleStartupServer(t)
	if err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON},
	}); err != nil {
		t.Fatalf("plugin phase: %v", err)
	}

	registered := s.dispatcher.Registry().Lookup(commandName)
	if registered == nil {
		t.Fatalf("the registry holds no %q", commandName)
	}
	if registered.Description != summary {
		t.Errorf("registered summary = %q, want %q", registered.Description, summary)
	}
	if registered.LongHelp != explanation {
		t.Errorf("registered explanation = %q, want %q", registered.LongHelp, explanation)
	}

	tree := &command.Node{Children: map[string]*command.Node{}}
	command.MergeCommandPaths(tree, s.dispatcher.Registry().VisibleCommandEntries())
	node := tree.Children["show"].Children["lifecycle"].Children["help"]
	if node.Description != summary {
		t.Errorf("tree summary = %q, want %q", node.Description, summary)
	}
	if node.LongHelp != explanation {
		t.Errorf("tree explanation = %q, want %q", node.LongHelp, explanation)
	}
}

// TestPluginWithNoHelpDeclarationKeepsItsSummary proves the zero value of the
// explanation is the refusal all the way through the boundary, not a blank
// summary.
//
// The method is the declaration a plugin compiled before `long-help` existed
// sends: a summary and nothing else.
//
// VALIDATES: AC-7, the zero-value half.
// PREVENTS: the failure recorded in plan/journal/field-carries-two-meanings.md,
// where the second field on a cross-process contract empties the first one for
// every peer that predates it.
func TestPluginWithNoHelpDeclarationKeepsItsSummary(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "lifecycle-help-absent"
	const commandName = "show lifecycle silent"
	const summary = "Show the fixture that declares no explanation."

	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		err := p.Run(context.Background(), sdk.Registration{
			Commands: []sdk.CommandDecl{{
				Name:        commandName,
				Description: summary,
			}},
		})
		if err != nil {
			return 1
		}
		return 0
	})

	s, _ := newLifecycleStartupServer(t)
	if err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON},
	}); err != nil {
		t.Fatalf("plugin phase: %v", err)
	}

	registered := s.dispatcher.Registry().Lookup(commandName)
	if registered == nil {
		t.Fatalf("the registry holds no %q", commandName)
	}
	if registered.Description != summary {
		t.Errorf("registered summary = %q, want the summary the plugin sent", registered.Description)
	}
	if registered.LongHelp != "" {
		t.Errorf("registered explanation = %q, want empty", registered.LongHelp)
	}

	tree := &command.Node{Children: map[string]*command.Node{}}
	command.MergeCommandPaths(tree, s.dispatcher.Registry().VisibleCommandEntries())
	node := tree.Children["show"].Children["lifecycle"].Children["silent"]
	if node.Description != summary {
		t.Errorf("tree summary = %q, want the summary the plugin sent", node.Description)
	}
	if node.LongHelp != "" {
		t.Errorf("tree explanation = %q, want empty", node.LongHelp)
	}
}

// TestValidateHelpDecls checks the bound and the control-character refusal on
// each of the two declared help texts.
//
// The summary is one line, so every control character is refused there. The
// explanation is a paragraph, so it keeps a newline and refuses the rest.
//
// VALIDATES: the Security Review rows for this spec: a plugin-supplied string
// reaches a terminal, an HTML page and a JSON document.
// PREVENTS: an unbounded declaration in the daemon's memory and its log, and an
// ANSI escape or a tab written into the one-line completion format.
func TestValidateHelpDecls(t *testing.T) {
	cases := []struct {
		name    string
		decl    rpc.CommandDecl
		wantErr string
	}{
		{name: "both empty", decl: rpc.CommandDecl{Name: "show x"}},
		{name: "both present", decl: rpc.CommandDecl{Name: "show x", Description: "Show x.", LongHelp: "One line.\nAnother line."}},
		{name: "summary at the bound", decl: rpc.CommandDecl{Name: "show x", Description: strings.Repeat("a", 256)}},
		{name: "summary past the bound", decl: rpc.CommandDecl{Name: "show x", Description: strings.Repeat("a", 257)}, wantErr: "257 bytes (max 256)"},
		{name: "summary with a newline", decl: rpc.CommandDecl{Name: "show x", Description: "Show x.\nAnd more."}, wantErr: "control character 0x0a at byte 7"},
		{name: "summary with a tab", decl: rpc.CommandDecl{Name: "show x", Description: "Show\tx."}, wantErr: "control character 0x09 at byte 4"},
		{name: "summary with an escape", decl: rpc.CommandDecl{Name: "show x", Description: "Show \x1b[31mx."}, wantErr: "control character 0x1b at byte 5"},
		{name: "explanation at the bound", decl: rpc.CommandDecl{Name: "show x", LongHelp: strings.Repeat("a", 4096)}},
		{name: "explanation past the bound", decl: rpc.CommandDecl{Name: "show x", LongHelp: strings.Repeat("a", 4097)}, wantErr: "4097 bytes (max 4096)"},
		{name: "explanation with an escape", decl: rpc.CommandDecl{Name: "show x", LongHelp: "Line.\n\x1b[31mLine."}, wantErr: "control character 0x1b at byte 6"},
		{name: "explanation with a delete", decl: rpc.CommandDecl{Name: "show x", LongHelp: "Line.\x7f"}, wantErr: "control character 0x7f at byte 5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHelpDecls([]rpc.CommandDecl{tc.decl})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateHelpDecls = %v, want no error", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateHelpDecls accepted the declaration, want %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("validateHelpDecls = %q, want it to name %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// TestValidateHelpDeclsRefusesTheRetiredKey drives the retired-key guard from
// the function a plugin registration reaches. `help` named the summary on Ze's
// own daemon-to-CLI answer until 2026-09-03, so a plugin author who copied that
// spelling declares a command whose Description decodes EMPTY, which no reader
// can tell from a command that states no summary.
//
// The empty case is the one that matters: `"help": ""` and an absent key both
// leave the Go field at its zero value, and only the json.RawMessage shape
// tells them apart. A guard that missed it would let the wrong spelling through
// whenever the plugin author sent an empty string.
//
// VALIDATES: AC-10 -- a Stage 1 CommandDecl carrying a retired key is refused,
// and the error names the retired key and its replacement.
// PREVENTS: every command of that plugin rendering with no summary, silently.
func TestValidateHelpDeclsRefusesTheRetiredKey(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{name: "retired key with a value", payload: `{"name":"show x","help":"Show x."}`},
		{name: "retired key with an empty value", payload: `{"name":"show x","help":""}`},
		{name: "retired key beside the current one", payload: `{"name":"show x","description":"Show x.","help":"Show x."}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var decl rpc.CommandDecl
			require.NoError(t, json.Unmarshal([]byte(tc.payload), &decl))

			err := validateHelpDecls([]rpc.CommandDecl{decl})
			require.Error(t, err, "the retired key was accepted")
			assert.Contains(t, err.Error(), retiredSummaryKey, "the refusal must name the retired key")
			assert.Contains(t, err.Error(), summaryKey, "the refusal must name its replacement")
			assert.Contains(t, err.Error(), "show x", "the refusal must name the command")
		})
	}
}

// TestValidateHelpDeclsAcceptsADeclarationWithNoRetiredKey is the negative half:
// without it a guard that refused every declaration would pass the test above.
func TestValidateHelpDeclsAcceptsADeclarationWithNoRetiredKey(t *testing.T) {
	var decl rpc.CommandDecl
	require.NoError(t, json.Unmarshal([]byte(`{"name":"show x","description":"Show x.","long-help":"The explanation."}`), &decl))

	require.NoError(t, validateHelpDecls([]rpc.CommandDecl{decl}))
	assert.Equal(t, "Show x.", decl.Description)
	assert.Nil(t, decl.RetiredHelp, "an absent retired key leaves the field nil")
}

// TestValidatePipeDeclsBoundsTheDescription checks the alias summary is held to
// the same one-line rule as a command summary, because completion writes both
// into the same tab-separated format.
//
// VALIDATES: the Security Review row on the shell-completion format.
// PREVENTS: one alias with a newline in its description breaking every
// completion row that follows it.
func TestValidatePipeDeclsBoundsTheDescription(t *testing.T) {
	commands := []rpc.CommandDecl{{Name: "show x"}}

	err := validatePipeDecls([]rpc.PipeDecl{{
		Command: "show x", Name: "best", Expansion: "match state best",
		Description: "Best routes.\nAnd more.",
	}}, commands)
	if err == nil {
		t.Fatal("a pipe alias description with a newline was accepted")
	}
	if !strings.Contains(err.Error(), "control character 0x0a") {
		t.Errorf("refusal = %q, want it to name the control character", err.Error())
	}

	err = validatePipeDecls([]rpc.PipeDecl{{
		Command: "show x", Name: "best", Expansion: "match state best",
		Description: strings.Repeat("a", 257),
	}}, commands)
	if err == nil {
		t.Fatal("a pipe alias description past the bound was accepted")
	}
	if !strings.Contains(err.Error(), "257 bytes (max 256)") {
		t.Errorf("refusal = %q, want it to name the bound", err.Error())
	}
}
