// Design: docs/architecture/behavior/fsm.md — the start event of each reconnect cycle
// Related: docs/architecture/behavior/peer-lifecycle.md — the peer run loop that restarts a session
// RFC: rfc/short/rfc4271.md — Section 8.2.2, Idle state

package reactor

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// automaticStartNeighbor is the far end of a dial-only Peer: the test accepts
// each connection the Peer makes and plays the BGP speaker by hand.
type automaticStartNeighbor struct {
	peer     *Peer
	listener *net.TCPListener
}

// startAutomaticStartNeighbor runs a dial-only Peer against a listener. The
// reconnect delay is long enough (half a second, jittered) for the test to read
// the ConnectRetryCounter between the end of one cycle and the start of the
// next, and short enough that the next cycle starts inside the test.
func startAutomaticStartNeighbor(t *testing.T) *automaticStartNeighbor {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // test cleanup
	tcpListener, isTCP := ln.(*net.TCPListener)
	require.True(t, isTCP, "Listen(\"tcp\") returns a TCPListener")
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address must be TCP")

	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Port = uint16(addr.Port) //nolint:gosec // a listener port fits 16 bits
	settings.Connection = ConnectionActive
	peer := NewPeer(settings)
	peer.setReconnectDelay(500*time.Millisecond, 500*time.Millisecond)
	t.Cleanup(startAndStop(t, peer))
	return &automaticStartNeighbor{peer: peer, listener: tcpListener}
}

// accept takes the Peer's next connection and reads its OPEN, so the Peer's
// session is in OpenSent when it returns. The connection is closed at cleanup.
func (n *automaticStartNeighbor) accept(t *testing.T) net.Conn {
	t.Helper()
	require.NoError(t, n.listener.SetDeadline(time.Now().Add(10*time.Second)))
	conn, err := n.listener.Accept()
	require.NoError(t, err, "the peer never dialed")
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

	buf := make([]byte, 4096)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	got, err := conn.Read(buf)
	require.NoError(t, err)
	require.GreaterOrEqual(t, got, message.HeaderLen)
	require.Equal(t, byte(1), buf[18], "the peer's first message is its OPEN")
	return conn
}

// establish answers the Peer's OPEN with an OPEN carrying holdTime and a
// KEEPALIVE, and waits until the Peer's session reports Established.
func (n *automaticStartNeighbor) establish(t *testing.T, conn net.Conn, holdTime uint16) {
	t.Helper()
	open := &message.Open{
		Version: 4, MyAS: 65001, HoldTime: holdTime, BGPIdentifier: 0x02020202,
		OptionalParams: []byte{2, 6, 65, 4, 0, 0, 0xFD, 0xE9}, // ASN4 65001
	}
	wire := message.PackTo(open, nil)
	wire = append(wire, automaticStartMessage(4)...) // KEEPALIVE
	_, err := conn.Write(wire)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		session := n.peer.currentSession()
		return session != nil && session.State() == fsm.StateEstablished
	}, 5*time.Second, time.Millisecond, "the peer's session never reached Established")
}

// automaticStartMessage builds a BGP message of the given type whose body is
// body: the all-ones marker, the length, the type.
func automaticStartMessage(msgType byte, body ...byte) []byte {
	msg := make([]byte, message.HeaderLen, message.HeaderLen+len(body))
	for i := range message.MarkerLen {
		msg[i] = 0xFF
	}
	binary.BigEndian.PutUint16(msg[16:18], uint16(message.HeaderLen+len(body))) //nolint:gosec // a test body is a few octets
	msg[18] = msgType
	return append(msg, body...)
}

// TestRFC4271AutomaticStartEvent3ZeroesCounter is the wiring test for RFC 4271
// Event 3 (AutomaticStart): a session that reached Established ends, and the
// reconnect loop starts the next cycle with Event 3, whose Idle action list
// "sets ConnectRetryCounter to zero".
//
// VALIDATES: for an Established session ended by a received Cease NOTIFICATION
// (Event 25) and for one ended by HoldTimer expiry (Event 10), the Peer's
// ConnectRetryCounter first reads 1 (the Established state's increment), and
// reads 0 once the Peer has dialed its next connection.
// PREVENTS: the reconnect loop firing Event 6 after a session that ran, which
// keeps the counter, so a peer that held a session for a day and then lost it
// would restart carrying the failures from before that session. Both exits are
// error returns from Session.Run, so a predicate tied to runOnce returning nil
// (reached only when the peer stops) would leave the counter at 1.
//
// RFC requirement: RFC4271-8.2.2-20 positive -- a running Peer whose Established session ends by a received Cease NOTIFICATION, or by HoldTimer expiry with a negotiated Hold Time of 3 seconds, holds ConnectRetryCounter 1 after the session ends and 0 once it has dialed its next connection.
func TestRFC4271AutomaticStartEvent3ZeroesCounter(t *testing.T) {
	tests := []struct {
		name     string
		holdTime uint16
		end      func(t *testing.T, conn net.Conn)
	}{
		{
			name:     "Cease NOTIFICATION",
			holdTime: 90,
			end: func(t *testing.T, conn net.Conn) {
				t.Helper()
				_, err := conn.Write(automaticStartMessage(3, byte(message.NotifyCease), 2))
				require.NoError(t, err)
			},
		},
		{
			name:     "HoldTimer expiry",
			holdTime: 3,
			end:      func(*testing.T, net.Conn) {}, // send nothing: Ze's HoldTimer expires
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := startAutomaticStartNeighbor(t)
			first := n.accept(t)
			n.establish(t, first, tt.holdTime)
			require.Equal(t, uint32(0), n.peer.ConnectRetryCounter(), "reaching Established moves no counter")

			tt.end(t, first)
			require.Eventually(t, func() bool {
				return n.peer.ConnectRetryCounter() == 1
			}, 10*time.Second, time.Millisecond, "the end of an Established session increments the counter")

			n.accept(t)
			require.Equal(t, uint32(0), n.peer.ConnectRetryCounter(),
				"the cycle after an Established session starts with Event 3, which zeroes the counter")
		})
	}
}

// TestRFC4271FailedCycleRestartKeepsCounter is the negative half: a cycle that
// never reached Established is followed by Event 6, which keeps the counter.
//
// VALIDATES: a Peer whose first session fails in OpenSent on a header with an
// all-zero marker (Event 21, which increments the counter) still reads
// ConnectRetryCounter 1 once it has dialed its next connection.
// PREVENTS: Event 3 on every automatic restart, which would zero the counter
// before each attempt and leave it unable to count oscillation.
//
// RFC requirement: RFC4271-8.2.2-20 negative -- a running Peer whose session fails in OpenSent on an all-zero-marker header, never having reached Established, holds ConnectRetryCounter 1 after the failure and still 1 once it has dialed its next connection.
func TestRFC4271FailedCycleRestartKeepsCounter(t *testing.T) {
	n := startAutomaticStartNeighbor(t)
	first := n.accept(t)

	badHeader := automaticStartMessage(4)
	for i := range message.MarkerLen {
		badHeader[i] = 0
	}
	_, err := first.Write(badHeader)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return n.peer.ConnectRetryCounter() == 1
	}, 10*time.Second, time.Millisecond, "a header error in OpenSent increments the counter")

	n.accept(t)
	require.Equal(t, uint32(1), n.peer.ConnectRetryCounter(),
		"the cycle after a failed attempt starts with Event 6, which keeps the counter")
}
