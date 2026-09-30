package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// communitiesAttr encodes an extended-length COMMUNITIES attribute of count
// distinct communities, the knob that sizes a single route's attributes.
func communitiesAttr(count int) []byte {
	length := 4 * count
	attr := make([]byte, 0, 4+length)
	attr = append(attr, 0xD0, 0x08, byte(length>>8), byte(length))
	for i := range count {
		attr = append(attr, 0xFD, 0xE8, byte(i>>8), byte(i))
	}
	return attr
}

// TestRFC4271RelayedRouteTooLargeForThePeerIsWithheld drives the relay rail
// (buildFwdBody) toward a destination limited to 4096 octets with a single
// route received from a peer that may send larger UPDATEs.
//
// VALIDATES: a single route whose UPDATE fits 4096 octets is relayed as one
// body, byte-identical to what was received; a single route whose attributes
// alone put its UPDATE past 4096 octets cannot be split, so the rail returns no
// body for that destination at all: no truncated, re-chunked or partial UPDATE.
// PREVENTS: the relay splitting what cannot be split and sending the route
// with its attributes cut, or sending the oversize UPDATE to a peer that never
// negotiated Extended Message.
//
// RFC requirement: RFC4271-9.2-10 positive -- a relayed single route that fits the peer's
// maximum UPDATE size is advertised to it unchanged.
// RFC requirement: RFC4271-9.2-10 negative -- a relayed single route that does not fit the
// peer's maximum UPDATE size is not advertised to it.
func TestRFC4271RelayedRouteTooLargeForThePeerIsWithheld(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, false)
	peer := forwardBodyTestPeer(ctx, ctxID)
	dest := netip.MustParseAddr("192.0.2.10")
	route := forwardBodyNLRIs(1, false)

	t.Run("fits", func(t *testing.T) {
		attrs := append(forwardBodyBaseAttrs(t, 65000), communitiesAttr(900)...)
		body := fwdShapeBody(nil, attrs, route)
		require.LessOrEqual(t, message.HeaderLen+len(body), message.MaxMsgLen, "guard: the UPDATE fits")

		result, ok := buildFwdBody(wireu.NewWireUpdate(body, ctxID), message.MaxMsgLen, ctxID, peer, dest, &fwdParseCache{})
		require.True(t, ok, "a route that fits is advertised")
		require.Len(t, result.rawBodies, 1)
		assert.Equal(t, body, result.rawBodies[0], "the route is relayed unchanged")
	})

	t.Run("does not fit", func(t *testing.T) {
		attrs := append(forwardBodyBaseAttrs(t, 65000), communitiesAttr(1100)...)
		body := fwdShapeBody(nil, attrs, route)
		require.Greater(t, message.HeaderLen+len(body), message.MaxMsgLen, "guard: the single route cannot fit")

		result, ok := buildFwdBody(wireu.NewWireUpdate(body, ctxID), message.MaxMsgLen, ctxID, peer, dest, &fwdParseCache{})
		assert.False(t, ok, "the destination is skipped for this UPDATE")
		assert.Empty(t, result.rawBodies, "no body carries the route")
		assert.Empty(t, result.updates, "no parsed UPDATE carries the route")
	})
}
