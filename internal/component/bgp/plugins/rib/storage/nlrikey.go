// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// Related: familyrib.go -- uses NLRIKey as map key (ADD-PATH) and NLRIToPrefix (BART)

package storage

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// NLRIToPrefix converts NLRI wire bytes to a netip.Prefix for use as BART trie key.
// Wire format: [prefix-len:1][prefix-bytes:0-16].
// Returns ok=false if the bytes are malformed or the family is not IPv4/IPv6.
func NLRIToPrefix(fam family.Family, nlriBytes []byte) (netip.Prefix, bool) {
	if len(nlriBytes) == 0 {
		return netip.Prefix{}, false
	}

	prefixLen := int(nlriBytes[0])
	prefixBytes := nlriBytes[1:]
	expectedBytes := (prefixLen + 7) / 8

	if len(prefixBytes) < expectedBytes {
		return netip.Prefix{}, false
	}

	if fam.AFI == family.AFIIPv4 {
		if prefixLen > 32 {
			return netip.Prefix{}, false
		}
		var ip4 [4]byte
		copy(ip4[:], prefixBytes[:expectedBytes])
		return netip.PrefixFrom(netip.AddrFrom4(ip4), prefixLen), true
	}

	if fam.AFI == family.AFIIPv6 {
		if prefixLen > 128 {
			return netip.Prefix{}, false
		}
		var ip6 [16]byte
		copy(ip6[:], prefixBytes[:expectedBytes])
		return netip.PrefixFrom(netip.AddrFrom16(ip6), prefixLen), true
	}

	// Non-IP families are not stored in the BART trie.
	return netip.Prefix{}, false
}

// PrefixToNLRI converts a netip.Prefix back to NLRI wire bytes.
// Returns [prefix-len:1][prefix-bytes] format.
func PrefixToNLRI(pfx netip.Prefix) []byte {
	bits := pfx.Bits()
	byteLen := (bits + 7) / 8
	addr := pfx.Addr()

	result := make([]byte, 1+byteLen)
	result[0] = byte(bits)

	if addr.Is4() {
		raw := addr.As4()
		copy(result[1:], raw[:byteLen])
	} else {
		raw := addr.As16()
		copy(result[1:], raw[:byteLen])
	}

	return result
}

// NLRIKey is a fixed-size map key for NLRI bytes, eliminating per-route string allocation.
//
// Only simple prefix families (IPv4/IPv6 unicast/multicast) are stored in the RIB,
// guarded by isSimplePrefixFamily(). Max NLRI size for these families:
// ADD-PATH IPv6 /128 = 4 (path-id) + 1 (prefix-len) + 16 (addr) = 21 bytes.
//
// The struct is comparable (all value types) and can be used as a map key directly.
type NLRIKey struct {
	len  uint8
	data [24]byte
}

// NewNLRIKey creates an NLRIKey from raw NLRI wire bytes.
// If nlriBytes is longer than 24, it is truncated (should never happen for unicast).
func NewNLRIKey(nlriBytes []byte) NLRIKey {
	var k NLRIKey
	n := min(len(nlriBytes), 24)
	k.len = uint8(n)
	copy(k.data[:n], nlriBytes)
	return k
}

// Bytes returns the original NLRI bytes (exact length, no trailing zeros).
func (k NLRIKey) Bytes() []byte {
	return k.data[:k.len]
}

// Len returns the length of the NLRI bytes.
func (k NLRIKey) Len() int {
	return int(k.len)
}
