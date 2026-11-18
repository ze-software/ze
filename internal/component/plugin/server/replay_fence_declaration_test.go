// Design: docs/architecture/api/process-protocol.md -- external replay-fence opt-in.
package server

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The authenticated Stage-1 value controls a genuine external program. Removing
// it and replacing it under the same name must not reuse that value. A compiled
// row with the same process name cannot lend a new external process its fence.
func TestExternalReplayFenceCurrentProcessLifetime(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	host, err := NewServer(&ServerConfig{ConfigNeedHolds: holdsForEmptyConfig}, nil)
	require.NoError(t, err)
	require.NoError(t, host.StartWithContext(t.Context()))
	t.Cleanup(host.Stop)
	t.Cleanup(func() { registry.SetRuntimeReplayFence(nil) })
	manager := process.NewProcessManager(nil)
	host.procManager.Store(manager)
	const name = "third-party-external-replay-fence"
	require.False(t, registry.Has(name))
	var declaration rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal([]byte(`{"signals-session-ready":true,"fences-live-forwards":true}`), &declaration))
	original := process.NewProcess(plugin.PluginConfig{Name: name, Run: "/third-party/fence-advertiser"})
	original.SetRunning(true)
	original.SetRegistration(registrationFromRPC(&declaration))
	require.NoError(t, manager.AddProcess(name, original))
	require.True(t, registry.FencesLiveForwards(name), "the current external declaration must raise the fence")
	manager.RemoveProcess(name)
	original.Stop()
	require.False(t, registry.FencesLiveForwards(name), "removed admission cannot survive by name")
	require.NoError(t, registry.Register(registry.Registration{Name: name, CLIHandler: func([]string) int { return 0 }, RunEngine: func(net.Conn) int { return 0 }, SignalsSessionReady: true, FencesLiveForwards: true}))
	replacement := process.NewProcess(plugin.PluginConfig{Name: name, Run: "/third-party/without-fence"})
	replacement.SetRunning(true)
	replacement.SetRegistration(&plugin.PluginRegistration{Done: true, SignalsSessionReady: true})
	require.NoError(t, manager.AddProcess(name, replacement))
	require.False(t, registry.FencesLiveForwards(name), "an external binary must not borrow a same-named compiled default")
	host.Stop()
	require.False(t, host.declaresReplayFence(name), "the stopped server must expose no remaining fence declaration")
}

// The existing compiled declaration remains a default for a real built-in
// runner attached under an internal alias. Forked built-ins declare at Stage 1.
func TestRuntimeReplayFencePreservesCompiledAliasDefault(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const implementation = "compiled-replay-fence-default"
	require.NoError(t, registry.Register(registry.Registration{Name: implementation, CLIHandler: func([]string) int { return 0 }, RunEngine: func(net.Conn) int { return 0 }, SignalsSessionReady: true, FencesLiveForwards: true}))
	host, err := NewServer(&ServerConfig{ConfigNeedHolds: holdsForEmptyConfig}, nil)
	require.NoError(t, err)
	require.NoError(t, host.StartWithContext(t.Context()))
	t.Cleanup(host.Stop)
	t.Cleanup(func() { registry.SetRuntimeReplayFence(nil) })
	manager := process.NewProcessManager(nil)
	host.procManager.Store(manager)
	proc := process.NewProcess(plugin.PluginConfig{Name: "compiled-alias", Run: "ze." + implementation, Internal: true})
	proc.SetRunning(true)
	proc.SetRegistration(&plugin.PluginRegistration{Done: true})
	require.NoError(t, manager.AddProcess(proc.Name(), proc))
	require.True(t, registry.FencesLiveForwards(proc.Name()))
	proc.SetRunning(false)
	require.False(t, registry.FencesLiveForwards(proc.Name()), "a dead runner's metadata cannot hold a new session")
}
