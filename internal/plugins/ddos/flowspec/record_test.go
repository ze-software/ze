// Design: docs/guide/ddos-mitigation.md -- the live-announcement record
// Related: exit_test.go (engineStub), responder.go (announce, withdraw)

package flowspec

import (
	"bytes"
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
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
