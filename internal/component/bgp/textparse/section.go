// Design: docs/architecture/api/commands.md — the nlri and mp section keywords of update text
// Related: keywords.go — shared keyword definitions

package textparse

import "strings"

// The two keywords that open an NLRI section of an update text command. Each
// names the UPDATE section its prefixes travel in, the same two keywords the
// filter text protocol uses.
const (
	// KWNLRISection opens IPv4 unicast in the NLRI and Withdrawn Routes fields.
	// It takes no family token.
	KWNLRISection = "nlri"
	// KWMPSection opens `mp <family>`: MP_REACH_NLRI and MP_UNREACH_NLRI.
	KWMPSection = "mp"
)

// LegacyNLRISection reports whether a command that announces or withdraws
// family writes it as `nlri` rather than `mp <family>`.
//
// RFC 4271 Section 4.3: the Network Layer Reachability Information field
// "contains a list of IP address prefixes", with no AFI or SAFI, so only IPv4
// unicast travels there. An IPv6 next hop for IPv4 unicast cannot: RFC 8950
// Section 3 carries it in MP_REACH_NLRI, so nextHopIPv6 selects `mp`. A
// withdrawal passes false, because ze withdraws IPv4 unicast in the Withdrawn
// Routes field whatever next hop announced it.
//
// An IPv4 next-hop literal never holds a colon, so IPv6NextHopText tells the two
// address families apart without parsing.
func LegacyNLRISection(family string, nextHopIPv6 bool) bool {
	if family != "ipv4/unicast" {
		return false
	}
	return !nextHopIPv6
}

// IPv6NextHopText reports whether a next-hop token is an IPv6 address literal.
// "self" and an IPv4 literal hold no colon.
func IPv6NextHopText(nextHop string) bool {
	return strings.IndexByte(nextHop, ':') >= 0
}
