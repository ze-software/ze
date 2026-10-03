// Design: docs/architecture/behavior/fsm-open-confirm.md — collision detection on a second connection
// Related: collision_test.go — setupOpenConfirmSession; rfc4271_session_core4271_test.go — core4271ReadMessage
// RFC: rfc/short/rfc4271.md — Section 6.8, connection collision detection

package reactor

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
)

// collisionCease is the NOTIFICATION a connection closed by collision detection
// carries: Cease (6), Connection Collision Resolution (7).
var collisionCease = []byte{6, 7}

// sendSecondOpen hands the Reactor a second connection for peer, as its listener
// would, and writes a complete OPEN with remoteID on it. It returns the far end of
// that second connection.
func sendSecondOpen(t *testing.T, peer *Peer, remoteID uint32) net.Conn {
	t.Helper()
	r := &Reactor{clock: clock.RealClock{}}
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))

	accepted := make(chan struct{})
	go func() { r.acceptOrReject(server, peer, nil); close(accepted) }()
	<-accepted
	require.True(t, peer.hasPendingConnection(), "the second connection is tracked until its OPEN")

	_, err := client.Write(message.PackTo(&message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: remoteID}, nil))
	require.NoError(t, err)
	return client
}

// requireCollisionCease reads one message from conn and requires it to be the
// Cease / Connection Collision Resolution NOTIFICATION.
func requireCollisionCease(t *testing.T, conn net.Conn, which string) {
	t.Helper()
	reply, err := core4271ReadMessage(conn)
	require.NoError(t, err, "%s: a NOTIFICATION is written", which)
	require.Equal(t, byte(3), reply[18], "%s: the message is a NOTIFICATION", which)
	require.Equal(t, collisionCease, reply[message.HeaderLen:], "%s: Cease, Connection Collision Resolution", which)
}

// TestRFC4271ReceivedOpenExaminesTheOpenConfirmConnection holds a session in
// OpenConfirm (local BGP Identifier 10.0.0.2), then hands the Reactor a second
// connection to the same peer and sends an OPEN on it, the path the listener
// takes. The OPEN's receipt is what triggers collision detection.
//
// VALIDATES: the outcome follows the OpenConfirm connection's BGP Identifier
// comparison. A remote Identifier above the local one closes the OpenConfirm
// connection with a Cease 6/7; one below it closes the new connection with a
// Cease 6/7 and leaves the OpenConfirm session in place.
// PREVENTS: an OPEN on a second connection that is accepted or refused without
// looking at the connection already in OpenConfirm.
//
// RFC requirement: RFC4271-6.8-2 positive -- on receipt of an OPEN on a second connection (Reactor.acceptOrReject, then handlePendingCollision), the session in OpenConfirm is examined: with the remote BGP Identifier 10.0.0.3 above the local 10.0.0.2 the OpenConfirm connection receives NOTIFICATION 6/7; with 10.0.0.1 below it the new connection receives NOTIFICATION 6/7 and the OpenConfirm session stays in OpenConfirm.
func TestRFC4271ReceivedOpenExaminesTheOpenConfirmConnection(t *testing.T) {
	const localID = 0x0a000002

	t.Run("remote higher closes the OpenConfirm connection", func(t *testing.T) {
		session, existing, server := setupOpenConfirmSession(t, localID)
		t.Cleanup(func() { _ = existing.Close(); _ = server.Close() })
		require.NoError(t, existing.SetDeadline(time.Now().Add(5*time.Second)))
		peer := NewPeer(session.settings)
		peer.session = session

		sendSecondOpen(t, peer, 0x0a000003)

		requireCollisionCease(t, existing, "the OpenConfirm connection")
	})

	t.Run("remote lower closes the new connection", func(t *testing.T) {
		session, existing, server := setupOpenConfirmSession(t, localID)
		t.Cleanup(func() { _ = existing.Close(); _ = server.Close() })
		peer := NewPeer(session.settings)
		peer.session = session

		second := sendSecondOpen(t, peer, 0x0a000001)

		requireCollisionCease(t, second, "the new connection")
		require.False(t, peer.hasPendingConnection())
		require.Equal(t, fsm.StateOpenConfirm, session.State(), "the OpenConfirm session stays")
	})
}

// TestRFC4271ReceivedOpenWithNoOpenConfirmConnection is the non-triggering input
// for the test above: the existing session is Established, not OpenConfirm, and
// the OPEN on the second connection carries a remote BGP Identifier (255.255.255.255)
// that would win an OpenConfirm comparison.
//
// VALIDATES: the Established session is not subjected to the OpenConfirm
// comparison: the new connection receives the Cease 6/7, and the existing session
// stays Established.
// PREVENTS: an examination applied to every connection whatever its state, which
// would close an Established session for a higher remote Identifier.
//
// RFC requirement: RFC4271-6.8-2 negative -- with the existing session Established (no connection in OpenConfirm), an OPEN received on a second connection with the higher remote BGP Identifier 255.255.255.255 closes the new connection with NOTIFICATION 6/7 and leaves the existing session Established.
func TestRFC4271ReceivedOpenWithNoOpenConfirmConnection(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session, _ := firstASSession(t, settings, nil, nil)
	require.Equal(t, fsm.StateEstablished, session.State())
	peer := NewPeer(settings)
	peer.session = session
	peer.setState(PeerStateEstablished)

	second := sendSecondOpen(t, peer, 0xffffffff)

	requireCollisionCease(t, second, "the new connection")
	require.False(t, peer.hasPendingConnection())
	require.Equal(t, fsm.StateEstablished, session.State(), "the Established session stays")
}
