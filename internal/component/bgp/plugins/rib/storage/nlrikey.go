// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// Related: familyrib.go -- uses NLRIKey as map key

package storage

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
