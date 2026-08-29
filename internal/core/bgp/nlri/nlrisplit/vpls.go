// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
// RFC: rfc/short/rfc4761.md -- VPLS NLRI wire format (Section 3.2.2)
// Related: register.go -- binds SplitVPLS to the l2vpn/vpls family
// Related: nlrisplit.go -- Splitter type and registry this splitter plugs into

package nlrisplit

import (
	"encoding/binary"
	"fmt"
)

// SplitVPLS is the Splitter for the L2VPN VPLS family, SAFI 65 (RFC 4761
// Section 3.2.2). Each NLRI is framed as:
//
//	[length:2][Route Distinguisher:8][VE-ID:2][VE Block Offset:2]
//	[VE Block Size:2][Label Base and control:3]
//
// The length is two octets, not one. A one-octet read takes the high half of
// the length as the whole of it, which frames the section at the wrong offset
// from the first NLRI onwards.
//
// The walk uses only the length field, so an NLRI carrying a body length other
// than the 17 octets Section 3.2.2 defines is still framed and handed on. What
// the body means is the VPLS plugin's business.
//
// Under ADD-PATH (RFC 7911) each NLRI is prefixed with a 4-byte path identifier
// that is included in the returned slice.
//
// Slices alias data. A malformed entry returns the partially-parsed result plus
// a non-nil error.
func SplitVPLS(data []byte, addPath bool) ([][]byte, error) {
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

		if start+head+2 > len(data) {
			return result, fmt.Errorf("nlrisplit: truncated VPLS length at offset %d", start)
		}

		length := int(binary.BigEndian.Uint16(data[start+head : start+head+2]))

		// A zero-length body would leave the walk on the same octets forever.
		if length == 0 {
			return result, fmt.Errorf("nlrisplit: zero-length VPLS NLRI at offset %d", start)
		}

		nlriLen := head + 2 + length
		if start+nlriLen > len(data) {
			return result, fmt.Errorf("nlrisplit: VPLS NLRI at offset %d extends past data (len=%d)", start, length)
		}

		result = append(result, data[start:start+nlriLen])
		offset = start + nlriLen
	}

	return result, nil
}
