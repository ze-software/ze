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

// VPNKeywords defines valid keywords for VPN routes (unicast + VPN-specific).
// Reserved for future VPN route support (L3VPN, MPLS).
//
//nolint:gochecknoglobals // Intentionally unused until VPN support is implemented.
var VPNKeywords = mergeKeywords(UnicastKeywords, KeywordSet{
	"rd":    true, // Route Distinguisher
	"rt":    true, // Route Target
	"label": true, // MPLS label
})

// mergeKeywords combines two keyword sets into a new set.
func mergeKeywords(base, extra KeywordSet) KeywordSet {
	result := make(KeywordSet, len(base)+len(extra))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range extra {
		result[k] = v
	}
	return result
}
