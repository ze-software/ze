package api

// KeywordSet defines which keywords are valid for a route family.
type KeywordSet map[string]bool

// UnicastKeywords defines valid keywords for IPv4/IPv6 unicast routes.
var UnicastKeywords = KeywordSet{
	"next-hop":           true,
	"origin":             true,
	"med":                true,
	"local-preference":   true,
	"as-path":            true,
	"community":          true,
	"large-community":    true,
	"extended-community": true, // RFC 4360 extended communities
	"split":              true, // ZeBGP extension
}

// MPLSKeywords defines valid keywords for MPLS labeled unicast routes (SAFI 4).
// This is unicast + label + split (no RD/RT - those are VPN-only).
var MPLSKeywords = KeywordSet{
	"next-hop":           true,
	"origin":             true,
	"med":                true,
	"local-preference":   true,
	"as-path":            true,
	"community":          true,
	"large-community":    true,
	"extended-community": true, // RFC 4360 extended communities
	"label":              true, // MPLS label stack
	"split":              true, // Prefix expansion (same label per prefix)
}

// VPNKeywords defines valid keywords for VPN routes.
// Used for L3VPN (SAFI 128) routes which require RD and label.
// Note: "split" is intentionally excluded - RD/label apply to entire prefix.
var VPNKeywords = KeywordSet{
	"next-hop":           true,
	"origin":             true,
	"med":                true,
	"local-preference":   true,
	"as-path":            true,
	"community":          true,
	"large-community":    true,
	"extended-community": true, // RFC 4360 extended communities
	"rd":                 true, // Route Distinguisher
	"rt":                 true, // Route Target
	"label":              true, // MPLS label
}

// MUPKeywords defines valid keywords for MUP routes (SAFI 85).
// Per draft-mpmz-bess-mup-safi for Mobile User Plane.
var MUPKeywords = KeywordSet{
	"next-hop":            true,
	"origin":              true,
	"local-preference":    true,
	"as-path":             true,
	"extended-community":  true, // Route targets
	"rd":                  true, // Route Distinguisher
	"bgp-prefix-sid-srv6": true, // SRv6 Prefix SID (RFC 9252)
	"teid":                true, // Tunnel Endpoint ID (for T1ST/T2ST)
	"qfi":                 true, // QoS Flow Identifier
	"endpoint":            true, // GTP endpoint address
	"source":              true, // Source address (optional)
}
