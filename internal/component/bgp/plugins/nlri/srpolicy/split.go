// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI splitting
// RFC: rfc/short/rfc9830.md -- SR-Policy NLRI wire format
// Related: types.go -- SRPolicy type and family constants
// Related: register.go -- splitter registration via init()

package srpolicy

import "fmt"

// SplitSRPolicy is the nlrisplit.Splitter for SR-Policy NLRIs (SAFI 73,
// RFC 9830).
//
// Each NLRI is framed as [length-bits:1][body:length-bits/8].
// The body is fixed-size per AFI: 12 bytes for IPv4, 24 bytes for IPv6.
// ADD-PATH is not used with SR-Policy.
//
// It visits each NLRI in wire order, calls fn for each when fn is non-nil, and
// returns how many it visited. Slices alias `data` (zero-copy), so fn copies
// what it retains. A malformed entry returns the count of the entries visited
// before it plus a non-nil error.
//
// The walk is bounded by len(data): a zero length is rejected below, so every
// entry advances the offset.
func SplitSRPolicy(data []byte, _ bool, fn func(nlri []byte)) (int, error) {
	count := 0
	offset := 0
	for offset < len(data) {
		lenBits := int(data[offset])
		if lenBits == 0 {
			return count, fmt.Errorf("srpolicy: zero-length NLRI at offset %d", offset)
		}
		if lenBits%8 != 0 {
			return count, fmt.Errorf("srpolicy: length %d bits is not byte-aligned at offset %d", lenBits, offset)
		}
		bodyBytes := lenBits / 8
		nlriLen := 1 + bodyBytes
		if offset+nlriLen > len(data) {
			return count, fmt.Errorf("srpolicy: NLRI at offset %d extends past data (need %d, have %d)", offset, nlriLen, len(data)-offset)
		}
		if fn != nil {
			fn(data[offset : offset+nlriLen])
		}
		count++
		offset += nlriLen
	}
	return count, nil
}
