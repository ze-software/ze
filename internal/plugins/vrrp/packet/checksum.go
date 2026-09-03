// RFC: rfc/short/rfc9568.md -- Section 5.2.8 (v3 checksum, IPv4 message-only) + rx RFC 5798 dual-accept
// RFC: rfc/short/rfc3768.md -- Section 5.3.7 (v2 checksum, no pseudo-header)
// Design: (RFC 8200 Section 8.1 -- IPv6 upper-layer checksum pseudo-header)
//
// checksum.go implements the RFC 1071 one's-complement checksum used by VRRP,
// the family-specific pseudo-headers, the tx backfill (FillChecksum) and the rx
// verification. For v3/IPv4, tx sends the RFC 5798 pseudo-header form because
// that is what the deployed base (keepalived and every pre-RFC-9568 peer)
// requires on the wire; rx dual-accepts both that form and the RFC 9568
// message-only form so a strict-RFC-9568 peer still interoperates. See
// FillChecksum for the interop evidence.
package packet

import "net/netip"

// onesComplementSum accumulates the 16-bit one's-complement sum of data into a
// 32-bit accumulator seeded with initial (a pseudo-header partial sum, or 0).
// RFC 1071.
func onesComplementSum(data []byte, initial uint32) uint32 {
	sum := initial
	i := 0
	for ; i+1 < len(data); i += 2 {
		sum += uint32(data[i])<<8 | uint32(data[i+1])
	}
	if i < len(data) {
		// Odd trailing byte is the high byte of a final 16-bit word.
		sum += uint32(data[i]) << 8
	}
	return sum
}

// fold reduces a 32-bit accumulator to 16 bits by adding the carries back in.
func fold(sum uint32) uint16 {
	for sum>>16 != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return uint16(sum)
}

// checksum16 returns the RFC 1071 checksum (one's complement of the folded sum)
// of data over the pseudo-header partial sum initial.
func checksum16(data []byte, initial uint32) uint16 {
	return ^fold(onesComplementSum(data, initial))
}

// verifyChecksumSum reports whether data (which INCLUDES its checksum field)
// folds to all-ones over the pseudo-header partial sum initial -- the standard
// RFC 1071 receive test that avoids mutating the buffer.
func verifyChecksumSum(data []byte, initial uint32) bool {
	return fold(onesComplementSum(data, initial)) == 0xFFFF
}

// pseudoSumV6 builds the RFC 8200 Section 8.1 IPv6 pseudo-header partial sum:
// src (16B) + dst (16B) + upper-layer length (4B) + zero (3B) + next header 112.
func pseudoSumV6(src, dst netip.Addr, upperLen int) uint32 {
	s := src.As16()
	d := dst.As16()
	sum := onesComplementSum(s[:], 0)
	sum = onesComplementSum(d[:], sum)
	sum += uint32(upperLen)    // upper-layer length (< 2^16 for VRRP)
	sum += uint32(ProtoNumber) // 3 zero bytes + next header 112 => word 0x0070
	return sum
}

// pseudoSumV4Legacy builds the RFC 5798 IPv4 pseudo-header partial sum: src
// (4B) + dst (4B) + zero (1B) + protocol 112 (1B) + VRRP length (2B). RFC 9568
// Section 5.2.8 no longer prepends this for IPv4, which is what "legacy" names,
// but ze uses it on BOTH sides. FillChecksum computes every v3/IPv4 advert ze
// transmits with it (the interop evidence is stated there), and verifyReceived
// tries it first on rx because the deployed base -- pre-#2324 keepalived, FRR
// pre-2022 and uvrrpd -- sends this form.
func pseudoSumV4Legacy(src, dst netip.Addr, vrrpLen int) uint32 {
	s := src.As4()
	d := dst.As4()
	sum := onesComplementSum(s[:], 0)
	sum = onesComplementSum(d[:], sum)
	sum += uint32(ProtoNumber) // zero byte + protocol 112 => word 0x0070
	sum += uint32(vrrpLen)     // VRRP length
	return sum
}

// FillChecksum computes the version/family-correct VRRP checksum over
// buf[off:off+n] and backfills bytes off+6..7 (skip-and-backfill,
// ai/rules/performance.md). The version is read from the message; the family
// is inferred from src (an IPv6 source selects the RFC 8200 pseudo-header).
//
// RFC 9568 Section 5.2.8: for IPv4 "the checksum computation only includes the
// VRRP message" (message-only); RFC 3768 Section 5.3.7: v2 is message-only too.
func FillChecksum(buf []byte, off, n int, src, dst netip.Addr) {
	msg := buf[off : off+n]
	// The checksum field MUST be zero during computation.
	msg[6] = 0
	msg[7] = 0

	var initial uint32
	switch {
	case msg[0]>>4 == VersionV3 && src.Is6():
		// RFC 9568 Section 5.2.8: IPv6 includes the RFC 8200 pseudo-header.
		initial = pseudoSumV6(src, dst, n)
	case msg[0]>>4 == VersionV3:
		// v3/IPv4. RFC 9568 Section 5.2.8 defines this as message-only (no
		// pseudo-header), but that form does NOT interoperate. keepalived 2.3.1
		// -- its own advert captured on the wire 2026-07-15, checksum 0xa102,
		// which matches the pseudo-header sum and not the message-only sum
		// 0x448e -- computes and REQUIRES the IPv4 pseudo-header, and rejects
		// message-only as "Invalid VRRPv3 checksum". The rest of the RFC 5798
		// deployed base (older FRR, hardware VRRP, uvrrpd) does the same. For a
		// first-hop-redundancy feature, interoperating with the installed base
		// outranks tracking the newest RFC's clarification, so ze transmits the
		// pseudo-header form. ze's rx (verifyReceived) still dual-accepts both,
		// so a future strict-RFC-9568 peer also interoperates. Proven by the
		// keepalived QEMU interop lab (internal/le/qemu/vrrp_keepalived_linux.go).
		initial = pseudoSumV4Legacy(src, dst, n)
	}
	// v2 has no pseudo-header (RFC 3768 Section 5.3.7): initial stays 0.

	cksum := checksum16(msg, initial)
	msg[6] = byte(cksum >> 8)
	msg[7] = byte(cksum)
}

// verifyReceived checks the received VRRP checksum over the FULL payload with
// the actual src/dst from RxMeta. It returns whether the packet matched only
// the RFC 9568 message-only sum (msgOnly) rather than the pseudo-header form ze
// and the deployed base use, and whether it verified at all (ok).
//
// v3/IPv4 dual-accepts both forms. The RFC 5798 pseudo-header sum is PRIMARY
// because that is what ze transmits (FillChecksum) and what keepalived and every
// pre-RFC-9568 peer sends; the RFC 9568 message-only sum is the accepted-and-
// flagged fallback for a strict-RFC-9568 peer. The ordering was reversed here
// until the keepalived interop lab proved the pseudo-header form is the one on
// the wire in practice.
func verifyReceived(payload []byte, version, family uint8, src, dst netip.Addr) (msgOnly, ok bool) {
	switch {
	case version == VersionV2:
		// RFC 3768 Section 5.3.7: entire VRRP message, no pseudo-header.
		return false, verifyChecksumSum(payload, 0)
	case family == V6:
		// RFC 9568 Section 5.2.8 / RFC 8200 Section 8.1: IPv6 pseudo-header.
		return false, verifyChecksumSum(payload, pseudoSumV6(src, dst, len(payload)))
	default:
		// RFC 5798 pseudo-header sum is primary: it is ze's own tx form and the
		// deployed base's, so a normal packet matches here and is NOT flagged.
		if verifyChecksumSum(payload, pseudoSumV4Legacy(src, dst, len(payload))) {
			return false, true
		}
		// RFC 9568 message-only fallback: accepted, but flagged so an operator
		// can see a strict-RFC-9568 peer on the segment.
		if verifyChecksumSum(payload, 0) {
			return true, true
		}
		return false, false
	}
}
