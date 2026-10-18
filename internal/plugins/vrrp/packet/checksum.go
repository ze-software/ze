// RFC: rfc/short/rfc9568.md -- Section 5.2.8 (v3 checksum, IPv4 message-only), Section 7.2 (compute on transmit)
// RFC: rfc/short/rfc5798.md -- Section 5.2.8 (v3 checksum, IPv4 pseudo-header, the opt-in compatible mode)
// RFC: rfc/short/rfc3768.md -- Section 5.3.7 (v2 checksum, no pseudo-header)
// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- checksum mode
//
// checksum.go implements the RFC 1071 one's-complement checksum used by VRRP,
// the family-specific pseudo-headers, the tx backfill (FillChecksum) and the rx
// verification. For v3/IPv4 the form is the group's ChecksumMode: the RFC 9568
// message-only form by default, or the RFC 5798 pseudo-header form a group opts
// into with `checksum rfc5798-compatible` to interoperate with keepalived and
// the pre-RFC-9568 deployed base. Each side uses only the configured form.
package packet

import "net/netip"

// ChecksumMode selects the VRRPv3 IPv4 checksum form of one group. It changes
// nothing for VRRPv2 (RFC 3768, always message-only) or for IPv6 (RFC 8200
// pseudo-header in both RFC 5798 and RFC 9568).
type ChecksumMode uint8

const (
	// ChecksumUnspecified is the zero value. No group carries it, because the
	// config parser always sets one of the two named modes. FillChecksum
	// panics on it and verifyReceived refuses every v3/IPv4 packet under it,
	// so a producer that forgot the mode cannot put a guessed form on the wire.
	ChecksumUnspecified ChecksumMode = iota
	// ChecksumRFC9568 is the default: the IPv4 checksum covers the VRRP
	// message alone (RFC 9568 Section 5.2.8).
	ChecksumRFC9568
	// ChecksumRFC5798Compatible prepends the IPv4 pseudo-header as RFC 5798
	// Section 5.2.8 did, which keepalived and the pre-RFC-9568 base require.
	ChecksumRFC5798Compatible
)

// String names the mode with the config enum spelling.
func (m ChecksumMode) String() string {
	switch m {
	case ChecksumRFC9568:
		return "rfc9568"
	case ChecksumRFC5798Compatible:
		return "rfc5798-compatible"
	case ChecksumUnspecified:
		return "unspecified"
	}
	return "unknown"
}

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

// pseudoSumV4 builds the RFC 5798 IPv4 pseudo-header partial sum, used only in
// ChecksumRFC5798Compatible mode:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                  IPv4 Source Address (0-3)                    |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                IPv4 Destination Address (4-7)                 |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|   Zero (8)    |Protocol 112(9)|      VRRP Length (10-11)      |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// RFC 5798 Section 5.2.8: "The checksum is the 16-bit one's complement of the
// one's complement sum of the entire VRRP message starting with the version
// field and a "pseudo-header" as defined in Section 8.1 of [RFC2460]."
func pseudoSumV4(src, dst netip.Addr, vrrpLen int) uint32 {
	s := src.As4()
	d := dst.As4()
	sum := onesComplementSum(s[:], 0)
	sum = onesComplementSum(d[:], sum)
	sum += uint32(ProtoNumber) // zero byte + protocol 112 => word 0x0070
	sum += uint32(vrrpLen)     // VRRP length
	return sum
}

// pseudoSumFor returns the pseudo-header partial sum the checksum of a v3
// message covers, for the family and the group's mode. ok is false only for
// v3/IPv4 under ChecksumUnspecified, which no config produces.
//
// The VRRPv3 message the sum always covers (RFC 9568 Section 5.1), with the
// Checksum field at octets 6-7:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|Version| Type  | Virtual Rtr ID|   Priority    |Count IPvX Addr|  0-3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|(rsvd) |     Max Adver Int     |          Checksum             |  4-7
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                     IPvX Address(es)                          |  8..
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func pseudoSumFor(src, dst netip.Addr, n int, family uint8, mode ChecksumMode) (uint32, bool) {
	if family == V6 {
		// RFC 9568 Section 5.2.8: "For the IPv6 address family, the checksum
		// calculation also includes a prepended "pseudo-header", as defined in
		// Section 8.1 of [RFC8200]."
		return pseudoSumV6(src, dst, n), true
	}
	switch mode {
	case ChecksumRFC9568:
		// RFC 9568 Section 5.2.8: "For the IPv4 address family, the checksum
		// calculation only includes the VRRP message starting with the
		// Version field and ending after the last IPv4 address (refer to
		// Section 5.2)."
		return 0, true
	case ChecksumRFC5798Compatible:
		// RFC 5798 Section 5.2.8 (pseudoSumV4), the form keepalived and the
		// pre-RFC-9568 base send and require.
		return pseudoSumV4(src, dst, n), true
	case ChecksumUnspecified:
		return 0, false
	}
	return 0, false
}

// FillChecksum computes the VRRP checksum over buf[off:off+n] and backfills
// bytes off+6..7 (skip-and-backfill, ai/rules/performance.md). The version is
// read from the message, the family is inferred from src (an IPv6 source
// selects the RFC 8200 pseudo-header), and mode picks the v3/IPv4 form.
//
// RFC 9568 Section 7.2: "The following operations MUST be performed when
// transmitting a VRRP packet: ... Compute the VRRP checksum".
// RFC 9568 Section 5.2.8: "For computing the checksum, the Checksum field is
// set to zero."
func FillChecksum(buf []byte, off, n int, src, dst netip.Addr, mode ChecksumMode) {
	msg := buf[off : off+n]
	msg[6] = 0
	msg[7] = 0

	// RFC 3768 Section 5.3.7: v2 covers the message alone, so initial stays 0.
	var initial uint32
	if msg[0]>>4 == VersionV3 {
		family := V4
		if src.Is6() {
			family = V6
		}
		sum, ok := pseudoSumFor(src, dst, n, family, mode)
		if !ok {
			panic("BUG: vrrp FillChecksum called with ChecksumUnspecified for a v3/IPv4 advertisement")
		}
		initial = sum
	}

	cksum := checksum16(msg, initial)
	msg[6] = byte(cksum >> 8)
	msg[7] = byte(cksum)
}

// verifyReceived checks the received VRRP checksum over the FULL payload with
// the actual src/dst from RxMeta, in the ONE form the group is configured for.
// A v3/IPv4 packet carrying the other form fails: accepting both would keep a
// message-only advertisement whose checksum is wrong whenever its pseudo-header
// sum happened to match, which RFC 9568 Section 5.2.8 does not allow.
func verifyReceived(payload []byte, version, family uint8, src, dst netip.Addr, mode ChecksumMode) bool {
	if version == VersionV2 {
		// RFC 3768 Section 5.3.7: entire VRRP message, no pseudo-header.
		return verifyChecksumSum(payload, 0)
	}
	// RFC 9568 Section 5.2.8
	initial, ok := pseudoSumFor(src, dst, len(payload), family, mode)
	if !ok {
		return false
	}
	return verifyChecksumSum(payload, initial)
}
