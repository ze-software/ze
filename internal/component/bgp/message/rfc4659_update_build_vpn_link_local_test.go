// Design: docs/architecture/update-building.md -- VPN UPDATE builders.
// Related: update_build_vpn.go -- buildMPReachVPN, the encoder under test.
package message

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestBuildVPNIPv6NextHopLinkLocalPair checks the two RFC 4659 Section 3.2.1.1
// Next Hop field forms of a VPN-IPv6 UPDATE.
//
// VALIDATES: with LinkLocalNextHop set to an IPv6 link-local unicast address,
// BuildVPN writes a 48-octet field (zero RD, global, zero RD, link-local); with
// it unset, or set to an address that is not link-local unicast, the field is
// the 24-octet zero RD plus global.
// PREVENTS: the link-local address dropped by the VPN builder, a non-zero second
// RD, or a non-link-local address written in the link-local slot.
//
// Method: build the UPDATE, extract the MP_REACH_NLRI value through its own
// attribute header, and compare the whole value as hex.
func TestBuildVPNIPv6NextHopLinkLocalPair(t *testing.T) {
	const (
		head   = "0002" + "80"                                            // AFI 2, SAFI 128
		rdZero = "0000000000000000"                                       // RFC 4659: "8-octet RD is set to zero"
		global = "20010db8000100000000000000000001"                       // 2001:db8:1::1
		ll     = "fe800000000000000000000000000001"                       // fe80::1
		nlri   = "00" + "78" + "000641" + "0001000000640064" + "20010db8" // Reserved, then the 120-bit VPN NLRI
	)
	tests := []struct {
		name      string
		linkLocal netip.Addr
		want      string
	}{
		{name: "shared subnet: global and link-local", linkLocal: netip.MustParseAddr("fe80::1"), want: head + "30" + rdZero + global + rdZero + ll + nlri},
		{name: "not shared: global alone", want: head + "18" + rdZero + global + nlri},
		{name: "global in the link-local slot is not written", linkLocal: netip.MustParseAddr("2001:db8:1::9"), want: head + "18" + rdZero + global + nlri},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ub := NewUpdateBuilder(65001, false, true, false)
			params := VPNParams{
				Prefix:           netip.MustParsePrefix("2001:db8::/32"),
				NextHop:          netip.MustParseAddr("2001:db8:1::1"),
				LinkLocalNextHop: tt.linkLocal,
				Origin:           attribute.OriginIGP,
				Labels:           []uint32{100},
				RDBytes:          [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
			}
			update := ub.BuildVPN(&params)
			got := hex.EncodeToString(extractMPReachValue(t, update.PathAttributes))
			if got != tt.want {
				t.Fatalf("MP_REACH_NLRI value\n got  %s\n want %s", got, tt.want)
			}
		})
	}
}
