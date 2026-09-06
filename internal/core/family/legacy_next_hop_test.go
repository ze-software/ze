// Design: docs/architecture/update-building.md -- which families restate an IPv4
// next hop as the RFC 4271 Section 5.1.3 NEXT_HOP attribute
// Overview: family.go -- Family.LegacyNextHop

package family

import "testing"

// TestLegacyNextHop pins the answer for every family Ze sends, because the answer
// is what makes the API rail and the config rail write the same bytes for one
// route (internal/component/bgp/reactor.legacyNextHopApplies, and
// message.(*UpdateBuilder).BuildVPN with the `nlri/mvpn` and `nlri/mup` config
// parsers on the other side).
//
// VALIDATES: RFC 4760 Section 3 makes the attribute a SHOULD NOT beside
// MP_REACH_NLRI, so the set is a compatibility decision the ported ExaBGP
// fixtures record rather than a conformance one.
// PREVENTS: a family drifting between the two rails, which put NEXT_HOP on a
// config-originated MCAST-VPN route and not on an API-announced one until
// 2026-09-06.
func TestLegacyNextHop(t *testing.T) {
	cases := []struct {
		name string
		fam  Family
		want bool
	}{
		{"ipv4 unicast", IPv4Unicast, true},
		{"ipv6 unicast", IPv6Unicast, true},
		{"ipv4 labeled unicast", Family{AFI: AFIIPv4, SAFI: SAFIMPLSLabel}, true},
		{"ipv4 mpls-vpn", Family{AFI: AFIIPv4, SAFI: SAFIVPN}, true},
		{"ipv6 mpls-vpn", Family{AFI: AFIIPv6, SAFI: SAFIVPN}, true},
		{"ipv4 mcast-vpn", Family{AFI: AFIIPv4, SAFI: SAFIMVPN}, true},
		{"ipv4 mup", Family{AFI: AFIIPv4, SAFI: SAFIMUP}, true},
		{"ipv4 flowspec", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}, false},
		{"ipv4 flowspec vpn", Family{AFI: AFIIPv4, SAFI: SAFIFlowSpecVPN}, false},
		{"l2vpn vpls", Family{AFI: AFIL2VPN, SAFI: SAFIVPLS}, false},
		{"l2vpn evpn", Family{AFI: AFIL2VPN, SAFI: SAFIEVPN}, false},
		{"ipv4 sr-policy", Family{AFI: AFIIPv4, SAFI: SAFISRPolicy}, false},
		{"ipv4 rtc", Family{AFI: AFIIPv4, SAFI: SAFIRTC}, false},
		{"bgp-ls", Family{AFI: AFIBGPLS, SAFI: SAFIBGPLinkState}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fam.LegacyNextHop(); got != tc.want {
				t.Errorf("%s.LegacyNextHop() = %v, want %v", tc.fam, got, tc.want)
			}
		})
	}
}
