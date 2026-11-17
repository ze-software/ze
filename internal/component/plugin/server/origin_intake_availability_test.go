// Design: docs/architecture/api/process-protocol.md -- configured owner availability.
package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

func TestOriginIntakeConfiguredOwnerRequiresDeclaredLiveProcess(t *testing.T) {
	host := &Server{ctx: t.Context()}
	_, _, err := host.OriginIntakeOwner("configured-owner")
	require.Error(t, err, "missing process manager is unknown capability, not legacy support")
	manager := process.NewProcessManager(nil)
	host.procManager.Store(manager)
	_, _, err = host.OriginIntakeOwner("configured-owner")
	require.Error(t, err, "absent configured owner is not legacy support")
	proc := selectedCurrentProcess(t, host)
	require.NoError(t, manager.AddProcess(proc.Name(), proc))
	proc.SetStage(plugin.StageRunning)
	_, _, err = host.OriginIntakeOwner(proc.Name())
	require.Error(t, err, "an empty pre-Stage1 registration cannot declare legacy support")
	proc.SetRegistration(&plugin.PluginRegistration{Done: true})
	_, supported, err := host.OriginIntakeOwner(proc.Name())
	require.NoError(t, err)
	require.False(t, supported, "actual completed registration may explicitly lack intake")
	proc.SetRegistration(&plugin.PluginRegistration{Done: true, OriginIntake: true})
	proc.SetStage(plugin.StageConfig)
	_, _, err = host.OriginIntakeOwner(proc.Name())
	require.Error(t, err, "declared but not ready cannot be bypassed")
}
