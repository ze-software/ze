// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
// Related: cidr.go -- CIDR splitter registered here
// Related: evpn.go, mvpn.go, mup.go -- the typed splitters registered here
// Related: flowspec.go, vpls.go, bgpls.go -- the length-framed splitters registered here

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

	// A family ze negotiates and decodes but does not split is accepted on the
	// wire and then dropped: every RIB entry point returns early on
	// Supported(fam), so the routes are lost with one Debug line and nothing
	// turns red. The six families below were in that state, so each one is
	// registered beside the families that always were.

	// MPLS VPN (RFC 4364 Section 4.3.4): the one length octet counts the label
	// stack and the Route Distinguisher as well as the prefix.
	for _, fam := range []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIVPN},
	} {
		Register(fam, splitVPN)
	}

	// Route Target Constrain (RFC 4684 Section 4): [length in bits][origin
	// AS:4][route target:8], which is the CIDR framing with a 96-bit maximum.
	for _, fam := range []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIRTC},
	} {
		Register(fam, splitCIDR)
	}

	// Flow specification (RFC 8955 Section 4, RFC 8956): a 1- or 2-octet length.
	for _, fam := range []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec},
		{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec},
		{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpecVPN},
	} {
		Register(fam, SplitFlowSpec)
	}

	// VPLS (RFC 4761 Section 3.2.2): a 2-octet length.
	Register(family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIVPLS}, SplitVPLS)

	// Link-State (RFC 9552 Section 5.1): [NLRI Type:2][Total NLRI Length:2].
	for _, fam := range []family.Family{
		{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState},
		{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkStateVPN},
	} {
		Register(fam, SplitBGPLS)
	}
}
