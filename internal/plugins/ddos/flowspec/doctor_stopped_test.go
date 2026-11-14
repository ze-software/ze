// Design: docs/guide/ddos-mitigation.md -- a FlowSpec rule no responder owns
// Related: doctor.go (checkOrphanRecord, pluginRunFromSnapshot)
// Related: doctor_stopped_record_test.go -- the tests of the two registered checks over these fixtures

package flowspec

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// configuredTestTree is a parsed configuration that names the ddos flowspec
// section, as checkOrphanRecord reads it.
func configuredTestTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("ddos").GetOrCreateContainer("flowspec")
	return tree
}

// recordedTestStore is a real store holding the record of orphanTestMatch,
// written by the producer's encoder.
func recordedTestStore(t *testing.T) storage.Storage {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	record, err := encodeAnnouncementRecord(orphanTestMatch())
	if err != nil {
		t.Fatalf("encode the record: %v", err)
	}
	if err := store.WriteKey(announcementKey.Pattern, record); err != nil {
		t.Fatalf("write the record: %v", err)
	}
	return store
}

// TestPluginRunFromSnapshotNamesEachState proves that the daemon's plugin
// snapshot is read into the run the check acts on, for each state a doctor run
// can meet: the plugin running (in process or forked, which the snapshot does
// not tell apart, because the process manager holds both), recorded down, held
// but not yet running, absent from the daemon's plugins, and no snapshot.
//
// VALIDATES: AC-16, that the check learns "not running" from the daemon and
// never reads a failed ask as "not running".
// METHOD: pluginRunFromSnapshot over built snapshots.
// PREVENTS: a stopped plugin read as running, and a run with no snapshot read
// as a stopped plugin.
// MUTATION: drop the `if state.Running` branch in pluginRunFromSnapshot, and
// the running cases go red.
func TestPluginRunFromSnapshotNamesEachState(t *testing.T) {
	stop := process.PluginStop{Cause: process.StopPolicyExit, Policy: "unspecified"}
	cases := []struct {
		name     string
		snapshot pluginserver.DoctorSnapshot
		err      error
		want     string
	}{
		{"running", snapshotOf(pluginserver.PluginDoctorState{Name: Name, Configured: true, HasProcess: true, Running: true}), nil, "running"},
		{"recorded-down", snapshotOf(pluginserver.PluginDoctorState{Name: Name, Configured: true, Down: true, Stop: stop}), nil, "not-running:" + process.StoppedMessage(Name, &stop)},
		{"held-not-yet-running", snapshotOf(pluginserver.PluginDoctorState{Name: Name, Configured: true, HasProcess: true}), nil, "not-running:" + notStartedWhy},
		{"absent", snapshotOf(pluginserver.PluginDoctorState{Name: "ddos-local", Configured: true, Running: true}), nil, "not-run"},
		{"no-snapshot", pluginserver.DoctorSnapshot{}, pluginserver.ErrDoctorSnapshotNotPublished, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := describeRun(pluginRunFromSnapshot(&tc.snapshot, tc.err))
			if got != tc.want {
				t.Errorf("run = %q, want %q", got, tc.want)
			}
		})
	}
}

// snapshotOf is a snapshot holding the given plugin states.
func snapshotOf(states ...pluginserver.PluginDoctorState) pluginserver.DoctorSnapshot {
	return pluginserver.DoctorSnapshot{Plugins: states}
}

// describeRun spells a pluginRun so a case can compare it.
func describeRun(run pluginRun) string {
	switch r := run.(type) {
	case pluginRunning:
		return "running"
	case pluginNotRunning:
		return "not-running:" + r.why
	case pluginNotRun:
		return "not-run"
	case pluginRunUnknown:
		if !errors.Is(r.err, pluginserver.ErrDoctorSnapshotNotPublished) {
			return "unknown with another error: " + r.err.Error()
		}
		return "unknown"
	}
	return "no known run"
}
