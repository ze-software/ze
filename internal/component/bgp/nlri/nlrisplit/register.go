// Design: plan/design-rib-unified.md -- Phase 3g (per-family NLRI split)
// Related: cidr.go -- CIDR splitter registered here
// Related: evpn.go -- EVPN splitter registered here

package nlrisplit

import "codeberg.org/thomas-mangin/ze/internal/core/family"

func init() {
	for _, fam := range []family.Family{
		family.IPv4Unicast,
		family.IPv6Unicast,
		{AFI: family.AFIIPv4, SAFI: family.SAFIMulticast},
		{AFI: family.AFIIPv6, SAFI: family.SAFIMulticast},
	} {
		Register(fam, SplitCIDR)
	}
	Register(family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, SplitEVPN)
}
