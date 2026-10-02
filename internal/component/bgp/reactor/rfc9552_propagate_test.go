// Design: docs/architecture/wire/nlri-bgpls.md -- BGP-LS NLRI and attribute propagation
// RFC: rfc/short/rfc9552.md -- Section 5.1, unknown and unsupported types are preserved and propagated
// Overview: reactor_api_forward.go -- forwardUpdateCore, the rail that propagates a received UPDATE
// Related: rfc_draft_linklocal_reflect_test.go -- the forward harness this file follows

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// rfc9552UnknownSubTLVNodeNLRI is a Link-State Node NLRI (RFC 9552 Section 5.2) whose Local
// Node Descriptors hold the Autonomous System sub-TLV 512 for AS 65001 and sub-TLV 700, a
// type no document assigns, carrying 0xBEEF.
var rfc9552UnknownSubTLVNodeNLRI = []byte{
	0x00, 0x01, 0x00, 0x1b, // NLRI Type 1, Total NLRI Length 27
	0x02,                                           // Protocol-ID: IS-IS Level 2
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
	0x01, 0x00, 0x00, 0x0e, // TLV 256 Local Node Descriptors, length 14
	0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9, // sub-TLV 512 Autonomous System 65001
	0x02, 0xbc, 0x00, 0x02, 0xbe, 0xef, // sub-TLV 700 (unassigned), value 0xBEEF
}

// rfc9552UnknownAttrValue is a BGP-LS Attribute value holding the IPv4 Router-ID TLV 1028,
// TLV 65000 (unassigned) and TLV 1088 (a Link attribute TLV, unexpected beside a Node NLRI).
var rfc9552UnknownAttrValue = []byte{
	0x04, 0x04, 0x00, 0x04, 0xc0, 0x00, 0x02, 0x01, // TLV 1028 IPv4 Router-ID 192.0.2.1
	0xfd, 0xe8, 0x00, 0x03, 0xaa, 0xbb, 0xcc, // TLV 65000 (unassigned), 3 octets
	0x04, 0x40, 0x00, 0x04, 0x00, 0x00, 0x00, 0x0a, // TLV 1088 Administrative Group, beside a Node NLRI
}

// rfc9552UnknownTypesPayload is a received UPDATE from AS 65001 carrying the Node NLRI above
// in an MP_REACH_NLRI for (AFI 16388, SAFI 71), next hop 192.0.2.1, and the BGP-LS Attribute
// above.
func rfc9552UnknownTypesPayload() []byte {
	mpReach := []byte{0x40, 0x04, 0x47, 0x04, 0xc0, 0x00, 0x02, 0x01, 0x00}
	mpReach = append(mpReach, rfc9552UnknownSubTLVNodeNLRI...)

	attrs := []byte{0x40, 0x01, 0x01, 0x00}                // ORIGIN igp
	aspValue := []byte{0x02, 0x01, 0x00, 0x00, 0x00, 0x00} // AS_SEQUENCE of one 4-octet AS
	binary.BigEndian.PutUint32(aspValue[2:], 65001)
	attrs = append(attrs, 0x40, 0x02, byte(len(aspValue)))
	attrs = append(attrs, aspValue...)
	attrs = append(attrs, 0x80, 0x0e, byte(len(mpReach)))
	attrs = append(attrs, mpReach...)
	attrs = append(attrs, 0x80, bgplsAttrCode, byte(len(rfc9552UnknownAttrValue)))
	attrs = append(attrs, rfc9552UnknownAttrValue...)
	return buildUpdatePayload(attrs, nil)
}

// rfc9552Destination builds an established peer of AS peerAS that negotiated BGP-LS, with
// local AS 65000 and local address 192.0.2.254.
func rfc9552Destination(t *testing.T, addr string, peerAS uint32) *Peer {
	t.Helper()
	settings := &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       netip.MustParseAddr(addr),
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        peerAS,
		RouterID:      0x0a000001,
		LocalAddress:  netip.MustParseAddr("192.0.2.254"),
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{lsFam: true}})
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.fwdFacts.Store(peer.buildForwardFacts())
	return peer
}

// rfc9552Forward forwards the payload, received from an external peer of AS 65001, through
// forwardUpdateCore to the destination and returns every path-attribute block the
// destination was asked to write.
func rfc9552Forward(t *testing.T, payload []byte, destination *Peer) [][]byte {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)
	update.SourcePeerIP = netip.MustParseAddr("192.0.2.1")

	delivered := make(chan [][]byte, 4)
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		delivered <- rfc9552ItemAttributes(t, items)
	}, fwdPoolConfig{chanSize: 4, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	key := fwdKey{peerAddr: destination.Settings().PeerKey()}
	pool.registerOutgoingPool(key, 4096)
	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           map[netip.AddrPort]*Peer{key.peerAddr: destination},
		fwdPool:         pool,
	}
	adapter := &reactorAPIAdapter{r: r}
	source := forwardSourceInfo{resolved: true, isIBGP: false, globalLocalAS: 65000}
	require.NoError(t, adapter.forwardUpdateCore(update, id, []*Peer{destination}, source))

	select {
	case blocks := <-delivered:
		return blocks
	case <-time.After(time.Second):
		return nil
	}
}

// rfc9552ItemAttributes copies the path attributes of every UPDATE in items, in whichever
// form the rail handed it.
func rfc9552ItemAttributes(t *testing.T, items []fwdItem) [][]byte {
	t.Helper()
	var blocks [][]byte
	for i := range items {
		for _, body := range items[i].rawBodies {
			u, err := message.UnpackUpdate(body)
			require.NoError(t, err)
			blocks = append(blocks, append([]byte(nil), u.PathAttributes...))
		}
		for _, u := range items[i].updates {
			blocks = append(blocks, append([]byte(nil), u.PathAttributes...))
		}
	}
	return blocks
}

// rfc9552PropagatedHalves reads, out of one forwarded attribute block, the Link-State NLRI
// that follows the MP_REACH next hop and the BGP-LS Attribute value, and the AS_PATH value.
func rfc9552PropagatedHalves(t *testing.T, attrs []byte) (nlriBytes, bgpls, asPath []byte) {
	t.Helper()
	_, _, mpReach, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	require.True(t, found, "the Link-State NLRI travels in MP_REACH_NLRI")
	require.GreaterOrEqual(t, len(mpReach), 4)
	require.Equal(t, []byte{0x40, 0x04, 0x47}, mpReach[:3], "MP_REACH_NLRI names AFI 16388, SAFI 71")
	start := 4 + int(mpReach[3]) + 1 // the next-hop field, then the Reserved octet
	require.GreaterOrEqual(t, len(mpReach), start)
	_, _, bgpls, found = attribute.AttrFind(attrs, attribute.AttributeCode(bgplsAttrCode))
	require.True(t, found, "the BGP-LS Attribute is propagated")
	_, _, asPath, found = attribute.AttrFind(attrs, attribute.AttrASPath)
	require.True(t, found)
	return mpReach[start:], bgpls, asPath
}

// TestRFC9552UnknownTypesPropagateOnAForwardedUpdate forwards a received BGP-LS UPDATE whose
// NLRI holds an unassigned Node Descriptor sub-TLV and whose BGP-LS Attribute holds an
// unassigned TLV and an unexpected one, toward an internal peer.
//
// VALIDATES: RFC 9552 Section 5.1 -- "Unknown and unsupported types MUST be preserved and
// propagated within both the NLRI and the BGP-LS Attribute." The UPDATE the peer is asked to
// write carries the NLRI and the attribute value byte-identical to what was received.
// PREVENTS: a propagator that drops, reorders or rewrites a TLV it cannot name.
//
// RFC requirement: RFC9552-5.1-3 positive -- forwarded to an internal peer, the Node NLRI with unassigned sub-TLV 700 and the BGP-LS Attribute with unassigned TLV 65000 and the unexpected TLV 1088 leave byte-identical to the received ones.
func TestRFC9552UnknownTypesPropagateOnAForwardedUpdate(t *testing.T) {
	blocks := rfc9552Forward(t, rfc9552UnknownTypesPayload(), rfc9552Destination(t, "192.0.2.10", 65000))
	require.Len(t, blocks, 1, "one UPDATE reaches the internal peer")

	nlriBytes, bgpls, _ := rfc9552PropagatedHalves(t, blocks[0])
	assert.Equal(t, rfc9552UnknownSubTLVNodeNLRI, nlriBytes, "the NLRI keeps sub-TLV 700")
	assert.Equal(t, rfc9552UnknownAttrValue, bgpls, "the attribute keeps TLVs 65000 and 1088")
}

// TestRFC9552UnknownTypesSurviveARewrittenUpdate forwards the same UPDATE toward an external
// peer, where Ze prepends its AS, so the attributes are re-encoded rather than relayed as
// received.
//
// VALIDATES: RFC 9552 Section 5.1 -- preservation holds where Ze rebuilds the attributes:
// the AS_PATH proves the rebuild (65000 prepended), and the NLRI and the BGP-LS Attribute
// still leave byte-identical.
// PREVENTS: unknown types surviving only the pass-through rail and being lost when an
// egress edit forces a re-encode.
//
// RFC requirement: RFC9552-5.1-3 negative -- forwarded to an external peer with AS 65000 prepended to the re-encoded AS_PATH, the NLRI and the BGP-LS Attribute are not stripped of sub-TLV 700, TLV 65000 or TLV 1088: both leave byte-identical.
func TestRFC9552UnknownTypesSurviveARewrittenUpdate(t *testing.T) {
	blocks := rfc9552Forward(t, rfc9552UnknownTypesPayload(), rfc9552Destination(t, "192.0.2.20", 65002))
	require.Len(t, blocks, 1, "one UPDATE reaches the external peer")

	nlriBytes, bgpls, asPath := rfc9552PropagatedHalves(t, blocks[0])
	assert.Equal(t, []byte{0x02, 0x02, 0x00, 0x00, 0xfd, 0xe8, 0x00, 0x00, 0xfd, 0xe9}, asPath,
		"the AS_PATH is re-encoded with 65000 prepended")
	assert.Equal(t, rfc9552UnknownSubTLVNodeNLRI, nlriBytes, "the NLRI keeps sub-TLV 700")
	assert.Equal(t, rfc9552UnknownAttrValue, bgpls, "the attribute keeps TLVs 65000 and 1088")
}
