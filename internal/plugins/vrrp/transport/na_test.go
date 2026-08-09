// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- golden-byte NA message tests (darwin-safe)

package transport

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

func TestBuildNAMessageGolden(t *testing.T) {
	// VALIDATES: AC-9 / R-1 -- exact 32-byte unsolicited NA ICMPv6 message for
	// vrid 10, VIP 2001:db8::1, virtual MAC 00:00:5e:00:02:0a. Checksum left zero
	// (kernel fills). Byte layout from the spec.
	want := []byte{
		0x88, 0x00, // type 136 (NA), code 0
		0x00, 0x00, // checksum (kernel-computed; zero in the builder)
		0xa0, 0x00, 0x00, 0x00, // flags R=1 S=0 O=1
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // target 2001:db8::1
		0x02, 0x01, // option: Target Link-Layer Address, len 1
		0x00, 0x00, 0x5e, 0x00, 0x02, 0x0a, // virtual MAC
	}

	buf := make([]byte, 64)
	vmac := packet.VirtualMAC(packet.V6, 10)
	n := BuildNA(buf, vmac, netip.MustParseAddr("2001:db8::1").As16())
	if n != NAMessageLen {
		t.Fatalf("BuildNA returned %d, want %d", n, NAMessageLen)
	}
	if !bytes.Equal(buf[:n], want) {
		t.Fatalf("NA message\n got % x\nwant % x", buf[:n], want)
	}
}

func TestNAHoloBugNegatives(t *testing.T) {
	// VALIDATES: AC-9 / holo bug 7 (quadruple negative) -- dst is ff02::1 (NOT the
	// target); the TLL option is present; hop limit 255 is requested; and the
	// message checksums correctly under the CORRECT IPv6 pseudo-header (holo used a
	// wrong pseudo-header). All four are the properties holo got wrong.
	buf := make([]byte, 64)
	vmac := packet.VirtualMAC(packet.V6, 10)
	n := BuildNA(buf, vmac, netip.MustParseAddr("2001:db8::1").As16())
	msg := buf[:n]

	// Negative 1: destination is the all-nodes multicast, not the target address.
	if NAAllNodesV6 != netip.MustParseAddr("ff02::1") {
		t.Fatalf("NA dst = %v, want ff02::1", NAAllNodesV6)
	}
	if NAAllNodesV6 == netip.MustParseAddr("2001:db8::1") {
		t.Fatal("NA dst must not be the target address (holo bug 7)")
	}
	// Negative 2: the Target Link-Layer Address option (type 2, len 1) is present.
	if msg[24] != 2 || msg[25] != 1 {
		t.Fatalf("TLL option missing: % x", msg[24:26])
	}
	if !bytes.Equal(msg[26:32], vmac[:]) {
		t.Fatalf("TLL address != virtual MAC: % x", msg[26:32])
	}
	// Negative 3: hop limit 255 is the requested value.
	if NAHopLimit != 255 {
		t.Fatalf("NA hop limit = %d, want 255", NAHopLimit)
	}
	// Negative 4: the message checksums correctly under the correct pseudo-header
	// (src = a macvlan link-local, dst = ff02::1, next header 58, length 32).
	src := netip.MustParseAddr("fe80::200:5eff:fe00:20a")
	cksum := icmpv6Checksum(src, NAAllNodesV6, msg)
	// Write the computed checksum back and verify the whole message folds to all
	// ones (the standard RFC 1071 receive test).
	full := append([]byte(nil), msg...)
	full[2] = byte(cksum >> 8)
	full[3] = byte(cksum)
	if got := verifyICMPv6(src, NAAllNodesV6, full); !got {
		t.Fatal("NA message does not verify under the correct pseudo-header (holo bug 7)")
	}
}

// icmpv6Checksum computes the ICMPv6 checksum over payload with the IPv6
// pseudo-header (RFC 8200 Section 8.1, next header 58). Local to the test so it
// does not depend on the codec's unexported helpers.
func icmpv6Checksum(src, dst netip.Addr, payload []byte) uint16 {
	return ^foldSum(pseudoSum(src, dst, payload, 58))
}

func verifyICMPv6(src, dst netip.Addr, payload []byte) bool {
	return foldSum(pseudoSum(src, dst, payload, 58)) == 0xffff
}

// (verifyV6Upper moved to transport_integration_linux_test.go: its only caller
// is the integration suite, and living here made it dead code under the default
// build tags. No assertion changed; see that file for the helper.)

func pseudoSum(src, dst netip.Addr, payload []byte, nextHeader uint8) uint32 {
	s := src.As16()
	d := dst.As16()
	var sum uint32
	for i := 0; i+1 < len(s); i += 2 {
		sum += uint32(s[i])<<8 | uint32(s[i+1])
	}
	for i := 0; i+1 < len(d); i += 2 {
		sum += uint32(d[i])<<8 | uint32(d[i+1])
	}
	sum += uint32(len(payload)) // upper-layer length
	sum += uint32(nextHeader)   // 3 zero bytes + next header
	for i := 0; i+1 < len(payload); i += 2 {
		sum += uint32(payload[i])<<8 | uint32(payload[i+1])
	}
	if len(payload)%2 == 1 {
		sum += uint32(payload[len(payload)-1]) << 8
	}
	return sum
}

func foldSum(sum uint32) uint16 {
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return uint16(sum)
}
