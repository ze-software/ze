// Design: docs/guide/ddos-mitigation.md -- a FlowSpec rule no responder owns
// Related: doctor.go (answerOrphanRuntimeCheck, checkOrphanRecord)

package flowspec

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// orphanTestMatch is the rule every doctor case records.
func orphanTestMatch() flowspecMatch {
	return buildMatch(ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(exitTestVictim), Proto: 17})
}

// orphanTestFix is the exact withdrawal the removed-section report must print
// for orphanTestMatch: no plugin runs to restart.
const orphanTestFix = `ze cli -c "send bgp * update text mp ipv4/flow del destination-ipv4 192.0.2.9/32 protocol =17"`

// restartTestFix is the fix the runtime report must print while the plugin
// runs: the restarted plugin's start withdraws the recorded rule and removes
// the record, which a withdrawal typed at the CLI does not. Written out rather
// than built from process.RestartCommand, because it is what the operator types.
const restartTestFix = "ze request plugin restart ddos-flowspec"

// TestOrphanCheckReportsARecordWithNoLiveResponder proves that the runtime check
// reports a recorded rule the running plugin does not hold as live, with the
// victim and the plugin restart as its fix.
//
// VALIDATES: AC-16, positive: a start-time withdrawal that failed leaves the
// record and no live announcement.
// METHOD: the callback's answer function over a fake holding the record, with
// no live announcement.
// PREVENTS: a rule left at the peers that nothing on the box reports.
func TestOrphanCheckReportsARecordWithNoLiveResponder(t *testing.T) {
	m := orphanTestMatch()
	disp := &fakeDispatcher{recorded: &m}

	diags, err := answerOrphanRuntimeCheck(orphanRuntimeCheck, disp, flowspecMatch{}, false)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != codeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, codeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if !strings.Contains(diags[0].Message, exitTestVictim) {
		t.Errorf("message %q does not name the victim %s", diags[0].Message, exitTestVictim)
	}
	if diags[0].Fix != restartTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, restartTestFix)
	}
}

// TestRuntimeOrphanReportNamesBothCausesOfAFailedWithdrawal proves that the
// runtime report says why the record outlived its rule in two alternatives,
// each true for every case it covers. The first is that no withdrawal is known
// to have reached the BGP peers: none was sent (the start read the record as
// unreadable), the engine did not take it, the engine took it and refused the
// send (test/plugin/ddos-flowspec-orphan-doctor.ci, where the send permission
// refuses it), or the dispatch errored after the engine sent it. The second is
// a withdrawal the engine sent whose record removal then failed (responder.go
// withdrawThrough and withdrawRecordedRule, where Forget fails after a
// successful Dispatch).
//
// VALIDATES: ai/rules/user-facing-errors.md, "why it matters", for the runtime
// report: the reason it gives is true in every case that reaches it, and the
// claim about the peers stays "may", which is true when the rule did leave them.
// METHOD: the runtime report for orphanTestMatch, read for each cause, for the
// hedged claim about the peers, and for the claim it replaced.
// PREVENTS: a report that says the withdrawal never reached the BGP engine,
// when the engine took it and refused to send it; and a report that says the
// withdrawal did not reach the peers, when it did and only the record removal
// failed.
func TestRuntimeOrphanReportNamesBothCausesOfAFailedWithdrawal(t *testing.T) {
	diag := unownedOrphanDiagnostic(orphanTestMatch())

	const reason = "no withdrawal of the rule is known to have reached the BGP peers (none was sent, the BGP engine did not take it or refused to send it, or its answer did not come back)"
	if !strings.Contains(diag.Message, reason) {
		t.Errorf("message %q does not carry the reason %q", diag.Message, reason)
	}
	const removalCause = "or the BGP engine sent the withdrawal and the plugin could not remove the record after it"
	if !strings.Contains(diag.Message, removalCause) {
		t.Errorf("message %q does not name the failed record removal %q", diag.Message, removalCause)
	}
	if !strings.Contains(diag.Message, "Either "+reason) {
		t.Errorf("message %q states the undelivered withdrawal as the only cause; it is one of two alternatives", diag.Message)
	}
	const hedge = "Every BGP peer that took the rule may still discard or rate-limit traffic to "
	if !strings.Contains(diag.Message, hedge) {
		t.Errorf("message %q does not carry the hedged peer claim %q", diag.Message, hedge)
	}
	if strings.Contains(diag.Message, "the last withdrawal of the rule did not reach the BGP peers") {
		t.Errorf("message %q says a withdrawal was sent and lost, which is false when none was sent", diag.Message)
	}
	if strings.Contains(diag.Message, "did not reach the BGP engine") {
		t.Errorf("message %q says the withdrawal did not reach the BGP engine, which is false when the engine refused the send", diag.Message)
	}
	if diag.Fix != restartTestFix {
		t.Errorf("fix = %q, want %q", diag.Fix, restartTestFix)
	}
}

// TestCorruptRecordReportNamesTheReadCommand proves that the report of a record
// that does not decode prints, as its fix, the command that reads the record,
// and never the record's removal.
//
// VALIDATES: ai/rules/user-facing-errors.md for the corrupt record: no plugin
// start withdraws from a record it cannot decode, and removing the record
// first leaves the rule at the peers untraced, so the first step is reading the
// record to find the rule; the removal is the second step, in the message.
// METHOD: the runtime check's answer function over a fake whose Recorded
// answers bytes that are not a record.
// PREVENTS: a fix line the running daemon refuses.
func TestCorruptRecordReportNamesTheReadCommand(t *testing.T) {
	diags, err := answerOrphanRuntimeCheck(orphanRuntimeCheck,
		corruptRecordDispatcher{&fakeDispatcher{}}, flowspecMatch{}, false)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Fix != recordReadTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, recordReadTestFix)
	}
}

// corruptRecordDispatcher answers a stored record that does not decode.
type corruptRecordDispatcher struct{ *fakeDispatcher }

func (corruptRecordDispatcher) Recorded() ([]byte, bool, error) { return []byte("not json"), true, nil }

// TestRemovedSectionReportNamesTheRemovalThatWorksLive proves that the report
// made while ddos flowspec is not configured keeps the withdrawal as its fix,
// says the withdrawal leaves the record, and names the removal of the record
// as the second step: `ze data rm`, which works while ze runs, refused while a
// ddos-flowspec plugin runs, with the restart for that case.
//
// VALIDATES: AC-2, the removed-section report: the two steps in order, why the
// order matters, the running-plugin refusal and the restart; the fix field
// stays the withdrawal. Also AC-16's "the report clears when the record is
// removed", as the message names what removes the record.
// METHOD: the removed-section report for orphanTestMatch, read for the fix,
// for each fact, and for the order of the two steps.
// PREVENTS: a report that says the removal needs ze stopped, a report that
// lets the operator remove the record before the rule is withdrawn, and a
// report whose removal a running plugin refuses with no way out named.
func TestRemovedSectionReportNamesTheRemovalThatWorksLive(t *testing.T) {
	diag := unconfiguredOrphanDiagnostic(orphanTestMatch())

	if diag.Fix != orphanTestFix {
		t.Errorf("fix = %q, want the withdrawal %q", diag.Fix, orphanTestFix)
	}
	assertLiveRecordRemoval(t, diag.Message, "first withdraw the rule with the command below")
	if !strings.Contains(diag.Message, "configure ddos flowspec again") {
		t.Errorf("message %q does not name the plugin start that removes the record", diag.Message)
	}
}

// TestUnrunReportsNameTheRemovalThatWorksLive proves that both reports of a
// daemon with no ddos-flowspec process say the same about the removal as the
// removed-section report, after the withdrawal they name.
//
// VALIDATES: AC-2, "the two unrun reports say the same about the removal".
// METHOD: unrunUndecidedDiagnostic and noConfigurationUnrunDiagnostic for
// orphanTestMatch, each read as the removed-section report is.
// PREVENTS: an unrun report that still says the removal needs ze stopped.
func TestUnrunReportsNameTheRemovalThatWorksLive(t *testing.T) {
	reports := map[string]orphanReport{
		"saved configuration names the section": unrunUndecidedDiagnostic,
		"no configuration loaded":               noConfigurationUnrunDiagnostic,
	}
	for name, report := range reports {
		t.Run(name, func(t *testing.T) {
			diag := report(orphanTestMatch())
			if diag.Fix != runningConfigCommand {
				t.Errorf("fix = %q, want %q", diag.Fix, runningConfigCommand)
			}
			assertLiveRecordRemoval(t, diag.Message, "first send that withdrawal")
		})
	}
}

// assertLiveRecordRemoval checks the facts an operator needs to remove the
// record of a withdrawn rule: withdrawStep comes before the removal command,
// the removal works while ze runs, removing first leaves the rule untraced,
// and a running ddos-flowspec plugin refuses it, with the restart named.
func assertLiveRecordRemoval(t *testing.T, message, withdrawStep string) {
	t.Helper()
	removal := "then remove the record with ze data rm " + announcementRecordKey + ", which works while ze runs"
	withdrawAt := strings.Index(message, withdrawStep)
	removalAt := strings.Index(message, removal)
	if withdrawAt < 0 {
		t.Errorf("message %q does not name the withdrawal step %q", message, withdrawStep)
	}
	if removalAt < 0 {
		t.Errorf("message %q does not name the removal %q", message, removal)
	}
	if removalAt < withdrawAt {
		t.Errorf("message %q names the removal before the withdrawal", message)
	}
	if !strings.Contains(message, "a record removed before its rule is withdrawn leaves the rule at the peers with no trace in the store and no report to find it") {
		t.Errorf("message %q does not say why the withdrawal comes first", message)
	}
	if !strings.Contains(message, "ze refuses the removal while a ddos-flowspec plugin runs") {
		t.Errorf("message %q does not say that a running ddos-flowspec plugin refuses the removal", message)
	}
	if !strings.Contains(message, "then run "+restartTestFix) {
		t.Errorf("message %q does not name the restart %q for a running plugin", message, restartTestFix)
	}
	if strings.Contains(message, "while ze is stopped") {
		t.Errorf("message %q still says the removal needs ze stopped", message)
	}
}

// TestCorruptRecordReportNamesTheRemoval proves that the report of a record
// that does not decode keeps the read command as its fix and names, in order,
// the withdrawal of the rule the printed bytes name and the removal of the
// record, which works while ze runs when no ddos-flowspec plugin runs; while
// one runs, its next announcement replaces the record.
//
// VALIDATES: AC-6. The report is shared with the configured-section checks, so
// it says what happens while the plugin runs too.
// METHOD: corruptRecordDiagnostic for a decode error, read for the fix, for
// each fact, and for the order of the two steps.
// PREVENTS: a corrupt-record report that says the removal needs ze stopped, or
// that names a removal a running plugin refuses with no word on what clears
// the record then.
func TestCorruptRecordReportNamesTheRemoval(t *testing.T) {
	diag := corruptRecordDiagnostic(errCorruptTestRecord)

	if diag.Fix != recordReadTestFix {
		t.Errorf("fix = %q, want %q", diag.Fix, recordReadTestFix)
	}
	withdrawAt := strings.Index(diag.Message, "First withdraw the rule the printed bytes name at the peers")
	removalAt := strings.Index(diag.Message, "then remove the record with ze data rm "+announcementRecordKey)
	if withdrawAt < 0 {
		t.Errorf("message %q does not name the withdrawal of the rule the printed bytes name", diag.Message)
	}
	if removalAt < 0 {
		t.Errorf("message %q does not name the removal of the record", diag.Message)
	}
	if removalAt < withdrawAt {
		t.Errorf("message %q names the removal before the withdrawal", diag.Message)
	}
	if !strings.Contains(diag.Message, "which works while ze runs when no ddos-flowspec plugin runs") {
		t.Errorf("message %q does not say the removal works while ze runs", diag.Message)
	}
	if !strings.Contains(diag.Message, "While one runs, ze refuses the removal, and the plugin's next announcement replaces the record") {
		t.Errorf("message %q does not say what replaces the record while the plugin runs", diag.Message)
	}
	if strings.Contains(diag.Message, "while ze is stopped") {
		t.Errorf("message %q still says the removal needs ze stopped", diag.Message)
	}
}

// errCorruptTestRecord is the decode error the corrupt-record report is built
// from in TestCorruptRecordReportNamesTheRemoval.
var errCorruptTestRecord = errors.New("decode announcement record: invalid character 'o' in literal null")

// TestRestartWithdrawsTheOrphanAndClearsTheReport proves the fix the runtime
// report prints: a plugin whose start-time withdrawal failed reports the record
// with the restart as its fix, and the restarted plugin withdraws the rule,
// removes the record, and then reports nothing.
//
// VALIDATES: AC-16, "the report clears when the record is removed", through the
// owner's fix for a running plugin (`ze request plugin restart`). The restart
// (handlePluginRestart -> restartPlugin) stops the process with no `bye` and
// runs a new one through the whole handshake, so its OnStarted withdraws the
// record (withdrawRecordedAnnouncement).
// METHOD: two runs of runEngine against one stub store, as the restart runs two
// processes against the daemon's store. The first run's engine refuses every
// update-route, so the start-time withdrawal fails and the record stays. Its
// connection then closes with no `bye`, as Process.Stop does. The second run's
// engine accepts.
// PREVENTS: a doctor report whose fix leaves the report standing.
func TestRestartWithdrawsTheOrphanAndClearsTheReport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	record, err := encodeAnnouncementRecord(orphanTestMatch())
	if err != nil {
		t.Fatalf("encode the record: %v", err)
	}
	store := newStubStore(map[string][]byte{announcementRecordKey: record})

	first, _ := startEngineStubOn(t, store,
		"send refused: no peer this selector names attaches this process with the required send permission")
	first.waitRequest(ctx, rpc.MethodUpdateRoute)
	before := first.doctorCheck(ctx, orphanRuntimeCheck)
	if len(before) != 1 {
		t.Fatalf("setup: findings after the refused start-time withdrawal = %v, want one", before)
	}
	if before[0].Fix != restartTestFix {
		t.Fatalf("setup: fix = %q, want %q", before[0].Fix, restartTestFix)
	}
	first.stop(ctx)

	second, _ := startEngineStubOn(t, store, "")
	second.waitRequest(ctx, rpc.MethodStateRemove)

	if got := countCommands(second.routeCommands(), "/flow del "); got != 1 {
		t.Fatalf("withdrawals of %s by the restarted plugin = %d, want 1 (commands: %v)",
			exitTestVictim, got, second.routeCommands())
	}
	if store.holdsAnnouncementRecord() {
		t.Fatalf("the record %s is still stored after the restarted plugin withdrew the rule", announcementRecordKey)
	}
	if after := second.doctorCheck(ctx, orphanRuntimeCheck); len(after) != 0 {
		t.Fatalf("findings after the restart = %v, want none", after)
	}
}

// TestOrphanCheckIsQuietForTheLiveAnnouncement proves that the runtime check
// reports nothing for the record of the rule the live responder holds.
//
// VALIDATES: AC-16, negative: a running mitigation is recorded by design.
// METHOD: the callback's answer function with the record equal to the live match.
// PREVENTS: a doctor warning during every mitigation.
func TestOrphanCheckIsQuietForTheLiveAnnouncement(t *testing.T) {
	m := orphanTestMatch()
	disp := &fakeDispatcher{recorded: &m}

	diags, err := answerOrphanRuntimeCheck(orphanRuntimeCheck, disp, m, true)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("findings for the live announcement = %v, want none", diags)
	}
}

// TestOrphanRecordCheckReportsARecordWithTheSectionAbsent proves that the
// registered check reports a stored record while ddos flowspec is not
// configured, when no plugin runs to answer the runtime check.
//
// VALIDATES: AC-16 with the plugin's process ended (A-13 broken: the daemon asks
// no removed plugin for its declared checks).
// METHOD: a real store holding a record written by the producer's encoder, and
// a config tree with no ddos flowspec section.
// PREVENTS: a recorded rule invisible to `ze doctor` once its plugin is gone.
func TestOrphanRecordCheckReportsARecordWithTheSectionAbsent(t *testing.T) {
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

	diags := checkOrphanRecord(diagnostic.DoctorCheckContext{Tree: config.NewTree(), Store: store})

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != codeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, codeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if diags[0].Fix != orphanTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, orphanTestFix)
	}
}

// recordReadTestFix is the command a doctor run that cannot judge the record
// must name: it prints the stored record.
const recordReadTestFix = "ze data cat meta/ddos-flowspec/announcement"

// TestOrphanRecordCheckWithNoStoreNamesTheReadCommand proves that a doctor run
// with no storage refuses the record check by name and gives the operator the
// command that reads the record.
//
// VALIDATES: ai/rules/user-facing-errors.md (every finding names the command
// that clears it), for the doctor-storage-unavailable refusal.
// METHOD: checkOrphanRecord with a config tree holding no ddos flowspec section
// and a nil store.
// PREVENTS: a refusal that leaves the operator nothing to run.
func TestOrphanRecordCheckWithNoStoreNamesTheReadCommand(t *testing.T) {
	diags := checkOrphanRecord(diagnostic.DoctorCheckContext{Tree: config.NewTree()})

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorStorageUnavailable {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorStorageUnavailable)
	}
	if diags[0].Fix != recordReadTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, recordReadTestFix)
	}
}

// TestOrphanRecordCheckWithNoTreeNamesTheReadCommand proves that a doctor run
// holding no parsed configuration says it cannot judge the record and gives the
// command that reads it.
//
// VALIDATES: ai/rules/user-facing-errors.md, for the no-configuration refusal.
// METHOD: checkOrphanRecord with no tree.
// PREVENTS: a refusal that leaves the operator nothing to run, and a silent nil
// read as "no record".
func TestOrphanRecordCheckWithNoTreeNamesTheReadCommand(t *testing.T) {
	diags := checkOrphanRecord(diagnostic.DoctorCheckContext{})

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != codeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, codeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if diags[0].Fix != recordReadTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, recordReadTestFix)
	}
}
