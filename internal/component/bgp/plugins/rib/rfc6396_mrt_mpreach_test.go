// Design: docs/architecture/mrt.md -- RIB snapshot MP_REACH_NLRI abbreviation
// RFC: rfc/short/rfc6396.md -- Section 4.3.4, abbreviated MP_REACH_NLRI in RIB entries
// Related: rib_mrt.go -- reconstructWireAttrs, appendOtherAttrsWireForMRT

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

// TestRFC6396MRTAbbreviatesMPReachNLRI proves that dumpRIBForMRT abbreviates
// MP_REACH_NLRI (code 14) inside TABLE_DUMP_V2 RIB entries to only Next Hop
// Address Length + Next Hop Address (omitting AFI, SAFI, Reserved, and NLRI),
// while preserving other optional attributes intact.
//
// RFC 6396 Section 4.3.4: "There is one exception to the encoding of BGP
// attributes for the BGP MP_REACH_NLRI attribute (BGP Type Code 14) [RFC4760].
// Since the AFI, SAFI, and NLRI information is already encoded in the RIB
// Entry Header or RIB_GENERIC Entry Header, only the Next Hop Address Length
// and Next Hop Address fields are included.  The Reserved field is omitted."
//
// RFC requirement: RFC6396-4.3.4-2 positive -- a dumped IPv6 unicast route's
// MP_REACH_NLRI attribute carries only [NextHopLen=16, 2001:db8::1] (17 bytes,
// omitting AFI/SAFI/Reserved/NLRI) and decodes via ParseMPReachRIBEntry.
// RFC requirement: RFC6396-4.3.4-2 negative -- non-MP_REACH_NLRI attributes in
// OtherAttrs (Tunnel Encapsulation, code 23) pass through dumpRIBForMRT
// unabbreviated.
func TestRFC6396MRTAbbreviatesMPReachNLRI(t *testing.T) {
	t.Parallel()

	r := newTestRIBManager(t)
	peerAddr := netip.MustParseAddr("2001:db8::1")
	peerRIB := storage.NewPeerRIB(peerAddr.String())
	r.bgpPeers[30] = peerRIB
	r.peerMeta[30] = &peerMetadata{Address: peerAddr, PeerASN: 65030, RemoteRouterID: 0xc000021e}
	t.Cleanup(peerRIB.Release)

	// Full on-wire MP_REACH_NLRI (code 14): AFI=2, SAFI=1, nhLen=16,
	// nextHop=2001:db8::1, reserved=0, NLRI=2001:db8:1::/48.
	mpReachVal := []byte{
		0x00, 0x02, 0x01, 0x10,
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
		0x00,
		0x30, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01,
	}
	tunnelVal := []byte{0x00, 0x02, 0x00, 0x08, 6, 6, 0, 0, 0, 0, 0, 0}

	rawAttrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xfe}
	rawAttrs = append(rawAttrs, 0x90, 14, 0, byte(len(mpReachVal))) // extended length on wire
	rawAttrs = append(rawAttrs, mpReachVal...)
	rawAttrs = append(rawAttrs, 0xc0, 23, byte(len(tunnelVal)))
	rawAttrs = append(rawAttrs, tunnelVal...)

	peerRIB.Insert(family.IPv6Unicast, rawAttrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	var dumpedAttrs []byte
	r.dumpRIBForMRT(registry.RIBDumpVisitor{
		OnPeer: func(string, uint32, [4]byte, bool) uint16 { return 0 },
		OnRoute: func(_, afi, safi uint16, prefixLen uint8, prefix, attrs []byte) {
			require.Equal(t, uint16(2), afi)
			require.Equal(t, uint16(1), safi)
			require.Equal(t, uint8(48), prefixLen)
			require.Equal(t, []byte{0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}, prefix)
			dumpedAttrs = append([]byte(nil), attrs...)
		},
	})
	require.NotEmpty(t, dumpedAttrs)

	found := map[attribute.AttributeCode][]byte{}
	iter := attribute.NewAttrIterator(dumpedAttrs)
	for code, _, val, ok := iter.Next(); ok; code, _, val, ok = iter.Next() {
		found[code] = append([]byte(nil), val...)
	}

	wantAbbrev := []byte{
		0x10,
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
	}
	require.Equal(t, wantAbbrev, found[attribute.AttrMPReachNLRI],
		"TABLE_DUMP_V2 MP_REACH_NLRI must carry only Next Hop Length + Next Hop")
	nh, err := mrtfmt.ParseMPReachRIBEntry(found[attribute.AttrMPReachNLRI])
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr("2001:db8::1"), nh)

	require.Equal(t, tunnelVal, found[attribute.AttrTunnelEncap],
		"non-MP_REACH_NLRI OtherAttrs must remain unabbreviated")
}
