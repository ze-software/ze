// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
// RFC: rfc/short/rfc9552.md -- Link-State NLRI wire format (Section 5.1)
// Related: register.go -- binds SplitBGPLS to the bgp-ls and bgp-ls-vpn families
// Related: nlrisplit.go -- Splitter type and registry this splitter plugs into

package nlrisplit

import (
	"encoding/binary"
	"fmt"
)

// bgpLSHeaderLen is the Link-State NLRI header: NLRI Type (2 octets) then Total
// NLRI Length (2 octets).
const bgpLSHeaderLen = 4

// SplitBGPLS is the Splitter for the Link-State families, SAFI 71 and SAFI 72
// (RFC 9552 Section 5.1). Each NLRI is framed as:
//
//	[NLRI Type:2][Total NLRI Length:2][value]
//
// SAFI 72 differs only inside the value, where an 8-octet Route Distinguisher
// precedes the Link-State NLRI, so the same length-driven walk frames both.
//
// The walk is NLRI-type-agnostic: it reads the Total NLRI Length alone, so an
// unknown NLRI type is framed like any other and reaches the RIB whole. That is
// what RFC 9552 Section 5.1 requires of a Propagator, which must preserve and
// propagate what it does not understand.
//
// Under ADD-PATH (RFC 7911) each NLRI is prefixed with a 4-byte path identifier
// that is included in the visited slice.
//
// Slices alias data. A malformed entry returns the count of the entries visited
// before it plus a non-nil error.
//
// The walk is bounded by len(data): a zero Total NLRI Length is rejected below,
// so every entry advances the offset.
func SplitBGPLS(data []byte, addPath bool, fn func(nlri []byte)) (int, error) {
	count := 0
	offset := 0
	for offset < len(data) {
		start := offset
		head := 0
		if addPath {
			head = 4
		}

		if start+head+bgpLSHeaderLen > len(data) {
			return count, fmt.Errorf("nlrisplit: truncated BGP-LS NLRI header at offset %d", start)
		}

		lengthAt := start + head + 2
		length := int(binary.BigEndian.Uint16(data[lengthAt : lengthAt+2]))

		// A zero Total NLRI Length carries no descriptors and would leave the walk
		// on the same header forever.
		if length == 0 {
			return count, fmt.Errorf("nlrisplit: zero-length BGP-LS NLRI at offset %d", start)
		}

		nlriLen := head + bgpLSHeaderLen + length
		if start+nlriLen > len(data) {
			return count, fmt.Errorf("nlrisplit: BGP-LS NLRI at offset %d extends past data (len=%d)", start, length)
		}

		if fn != nil {
			fn(data[start : start+nlriLen])
		}
		count++
		offset = start + nlriLen
	}

	return count, nil
}
