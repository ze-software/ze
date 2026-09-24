package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgpfilter "github.com/ze-software/ze/internal/component/bgp/reactor/filter"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// RFC requirement: RFC7611-2.2-1 positive -- non-RD IPv4 receive publication strips ACCEPT_OWN, not other communities or the route.
// RFC requirement: RFC7611-2.2-1 negative -- VPN-IP publication retains ACCEPT_OWN because its RD identifies a source routing context.
func TestACCEPTOwnFamilyBoundary(t *testing.T) {
	for _, vpn := range []bool{false, true} {
		s := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020304))
		attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1, 0x40, 5, 4, 0, 0, 0, 100}
		attrs = append(attrs, 0xc0, byte(attribute.AttrCommunity), 12)
		attrs = binary.BigEndian.AppendUint32(attrs, uint32(attribute.CommunityAcceptOwn))
		attrs = binary.BigEndian.AppendUint32(attrs, 65001<<16|100)
		attrs = binary.BigEndian.AppendUint32(attrs, uint32(attribute.CommunityAcceptOwn))
		nlri := []byte{24, 10, 20, 0}
		if vpn {
			mp := []byte{0, 1, 128, 12, 0, 0, 0, 0, 0, 0, 0, 0, 192, 0, 2, 1, 0,
				112, 0, 16, 1, 0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 10, 20, 0}
			attrs = append(attrs, 0x80, byte(attribute.AttrMPReachNLRI), byte(len(mp)))
			attrs = append(attrs, mp...)
			nlri = nil
		}
		body := makeUpdateBody(nil, attrs, nlri)
		published, _, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
		require.NoError(t, err)
		require.NotNil(t, published)
		sections, err := wire.ParseUpdateSections(published.Payload())
		require.NoError(t, err)
		_, _, communities, present := attribute.AttrFind(sections.Attrs(published.Payload()), attribute.AttrCommunity)
		require.True(t, present)
		if vpn {
			require.Len(t, communities, 12)
			require.Equal(t, uint32(attribute.CommunityAcceptOwn), binary.BigEndian.Uint32(communities))
			_, _, actual, found := attribute.AttrFind(sections.Attrs(published.Payload()), attribute.AttrMPReachNLRI)
			require.True(t, found)
			require.Equal(t, []byte{10, 20, 0}, actual[len(actual)-3:])
		} else {
			require.Len(t, communities, 4)
			require.Equal(t, uint32(65001<<16|100), binary.BigEndian.Uint32(communities))
			require.Equal(t, nlri, sections.NLRI(published.Payload()))
		}
	}
}

// RFC requirement: RFC7611-2.3-2 positive -- the default ingress path does not exempt an own ORIGINATOR_ID merely because ACCEPT_OWN is present.
// RFC requirement: RFC7611-2.3-2 negative -- a route from another originator is not refused merely because the community is present.
func TestACCEPTOwnDoesNotEnableAcceptanceImplicitly(t *testing.T) {
	for _, own := range []bool{false, true} {
		id := uint32(0x01020305)
		if own {
			id = 0x01020304
		}
		attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, byte(attribute.AttrOriginatorID), 4}
		attrs = binary.BigEndian.AppendUint32(attrs, id)
		attrs = append(attrs, 0xc0, byte(attribute.AttrCommunity), 4)
		attrs = binary.BigEndian.AppendUint32(attrs, uint32(attribute.CommunityAcceptOwn))
		accepted, _ := bgpfilter.LoopIngress(filterapi.PeerFilterInfo{LocalAS: 65001, PeerAS: 65001, RouterID: 0x01020304}, makeUpdateBody(nil, attrs, []byte{24, 10, 20, 0}), nil)
		require.Equal(t, !own, accepted)
	}
}
