// Design: docs/architecture/route-types.md — route definitions

package route

// Route attribute keywords, as an API route command spells them. Each table
// below names the subset that one family accepts, so a keyword cannot be
// spelled two ways across the four tables.
const (
	keywordNextHop           = "next-hop"
	keywordOrigin            = "origin"
	keywordMED               = "med"
	keywordLocalPreference   = "local-preference"
	keywordASPath            = "as-path"
	keywordCommunity         = "community"
	keywordLargeCommunity    = "large-community"
	keywordExtendedCommunity = "extended-community"
	keywordAIGP              = "aigp"
	keywordSplit             = "split"
	keywordLabel             = "label"
	keywordPathID            = "path-id"
	keywordRD                = "rd"
	keywordRT                = "rt"
	keywordPrefixSIDSRv6     = "bgp-prefix-sid-srv6"
	keywordTEID              = "teid"
	keywordQFI               = "qfi"
	keywordEndpoint          = "endpoint"
	keywordSource            = "source"
)

// KeywordSet defines which keywords are valid for a route family.
type KeywordSet map[string]bool

// UnicastKeywords defines valid keywords for IPv4/IPv6 unicast routes.
var UnicastKeywords = KeywordSet{
	keywordNextHop:           true,
	keywordOrigin:            true,
	keywordMED:               true,
	keywordLocalPreference:   true,
	keywordASPath:            true,
	keywordCommunity:         true,
	keywordLargeCommunity:    true,
	keywordExtendedCommunity: true, // RFC 4360 extended communities
	keywordAIGP:              true, // RFC 7311 accumulated IGP metric
	keywordSplit:             true, // ze extension
}

// MPLSKeywords defines valid keywords for MPLS labeled unicast routes (SAFI 4).
// This is unicast + label + split + path-id (no RD/RT - those are VPN-only).
var MPLSKeywords = KeywordSet{
	keywordNextHop:           true,
	keywordOrigin:            true,
	keywordMED:               true,
	keywordLocalPreference:   true,
	keywordASPath:            true,
	keywordCommunity:         true,
	keywordLargeCommunity:    true,
	keywordExtendedCommunity: true, // RFC 4360 extended communities
	keywordAIGP:              true, // RFC 7311 accumulated IGP metric
	keywordLabel:             true, // MPLS label stack
	keywordSplit:             true, // Prefix expansion (same label per prefix)
	keywordPathID:            true, // ADD-PATH identifier (RFC 7911)
}

// VPNKeywords defines valid keywords for VPN routes.
// Used for L3VPN (SAFI 128) routes which require RD and label.
// Note: "split" is intentionally excluded - RD/label apply to entire prefix.
var VPNKeywords = KeywordSet{
	keywordNextHop:           true,
	keywordOrigin:            true,
	keywordMED:               true,
	keywordLocalPreference:   true,
	keywordASPath:            true,
	keywordCommunity:         true,
	keywordLargeCommunity:    true,
	keywordExtendedCommunity: true, // RFC 4360 extended communities
	keywordAIGP:              true, // RFC 7311 accumulated IGP metric
	keywordRD:                true, // Route Distinguisher
	keywordRT:                true, // Route Target
	keywordLabel:             true, // MPLS label
}

// MUPKeywords defines valid keywords for MUP routes (SAFI 85).
// Per draft-mpmz-bess-mup-safi for Mobile User Plane.
var MUPKeywords = KeywordSet{
	keywordNextHop:           true,
	keywordOrigin:            true,
	keywordLocalPreference:   true,
	keywordASPath:            true,
	keywordExtendedCommunity: true, // Route targets
	keywordRD:                true, // Route Distinguisher
	keywordPrefixSIDSRv6:     true, // SRv6 Prefix SID (RFC 9252)
	keywordTEID:              true, // Tunnel Endpoint ID (for T1ST/T2ST)
	keywordQFI:               true, // QoS Flow Identifier
	keywordEndpoint:          true, // GTP endpoint address
	keywordSource:            true, // Source address (optional)
}
