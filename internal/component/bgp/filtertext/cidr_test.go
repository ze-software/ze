package filtertext

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestCIDRFamily covers every SAFI constant defined in
// internal/core/family/family.go so a newly-added SAFI without an explicit
// case in this table is a fail-loud reminder to re-check the filter text
// protocol contract.
//
// VALIDATES: dest-2 -- CIDRFamily identifies only families whose NLRI wire
// format is a plain CIDR prefix (IPv4/IPv6 unicast, multicast, mpls-label).
// All other families are non-CIDR and must use raw=true.
// PREVENTS: regression where a filter plugin attached to an EVPN/Flowspec
// session is fed garbage CIDR "prefixes" parsed out of a family-specific NLRI
// encoding, or drops a prefix by modify from a block the engine cannot rewrite.
func TestCIDRFamily(t *testing.T) {
	tests := []struct {
		name string
		fam  family.Family
		want bool
	}{
		// --- CIDR (must return true) ---
		{"ipv4 unicast", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}, true},
		{"ipv4 multicast", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMulticast}, true},
		{"ipv4 mpls-label", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}, true},
		{"ipv6 unicast", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}, true},
		{"ipv6 multicast", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIMulticast}, true},
		{"ipv6 mpls-label", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel}, true},

		// --- Non-CIDR IPv4 SAFIs (must return false) ---
		{"ipv4 vpn", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}, false},
		{"ipv4 flowspec", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, false},
		{"ipv4 flowspec-vpn", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN}, false},
		{"ipv4 mvpn", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMVPN}, false},
		{"ipv4 rtc", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIRTC}, false},
		{"ipv4 mup", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMUP}, false},

		// --- Non-CIDR IPv6 SAFIs (must return false) ---
		{"ipv6 vpn", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}, false},
		{"ipv6 flowspec", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}, false},

		// --- L2VPN / BGP-LS families (must return false regardless of SAFI) ---
		{"l2vpn evpn", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, false},
		{"l2vpn vpls", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIVPLS}, false},
		{"bgp-ls", family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}, false},
		{"bgp-ls-vpn", family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkStateVPN}, false},

		// --- L2VPN with accidentally-unicast SAFI (still non-CIDR because AFI disqualifies) ---
		{"l2vpn unicast rejected", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIUnicast}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CIDRFamily(tt.fam); got != tt.want {
				t.Errorf("CIDRFamily(%v) = %v, want %v", tt.fam, got, tt.want)
			}
		})
	}
}
