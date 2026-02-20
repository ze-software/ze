// Design: docs/architecture/wire/nlri.md — MUP NLRI prefix encoding helpers
//
// Shared utilities for building MUP NLRI data. Used by config (loader.go)
// and reactor (reactor.go) to avoid duplicating MUP prefix/TEID encoding.
package bgp_nlri_mup

import "net/netip"

// WriteMUPPrefix writes a MUP prefix into buf at off.
// Format: prefix-length (1 byte) + prefix-address (variable).
func WriteMUPPrefix(buf []byte, off int, prefix netip.Prefix) {
	bits := prefix.Bits()
	addr := prefix.Addr()
	addrBytes := addr.AsSlice()
	prefixBytes := (bits + 7) / 8
	buf[off] = byte(bits) //nolint:gosec // prefix bits bounded 0-128
	copy(buf[off+1:], addrBytes[:prefixBytes])
}

// MUPPrefixLen returns the encoded byte length of a MUP prefix.
// This is 1 (length byte) + ceil(prefix bits / 8).
func MUPPrefixLen(prefix netip.Prefix) int {
	return 1 + (prefix.Bits()+7)/8
}

// TEIDFieldLen returns the encoded byte length for a TEID field.
// Returns 0 if bits <= 0.
func TEIDFieldLen(bits int) int {
	if bits <= 0 {
		return 0
	}
	return (bits + 7) / 8
}
