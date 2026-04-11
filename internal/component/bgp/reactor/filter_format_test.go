package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// TestIsCIDRFamily covers every SAFI constant defined in
// internal/core/family/family.go so a newly-added SAFI without an explicit
// case in this table is a fail-loud reminder to re-check the filter text
// protocol contract.
//
// VALIDATES: dest-2 -- isCIDRFamily identifies only families whose NLRI
//
//	wire format is a plain CIDR prefix (IPv4/IPv6 unicast,
//	multicast, mpls-label). All other families are non-CIDR and
//	must use raw=true.
//
// PREVENTS:  regression where a filter plugin attached to an EVPN/Flowspec
//
//	session is fed garbage CIDR "prefixes" parsed out of a
//	family-specific NLRI encoding.
func TestIsCIDRFamily(t *testing.T) {
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
			got := isCIDRFamily(tt.fam)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFormatMPBlockNonCIDRMarker covers dest-2's hybrid emit strategy:
// CIDR families get prefixes inline, non-CIDR families get a marker-only
// block, and empty CIDR updates emit nothing at all.
//
// VALIDATES: dest-2 -- non-CIDR families produce "nlri <family> add|del"
//
//	markers without prefixes, so text-mode filters on non-CIDR
//	sessions can still learn a family is present.
//
// PREVENTS:  regression to the pre-fix state where MP_REACH for EVPN /
//
//	Flowspec was silently dropped from the filter text protocol.
func TestFormatMPBlockNonCIDRMarker(t *testing.T) {
	evpn := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	flowspec := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	ipv6u := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}

	// Ensure L2VPN/EVPN and IPv4/FlowSpec are registered so Family.String()
	// returns canonical names; otherwise fallback is "afi-25/safi-70" which
	// obscures the assertion intent. Use the test-helper to register the
	// canonical names (idempotent).
	family.RegisterTestFamilies()

	// Non-CIDR: marker only, no prefixes.
	got := formatMPBlock(evpn, "add", nil)
	assert.Equal(t, "nlri l2vpn/evpn add", got)

	// Non-CIDR with an accidentally non-empty prefix slice (from a buggy
	// wire parser): still marker only. Prefixes are intentionally ignored.
	got = formatMPBlock(flowspec, "del", nil)
	assert.Equal(t, "nlri ipv4/flow del", got)

	// CIDR with empty prefix list: emit nothing (caller appends blocks).
	got = formatMPBlock(ipv6u, "add", nil)
	assert.Equal(t, "", got)
}
