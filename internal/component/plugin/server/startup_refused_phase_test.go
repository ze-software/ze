package server

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// refusingPhaseSpawner refuses every phase with cause and answers the process
// manager it recorded the refused plugin down in, as the plugin manager does
// for a plugin that sent no declaration before the config was parsed.
type refusingPhaseSpawner struct {
	pm     *process.ProcessManager
	cause  error
	sealed atomic.Bool
}

func (s *refusingPhaseSpawner) SpawnMore([]plugin.PluginConfig) error {
	if s.sealed.Load() {
		return process.ErrDaemonStopping
	}
	return s.cause
}

func (s *refusingPhaseSpawner) SealProcessAdmission() func() {
	s.sealed.Store(true)
	return func() {}
}

func (s *refusingPhaseSpawner) GetProcessManager() any { return s.pm }

// TestRefusedPhasePublishesItsStopRecord proves the stop record a spawner
// writes when it refuses a phase reaches the doctor snapshot: the server
// publishes the spawner's process manager although the phase did not run, so
// the snapshot names the plugin as configured and down with its cause. With
// no publish the snapshot answers ErrDoctorSnapshotNotStarted and the
// doctor-plugin-stopped check cannot name the plugin.
func TestRefusedPhasePublishesItsStopRecord(t *testing.T) {
	const name = "silent"
	config := plugin.PluginConfig{Name: name, Run: "silent-plugin"}
	cause := errors.New(`external plugin "silent" connected and sent no declaration within 5s, so it was stopped`)

	s, _ := newLifecycleStartupServer(t)
	pm := process.NewProcessManager([]plugin.PluginConfig{config})
	pm.RecordStopped(name, process.PluginStop{Cause: process.StopHandshakeFailed, Detail: cause.Error()})
	// Registered after the server's own cleanup, so it runs first: the
	// process manager was never started and is not the server's to stop, and
	// the removal clears the warning the record raised on the report bus.
	t.Cleanup(func() {
		s.procManager.Store(nil)
		pm.RemoveProcess(name)
	})
	s.SetProcessSpawner(&refusingPhaseSpawner{pm: pm, cause: cause})

	err := s.runPluginPhase([]plugin.PluginConfig{config})

	require.ErrorIs(t, err, cause)
	snapshot, snapshotErr := s.doctorSnapshot()
	require.NoError(t, snapshotErr, "the refused phase published the process manager that holds the record")
	require.Len(t, snapshot.Plugins, 1)
	state := snapshot.Plugins[0]
	assert.Equal(t, name, state.Name)
	assert.True(t, state.Configured)
	assert.True(t, state.Down)
	assert.False(t, state.Running)
	assert.Equal(t, process.StopHandshakeFailed, state.Stop.Cause)
	assert.Equal(t, cause.Error(), state.Stop.Detail)
}
