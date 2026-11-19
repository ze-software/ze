// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting under the L2TP session event order
// Related: session_reader_contract_test.go -- the engine bus and the reactor tripwire
// Related: acct_original_task_test.go -- the ordered RADIUS server and its wire-order check

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
// one Accounting-Start. The server receives that Start strictly before the
// Stop, and receives no record after the Stop. RFC 2866 Section 2: "At the
// end of service delivery the client will generate an Accounting Stop packet
// describing the type of service that was delivered and optionally
// statistics such as elapsed time, input and output octets, or input and
// output packets." A Start that reaches the server after the Stop opens a
// record nothing closes (spec-radius-acct-start-after-stop AC-1).
//
// Method: radiusAcct.subscribeEventBus subscribes the reader to the engine
// bus (server.Server) as runPlugin does, and the two events are published on
// that bus, back to back, so onSessionDown runs while the accounting task
// onSessionIPAssigned started may not have sent its Start yet. The RADIUS
// server is the ordered server of acct_original_task_test.go, holding no
// record, with one reader that captures every Accounting-Request in the order
// it arrives and before it acknowledges it. acct.Stop joins the session's
// accounting task, so every record it sends is captured before
// assertOrderedAccountingWire reads the whole capture: exactly Start then
// Stop, both carrying the Start's Acct-Session-Id.
//
// What it does not prove: the order of the two events. The reactor is
// unexported and this package cannot drive it, so the test publishes the
// order rather than provoking it. That the reactor never lets a SessionDown
// overtake the SessionIPAssigned before it is proven in
// internal/component/l2tp/session_event_order_test.go, and through the
// daemon by test/l2tp/session-clear-leaves-no-subscriber.ci.
//
// The behavior is preserved, not new: the test passes before and after the
// reactor's order fix.
//
// MUTATION: in runAccounting (acct.go), wait on sess.ended and send the Stop
// before the Start (move `<-sess.ended` and the sendAcctStop part above the
// sendAcctStart part). The Down has already closed sess.ended, so the wire
// carries Stop then Start: one of each with the same Acct-Session-Id, so every
// count and identity check above passes, and assertOrderedAccountingWire
// fails with "record 0 status=2, want 1".
func TestTeardownDuringIPAssignedLeavesNoAccountingSession(t *testing.T) {
	bus := newSessionNestingBus(t)
	acct, server := newOrderedAccounting(t, 0, 300*time.Second)
	capture := server.capture
	t.Cleanup(acct.subscribeEventBus(bus))

	const tunnelID, sessionID = uint16(11), uint16(7)
	if _, err := l2tpevents.SessionIPAssigned.Emit(bus, &l2tpevents.SessionIPAssignedPayload{
		SubscriberID: "l2tp-11-7",
		TunnelID:     tunnelID, SessionID: sessionID, Username: "grace",
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

	// The held-session checks above read a.sessions, which Stop empties, so
	// they run first. Stop then joins the session's task: no record it could
	// still send is missing from the capture the wire check reads.
	acct.Stop()
	assertOrderedAccountingWire(t, server, []uint8{radius.AcctStatusStart, radius.AcctStatusStop}, starts[0].sessionID)
}
