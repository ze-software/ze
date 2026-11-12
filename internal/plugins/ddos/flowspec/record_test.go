// Design: docs/guide/ddos-mitigation.md -- the live-announcement record
// Related: exit_test.go (engineStub), responder.go (announce, withdraw)

package flowspec

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// announcementRecordKey is the runtime state key of the live-announcement
// record, written out here rather than read from the producer's constant: it is
// the name an operator reads in `ze data registered`, so a rename is a contract
// change this test reports.
const announcementRecordKey = "meta/ddos-flowspec/announcement"

// stateCallsFor returns the calls of method on the announcement record key.
func stateCallsFor(calls []stateCallRecord, method string) []stateCallRecord {
	var out []stateCallRecord
	for _, c := range calls {
		if c.method == method && c.key == announcementRecordKey {
			out = append(out, c)
		}
	}
	return out
}

// TestAnnounceRecordsTheRule proves that a successful announcement is recorded
// in the plugin's runtime state before the event that caused it has been
// handled, and that the record names the victim prefix.
//
// VALIDATES: AC-14, write half. The record is what lets a later start withdraw
// a rule a daemon stop or a kill left at the peers (AC-15).
// METHOD: runEngine over a real sdk.Plugin pair; one announcement through the
// bus. The bus runs the handler on the emitting goroutine and the stub records
// a call before answering it, so the record is visible when Emit returns.
// PREVENTS: plan/journal/component-rebuilt-during-reload.md (2026-11-11): a rule
// the plugin could not withdraw was remembered nowhere, so the next start did
// not know it existed.
func TestAnnounceRecordsTheRule(t *testing.T) {
	e, bus := startEngineStub(t, nil)
	announceVictim(t, e, bus)

	puts := stateCallsFor(e.stateCalls(), rpc.MethodStatePut)
	if len(puts) != 1 {
		t.Fatalf("writes of %s after one announcement = %d, want 1 (state calls: %v)",
			announcementRecordKey, len(puts), e.stateCalls())
	}
	if !bytes.Contains(puts[0].data, []byte(exitTestVictim)) {
		t.Fatalf("the record %q does not name the victim %s", puts[0].data, exitTestVictim)
	}
}

// TestWithdrawRemovesTheRecord proves that a withdrawal removes the
// live-announcement record, so a later start finds nothing to withdraw.
//
// VALIDATES: AC-14, remove half, through the withdrawal a characterized
// exemption makes after a blackhole fallback (onCharacterized -> withdraw).
// METHOD: runEngine over a real sdk.Plugin pair; announce, then the exemption,
// then read the state calls that followed the withdrawal.
// PREVENTS: a record left behind by a withdrawal, which the next start would
// turn into a second withdrawal and `ze doctor` into a false report.
func TestWithdrawRemovesTheRecord(t *testing.T) {
	e, bus := startEngineStub(t, nil)
	announceVictim(t, e, bus)

	if _, err := ddosevent.Characterized.Emit(bus, &ddosevent.AttackCharacterized{
		Interface:          "xe0",
		Target:             ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(exitTestVictim), Proto: 17},
		Family:             ddosevent.FamilyUDPFlood,
		SuppressMitigation: true,
	}); err != nil {
		t.Fatalf("emit attack-characterized: %v", err)
	}
	if got := countCommands(e.routeCommands(), "/flow del "); got != 1 {
		t.Fatalf("setup: withdrawals for %s = %d, want 1 (commands: %v)", exitTestVictim, got, e.routeCommands())
	}

	if removes := stateCallsFor(e.stateCalls(), rpc.MethodStateRemove); len(removes) != 1 {
		t.Fatalf("removals of %s after the withdrawal = %d, want 1 (state calls: %v)",
			announcementRecordKey, len(removes), e.stateCalls())
	}
}

// earlierRecordedRule is a rule a failed withdrawal left recorded, for another
// victim than exitTestVictim, so a command that names it can only be its
// withdrawal.
func earlierRecordedRule() flowspecMatch {
	return buildMatch(ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("198.51.100.7/32"), Proto: 6})
}

// enforcingResponder returns a responder at response-level enforce with the
// blackhole fallback, so one critical remote detection announces.
func enforcingResponder(d routeDispatcher) *responder {
	cfg := DefaultConfig()
	cfg.ResponseLevel = responseEnforce
	cfg.Action = actionDiscard
	cfg.BlackholeFallback = true
	return newResponder(cfg, d)
}

// detectVictim is the critical remote detection the blackhole fallback
// announces against exitTestVictim.
func detectVictim(r *responder) {
	r.onDetected(&ddosevent.AttackDetected{
		Interface: "xe0",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(exitTestVictim), Proto: 17},
		Family:    ddosevent.FamilyUDPFlood,
		Severity:  ddosevent.SeverityCritical,
		Direction: ddosevent.DirectionRemote,
	})
}

// TestAnnouncementWithdrawsAnEarlierRecordedRuleFirst proves that a new
// announcement withdraws the rule a failed withdrawal left recorded before it
// records its own rule over it.
//
// VALIDATES: AC-14 with AC-16's condition (a record no live responder holds):
// one key holds one record, so a rule at the peers is always either live or
// recorded only if the earlier rule leaves the peers before the record names
// the new one.
// METHOD: a responder over a fake whose record names another victim; one
// critical detection; the commands in order and the record after.
// PREVENTS: an announcement that overwrites the record of a rule still at the
// peers, which then stays there with nothing on the box remembering it.
func TestAnnouncementWithdrawsAnEarlierRecordedRuleFirst(t *testing.T) {
	earlier := earlierRecordedRule()
	disp := &fakeDispatcher{recorded: &earlier}

	detectVictim(enforcingResponder(disp))

	if len(disp.cmds) != 2 {
		t.Fatalf("commands = %v, want the earlier rule's withdrawal, then the announcement", disp.cmds)
	}
	want := renderFlowspecCommand(earlier, "", 0, "del")
	if disp.cmds[0] != want {
		t.Fatalf("first command = %q, want the earlier rule's withdrawal %q", disp.cmds[0], want)
	}
	if countCommands(disp.cmds[1:], "/flow add ") != 1 {
		t.Fatalf("second command = %q, want the announcement for %s", disp.cmds[1], exitTestVictim)
	}
	if disp.recorded == nil || disp.recorded.DstPrefix.String() != exitTestVictim {
		t.Fatalf("record after the announcement = %v, want the rule for %s", disp.recorded, exitTestVictim)
	}
}

// TestAnnouncementWaitsWhileAnEarlierRecordedRuleCannotBeWithdrawn proves that
// no rule is announced while the withdrawal of an earlier recorded rule fails,
// and that the record still names the earlier rule.
//
// VALIDATES: the refusal half of TestAnnouncementWithdrawsAnEarlierRecordedRuleFirst:
// a rule at the peers stays recorded until its withdrawal reaches the engine.
// METHOD: a fake that refuses the earlier rule's withdrawal and accepts the
// rest; one critical detection.
// PREVENTS: a new rule announced and recorded over an earlier rule the engine
// may still hold.
func TestAnnouncementWaitsWhileAnEarlierRecordedRuleCannotBeWithdrawn(t *testing.T) {
	earlier := earlierRecordedRule()
	disp := &fakeDispatcher{recorded: &earlier, refuse: earlier.DstPrefix.String()}
	r := enforcingResponder(disp)

	detectVictim(r)

	if got := countCommands(disp.cmds, "/flow add "); got != 0 {
		t.Fatalf("announcements while the earlier rule could not be withdrawn = %d, want 0 (commands: %v)", got, disp.cmds)
	}
	if disp.recorded == nil || *disp.recorded != earlier {
		t.Fatalf("record = %v, want the earlier rule %v still recorded", disp.recorded, earlier)
	}
	if active, _, _ := r.status(); active {
		t.Fatalf("the responder claims a live announcement it never sent")
	}
}

// TestAnnouncementOfTheRecordedRuleSendsNoWithdrawal proves that a record of
// the rule about to be announced is not withdrawn first.
//
// VALIDATES: the negative partner of TestAnnouncementWithdrawsAnEarlierRecordedRuleFirst:
// the announcement replaces the same NLRI at the peers, so a withdrawal would
// only lift the rule for a moment.
// METHOD: a fake whose record is the match the detection announces.
// PREVENTS: a withdrawal before every announcement of a rule left recorded.
func TestAnnouncementOfTheRecordedRuleSendsNoWithdrawal(t *testing.T) {
	same := buildMatch(ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(exitTestVictim), Proto: 17})
	disp := &fakeDispatcher{recorded: &same}

	detectVictim(enforcingResponder(disp))

	if got := countCommands(disp.cmds, "/flow del "); got != 0 {
		t.Fatalf("withdrawals before announcing the recorded rule = %d, want 0 (commands: %v)", got, disp.cmds)
	}
	if got := countCommands(disp.cmds, "/flow add "); got != 1 {
		t.Fatalf("announcements = %d, want 1 (commands: %v)", got, disp.cmds)
	}
}

// unreadableRecordDispatcher answers every read of the record with an error,
// as the state call does when the engine cannot answer it.
type unreadableRecordDispatcher struct{ *fakeDispatcher }

func (unreadableRecordDispatcher) Recorded() ([]byte, bool, error) {
	return nil, false, errors.New("state-get: the engine did not answer")
}

// captureFlowspecLogs points the plugin logger at a buffer for the test's life.
func captureFlowspecLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previous := loggerPtr.Load()
	setLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { loggerPtr.Store(previous) })
	return &logs
}

// recordConsequence is the phrase the WARN line of an announcement over a
// record it could not use carries: what recording the announcement costs.
const recordConsequence = "can no longer be withdrawn from its record"

// orphanCodeWarnings returns the WARN lines that carry the orphan code.
func orphanCodeWarnings(logs string) []string {
	var out []string
	for line := range strings.SplitSeq(strings.TrimSpace(logs), "\n") {
		if !strings.Contains(line, "level=WARN") {
			continue
		}
		if strings.Contains(line, "code="+diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement) {
			out = append(out, line)
		}
	}
	return out
}

// assertAnnouncedOverTheRecord fails unless the detection announced exactly
// once, withdrew nothing, and recorded its own rule.
func assertAnnouncedOverTheRecord(t *testing.T, disp *fakeDispatcher, r *responder) {
	t.Helper()
	if got := countCommands(disp.cmds, "/flow add "); got != 1 {
		t.Fatalf("announcements = %d, want 1: mitigation is never refused over the record (commands: %v)", got, disp.cmds)
	}
	if got := countCommands(disp.cmds, "/flow del "); got != 0 {
		t.Fatalf("withdrawals = %d, want 0: the record names no rule to withdraw (commands: %v)", got, disp.cmds)
	}
	if active, _, _ := r.status(); !active {
		t.Fatalf("the responder holds no live announcement after announcing")
	}
	if disp.recorded == nil || disp.recorded.DstPrefix.String() != exitTestVictim {
		t.Fatalf("record after the announcement = %v, want the rule for %s", disp.recorded, exitTestVictim)
	}
}

// TestAnnouncementOverAnUnreadableRecordGoesOutAndLogsTheConsequence proves
// that an announcement goes out while the record cannot be read, and that its
// one WARN line says what recording it over that record costs.
//
// VALIDATES: R-8 (the announcement still goes out over record trouble) and
// ai/rules/principles.md: a read that failed is not read as "no earlier rule";
// the line says the earlier rule, if one was recorded, can no longer be
// withdrawn from its record, and gives the withdrawal by hand.
// METHOD: a responder over a fake whose Recorded fails; one critical
// detection; the commands, the record and the WARN lines under the orphan code.
// PREVENTS: an unreadable record treated as an absent one, in silence or with
// a line that says nothing of the rule it may have named.
func TestAnnouncementOverAnUnreadableRecordGoesOutAndLogsTheConsequence(t *testing.T) {
	logs := captureFlowspecLogs(t)
	disp := &fakeDispatcher{}
	r := enforcingResponder(unreadableRecordDispatcher{disp})

	detectVictim(r)

	assertAnnouncedOverTheRecord(t, disp, r)
	warnings := orphanCodeWarnings(logs.String())
	if len(warnings) != 1 {
		t.Fatalf("WARN lines under the orphan code = %d, want 1, one line per event (log: %s)", len(warnings), logs.String())
	}
	if !strings.Contains(warnings[0], recordConsequence) {
		t.Errorf("the WARN line %q does not say the earlier rule %s", warnings[0], recordConsequence)
	}
	if !strings.Contains(warnings[0], "state-get: the engine did not answer") {
		t.Errorf("the WARN line %q does not carry the read error", warnings[0])
	}
	if !strings.Contains(warnings[0], "fix=") {
		t.Errorf("the WARN line %q names no fix", warnings[0])
	}
}

// TestAnnouncementOverACorruptRecordGoesOutAndLogsTheConsequence proves that
// an announcement goes out over a record that does not decode, and that its
// one WARN line says what recording it over that record costs, with the bytes
// the record held.
//
// VALIDATES: R-8 and ai/rules/principles.md, as for the unreadable record. The
// stored bytes can still show the earlier rule's victim, and recording the
// announcement removes them from the store, so the line keeps them.
// METHOD: a responder over a fake whose Recorded answers bytes that are not a
// record; one critical detection; the commands, the record and the WARN lines.
// PREVENTS: a corrupt record overwritten with no word of the rule it may have
// named, after which nothing on the box can show that rule.
func TestAnnouncementOverACorruptRecordGoesOutAndLogsTheConsequence(t *testing.T) {
	logs := captureFlowspecLogs(t)
	disp := &fakeDispatcher{}
	r := enforcingResponder(corruptRecordDispatcher{disp})

	detectVictim(r)

	assertAnnouncedOverTheRecord(t, disp, r)
	warnings := orphanCodeWarnings(logs.String())
	if len(warnings) != 1 {
		t.Fatalf("WARN lines under the orphan code = %d, want 1, one line per event (log: %s)", len(warnings), logs.String())
	}
	if !strings.Contains(warnings[0], recordConsequence) {
		t.Errorf("the WARN line %q does not say the earlier rule %s", warnings[0], recordConsequence)
	}
	if !strings.Contains(warnings[0], `record="not json"`) {
		t.Errorf("the WARN line %q does not carry the stored bytes of the corrupt record", warnings[0])
	}
	if !strings.Contains(warnings[0], "fix=") {
		t.Errorf("the WARN line %q names no fix", warnings[0])
	}
}

// TestStartWithdrawsTheRecordedAnnouncement proves that a plugin starting with
// a recorded rule withdraws it, removes the record, and adopts nothing.
//
// VALIDATES: AC-15. A daemon stop or a kill left the rule at the peers with the
// record in the store; the next start is where it is withdrawn.
// METHOD: runEngine over a real sdk.Plugin pair whose store holds a record
// written by the producer's own encoder; the test waits for the record's
// removal, then reads what the engine received.
// PREVENTS: plan/journal/component-rebuilt-during-reload.md (2026-11-11): a
// restarted plugin did not know the rule existed, so nothing withdrew it.
func TestStartWithdrawsTheRecordedAnnouncement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	record, err := encodeAnnouncementRecord(buildMatch(ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix(exitTestVictim), Proto: 17,
	}))
	if err != nil {
		t.Fatalf("encode the record: %v", err)
	}
	e, _ := startEngineStub(t, map[string][]byte{announcementRecordKey: record})

	e.waitRequest(ctx, rpc.MethodStateRemove)

	cmds := e.routeCommands()
	if got := countCommands(cmds, "/flow del "); got != 1 {
		t.Fatalf("withdrawals of the recorded %s at start = %d, want 1 (commands: %v)", exitTestVictim, got, cmds)
	}
	if got := countCommands(cmds, "/flow add "); got != 0 {
		t.Fatalf("announcements at start = %d, want 0: a recorded rule is withdrawn, never adopted (commands: %v)", got, cmds)
	}
	if r := activeResponder.Load(); r != nil {
		if active, _, _ := r.status(); active {
			t.Fatalf("the responder claims a live announcement after the start-time withdrawal")
		}
	}
}
