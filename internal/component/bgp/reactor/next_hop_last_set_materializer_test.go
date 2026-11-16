// Design: docs/architecture/bgp/structural-forwarding.md -- final next-hop fields and identity
// Related: filter_delta_handlers.go -- the last selected MP Set may retain received bytes
// Related: forward_next_hop.go -- effectiveNextHop and the destination withdrawal gate
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// The gate must inspect the field the real materializer emits. An invalid last
// nonempty Set makes that writer keep the received field, not an earlier valid
// Set. Empty Sets remain ignored for forwarding families. Known and unknown
// length declarations, useful replacements and an unused code3 are controls.
func TestEffectiveNextHopLastSetMatchesMaterializer(t *testing.T) {
	peer6 := netip.MustParseAddr("2001:db8::2")
	good6 := netip.MustParseAddr("2001:db8::9").AsSlice()
	other6 := netip.MustParseAddr("2001:db8::7").AsSlice()
	peer4 := netip.MustParseAddr("192.0.2.2")
	good4 := netip.MustParseAddr("192.0.2.9").AsSlice()
	other4 := netip.MustParseAddr("192.0.2.7").AsSlice()
	unknown := family.Family{AFI: family.AFIIPv4, SAFI: 250}
	bad := []byte{0xfe}
	offTable := append(make([]byte, 8), other4...)
	unknownBad := append(bytes.Clone(good4), other4...)
	for _, tc := range []struct {
		name              string
		fam               family.Family
		peer              netip.Addr
		received, want    []byte
		sets              [][]byte
		own, unusedLegacy bool
	}{
		{name: "known-valid-then-invalid", fam: family.IPv6Unicast, peer: peer6, received: peer6.AsSlice(), sets: [][]byte{good6, bad}, want: peer6.AsSlice(), own: true},
		{name: "known-invalid-only", fam: family.IPv6Unicast, peer: peer6, received: peer6.AsSlice(), sets: [][]byte{bad}, want: peer6.AsSlice(), own: true},
		{name: "known-invalid-then-valid", fam: family.IPv6Unicast, peer: peer6, received: peer6.AsSlice(), sets: [][]byte{bad, good6}, want: good6},
		{name: "known-last-valid-peer-own", fam: family.IPv6Unicast, peer: peer6, received: peer6.AsSlice(), sets: [][]byte{good6, peer6.AsSlice()}, want: peer6.AsSlice(), own: true},
		{name: "known-last-valid-useful", fam: family.IPv6Unicast, peer: peer6, received: peer6.AsSlice(), sets: [][]byte{peer6.AsSlice(), good6}, want: good6},
		{name: "known-empty-last-ignored", fam: family.IPv6Unicast, peer: peer6, received: peer6.AsSlice(), sets: [][]byte{good6, nil}, want: good6},
		{name: "known-useful-received-retained", fam: family.IPv6Unicast, peer: peer6, received: good6, sets: [][]byte{other6, bad}, want: good6},
		{name: "known-decodable-off-table-last", fam: family.IPv6Unicast, peer: peer6, received: peer6.AsSlice(), sets: [][]byte{good6, offTable}, want: peer6.AsSlice(), own: true},
		{name: "known-useful-off-table-last", fam: family.IPv6Unicast, peer: peer6, received: good6, sets: [][]byte{other6, offTable}, want: good6},
		{name: "unused-legacy-sibling", fam: family.IPv6Unicast, peer: peer4, received: good6, sets: [][]byte{other6, bad}, want: good6, unusedLegacy: true},
		{name: "unknown-valid-then-invalid", fam: unknown, peer: peer4, received: peer4.AsSlice(), sets: [][]byte{good4, unknownBad}, want: peer4.AsSlice(), own: true},
		{name: "unknown-invalid-then-valid", fam: unknown, peer: peer4, received: peer4.AsSlice(), sets: [][]byte{unknownBad, good4}, want: good4},
		{name: "unknown-useful-received-retained", fam: unknown, peer: peer4, received: good4, sets: [][]byte{other4, unknownBad}, want: good4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := lastSetNextHopBody(tc.fam, tc.received, tc.unusedLegacy, tc.peer)
			original := bytes.Clone(body)
			base := payloadNextHop(body)
			facts := &peerForwardFacts{addr: tc.peer}
			dest := NewPeer(&PeerSettings{Address: tc.peer})
			dest.llScope.Store(newLinkScopeWith(nil, neighborView{}, tc.peer))
			var mods filterapi.ModAccumulator
			if tc.unusedLegacy {
				mods.Op(uint8(attribute.AttrNextHop), filterapi.AttrModSet, tc.peer.AsSlice())
			}
			for _, set := range tc.sets {
				mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, set)
			}
			var retained [][]byte
			for _, op := range mods.Ops() {
				if op.Code == uint8(attribute.AttrMPReachNLRI) {
					require.Equal(t, filterapi.AttrModSet, op.Action)
					retained = append(retained, op.Buf)
				}
			}
			require.Equal(t, tc.sets, retained, "Op must retain both Sets and their order")
			applyEgressNextHopScope(dest, facts, &mods, body, tc.fam)
			output, _, failure := buildModifiedPayload(body, &mods, attrModHandlersWithDefaults(), nil, nil)
			require.Equal(t, modifyFailureNone, failure)
			require.Equal(t, original, body, "the received body remains borrowed")
			if output == nil {
				// modifyFailureNone with no replacement means the writer uses its source.
				output = body
			}
			update, err := message.UnpackUpdate(output)
			require.NoError(t, err)
			require.Empty(t, update.NLRI)
			require.Empty(t, update.WithdrawnRoutes)
			_, _, unreach, withdrawn := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
			require.False(t, withdrawn, "materialization itself does not invent a withdrawal: %x", unreach)
			_, _, reach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
			require.True(t, found)
			wantReach := lastSetNextHopReach(tc.fam, tc.want)
			require.Equal(t, wantReach, reach, "literal selected field, reserved byte and exact NLRI")
			actual := aigpNextHop(output)
			require.Equal(t, tc.own, actual.has(tc.peer), "the observed active field determines peer identity")
			wantGate := withholdNone
			if tc.own {
				wantGate = withholdPeerOwn
			}
			require.Equal(t, wantGate, egressNextHopWithheld(dest, facts, &mods, base, false, netip.Addr{}),
				"the final gate must judge the bytes materialization selected")
			emitted := effectiveNextHop(&mods, base)
			require.Equal(t, actual.mp, emitted.mp)
			require.Equal(t, actual.mpLL, emitted.mpLL)
			require.Equal(t, actual.mpWireLen, emitted.mpWireLen)
			require.Equal(t, actual.mpIPv6, emitted.mpIPv6)
		})
	}
}

// These literal frames keep the materializer oracle independent of its profile
// decoder. The unknown family is a byte-preservation probe, not an NLRI codec.
func lastSetNextHopReach(fam family.Family, hop []byte) []byte {
	value := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI), byte(len(hop))}
	value = append(value, hop...)
	value = append(value, 0)
	return append(value, mixedV6Prefix...)
}

func lastSetNextHopBody(fam family.Family, hop []byte, unusedLegacy bool, peer netip.Addr) []byte {
	attrs := mixedAttrs(mixedAttr(byte(attribute.AttrMPReachNLRI), lastSetNextHopReach(fam, hop)))
	if unusedLegacy {
		attrs = append(attrs, 0x40, 3, 4)
		attrs = append(attrs, peer.AsSlice()...)
	}
	return buildUpdatePayload(attrs, nil)
}
