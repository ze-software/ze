// Design: docs/architecture/api/update-syntax.md -- the update text entry point for VPN routes
// RFC: rfc/short/rfc4659.md -- RFC4659-3.2-1, an advertised IPv6 VPN route carries its label
// Related: update_text_nlri.go -- parseVPNNLRI, the label check under test

package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// TestRFC4659VPNv6AnnouncementCarriesItsLabel announces one IPv6 VPN route
// through the update text entry point and reads the NLRI octets it produced.
//
// RFC 4659 Section 3.2: "When distributing IPv6 VPN routes, the advertising PE
// router MUST assign and distribute MPLS labels with the IPv6 VPN routes."
//
// RFC requirement: RFC4659-3.2-1 positive -- an ipv6/mpls-vpn announcement given label 1000
// produces the NLRI 88 003E81 0000FDE800000064 20010DB80001: length 136 bits, the label
// 1000 with the bottom-of-stack bit, the RD 65000:100, then 2001:db8:1::/48.
func TestRFC4659VPNv6AnnouncementCarriesItsLabel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "2001:db8::1",
		"rd", "65000:100",
		"label", "1000",
		"nlri", "ipv6/mpls-vpn", "add", "2001:db8:1::/48",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	wire, ok := result.Groups[0].Announce[0].(*nlri.WireNLRI)
	require.True(t, ok, "expected WireNLRI, got %T", result.Groups[0].Announce[0])
	assert.Equal(t, []byte{
		0x88,
		0x00, 0x3E, 0x81,
		0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64,
		0x20, 0x01, 0x0D, 0xB8, 0x00, 0x01,
	}, wire.Bytes(), "the advertised IPv6 VPN NLRI carries label 1000 ahead of the RD")
}

// TestRFC4659VPNv6AnnouncementWithoutLabelRefused announces the same IPv6 VPN
// route with no label, which would advertise it with none.
//
// RFC requirement: RFC4659-3.2-1 negative -- an ipv6/mpls-vpn announcement carrying an RD
// and no label is refused with route.ErrMissingLabel, so no labelless IPv6 VPN NLRI is
// produced.
func TestRFC4659VPNv6AnnouncementWithoutLabelRefused(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "2001:db8::1",
		"rd", "65000:100",
		"nlri", "ipv6/mpls-vpn", "add", "2001:db8:1::/48",
	})
	require.ErrorIs(t, err, route.ErrMissingLabel)
	assert.Nil(t, result, "a refused announcement produces nothing to advertise")
}
