// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
// Related: cidr.go -- CIDR splitter registered here
// Related: evpn.go, mvpn.go, mup.go -- the typed splitters registered here

package nlrisplit

import "github.com/ze-software/ze/internal/core/family"

func init() {
	for _, fam := range []family.Family{
		family.IPv4Unicast,
		family.IPv6Unicast,
		{AFI: family.AFIIPv4, SAFI: family.SAFIMulticast},
		{AFI: family.AFIIPv6, SAFI: family.SAFIMulticast},
	} {
		Register(fam, splitCIDR)
	}
	Register(family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, splitEVPN)
	// RFC 7606 Section 5.4 names MCAST-VPN and EVPN as typed families in the same
	// sentence. Both need a splitter before a route type can be judged, so both have
	// one here; draft-ietf-bess-mup-safi frames MUP the same way with a wider header.
	for _, fam := range []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIMVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIMVPN},
	} {
		Register(fam, splitMVPN)
	}
	for _, fam := range []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIMUP},
		{AFI: family.AFIIPv6, SAFI: family.SAFIMUP},
	} {
		Register(fam, SplitMUP)
	}
	for _, fam := range []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
		{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel},
	} {
		Register(fam, SplitLabeled)
	}
}
