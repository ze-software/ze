// Design: plan/learned/639-rib-unified.md -- Phase 3g (per-family NLRI split)
// Related: register.go -- binds SplitEVPN to AFI L2VPN / SAFI EVPN
// Related: typelen.go -- the shared type-and-length framing walk

package nlrisplit

// SplitEVPN is the Splitter for L2VPN/EVPN NLRIs (RFC 7432 Section 7.1
// and RFC 8365 Section 8). Every EVPN NLRI is framed as
// [route-type:1][length:1][route-type-specific:length]. Under ADD-PATH
// (RFC 7911) each NLRI is prefixed with a 4-byte path-id that is
// included in the returned slice.
//
// EVPN route-types 1-5 are standardized (Ethernet Auto-Discovery, MAC/IP
// Advertisement, Inclusive Multicast Ethernet Tag, Ethernet Segment,
// IP Prefix); higher numbers are reserved or IANA-assigned. The
// splitter is route-type-agnostic -- it only uses the length byte to
// carve boundaries. Semantic interpretation lives in the EVPN plugin
// (internal/component/bgp/plugins/nlri/evpn).
//
// Slices alias `data`. A malformed entry returns the partially-parsed
// result plus a non-nil error; the caller decides whether to use it.
func SplitEVPN(data []byte, addPath bool) ([][]byte, error) {
	return splitTypeLength(data, addPath, 2, 1, "EVPN")
}
