// Design: plan/design-rib-unified.md -- Phase 3g (per-family NLRI split)
// Related: register.go -- binds SplitEVPN to AFI L2VPN / SAFI EVPN

package nlrisplit

import "fmt"

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
	if len(data) == 0 {
		return nil, nil
	}
	var result [][]byte
	offset := 0
	for offset < len(data) {
		start := offset
		head := 0
		if addPath {
			head = 4
		}
		// Need at least path-id (if any) + route-type + length byte.
		if start+head+2 > len(data) {
			return result, fmt.Errorf("nlrisplit: truncated EVPN header at offset %d", start)
		}
		length := int(data[start+head+1])
		nlriLen := head + 2 + length
		if start+nlriLen > len(data) {
			return result, fmt.Errorf("nlrisplit: EVPN NLRI at offset %d extends past data (len=%d)", start, length)
		}
		result = append(result, data[start:start+nlriLen])
		offset = start + nlriLen
	}
	return result, nil
}
