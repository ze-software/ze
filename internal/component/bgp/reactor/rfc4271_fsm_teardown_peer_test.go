// Design: docs/architecture/behavior/fsm.md — the peer run loop owns the connection a teardown releases
// Related: rfc4271_opensent_error_peer_test.go — the OpenSent harness and the shared negative
// RFC: rfc/short/rfc4271.md — Section 8.2.2, OpenSent and OpenConfirm states

package reactor

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// requireReleased asserts the teardown every Section 8.2.2 action list in this file
// names, as the far end and the Peer see it: exactly the NOTIFICATIONs in want on
// the wire, the TCP connection dropped (EOF at the far end), the session's HoldTimer,
// KeepaliveTimer and ConnectRetryTimer stopped, the Peer no longer holding the
// session, and the ConnectRetryCounter at counter.
func requireReleased(t *testing.T, n *openSentNeighbor, want [][2]byte, counter uint32) {
	t.Helper()
	wire, closed := n.readUntil(t, 5*time.Second)

	assert.Equal(t, want, notifications(t, wire), "the NOTIFICATIONs the Peer writes")
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
	assert.Eventually(t, func() bool { return n.peer.ConnectRetryCounter() == counter }, 5*time.Second, time.Millisecond,
		"ConnectRetryCounter ends at %d", counter)
}

// TestRFC4271OpenSentManualStopReleasesTheConnection drives a running Peer into
// OpenSent with its ConnectRetryCounter at 3, then issues the operator stop
// (shutdownNotify, the ManualStop the daemon's stop path sends to every peer).
//
// VALIDATES: one NOTIFICATION Cease (6/2), the TCP connection dropped, the three
// session timers stopped, the session released, and the counter set to 0.
// PREVENTS: a ManualStop that closes without a Cease, keeps timers running, or
// counts the stop as a failed attempt.
//
// RFC requirement: RFC4271-8.2.2-8 positive -- ManualStop (shutdownNotify) on a running Peer in OpenSent at ConnectRetryCounter 3 writes one NOTIFICATION 6/2 (Cease), closes the TCP connection, stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, drops the session, and sets the counter to 0.
// RFC requirement: RFC4271-8.2.2-18 positive -- ManualStop (shutdownNotify) on a running Peer in OpenSent writes one NOTIFICATION 6/2 (Cease), closes the TCP connection, stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, and drops the session.
func TestRFC4271OpenSentManualStopReleasesTheConnection(t *testing.T) {
	n := startOpenSentNeighbor(t)
	for range 3 {
		n.peer.connectRetryCounter.Increment()
	}

	n.peer.shutdownNotify()

	requireReleased(t, n, [][2]byte{{6, message.NotifyCeaseAdminShutdown}}, 0)
}

// TestRFC4271OpenSentAutomaticStopReleasesTheConnection drives a running Peer into
// OpenSent, then stops its session the way the local system does (teardownAutomatic,
// the Event 8 path a BFD Down or a forward-pool overflow takes).
//
// VALIDATES: one NOTIFICATION Cease with the caller's subcode (6/8), the TCP
// connection dropped, the three session timers stopped, the session released, and
// the counter moved from 0 to 1.
// PREVENTS: an AutomaticStop handled as a ManualStop (counter zeroed), or one that
// closes without a Cease.
//
// RFC requirement: RFC4271-8.2.2-16 positive -- AutomaticStop (Session.teardownAutomatic, Cease subcode 8) on a running Peer in OpenSent writes one NOTIFICATION 6/8, closes the TCP connection, stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, drops the session, and moves the ConnectRetryCounter from 0 to 1.
func TestRFC4271OpenSentAutomaticStopReleasesTheConnection(t *testing.T) {
	n := startOpenSentNeighbor(t)
	require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())

	require.NoError(t, n.session.teardownAutomatic(message.NotifyCeaseOutOfResources, ""))

	requireReleased(t, n, [][2]byte{{6, message.NotifyCeaseOutOfResources}}, 1)
}

// TestRFC4271OpenSentCollisionDumpReleasesTheConnection drives a running Peer into
// OpenSent, then closes the session the way collision resolution does when it
// chooses this connection (CloseWithNotification, Event 23).
//
// VALIDATES: one NOTIFICATION Cease / Connection Collision Resolution (6/7), the
// TCP connection dropped, the three session timers stopped, the session released,
// and the counter moved from 0 to 1.
// PREVENTS: a collision loser closed without a Cease, or counted twice (once for the
// dump and once for the TCP close it causes).
//
// RFC requirement: RFC4271-8.2.2-17 positive -- OpenCollisionDump (Session.CloseWithNotification 6/7) on a running Peer in OpenSent writes one NOTIFICATION 6/7, closes the TCP connection, stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, drops the session, and moves the ConnectRetryCounter from 0 to 1.
func TestRFC4271OpenSentCollisionDumpReleasesTheConnection(t *testing.T) {
	n := startOpenSentNeighbor(t)
	require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())

	require.NoError(t, n.session.CloseWithNotification(message.NotifyCease, message.NotifyCeaseConnectionCollision))

	requireReleased(t, n, [][2]byte{{6, message.NotifyCeaseConnectionCollision}}, 1)
}

// TestRFC4271OpenSentHoldTimerExpiryReleasesTheConnection drives a running Peer into
// OpenSent, re-arms the session's own HoldTimer (the OpenSent open-wait bound) to
// 150 ms, and lets it expire with no OPEN from the far end (Event 10).
//
// VALIDATES: one NOTIFICATION Hold Timer Expired (4/0), the TCP connection dropped,
// the three session timers stopped, the session released, and the counter moved
// from 0 to 1.
// PREVENTS: an OpenSent that waits for an OPEN forever, or that times out silently.
//
// RFC requirement: RFC4271-8.2.2-9 positive -- HoldTimer expiry in OpenSent (the session's HoldTimer re-armed to 150 ms, no OPEN sent) writes one NOTIFICATION 4/0, closes the TCP connection, stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, drops the session, and moves the ConnectRetryCounter from 0 to 1.
func TestRFC4271OpenSentHoldTimerExpiryReleasesTheConnection(t *testing.T) {
	n := startOpenSentNeighbor(t)
	require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())

	n.session.timers.SetHoldTime(150 * time.Millisecond)
	n.session.timers.StartHoldTimer()

	requireReleased(t, n, [][2]byte{{uint8(message.NotifyHoldTimerExpired), 0}}, 1)
}

// startOpenConfirmNeighbor is startOpenSentNeighbor followed by a well-formed OPEN
// from the far end: it reads the Peer's KEEPALIVE and returns once the session
// reports OpenConfirm.
func startOpenConfirmNeighbor(t *testing.T) *openSentNeighbor {
	t.Helper()
	n := startOpenSentNeighbor(t)
	_, err := n.conn.Write(wellFormedOpen())
	require.NoError(t, err)
	require.Eventually(t, func() bool { return n.session.State() == fsm.StateOpenConfirm }, 5*time.Second, time.Millisecond,
		"the session never reported OpenConfirm")
	wire, closed := n.readUntil(t, 200*time.Millisecond)
	require.False(t, closed, "OpenConfirm keeps the connection")
	msgs := splitWireMessages(t, wire)
	require.Len(t, msgs, 1, "the Peer answers the OPEN with one message")
	require.Equal(t, byte(4), msgs[0][18], "the Peer answers the OPEN with a KEEPALIVE")
	return n
}

// TestRFC4271OpenConfirmNotificationOrTCPFailureReleasesTheConnection drives a
// running Peer into OpenConfirm, then either sends it a NOTIFICATION (Cease 6/2,
// Event 25) or closes the TCP connection from the far end (Event 18).
//
// VALIDATES: the Peer writes no NOTIFICATION of its own, the TCP connection is
// dropped (for Event 25 the far end reads EOF), the three session timers stop, the
// session is released, and the counter moves from 0 to 1.
// PREVENTS: an OpenConfirm that keeps its timers or its socket after the peer said
// goodbye, or that does not count the failed attempt.
//
// RFC requirement: RFC4271-8.2.2-11 positive -- a running Peer in OpenConfirm that receives a NOTIFICATION (Event 25) or loses its TCP connection (Event 18) writes no NOTIFICATION, closes the TCP connection (EOF at the far end for Event 25), stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, drops the session, and moves the ConnectRetryCounter from 0 to 1.func TestRFC4271OpenConfirmNotificationOrTCPFailureReleasesTheConnection(t *testing.T) {
func TestRFC4271OpenConfirmNotificationOrTCPFailureReleasesTheConnection(t *testing.T) {
	t.Run("Event 25 NOTIFICATION", func(t *testing.T) {
		n := startOpenConfirmNeighbor(t)
		require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())

		cease := &message.Notification{ErrorCode: message.NotifyCease, ErrorSubcode: message.NotifyCeaseAdminShutdown}
		_, err := n.conn.Write(message.PackTo(cease, nil))
		require.NoError(t, err)

		requireReleased(t, n, nil, 1)
	})

	t.Run("Event 18 TCP failure", func(t *testing.T) {
		n := startOpenConfirmNeighbor(t)
		require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())

		require.NoError(t, n.conn.Close())

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
			"Event 18 increments the ConnectRetryCounter by 1")
	})
}

// TestRFC4271OpenConfirmKeepaliveKeepsTheConnection is the non-triggering input for
// the OpenConfirm teardown test: the far end answers with a KEEPALIVE.
//
// VALIDATES: the session reaches Established, the Peer writes no NOTIFICATION, the
// connection stays up, the session stays the Peer's, the HoldTimer runs, and the
// counter stays 0.
// PREVENTS: an OpenConfirm that tears down or counts on any message.
//
// RFC requirement: RFC4271-8.2.2-11 negative -- a running Peer in OpenConfirm that receives a KEEPALIVE instead of a NOTIFICATION or a TCP failure reaches Established, writes no NOTIFICATION within 500 ms, keeps the TCP connection, the session and its HoldTimer, and leaves the ConnectRetryCounter at 0.
func TestRFC4271OpenConfirmKeepaliveKeepsTheConnection(t *testing.T) {
	n := startOpenConfirmNeighbor(t)

	_, err := n.conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return n.session.State() == fsm.StateEstablished }, 5*time.Second, time.Millisecond,
		"a KEEPALIVE in OpenConfirm reaches Established")
	wire, closed := n.readUntil(t, 500*time.Millisecond)

	assert.Empty(t, notifications(t, wire), "no NOTIFICATION")
	assert.False(t, closed, "the connection stays up")
	assert.Same(t, n.session, n.peer.currentSession(), "the session stays the Peer's")
	assert.True(t, n.session.timers.IsHoldTimerRunning(), "the HoldTimer runs")
	assert.Equal(t, uint32(0), n.peer.ConnectRetryCounter(), "the counter stays 0")
}

// startEstablishedNeighbor is startOpenConfirmNeighbor followed by a KEEPALIVE from
// the far end: it returns once the session reports Established.
func startEstablishedNeighbor(t *testing.T) *openSentNeighbor {
	t.Helper()
	n := startOpenConfirmNeighbor(t)
	_, err := n.conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return n.session.State() == fsm.StateEstablished }, 5*time.Second, time.Millisecond,
		"a KEEPALIVE in OpenConfirm reaches Established")
	return n
}

// TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List drives a running Peer to
// Established, re-arms the session's HoldTimer to 200 ms, and sends nothing more, so
// the HoldTimer expires (Event 10).
//
// VALIDATES: the whole Event 10 list as the far end and the Peer see it: exactly one
// NOTIFICATION 4/0, then EOF; the session's HoldTimer, KeepaliveTimer and
// ConnectRetryTimer stopped; the Peer no longer holds the session; the counter moved
// from 0 to 1; the session's FSM in Idle.
// PREVENTS: an expiry that tells the peer and then keeps the socket, a timer, the
// session, or the Established state, or that does not count the failure.
//
// RFC requirement: RFC4271-8.2.2-2 positive -- HoldTimer expiry on a running Peer in Established writes exactly one NOTIFICATION 4/0 and stops the session's ConnectRetryTimer.
// RFC requirement: RFC4271-8.2.2-3 positive -- HoldTimer expiry on a running Peer in Established writes exactly one NOTIFICATION 4/0, stops the ConnectRetryTimer, and releases the session: its HoldTimer and KeepaliveTimer stop and the Peer no longer holds it.
// RFC requirement: RFC4271-8.2.2-4 positive -- HoldTimer expiry on a running Peer in Established writes exactly one NOTIFICATION 4/0, stops the three session timers, releases the session, and closes the TCP connection (EOF at the far end).
// RFC requirement: RFC4271-8.2.2-5 positive -- HoldTimer expiry on a running Peer in Established writes exactly one NOTIFICATION 4/0, stops the three session timers, releases the session, closes the TCP connection, moves the ConnectRetryCounter from 0 to 1, and leaves the session's FSM in Idle.
func TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List(t *testing.T) {
	n := startEstablishedNeighbor(t)
	require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())

	n.session.timers.SetHoldTime(200 * time.Millisecond)
	n.session.timers.StartHoldTimer()

	requireReleased(t, n, [][2]byte{{uint8(message.NotifyHoldTimerExpired), 0}}, 1)
	assert.Equal(t, fsm.StateIdle, n.session.State(), "the session's FSM changes its state to Idle")
}

// TestRFC4271EstablishedHoldTimerFedByKeepalivesNeverExpires is the non-triggering
// input for the Event 10 test: the same Established session with its HoldTimer at
// 400 ms, fed a KEEPALIVE every 100 ms for 1.2 s, three hold times.
//
// VALIDATES: no NOTIFICATION, the connection stays up, the session stays the Peer's
// and Established, its HoldTimer and KeepaliveTimer run, and the counter stays 0.
// PREVENTS: a hold timer that expires on elapsed time rather than on silence, which
// would pass the Event 10 test while tearing down healthy sessions.
//
// RFC requirement: RFC4271-8.2.2-2 negative -- an Established session whose 400 ms HoldTimer is fed a KEEPALIVE every 100 ms for 1.2 s writes no NOTIFICATION and keeps its session.
// RFC requirement: RFC4271-8.2.2-3 negative -- an Established session whose 400 ms HoldTimer is fed a KEEPALIVE every 100 ms for 1.2 s keeps its session, its HoldTimer and its KeepaliveTimer.
// RFC requirement: RFC4271-8.2.2-4 negative -- an Established session whose 400 ms HoldTimer is fed a KEEPALIVE every 100 ms for 1.2 s keeps its TCP connection open.
// RFC requirement: RFC4271-8.2.2-5 negative -- an Established session whose 400 ms HoldTimer is fed a KEEPALIVE every 100 ms for 1.2 s stays Established and leaves the ConnectRetryCounter at 0.
func TestRFC4271EstablishedHoldTimerFedByKeepalivesNeverExpires(t *testing.T) {
	n := startEstablishedNeighbor(t)

	n.session.timers.SetHoldTime(400 * time.Millisecond)
	n.session.timers.StartHoldTimer()
	keepalive := message.PackTo(message.NewKeepalive(), nil)
	for range 12 {
		_, err := n.conn.Write(keepalive)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
	}
	wire, closed := n.readUntil(t, 100*time.Millisecond)

	assert.Empty(t, notifications(t, wire), "no NOTIFICATION")
	assert.False(t, closed, "the connection stays up")
	assert.Same(t, n.session, n.peer.currentSession(), "the session stays the Peer's")
	assert.Equal(t, fsm.StateEstablished, n.session.State(), "the session stays Established")
	assert.True(t, n.session.timers.IsHoldTimerRunning(), "the HoldTimer runs")
	assert.True(t, n.session.timers.IsKeepaliveTimerRunning(), "the KeepaliveTimer runs")
	assert.Equal(t, uint32(0), n.peer.ConnectRetryCounter(), "the counter stays 0")
}

// TestRFC4271ManualStartZeroesTheCounterAndDials builds a dial-only Peer whose
// ConnectRetryCounter reads 7, then starts it (Peer.Start, the operator's start,
// which runOnce answers with Event 1 ManualStart).
//
// VALIDATES: the Peer initializes its connection resources (it builds a session and
// dials: the far end accepts a TCP connection and reads an OPEN) and the counter
// reads 0.
// PREVENTS: an operator start that keeps a stale retry history, or that zeroes the
// counter without starting the connection.
//
// RFC requirement: RFC4271-8.2.2-7 positive -- Peer.Start (Event 1) on a dial-only Peer whose ConnectRetryCounter is 7 builds a session, dials the far end, which reads an OPEN, and sets the counter to 0.
func TestRFC4271ManualStartZeroesTheCounterAndDials(t *testing.T) {
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
	for range 7 {
		peer.connectRetryCounter.Increment()
	}
	require.Equal(t, uint32(7), peer.ConnectRetryCounter())
	require.Nil(t, peer.currentSession(), "no session before the start")

	t.Cleanup(startAndStop(t, peer))

	tcpListener, isTCP := ln.(*net.TCPListener)
	require.True(t, isTCP, "Listen(\"tcp\") returns a TCPListener")
	require.NoError(t, tcpListener.SetDeadline(time.Now().Add(5*time.Second)))
	conn, err := ln.Accept()
	require.NoError(t, err, "the started Peer never dialed")
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup
	buf := make([]byte, 4096)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	n, err := conn.Read(buf)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, message.HeaderLen)
	assert.Equal(t, byte(1), buf[18], "the started Peer sends its OPEN")
	assert.NotNil(t, peer.currentSession(), "the start built a session")
	assert.Equal(t, uint32(0), peer.ConnectRetryCounter(), "ManualStart sets the counter to 0")
}
