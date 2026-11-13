// Design: docs/guide/ddos-mitigation.md -- a FlowSpec rule no responder owns
// Related: doctor.go (checkOrphanRecord, judgeOrphanRecord, pluginRunFromSnapshot)

package flowspec

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
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

// answerRun is a readRun that answers run.
func answerRun(run pluginRun) func() pluginRun {
	return func() pluginRun { return run }
}

// TestOrphanRecordCheckReportsARecordWhileTheConfiguredPluginIsStopped proves
// that the registered check reports a recorded rule while ddos flowspec is
// configured and the daemon says the plugin is not running, with the victim,
// why nothing withdraws it, and the plugin restart as its fix.
//
// VALIDATES: AC-16, "with the plugin's process ended, ze doctor still reports
// the record", for the plugin that stopped on its own while its section stays
// configured: no failure policy restarts it, so its runtime check cannot
// answer.
// METHOD: judgeOrphanRecord over a configured tree, a real store holding the
// record, and a run answering a recorded stop.
// PREVENTS: a rule left at the peers while doctor names only the stopped
// plugin.
// MUTATION: return nil, true for pluginNotRunning in configuredOrphanReport,
// and this test goes red with no finding.
func TestOrphanRecordCheckReportsARecordWhileTheConfiguredPluginIsStopped(t *testing.T) {
	stop := process.PluginStop{Cause: process.StopPolicyExit, Policy: "unspecified"}
	why := process.StoppedMessage(Name, &stop)

	diags := judgeOrphanRecord(diagnostic.DoctorCheckContext{Tree: configuredTestTree(), Store: recordedTestStore(t)},
		answerRun(pluginNotRunning{why: why}))

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if !strings.Contains(diags[0].Message, exitTestVictim) {
		t.Errorf("message %q does not name the victim %s", diags[0].Message, exitTestVictim)
	}
	if !strings.Contains(diags[0].Message, why) {
		t.Errorf("message %q does not say why the plugin is not running (%q)", diags[0].Message, why)
	}
	if diags[0].Fix != restartTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, restartTestFix)
	}
}

// TestOrphanRecordCheckIsQuietWhileThePluginRuns proves that the registered
// check says nothing while the daemon runs the plugin: the record of a running
// mitigation is healthy, and the plugin's runtime check owns the judgement.
//
// VALIDATES: AC-16, negative: no report of a live rule, and no second report of
// a record the runtime check already reports.
// METHOD: judgeOrphanRecord over a configured tree and a stored record, with a
// run answering that the plugin runs.
// PREVENTS: a doctor warning during every mitigation, and a finding listed
// twice.
// MUTATION: drop the `if running { return nil }` in judgeOrphanRecord, and
// this test goes red with one finding.
func TestOrphanRecordCheckIsQuietWhileThePluginRuns(t *testing.T) {
	diags := judgeOrphanRecord(diagnostic.DoctorCheckContext{Tree: configuredTestTree(), Store: recordedTestStore(t)},
		answerRun(pluginRunning{}))

	if len(diags) != 0 {
		t.Fatalf("findings while the plugin runs = %v, want none", diags)
	}
}

// TestOrphanRecordCheckOutsideTheDaemonSaysWhatItCannotTell proves that a
// doctor run with no plugin snapshot (the local `ze doctor` process) reports a
// recorded rule while the section is configured, says it cannot tell whether a
// running plugin holds it live, and names the doctor run through the daemon as
// its fix.
//
// VALIDATES: AC-16 for the local route: the check says what it found and what
// it cannot tell, and never reads "no daemon to ask" as healthy.
// METHOD: checkOrphanRecord itself, over a configured tree and a stored record;
// no plugin server runs in the test binary, so PluginDoctorSnapshot answers
// ErrDoctorSnapshotNotPublished.
// PREVENTS: a recorded rule hidden from the local route because the section is
// configured.
// MUTATION: return nil, true for pluginRunUnknown in configuredOrphanReport,
// and this test goes red with no finding.
func TestOrphanRecordCheckOutsideTheDaemonSaysWhatItCannotTell(t *testing.T) {
	diags := checkOrphanRecord(diagnostic.DoctorCheckContext{Tree: configuredTestTree(), Store: recordedTestStore(t)})

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if !strings.Contains(diags[0].Message, exitTestVictim) {
		t.Errorf("message %q does not name the victim %s", diags[0].Message, exitTestVictim)
	}
	if !strings.Contains(diags[0].Message, "cannot tell whether the plugin runs") {
		t.Errorf("message %q does not say what this run cannot tell", diags[0].Message)
	}
	if diags[0].Fix != "ze show doctor" {
		t.Errorf("fix = %q, want %q", diags[0].Fix, "ze show doctor")
	}
}

// TestOrphanRecordCheckWithTheSectionUnrunPrintsTheWithdrawal proves that a
// configured section the daemon runs no plugin for gets the exact withdrawal
// as its fix: no plugin runs to restart.
//
// VALIDATES: AC-16, "the exact command that withdraws it", when the daemon
// loaded a configuration that does not name the section.
// METHOD: judgeOrphanRecord with a run answering that the daemon's plugin set
// does not name ddos-flowspec.
// PREVENTS: a restart fix for a plugin the daemon cannot restart.
// MUTATION: return stoppedOrphanReport for pluginNotRun in
// configuredOrphanReport, and the fix check goes red.
func TestOrphanRecordCheckWithTheSectionUnrunPrintsTheWithdrawal(t *testing.T) {
	diags := judgeOrphanRecord(diagnostic.DoctorCheckContext{Tree: configuredTestTree(), Store: recordedTestStore(t)},
		answerRun(pluginNotRun{}))

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Fix != orphanTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, orphanTestFix)
	}
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
