// Design: docs/architecture/core-design.md — family and nexthop syntax conversion
// Overview: migrate.go — migration orchestration and neighbor conversion
// Related: migrate_routes.go — route conversion to update blocks
// Related: migrate_serialize.go — tree serialization

package migration

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// convertFamilyToList converts ExaBGP family syntax to ZeBGP list entries.
// ExaBGP: "ipv4 unicast;" -> ZeBGP: session > family list: key="ipv4/unicast".
func convertFamilyToList(src, dst *config.Tree) {
	// Get keys and sort for deterministic output.
	keys := src.Values()
	sort.Strings(keys)

	// Families go into session > family.
	sessionContainer := dst.GetContainer("session")
	if sessionContainer == nil {
		sessionContainer = config.NewTree()
		dst.SetContainer("session", sessionContainer)
	}

	for _, key := range keys {
		// Convert "ipv4 unicast" -> "ipv4/unicast".
		converted := convertFamilySyntax(key)
		// Every family requires prefix { maximum N; } (RFC 4486).
		// Use 10000 as a sensible default for migrated configs.
		familyTree := config.NewTree()
		prefixTree := config.NewTree()
		prefixTree.Set("maximum", "10000")
		familyTree.SetContainer("prefix", prefixTree)
		sessionContainer.AddListEntry("family", converted, familyTree)
	}
}

// convertFamilySyntax converts ExaBGP family format to ZeBGP.
// Examples: "ipv4 unicast" → "ipv4/unicast", "ipv6 multicast" → "ipv6/multicast".
//
// Several ExaBGP SAFI names differ from Ze's canonical names registered via
// family.MustRegister: ExaBGP "nlri-mpls" → Ze "mpls-label", ExaBGP
// "mcast-vpn" → Ze "mvpn", ExaBGP "flowspec" → Ze "flow". The replacements
// table here is the source of truth for ExaBGP→Ze family renames at the
// migration layer.
func convertFamilySyntax(family string) string {
	// Common ExaBGP family formats.
	replacements := map[string]string{
		"ipv4 unicast":   "ipv4/unicast",
		"ipv4 multicast": "ipv4/multicast",
		"ipv4 nlri-mpls": "ipv4/mpls-label",
		"ipv4 flowspec":  "ipv4/flow",
		"ipv4 mcast-vpn": "ipv4/mvpn",
		"ipv6 unicast":   "ipv6/unicast",
		"ipv6 multicast": "ipv6/multicast",
		"ipv6 nlri-mpls": "ipv6/mpls-label",
		"ipv6 flowspec":  "ipv6/flow",
		"ipv6 mcast-vpn": "ipv6/mvpn",
		"l2vpn vpls":     "l2vpn/vpls",
		"l2vpn evpn":     "l2vpn/evpn",
	}

	if converted, ok := replacements[strings.ToLower(family)]; ok {
		return converted
	}

	// Fallback: replace first space with slash.
	return strings.Replace(family, " ", "/", 1)
}

// convertNexthopBlock converts ExaBGP nexthop syntax to ZeBGP.
// ExaBGP: "ipv4 unicast ipv6;" → ZeBGP: "ipv4/unicast ipv6;".
// The nexthop block maps (AFI, SAFI) → NextHop-AFI.
func convertNexthopBlock(src *config.Tree) *config.Tree {
	dst := config.NewTree()

	// Get keys and sort for deterministic output.
	keys := src.Values()
	sort.Strings(keys)

	for _, key := range keys {
		// ExaBGP stores "ipv4 unicast ipv6" as key, value "true".
		// Convert to ZeBGP format: "ipv4/unicast ipv6".
		converted := convertNexthopSyntax(key)
		dst.Set(converted, "")
	}

	return dst
}

// convertNexthopSyntax converts ExaBGP nexthop format to ZeBGP.
// ExaBGP: "ipv4 unicast ipv6" → ZeBGP: "ipv4/unicast ipv6".
// Format: "<afi> <safi> <nhafi>" → "<afi>/<safi> <nhafi>".
func convertNexthopSyntax(nexthop string) string {
	parts := strings.Fields(nexthop)
	if len(parts) != 3 {
		// Unknown format, return as-is.
		return nexthop
	}

	// parts[0] = afi (ipv4/ipv6)
	// parts[1] = safi (unicast/mpls-vpn/etc)
	// parts[2] = nexthop-afi (ipv4/ipv6)

	// Normalize SAFI names to ZeBGP conventions.
	// ZeBGP's parseNexthopFamilies expects "mpls-label" for SAFI 4.
	safi := normalizeSAFI(parts[1])

	return parts[0] + "/" + safi + " " + parts[2]
}

// normalizeSAFI converts ExaBGP SAFI names to ZeBGP conventions.
// ExaBGP uses "nlri-mpls" and "labeled-unicast" for SAFI 4.
// ZeBGP's nexthop parser expects "mpls-label".
func normalizeSAFI(safi string) string {
	switch strings.ToLower(safi) {
	case "nlri-mpls", "labeled-unicast":
		return "mpls-label"
	default: // pass through: unknown SAFIs are preserved as-is for the Ze parser to validate
		return safi
	}
}

// canonicalSAFI translates an ExaBGP SAFI string to the canonical Ze SAFI
// expected by the family registry (`internal/core/family`). Used when
// constructing Ze family names ("<afi>/<safi>") from ExaBGP source config.
// Unknown SAFIs pass through unchanged so the Ze parser produces an
// "unknown address family" error at config load time.
func canonicalSAFI(safi string) string {
	switch strings.ToLower(safi) {
	case "mcast-vpn":
		return "mvpn"
	case "nlri-mpls", "labeled-unicast":
		return "mpls-label"
	case "flowspec":
		return "flow"
	default: // pass through: unknown SAFIs are preserved for the Ze parser to validate
		return safi
	}
}
