// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
// Related: nlrisplit.go -- Splitter type and registry this splitter plugs into
// Related: register.go -- binds SplitCIDR to IPv4/IPv6 unicast/multicast

package nlrisplit

import (
	"fmt"
)

const (
	// maxPrefixBits is the largest legal prefix length across CIDR families
	// (IPv6 /128). IPv4 prefixes are bounded at /32 but the wider bound is
	// safe because malformed inputs return an error regardless.
	maxPrefixBits = 128

	// maxLengthOctetBits is the largest value a one-octet length field can hold,
	// so it bounds nothing on its own. It is the bound for a family whose length
	// counts more than a prefix: RFC 4364 Section 4.3.4 counts the label stack
	// and the 8-octet Route Distinguisher as well, which alone exceed 128.
	maxLengthOctetBits = 255
)

// splitCIDR is the Splitter for families with [prefix-len(1 byte, bits)]
// [address-bytes((prefix-len+7)/8)] wire NLRIs -- RFC 4271 unicast and
// multicast for IPv4 / IPv6. Under ADD-PATH (RFC 7911) each NLRI is
// prefixed with a 4-byte path-id that is included in the visited slice.
//
// Slices alias `data` -- fn must copy what it retains. Returns an error
// when the first malformed NLRI is encountered; the NLRIs before that
// point have already been visited and are counted.
func splitCIDR(data []byte, addPath bool, fn func(nlri []byte)) (int, error) {
	return splitByBitLength(data, addPath, maxPrefixBits, fn)
}

// splitVPN is the Splitter for MPLS VPN families (SAFI 128, RFC 4364 Section
// 4.3.4), whose NLRI is [length(1 byte, bits)][label stack][RD:8][prefix]. The
// length counts every one of those bits, so the same walk frames it -- with a
// bound that admits a value above 128, which a prefix alone never reaches but a
// label stack and a Route Distinguisher together always exceed.
func splitVPN(data []byte, addPath bool, fn func(nlri []byte)) (int, error) {
	return splitByBitLength(data, addPath, maxLengthOctetBits, fn)
}

// splitByBitLength walks NLRIs framed as [length(1 byte, in bits)][value bytes],
// rejecting a length above maxBits. The walk is bounded by len(data): every
// entry advances the offset by at least the length octet.
func splitByBitLength(data []byte, addPath bool, maxBits int, fn func(nlri []byte)) (int, error) {
	count := 0
	offset := 0
	for offset < len(data) {
		start := offset
		var prefixLen, nlriLen int

		if addPath {
			if start+5 > len(data) {
				return count, fmt.Errorf("nlrisplit: truncated ADD-PATH NLRI at offset %d", start)
			}
			prefixLen = int(data[start+4])
			nlriLen = 4 + 1 + (prefixLen+7)/8
		} else {
			prefixLen = int(data[start])
			nlriLen = 1 + (prefixLen+7)/8
		}

		if prefixLen > maxBits {
			return count, fmt.Errorf("nlrisplit: invalid prefix length %d (max %d)", prefixLen, maxBits)
		}
		if start+nlriLen > len(data) {
			return count, fmt.Errorf("nlrisplit: NLRI at offset %d extends past data", start)
		}

		if fn != nil {
			fn(data[start : start+nlriLen])
		}
		count++
		offset = start + nlriLen
	}
	return count, nil
}
