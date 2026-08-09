// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
// RFC: rfc/short/draft-ietf-bess-mup-safi.md -- Section 3.1, BGP-MUP NLRI envelope
// Related: register.go -- binds SplitMUP to AFI IPv4/IPv6 with SAFI MUP
// Related: typelen.go -- the shared type-and-length framing walk

package nlrisplit

// SplitMUP is the Splitter for BGP-MUP NLRIs (draft-ietf-bess-mup-safi Section 3.1).
//
// The BGP-MUP NLRI envelope is Architecture Type (1 octet), Route Type (2
// octets), Length (1 octet), then Route Type specific data of exactly that
// length. The header is therefore 4 octets with the length octet last, which is
// the only way this family differs from EVPN and MCAST-VPN.
//
// The splitter is route-type-agnostic. Which architecture and route types ze
// implements is RFC 7606 Section 5.4's question, answered in the MUP plugin
// (internal/component/bgp/plugins/nlri/mup, nlritype.Recognizer).
//
// Slices alias data. A malformed entry returns the partially-parsed result plus
// a non-nil error; the caller decides whether to use it.
func SplitMUP(data []byte, addPath bool) ([][]byte, error) {
	return splitTypeLength(data, addPath, 4, 3, "MUP")
}
