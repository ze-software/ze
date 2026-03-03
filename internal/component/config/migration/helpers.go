// Design: docs/architecture/config/syntax.md — config migration

package migration

import (
	"net/netip"
	"strings"
)

// isIPv6Prefix returns true if the prefix is IPv6.
// Detection: contains ":" (IPv4-mapped IPv6 like ::ffff:x.x.x.x is IPv6).
func isIPv6Prefix(prefix string) bool {
	return strings.Contains(prefix, ":")
}

// isMulticastPrefix returns true if the prefix is in the multicast range.
// IPv4: 224.0.0.0/4 (224.0.0.0 - 239.255.255.255).
// IPv6: ff00::/8.
func isMulticastPrefix(prefix string) bool {
	// Parse the prefix
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		return false
	}

	addr := p.Addr()

	if addr.Is4() {
		// IPv4 multicast: 224.0.0.0/4 (first octet 224-239)
		bytes := addr.As4()                       // As4() returns [4]byte, index 0 is always valid
		return bytes[0] >= 224 && bytes[0] <= 239 //nolint:gosec // As4 returns fixed [4]byte
	}

	if addr.Is6() {
		// IPv6 multicast: ff00::/8 (first byte is 0xff)
		bytes := addr.As16()
		return bytes[0] == 0xff
	}

	return false
}

// detectSAFI determines the SAFI for a route based on prefix and attributes.
//
// Detection order:
//  1. Multicast range → "multicast"
//  2. Has rd → "mpls-vpn" (SAFI 128, L3VPN)
//  3. Has label only → "nlri-mpls" (SAFI 4, labeled unicast)
//  4. Default → "unicast"
//
// RFC 8277: Labeled unicast uses SAFI 4 (prefix + label, no RD).
// RFC 4364: L3VPN uses SAFI 128 (RD:prefix + label).
func detectSAFI(prefix string, hasRD, hasLabel bool) string {
	// Check multicast first (takes precedence)
	if isMulticastPrefix(prefix) {
		return "multicast"
	}

	// RFC 4364: RD present = L3VPN (SAFI 128)
	if hasRD {
		return "mpls-vpn"
	}

	// RFC 8277: Label only (no RD) = labeled unicast (SAFI 4)
	if hasLabel {
		return "nlri-mpls"
	}

	return "unicast"
}
