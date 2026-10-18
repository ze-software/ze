// Design: bmp_events.go -- peerDownFor, which turns the reactor's close into a Peer Down reason
// Related: rfc7854_peerdown_data_test.go -- peerDownHarness and readPeerDown, the path these drive
//
// RFC 7854 Section 4.9 gives a Peer Down five reasons. The NOTIFICATION decides reasons 1
// and 3; these tests cover the two the reactor's close reason decides when no NOTIFICATION
// crossed the wire: a lost transport, and a session ze closed itself.

package bmp

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// closeEvent is a state-down event carrying the reactor's close reason and FSM event.
func closeEvent(reason string, fsmEvent uint16) *rpc.StructuredEvent {
	se := peerDownEvent(reason)
	se.FSMEvent = fsmEvent
	return se
}

// RFC requirement: RFC7854-x-10 positive -- a Peer Down for a session whose transport
// ended with no NOTIFICATION carries Reason 4 and no Data.
//
// RFC 7854 Section 4.9: "Reason 4: The remote system closed the session without a
// notification message.  This includes any unexpected termination of the transport
// session". The reactor names that close rpc.ReasonTCPFailure, with FSM Event 18.
//
// MUTATION: in peerDownFor, drop the rpc.ReasonTCPFailure branch so the loss falls to
// reason 2; this test goes red.
func TestRFC7854PeerDownTransportLossIsReason4(t *testing.T) {
	bp, server := peerDownHarness(t)
	recordPeerUpFor(t, bp, closeEvent(rpc.ReasonTCPFailure, 18))
	pd := readPeerDown(t, bp, server, closeEvent(rpc.ReasonTCPFailure, 18))

	if pd.Reason != PeerDownRemoteNoData {
		t.Fatalf("reason = %d, want %d: the transport ended with no NOTIFICATION", pd.Reason, PeerDownRemoteNoData)
	}
	if len(pd.Data) != 0 {
		t.Errorf("data = %x, want none: reason 4 has no Data field", pd.Data)
	}
}

// RFC requirement: RFC7854-4.9-1 positive -- a Peer Down for a session ze closed with no
// NOTIFICATION carries Reason 2 and, as its two-byte Data, the code of the FSM event that
// closed the session.
//
// RFC 7854 Section 4.9: "Reason 2: The local system closed the session.  No notification
// message was sent.  Following the reason code is a 2-byte field containing the code
// corresponding to the Finite State Machine (FSM) Event that caused the system to close
// the session (see Section 8.1 of [RFC4271])." The event here is RFC 4271 Event 8,
// AutomaticStop, whose Cease was never written.
//
// MUTATION: in peerDownFor, write the two zero octets in place of the event; this test
// goes red.
func TestRFC7854PeerDownLocalCloseCarriesTheFSMEvent(t *testing.T) {
	bp, server := peerDownHarness(t)
	recordPeerUpFor(t, bp, closeEvent(rpc.ReasonLocalClose, 8))
	pd := readPeerDown(t, bp, server, closeEvent(rpc.ReasonLocalClose, 8))

	if pd.Reason != PeerDownLocalNoNotify {
		t.Fatalf("reason = %d, want %d", pd.Reason, PeerDownLocalNoNotify)
	}
	if !bytes.Equal(pd.Data, []byte{0, 8}) {
		t.Errorf("data = %x, want 0008: FSM Event 8, AutomaticStop", pd.Data)
	}
}
