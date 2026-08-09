// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
// RFC: rfc/short/rfc7606.md -- Section 5.4 names MCAST-VPN as a typed family
// Related: register.go -- binds SplitMVPN to AFI IPv4/IPv6 with SAFI MCAST-VPN
// Related: typelen.go -- the shared type-and-length framing walk

package nlrisplit

// SplitMVPN is the Splitter for MCAST-VPN NLRIs (RFC 6514 Section 4).
//
// RFC 6514 Section 4: "The following is the format of the MCAST-VPN NLRI:
// Route Type (1 octet), Length (1 octet), Route Type specific (variable)", where
// "The Length field indicates the length in octets of the Route Type specific
// field". That is the same envelope EVPN uses, so both share one walk.
//
// The splitter is route-type-agnostic. RFC 7606 Section 5.4 names MCAST-VPN as
// one of its own examples of a typed address family, and the ruling on which
// types ze implements lives in the MVPN plugin
// (internal/component/bgp/plugins/nlri/mvpn, nlritype.Recognizer).
//
// Slices alias data. A malformed entry returns the partially-parsed result plus
// a non-nil error; the caller decides whether to use it.
func SplitMVPN(data []byte, addPath bool) ([][]byte, error) {
	return splitTypeLength(data, addPath, 2, 1, "MVPN")
}
