// Design: docs/architecture/behavior/fsm-open-sent.md — the OpenSent "any other event" row
// Related: docs/architecture/behavior/fsm-open-confirm.md — the OpenConfirm "any other event" row
// RFC: rfc/short/rfc4271.md — Section 8.2.2, OpenSent and OpenConfirm states

package reactor

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// fsmErrorNotification is the (code, subcode) a Finite State Machine Error
// writes: code 5, subcode 0 (Unspecified Error).
var fsmErrorNotification = [2]byte{uint8(message.NotifyFSMError), 0}

// emptyUpdate is a well-formed UPDATE with no withdrawn routes, no path
// attributes and no NLRI: 19 octets of header and two zero length fields.
func emptyUpdate() []byte {
	msg := make([]byte, message.HeaderLen+4)
	for i := range message.MarkerLen {
		msg[i] = 0xFF
	}
	binary.BigEndian.PutUint16(msg[16:18], uint16(len(msg))) //nolint:gosec // 23 fits 16 bits
	msg[18] = 2
	return msg
}

// malformedUpdate is an UPDATE whose Withdrawn Routes Length (200) runs past
// the end of the message, which UPDATE error handling refuses (Event 28).
func malformedUpdate() []byte {
	msg := emptyUpdate()
	binary.BigEndian.PutUint16(msg[message.HeaderLen:], 200)
	return msg
}

// TestRFC4271OpenSentUnexpectedMessageIsAnFSMError drives a running Peer into
// OpenSent over TCP and, instead of an OPEN, sends it one of the messages RFC 4271
// Section 8.2.2 files under OpenSent's "any other event": a NOTIFICATION that is
// not a version error (Event 25), a KEEPALIVE (Event 26), an UPDATE (Event 27), or
// a malformed UPDATE (Event 28).
//
// VALIDATES: for each, the Peer writes exactly one NOTIFICATION, 5/0 (Finite State
// Machine Error), then drops the TCP connection (EOF at the far end); the session's
// HoldTimer, KeepaliveTimer and ConnectRetryTimer stop; the Peer releases the
// session; the ConnectRetryCounter moves from 0 to 1.
// PREVENTS: an OpenSent that tears the session down on an unexpected message
// without telling the peer why (until 2026-10-01 the FSM answered ErrFSMError and
// no NOTIFICATION producer existed), and an UPDATE received before Established that
// is parsed and handed to the plugins.
//
// RFC requirement: RFC4271-8.2.2-24 positive -- a running Peer in OpenSent that receives a Cease NOTIFICATION (Event 25), a KEEPALIVE (Event 26), an empty UPDATE (Event 27) or an UPDATE whose Withdrawn Routes Length overruns the message (Event 28) writes exactly one NOTIFICATION 5/0, closes the TCP connection, stops its three session timers, drops the session, and moves the ConnectRetryCounter from 0 to 1.
// RFC requirement: RFC4271-6.7-1 negative -- an FSM error on a running Peer in OpenSent (NOTIFICATION, KEEPALIVE or UPDATE received before an OPEN) is answered by exactly one NOTIFICATION, 5/0, never Cease.
func TestRFC4271OpenSentUnexpectedMessageIsAnFSMError(t *testing.T) {
	cease := &message.Notification{ErrorCode: message.NotifyCease, ErrorSubcode: message.NotifyCeaseAdminShutdown}

	for _, tc := range []struct {
		name string
		send []byte
	}{
		{name: "Event 25 NOTIFICATION", send: message.PackTo(cease, nil)},
		{name: "Event 26 KEEPALIVE", send: message.PackTo(message.NewKeepalive(), nil)},
		{name: "Event 27 UPDATE", send: emptyUpdate()},
		{name: "Event 28 malformed UPDATE", send: malformedUpdate()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := startOpenSentNeighbor(t)
			require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())

			_, err := n.conn.Write(tc.send)
			require.NoError(t, err)

			requireReleased(t, n, [][2]byte{fsmErrorNotification}, 1)
		})
	}
}

// TestRFC4271OpenConfirmUpdateIsAnFSMError drives a running Peer into OpenConfirm
// (OPEN exchanged, the Peer's KEEPALIVE read) and sends it an UPDATE before any
// KEEPALIVE: Event 27, or Event 28 when the UPDATE is malformed, both under
// OpenConfirm's "any other event".
//
// VALIDATES: the Peer writes exactly one NOTIFICATION, 5/0, then drops the TCP
// connection; the three session timers stop; the session is released; the
// ConnectRetryCounter moves from 0 to 1.
// PREVENTS: an OpenConfirm that drops an early UPDATE's session in silence, or that
// processes the UPDATE as if the session were Established.
//
// RFC requirement: RFC4271-8.2.2-25 positive -- a running Peer in OpenConfirm that receives an empty UPDATE (Event 27) or an UPDATE whose Withdrawn Routes Length overruns the message (Event 28) writes exactly one NOTIFICATION 5/0, closes the TCP connection, stops its three session timers, drops the session, and moves the ConnectRetryCounter from 0 to 1.
// RFC requirement: RFC4271-6.7-1 negative -- an FSM error on a running Peer in OpenConfirm (UPDATE received before the KEEPALIVE) is answered by exactly one NOTIFICATION, 5/0, never Cease.
func TestRFC4271OpenConfirmUpdateIsAnFSMError(t *testing.T) {
	for _, tc := range []struct {
		name string
		send []byte
	}{
		{name: "Event 27 UPDATE", send: emptyUpdate()},
		{name: "Event 28 malformed UPDATE", send: malformedUpdate()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := startOpenConfirmNeighbor(t)
			require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())

			_, err := n.conn.Write(tc.send)
			require.NoError(t, err)

			requireReleased(t, n, [][2]byte{fsmErrorNotification}, 1)
		})
	}
}

// TestRFC4271ExpectedMessagesRaiseNoFSMError is the non-triggering input for both
// tests above: the same running Peer receives each message in the state that
// expects it. An OPEN in OpenSent moves it to OpenConfirm, a KEEPALIVE in
// OpenConfirm moves it to Established, and an UPDATE in Established is processed.
//
// VALIDATES: at each step the Peer writes no NOTIFICATION within 300 ms, keeps the
// TCP connection and the session, and leaves the ConnectRetryCounter at 0.
// PREVENTS: an FSM-Error path that fires on every message, or on every UPDATE or
// KEEPALIVE whatever the state, so the two positive tests would pass without the
// state deciding.
//
// RFC requirement: RFC4271-8.2.2-24 negative -- a running Peer in OpenSent that receives a well-formed OPEN (Event 19, not "any other event") writes no NOTIFICATION, keeps the connection and session, reaches OpenConfirm, and leaves the ConnectRetryCounter at 0; the same KEEPALIVE and UPDATE that raise an FSM Error in OpenSent raise none once the session has reached Established.
// RFC requirement: RFC4271-8.2.2-25 negative -- a running Peer in OpenConfirm that receives a KEEPALIVE (Event 26, not "any other event") writes no NOTIFICATION, keeps the connection and session, reaches Established, and leaves the ConnectRetryCounter at 0; the UPDATE that raises an FSM Error in OpenConfirm raises none in Established.
func TestRFC4271ExpectedMessagesRaiseNoFSMError(t *testing.T) {
	n := startOpenSentNeighbor(t)

	requireQuiet := func(step string) {
		t.Helper()
		wire, closed := n.readUntil(t, 300*time.Millisecond)
		for _, msg := range splitWireMessages(t, wire) {
			assert.NotEqual(t, byte(3), msg[18], "%s: no NOTIFICATION", step)
		}
		assert.False(t, closed, "%s: the connection stays up", step)
		assert.Same(t, n.session, n.peer.currentSession(), "%s: the session stays the Peer's", step)
		assert.Equal(t, uint32(0), n.peer.ConnectRetryCounter(), "%s: the counter stays 0", step)
	}

	_, err := n.conn.Write(wellFormedOpen())
	require.NoError(t, err)
	require.Eventually(t, func() bool { return n.session.State() == fsm.StateOpenConfirm }, 5*time.Second, time.Millisecond,
		"an OPEN in OpenSent reaches OpenConfirm")
	requireQuiet("OPEN in OpenSent")

	_, err = n.conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return n.session.State() == fsm.StateEstablished }, 5*time.Second, time.Millisecond,
		"a KEEPALIVE in OpenConfirm reaches Established")
	requireQuiet("KEEPALIVE in OpenConfirm")

	_, err = n.conn.Write(emptyUpdate())
	require.NoError(t, err)
	_, err = n.conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	requireQuiet("UPDATE and KEEPALIVE in Established")
	assert.Equal(t, fsm.StateEstablished, n.session.State(), "the session stays Established")
}
