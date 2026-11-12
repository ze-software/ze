// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting under the L2TP session event order
// Related: session_reader_contract_test.go -- the engine bus and the reactor tripwire

package l2tpauthradius

import (
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
)

// TestTeardownDuringIPAssignedLeavesNoAccountingSession proves AC-8 at RADIUS
// accounting, for the reader: when a session's SessionIPAssigned and then its
// SessionDown reach the accounting reader in that order, which is the order
// the reactor guarantees for a teardown that lands while the IP-assigned
// event is being handled, accounting holds no session for it afterwards and
// has sent exactly one Accounting-Stop, carrying the Acct-Session-Id of its
// one Accounting-Start.
//
// Method: radiusAcct.subscribeEventBus subscribes the reader to the engine
// bus (server.Server) as runPlugin does, and the two events are published on
// that bus, back to back, so onSessionDown runs while the accounting task
// onSessionIPAssigned started may not have sent its Start yet. The RADIUS
// server is a local capture of every Accounting-Request.
//
// What it does not prove: the order itself. The reactor is unexported and
// this package cannot drive it, so the test publishes the order rather than
// provoking it. That the reactor never lets a SessionDown overtake the
// SessionIPAssigned before it is proven in
// internal/component/l2tp/session_event_order_test.go, and through the
// daemon by test/l2tp/session-clear-leaves-no-subscriber.ci. Nor does it
// assert the order of the two records on the wire: onSessionDown sends the
// Stop synchronously while runAccounting sends the Start from its own
// goroutine, so either may arrive first.
//
// The behavior is preserved, not new: the test passes before and after the
// reactor's order fix.
func TestTeardownDuringIPAssignedLeavesNoAccountingSession(t *testing.T) {
	bus := newSessionNestingBus(t)
	acct, capture := newCounterAccounting(t, 300*time.Second)
	t.Cleanup(acct.subscribeEventBus(bus))

	const tunnelID, sessionID = uint16(11), uint16(7)
	if _, err := l2tpevents.SessionIPAssigned.Emit(bus, &l2tpevents.SessionIPAssignedPayload{
		TunnelID: tunnelID, SessionID: sessionID, Username: "grace",
		PeerAddr: "192.0.2.7", PppInterface: "ppp7",
	}); err != nil {
		t.Fatalf("publish SessionIPAssigned: %v", err)
	}
	if _, err := l2tpevents.SessionDown.Emit(bus, &l2tpevents.SessionDownPayload{
		TunnelID: tunnelID, SessionID: sessionID, Username: "grace",
		Cause: l2tpevents.TerminateCauseAdminReset,
	}); err != nil {
		t.Fatalf("publish SessionDown: %v", err)
	}

	packets := capture.waitN(t, 2)
	var starts, stops []capturedAcct
	for _, packet := range packets {
		switch packet.statusType {
		case radius.AcctStatusStart:
			starts = append(starts, packet)
		case radius.AcctStatusStop:
			stops = append(stops, packet)
		default:
			t.Fatalf("unexpected Acct-Status-Type %d for a session that lived for one handler", packet.statusType)
		}
	}
	if len(starts) != 1 {
		t.Fatalf("Accounting-Start records = %d, want 1", len(starts))
	}
	if len(stops) != 1 {
		t.Fatalf("Accounting-Stop records = %d, want 1 for the one Start", len(stops))
	}
	if stops[0].sessionID != starts[0].sessionID {
		t.Fatalf("the Stop carries Acct-Session-Id %q, the Start %q: the Stop does not close the Start",
			stops[0].sessionID, starts[0].sessionID)
	}

	acct.mu.Lock()
	_, held := acct.sessions[sessionKey{tunnelID: tunnelID, sessionID: sessionID}]
	remaining := len(acct.sessions)
	acct.mu.Unlock()
	if held {
		t.Fatalf("accounting still holds session %d/%d after its SessionDown", tunnelID, sessionID)
	}
	if remaining != 0 {
		t.Fatalf("accounting holds %d sessions after the only one went down, want 0", remaining)
	}
}
