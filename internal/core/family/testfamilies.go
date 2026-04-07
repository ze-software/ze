// Design: docs/architecture/wire/nlri.md -- test helper for family registration

package family

import "log/slog"

// RegisterTestFamilies registers all standard families for use in tests.
// Call from TestMain in packages that need Family.String() or LookupFamily().
// Safe to call multiple times (re-registration with same values is a no-op).
func RegisterTestFamilies() {
	families := []struct {
		afi     AFI
		safi    SAFI
		afiStr  string
		safiStr string
	}{
		{AFIIPv4, SAFIUnicast, "ipv4", "unicast"},
		{AFIIPv6, SAFIUnicast, "ipv6", "unicast"},
		{AFIIPv4, SAFIMulticast, "ipv4", "multicast"},
		{AFIIPv6, SAFIMulticast, "ipv6", "multicast"},
		{AFIIPv4, SAFIMPLSLabel, "ipv4", "mpls-label"},
		{AFIIPv6, SAFIMPLSLabel, "ipv6", "mpls-label"},
		{AFIIPv4, SAFIVPN, "ipv4", "mpls-vpn"},
		{AFIIPv6, SAFIVPN, "ipv6", "mpls-vpn"},
		{AFIL2VPN, SAFIEVPN, "l2vpn", "evpn"},
		{AFIIPv4, SAFIFlowSpec, "ipv4", "flow"},
		{AFIIPv6, SAFIFlowSpec, "ipv6", "flow"},
		{AFIIPv4, SAFIFlowSpecVPN, "ipv4", "flow-vpn"},
		{AFIIPv6, SAFIFlowSpecVPN, "ipv6", "flow-vpn"},
		{AFIIPv4, SAFIMVPN, "ipv4", "mvpn"},
		{AFIIPv6, SAFIMVPN, "ipv6", "mvpn"},
		{AFIL2VPN, SAFIVPLS, "l2vpn", "vpls"},
		{AFIIPv4, SAFIRTC, "ipv4", "rtc"},
		{AFIIPv4, SAFIMUP, "ipv4", "mup"},
		{AFIIPv6, SAFIMUP, "ipv6", "mup"},
		{AFIBGPLS, SAFIBGPLinkState, "bgp-ls", "bgp-ls"},
		{AFIBGPLS, SAFIBGPLinkStateVPN, "bgp-ls", "bgp-ls-vpn"},
	}
	for _, f := range families {
		if _, err := RegisterFamily(f.afi, f.safi, f.afiStr, f.safiStr); err != nil {
			slog.Error("RegisterTestFamilies failed", "afi", f.afi, "safi", f.safi, "error", err)
		}
	}
}
