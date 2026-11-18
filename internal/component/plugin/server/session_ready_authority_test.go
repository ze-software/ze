// Design: docs/architecture/api/process-protocol.md -- current Process declaration authority.
package server

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// External flags, including explicit absence, cannot borrow a compiled row.
// Replacement and stop change both predicates; real Internal defaults survive.
func TestExternalReadinessAndFenceUseCurrentProcess(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const silent = "same-name-compiled-silent"
	const reporting = "same-name-compiled-reporting"
	for _, item := range []struct {
		name  string
		ready bool
	}{{silent, false}, {reporting, true}} {
		require.NoError(t, registry.Register(registry.Registration{Name: item.name,
			SignalsSessionReady: item.ready, FencesLiveForwards: item.ready,
			RunEngine: func(net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 }}))
	}
	host, err := NewServer(&ServerConfig{ConfigNeedHolds: holdsForEmptyConfig}, nil)
	require.NoError(t, err)
	require.NoError(t, host.StartWithContext(t.Context()))
	t.Cleanup(host.Stop)
	t.Cleanup(func() { registry.SetRuntimeSessionReady(nil); registry.SetRuntimeReplayFence(nil) })
	manager := process.NewProcessManager(nil)
	host.procManager.Store(manager)
	original := process.NewProcess(plugin.PluginConfig{Name: silent, Run: "/external/readiness"})
	original.SetRunning(true)
	original.SetRegistration(&plugin.PluginRegistration{Done: true, SignalsSessionReady: true, FencesLiveForwards: true})
	t.Cleanup(original.Stop)
	require.NoError(t, manager.AddProcess(silent, original))
	require.True(t, registry.SignalsSessionReady(silent))
	require.True(t, registry.FencesLiveForwards(silent))
	original.SetRunning(false)
	require.False(t, registry.SignalsSessionReady(silent), "an ended external process is no current reporter")
	require.False(t, registry.FencesLiveForwards(silent))
	original.SetRunning(true)
	replacement := process.NewProcess(plugin.PluginConfig{Name: silent, Run: "/external/replacement"})
	replacement.SetRunning(true)
	replacement.SetRegistration(&plugin.PluginRegistration{Done: true})
	t.Cleanup(replacement.Stop)
	require.NoError(t, manager.AddProcess(silent, replacement))
	require.True(t, original.Running(), "absence must come from the current replacement, not cancellation of the old record")
	require.False(t, registry.SignalsSessionReady(silent))
	require.False(t, registry.FencesLiveForwards(silent))
	manager.RemoveProcess(silent)
	require.False(t, registry.SignalsSessionReady(silent))
	require.False(t, registry.FencesLiveForwards(silent))
	external := process.NewProcess(plugin.PluginConfig{Name: reporting, Run: "/external/undeclared"})
	external.SetRunning(true)
	external.SetRegistration(&plugin.PluginRegistration{Done: true})
	t.Cleanup(external.Stop)
	require.NoError(t, manager.AddProcess(reporting, external))
	require.False(t, registry.SignalsSessionReady(reporting), "external false cannot borrow same-named compiled true")
	require.False(t, registry.FencesLiveForwards(reporting))
	internal := process.NewProcess(plugin.PluginConfig{Name: "internal-alias", Run: "ze." + reporting, Internal: true})
	internal.SetRunning(true)
	internal.SetRegistration(&plugin.PluginRegistration{Done: true})
	t.Cleanup(internal.Stop)
	require.NoError(t, manager.AddProcess(internal.Name(), internal))
	require.True(t, registry.SignalsSessionReady(internal.Name()), "a real Internal runner keeps its compiled default")
	require.True(t, registry.FencesLiveForwards(internal.Name()))
	host.Stop()
	require.False(t, host.declaresSessionReady(internal.Name()))
	require.False(t, host.declaresReplayFence(internal.Name()))
}
