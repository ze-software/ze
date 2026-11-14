// Design: docs/guide/ddos-mitigation.md -- a FlowSpec rule no responder owns
// Related: doctor.go (checkOrphanRecord, judgeStoppedRecord, stoppedRunReport, unrunReport, unavailableStoreFindings, pluginRunFromSnapshot)
// Related: doctor_stopped_test.go -- configuredTestTree, recordedTestStore, snapshotOf

package flowspec

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/statestore"
)

// storedReading is a readRecord over a real store holding the record of
// orphanTestMatch, written by the producer's encoder.
func storedReading(t *testing.T) func() recordReading {
	t.Helper()
	store := recordedTestStore(t)
	return func() recordReading { return storedRecordReading(store) }
}

// stoppedTestRun is the run of a plugin whose startup handshake failed, as the
// daemon records it (server/startup.go RecordStopped).
func stoppedTestRun(cleanupOwed bool) pluginNotRunning {
	stop := process.PluginStop{Cause: process.StopHandshakeFailed, Detail: "ddos-flowspec: event bus not configured"}
	return pluginNotRunning{why: process.StoppedMessage(Name, &stop), cleanupOwed: cleanupOwed}
}

// TestOrphanRecordCheckStandsDownWhileTheSectionIsConfigured proves that the
// post-config check says nothing while the configuration it read names ddos
// flowspec, whatever the store holds: the plugin's own runtime check, or the
// daemon's stoppedRecordCheck, judges the record then. The post-config check
// also runs on the local route, with no daemon, where it cannot tell a running
// mitigation from a stopped plugin.
//
// VALIDATES: the phase rule (docs/architecture/doctor-and-health-checks.md,
// "The phase decides what a check may read"): the post-config check reads no
// live daemon state.
// METHOD: checkOrphanRecord itself, over a configured tree and a stored record.
// PREVENTS: a warning on every local `ze doctor` after a stopped daemon left a
// record, with a fix that needs the daemon.
// MUTATION: delete the configured-section `return nil` in checkOrphanRecord,
// and this test goes red with one finding, the not-configured report.
func TestOrphanRecordCheckStandsDownWhileTheSectionIsConfigured(t *testing.T) {
	diags := checkOrphanRecord(diagnostic.DoctorCheckContext{Tree: configuredTestTree(), Store: recordedTestStore(t)})

	if len(diags) != 0 {
		t.Fatalf("findings while the section is configured = %v, want none", diags)
	}
}

// TestStoppedRecordCheckReportsARecordWhileTheConfiguredPluginIsStopped proves
// that the daemon's runtime check reports a recorded rule while the daemon holds
// ddos flowspec configured and the plugin is not running, with the victim, why
// the plugin is not running, that nothing withdraws the rule, and the plugin
// restart as its fix.
//
// VALIDATES: AC-16, "with the plugin's process ended, ze doctor still reports
// the record", for a plugin that is not running while its section stays
// configured: the daemon does not ask a stopped plugin its runtime check.
// METHOD: judgeStoppedRecord with a run answering a recorded stop and a real
// store holding the record.
// PREVENTS: a rule left at the peers while doctor names only the stopped
// plugin.
// MUTATION: answer `nil, false` for pluginNotRunning in stoppedRunReport, and
// this test goes red with no finding.
func TestStoppedRecordCheckReportsARecordWhileTheConfiguredPluginIsStopped(t *testing.T) {
	run := stoppedTestRun(false)

	diags := judgeStoppedRecord(configuredTestTree(), run, storedReading(t))

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if !strings.Contains(diags[0].Message, exitTestVictim) {
		t.Errorf("message %q does not name the victim %s", diags[0].Message, exitTestVictim)
	}
	if !strings.Contains(diags[0].Message, run.why) {
		t.Errorf("message %q does not say why the plugin is not running (%q)", diags[0].Message, run.why)
	}
	if !strings.Contains(diags[0].Message, "Nothing withdraws the rule until the plugin runs again") {
		t.Errorf("message %q does not say that nothing withdraws the rule while the plugin is stopped", diags[0].Message)
	}
	if strings.Contains(diags[0].Message, restartRefusedSentence) {
		t.Errorf("message %q says the restart is refused, and no cleanup is owed", diags[0].Message)
	}
	if diags[0].Fix != restartTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, restartTestFix)
	}
}

// TestStoppedRecordCheckIsQuietWhileThePluginRuns proves that the daemon's
// runtime check says nothing while the daemon runs the plugin: the record of a
// running mitigation is healthy, and the plugin's own runtime check owns the
// judgement.
//
// VALIDATES: AC-16, negative: no report of a live rule, and no second report of
// a record the plugin's runtime check already reports.
// METHOD: judgeStoppedRecord with a run answering that the plugin runs, and a
// stored record.
// PREVENTS: a doctor warning during every mitigation, and a finding listed
// twice.
// MUTATION: answer `stoppedOrphanReport(pluginNotRunning{why: notStartedWhy}),
// true` for pluginRunning in stoppedRunReport, and this test goes red with one
// finding.
func TestStoppedRecordCheckIsQuietWhileThePluginRuns(t *testing.T) {
	diags := judgeStoppedRecord(configuredTestTree(), pluginRunning{}, storedReading(t))

	if len(diags) != 0 {
		t.Fatalf("findings while the plugin runs = %v, want none", diags)
	}
}

// TestStoppedRecordCheckWithCleanupOwedSaysTheRestartWaits proves that while a
// stopped process of the plugin still owes its cleanup, the report says that
// ze refuses the restart until that cleanup has finished, and names the
// doctor-plugin-stopped row that says when it has.
//
// VALIDATES: the report's fix is true in the state the restart refuses
// (server/reload_tx.go errReplacedCleanupLate).
// METHOD: judgeStoppedRecord with a recorded stop whose cleanup is owed.
// PREVENTS: a restart offered as working while ze refuses it.
// MUTATION: drop the cleanupOwed branch in stoppedOrphanReport, and the
// refusal check goes red.
func TestStoppedRecordCheckWithCleanupOwedSaysTheRestartWaits(t *testing.T) {
	diags := judgeStoppedRecord(configuredTestTree(), stoppedTestRun(true), storedReading(t))

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if !strings.Contains(diags[0].Message, restartRefusedSentence) {
		t.Errorf("message %q does not say ze refuses the restart until the cleanup has finished", diags[0].Message)
	}
	if !strings.Contains(diags[0].Message, diagnostic.CodeDoctorPluginStopped) {
		t.Errorf("message %q does not name the %s row that says when the cleanup has finished", diags[0].Message, diagnostic.CodeDoctorPluginStopped)
	}
	if diags[0].Fix != restartTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, restartTestFix)
	}
}

// TestStoppedRecordCheckBeforePluginStartupSaysWhatItCannotTell proves that a
// daemon whose plugin state is not available yet reports a recorded rule,
// says it cannot tell yet whether the plugin runs, and names the daemon doctor
// run as its fix, which can tell once the plugins started.
//
// VALIDATES: AC-16: the check never reads "no plugin state" as healthy.
// METHOD: judgeStoppedRecord with the run of a snapshot that answers
// ErrDoctorSnapshotNotStarted, and a stored record.
// PREVENTS: a recorded rule hidden while the daemon is starting its plugins.
// MUTATION: answer `nil, false` for pluginRunUnknown in stoppedRunReport, and
// this test goes red with no finding.
func TestStoppedRecordCheckBeforePluginStartupSaysWhatItCannotTell(t *testing.T) {
	diags := judgeStoppedRecord(configuredTestTree(), pluginRunUnknown{err: pluginserver.ErrDoctorSnapshotNotStarted}, storedReading(t))

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if !strings.Contains(diags[0].Message, exitTestVictim) {
		t.Errorf("message %q does not name the victim %s", diags[0].Message, exitTestVictim)
	}
	if !strings.Contains(diags[0].Message, "cannot tell yet whether the plugin runs") {
		t.Errorf("message %q does not say what this run cannot tell", diags[0].Message)
	}
	if !strings.Contains(diags[0].Message, pluginserver.ErrDoctorSnapshotNotStarted.Error()) {
		t.Errorf("message %q does not carry why the plugin state is not available", diags[0].Message)
	}
	if diags[0].Fix != "ze show doctor" {
		t.Errorf("fix = %q, want %q", diags[0].Fix, "ze show doctor")
	}
}

// TestStoppedRecordCheckLeavesAnUnconfiguredPluginToThePostConfigCheck proves
// that the daemon's runtime check says nothing while the saved configuration
// doctor read does not name ddos flowspec and the daemon holds no ddos-flowspec
// process: the post-config check reported the record from that same tree, and
// a second report would list it twice.
//
// VALIDATES: one report per record on the daemon route.
// METHOD: judgeStoppedRecord with a tree that does not name the section, a run
// answering that the daemon holds no ddos-flowspec process, and a stored
// record.
// PREVENTS: the not-configured record listed twice by `show doctor`.
// MUTATION: delete the section-absent `return nil` in judgeStoppedRecord, and
// this test goes red with one finding, the unrun report.
func TestStoppedRecordCheckLeavesAnUnconfiguredPluginToThePostConfigCheck(t *testing.T) {
	diags := judgeStoppedRecord(config.NewTree(), pluginNotRun{}, storedReading(t))

	if len(diags) != 0 {
		t.Fatalf("findings for a plugin the daemon does not configure = %v, want none", diags)
	}
}

// TestStoppedRecordCheckLeavesAStoppedPluginTheSavedFileOmitsToThePostConfigCheck
// proves that the daemon's runtime check says nothing while the saved
// configuration doctor read does not name ddos flowspec, even though the daemon
// holds the plugin configured and stopped: the post-config check reported the
// record from that same tree, with the withdrawal as its fix, and a stopped
// report beside it would give the same record a second row whose fix, the
// restart, disagrees with that one.
//
// VALIDATES: one report per record on the daemon route, decided by the saved
// file before the run is looked at.
// METHOD: judgeStoppedRecord with a tree that does not name the section, a run
// answering a recorded stop, and a stored record.
// PREVENTS: two rows for one record, with fixes that disagree.
// MUTATION: delete the section-absent `return nil` in judgeStoppedRecord, and
// this test goes red with one finding, the stopped report.
func TestStoppedRecordCheckLeavesAStoppedPluginTheSavedFileOmitsToThePostConfigCheck(t *testing.T) {
	diags := judgeStoppedRecord(config.NewTree(), stoppedTestRun(false), storedReading(t))

	if len(diags) != 0 {
		t.Fatalf("findings while the saved configuration does not name the section = %v, want none", diags)
	}
}

// TestStoppedRecordCheckLeavesAnUnknownRunTheSavedFileOmitsToThePostConfigCheck
// proves that the daemon's runtime check says nothing while the saved
// configuration doctor read does not name ddos flowspec and the daemon cannot
// tell yet whether the plugin runs: the post-config check reported the record
// from that same tree, and a cannot-tell report beside it would list it twice.
//
// VALIDATES: one report per record on the daemon route, for every run kind.
// METHOD: judgeStoppedRecord with a tree that does not name the section, the
// run of a snapshot that answers ErrDoctorSnapshotNotStarted, and a stored
// record.
// PREVENTS: two rows for one record, with fixes that disagree.
// MUTATION: delete the section-absent `return nil` in judgeStoppedRecord, and
// this test goes red with one finding, the cannot-tell report.
func TestStoppedRecordCheckLeavesAnUnknownRunTheSavedFileOmitsToThePostConfigCheck(t *testing.T) {
	diags := judgeStoppedRecord(config.NewTree(), pluginRunUnknown{err: pluginserver.ErrDoctorSnapshotNotStarted}, storedReading(t))

	if len(diags) != 0 {
		t.Fatalf("findings while the saved configuration does not name the section = %v, want none", diags)
	}
}

// TestStoppedRecordCheckUnrunNamesTheWithdrawalOnlyInTheMessage proves that
// the report of a record while the daemon holds no process named ddos-flowspec
// names the exact withdrawal for the victim in its message, for the case where
// no process runs the plugin, and never as its fix. The fix is read-only, so
// the message is the only place the operator meets the remedy.
//
// VALIDATES: the unrun report gives the remedy of the case where nothing
// withdraws the rule, without offering it as the fix.
// METHOD: judgeStoppedRecord with a tree naming the section, a run answering
// that the daemon holds no ddos-flowspec process, and a stored record.
// PREVENTS: an operator left with no remedy once the running configuration
// shows no process runs the plugin.
// MUTATION: drop the withdrawal (`.Str(withdrawalCommand(m))`) from unrunCases,
// and the message check goes red.
func TestStoppedRecordCheckUnrunNamesTheWithdrawalOnlyInTheMessage(t *testing.T) {
	diags := judgeStoppedRecord(configuredTestTree(), pluginNotRun{}, storedReading(t))

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if !strings.Contains(diags[0].Message, orphanTestFix) {
		t.Errorf("message %q does not name the exact withdrawal %q", diags[0].Message, orphanTestFix)
	}
	if diags[0].Fix == orphanTestFix {
		t.Errorf("fix = %q, the withdrawal, which can end a live mitigation", diags[0].Fix)
	}
}

// TestStoppedRecordCheckWithNoConfigurationAndNoStoreIsSilentWhileNothingRunsThePlugin
// proves that a daemon holding no ddos-flowspec process and publishing no
// store gets no finding from the runtime check, in a doctor run whose
// post-config phase did not run: the plugin writes the record only through the
// store the daemon publishes, so no process of this daemon can have written
// one, and a storage row from ddos-flowspec would name a record that cannot
// exist.
//
// VALIDATES: the storage-unavailable row is given only while the daemon holds
// the plugin configured, or cannot tell yet.
// METHOD: judgeStoppedRecord with no tree, a run answering that the daemon
// holds no ddos-flowspec process, and the reading statestore gives when no
// store is registered.
// PREVENTS: a ddos-flowspec storage warning on every box that never configured
// the plugin and runs with no store.
// MUTATION: delete the pluginNotRun `return nil` in unavailableStoreFindings,
// and this test goes red with one finding under doctor-storage-unavailable.
func TestStoppedRecordCheckWithNoConfigurationAndNoStoreIsSilentWhileNothingRunsThePlugin(t *testing.T) {
	diags := judgeStoppedRecord(nil, pluginNotRun{}, func() recordReading {
		return recordUnreadable{err: statestore.ErrUnavailable}
	})

	if len(diags) != 0 {
		t.Fatalf("findings with no ddos-flowspec process and no store = %v, want none", diags)
	}
}

// TestStoppedRecordCheckWithTheSectionUnrunOffersNoWithdrawal proves that a
// record, a saved configuration that names ddos flowspec and a daemon holding
// no process named ddos-flowspec get one warning that says only what is known:
// the daemon may run a configuration without the section, or a plugin block
// may run the plugin under another name, and doctor cannot tell which. Its fix
// is the read-only command that tells, never a withdrawal.
//
// Lineage: it replaces TestOrphanRecordCheckWithTheSectionUnrunPrintsTheWithdrawal
// (doctor_stopped_test.go) and judges the same state. That test expected the
// withdrawal as the fix, and a withdrawal can end a live mitigation that a
// `plugin { internal <x> { use ddos-flowspec } }` block runs.
//
// VALIDATES: AC-16 for the state no other check reports on the daemon route.
// METHOD: judgeStoppedRecord with a tree naming the section, a run answering
// that the daemon holds no ddos-flowspec process, and a stored record.
// PREVENTS: the record reported by nobody, and a withdrawal offered while a
// plugin under another name may hold the rule live.
// MUTATION: return unconfiguredOrphanDiagnostic for a read saved configuration
// in unrunReport, and the fix and sentence checks go red.
func TestStoppedRecordCheckWithTheSectionUnrunOffersNoWithdrawal(t *testing.T) {
	diags := judgeStoppedRecord(configuredTestTree(), pluginNotRun{}, storedReading(t))

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if !strings.Contains(diags[0].Message, exitTestVictim) {
		t.Errorf("message %q does not name the victim %s", diags[0].Message, exitTestVictim)
	}
	if !strings.Contains(diags[0].Message, unrunUndecidedSentence) {
		t.Errorf("message %q does not say that doctor cannot tell which case holds (%q)", diags[0].Message, unrunUndecidedSentence)
	}
	if diags[0].Fix == orphanTestFix {
		t.Errorf("fix = %q, the withdrawal, which can end a live mitigation", diags[0].Fix)
	}
	if diags[0].Fix != runningConfigCommand {
		t.Errorf("fix = %q, want %q", diags[0].Fix, runningConfigCommand)
	}
}

// TestStoppedRecordCheckWithNoConfigurationSaysWhatItCannotTell proves that a
// record and a daemon holding no ddos-flowspec process, in a doctor run whose
// post-config phase did not run, get one warning that says doctor loaded no
// saved configuration and so cannot tell whether the section is configured,
// with the read-only command that tells as its fix.
//
// VALIDATES: a runtime check with no tree says what it cannot tell, and never
// guesses (docs/architecture/doctor-and-health-checks.md, "The runtime phase:
// checks only the daemon runs").
// METHOD: judgeStoppedRecord with no tree, a run answering that the daemon
// holds no ddos-flowspec process, and a stored record.
// PREVENTS: a record nobody reports because the configuration did not load.
// MUTATION: answer unrunUndecidedDiagnostic when no saved configuration was
// read in unrunReport, and the no-configuration sentence check goes red.
func TestStoppedRecordCheckWithNoConfigurationSaysWhatItCannotTell(t *testing.T) {
	diags := judgeStoppedRecord(nil, pluginNotRun{}, storedReading(t))

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if !strings.Contains(diags[0].Message, exitTestVictim) {
		t.Errorf("message %q does not name the victim %s", diags[0].Message, exitTestVictim)
	}
	if !strings.Contains(diags[0].Message, noConfigurationSentence) {
		t.Errorf("message %q does not say that doctor loaded no configuration (%q)", diags[0].Message, noConfigurationSentence)
	}
	if diags[0].Fix != runningConfigCommand {
		t.Errorf("fix = %q, want %q", diags[0].Fix, runningConfigCommand)
	}
}

// TestDaemonRecordReadingReadsThePublishedStore proves that the production
// reader of the runtime check reads the record from the store the daemon
// publishes through statestore.
//
// VALIDATES: the record the runtime check judges is the one the plugin's state
// calls wrote.
// METHOD: daemonRecordReading over a real store holding the record, published
// with statestore.SetStore; the previous handle is restored after.
// PREVENTS: a reader that looks where the plugin never writes, and so reads
// every record as absent.
// MUTATION: read another key in daemonRecordReading, and this test goes red with
// recordAbsent.
func TestDaemonRecordReadingReadsThePublishedStore(t *testing.T) {
	publishTestStore(t, recordedTestStore(t))

	reading := daemonRecordReading()
	rule, ok := reading.(recordedRule)
	if !ok {
		t.Fatalf("reading = %#v, want the recorded rule", reading)
	}
	if rule.rule != orphanTestMatch() {
		t.Errorf("rule = %v, want %v", rule.rule, orphanTestMatch())
	}
}

// TestStoppedRecordCheckWithNoPublishedStoreSaysSo proves that the runtime
// check, reading through the production reader while the daemon publishes no
// store, reports that it cannot read the record under the storage code.
//
// VALIDATES: the unavailable answer statestore gives is reached for real, and
// read as Unknown with its reason, never as "no record".
// METHOD: judgeStoppedRecord with a recorded stop and daemonRecordReading,
// after statestore.SetStore(nil); the previous handle is restored after.
// PREVENTS: a recorded rule hidden because the store was not published.
// MUTATION: drop the statestore.ErrUnavailable branch in judgeStoppedRecord, and
// the code check goes red (the finding then carries the orphan code).
func TestStoppedRecordCheckWithNoPublishedStoreSaysSo(t *testing.T) {
	publishTestStore(t, nil)

	diags := judgeStoppedRecord(configuredTestTree(), stoppedTestRun(false), daemonRecordReading)

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorStorageUnavailable {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorStorageUnavailable)
	}
}

// publishTestStore publishes store as the daemon's statestore handle for one
// test, and restores the handle it replaced when the test ends.
func publishTestStore(t *testing.T, store statestore.Storage) {
	t.Helper()
	previous := statestore.Store()
	statestore.SetStore(store)
	t.Cleanup(func() { statestore.SetStore(previous) })
}

// TestRecordChecksRegisterAtTheirPhases proves that the post-config check and
// the runtime check are registered at the phases their reads need.
//
// VALIDATES: the runtime check runs only on the daemon route, where the plugin
// snapshot and the published store exist, and the post-config check runs in
// whichever process answers doctor.
// METHOD: diagnostic.DoctorChecksForPhase over the registry this package's
// init() filled.
// PREVENTS: the runtime check run by the local route over empty daemon state.
// MUTATION: set stoppedRecordCheck's Phase to DoctorPhasePostConfig, and the
// runtime case goes red.
func TestRecordChecksRegisterAtTheirPhases(t *testing.T) {
	cases := []struct {
		check string
		phase diagnostic.DoctorCheckPhase
	}{
		{stoppedRecordCheck.Name, diagnostic.DoctorPhaseRuntime},
		{orphanRecordCheck.Name, diagnostic.DoctorPhasePostConfig},
	}
	for _, tc := range cases {
		t.Run(tc.check, func(t *testing.T) {
			found := false
			for _, check := range diagnostic.DoctorChecksForPhase(tc.phase) {
				if check.Name == tc.check {
					found = true
				}
			}
			if !found {
				t.Errorf("%s is not registered at the %s phase", tc.check, tc.phase)
			}
		})
	}
}

// TestStoppedRecordCheckWithNoDaemonStoreSaysSo proves that a daemon that
// publishes no store reports that it cannot read the record, under the storage
// code, and never reads the missing store as "no record".
//
// VALIDATES: a read the daemon cannot make answers Unknown with its reason.
// METHOD: judgeStoppedRecord with a recorded stop and the reading statestore
// gives when no store is registered.
// PREVENTS: a recorded rule hidden because the store was not published.
// MUTATION: drop the statestore.ErrUnavailable branch in judgeStoppedRecord,
// and the code check goes red (the finding then carries the orphan code).
func TestStoppedRecordCheckWithNoDaemonStoreSaysSo(t *testing.T) {
	diags := judgeStoppedRecord(configuredTestTree(), stoppedTestRun(false), func() recordReading {
		return recordUnreadable{err: statestore.ErrUnavailable}
	})

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorStorageUnavailable {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorStorageUnavailable)
	}
	if diags[0].Fix != recordReadCommand() {
		t.Errorf("fix = %q, want %q", diags[0].Fix, recordReadCommand())
	}
}

// TestPluginRunFromSnapshotNamesTheStatesTheStoppedCheckActsOn proves the
// snapshot states TestPluginRunFromSnapshotNamesEachState does not hold: a
// process the daemon holds and no configuration names, a stop whose cleanup is
// owed, and the snapshot of a daemon whose plugin startup has not begun.
//
// VALIDATES: a held but unconfigured process is not read as a stopped plugin,
// the owed cleanup reaches the report, and each snapshot error is Unknown.
// METHOD: pluginRunFromSnapshot over built snapshots.
// PREVENTS: a restart offered for a plugin no configuration names, and a
// restart offered as working while ze refuses it.
// MUTATION: drop the `if !state.Configured` branch in pluginRunFromSnapshot,
// and the held-not-configured case goes red.
func TestPluginRunFromSnapshotNamesTheStatesTheStoppedCheckActsOn(t *testing.T) {
	stop := process.PluginStop{Cause: process.StopPolicyExit, Policy: "unspecified"}
	cases := []struct {
		name     string
		snapshot pluginserver.DoctorSnapshot
		err      error
		want     string
	}{
		{"held-not-configured", snapshotOf(pluginserver.PluginDoctorState{Name: Name, HasProcess: true}), nil, "not-run"},
		{"down-cleanup-owed", snapshotOf(pluginserver.PluginDoctorState{Name: Name, Configured: true, Down: true, Stop: stop, CleanupOwed: true}), nil,
			"not-running:" + process.StoppedMessage(Name, &stop) + " cleanup-owed"},
		{"not-started", pluginserver.DoctorSnapshot{}, pluginserver.ErrDoctorSnapshotNotStarted, "unknown:" + pluginserver.ErrDoctorSnapshotNotStarted.Error()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := describeStoppedRun(pluginRunFromSnapshot(&tc.snapshot, tc.err))
			if got != tc.want {
				t.Errorf("run = %q, want %q", got, tc.want)
			}
		})
	}
}

// describeStoppedRun spells a pluginRun with the owed cleanup and the exact
// snapshot error, so a case can compare them.
func describeStoppedRun(run pluginRun) string {
	switch r := run.(type) {
	case pluginRunning:
		return "running"
	case pluginNotRunning:
		if r.cleanupOwed {
			return "not-running:" + r.why + " cleanup-owed"
		}
		return "not-running:" + r.why
	case pluginNotRun:
		return "not-run"
	case pluginRunUnknown:
		if errors.Is(r.err, pluginserver.ErrDoctorSnapshotNotStarted) {
			return "unknown:" + r.err.Error()
		}
		if errors.Is(r.err, pluginserver.ErrDoctorSnapshotNotPublished) {
			return "unknown:" + r.err.Error()
		}
		return "unknown with another error"
	}
	return "no known run"
}
