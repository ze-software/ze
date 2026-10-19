package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// RFC 7606 Section 5.1: "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if
// present) SHALL be encoded as the very first path attribute in an UPDATE
// message." and "An UPDATE message MUST NOT contain more than one of the
// following: non-empty Withdrawn Routes field, non-empty Network Layer
// Reachability Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI
// attribute."
//
// An originated UPDATE reaches the peer through the egress gate in
// writeUpdateGated, where a raw export filter can replace the whole body. Ze
// sends that override, so it owes the override the shape the forward path gives
// a relayed UPDATE. These tests install the override as the session's
// egressRouteFilter, send an originated IPv4 route through Session.SendUpdate and
// read the UPDATEs that reached the socket.

// overrideShapeIPv4NLRI is 10.0.0.0/24, the originated route the override replaces.
var overrideShapeIPv4NLRI = []byte{24, 10, 0, 0}

// overrideShapeSend sends an originated IPv4 route whose export filter returns
// override, and returns the UPDATE bodies that reached the socket in order.
func overrideShapeSend(t *testing.T, override []byte) [][]byte {
	t.Helper()
	peer, conn := newAnnouncePeer(t, "192.0.2.2")
	s := peer.session
	source := append([]byte(nil), override...)
	s.egressRouteFilter = func([]byte) (bool, []byte) { return false, override }
	require.NoError(t, s.SendUpdate(&message.Update{
		PathAttributes: forwardBodyBaseAttrs(t, 65000),
		NLRI:           overrideShapeIPv4NLRI,
	}))
	require.Equal(t, source, override, "the writer edited the filter's bytes in place")

	var bodies [][]byte
	stream := conn.written()
	for len(stream) > 0 {
		require.GreaterOrEqual(t, len(stream), message.HeaderLen, "a truncated message reached the socket")
		size := int(binary.BigEndian.Uint16(stream[16:18]))
		require.GreaterOrEqual(t, len(stream), size, "a truncated message reached the socket")
		bodies = append(bodies, stream[message.HeaderLen:size])
		stream = stream[size:]
	}
	return bodies
}

// VALIDATES: a raw export-filter override whose MP_REACH_NLRI follows ORIGIN,
// AS_PATH and NEXT_HOP reaches the peer with MP_REACH_NLRI first, and is
// otherwise the byte string of the same override with MP_REACH_NLRI first.
// PREVENTS: the egress gate writing an override's attribute order as returned.
//
// RFC requirement: RFC7606-5.1-1 positive -- an originated UPDATE whose raw export-filter override puts MP_REACH_NLRI after other path attributes is sent with MP_REACH_NLRI as the first path attribute, with every other byte equal to the override's.
func TestEgressOverrideMovesTrailingMPAttributeFirst(t *testing.T) {
	trailing, leading := fwdMPFirstBodies(t, 65000)

	bodies := overrideShapeSend(t, trailing)
	require.Len(t, bodies, 1, "an MP-only override that fits is one UPDATE")
	assert.Equal(t, leading, bodies[0], "the override must leave with MP_REACH_NLRI first and nothing else changed")
}

// VALIDATES: a raw export-filter override that already leads with MP_REACH_NLRI
// reaches the peer byte for byte as the filter returned it.
// PREVENTS: the shape check rewriting a compliant override.
func TestEgressOverrideKeepsLeadingMPAttribute(t *testing.T) {
	_, leading := fwdMPFirstBodies(t, 65000)

	bodies := overrideShapeSend(t, leading)
	require.Len(t, bodies, 1)
	assert.Equal(t, leading, bodies[0])
}

// VALIDATES: a raw export-filter override that fits in one message but carries
// both IPv4 NLRI and a trailing MP_REACH_NLRI reaches the peer as UPDATEs that
// each carry exactly one NLRI-bearing field, together announcing both routes,
// and every UPDATE with an MP attribute leads with it.
// PREVENTS: the egress gate sizing a mixed override with Split, which emits an
// UPDATE that fits unsplit.
//
// RFC requirement: RFC7606-5.1-2 positive -- an originated UPDATE whose raw export-filter override fits in one message but carries both a non-empty NLRI field and an MP_REACH_NLRI attribute is sent as UPDATEs that each carry exactly one NLRI-bearing field, with the IPv4 NLRI and the MP_REACH_NLRI both sent.
func TestEgressOverrideSplitsMixedShape(t *testing.T) {
	attrs := append(append([]byte{}, forwardBodyBaseAttrs(t, 65000)...), fwdMPFirstReach...)
	mixed := fwdShapeBody(nil, attrs, overrideShapeIPv4NLRI)
	require.Less(t, len(mixed)+message.HeaderLen, message.MaxMsgLen, "the override must fit, so only its shape can split it")

	bodies := overrideShapeSend(t, mixed)
	require.Len(t, bodies, 2, "a mixed override must leave as two UPDATEs")
	sawNLRI, sawReach := false, false
	for _, body := range bodies {
		require.Equal(t, 1, fwdShapeFieldCount(t, body), "each UPDATE carries one NLRI-bearing field")
		u, err := message.UnpackUpdate(body)
		require.NoError(t, err)
		if len(u.NLRI) > 0 {
			assert.Equal(t, overrideShapeIPv4NLRI, u.NLRI)
			sawNLRI = true
			continue
		}
		assert.Equal(t, attribute.AttrMPReachNLRI, fwdMPFirstCode(t, u.PathAttributes), "the MP UPDATE leads with MP_REACH_NLRI")
		sawReach = true
	}
	assert.True(t, sawNLRI, "the IPv4 route must be sent")
	assert.True(t, sawReach, "the MP_REACH_NLRI route must be sent")
}
