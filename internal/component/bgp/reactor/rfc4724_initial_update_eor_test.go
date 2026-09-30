package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/family"
)

// splitWireMessages cuts a byte stream of BGP messages at each header's Length.
func splitWireMessages(t *testing.T, wire []byte) [][]byte {
	t.Helper()
	var out [][]byte
	for len(wire) > 0 {
		require.GreaterOrEqual(t, len(wire), message.HeaderLen, "truncated header")
		length := int(binary.BigEndian.Uint16(wire[16:18]))
		require.GreaterOrEqual(t, length, message.HeaderLen)
		require.GreaterOrEqual(t, len(wire), length, "truncated message")
		out = append(out, wire[:length])
		wire = wire[length:]
	}
	return out
}

// TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete drives the initial
// sync of an established session and reads what reached the socket.
//
// VALIDATES: with nothing to send, the End-of-RIB for IPv4 unicast is the one
// message on the wire; with a default-originate route to send, the wire holds
// the route's UPDATE and then exactly one End-of-RIB, as the last message, with
// no marker before the route.
// PREVENTS: a marker that is skipped when the table is empty, or one written
// before the initial update has finished, which tells the peer the table is
// complete while a route is still to come.
//
// RFC requirement: RFC4724-4.1-7 positive -- once the initial update for IPv4 unicast is
// complete the End-of-RIB is sent, both with no route to send and after a default route.
// RFC requirement: RFC4724-4.1-7 negative -- no End-of-RIB precedes the initial update's
// route, and exactly one follows it.
func TestRFC4724EndOfRIBSentOnceTheInitialUpdateIsComplete(t *testing.T) {
	eor := eorWire(family.IPv4Unicast)

	t.Run("nothing to send", func(t *testing.T) {
		peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
		peer.sendInitialRoutes()
		assert.Equal(t, eor, conn.written(), "the marker is owed with no update to send")
	})

	t.Run("after the default route", func(t *testing.T) {
		peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
		peer.settings.LocalAddress = netip.MustParseAddr("10.0.0.1")
		peer.settings.DefaultOriginate = map[string]bool{"ipv4/unicast": true}
		peer.sendInitialRoutes()

		msgs := splitWireMessages(t, conn.written())
		require.Len(t, msgs, 2, "the route's UPDATE, then the marker")
		assert.NotEqual(t, eor, msgs[0], "no marker before the initial update's route")
		// UPDATE body: no withdrawn routes, attributes, then NLRI 0.0.0.0/0 (one zero octet).
		assert.True(t, bytes.HasSuffix(msgs[0], []byte{0x00}), "the first message announces 0.0.0.0/0")
		assert.Equal(t, eor, msgs[1], "the marker follows the completed update")
		assert.Equal(t, uint32(1), peer.Stats().EORSent)
	})
}
