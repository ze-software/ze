// Design: docs/architecture/behavior/fsm-connect.md — Connect is the blocking dial the peer run loop makes
// Related: rfc4271_active_any_other_event_peer_test.go — the same action list proven from Active
// RFC: rfc/short/rfc4271.md — Section 8.2.2, Connect state

package reactor

import (
	"context"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// heldDialer completes a real TCP dial, then holds the connection back from its
// caller until release is closed. While it holds, the session that called it
// sits in Connect with a TCP connection the far end has already accepted, which
// is the window in which RFC 4271 Section 8.2.2 lets an event reach Connect.
//
// Safe for concurrent use. Only the first dial is held and reported on dialed;
// the tests set a one-minute reconnect delay, so no second dial happens.
type heldDialer struct {
	dialed  chan struct{}
	release chan struct{}
	once    sync.Once
}

func newHeldDialer() *heldDialer {
	return &heldDialer{dialed: make(chan struct{}), release: make(chan struct{})}
}

// DialContext dials, reports the dial on dialed, and returns the connection
// only once release is closed. A canceled context closes the connection it
// holds, so a stopped Peer leaks nothing.
func (d *heldDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	d.once.Do(func() { close(d.dialed) })

	select {
	case <-d.release:
		return conn, nil
	case <-ctx.Done():
		closeConnQuietly(conn)
		return nil, ctx.Err()
	}
}

// startConnectingPeer runs a dial-only Peer whose dial completes at the TCP
// level and is then held, so the Peer's session sits in Connect inside
// Session.Connect. It returns the Peer, that session, the dialer (whose release
// channel the test closes), and the far end's side of the dialed connection.
// The ConnectRetryCounter is set to 3 once the session is in Connect.
func startConnectingPeer(t *testing.T) (*Peer, *Session, *heldDialer, net.Conn) {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, ln.Close()) })
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address must be TCP")

	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Port = uint16(addr.Port) //nolint:gosec // a listener port fits 16 bits
	settings.Connection = ConnectionActive
	peer := NewPeer(settings)
	peer.setReconnectDelay(time.Minute, time.Minute)
	dialer := newHeldDialer()
	peer.SetDialer(dialer)

	t.Cleanup(startAndStop(t, peer))

	select {
	case <-dialer.dialed:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the started Peer never dialed")
	}
	tcpListener, isTCP := ln.(*net.TCPListener)
	require.True(t, isTCP, "Listen(\"tcp\") returns a TCPListener")
	require.NoError(t, tcpListener.SetDeadline(time.Now().Add(5*time.Second)))
	far, err := ln.Accept()
	require.NoError(t, err, "the far end never saw the dialed connection")
	t.Cleanup(func() { assert.NoError(t, far.Close()) })

	var session *Session
	require.Eventually(t, func() bool {
		session = peer.currentSession()
		return session != nil && session.State() == fsm.StateConnect
	}, 5*time.Second, time.Millisecond, "the dialing Peer's session is in Connect")

	// The operator start (Event 1) zeroed the counter before the dial, so the
	// history is seeded only now, where an increment and a reset differ.
	for range 3 {
		peer.connectRetryCounter.Increment()
	}
	require.Equal(t, uint32(3), peer.ConnectRetryCounter())
	return peer, session, dialer, far
}

// TestRFC4271ConnectAutomaticStopDuringTheDialReleasesThePeer holds a running
// dial-only Peer inside its Connect dial, stops the session the way the local
// system does (teardownAutomatic, the Event 8 path a BFD Down or a forward-pool
// overflow takes), then lets the dial return.
//
// VALIDATES: the session's FSM goes to Idle and the ConnectRetryCounter from 3
// to 4; once the dial returns, the far end reads end of file with no octet (the
// TCP connection is dropped, no OPEN is sent); the session never holds the
// connection; the Peer drops the session; the HoldTimer, KeepaliveTimer and
// ConnectRetryTimer are stopped; and the refused connection is not counted a
// second time.
// PREVENTS: an AutomaticStop during the dial that the dial outlives, so the
// stopped session goes on to OPEN the peer, or a stop that does not count the
// failed attempt.
//
// RFC requirement: RFC4271-8.2.2-15 positive -- AutomaticStop (Event 8, Session.teardownAutomatic) on a running dial-only Peer held in its Connect dial moves its FSM to Idle and the ConnectRetryCounter from 3 to 4; when the dial returns, the far end reads EOF with no octet, the session holds no connection, the Peer drops the session, the HoldTimer, KeepaliveTimer and ConnectRetryTimer are stopped, and the counter stays at 4.
func TestRFC4271ConnectAutomaticStopDuringTheDialReleasesThePeer(t *testing.T) {
	peer, session, dialer, far := startConnectingPeer(t)

	require.NoError(t, session.teardownAutomatic(message.NotifyCeaseOutOfResources, ""))

	assert.Equal(t, fsm.StateIdle, session.State(), "the state changes to Idle")
	assert.Equal(t, uint32(4), peer.ConnectRetryCounter(), "the ConnectRetryCounter is incremented by 1")

	close(dialer.release)

	require.NoError(t, far.SetReadDeadline(time.Now().Add(5*time.Second)))
	buf := make([]byte, 4096)
	read, readErr := far.Read(buf)
	assert.Zero(t, read, "the stopped session put %d octets on the wire", read)
	assert.ErrorIs(t, readErr, io.EOF, "the TCP connection is dropped")

	assert.Eventually(t, func() bool { return peer.currentSession() != session }, 5*time.Second, time.Millisecond,
		"the Peer releases the session")
	assert.Nil(t, session.Conn(), "the stopped session holds no TCP connection")
	timers := session.timers
	assert.False(t, timers.IsConnectRetryTimerRunning(), "ConnectRetryTimer stopped")
	assert.False(t, timers.IsHoldTimerRunning(), "HoldTimer released")
	assert.False(t, timers.IsKeepaliveTimerRunning(), "KeepaliveTimer released")
	assert.Never(t, func() bool { return peer.ConnectRetryCounter() != 4 }, 300*time.Millisecond, 10*time.Millisecond,
		"the refused connection is not counted again")
}

// TestRFC4271ConnectDuplicateStartDuringTheDialKeepsThePeer holds a running
// dial-only Peer inside its Connect dial, repeats the operator start on its
// session (Event 1, which Section 8.2.2 says is ignored in Connect), then lets
// the dial return.
//
// VALIDATES: the FSM stays in Connect and the ConnectRetryCounter at 3; once the
// dial returns, the far end reads an OPEN on the same connection; the Peer still
// holds the same session, which is not sealed.
// PREVENTS: the Connect "any other event" action list running for an event it
// does not name.
//
// RFC requirement: RFC4271-8.2.2-15 negative -- ManualStart (Event 1, Session.Start) on a running dial-only Peer held in its Connect dial leaves the FSM in Connect and the ConnectRetryCounter at 3; when the dial returns, the far end reads an OPEN and the Peer still holds the same, unsealed session.
func TestRFC4271ConnectDuplicateStartDuringTheDialKeepsThePeer(t *testing.T) {
	peer, session, dialer, far := startConnectingPeer(t)

	require.NoError(t, session.Start())

	assert.Equal(t, fsm.StateConnect, session.State(), "Event 1 is ignored in Connect")
	assert.Never(t, func() bool { return peer.ConnectRetryCounter() != 3 }, 300*time.Millisecond, 10*time.Millisecond,
		"Event 1 does not move the ConnectRetryCounter")

	close(dialer.release)

	require.NoError(t, far.SetReadDeadline(time.Now().Add(5*time.Second)))
	buf := make([]byte, 4096)
	read, err := far.Read(buf)
	require.NoError(t, err, "the dialed connection is kept")
	require.GreaterOrEqual(t, read, message.HeaderLen)
	assert.Equal(t, byte(1), buf[18], "the session sends its OPEN on the dialed connection")
	assert.Same(t, session, peer.currentSession(), "the Peer keeps the session")
	assert.False(t, session.tearingDown.Load(), "the session is not sealed")
	assert.Equal(t, uint32(3), peer.ConnectRetryCounter(), "the dial is not counted")
}
