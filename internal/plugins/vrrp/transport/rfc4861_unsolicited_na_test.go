// RFC 4861 sender obligations on the unsolicited Neighbor Advertisement a
// VRRP Master multicasts.
//
// VALIDATES: the flags word of BuildNA's message carries R=1 and S=0 and zero
// Reserved bits, written over a buffer that held ones (Sections 4.4 and
// 7.2.6).
// PREVENTS: an unsolicited advertisement that confuses Neighbor
// Unreachability Detection, or one that tells neighbors the virtual router is
// a host.

package transport

import (
	"net/netip"
	"testing"
)

// rfc4861NAFlags returns the flags word of the unsolicited NA BuildNA writes
// for 2001:db8::1 over a buffer whose octets all hold 0xff.
func rfc4861NAFlags(t *testing.T) []byte {
	t.Helper()
	buf := make([]byte, NAMessageLen)
	for i := range buf {
		buf[i] = 0xff
	}
	var vmac [6]byte
	if n := BuildNA(buf, vmac, netip.MustParseAddr("2001:db8::1").As16()); n != NAMessageLen {
		t.Fatalf("BuildNA wrote %d octets, want %d", n, NAMessageLen)
	}
	return buf[4:8]
}

// RFC requirement: RFC4861-4.4-1 negative -- the unsolicited multicast NA
// BuildNA writes over a buffer of 0xff octets has the Solicited flag (bit 1 of
// the flags word) clear.
// RFC requirement: RFC4861-7.2.6-2 positive -- that unsolicited NA has its
// Solicited flag set to zero.
func TestRFC4861UnsolicitedNAClearsTheSolicitedFlag(t *testing.T) {
	if flags := rfc4861NAFlags(t); flags[0]&0x40 != 0 {
		t.Errorf("flags octet = %#02x: the Solicited flag is set", flags[0])
	}
}

// RFC requirement: RFC4861-7.2.6-3 positive -- the unsolicited NA a VRRP router
// sends has the Router flag (bit 0 of the flags word) set to one.
func TestRFC4861UnsolicitedNASetsTheRouterFlag(t *testing.T) {
	if flags := rfc4861NAFlags(t); flags[0]&0x80 == 0 {
		t.Errorf("flags octet = %#02x: the Router flag is clear", flags[0])
	}
}

// RFC requirement: RFC4861-4.4-2 positive -- the 29 Reserved bits after the R, S
// and O flags are written as zero over a buffer of 0xff octets.
func TestRFC4861UnsolicitedNAReservedBitsAreZero(t *testing.T) {
	flags := rfc4861NAFlags(t)
	if flags[0]&0x1f != 0 {
		t.Errorf("flags octet = %#02x: a Reserved bit below the R, S and O flags is set", flags[0])
	}
	for i, b := range flags[1:] {
		if b != 0 {
			t.Errorf("Reserved octet %d = %#02x, want 0", i+1, b)
		}
	}
}
