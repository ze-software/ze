// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI splitting
// RFC: rfc/short/rfc9830.md -- SR-Policy NLRI wire format
// Related: types.go -- SRPolicy type and family constants
// Related: register.go -- splitter registration via init()

package srpolicy

import "fmt"

// SplitSRPolicy is the Splitter for SR-Policy NLRIs (SAFI 73, RFC 9830).
//
// Each NLRI is framed as [length-bits:1][body:length-bits/8].
// The body is fixed-size per AFI: 12 bytes for IPv4, 24 bytes for IPv6.
// ADD-PATH is not used with SR-Policy.
//
// Slices alias `data` (zero-copy). A malformed entry returns
// partially-parsed results plus a non-nil error.
func SplitSRPolicy(data []byte, _ bool) ([][]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var result [][]byte
	offset := 0
	for offset < len(data) {
		lenBits := int(data[offset])
		if lenBits == 0 {
			return result, fmt.Errorf("srpolicy: zero-length NLRI at offset %d", offset)
		}
		if lenBits%8 != 0 {
			return result, fmt.Errorf("srpolicy: length %d bits is not byte-aligned at offset %d", lenBits, offset)
		}
		bodyBytes := lenBits / 8
		nlriLen := 1 + bodyBytes
		if offset+nlriLen > len(data) {
			return result, fmt.Errorf("srpolicy: NLRI at offset %d extends past data (need %d, have %d)", offset, nlriLen, len(data)-offset)
		}
		result = append(result, data[offset:offset+nlriLen])
		offset += nlriLen
	}
	return result, nil
}
