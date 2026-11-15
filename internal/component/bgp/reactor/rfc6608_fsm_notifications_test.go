// Design: docs/architecture/behavior/fsm.md — state determines unexpected-message diagnostics
// Related: rfc4271_opensent_error_peer_test.go — the running TCP peer harness
// Related: rfc4271_fsm_teardown_peer_test.go — the OpenConfirm handshake
// RFC: rfc/short/rfc6608.md — state-specific FSM Error subcodes and message-type data

package reactor

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestRFC6608OpenSentUnexpectedMessages inspects actual TCP output for each
// message the peer receives before its OPEN. A malformed UPDATE is deliberately
// included: its unexpected state must be diagnosed before UPDATE parsing.
// RFC requirement: RFC6608-4-1 positive -- unexpected KEEPALIVE, UPDATE, ROUTE-REFRESH and non-version NOTIFICATION messages in OpenSent cause exactly one FSM Error 5/1 with their one-octet message type, followed by TCP EOF.
func TestRFC6608OpenSentUnexpectedMessages(t *testing.T) {
	cease := &message.Notification{
		ErrorCode: message.NotifyCease, ErrorSubcode: message.NotifyCeaseAdminShutdown,
	}
	for _, tc := range []struct {
		name string
		wire []byte
	}{
		{"KEEPALIVE", message.PackTo(message.NewKeepalive(), nil)},
		{"UPDATE", emptyUpdate()},
		{"malformed UPDATE", malformedUpdate()},
		{"ROUTE-REFRESH", message.PackTo(&message.RouteRefresh{AFI: 1, SAFI: 1}, nil)},
		{"NOTIFICATION", message.PackTo(cease, nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := startOpenSentNeighbor(t)
			rfc6608Send(t, n, tc.wire)
			rfc6608RequireNotification(t, n, message.NotifyFSMUnexpectedOpenSent, tc.wire[18])
		})
	}
}

// TestRFC6608OpenConfirmUnexpectedMessages sends each unexpected message after
// the OPEN exchange but before the peer's KEEPALIVE, retaining the state that
// must select subcode 2 even when the message is another OPEN.
// RFC requirement: RFC6608-4-2 positive -- OPEN, UPDATE and ROUTE-REFRESH received in OpenConfirm cause exactly one FSM Error 5/2 with the received one-octet message type, followed by TCP EOF.
func TestRFC6608OpenConfirmUnexpectedMessages(t *testing.T) {
	for _, tc := range []struct {
		name string
		wire []byte
	}{
		{"OPEN", wellFormedOpen()},
		{"UPDATE", emptyUpdate()},
		{"malformed UPDATE", malformedUpdate()},
		{"ROUTE-REFRESH", message.PackTo(&message.RouteRefresh{AFI: 1, SAFI: 1}, nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := startOpenConfirmNeighbor(t)
			rfc6608Send(t, n, tc.wire)
			rfc6608RequireNotification(t, n, message.NotifyFSMUnexpectedOpenConfirm, tc.wire[18])
		})
	}
}

// TestRFC6608EstablishedUnexpectedOpen drives the full handshake before sending
// a second OPEN, so neither an OpenConfirm diagnosis nor a Cease can satisfy it.
// RFC requirement: RFC6608-4-3 positive -- an OPEN received in Established causes exactly one FSM Error 5/3 whose Data is the single OPEN type octet, followed by TCP EOF.
func TestRFC6608EstablishedUnexpectedOpen(t *testing.T) {
	n := rfc6608EstablishedNeighbor(t)
	rfc6608Send(t, n, wellFormedOpen())
	rfc6608RequireNotification(t, n, message.NotifyFSMUnexpectedEstablished, 1)
}

// TestRFC6608OpenSentExpectedOpenKeepsTheConnection proves that the same OPEN
// rejected later in the handshake is accepted where it belongs.
// RFC requirement: RFC6608-4-1 negative -- a valid OPEN in OpenSent reaches OpenConfirm without a NOTIFICATION, socket close or retry-counter increment.
func TestRFC6608OpenSentExpectedOpenKeepsTheConnection(t *testing.T) {
	n := startOpenSentNeighbor(t)
	rfc6608Send(t, n, wellFormedOpen())
	require.Eventually(t, func() bool { return n.session.State() == fsm.StateOpenConfirm },
		5*time.Second, time.Millisecond, "the expected OPEN advances the handshake")
	rfc6608RequireQuiet(t, n, fsm.StateOpenConfirm)
}

// TestRFC6608OpenConfirmExpectedKeepaliveKeepsTheConnection supplies the
// KEEPALIVE that is an error in OpenSent and a valid confirmation here.
// RFC requirement: RFC6608-4-2 negative -- a KEEPALIVE in OpenConfirm reaches Established without a NOTIFICATION, socket close or retry-counter increment.
func TestRFC6608OpenConfirmExpectedKeepaliveKeepsTheConnection(t *testing.T) {
	n := rfc6608EstablishedNeighbor(t)
	rfc6608RequireQuiet(t, n, fsm.StateEstablished)
}

// TestRFC6608EstablishedExpectedMessagesKeepTheConnection exercises the same
// UPDATE, ROUTE-REFRESH and KEEPALIVE types rejected before establishment.
// RFC requirement: RFC6608-4-3 negative -- Established KEEPALIVE, UPDATE and ROUTE-REFRESH survive until a terminal same-connection OPEN; only that OPEN receives exact FSM Error 5/3 with Data 1 and EOF, with one failed attempt counted.
func TestRFC6608EstablishedExpectedMessagesKeepTheConnection(t *testing.T) {
	n := rfc6608EstablishedNeighbor(t)
	for _, wire := range [][]byte{
		message.PackTo(message.NewKeepalive(), nil),
		emptyUpdate(),
		message.PackTo(&message.RouteRefresh{AFI: 1, SAFI: 1}, nil),
	} {
		rfc6608Send(t, n, wire)
		rfc6608RequireQuiet(t, n, fsm.StateEstablished)
	}
	// TCP FIFO makes this terminal response a processing barrier for every
	// preceding valid message. A prior refusal has a different message Data
	// octet, an extra notification or a socket that cannot accept this OPEN.
	rfc6608Send(t, n, wellFormedOpen())
	rfc6608RequireNotification(t, n, message.NotifyFSMUnexpectedEstablished, 1)
	require.Equal(t, uint32(1), n.peer.ConnectRetryCounter(),
		"only the terminal unexpected OPEN counts as a failed attempt")
}

// rfc6608Send bounds the peer's write so a broken receiver cannot hang a test.
func rfc6608Send(t *testing.T, n *openSentNeighbor, wire []byte) {
	t.Helper()
	require.NoError(t, n.conn.SetWriteDeadline(time.Now().Add(5*time.Second)))
	written, err := n.conn.Write(wire)
	require.NoError(t, err)
	require.Equal(t, len(wire), written)
}

// rfc6608EstablishedNeighbor completes the real peer handshake. It never
// assigns an FSM state directly, which would skip the producers under test.
func rfc6608EstablishedNeighbor(t *testing.T) *openSentNeighbor {
	t.Helper()
	n := startOpenConfirmNeighbor(t)
	rfc6608Send(t, n, message.PackTo(message.NewKeepalive(), nil))
	require.Eventually(t, func() bool { return n.session.State() == fsm.StateEstablished },
		5*time.Second, time.Millisecond, "the expected KEEPALIVE establishes the session")
	// Initial EOR output belongs to the handshake, before the message under test.
	rfc6608RequireQuiet(t, n, fsm.StateEstablished)
	return n
}

// rfc6608RequireNotification requires EOF rather than accepting a read timeout
// as evidence of closure. The entire payload comparison refuses absent, extra,
// wrong-subcode or wrong-message-type Data bytes.
func rfc6608RequireNotification(t *testing.T, n *openSentNeighbor, subcode, received byte) {
	t.Helper()
	require.NoError(t, n.conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	wire, err := io.ReadAll(n.conn)
	require.NoError(t, err, "the peer must close with EOF after its NOTIFICATION")
	packets := splitWireMessages(t, wire)
	require.Len(t, packets, 1, "exactly one NOTIFICATION precedes closure")
	require.Equal(t, byte(3), packets[0][18], "the response must be a NOTIFICATION")
	assert.Equal(t, []byte{byte(message.NotifyFSMError), subcode, received},
		packets[0][message.HeaderLen:], "state-specific subcode and exactly one Data octet")
	require.Eventually(t, func() bool { return n.peer.currentSession() != n.session },
		5*time.Second, time.Millisecond, "the peer releases its failed session")
}

// rfc6608RequireQuiet checks both the peer-visible output and the live state;
// an implementation that accepts a message and then silently closes still fails.
func rfc6608RequireQuiet(t *testing.T, n *openSentNeighbor, state fsm.State) {
	t.Helper()
	wire, closed := n.readUntil(t, 200*time.Millisecond)
	for _, packet := range splitWireMessages(t, wire) {
		assert.NotEqual(t, byte(3), packet[18], "expected messages generate no NOTIFICATION")
	}
	assert.False(t, closed, "the connection remains open")
	assert.Equal(t, state, n.session.State())
	assert.Same(t, n.session, n.peer.currentSession(), "the peer retains this live session")
	assert.Zero(t, n.peer.ConnectRetryCounter(), "no failed attempt is counted")
}
