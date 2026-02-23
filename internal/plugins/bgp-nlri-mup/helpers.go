// Design: docs/architecture/wire/nlri.md — MUP NLRI prefix encoding helpers
// RFC: rfc/short/draft-ietf-bess-mup-safi.md — MUP SAFI encoding
//
// Shared utilities for building MUP NLRI data. Used by config (loader.go)
// and reactor (reactor.go) to avoid duplicating MUP prefix/TEID encoding.
package bgp_nlri_mup

import "net/netip"

// WriteMUPPrefix writes a MUP prefix into buf at off.
// Format: prefix-length (1 byte) + prefix-address (variable).
func WriteMUPPrefix(buf []byte, off int, prefix netip.Prefix) {
	bits := prefix.Bits()
	prefixBytes := (bits + 7) / 8
	buf[off] = byte(bits) //nolint:gosec // prefix bits bounded 0-128
	writeAddrBytes(buf, off+1, prefix.Addr(), prefixBytes)
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

// addrByteLen returns 4 for IPv4, 16 for IPv6. Zero-alloc.
func addrByteLen(addr netip.Addr) int {
	if addr.Is4() {
		return 4
	}
	return 16
}

// writeAddr writes the full address bytes into buf at off. Zero-alloc.
// Returns bytes written (4 for IPv4, 16 for IPv6).
func writeAddr(buf []byte, off int, addr netip.Addr) int {
	if addr.Is4() {
		a := addr.As4()
		copy(buf[off:], a[:])
		return 4
	}
	a := addr.As16()
	copy(buf[off:], a[:])
	return 16
}

// writeAddrBytes writes the first n bytes of addr into buf at off. Zero-alloc.
func writeAddrBytes(buf []byte, off int, addr netip.Addr, n int) {
	if addr.Is4() {
		a := addr.As4()
		copy(buf[off:], a[:n])
		return
	}
	a := addr.As16()
	copy(buf[off:], a[:n])
}
