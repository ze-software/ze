// Design: docs/architecture/behavior/fsm.md — the peer run loop owns the connection a ManualStop releases
// Related: rfc4271_fsm_teardown_peer_test.go — the OpenSent ManualStop and the OpenConfirm harness
// Related: rfc4271_established_teardown_peer_test.go — the Established harness with the peer-down observer
// RFC: rfc/short/rfc4271.md — Section 8.2.2, OpenConfirm and Established ManualStop

package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestRFC4271OpenConfirmManualStopReleasesTheConnection drives a running Peer
// into OpenConfirm with its ConnectRetryCounter at 3, then issues the operator
// stop (shutdownNotify, the ManualStop the daemon's stop path sends every peer).
//
// VALIDATES: exactly one NOTIFICATION Cease (6/2), the TCP connection dropped
// (EOF), the HoldTimer, KeepaliveTimer and ConnectRetryTimer stopped, the session
// released, the FSM in Idle, and the counter set to 0.
// PREVENTS: an OpenConfirm ManualStop that closes without a Cease, keeps a timer
// or the session, or counts the stop as a failed attempt.
//
// RFC requirement: RFC4271-8.2.2-22 positive -- ManualStop (shutdownNotify) on a running Peer in OpenConfirm at ConnectRetryCounter 3 writes exactly one NOTIFICATION 6/2 (Cease), closes the TCP connection, stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, drops the session, moves the FSM to Idle and sets the counter to 0.
func TestRFC4271OpenConfirmManualStopReleasesTheConnection(t *testing.T) {
	n := startOpenConfirmNeighbor(t)
	for range 3 {
		n.peer.connectRetryCounter.Increment()
	}
	require.Equal(t, uint32(3), n.peer.ConnectRetryCounter())

	n.peer.shutdownNotify()

	requireReleased(t, n, [][2]byte{{6, message.NotifyCeaseAdminShutdown}}, 0)
	assert.Equal(t, fsm.StateIdle, n.session.State(), "the state changes to Idle")
}

// TestRFC4271OpenConfirmWithoutManualStopKeepsTheConnection is the input with no
// ManualStop for the OpenConfirm stop test: the counter is at 3 and the far end
// answers with a KEEPALIVE.
//
// VALIDATES: no NOTIFICATION, the connection kept, the session kept and
// Established, and the counter left at 3.
// PREVENTS: the ManualStop action list (Cease, release, counter to zero) running
// in OpenConfirm on an input that is not a ManualStop.
//
// RFC requirement: RFC4271-8.2.2-22 negative -- a running Peer in OpenConfirm at ConnectRetryCounter 3 that receives a KEEPALIVE and no ManualStop writes no NOTIFICATION within 500 ms, keeps the TCP connection and the session, reaches Established and leaves the counter at 3.
func TestRFC4271OpenConfirmWithoutManualStopKeepsTheConnection(t *testing.T) {
	n := startOpenConfirmNeighbor(t)
	for range 3 {
		n.peer.connectRetryCounter.Increment()
	}

	_, err := n.conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return n.session.State() == fsm.StateEstablished }, 5*time.Second, time.Millisecond,
		"a KEEPALIVE in OpenConfirm reaches Established")
	wire, closed := n.readUntil(t, 500*time.Millisecond)

	assert.Empty(t, notifications(t, wire), "no NOTIFICATION")
	assert.False(t, closed, "the connection stays up")
	assert.Same(t, n.session, n.peer.currentSession(), "the session stays the Peer's")
	assert.Equal(t, uint32(3), n.peer.ConnectRetryCounter(), "the counter stays 3")
}

// TestRFC4271EstablishedManualStopReleasesTheConnection runs a Peer to
// Established with a Reactor observing it, sets its ConnectRetryCounter to 3, then
// issues the operator stop (shutdownNotify).
//
// VALIDATES: exactly one NOTIFICATION Cease (6/2) and then EOF at the far end, the
// peer-down raised for that neighbor (the event on which the RIB deletes the
// routes received on the connection), the session released, the HoldTimer,
// KeepaliveTimer and ConnectRetryTimer stopped, and the counter set to 0.
// PREVENTS: an Established ManualStop that closes without a Cease, keeps the
// neighbor's routes, keeps a timer or the session, or counts the stop as a
// failed attempt.
//
// RFC requirement: RFC4271-8.2.2-23 positive -- ManualStop (shutdownNotify) on a running Peer in Established at ConnectRetryCounter 3 writes exactly one NOTIFICATION 6/2 (Cease) and then closes the TCP connection, reports the neighbor closed to its lifecycle observers (the peer-down on which the RIB deletes that neighbor's routes), drops the session, stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, and sets the counter to 0.
func TestRFC4271EstablishedManualStopReleasesTheConnection(t *testing.T) {
	link := startMPLinkNeighbor(t)
	session := link.peer.currentSession()
	require.NotNil(t, session)
	for range 3 {
		link.peer.connectRetryCounter.Increment()
	}
	require.Equal(t, uint32(3), link.peer.ConnectRetryCounter())

	link.peer.shutdownNotify()

	select {
	case closed := <-link.events.closed:
		assert.Same(t, link.peer, closed, "the peer-down names this neighbor")
	case <-time.After(5 * time.Second):
		t.Fatal("no peer-down raised: the RIB would keep the routes received on this connection")
	}
	assert.Equal(t, [][2]byte{{6, message.NotifyCeaseAdminShutdown}}, notifications(t, streamAtEOF(t, link)),
		"exactly one Cease, then the connection is dropped")
	assert.Eventually(t, func() bool { return link.peer.currentSession() != session }, 5*time.Second, time.Millisecond,
		"the Peer releases the session")
	timers := session.timers
	assert.Eventually(t, func() bool { return !timers.IsConnectRetryTimerRunning() }, time.Second, time.Millisecond,
		"ConnectRetryTimer set to zero")
	assert.Eventually(t, func() bool { return !timers.IsHoldTimerRunning() }, time.Second, time.Millisecond,
		"HoldTimer released")
	assert.Eventually(t, func() bool { return !timers.IsKeepaliveTimerRunning() }, time.Second, time.Millisecond,
		"KeepaliveTimer released")
	assert.Eventually(t, func() bool { return link.peer.ConnectRetryCounter() == 0 }, 5*time.Second, time.Millisecond,
		"the ConnectRetryCounter is set to zero")
}

// TestRFC4271EstablishedWithoutManualStopKeepsTheConnection is the input with no
// ManualStop for the Established stop test: the counter is at 3 and the far end
// sends a KEEPALIVE.
//
// VALIDATES: for 300 ms after the KEEPALIVE, no peer-down, the connection not
// dropped, the Peer Established with the same session and running timers, and
// the counter left at 3.
// PREVENTS: the ManualStop action list running in Established on an input that
// is not a ManualStop.
//
// RFC requirement: RFC4271-8.2.2-23 negative -- a running Peer in Established at ConnectRetryCounter 3 that receives a KEEPALIVE and no ManualStop raises no peer-down, keeps the TCP connection, the session, its HoldTimer and KeepaliveTimer, stays Established and leaves the counter at 3.
func TestRFC4271EstablishedWithoutManualStopKeepsTheConnection(t *testing.T) {
	link := startMPLinkNeighbor(t)
	session := link.peer.currentSession()
	require.NotNil(t, session)
	for range 3 {
		link.peer.connectRetryCounter.Increment()
	}

	_, err := link.conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	time.Sleep(300 * time.Millisecond)

	assert.Empty(t, link.events.closed, "no peer-down")
	assert.Empty(t, link.fromPeer, "the connection is not dropped")
	assert.Equal(t, PeerStateEstablished, link.peer.State(), "the Peer stays Established")
	assert.Same(t, session, link.peer.currentSession(), "the session stays the Peer's")
	assert.True(t, session.timers.IsHoldTimerRunning(), "the HoldTimer runs")
	assert.True(t, session.timers.IsKeepaliveTimerRunning(), "the KeepaliveTimer runs")
	assert.Equal(t, uint32(3), link.peer.ConnectRetryCounter(), "the counter stays 3")
}
