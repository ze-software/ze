// Design: docs/architecture/update-building.md -- NEXT_HOP beside MP_REACH_NLRI
// Overview: update_build_labeled.go, update_build_evpn.go -- the MP builders

package message

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestMPBuildersWriteNoNextHop checks every multiprotocol builder that once added
// the RFC 4271 NEXT_HOP attribute for an IPv4 next hop.
//
// VALIDATES: RFC 4760 Section 3, "An UPDATE message that carries no NLRI, other
// than the one encoded in the MP_REACH_NLRI attribute, SHOULD NOT carry the
// NEXT_HOP attribute." Method: build a labeled-unicast and an EVPN route with an
// IPv4 next hop and look for attribute code 3 in the path attributes.
// PREVENTS: an MP UPDATE restating its next hop as a NEXT_HOP the RFC says not
// to send, which ExaBGP 6.0 does not send either.
func TestMPBuildersWriteNoNextHop(t *testing.T) {
	nextHop := netip.MustParseAddr("192.0.2.1")

	t.Run("labeled unicast", func(t *testing.T) {
		ub := NewUpdateBuilder(65001, true, true, false)
		update := ub.BuildLabeledUnicast(&LabeledUnicastParams{
			Prefix:  netip.MustParsePrefix("10.0.0.0/8"),
			NextHop: nextHop,
			Labels:  []uint32{100},
			Origin:  attribute.OriginIGP,
		})
		require.NotNil(t, update)
		_, _, _, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
		require.False(t, found, "labeled unicast carries NEXT_HOP beside MP_REACH_NLRI")
		_, _, _, found = attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
		require.True(t, found, "labeled unicast carries its next hop in MP_REACH_NLRI")
	})

	t.Run("evpn", func(t *testing.T) {
		ub := NewUpdateBuilder(65001, true, true, false)
		update, err := ub.BuildEVPN(EVPNParams{
			NLRI:              testEVPNType3Bytes(makeRD(1), nextHop),
			NextHop:           nextHop,
			Origin:            attribute.OriginIGP,
			ExtCommunityBytes: []byte{0, 2, 0xfd, 0xe8, 0, 0, 0, 1},
		})
		require.NoError(t, err)
		_, _, _, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
		require.False(t, found, "EVPN carries NEXT_HOP beside MP_REACH_NLRI")
		_, _, _, found = attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
		require.True(t, found, "EVPN carries its next hop in MP_REACH_NLRI")
	})
}
