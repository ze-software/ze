// Design: bmp_events.go -- clearPeerState, peerDownFor, the producers under test
//
// RFC 7854 Section 4.9 draws the Peer Down body as "Reason" followed by "Data
// (present if Reason = 1, 2 or 3)". These tests drive the collector-facing path
// end to end: a NOTIFICATION event, then the state-down event, then the bytes a
// collector reads off the socket.

package bmp

import (
	"bytes"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// peerDownHarness gives a plugin with one collector session on a net.Pipe, and
// the server end a test reads the emitted messages from.
func peerDownHarness(t *testing.T) (*BMPPlugin, net.Conn) {
	t.Helper()

	server, client := net.Pipe()
	t.Cleanup(func() { closeLog(server, "server") })
	t.Cleanup(func() { closeLog(client, "client") })

	bp := &BMPPlugin{
		state:       newBMPState(),
		openCache:   make(map[string]*openPair),
		notifyCache: make(map[string]*notifyPDU),
		stopCh:      make(chan struct{}),
		senders: []*senderSession{{
			name:   "test",
			conn:   client,
			stopCh: make(chan struct{}),
		}},
	}
	return bp, server
}

// notificationEvent is the event the reactor delivers for one NOTIFICATION.
func notificationEvent(direction rpc.MessageDirection, body []byte) *rpc.StructuredEvent {
	return &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindNotification,
		Direction:   direction,
		RawMessage:  &bgptypes.RawMessage{Type: msgtype.TypeNOTIFICATION, RawBytes: body},
	}
}

// peerDownEvent is the state event that follows it.
func peerDownEvent(reason string) *rpc.StructuredEvent {
	return &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindState,
		State:       rpc.SessionStateDown,
		Reason:      reason,
	}
}

// readPeerDown drives the events and returns the Peer Down the collector reads.
func readPeerDown(t *testing.T, bp *BMPPlugin, server net.Conn, events ...*rpc.StructuredEvent) *PeerDown {
	t.Helper()

	for _, se := range events[:len(events)-1] {
		bp.handleStructuredEvent(se)
	}

	result := asyncRead(server)
	bp.handleStructuredEvent(events[len(events)-1])

	r := <-result
	if r.err != nil {
		t.Fatalf("read: %v", r.err)
	}
	pd, ok := r.msg.(*PeerDown)
	if !ok {
		t.Fatalf("expected *PeerDown, got %T", r.msg)
	}
	return pd
}

// RFC requirement: RFC7854-4.9-1 positive -- a Peer Down whose reason is 1 or 3
// carries the BGP NOTIFICATION PDU as its Data, and a Peer Down whose reason is
// 2 carries the two-byte FSM event code.
//
// RFC 7854 Section 4.9: "Reason 1: The local system closed the session.
// Following the Reason is a BGP PDU containing a BGP NOTIFICATION message that
// would have been sent to the peer", "Reason 3: The remote system closed the
// session with a notification message.  Following the Reason is a BGP PDU
// containing the BGP NOTIFICATION message as received from the peer", and
// "Reason 2: ... Following the reason code is a 2-byte field containing the
// code corresponding to the Finite State Machine (FSM) Event that caused the
// system to close the session ... Two bytes both set to 0 are used to indicate
// that no relevant Event code is defined."
func TestRFC7854PeerDownCarriesTheDataItsReasonRequires(t *testing.T) {
	body := []byte{6, 2} // Cease / Administrative Shutdown

	t.Run("reason 1 carries the notification ze sent", func(t *testing.T) {
		bp, server := peerDownHarness(t)
		pd := readPeerDown(t, bp, server,
			notificationEvent(rpc.DirectionSent, body),
			peerDownEvent("session closed"))

		if pd.Reason != PeerDownLocalNotify {
			t.Fatalf("reason = %d, want %d", pd.Reason, PeerDownLocalNotify)
		}
		want := bgpPDU(msgtype.TypeNOTIFICATION, body)
		if !bytes.Equal(pd.Data, want) {
			t.Errorf("data = %x, want the NOTIFICATION PDU %x", pd.Data, want)
		}
	})

	t.Run("reason 3 carries the notification the peer sent", func(t *testing.T) {
		bp, server := peerDownHarness(t)
		pd := readPeerDown(t, bp, server,
			notificationEvent(rpc.DirectionReceived, body),
			peerDownEvent("connection lost"))

		if pd.Reason != PeerDownRemoteNotify {
			t.Fatalf("reason = %d, want %d", pd.Reason, PeerDownRemoteNotify)
		}
		want := bgpPDU(msgtype.TypeNOTIFICATION, body)
		if !bytes.Equal(pd.Data, want) {
			t.Errorf("data = %x, want the NOTIFICATION PDU %x", pd.Data, want)
		}
	})

	t.Run("reason 2 carries the two-byte event code", func(t *testing.T) {
		bp, server := peerDownHarness(t)
		pd := readPeerDown(t, bp, server, peerDownEvent("connection lost"))

		if pd.Reason != PeerDownLocalNoNotify {
			t.Fatalf("reason = %d, want %d", pd.Reason, PeerDownLocalNoNotify)
		}
		if !bytes.Equal(pd.Data, []byte{0, 0}) {
			t.Errorf("data = %x, want two zero bytes", pd.Data)
		}
	})
}

// RFC requirement: RFC7854-4.9-1 negative -- a reason the figure gives no Data
// field carries none, so a collector that reads a Data field after reason 5 is
// reading a message ze never wrote.
//
// RFC 7854 Section 4.9 draws the body as "Reason" then "Data (present if Reason
// = 1, 2 or 3)", and reason 5 is "Information for this peer will no longer be
// sent to the monitoring station for configuration reasons."
//
// This is the polarity that keeps the positive case from being satisfied by an
// encoder that always appends something: the same path must produce an empty
// Data for the one reason that forbids it.
func TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone(t *testing.T) {
	bp, server := peerDownHarness(t)
	pd := readPeerDown(t, bp, server, peerDownEvent(rpc.ReasonPeerRemoved))

	if pd.Reason != PeerDownDeconfigured {
		t.Fatalf("reason = %d, want %d", pd.Reason, PeerDownDeconfigured)
	}
	if len(pd.Data) != 0 {
		t.Errorf("data = %x, want none: reason 5 has no Data field", pd.Data)
	}
}

// TestPeerDownDropsTheNotificationWithTheSession proves the cache does not
// outlive the session it describes: a second peer down with no NOTIFICATION of
// its own must not reuse the first one's PDU, which would report a reason the
// session never had.
func TestPeerDownDropsTheNotificationWithTheSession(t *testing.T) {
	bp, server := peerDownHarness(t)

	first := readPeerDown(t, bp, server,
		notificationEvent(rpc.DirectionSent, []byte{6, 2}),
		peerDownEvent("session closed"))
	if first.Reason != PeerDownLocalNotify {
		t.Fatalf("first reason = %d, want %d", first.Reason, PeerDownLocalNotify)
	}

	second := readPeerDown(t, bp, server, peerDownEvent("connection lost"))
	if second.Reason != PeerDownLocalNoNotify {
		t.Fatalf("second reason = %d, want %d: the cached NOTIFICATION must not survive", second.Reason, PeerDownLocalNoNotify)
	}
	if !bytes.Equal(second.Data, []byte{0, 0}) {
		t.Errorf("second data = %x, want two zero bytes", second.Data)
	}
}
