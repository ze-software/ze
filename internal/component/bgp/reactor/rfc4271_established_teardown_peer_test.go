// Design: docs/architecture/behavior/fsm.md — the peer run loop owns the connection an Established teardown releases
// Related: rfc4760_peer_down_link_test.go — the Established harness with the peer-down observer
// Related: ../plugins/rib/rfc4760_peer_down_test.go — the RIB deleting a neighbor's routes on that peer-down
// RFC: rfc/short/rfc4271.md — Section 8.2.2, Established state

package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// requireEstablishedReleased asserts the Section 8.2.2 Established teardown as the
// Peer, its observers and the far end see it: the Peer reports the neighbor closed to
// its lifecycle observers (the peer-down on which the RIB deletes the routes received
// on the connection), releases the session, stops the session's HoldTimer,
// KeepaliveTimer and ConnectRetryTimer, and moves the ConnectRetryCounter from 0 to 1.
// It returns the session it watched.
func requireEstablishedReleased(t *testing.T, link *mpLinkNeighbor, session *Session) {
	t.Helper()
	select {
	case closed := <-link.events.closed:
		assert.Same(t, link.peer, closed, "the peer-down names this neighbor")
	case <-time.After(5 * time.Second):
		t.Fatal("no peer-down raised: the RIB would keep the routes received on this connection")
	}
	assert.Eventually(t, func() bool { return link.peer.currentSession() != session }, 5*time.Second, time.Millisecond,
		"the Peer releases the session")
	timers := session.timers
	assert.Eventually(t, func() bool { return !timers.IsConnectRetryTimerRunning() }, time.Second, time.Millisecond,
		"ConnectRetryTimer set to zero")
	assert.Eventually(t, func() bool { return !timers.IsHoldTimerRunning() }, time.Second, time.Millisecond,
		"HoldTimer released")
	assert.Eventually(t, func() bool { return !timers.IsKeepaliveTimerRunning() }, time.Second, time.Millisecond,
		"KeepaliveTimer released")
	assert.Eventually(t, func() bool { return link.peer.ConnectRetryCounter() == 1 }, 5*time.Second, time.Millisecond,
		"the ConnectRetryCounter is incremented by 1")
}

// streamAtEOF returns what the Peer wrote after the handshake, once the Peer has
// dropped the TCP connection. It fails when the connection is still open.
func streamAtEOF(t *testing.T, link *mpLinkNeighbor) []byte {
	t.Helper()
	select {
	case stream := <-link.fromPeer:
		return stream
	case <-time.After(10 * time.Second):
		t.Fatal("the Peer never dropped the TCP connection")
		return nil
	}
}

// incorrectMPReach is an UPDATE whose MP_REACH_NLRI announces a /24 with two of its
// three prefix octets, an error RFC 7606 still answers with a session reset.
func incorrectMPReach() []byte {
	value := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01,                   // SAFI = 1 (Unicast)
		0x04,                   // Next-hop length = 4
		0xc0, 0x00, 0x02, 0x01, // Next hop = 192.0.2.1
		0x00,             // Reserved
		0x18, 0x0a, 0x00, // NLRI: /24 needs three octets, two follow
	}
	return buildUpdateMsg(mpReachUpdate(append(firstASAttrs(4, 65001), mpAttr(14, value)...)))
}

// TestRFC4271EstablishedNotificationOrTCPFailureReleasesTheConnection runs a Peer to
// Established with a Reactor observing it, then ends the session from the far end in
// one of the three ways Section 8.2.2 lists: a NOTIFICATION with a version error
// (Event 24), any other NOTIFICATION (Event 25, a Cease), or closing the TCP
// connection (Event 18).
//
// VALIDATES: for each, the Peer raises the peer-down for that neighbor, releases the
// session, stops the three session timers and moves the counter from 0 to 1; for the
// two NOTIFICATIONs it writes no NOTIFICATION of its own and drops the connection (EOF
// at the far end).
// PREVENTS: an Established session that outlives its peer's goodbye, keeps a timer,
// keeps the neighbor's routes (no peer-down), or does not count the failure.
//
// RFC requirement: RFC4271-8.2.2-12 positive -- an Established Peer that receives NOTIFICATION 2/1 (Event 24), NOTIFICATION 6/2 (Event 25) or a far-end TCP close (Event 18) reports the neighbor closed to its lifecycle observers (the RIB's peer-down), drops the session, stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, and moves the ConnectRetryCounter from 0 to 1; for Events 24 and 25 it writes no NOTIFICATION and the far end reads EOF.
func TestRFC4271EstablishedNotificationOrTCPFailureReleasesTheConnection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		notify *message.Notification
	}{
		{name: "Event 24 version error", notify: &message.Notification{
			ErrorCode: message.NotifyOpenMessage, ErrorSubcode: 1, Data: []byte{0, 4},
		}},
		{name: "Event 25 Cease", notify: &message.Notification{
			ErrorCode: message.NotifyCease, ErrorSubcode: message.NotifyCeaseAdminShutdown,
		}},
		{name: "Event 18 TCP failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			link := startMPLinkNeighbor(t)
			session := link.peer.currentSession()
			require.NotNil(t, session)
			require.Equal(t, uint32(0), link.peer.ConnectRetryCounter())

			if tc.notify == nil {
				require.NoError(t, link.conn.Close())
				requireEstablishedReleased(t, link, session)
				return
			}
			_, err := link.conn.Write(message.PackTo(tc.notify, nil))
			require.NoError(t, err)

			requireEstablishedReleased(t, link, session)
			assert.Empty(t, notifications(t, streamAtEOF(t, link)), "the Peer answers a NOTIFICATION with none")
		})
	}
}

// TestRFC4271EstablishedUpdateErrorReleasesTheConnection runs a Peer to Established
// with a Reactor observing it, then sends an UPDATE whose MP_REACH_NLRI is malformed,
// which the UPDATE error handling answers with a session reset (Event 28).
//
// VALIDATES: exactly one NOTIFICATION with Error Code 3 (UPDATE Message Error), the
// connection dropped (EOF), the peer-down raised for that neighbor, the session
// released, the three session timers stopped, and the counter moved from 0 to 1.
// PREVENTS: an UPDATE error that resets without telling the peer, without the
// peer-down that clears the neighbor's routes, or without counting the failure.
//
// RFC requirement: RFC4271-8.2.2-14 positive -- an Established Peer that receives an UPDATE with a malformed MP_REACH_NLRI (Event 28) writes exactly one NOTIFICATION with Error Code 3, drops the TCP connection (EOF), reports the neighbor closed to its lifecycle observers (the RIB's peer-down), drops the session, stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, and moves the ConnectRetryCounter from 0 to 1.
// RFC requirement: RFC4271-6.3-1 positive -- the UPDATE error of a malformed MP_REACH_NLRI on a running Established Peer is indicated by exactly one NOTIFICATION whose Error Code is 3 (UPDATE Message Error).
// RFC requirement: RFC4271-6.7-1 negative -- an UPDATE Message Error (malformed MP_REACH_NLRI) on a running Established Peer is answered by exactly one NOTIFICATION, Error Code 3, never Cease.
func TestRFC4271EstablishedUpdateErrorReleasesTheConnection(t *testing.T) {
	link := startMPLinkNeighbor(t)
	session := link.peer.currentSession()
	require.NotNil(t, session)
	require.Equal(t, uint32(0), link.peer.ConnectRetryCounter())

	_, err := link.conn.Write(incorrectMPReach())
	require.NoError(t, err)

	requireEstablishedReleased(t, link, session)
	got := notifications(t, streamAtEOF(t, link))
	require.Len(t, got, 1, "exactly one NOTIFICATION")
	assert.Equal(t, byte(message.NotifyUpdateMessage), got[0][0], "Error Code UPDATE Message Error")
}

// TestRFC4271EstablishedGoodMessagesKeepTheConnection is the non-triggering input for
// the two Established teardown tests: the far end sends a KEEPALIVE and a well-formed
// UPDATE, no NOTIFICATION, no error, and keeps the connection.
//
// VALIDATES: the UPDATE reaches the Peer's consumers, and afterwards the Peer has
// raised no peer-down, is still Established on the same session with its HoldTimer and
// KeepaliveTimer running, and the counter is 0.
// PREVENTS: an Established session that tears down, or drops the neighbor's routes, on
// an ordinary message.
//
// RFC requirement: RFC4271-8.2.2-12 negative -- an Established Peer that receives a KEEPALIVE and a well-formed UPDATE (no NOTIFICATION, no TCP failure) raises no peer-down, keeps its session, HoldTimer and KeepaliveTimer, stays Established and leaves the ConnectRetryCounter at 0.
// RFC requirement: RFC4271-8.2.2-14 negative -- an Established Peer that receives a well-formed UPDATE (no UPDATE error) dispatches it, raises no peer-down, keeps its session and timers, stays Established and leaves the ConnectRetryCounter at 0.
func TestRFC4271EstablishedGoodMessagesKeepTheConnection(t *testing.T) {
	link := startMPLinkNeighbor(t)
	session := link.peer.currentSession()
	require.NotNil(t, session)

	correct := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01,                   // SAFI = 1 (Unicast)
		0x04,                   // Next-hop length = 4
		0xc0, 0x00, 0x02, 0x01, // Next hop = 192.0.2.1
		0x00,                   // Reserved
		0x18, 0x0a, 0x00, 0x00, // NLRI: 10.0.0.0/24, three octets present
	}
	_, err := link.conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	_, err = link.conn.Write(buildUpdateMsg(mpReachUpdate(append(firstASAttrs(4, 65001), mpAttr(14, correct)...))))
	require.NoError(t, err)

	select {
	case <-link.updates:
	case <-time.After(5 * time.Second):
		t.Fatal("the well-formed UPDATE never reached the consumers")
	}
	time.Sleep(300 * time.Millisecond)

	assert.Empty(t, link.events.closed, "no peer-down")
	assert.Equal(t, PeerStateEstablished, link.peer.State(), "the Peer stays Established")
	assert.Same(t, session, link.peer.currentSession(), "the session stays the Peer's")
	assert.True(t, session.timers.IsHoldTimerRunning(), "the HoldTimer runs")
	assert.True(t, session.timers.IsKeepaliveTimerRunning(), "the KeepaliveTimer runs")
	assert.Equal(t, uint32(0), link.peer.ConnectRetryCounter(), "the counter stays 0")
}
