package probe

import (
	"net/netip"
	"testing"
)

// TestBuildICMPEchoChecksum verifies the built packet carries the type, id, and
// sequence in the right offsets and a checksum that makes the full packet sum to
// zero (one's-complement), the standard ICMP validity property.
func TestBuildICMPEchoChecksum(t *testing.T) {
	pkt := BuildICMPEcho(8, 0x1234, 0x0007, []byte("ze-ping"))
	if pkt[0] != 8 {
		t.Fatalf("type = %d, want 8", pkt[0])
	}
	if got := uint16(pkt[4])<<8 | uint16(pkt[5]); got != 0x1234 {
		t.Errorf("id = %#x, want 0x1234", got)
	}
	if got := uint16(pkt[6])<<8 | uint16(pkt[7]); got != 0x0007 {
		t.Errorf("seq = %#x, want 0x0007", got)
	}
	// Verifying checksum: summing all 16-bit words (including the checksum
	// field) in one's complement must yield 0xffff.
	var sum uint32
	for i := 0; i+1 < len(pkt); i += 2 {
		sum += uint32(pkt[i])<<8 | uint32(pkt[i+1])
	}
	if len(pkt)%2 == 1 {
		sum += uint32(pkt[len(pkt)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	if uint16(sum) != 0xffff {
		t.Errorf("checksum folded sum = %#x, want 0xffff", uint16(sum))
	}
}

// TestResolveTargetLiteral verifies an IP literal resolves to itself without DNS.
func TestResolveTargetLiteral(t *testing.T) {
	for _, lit := range []string{"192.0.2.1", "2001:db8::1"} {
		got, err := ResolveTarget(lit)
		if err != nil {
			t.Fatalf("ResolveTarget(%q): %v", lit, err)
		}
		want := netip.MustParseAddr(lit)
		if got != want {
			t.Errorf("ResolveTarget(%q) = %v, want %v", lit, got, want)
		}
	}
}

// checksumOnesFold sums b as 16-bit big-endian words with end-around carry and
// returns the folded 16-bit value. A message whose checksum field is correct
// folds to 0xffff (the RFC 1071 verification property), so this is the mirror of
// the encoder's icmpChecksum used to assert RFC 792's checksum obligation.
func checksumOnesFold(b []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(b[i])<<8 | uint32(b[i+1])
	}
	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	return uint16(sum)
}

// RFC requirement: RFC792-Echo-1 positive -- a ze-built ICMP echo request carries Type 8.
func TestRFC792EchoRequestType(t *testing.T) {
	if got := BuildICMPEcho(8, 0x1234, 7, []byte("ze-ping"))[0]; got != 8 {
		t.Errorf("echo request Type = %d, want 8", got)
	}
}

// RFC requirement: RFC792-Echo-2 positive -- a ze-built ICMP echo request carries Code 0.
func TestRFC792EchoRequestCode(t *testing.T) {
	if got := BuildICMPEcho(8, 0x1234, 7, []byte("ze-ping"))[1]; got != 0 {
		t.Errorf("echo request Code = %d, want 0", got)
	}
}

// RFC requirement: RFC792-Echo-3 positive -- the built echo carries a checksum that makes
// the one's-complement sum over the whole message fold to 0xffff.
func TestRFC792ChecksumValid(t *testing.T) {
	pkt := BuildICMPEcho(8, 0x1234, 7, []byte("ze-ping"))
	if got := checksumOnesFold(pkt); got != 0xffff {
		t.Errorf("checksum fold = %#x, want 0xffff", got)
	}
}

// RFC requirement: RFC792-Echo-3 negative -- altering a message byte after the checksum is
// computed breaks the property, so a corrupted echo does not carry a valid checksum.
func TestRFC792ChecksumRejectsCorruption(t *testing.T) {
	pkt := BuildICMPEcho(8, 0x1234, 7, []byte("ze-ping"))
	pkt[9] ^= 0xff // flip a payload byte; leave the checksum field intact
	if got := checksumOnesFold(pkt); got == 0xffff {
		t.Errorf("corrupted echo still folds to 0xffff; checksum does not detect the change")
	}
}

// RFC requirement: RFC792-Echo-4 positive -- an odd total length is padded with one zero
// octet for the computation, so an odd-length payload still yields a valid checksum.
func TestRFC792ChecksumOddLength(t *testing.T) {
	pkt := BuildICMPEcho(8, 0x1234, 7, []byte("odd")) // 8 + 3 = 11 bytes, odd
	if len(pkt)%2 == 0 {
		t.Fatalf("want an odd-length packet to exercise padding, got len %d", len(pkt))
	}
	if got := checksumOnesFold(pkt); got != 0xffff {
		t.Errorf("odd-length checksum fold = %#x, want 0xffff", got)
	}
}
