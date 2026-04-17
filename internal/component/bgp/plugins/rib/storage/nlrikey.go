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
// Returns [prefix-len:1][prefix-bytes] format. Allocates; for zero-alloc
// iteration, use PrefixToNLRIInto with a caller-provided buffer.
func PrefixToNLRI(pfx netip.Prefix) []byte {
	buf := make([]byte, 17) // max: 1 (prefix-len) + 16 (IPv6 addr)
	return PrefixToNLRIInto(pfx, buf)
}

// PrefixToNLRIInto writes NLRI wire bytes for pfx into buf and returns the
// slice header. The returned slice aliases buf -- callers must not retain it
// past the next write into buf. buf must be at least 17 bytes (1 prefix-len +
// 16 for IPv6); returns nil if too small.
//
// Used on hot iteration paths to avoid per-entry allocation: the caller
// declares `var buf [17]byte` on the stack, calls this in a loop, and the
// trie iterator yields nlriBytes without touching the heap.
func PrefixToNLRIInto(pfx netip.Prefix, buf []byte) []byte {
	bits := pfx.Bits()
	if bits < 0 {
		// Zero-value / invalid Prefix -- refuse rather than write the
		// sentinel byte 0xFF that would otherwise encode to a malformed
		// NLRI (prefix-length 255, zero bytes).
		return nil
	}
	byteLen := (bits + 7) / 8
	needed := 1 + byteLen
	if len(buf) < needed {
		return nil
	}
	buf[0] = byte(bits)
	addr := pfx.Addr()
	if addr.Is4() {
		raw := addr.As4()
		copy(buf[1:needed], raw[:byteLen])
	} else {
		raw := addr.As16()
		copy(buf[1:needed], raw[:byteLen])
	}
	return buf[:needed]
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
