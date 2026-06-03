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
