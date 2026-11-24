// Design: docs/guide/ddos-mitigation.md -- a FlowSpec rule no responder owns
// Related: doctor.go (checkOrphanRecord, pluginRunFromSnapshot)
// Related: doctor_stopped_record_test.go -- the tests of the two registered checks over these fixtures

package flowspec

import (
	"errors"
	"strings"
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
		{"running", snapshotOf(pluginserver.PluginDoctorState{Name: Name, RegistrationName: Name, Configured: true, HasProcess: true, Running: true}), nil, "running"},
		{"recorded-down", snapshotOf(pluginserver.PluginDoctorState{Name: Name, RegistrationName: Name, Configured: true, Down: true, Stop: stop}), nil, "not-running:" + process.StoppedMessage(Name, &stop)},
		{"held-not-yet-running", snapshotOf(pluginserver.PluginDoctorState{Name: Name, RegistrationName: Name, Configured: true, HasProcess: true}), nil, "not-running:" + notStartedWhy},
		{"absent", snapshotOf(pluginserver.PluginDoctorState{Name: "ddos-local", RegistrationName: "ddos-local", Configured: true, Running: true}), nil, "not-run"},
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

// TestPluginRunFromSnapshotMatchesByCompiledIdentity proves that
// pluginRunFromSnapshot and stoppedOrphanReport key on compiled registration
// identity (PluginDoctorState.RegistrationName) while naming the operator's
// configured endpoint alias (PluginDoctorState.Name) in stopped diagnostics
// and restart fixes.
//
// VALIDATES: owned journal row 2026-11-14 (ddos/flowspec/doctor.go) — aliased
// compiled ddos-flowspec is recognized when running and reports its alias when
// stopped; a foreign external program named ddos-flowspec is treated as not run.
// PREVENTS: doctor reporting a healthy aliased ddos-flowspec as unrun, offering
// a restart for the hardcoded name instead of the alias, or treating a foreign
// external program named ddos-flowspec as the compiled plugin.
func TestPluginRunFromSnapshotMatchesByCompiledIdentity(t *testing.T) {
	const alias = "my-ddos"
	stop := process.PluginStop{Cause: process.StopPolicyExit, Policy: "unspecified"}

	t.Run("aliased running reports healthy", func(t *testing.T) {
		snap := snapshotOf(pluginserver.PluginDoctorState{
			Name:             alias,
			RegistrationName: Name,
			Configured:       true,
			HasProcess:       true,
			Running:          true,
		})
		run := pluginRunFromSnapshot(&snap, nil)
		if _, ok := run.(pluginRunning); !ok {
			t.Fatalf("run = %#v, want pluginRunning", run)
		}
		diags := judgeStoppedRecord(configuredTestTree(), run, storedReading(t))
		if len(diags) != 0 {
			t.Fatalf("findings for running aliased plugin = %v, want none", diags)
		}
	})

	t.Run("aliased stopped names alias in message and restart fix", func(t *testing.T) {
		snap := snapshotOf(pluginserver.PluginDoctorState{
			Name:             alias,
			RegistrationName: Name,
			Configured:       true,
			Down:             true,
			Stop:             stop,
		})
		run := pluginRunFromSnapshot(&snap, nil)
		notRunning, ok := run.(pluginNotRunning)
		if !ok {
			t.Fatalf("run = %#v, want pluginNotRunning", run)
		}
		wantWhy := process.StoppedMessage(alias, &stop)
		if notRunning.why != wantWhy {
			t.Errorf("why = %q, want %q", notRunning.why, wantWhy)
		}
		diags := judgeStoppedRecord(configuredTestTree(), run, storedReading(t))
		if len(diags) != 1 {
			t.Fatalf("findings = %v, want one", diags)
		}
		wantFix := process.RestartCommand(alias)
		if diags[0].Fix != wantFix {
			t.Errorf("fix = %q, want %q", diags[0].Fix, wantFix)
		}
		if !strings.Contains(diags[0].Message, wantWhy) {
			t.Errorf("message %q does not contain alias stop reason %q", diags[0].Message, wantWhy)
		}
	})

	t.Run("foreign external named ddos-flowspec is treated as not running", func(t *testing.T) {
		snap := snapshotOf(pluginserver.PluginDoctorState{
			Name:             Name,
			RegistrationName: "",
			Configured:       true,
			HasProcess:       true,
			Running:          true,
		})
		run := pluginRunFromSnapshot(&snap, nil)
		if _, ok := run.(pluginNotRun); !ok {
			t.Fatalf("run = %#v, want pluginNotRun for foreign external named %s", run, Name)
		}
	})
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
