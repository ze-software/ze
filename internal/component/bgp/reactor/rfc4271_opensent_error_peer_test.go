// Design: docs/architecture/behavior/fsm.md — the peer run loop owns the connection a session error releases
// RFC: rfc/short/rfc4271.md — Section 8.2.2, OpenSent state

package reactor

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// openSentNeighbor is the far end of one connection a running dial-only Peer made,
// held right after the Peer's OPEN arrived, so the Peer's session is in OpenSent.
type openSentNeighbor struct {
	peer    *Peer
	session *Session
	conn    net.Conn
}

// startOpenSentNeighbor runs a dial-only Peer (AS 65000) against a listener, reads
// its OPEN, and returns once the Peer's session reports OpenSent. The reconnect
// delay is a minute, so the Peer does not redial inside the test.
func startOpenSentNeighbor(t *testing.T) *openSentNeighbor {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // test cleanup
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address must be TCP")

	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Port = uint16(addr.Port) //nolint:gosec // a listener port fits 16 bits
	settings.Connection = ConnectionActive
	peer := NewPeer(settings)
	peer.setReconnectDelay(time.Minute, time.Minute)
	t.Cleanup(startAndStop(t, peer))

	tcpListener, isTCP := ln.(*net.TCPListener)
	require.True(t, isTCP, "Listen(\"tcp\") returns a TCPListener")
	require.NoError(t, tcpListener.SetDeadline(time.Now().Add(5*time.Second)))
	conn, err := ln.Accept()
	require.NoError(t, err, "the peer never dialed")
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

	buf := make([]byte, 4096)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	n, err := conn.Read(buf)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, message.HeaderLen)
	require.Equal(t, byte(1), buf[18], "the peer's first message is its OPEN")

	var session *Session
	require.Eventually(t, func() bool {
		session = peer.currentSession()
		return session != nil && session.State() == fsm.StateOpenSent
	}, 5*time.Second, time.Millisecond, "the peer's session never reported OpenSent")
	// connectionEstablished arms the HoldTimer after the OPEN write returns, so the
	// far end can read the OPEN a moment before the timer is armed.
	require.Eventually(t, session.timers.IsHoldTimerRunning, 5*time.Second, time.Millisecond,
		"OpenSent arms the HoldTimer")
	return &openSentNeighbor{peer: peer, session: session, conn: conn}
}

// readUntil collects what the Peer writes until EOF or until the deadline. It
// returns the bytes and whether the Peer closed the connection (EOF) rather than
// leaving it open past the deadline.
func (n *openSentNeighbor) readUntil(t *testing.T, wait time.Duration) (wire []byte, closed bool) {
	t.Helper()
	require.NoError(t, n.conn.SetReadDeadline(time.Now().Add(wait)))
	chunk := make([]byte, 4096)
	for {
		got, err := n.conn.Read(chunk)
		wire = append(wire, chunk[:got]...)
		if err == nil {
			continue
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return wire, false
		}
		return wire, true
	}
}

// notifications returns the (code, subcode) of every NOTIFICATION in wire.
func notifications(t *testing.T, wire []byte) [][2]byte {
	t.Helper()
	var out [][2]byte
	for _, msg := range splitWireMessages(t, wire) {
		if msg[18] == 3 && len(msg) >= message.HeaderLen+2 {
			out = append(out, [2]byte{msg[19], msg[20]})
		}
	}
	return out
}

// TestRFC4271OpenSentErrorReleasesTheConnection drives a running Peer into OpenSent
// over TCP, then answers its OPEN with a message that fails header checking (Event
// 21: an all-zero marker) or OPEN checking (Event 22: BGP version 3), and reads what
// the Peer does from the far end of the socket and from the session it ran.
//
// VALIDATES: for each error the Peer writes exactly one NOTIFICATION, with the error
// code of the check that failed (1/1 Connection Not Synchronized, 1/2 Bad Message
// Length for a Length below 19, 2/1 Unsupported Version Number), then drops the TCP
// connection; the session's HoldTimer, KeepaliveTimer and ConnectRetryTimer are all
// stopped; the Peer no longer holds the session; and the ConnectRetryCounter went
// from 0 to 1.
// PREVENTS: an FSM that counts the error while the session keeps its socket or its
// timers, or that drops the connection without telling the peer why (the header
// branch of session_read.go and session_coalesce.go did exactly that until
// 2026-09-30).
//
// RFC requirement: RFC4271-8.2.2-10 positive -- a running Peer in OpenSent that reads a header with an all-zero marker, a header with Length 18, or an OPEN with version 3 writes exactly one NOTIFICATION with code/subcode 1/1, 1/2 or 2/1, closes the TCP connection, stops its HoldTimer, KeepaliveTimer and ConnectRetryTimer, drops the session, and moves its ConnectRetryCounter from 0 to 1.
// RFC requirement: RFC4271-6.7-1 negative -- an OPEN Message Error (OPEN version 3) and a Message Header Error (all-zero marker, Length 18) on a running Peer in OpenSent are each answered by exactly one NOTIFICATION whose code/subcode is 2/1, 1/1 or 1/2, never Cease.
func TestRFC4271OpenSentErrorReleasesTheConnection(t *testing.T) {
	var badHeader [message.HeaderLen]byte // marker all zeros; a KEEPALIVE otherwise
	binary.BigEndian.PutUint16(badHeader[16:18], message.HeaderLen)
	badHeader[18] = 4

	var shortLength [message.HeaderLen]byte // marker all ones, Length 18
	for i := range message.MarkerLen {
		shortLength[i] = 0xFF
	}
	binary.BigEndian.PutUint16(shortLength[16:18], message.HeaderLen-1)
	shortLength[18] = 4

	badVersion := message.PackTo(&message.Open{Version: 3, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x02020202}, nil)

	for _, tc := range []struct {
		name  string
		send  []byte
		code  [2]byte
		event string
	}{
		{name: "header error", send: badHeader[:], code: [2]byte{1, 1}, event: "Event 21"},
		{name: "header length error", send: shortLength[:], code: [2]byte{1, 2}, event: "Event 21"},
		{name: "OPEN error", send: badVersion, code: [2]byte{2, 1}, event: "Event 22"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := startOpenSentNeighbor(t)
			require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())

			_, err := n.conn.Write(tc.send)
			require.NoError(t, err)
			wire, closed := n.readUntil(t, 5*time.Second)

			assert.Equal(t, [][2]byte{tc.code}, notifications(t, wire), "one NOTIFICATION with the failed check's error code")
			assert.True(t, closed, "the TCP connection is dropped")
			assert.Eventually(t, func() bool { return n.peer.currentSession() != n.session }, 5*time.Second, time.Millisecond,
				"the Peer releases the session")
			timers := n.session.timers
			assert.Eventually(t, func() bool { return !timers.IsConnectRetryTimerRunning() }, time.Second, time.Millisecond,
				"ConnectRetryTimer set to zero")
			assert.Eventually(t, func() bool { return !timers.IsHoldTimerRunning() }, time.Second, time.Millisecond,
				"HoldTimer released")
			assert.Eventually(t, func() bool { return !timers.IsKeepaliveTimerRunning() }, time.Second, time.Millisecond,
				"KeepaliveTimer released")
			assert.Eventually(t, func() bool { return n.peer.ConnectRetryCounter() == 1 }, 5*time.Second, time.Millisecond,
				"%s increments the ConnectRetryCounter by 1", tc.event)
		})
	}
}

// wellFormedOpen is the far end's OPEN: version 4, AS 65001 with the AS4 capability,
// Hold Time 90, so the Peer's session moves from OpenSent to OpenConfirm.
func wellFormedOpen() []byte {
	open := &message.Open{
		Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x02020202,
		OptionalParams: []byte{2, 6, 65, 4, 0, 0, 0xFD, 0xE9}, // ASN4 65001
	}
	return message.PackTo(open, nil)
}

// TestRFC4271OpenSentWellFormedOpenKeepsTheConnection is the non-triggering input
// for every OpenSent teardown test in this package (header or OPEN error, ManualStop,
// AutomaticStop, OpenCollisionDump, HoldTimer expiry): the same running Peer in
// OpenSent, its ConnectRetryCounter at 3, answered with a well-formed OPEN.
//
// VALIDATES: no NOTIFICATION, the connection stays up, the session stays the Peer's,
// and the ConnectRetryCounter stays 3 (neither zeroed nor incremented).
// PREVENTS: an OpenSent that tears the session down, sends a Cease, or moves the
// counter on an event none of those rows names, so the teardown tests would pass
// without the FSM telling the events apart.
//
// RFC requirement: RFC4271-8.2.2-10 negative -- a running Peer in OpenSent that reads a well-formed OPEN (version 4, AS4 capability) writes no NOTIFICATION within 500 ms, keeps the TCP connection open, keeps the same session, and leaves its ConnectRetryCounter at 3.
// RFC requirement: RFC4271-8.2.2-8 negative -- with no ManualStop, a Peer in OpenSent at ConnectRetryCounter 3 that reads a well-formed OPEN sends no Cease, keeps the connection and session, and leaves the counter at 3 (not zeroed).
// RFC requirement: RFC4271-8.2.2-9 negative -- a Peer in OpenSent whose peer answers before the HoldTimer expires sends no Hold Timer Expired NOTIFICATION, keeps the connection and session, and leaves the counter at 3.
// RFC requirement: RFC4271-8.2.2-16 negative -- with no AutomaticStop, a Peer in OpenSent that reads a well-formed OPEN sends no Cease, keeps the connection and session, and leaves the counter at 3.
// RFC requirement: RFC4271-6.1-1 negative -- a header with an all-ones marker and a valid Length (a well-formed OPEN) draws no Message Header Error, and no NOTIFICATION at all, within 500 ms.
// RFC requirement: RFC4271-6.1-2 negative -- a marker of all ones (a well-formed OPEN) draws no Connection Not Synchronized, and no NOTIFICATION at all, within 500 ms.
// RFC requirement: RFC4271-6.1-3 negative -- a header Length within bounds (a well-formed OPEN) draws no Bad Message Length, and no NOTIFICATION at all, within 500 ms.
// RFC requirement: RFC4271-8.2.2-17 negative -- with no OpenCollisionDump, a Peer in OpenSent that reads a well-formed OPEN sends no Cease, keeps the connection and session, and leaves the counter at 3.
func TestRFC4271OpenSentWellFormedOpenKeepsTheConnection(t *testing.T) {
	n := startOpenSentNeighbor(t)
	for range 3 {
		n.peer.connectRetryCounter.Increment()
	}

	_, err := n.conn.Write(wellFormedOpen())
	require.NoError(t, err)
	wire, closed := n.readUntil(t, 500*time.Millisecond)

	assert.Empty(t, notifications(t, wire), "no error and no stop, so no NOTIFICATION")
	assert.False(t, closed, "the connection stays up")
	assert.Same(t, n.session, n.peer.currentSession(), "the session stays the Peer's")
	assert.Equal(t, uint32(3), n.peer.ConnectRetryCounter(), "no clause zeroes or increments the counter")
}
