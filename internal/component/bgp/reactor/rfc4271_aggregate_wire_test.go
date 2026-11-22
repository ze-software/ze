// Design: docs/architecture/update-building.md -- wire BGP UPDATE encoding of synthesized aggregate routes
// Related: reactor_api_batch.go -- buildBatchAnnounceUpdate, planBatchAttrs (2-byte/4-byte AGGREGATOR, eBGP AS_PATH prepend, IPv6 MP_REACH_NLRI)
// RFC 4271 Sections 5.1.7 and 9.2.2.2; RFC 9774 Section 5.2 -- see rfc/short/rfc4271.md and rfc/short/rfc9774.md

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC4271AggregateWireEncodingFor2Byte4ByteEBGPAndIPv6 verifies that
// aggregate route attributes on BOTH the Builder rail (batch.Attrs) AND the
// live `update text` wire rail (batch.Wire = NewAttributesWire(b.Build(),
// APIContextID), matching ParseUpdateText -> DispatchNLRIGroups) encode
// conformant wire BGP UPDATE messages for:
//   - 4-byte eBGP peers (prepends local AS onto empty AS_PATH for local-AS brief
//     aggregation, strips LOCAL_PREF, writes 8-byte AGGREGATOR and 0-byte
//     ATOMIC_AGGREGATE, preserves ORIGIN and NEXT_HOP),
//   - 2-byte eBGP peers (transcodes AGGREGATOR to 6-byte 2-octet ASN + 4-octet
//     IP, and emits AS4_AGGREGATOR when local ASN > 65535 per RFC 6793),
//   - IPv6 unicast peers (encodes IPv6 NEXT_HOP and prefix inside MP_REACH_NLRI
//     with no IPv4 NEXT_HOP attribute).
//
// RFC requirement: RFC4271-5.1.7-1 positive -- both wire-backed (`update text` batch.Wire) and Builder-backed aggregate announces encode speaker-own AGGREGATOR in 8-octet 4-byte form toward 4-byte peers and 6-octet 2-byte form (+ AS4_AGGREGATOR when ASN > 65535) toward 2-byte peers.
func TestRFC4271AggregateWireEncodingFor2Byte4ByteEBGPAndIPv6(t *testing.T) {
	adapter := &reactorAPIAdapter{r: &Reactor{}}
	localIFv4 := netip.MustParseAddr("198.51.100.10")
	localIFv6 := netip.MustParseAddr("2001:db8:fe::10")
	routerID := netip.MustParseAddr("198.51.100.1")

	pfx4 := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("203.0.113.0/24"), 0, false)
	pfx6 := nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:100::/48"), 0, false)

	makeBatch := func(fam family.Family, n nlri.NLRI, b *attribute.Builder, wireBacked bool) bgptypes.NLRIBatch {
		if wireBacked {
			return bgptypes.NLRIBatch{
				Family: fam,
				NLRIs:  []nlri.NLRI{n},
				Wire:   attribute.NewAttributesWire(b.Build(), bgpctx.APIContextID),
			}
		}
		return bgptypes.NLRIBatch{
			Family: fam,
			NLRIs:  []nlri.NLRI{n},
			Attrs:  b,
		}
	}

	for _, tc := range []struct {
		name       string
		wireBacked bool
	}{
		{"wire-update-text", true},
		{"builder", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var attrBuf, nlriBuf [4096]byte

			builder := attribute.NewBuilder()
			builder.SetOrigin(uint8(attribute.OriginEGP))
			builder.SetMED(50)
			builder.SetLocalPref(100)
			builder.SetAtomicAggregate()
			builder.SetAggregator(65000, routerID)

			batch4 := makeBatch(family.IPv4Unicast, pfx4, builder, tc.wireBacked)

			// Case 1: 4-byte eBGP peer (local AS 65000 prepended as origin AS,
			// 8-byte AGGREGATOR, 0-byte ATOMIC_AGGREGATE, LOCAL_PREF stripped).
			factsEBGP4 := announceFacts{
				isIBGP:  false,
				asn4:    true,
				nextHop: localIFv4,
				prepend: localASPrepend{primary: 65000},
			}
			upd4, err := adapter.buildBatchAnnounceUpdate(attrBuf[:], nlriBuf[:], batch4, factsEBGP4)
			require.NoError(t, err)
			require.NotNil(t, upd4)

			_, _, origVal, found := attribute.AttrFind(upd4.PathAttributes, attribute.AttrOrigin)
			require.True(t, found)
			require.Equal(t, []byte{byte(attribute.OriginEGP)}, origVal)

			_, _, asVal, found := attribute.AttrFind(upd4.PathAttributes, attribute.AttrASPath)
			require.True(t, found)
			require.Equal(t, []byte{byte(attribute.ASSequence), 1, 0, 0, 0xfd, 0xe8}, asVal,
				"eBGP peer must receive local AS 65000 prepended as origin AS")

			_, _, nhVal, found := attribute.AttrFind(upd4.PathAttributes, attribute.AttrNextHop)
			require.True(t, found)
			require.Equal(t, localIFv4.AsSlice(), nhVal)

			_, _, _, hasLP := attribute.AttrFind(upd4.PathAttributes, attribute.AttrLocalPref)
			require.False(t, hasLP, "LOCAL_PREF must be stripped toward eBGP")

			_, _, atomicVal, found := attribute.AttrFind(upd4.PathAttributes, attribute.AttrAtomicAggregate)
			require.True(t, found)
			require.Empty(t, atomicVal)

			_, _, aggVal, found := attribute.AttrFind(upd4.PathAttributes, attribute.AttrAggregator)
			require.True(t, found)
			require.Len(t, aggVal, 8, "4-byte peer must receive 8-byte AGGREGATOR")
			require.Equal(t, uint32(65000), binary.BigEndian.Uint32(aggVal[0:4]))
			require.Equal(t, routerID.AsSlice(), aggVal[4:8])

			// Case 2: 2-byte eBGP peer with 16-bit ASN (65000) -> 6-byte AGGREGATOR, no AS4_AGGREGATOR.
			factsEBGP2 := announceFacts{
				isIBGP:  false,
				asn4:    false,
				nextHop: localIFv4,
				prepend: localASPrepend{primary: 65000},
			}
			upd2, err := adapter.buildBatchAnnounceUpdate(attrBuf[:], nlriBuf[:], batch4, factsEBGP2)
			require.NoError(t, err)
			_, _, aggVal2, found := attribute.AttrFind(upd2.PathAttributes, attribute.AttrAggregator)
			require.True(t, found)
			require.Len(t, aggVal2, 6, "2-byte peer must receive 6-byte AGGREGATOR")
			require.Equal(t, uint16(65000), binary.BigEndian.Uint16(aggVal2[0:2]))
			require.Equal(t, routerID.AsSlice(), aggVal2[2:6])
			_, _, _, hasAS4Agg := attribute.AttrFind(upd2.PathAttributes, attribute.AttrAS4Aggregator)
			require.False(t, hasAS4Agg)

			// Case 3: 2-byte eBGP peer with 32-bit speaker ASN (4200000000) ->
			// 6-byte AGGREGATOR with AS_TRANS (23456) + 8-byte AS4_AGGREGATOR (4200000000).
			builder4ByteAS := attribute.NewBuilder()
			builder4ByteAS.SetOrigin(uint8(attribute.OriginIGP))
			builder4ByteAS.SetAggregator(4200000000, routerID)
			batch4ByteAS := makeBatch(family.IPv4Unicast, pfx4, builder4ByteAS, tc.wireBacked)
			factsEBGP2High := announceFacts{
				isIBGP:  false,
				asn4:    false,
				nextHop: localIFv4,
				prepend: localASPrepend{primary: 4200000000},
			}
			upd2High, err := adapter.buildBatchAnnounceUpdate(attrBuf[:], nlriBuf[:], batch4ByteAS, factsEBGP2High)
			require.NoError(t, err)
			_, _, aggValTrans, found := attribute.AttrFind(upd2High.PathAttributes, attribute.AttrAggregator)
			require.True(t, found)
			require.Len(t, aggValTrans, 6)
			require.Equal(t, uint16(23456), binary.BigEndian.Uint16(aggValTrans[0:2]))
			_, _, as4AggVal, found := attribute.AttrFind(upd2High.PathAttributes, attribute.AttrAS4Aggregator)
			require.True(t, found)
			require.Len(t, as4AggVal, 8)
			require.Equal(t, uint32(4200000000), binary.BigEndian.Uint32(as4AggVal[0:4]))
			require.Equal(t, routerID.AsSlice(), as4AggVal[4:8])

			// Case 4: IPv6 unicast aggregate encodes IPv6 next-hop in MP_REACH_NLRI with no AttrNextHop.
			batch6 := makeBatch(family.IPv6Unicast, pfx6, builder, tc.wireBacked)
			factsV6 := announceFacts{
				isIBGP:  true,
				asn4:    true,
				nextHop: localIFv6,
			}
			upd6, err := adapter.buildBatchAnnounceUpdate(attrBuf[:], nlriBuf[:], batch6, factsV6)
			require.NoError(t, err)
			_, _, _, hasV4NH := attribute.AttrFind(upd6.PathAttributes, attribute.AttrNextHop)
			require.False(t, hasV4NH, "IPv6 aggregate must not carry IPv4 NEXT_HOP attribute")
			_, _, mpReach, found := attribute.AttrFind(upd6.PathAttributes, attribute.AttrMPReachNLRI)
			require.True(t, found, "IPv6 aggregate must carry MP_REACH_NLRI")
			require.GreaterOrEqual(t, len(mpReach), 20)
			require.Equal(t, byte(16), mpReach[3], "IPv6 MP_REACH_NLRI next-hop length must be 16")
			require.Equal(t, localIFv6.AsSlice(), mpReach[4:20])
		})
	}
}
