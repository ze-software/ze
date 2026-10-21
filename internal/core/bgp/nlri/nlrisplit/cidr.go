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
func splitCIDR(data []byte, addPath bool, fn func(nlri []byte) bool) (int, error) {
	return splitByBitLength(data, addPath, maxPrefixBits, fn)
}

// splitVPN is the Splitter for MPLS VPN families (SAFI 128, RFC 4364 Section
// 4.3.4), whose NLRI is [length(1 byte, bits)][label stack][RD:8][prefix]. The
// length counts every one of those bits, so the same walk frames it -- with a
// bound that admits a value above 128, which a prefix alone never reaches but a
// label stack and a Route Distinguisher together always exceed.
func splitVPN(data []byte, addPath bool, fn func(nlri []byte) bool) (int, error) {
	return splitByBitLength(data, addPath, maxLengthOctetBits, fn)
}

// splitByBitLength walks NLRIs framed as [length(1 byte, in bits)][value bytes],
// rejecting a length above maxBits. The walk is bounded by len(data): every
// entry advances the offset by at least the length octet.
func splitByBitLength(data []byte, addPath bool, maxBits int, fn func(nlri []byte) bool) (int, error) {
	head := 0
	if addPath {
		head = 4
	}
	count := 0
	offset := 0
	for offset < len(data) {
		start := offset
		end := bitLengthEntryEnd(data, start, head, maxBits)
		if end > len(data) {
			return count, bitLengthEntryError(data, start, head, maxBits)
		}

		count++
		if fn != nil {
			if !fn(data[start:end]) {
				return count, nil
			}
		}
		offset = end
	}
	return count, nil
}

// bitLengthEntryEnd frames the NLRI that starts at data[start] as
// [path-id(head octets)][length(1 octet, in bits)][value((length+7)/8 octets)]
// and returns the offset just past it. head is 4 under ADD-PATH and 0
// otherwise; start MUST be below len(data). An end past len(data) refuses the
// entry: data ends before its length octet, the length exceeds maxBits, or the
// value runs past data; bitLengthEntryError words which.
//
// It is the one per-entry framing of this layout, shared by splitByBitLength
// and the labeled withdrawal writer (writeWithdrawalField). It returns one
// int and holds no formatting call, so it inlines into splitByBitLength and
// the unicast walk keeps HEAD's branch count (BenchmarkSplitCIDR).
func bitLengthEntryEnd(data []byte, start, head, maxBits int) int {
	at := start + head
	if at >= len(data) {
		return len(data) + 1
	}
	prefixLen := int(data[at])
	if prefixLen > maxBits {
		return len(data) + 1
	}
	return at + 1 + (prefixLen+7)/8
}

// bitLengthEntryError words why bitLengthEntryEnd refused the entry at
// data[start]. It runs only on that refusal, out of the walk's loop.
func bitLengthEntryError(data []byte, start, head, maxBits int) error {
	at := start + head
	if at >= len(data) {
		return fmt.Errorf("nlrisplit: truncated ADD-PATH NLRI at offset %d", start)
	}
	if prefixLen := int(data[at]); prefixLen > maxBits {
		return fmt.Errorf("nlrisplit: invalid prefix length %d (max %d)", prefixLen, maxBits)
	}
	return fmt.Errorf("nlrisplit: NLRI at offset %d extends past data", start)
}
