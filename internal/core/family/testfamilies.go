// Design: docs/architecture/wire/nlri.md -- test helper for family registration

package family

import "log/slog"

// RegisterTestFamilies registers all standard families for use in tests.
// Call from TestMain in packages that need Family.String() or LookupFamily().
// Safe to call multiple times (re-registration with same values is a no-op).
func RegisterTestFamilies() {
	families := []struct {
		afi      AFI
		safi     SAFI
		afiName  string
		safiName string
	}{
		{AFIIPv4, SAFIUnicast, afiNameIPv4, "unicast"},
		{AFIIPv6, SAFIUnicast, afiNameIPv6, "unicast"},
		{AFIIPv4, SAFIMulticast, afiNameIPv4, "multicast"},
		{AFIIPv6, SAFIMulticast, afiNameIPv6, "multicast"},
		{AFIIPv4, SAFIMPLSLabel, afiNameIPv4, "mpls-label"},
		{AFIIPv6, SAFIMPLSLabel, afiNameIPv6, "mpls-label"},
		{AFIIPv4, SAFIVPN, afiNameIPv4, "mpls-vpn"},
		{AFIIPv6, SAFIVPN, afiNameIPv6, "mpls-vpn"},
		{AFIL2VPN, SAFIEVPN, afiNameL2VPN, "evpn"},
		{AFIIPv4, SAFIFlowSpec, afiNameIPv4, "flow"},
		{AFIIPv6, SAFIFlowSpec, afiNameIPv6, "flow"},
		{AFIIPv4, SAFIFlowSpecVPN, afiNameIPv4, "flow-vpn"},
		{AFIIPv6, SAFIFlowSpecVPN, afiNameIPv6, "flow-vpn"},
		{AFIIPv4, SAFIMVPN, afiNameIPv4, "mvpn"},
		{AFIIPv6, SAFIMVPN, afiNameIPv6, "mvpn"},
		{AFIL2VPN, SAFIVPLS, afiNameL2VPN, "vpls"},
		{AFIIPv4, SAFIRTC, afiNameIPv4, "rtc"},
		{AFIIPv4, SAFIMUP, afiNameIPv4, "mup"},
		{AFIIPv6, SAFIMUP, afiNameIPv6, "mup"},
		{AFIBGPLS, SAFIBGPLinkState, afiNameBGPLS, "bgp-ls"},
		{AFIBGPLS, SAFIBGPLinkStateVPN, afiNameBGPLS, "bgp-ls-vpn"},
	}
	for _, f := range families {
		if _, err := RegisterFamily(f.afi, f.safi, f.afiName, f.safiName); err != nil {
			slog.Error("RegisterTestFamilies failed", "afi", f.afi, "safi", f.safi, "error", err)
		}
	}
}
