// Design: docs/architecture/core-design.md -- session ingress validation
// RFC: rfc/short/rfc4271.md
// RFC: rfc/short/rfc7611.md

package reactor

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpfilter "github.com/ze-software/ze/internal/component/bgp/reactor/filter"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/clock"
)

// VALIDATES: an UPDATE from an external peer whose leftmost AS is not the
// peer's AS is handled as a Malformed AS_PATH, which RFC 7606 Section 7.2
// turns into treat-as-withdraw, and the session stays Established.
// PREVENTS: installing a route whose leftmost AS fails the neighbor check.
func TestRFC4271LeftmostASMismatchIsMalformedASPath(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	s, client := firstASSession(t, settings, []capability.Capability{&capability.ASN4{ASN: 65002}}, nil)
	prefix := []byte{24, 203, 0, 113}
	// RFC requirement: RFC4271-6.3-13 positive -- an external UPDATE whose leftmost AS 65003 is not the peer AS 65002 is delivered as a withdrawal of its prefix, not an announcement.
	wu := firstASReceive(t, s, client, makeUpdateBody(nil, firstASAttrs(4, 65003, 65002), prefix))
	require.Equal(t, makeUpdateBody(prefix, nil, nil), wu.Payload())
	// RFC requirement: RFC4271-6.3-13 negative -- an external UPDATE whose leftmost AS is the peer AS is announced unchanged.
	wu = firstASReceive(t, s, client, makeUpdateBody(nil, firstASAttrs(4, 65002, 65003), prefix))
	nlri, err := wu.NLRI()
	require.NoError(t, err)
	require.True(t, bytes.Equal(prefix, nlri), "matching leftmost AS announced %x", nlri)
}

// VALIDATES: collision resolution sends Cease and closes the selected TCP
// connection, while the other connection still transfers bytes both ways.
// PREVENTS: ignoring or reversing the decision, or closing both connections.
// RFC 4271 Section 6.8: "In the event of connection collision, one of the connections MUST be closed."
// MUTATION: ignoring the decision loses the loser's NOTIFICATION/EOF; reversing
// it selects the wrong loser; closing both breaks the survivor's byte exchange.
func TestRFC4271CollisionClosesExactlyOneConnection(t *testing.T) {
	// RFC requirement: RFC4271-6.8-1 positive -- with two live TCP connections and the existing session in OpenConfirm, handlePendingCollision sends Cease 6/7 then EOF on the incoming connection when the local identifier is higher, and on the existing connection when the remote identifier is higher; the selected survivor transfers exact bytes in both directions.
	for _, tc := range []struct {
		name             string
		localID, remote  uint32
		wantCloseCurrent bool
	}{
		{"local-wins", 0x0a000002, 0x0a000001, false},
		{"remote-wins", 0x0a000001, 0x0a000002, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			existing, current := collisionProofTCPPair(t)
			incoming, pending := collisionProofTCPPair(t)
			settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65001, 65002, tc.localID)
			settings.Connection = ConnectionPassive
			session := NewSession(settings)
			t.Cleanup(func() { require.NoError(t, session.Stop()) })
			// RFC 4271 Section 8.2.2: reach OpenConfirm through the real OPEN exchange.
			require.NoError(t, session.Start())
			require.NoError(t, session.Accept(current))
			reply, err := core4271ReadMessage(existing)
			require.NoError(t, err)
			require.Equal(t, byte(1), reply[18], "the existing connection receives our OPEN")
			// RFC 4271 Section 4.2: both connections advertise the same remote Identifier.
			open := message.PackTo(&message.Open{
				Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: tc.remote,
			}, nil)
			_, err = existing.Write(open)
			require.NoError(t, err)
			// RFC 4271 Section 8.2.2: process the peer OPEN before collision resolution.
			require.NoError(t, session.ReadAndProcess())
			reply, err = core4271ReadMessage(existing)
			require.NoError(t, err)
			require.Equal(t, byte(4), reply[18], "the OPEN receives a KEEPALIVE")
			require.Equal(t, fsm.StateOpenConfirm, session.State())

			peer := NewPeer(settings)
			peer.session = session
			// RFC 4271 Section 6.8: enter the consumer with a pending socket and its OPEN.
			require.NoError(t, peer.setPendingConnection(pending))
			_, err = incoming.Write(open)
			require.NoError(t, err)
			r := &Reactor{clock: clock.RealClock{}}
			r.handlePendingCollision(peer, pending)
			require.False(t, peer.hasPendingConnection())

			loser, survivor, survivorLocal := incoming, existing, current
			if tc.wantCloseCurrent {
				loser, survivor, survivorLocal = existing, incoming, pending
				peer.mu.RLock()
				retained := peer.inbound.conn
				peer.mu.RUnlock()
				require.Same(t, pending, retained, "the incoming winner is retained for the next session")
			} else {
				require.Equal(t, fsm.StateOpenConfirm, session.State())
			}
			// RFC 4271 Section 6.8: "Closing the BGP connection (that results from the collision
			// resolution procedure) is accomplished by sending the NOTIFICATION message with the Error Code Cease."
			requireCollisionCease(t, loser, "the selected losing connection")
			var octet [1]byte
			n, err := loser.Read(octet[:])
			require.Zero(t, n, "nothing follows the one collision NOTIFICATION")
			require.ErrorIs(t, err, io.EOF, "the consumer closes the loser before fixture cleanup")

			// TCP writes alone can succeed after remote closure. Read back exact bytes
			// in both directions to prove the selected survivor is still usable.
			for _, direction := range []struct{ from, to net.Conn }{
				{survivor, survivorLocal}, {survivorLocal, survivor},
			} {
				payload := []byte{0x43, 0x4f, 0x4c, 0x4c}
				n, err := direction.from.Write(payload)
				require.NoError(t, err, "the selected survivor accepts a write")
				require.Equal(t, len(payload), n)
				var received [4]byte
				_, err = io.ReadFull(direction.to, received[:])
				require.NoError(t, err, "the selected survivor delivers the write")
				require.Equal(t, payload, received[:])
			}
		})
	}
}

// collisionProofTCPPair gives the collision consumer real sockets with bounded
// I/O. Cleanup runs only after the test observes the loser and survivor.
func collisionProofTCPPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // Test cleanup.
	tcpListener, ok := listener.(*net.TCPListener)
	require.True(t, ok)
	require.NoError(t, tcpListener.SetDeadline(time.Now().Add(5*time.Second)))
	far, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(t.Context(), "tcp4", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { far.Close() }) //nolint:errcheck // The loser is already closed.
	near, err := listener.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { near.Close() }) //nolint:errcheck // The loser is already closed.
	require.NoError(t, far.SetDeadline(time.Now().Add(5*time.Second)))
	require.NoError(t, near.SetDeadline(time.Now().Add(5*time.Second)))
	return far, near
}

// VALIDATES: a route this router itself originated (its own ORIGINATOR_ID)
// is refused at ingress even when it carries ACCEPT_OWN, so it never
// re-enters the routing context it came from; a route from another
// originator carrying the same community is accepted.
// PREVENTS: ACCEPT_OWN reinjecting a route into its own source context.
func TestRFC7611OwnRouteNeverReacceptedIntoSource(t *testing.T) {
	for _, own := range []bool{true, false} {
		id := uint32(0x01020305)
		if own {
			id = 0x01020304
		}
		attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, byte(attribute.AttrOriginatorID), 4}
		attrs = binary.BigEndian.AppendUint32(attrs, id)
		attrs = append(attrs, 0xc0, byte(attribute.AttrCommunity), 4)
		attrs = binary.BigEndian.AppendUint32(attrs, uint32(attribute.CommunityAcceptOwn))
		accepted, _ := bgpfilter.LoopIngress(filterapi.PeerFilterInfo{LocalAS: 65001, PeerAS: 65001, RouterID: 0x01020304}, makeUpdateBody(nil, attrs, []byte{24, 10, 20, 0}), nil)
		if own {
			// RFC requirement: RFC7611-2.1-1 negative -- a route carrying ACCEPT_OWN whose ORIGINATOR_ID is this router is refused at ingress.
			require.False(t, accepted)
			continue
		}
		// RFC requirement: RFC7611-2.1-1 positive -- a route carrying ACCEPT_OWN from another originator is accepted at ingress.
		require.True(t, accepted)
	}
}
