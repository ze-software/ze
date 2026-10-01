// Design: docs/architecture/behavior/fsm-established.md — the Established "any other event" row
// Related: rfc4271_established_teardown_peer_test.go — requireEstablishedReleased, streamAtEOF
// RFC: rfc/short/rfc4271.md — Section 8.2.2, Established state, Events 21 and 22

package reactor

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestRFC4271EstablishedHeaderErrorReleasesTheConnection runs a Peer to Established
// with a Reactor observing it, then sends a message whose header fails checking
// (Event 21): an all-zero Marker, or a Length of 18. Event 21 is the one event of
// Established's "any other event (Events 9, 12-13, 20-22)" list Ze can meet there
// (rfc/corrections/rfc4271.md, RFC4271-8.2.2-28); Section 6.1 names its Error Code.
//
// VALIDATES: the Peer writes exactly one NOTIFICATION, Message Header Error with
// subcode Connection Not Synchronized (1/1) or Bad Message Length (1/2), drops the
// connection (EOF at the far end), raises the peer-down for that neighbor, releases
// the session, stops its three session timers, and moves the counter from 0 to 1.
// PREVENTS: a header error in Established that resets without the Section 6.1
// NOTIFICATION, keeps the neighbor's routes, or does not count the failure.
//
// RFC requirement: RFC4271-8.2.2-28 positive -- an Established Peer that reads a header with an all-zero Marker or with Length 18 (Event 21) writes exactly one NOTIFICATION 1/1 or 1/2 (the Section 6.1 Message Header Error, not Finite State Machine Error), drops the TCP connection (EOF), reports the neighbor closed to its lifecycle observers (the RIB's peer-down), drops the session, stops the HoldTimer, KeepaliveTimer and ConnectRetryTimer, and moves the ConnectRetryCounter from 0 to 1.
func TestRFC4271EstablishedHeaderErrorReleasesTheConnection(t *testing.T) {
	var zeroMarker [message.HeaderLen]byte // a KEEPALIVE with an all-zero Marker
	binary.BigEndian.PutUint16(zeroMarker[16:18], message.HeaderLen)
	zeroMarker[18] = 4

	var shortLength [message.HeaderLen]byte // a KEEPALIVE whose Length says 18
	for i := range message.MarkerLen {
		shortLength[i] = 0xFF
	}
	binary.BigEndian.PutUint16(shortLength[16:18], message.HeaderLen-1)
	shortLength[18] = 4

	for _, tc := range []struct {
		name string
		send []byte
		code [2]byte
	}{
		{name: "Marker", send: zeroMarker[:], code: [2]byte{1, 1}},
		{name: "Length", send: shortLength[:], code: [2]byte{1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			link := startMPLinkNeighbor(t)
			session := link.peer.currentSession()
			require.NotNil(t, session)
			require.Equal(t, uint32(0), link.peer.ConnectRetryCounter())

			_, err := link.conn.Write(tc.send)
			require.NoError(t, err)

			requireEstablishedReleased(t, link, session)
			assert.Equal(t, [][2]byte{tc.code}, notifications(t, streamAtEOF(t, link)),
				"one Section 6.1 Message Header Error NOTIFICATION")
		})
	}
}

// TestRFC4271EstablishedValidHeaderKeepsTheConnection is the non-triggering input
// for the test above: the far end sends a KEEPALIVE whose Marker is all ones and
// whose Length is 19, the lower bound Section 4.1 allows, so header checking passes.
//
// VALIDATES: 300 ms later the Peer has raised no peer-down, is still Established
// on the same session, and the counter is 0.
// PREVENTS: a header check that refuses a well-formed header, which would make the
// positive test pass on any message.
//
// RFC requirement: RFC4271-8.2.2-28 negative -- an Established Peer that reads a KEEPALIVE with an all-ones Marker and Length 19 (no Event 21) raises no peer-down, keeps its session, stays Established and leaves the ConnectRetryCounter at 0.
func TestRFC4271EstablishedValidHeaderKeepsTheConnection(t *testing.T) {
	link := startMPLinkNeighbor(t)
	session := link.peer.currentSession()
	require.NotNil(t, session)

	_, err := link.conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	time.Sleep(300 * time.Millisecond)

	assert.Empty(t, link.events.closed, "no peer-down")
	assert.Equal(t, PeerStateEstablished, link.peer.State(), "the Peer stays Established")
	assert.Same(t, session, link.peer.currentSession(), "the session stays the Peer's")
	assert.Equal(t, uint32(0), link.peer.ConnectRetryCounter(), "the counter stays 0")
}
