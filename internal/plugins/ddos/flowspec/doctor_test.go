// Design: docs/guide/ddos-mitigation.md -- a FlowSpec rule no responder owns
// Related: doctor.go (answerOrphanRuntimeCheck, checkOrphanRecord)

package flowspec

import (
	"context"
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
	if diags[0].Code != diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if !strings.Contains(diags[0].Message, exitTestVictim) {
		t.Errorf("message %q does not name the victim %s", diags[0].Message, exitTestVictim)
	}
	if diags[0].Fix != restartTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, restartTestFix)
	}
}

// TestRuntimeOrphanReportNamesBothCausesOfAFailedWithdrawal proves that the
// runtime report says why the record outlived the withdrawal in words true for
// all three causes: an engine that did not take the withdrawal, an engine that
// took it and refused the send (test/plugin/ddos-flowspec-orphan-doctor.ci,
// where the send permission refuses it), and a withdrawal the engine sent
// whose record removal then failed (responder.go withdrawThrough and
// withdrawRecordedRule, where Forget fails after a successful Dispatch). The
// name predates the third cause.
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

	const reason = "the last withdrawal of the rule did not reach the BGP peers, because the BGP engine did not take it or refused to send it"
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
// VALIDATES: ai/rules/user-facing-errors.md for the corrupt record: `ze data rm`
// is refused while the daemon owns the store, and no plugin start withdraws
// from a record it cannot decode, so neither clears it.
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

// TestRemovedSectionReportSaysWhatRemovesTheRecord proves that the report made
// while ddos flowspec is not configured says its withdrawal leaves the record,
// and that the store's removal works only while ze is stopped.
//
// VALIDATES: AC-16's "the report clears when the record is removed", as things
// work while no plugin runs: the fix withdraws the rule, and the message names
// what removes the record.
// METHOD: the removed-section report for orphanTestMatch.
// PREVENTS: a report that presents `ze data rm` as a step the running daemon
// takes, when the store refuses a second writer.
func TestRemovedSectionReportSaysWhatRemovesTheRecord(t *testing.T) {
	diag := unconfiguredOrphanDiagnostic(orphanTestMatch())

	if !strings.Contains(diag.Message, "ze data rm "+announcementRecordKey+" removes the record only while ze is stopped") {
		t.Errorf("message %q does not say that ze data rm removes the record only while ze is stopped", diag.Message)
	}
	if !strings.Contains(diag.Message, "configure ddos flowspec again") {
		t.Errorf("message %q does not name the plugin start that removes the record", diag.Message)
	}
}

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
	if diags[0].Code != diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement)
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
	if diags[0].Code != diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if diags[0].Fix != recordReadTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, recordReadTestFix)
	}
}
