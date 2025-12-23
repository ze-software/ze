package api

// KeywordSet defines which keywords are valid for a route family.
type KeywordSet map[string]bool

// UnicastKeywords defines valid keywords for IPv4/IPv6 unicast routes.
var UnicastKeywords = KeywordSet{
	"next-hop":         true,
	"origin":           true,
	"med":              true,
	"local-preference": true,
	"as-path":          true,
	"community":        true,
	"large-community":  true,
	"split":            true, // ZeBGP extension
}

// MPLSKeywords defines valid keywords for MPLS labeled unicast routes (SAFI 4).
// This is unicast + label (no RD/RT - those are VPN-only).
// Note: "split" is intentionally excluded - label applies to entire prefix.
var MPLSKeywords = KeywordSet{
	"next-hop":         true,
	"origin":           true,
	"med":              true,
	"local-preference": true,
	"as-path":          true,
	"community":        true,
	"large-community":  true,
	"label":            true, // MPLS label stack
}

// VPNKeywords defines valid keywords for VPN routes.
// Used for L3VPN (SAFI 128) routes which require RD and label.
// Note: "split" is intentionally excluded - RD/label apply to entire prefix.
var VPNKeywords = KeywordSet{
	"next-hop":         true,
	"origin":           true,
	"med":              true,
	"local-preference": true,
	"as-path":          true,
	"community":        true,
	"large-community":  true,
	"rd":               true, // Route Distinguisher
	"rt":               true, // Route Target
	"label":            true, // MPLS label
}
