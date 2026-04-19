// Design: plan/design-rib-unified.md -- Phase 3g (per-family NLRI split)
// Related: nlrisplit.go -- Splitter type and registry this splitter plugs into
// Related: register.go -- binds SplitCIDR to IPv4/IPv6 unicast/multicast

package nlrisplit

import (
	"fmt"
)

// maxPrefixBits is the largest legal prefix length across CIDR families
// (IPv6 /128). IPv4 prefixes are bounded at /32 but the wider bound is
// safe because malformed inputs return an error regardless.
const maxPrefixBits = 128

// SplitCIDR is the Splitter for families with [prefix-len(1 byte, bits)]
// [address-bytes((prefix-len+7)/8)] wire NLRIs -- RFC 4271 unicast and
// multicast for IPv4 / IPv6. Under ADD-PATH (RFC 7911) each NLRI is
// prefixed with a 4-byte path-id that is included in the returned slice.
//
// Walks the input twice: the first pass counts NLRIs so the returned
// slice is pre-sized, the second fills it. Slices alias `data` -- callers
// that need to retain bytes must copy. Returns an error when the first
// malformed NLRI is encountered; any successfully-parsed NLRIs before
// that point are still returned so partial processing is possible.
func SplitCIDR(data []byte, addPath bool) ([][]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	count, countErr := countCIDR(data, addPath)
	if count == 0 {
		return nil, countErr
	}

	result := make([][]byte, 0, count)
	offset := 0
	for offset < len(data) {
		start := offset
		var prefixLen, nlriLen int

		if addPath {
			if offset+5 > len(data) {
				return result, fmt.Errorf("nlrisplit: truncated ADD-PATH NLRI at offset %d", offset)
			}
			prefixLen = int(data[offset+4])
			nlriLen = 4 + 1 + (prefixLen+7)/8
		} else {
			prefixLen = int(data[offset])
			nlriLen = 1 + (prefixLen+7)/8
		}

		if prefixLen > maxPrefixBits {
			return result, fmt.Errorf("nlrisplit: invalid prefix length %d (max %d)", prefixLen, maxPrefixBits)
		}
		if start+nlriLen > len(data) {
			return result, fmt.Errorf("nlrisplit: NLRI at offset %d extends past data", start)
		}

		result = append(result, data[start:start+nlriLen])
		offset = start + nlriLen
	}
	return result, nil
}

// countCIDR is a sizing pre-pass; returns the number of NLRIs in a
// well-formed input and any error encountered.
func countCIDR(data []byte, addPath bool) (int, error) {
	count := 0
	offset := 0
	for offset < len(data) {
		var prefixLen, nlriLen int
		if addPath {
			if offset+5 > len(data) {
				return count, fmt.Errorf("nlrisplit: truncated ADD-PATH NLRI at offset %d", offset)
			}
			prefixLen = int(data[offset+4])
			nlriLen = 4 + 1 + (prefixLen+7)/8
		} else {
			prefixLen = int(data[offset])
			nlriLen = 1 + (prefixLen+7)/8
		}
		if prefixLen > maxPrefixBits {
			return count, fmt.Errorf("nlrisplit: invalid prefix length %d (max %d)", prefixLen, maxPrefixBits)
		}
		if offset+nlriLen > len(data) {
			return count, fmt.Errorf("nlrisplit: NLRI at offset %d extends past data", offset)
		}
		count++
		offset += nlriLen
	}
	return count, nil
}
