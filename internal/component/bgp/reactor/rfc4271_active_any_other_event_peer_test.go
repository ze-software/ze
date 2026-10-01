// Design: docs/architecture/behavior/fsm.md — the peer run loop owns the session an Active teardown releases
// Related: rfc4271_fsm_teardown_peer_test.go — the same teardown proven from OpenSent and later states
// RFC: rfc/short/rfc4271.md — Section 8.2.2, Connect and Active states

package reactor

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// startActivePeer runs a passive Peer until its session sits in Active,
// listening for a connection that never comes, then sets its
// ConnectRetryCounter to 3. The
// reconnect delay is a minute, so the session the test inspects is the only one
// the Peer builds while the test runs.
func startActivePeer(t *testing.T) (*Peer, *Session) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Connection = ConnectionPassive
	peer := NewPeer(settings)
	peer.setReconnectDelay(time.Minute, time.Minute)

	t.Cleanup(startAndStop(t, peer))

	var session *Session
	require.Eventually(t, func() bool {
		session = peer.currentSession()
		return session != nil && session.State() == fsm.StateActive
	}, 5*time.Second, time.Millisecond, "the passive Peer reaches Active")

	// The operator start (Event 1) zeroed the counter on the way to Active, so
	// the history is seeded only now, where an increment and a reset differ.
	for range 3 {
		peer.connectRetryCounter.Increment()
	}
	require.Equal(t, uint32(3), peer.ConnectRetryCounter())
	return peer, session
}

// TestRFC4271ActiveAutomaticStopReleasesThePeer drives a running passive Peer
// into Active, then stops its session the way the local system does
// (teardownAutomatic, the Event 8 path a BFD Down or a forward-pool overflow
// takes).
//
// VALIDATES: the session's FSM goes to Idle, the ConnectRetryCounter moves from
// 3 to 4, the HoldTimer, KeepaliveTimer and ConnectRetryTimer are stopped, the
// Peer no longer holds the session, and a TCP connection offered to the stopped
// session afterwards is refused with ErrSessionTearingDown and never becomes its
// connection.
// PREVENTS: an AutomaticStop in Active that leaves the listening session alive,
// or one that does not count the failed attempt.
//
// RFC requirement: RFC4271-8.2.2-15 positive -- AutomaticStop (Event 8, Session.teardownAutomatic) on a running passive Peer in Active moves its FSM to Idle and the ConnectRetryCounter from 3 to 4, stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, drops the session from the Peer, and refuses a TCP connection offered to the stopped session.
// RFC requirement: RFC4271-8.2.2-21 positive -- AutomaticStop (Event 8, Session.teardownAutomatic) on a running passive Peer in Active leaves the ConnectRetryTimer stopped, stops the HoldTimer and KeepaliveTimer, drops the session from the Peer, refuses a TCP connection offered to the stopped session, moves the ConnectRetryCounter from 3 to 4 and the FSM to Idle.
func TestRFC4271ActiveAutomaticStopReleasesThePeer(t *testing.T) {
	peer, session := startActivePeer(t)

	require.NoError(t, session.teardownAutomatic(message.NotifyCeaseOutOfResources, ""))

	assert.Equal(t, fsm.StateIdle, session.State(), "the state changes to Idle")
	assert.Eventually(t, func() bool { return peer.ConnectRetryCounter() == 4 }, 5*time.Second, time.Millisecond,
		"the ConnectRetryCounter is incremented by 1")
	assert.Eventually(t, func() bool { return peer.currentSession() != session }, 5*time.Second, time.Millisecond,
		"the Peer releases the session")
	timers := session.timers
	assert.False(t, timers.IsConnectRetryTimerRunning(), "ConnectRetryTimer stopped")
	assert.False(t, timers.IsHoldTimerRunning(), "HoldTimer released")
	assert.False(t, timers.IsKeepaliveTimerRunning(), "KeepaliveTimer released")

	local, remote := net.Pipe()
	t.Cleanup(func() {
		assert.NoError(t, local.Close())
		assert.NoError(t, remote.Close())
	})
	require.ErrorIs(t, session.Accept(local), ErrSessionTearingDown, "the stopped session takes no TCP connection")
	assert.Nil(t, session.Conn(), "the stopped session holds no TCP connection")
	assert.Equal(t, uint32(4), peer.ConnectRetryCounter(), "the refused connection is not counted again")
}

// TestRFC4271ActiveDuplicateStartKeepsThePeer drives a running passive Peer into
// Active, then repeats the operator start on its session (Event 1, which §8.2.2
// says is ignored in Active).
//
// VALIDATES: the FSM stays in Active, the ConnectRetryCounter stays at 3, and the
// Peer still holds the same session, which still accepts a TCP connection.
// PREVENTS: the Active "any other event" action list running for an event it
// does not name.
//
// RFC requirement: RFC4271-8.2.2-15 negative -- ManualStart (Event 1, Session.Start) on a running passive Peer in Active leaves the FSM in Active, the ConnectRetryCounter at 3, and the session held by the Peer and not refusing connections.
// RFC requirement: RFC4271-8.2.2-21 negative -- ManualStart (Event 1, Session.Start), an event the Active "any other event" list does not name, on a running passive Peer in Active leaves the FSM in Active, the ConnectRetryCounter at 3, and the session held by the Peer and not sealed.
func TestRFC4271ActiveDuplicateStartKeepsThePeer(t *testing.T) {
	peer, session := startActivePeer(t)

	require.NoError(t, session.Start())

	assert.Equal(t, fsm.StateActive, session.State(), "Event 1 is ignored in Active")
	assert.Never(t, func() bool { return peer.ConnectRetryCounter() != 3 }, 300*time.Millisecond, 10*time.Millisecond,
		"Event 1 does not move the ConnectRetryCounter")
	assert.Same(t, session, peer.currentSession(), "the Peer keeps the session")
	assert.False(t, session.tearingDown.Load(), "the session still accepts a TCP connection")
}
