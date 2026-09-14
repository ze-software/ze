// Design: docs/architecture/core-design.md — family and nexthop syntax conversion
// Overview: migrate.go — migration orchestration and neighbor conversion
// Related: migrate_routes.go — route conversion to update blocks
// Related: migrate_serialize.go — tree serialization

package migration

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/family"
)

const (
	safiNLRIMPLS = "nlri-mpls"
	safiVPLS     = "vpls"
)

// convertFamilyToList converts ExaBGP family syntax to ZeBGP list entries.
// ExaBGP: "ipv4 unicast;" -> ZeBGP: session > family list: key="ipv4/unicast".
func convertFamilyToList(src, dst *config.Tree) {
	// Get keys and sort for deterministic output.
	keys := src.Values()
	slices.Sort(keys)

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

// exabgpFamilies maps each ExaBGP family phrase to the Ze family it names.
//
// The ExaBGP vocabulary on the left is external, and no Ze registry holds it:
// ExaBGP writes "nlri-mpls", "mcast-vpn" and "flowspec" where Ze's registrars
// write "mpls-label", "mvpn" and "flow". That mapping is this file's own
// content and it is the source of truth for an ExaBGP to Ze family rename.
//
// The Ze NAME is not this file's content. Each entry holds the AFI and SAFI
// pair, and the family registry renders the name family.MustRegister composed
// from them, so a renamed SAFI reaches the migrated config without an edit
// here. TestExaBGPFamiliesAreRegistered holds every entry to a registered
// family, so a pair that names none cannot reach a config as "afi-1/safi-133".
var exabgpFamilies = map[string]family.Family{
	"ipv4 unicast":   {AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
	"ipv4 multicast": {AFI: family.AFIIPv4, SAFI: family.SAFIMulticast},
	"ipv4 nlri-mpls": {AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
	"ipv4 flowspec":  {AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec},
	"ipv4 mcast-vpn": {AFI: family.AFIIPv4, SAFI: family.SAFIMVPN},
	"ipv6 unicast":   {AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
	"ipv6 multicast": {AFI: family.AFIIPv6, SAFI: family.SAFIMulticast},
	"ipv6 nlri-mpls": {AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel},
	"ipv6 flowspec":  {AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec},
	"ipv6 mcast-vpn": {AFI: family.AFIIPv6, SAFI: family.SAFIMVPN},
	"l2vpn vpls":     {AFI: family.AFIL2VPN, SAFI: family.SAFIVPLS},
	"l2vpn evpn":     {AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN},
}

// convertFamilySyntax converts an ExaBGP family phrase to its Ze family name.
// Examples: "ipv4 unicast" becomes "ipv4/unicast", "ipv4 flowspec" becomes
// "ipv4/flow". A phrase the table does not hold keeps its two words, joined by
// a slash.
func convertFamilySyntax(name string) string {
	if fam, ok := exabgpFamilies[strings.ToLower(name)]; ok {
		return fam.String()
	}

	// Fallback: replace first space with slash.
	return strings.Replace(name, " ", "/", 1)
}

// convertNexthopBlock converts ExaBGP nexthop syntax to ZeBGP.
// ExaBGP: "ipv4 unicast ipv6;" → ZeBGP: "ipv4/unicast ipv6;".
// The nexthop block maps (AFI, SAFI) → NextHop-AFI.
func convertNexthopBlock(src *config.Tree) *config.Tree {
	dst := config.NewTree()

	// Get keys and sort for deterministic output.
	keys := src.Values()
	slices.Sort(keys)

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
	case safiNLRIMPLS, "labeled-unicast":
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
	case safiNLRIMPLS, "labeled-unicast":
		return "mpls-label"
	case "flowspec":
		return "flow"
	default: // pass through: unknown SAFIs are preserved for the Ze parser to validate
		return safi
	}
}
