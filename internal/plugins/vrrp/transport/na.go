// RFC: rfc/short/rfc9568.md -- Section 6.4.1/6.4.2 + Section 8.2.2 (unsolicited NA)
// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- pure NA message builder (testable on darwin)
// Cites (no short summary file): RFC 4861 (Neighbor Discovery), RFC 3542 (raw ICMPv6)
//
// na.go builds the unsolicited ICMPv6 Neighbor Advertisement VRRP multicasts when
// a router becomes Master, so neighbors repoint their ND caches at the virtual
// MAC. It builds ONLY the ICMPv6 message (no IPv6 header): the raw ICMPv6 sender
// (na_linux.go) supplies the IPv6 header (src = macvlan link-local, dst ff02::1,
// hop limit 255) and the kernel computes the checksum. Reference implementations
// get this wrong (holo bug 7: wrong pseudo-header/dst/TLL/hop-limit); the golden
// test asserts each byte and the correct pseudo-header, so this file has no build
// tag and runs on the native host.

package transport

import "net/netip"

// NAMessageLen is the exact length of the unsolicited NA ICMPv6 message: a 4-byte
// ICMPv6 header, a 4-byte flags/reserved word, a 16-byte target address, and an
// 8-byte Target Link-Layer Address option.
const NAMessageLen = 32

// icmpv6TypeNA is the ICMPv6 type for a Neighbor Advertisement (RFC 4861 Section
// 4.4: type 136).
const icmpv6TypeNA = 136

// naFlags carries R=1 (router), S=0 (unsolicited, not in response to a solicit),
// O=1 (override) in the high three bits of the 32-bit flags word.
// RFC 9568 Section 6.4.1/6.4.2: an unsolicited NA sets R and O and clears S.
const naFlags = 0xa0000000

// optTargetLinkLayer is the ND Target Link-Layer Address option type (RFC 4861
// Section 4.6.1: type 2, length 1 in 8-octet units for a 6-byte Ethernet MAC).
const optTargetLinkLayer = 2

// NAHopLimit is the IPv6 hop limit an ND message MUST carry so receivers accept
// it (RFC 4861 Section 7.1.2: on-link verification requires Hop Limit 255).
// Asserted by the sender and by TestNAHoloBugNegatives (holo bug 13 negative).
const NAHopLimit = 255

// NAAllNodesV6 is the link-local all-nodes multicast destination for an
// unsolicited NA (RFC 4861 Section 7.2.6: ff02::1). Asserted by the golden test
// so the dst is NOT the target address (holo bug 7 negative).
var NAAllNodesV6 = netip.AddrFrom16([16]byte{0xff, 0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01})

// BuildNA writes the 32-byte unsolicited Neighbor Advertisement ICMPv6 message
// announcing vip owned by virtualMAC into buf and returns the number of bytes
// written (NAMessageLen). The caller MUST provide a buffer of at least
// NAMessageLen bytes; BuildNA indexes directly (buffer-first). The checksum field
// is left zero: the raw ICMPv6 socket computes it (RFC 3542 Section 3.1), and the
// golden test recomputes it over the correct pseudo-header.
//
//	RFC 4861 Section 4.4: Neighbor Advertisement is ICMPv6 type 136, code 0.
//	RFC 9568 Section 6.4.1/6.4.2: unsolicited NA sets R and O, clears S, and the
//	Target Link-Layer Address option carries the Virtual Router MAC.
func BuildNA(buf []byte, virtualMAC [6]byte, vip [16]byte) int {
	// ICMPv6 header: type 136, code 0, checksum zero (kernel fills).
	buf[0] = icmpv6TypeNA
	buf[1] = 0x00
	buf[2] = 0x00
	buf[3] = 0x00

	// Flags word: R=1, S=0, O=1 (0xa0000000), remaining bits reserved (zero).
	buf[4] = byte(naFlags >> 24)
	buf[5] = 0x00
	buf[6] = 0x00
	buf[7] = 0x00

	// Target Address = the virtual IPv6 address being announced.
	copy(buf[8:24], vip[:])

	// Target Link-Layer Address option: type 2, length 1 (8 octets), the MAC.
	buf[24] = optTargetLinkLayer
	buf[25] = 0x01
	copy(buf[26:32], virtualMAC[:])

	return NAMessageLen
}
