// Design: docs/architecture/config/syntax.md — prefix expansion for route splitting
// Overview: loader.go — reactor loading and creation
// Related: peers.go — peer extraction that calls parseSplitLen/expandPrefix

package bgpconfig

import (
	"encoding/binary"
	"net/netip"
	"strconv"
	"strings"
)

// parseSplitLen parses a split specification like "/25" and returns the prefix length.
// Returns 0 if no split or invalid format.
func parseSplitLen(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = strings.TrimPrefix(s, "/")
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 128 {
		return 0
	}
	return n
}

// expandPrefix expands a prefix into more-specific prefixes with the given length.
// For example, 10.0.0.0/24 expanded to /25 produces two /25 prefixes.
// Returns the original prefix unchanged if targetLen is invalid.
// Note: route.splitPrefix has the same logic but returns an error for invalid input.
func expandPrefix(prefix netip.Prefix, targetLen int) []netip.Prefix {
	sourceBits := prefix.Bits()

	// Validate target length
	maxBits := 32
	if prefix.Addr().Is6() {
		maxBits = 128
	}

	if targetLen <= sourceBits || targetLen > maxBits {
		return []netip.Prefix{prefix}
	}

	// Calculate number of resulting prefixes: 2^(targetLen - sourceBits)
	numPrefixes := 1 << (targetLen - sourceBits)
	result := make([]netip.Prefix, 0, numPrefixes)

	baseAddr := prefix.Addr()
	for i := range numPrefixes {
		newAddr := addToAddr(baseAddr, i, targetLen)
		result = append(result, netip.PrefixFrom(newAddr, targetLen))
	}

	return result
}

// addToAddr adds an offset to an address at the given prefix boundary.
// Identical to route.addToAddr — not consolidated because config must not import plugin packages.
func addToAddr(addr netip.Addr, offset, prefixLen int) netip.Addr {
	if offset == 0 {
		return addr
	}

	maxBits := 32
	if addr.Is6() {
		maxBits = 128
	}
	shift := maxBits - prefixLen

	if addr.Is4() {
		v4 := addr.As4()
		val := uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
		val += uint32(offset) << shift //nolint:gosec // offset is bounded
		return netip.AddrFrom4([4]byte{byte(val >> 24), byte(val >> 16), byte(val >> 8), byte(val)})
	}

	// IPv6
	v6 := addr.As16()
	hi := uint64(v6[0])<<56 | uint64(v6[1])<<48 | uint64(v6[2])<<40 | uint64(v6[3])<<32 |
		uint64(v6[4])<<24 | uint64(v6[5])<<16 | uint64(v6[6])<<8 | uint64(v6[7])
	lo := uint64(v6[8])<<56 | uint64(v6[9])<<48 | uint64(v6[10])<<40 | uint64(v6[11])<<32 |
		uint64(v6[12])<<24 | uint64(v6[13])<<16 | uint64(v6[14])<<8 | uint64(v6[15])

	if shift >= 64 {
		hi += uint64(offset) << (shift - 64) //nolint:gosec // offset is bounded
	} else {
		addLo := uint64(offset) << shift //nolint:gosec // offset is bounded
		newLo := lo + addLo
		if newLo < lo {
			hi++
		}
		lo = newLo
	}

	var b16 [16]byte
	binary.BigEndian.PutUint64(b16[:8], hi)
	binary.BigEndian.PutUint64(b16[8:], lo)
	return netip.AddrFrom16(b16)
}
