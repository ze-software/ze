// Design: docs/architecture/core-design.md — egress attribute modification on the forward rails
// RFC: rfc/short/rfc8669.md — BGP Prefix-SID, Section 3 and Section 5
// Related: peer_forward_facts.go — applyFactsNextHop, which records the next-hop-change edit
// Related: forward_prefix_sid.go — prefixSIDNextHopHandler, which plans the rewritten attribute
//
// RFC 8669 Section 3: "For future extensibility, unknown TLVs MUST be ignored and
// propagated unmodified." Section 5: "A BGP speaker that advertises a path received from
// one of its neighbors SHOULD advertise the BGP Prefix-SID received with the path without
// modification as long as the BGP Prefix-SID was acceptable."
//
// RFC 9252 Section 2 speaks only of the SRv6 Service TLVs (types 5 and 6): "If the BGP
// next hop is changed, the TLVs, Sub-TLVs, and Sub-Sub-TLVs SHOULD be updated with the
// locally allocated SRv6 SID information. Any received Sub-TLVs and Sub-Sub-TLVs that are
// unrecognized MUST be removed." It gives no license to drop a Label-Index TLV, an
// Originator SRGB TLV, or an unknown top-level TLV. Ze used to drop the whole attribute on
// every next-hop change; these tests pin the per-TLV rewrite that replaced it.

package reactor

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// The TLVs of the relayed Prefix-SID. The Label-Index is RFC 8669 Section 3.1 (index
// 100), the Originator SRGB is Section 3.2 (one range, base 16000, size 1000), and type
// 200 is unallocated, so every speaker must treat it as unknown.
var (
	rfc8669LabelIndexTLV = []byte{0x01, 0x00, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64}
	rfc8669UnknownTLV    = []byte{0xC8, 0x00, 0x03, 0xAA, 0xBB, 0xCC}
	rfc8669SRGBTLV       = []byte{0x03, 0x00, 0x08, 0x00, 0x00, 0x00, 0x3E, 0x80, 0x00, 0x03, 0xE8}
)

// rfc8669ServiceTLV builds an SRv6 Service TLV of the given type (5 is L3, 6 is L2,
// RFC 9252 Section 2): RESERVED, then one SRv6 SID Information Sub-TLV (Section 3.1)
// carrying SID 2001:db8::1 and endpoint behavior End.DT4 (0x0013).
func rfc8669ServiceTLV(tlvType byte) []byte {
	tlv := []byte{tlvType, 0x00, 25, 0x00, 0x01, 0x00, 21, 0x00}
	sid := netip.MustParseAddr("2001:db8::1").As16()
	tlv = append(tlv, sid[:]...)
	return append(tlv, 0x00, 0x00, 0x13, 0x00)
}

// rfc8669RelayBody is an UPDATE body from an external source: ORIGIN igp, AS_PATH
// [65001], NEXT_HOP 192.0.2.99, the given PREFIX_SID value, NLRI 198.51.100.0/24.
func rfc8669RelayBody(sid []byte) []byte {
	attrs := []byte{
		0x40, 1, 1, 0, // ORIGIN igp
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH AS_SEQUENCE [65001]
		0x40, 3, 4, 192, 0, 2, 99, // NEXT_HOP 192.0.2.99
		0xC0, prefixSIDCodeByte, byte(len(sid)),
	}
	attrs = append(attrs, sid...)
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	body = append(body, attrs...)
	return append(body, 24, 198, 51, 100)
}

// rfc8669Relay relays one UPDATE carrying sid to an external destination the operator
// placed inside the SR domain (so RFC 8669 Section 8 keeps the attribute), with the given
// next-hop mode, over the general rail or the route-server rail. It returns the one raw
// UPDATE body the destination was asked to write.
func rfc8669Relay(t *testing.T, routeServer bool, mode uint8, sid []byte) []byte {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	dest := wkPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	dest.Settings().NextHopMode = mode
	dest.Settings().PropagateSRv6PrefixSID = true
	dest.refreshForwardFacts()

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, rfc8669RelayBody(sid), ctxID)

	delivered := make(chan []fwdItem, 4)
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		delivered <- items
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	key := fwdKey{peerAddr: dest.Settings().PeerKey()}
	pool.registerOutgoingPool(key, 4096)
	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           map[netip.AddrPort]*Peer{key.peerAddr: dest},
		fwdPool:         pool,
	}
	if routeServer {
		source := makeRSPeer(t, "203.0.113.9", 65009, ctx, ctxID)
		r.peers[source.Settings().PeerKey()] = source
		r.rsForwardingEnabled = true
		reactorForwardRS(r, update, id, source.Settings().Address, source)
	} else {
		adapter := &reactorAPIAdapter{r: r}
		require.NoError(t, adapter.forwardUpdateCore(update, id, []*Peer{dest}, forwardSourceInfo{resolved: true}))
	}

	select {
	case items := <-delivered:
		var bodies [][]byte
		for i := range items {
			require.Empty(t, items[i].updates, "the destination shares the source's context, so it is sent raw wire")
			bodies = append(bodies, items[i].rawBodies...)
		}
		require.Len(t, bodies, 1)
		return bodies[0]
	case <-time.After(2 * time.Second):
		t.Fatal("the destination was sent nothing, so no claim about its wire can be made")
		return nil
	}
}

// TestRFC8669UnknownTLVPropagatedAcrossANextHopChange relays a Prefix-SID carrying a
// Label-Index TLV, an L3 Service TLV, an unknown TLV (type 200), an L2 Service TLV and an
// Originator SRGB TLV, over both relay rails, and reads the attribute on the wire.
//
// VALIDATES: with the next hop unchanged the attribute arrives byte-identical; with
// next-hop-self only the SRv6 Service TLVs leave, and every other TLV arrives
// byte-identical and in its received order.
// PREVENTS: the whole attribute being dropped on every next-hop change.
//
// RFC requirement: RFC8669-3-1 positive -- relayed with the next hop unchanged, on the general and the route-server rail, the Prefix-SID carrying the unknown TLV type 200 arrives byte-identical.
// RFC requirement: RFC8669-3-1 negative -- relayed with next-hop-self, the next-hop change that used to remove the whole attribute, the unknown TLV type 200 still arrives byte-identical and in its received position between the Label-Index and Originator SRGB TLVs, on both rails; only the SRv6 Service TLVs (types 5 and 6) are gone.
func TestRFC8669UnknownTLVPropagatedAcrossANextHopChange(t *testing.T) {
	received := slices.Concat(rfc8669LabelIndexTLV, rfc8669ServiceTLV(5), rfc8669UnknownTLV,
		rfc8669ServiceTLV(6), rfc8669SRGBTLV)
	kept := slices.Concat(rfc8669LabelIndexTLV, rfc8669UnknownTLV, rfc8669SRGBTLV)

	for _, rail := range []struct {
		name        string
		routeServer bool
	}{{name: "general rail"}, {name: "route-server rail", routeServer: true}} {
		t.Run(rail.name+"/next hop unchanged", func(t *testing.T) {
			attrs := decodeBodyAttrs(t, rfc8669Relay(t, rail.routeServer, NextHopUnchanged, received))
			assert.Equal(t, []byte{192, 0, 2, 99}, attrs[3], "the next hop is the source's")
			assert.Equal(t, received, attrs[prefixSIDCodeByte], "RFC 8669 Section 5: the Prefix-SID is advertised without modification")
		})
		t.Run(rail.name+"/next-hop-self", func(t *testing.T) {
			attrs := decodeBodyAttrs(t, rfc8669Relay(t, rail.routeServer, NextHopSelf, received))
			assert.Equal(t, []byte{10, 0, 0, 254}, attrs[3], "the next hop really changed")
			assert.Equal(t, kept, attrs[prefixSIDCodeByte],
				"RFC 8669 Section 3: the unknown TLV is propagated unmodified; only the SRv6 Service TLVs leave")
		})
	}
}

// TestRFC9252ServiceOnlyPrefixSIDDroppedOnANextHopChange relays, over both relay rails, a
// Prefix-SID whose only TLVs are SRv6 Service TLVs with next-hop-self. The L3 Service TLV
// carries an unrecognized Sub-TLV (0xF0) and an unrecognized Sub-Sub-TLV (0xEE)
// (rfc9252ReservedServiceTLV).
//
// VALIDATES: when removing the Service TLVs leaves no TLV, the attribute is not sent at
// all (an empty Prefix-SID is not a valid attribute), so none of their Sub-TLVs or
// Sub-Sub-TLVs, recognized or not, reaches the wire, and the route itself still arrives.
// PREVENTS: an empty attribute 40 on the wire, the route being withheld, or a handler
// that ignores the removal and relays the Service TLVs after the next hop changed.
//
// RFC requirement: RFC9252-3.3-2 positive -- relayed with next-hop-self on the general and the route-server rail, the Service TLVs, with their unrecognized Sub-TLV and Sub-Sub-TLV, are gone from the wire (attribute 40 absent, since Ze allocates no local SRv6 SID to rebuild them with), while the route arrives with the new next hop.
func TestRFC9252ServiceOnlyPrefixSIDDroppedOnANextHopChange(t *testing.T) {
	serviceOnly := slices.Concat(rfc9252ReservedServiceTLV(5), rfc8669ServiceTLV(6))
	for _, routeServer := range []bool{false, true} {
		body := rfc8669Relay(t, routeServer, NextHopSelf, serviceOnly)
		attrs := decodeBodyAttrs(t, body)
		_, present := attrs[prefixSIDCodeByte]
		assert.False(t, present, "no TLV remains, so the attribute leaves (route server %v)", routeServer)
		assert.Equal(t, []byte{10, 0, 0, 254}, attrs[3], "the route still arrives, with its new next hop")
		assert.Equal(t, []byte{24, 198, 51, 100}, body[len(body)-4:], "the NLRI still arrives")
	}
}

// TestPrefixSIDUnknownTLVSurvivesANextHopChange reads the operation the next-hop-self
// egress decision records for attribute 40.
//
// VALIDATES: a next-hop change records the removal of the SRv6 Service TLV types 5 and 6
// and never a suppression of the whole attribute.
// PREVENTS: applyFactsNextHop recording AttrModSuppress on code 40, which drops every TLV
// (Label-Index, Originator SRGB, unknown types) whenever the next hop changes.
func TestPrefixSIDUnknownTLVSurvivesANextHopChange(t *testing.T) {
	var facts peerForwardFacts
	precomputeNextHop(&PeerSettings{
		NextHopMode:  NextHopSelf,
		LocalAddress: netip.MustParseAddr("192.0.2.1"),
	}, &facts)
	var mods filterapi.ModAccumulator
	applyFactsNextHop(&facts, &mods, family.IPv4Unicast)

	var removals int
	for _, op := range mods.Ops() {
		if op.Code != uint8(attribute.AttrPrefixSID) {
			continue
		}
		require.NotEqual(t, filterapi.AttrModSuppress, op.Action,
			"RFC 8669 Section 3: unknown TLVs MUST be propagated unmodified; a next-hop change removes the whole attribute")
		require.Equal(t, filterapi.AttrModRemove, op.Action)
		assert.Equal(t, []byte{5, 6}, op.Buf, "only the SRv6 Service TLV types are removed")
		removals++
	}
	assert.Equal(t, 1, removals)
}
