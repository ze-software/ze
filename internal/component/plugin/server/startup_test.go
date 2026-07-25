package server

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
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
