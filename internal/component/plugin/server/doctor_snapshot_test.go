package server

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// TestPluginSnapshotNotPublished proves a runtime check reading plugin state
// with no server running is told so, never handed an empty list that reads as
// "every plugin is fine". Method: read with nothing published, then with a
// server published that holds no process manager, then with one whose manager
// holds a configured plugin that has no process; then withdraw and read again.
//
// VALIDATES: AC-5 (spec-doctor-coverage-12-plugin-infra).
// PREVENTS: a doctor run in a process with no plugin server reporting a
// healthy plugin set.
func TestPluginSnapshotNotPublished(t *testing.T) {
	previous := doctorSnapshotSource.Swap(nil)
	t.Cleanup(func() { doctorSnapshotSource.Store(previous) })

	if snapshot, err := PluginDoctorSnapshot(); !errors.Is(err, ErrDoctorSnapshotNotPublished) {
		t.Fatalf("PluginDoctorSnapshot with no server = %+v, %v; want ErrDoctorSnapshotNotPublished", snapshot, err)
	}

	s, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	publishDoctorSnapshot(s)
	if _, err := PluginDoctorSnapshot(); !errors.Is(err, ErrDoctorSnapshotNotStarted) {
		t.Fatalf("PluginDoctorSnapshot before plugin startup = %v; want ErrDoctorSnapshotNotStarted", err)
	}

	pm := process.NewProcessManager([]plugin.PluginConfig{{Name: "never-spawned", Internal: true}})
	s.procManager.Store(pm)
	snapshot, err := PluginDoctorSnapshot()
	require.NoError(t, err)
	require.Len(t, snapshot.Plugins, 1, "a configured plugin with no process must still be listed")
	got := snapshot.Plugins[0]
	want := PluginDoctorState{Name: "never-spawned", Configured: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("snapshot entry = %+v, want %+v", got, want)
	}

	other, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	withdrawDoctorSnapshot(other)
	if _, err := PluginDoctorSnapshot(); err != nil {
		t.Errorf("withdrawing another server removed this one's publication: %v", err)
	}

	withdrawDoctorSnapshot(s)
	if _, err := PluginDoctorSnapshot(); !errors.Is(err, ErrDoctorSnapshotNotPublished) {
		t.Errorf("PluginDoctorSnapshot after the server withdrew = %v; want ErrDoctorSnapshotNotPublished", err)
	}
}
