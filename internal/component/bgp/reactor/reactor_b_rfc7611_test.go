package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// acceptOwnMPCase is one MP_REACH_NLRI family offered to the ACCEPT_OWN
// receive rule: the attribute value and whether the family carries an RD.
type acceptOwnMPCase struct {
	name string
	mp   []byte
	rd   bool
}

// acceptOwnMPCases returns MP_REACH_NLRI values for two families without a
// Route Distinguisher (IPv6 unicast, IPv4 labeled unicast) and one with one
// (VPN-IPv6), each carrying one prefix.
func acceptOwnMPCases() []acceptOwnMPCase {
	return []acceptOwnMPCase{
		{name: "ipv6-unicast", mp: []byte{
			0, 2, 1, 16, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0,
			32, 0x20, 0x01, 0x0d, 0xb8}},
		{name: "ipv4-labeled-unicast", mp: []byte{
			0, 1, 4, 4, 192, 0, 2, 1, 0,
			48, 0, 0x01, 0x01, 10, 20, 0}},
		{name: "vpn-ipv6", rd: true, mp: []byte{
			0, 2, 128, 24, 0, 0, 0, 0, 0, 0, 0, 0,
			0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0,
			120, 0, 0x01, 0x01, 0, 1, 0xfd, 0xe9, 0, 0, 0, 1, 0x20, 0x01, 0x0d, 0xb8}},
	}
}

// acceptOwnPublish runs one UPDATE through the session's receive enforcement
// and returns the published attribute section.
func acceptOwnPublish(t *testing.T, mp []byte, communities []uint32) []byte {
	t.Helper()
	s := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020304))
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 5, 4, 0, 0, 0, 100}
	attrs = append(attrs, 0xc0, byte(attribute.AttrCommunity), byte(4*len(communities)))
	for _, community := range communities {
		attrs = binary.BigEndian.AppendUint32(attrs, community)
	}
	attrs = append(attrs, 0x80, byte(attribute.AttrMPReachNLRI), byte(len(mp)))
	attrs = append(attrs, mp...)
	published, _, err := s.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, attrs, nil), 0))
	require.NoError(t, err)
	require.NotNil(t, published)
	sections, err := wire.ParseUpdateSections(published.Payload())
	require.NoError(t, err)
	return sections.Attrs(published.Payload())
}

// Goal: prove the ACCEPT_OWN discard covers every family without a Route
// Distinguisher that arrives in MP_REACH_NLRI, not only IPv4 in the body NLRI,
// and that a route whose only community is ACCEPT_OWN loses the attribute.
// Method: publish IPv6 unicast, IPv4 labeled unicast and VPN-IPv6 routes
// through enforceRFC7606 and read the COMMUNITY attribute and the
// MP_REACH_NLRI that the session publishes.
//
// VALIDATES: RFC 7611 Section 2.2, ACCEPT_OWN discarded in a family with no RD.
// PREVENTS: a non-RD family in MP_REACH_NLRI keeping ACCEPT_OWN.
//
// RFC requirement: RFC7611-2.2-1 positive -- an IPv6 unicast or IPv4 labeled unicast route in MP_REACH_NLRI loses both ACCEPT_OWN copies and keeps 65001:100 and its MP_REACH_NLRI byte for byte; with ACCEPT_OWN as its only community, the COMMUNITY attribute is gone and the MP_REACH_NLRI is kept.
// RFC requirement: RFC7611-2.2-1 negative -- a VPN-IPv6 route in MP_REACH_NLRI, whose RD identifies the source VRF, keeps ACCEPT_OWN whether or not it is the only community.
func TestRFC7611AcceptOwnDiscardedForEveryNonRDFamilyInMPReach(t *testing.T) {
	own := uint32(attribute.CommunityAcceptOwn)
	for _, tc := range acceptOwnMPCases() {
		t.Run(tc.name, func(t *testing.T) {
			attrs := acceptOwnPublish(t, tc.mp, []uint32{own, 65001<<16 | 100, own})
			_, _, communities, present := attribute.AttrFind(attrs, attribute.AttrCommunity)
			require.True(t, present)
			_, _, mp, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
			require.True(t, found)
			require.Equal(t, tc.mp, mp)
			if tc.rd {
				require.Len(t, communities, 12)
				require.Equal(t, own, binary.BigEndian.Uint32(communities))
				require.Equal(t, own, binary.BigEndian.Uint32(communities[8:]))
			} else {
				require.Len(t, communities, 4)
				require.Equal(t, uint32(65001<<16|100), binary.BigEndian.Uint32(communities))
			}

			attrs = acceptOwnPublish(t, tc.mp, []uint32{own})
			_, _, communities, present = attribute.AttrFind(attrs, attribute.AttrCommunity)
			_, _, mp, found = attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
			require.True(t, found)
			require.Equal(t, tc.mp, mp)
			if tc.rd {
				require.True(t, present)
				require.Equal(t, []byte{0xff, 0xff, 0, 1}, communities)
			} else {
				require.False(t, present, "a COMMUNITY attribute holding only ACCEPT_OWN must be removed")
			}
		})
	}
}
