// VALIDATES: spec-ospfv3-2-wire AC-15, AC-16 -- the LSA Fletcher checksum is
// computed over lsa[2:length] (LS Age excluded), is non-zero, and a flipped
// covered byte fails; the packet checksum is the IPv6 upper-layer checksum over
// the pseudo-header (src, dst, length, Next Header 89) plus the zero-checksum
// packet, and a wrong source address fails verification.
// PREVENTS: an over-the-packet OSPFv2-style checksum, a checksum that does not
// bind the IPv6 addresses, or LS Age changes invalidating a flooded LSA.

package packet

import (
	"net/netip"
	"testing"
)

func TestOSPFv3LSAChecksum(t *testing.T) {
	wire := encodeLSA(t, sampleRouterLSA(t))
	if readUint16(wire, lsaChecksumOff) == 0 {
		t.Fatalf("LSA checksum was not backfilled (must be non-zero)")
	}
	if !VerifyLSAChecksum(wire) {
		t.Fatalf("VerifyLSAChecksum rejected an encoded LSA: % x", wire)
	}

	// LS Age (bytes 0..1) is excluded from coverage: changing it must not break
	// the checksum.
	mutAge := append([]byte(nil), wire...)
	mutAge[0] ^= 0xff
	mutAge[1] ^= 0xff
	if !VerifyLSAChecksum(mutAge) {
		t.Fatalf("LS Age change invalidated the Fletcher checksum (should be excluded)")
	}

	// A covered byte (the LS Type, offset 2) breaks the checksum.
	mutCovered := append([]byte(nil), wire...)
	mutCovered[2] ^= 0xff
	if VerifyLSAChecksum(mutCovered) {
		t.Fatalf("covered-byte change at the LS Type did not invalidate the Fletcher checksum")
	}

	// A byte deep in the body (a Router-LSA link record, well past the header) must
	// also break it -- the Fletcher must cover the whole LSA, not just the first octets.
	// Pick a byte that is neither 0x00 nor 0xff: the Fletcher-255 checksum treats those
	// two values as equivalent (255 mod 255 == 0), so flipping one is correctly
	// invisible and is not the property under test here.
	mutBody := append([]byte(nil), wire...)
	flipped := false
	for i := LSAHeaderLen; i < len(mutBody); i++ {
		if mutBody[i] != 0x00 && mutBody[i] != 0xff {
			mutBody[i] ^= 0xff
			flipped = true
			break
		}
	}
	if !flipped {
		t.Fatal("no flippable (non-0x00/0xff) body byte found")
	}
	if VerifyLSAChecksum(mutBody) {
		t.Fatalf("body-byte change did not invalidate the Fletcher checksum")
	}
}

func TestOSPFv3PacketChecksum(t *testing.T) {
	src := netip.MustParseAddr("fe80::1")
	dst := netip.MustParseAddr("ff02::5")

	hello := sampleHello(t)
	p := Packet{Header: sampleHeader(t, PacketTypeHello), Hello: &hello}
	pkt := encodePacket(t, p)

	// WriteTo leaves the checksum field zero (the codec cannot know the addresses);
	// transport finalizes it.
	if readUint16(pkt, offChecksum) != 0 {
		t.Fatalf("Packet.WriteTo should leave the checksum field zero, got %#04x", readUint16(pkt, offChecksum))
	}
	cksum := PacketChecksum(src, dst, pkt)
	if cksum == 0 {
		t.Fatalf("PacketChecksum returned 0")
	}
	writeUint16(pkt, offChecksum, cksum)

	if !VerifyPacketChecksum(src, dst, pkt) {
		t.Fatalf("VerifyPacketChecksum rejected a correctly-checksummed packet")
	}

	// A wrong source address changes the pseudo-header and must fail.
	wrongSrc := netip.MustParseAddr("fe80::2")
	if VerifyPacketChecksum(wrongSrc, dst, pkt) {
		t.Fatalf("VerifyPacketChecksum accepted a spoofed source address")
	}
	// A wrong destination must also fail.
	wrongDst := netip.MustParseAddr("ff02::6")
	if VerifyPacketChecksum(src, wrongDst, pkt) {
		t.Fatalf("VerifyPacketChecksum accepted a wrong destination address")
	}
	// A flipped body byte must fail.
	mut := append([]byte(nil), pkt...)
	mut[len(mut)-1] ^= 0xff
	if VerifyPacketChecksum(src, dst, mut) {
		t.Fatalf("VerifyPacketChecksum accepted a corrupted body")
	}
}

func TestOSPFv3FinalizePacketChecksum(t *testing.T) {
	src := netip.MustParseAddr("fe80::1")
	dst := netip.MustParseAddr("ff02::5")

	hello := sampleHello(t)
	p := Packet{Header: sampleHeader(t, PacketTypeHello), Hello: &hello}
	pkt := encodePacket(t, p)

	if readUint16(pkt, offChecksum) != 0 {
		t.Fatalf("Packet.WriteTo should leave the checksum field zero, got %#04x", readUint16(pkt, offChecksum))
	}

	// FinalizePacketChecksum writes the value PacketChecksum computes into the
	// field and returns it.
	want := PacketChecksum(src, dst, pkt)
	got := FinalizePacketChecksum(src, dst, pkt)
	if got != want || got == 0 {
		t.Fatalf("FinalizePacketChecksum returned %#04x, want non-zero %#04x", got, want)
	}
	if onWire := readUint16(pkt, offChecksum); onWire != got {
		t.Fatalf("FinalizePacketChecksum wrote %#04x at offChecksum, returned %#04x", onWire, got)
	}
	if !VerifyPacketChecksum(src, dst, pkt) {
		t.Fatalf("VerifyPacketChecksum rejected a finalized packet")
	}

	// Idempotent: PacketChecksum excludes the checksum field from its own sum, so
	// finalizing an already-finalized packet yields the same value and leaves the
	// field unchanged.
	if again := FinalizePacketChecksum(src, dst, pkt); again != got {
		t.Fatalf("FinalizePacketChecksum is not idempotent: re-ran to %#04x, first %#04x", again, got)
	}
	if !VerifyPacketChecksum(src, dst, pkt) {
		t.Fatalf("VerifyPacketChecksum rejected a re-finalized packet")
	}

	// A sub-header buffer is a no-op returning 0.
	if FinalizePacketChecksum(src, dst, make([]byte, CommonHeaderLen-1)) != 0 {
		t.Fatalf("FinalizePacketChecksum on a sub-header buffer should return 0")
	}
}

func TestOSPFv3PacketChecksumRejectsShort(t *testing.T) {
	src := netip.MustParseAddr("fe80::1")
	dst := netip.MustParseAddr("ff02::5")
	if PacketChecksum(src, dst, make([]byte, CommonHeaderLen-1)) != 0 {
		t.Fatalf("PacketChecksum on a sub-header buffer should return 0")
	}
	if VerifyPacketChecksum(src, dst, make([]byte, CommonHeaderLen-1)) {
		t.Fatalf("VerifyPacketChecksum accepted a sub-header buffer")
	}
}
