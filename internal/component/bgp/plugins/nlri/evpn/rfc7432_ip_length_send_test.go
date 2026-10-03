package evpn

import (
	"bytes"
	"net/netip"
	"testing"
)

// TestRFC7432MACIPSenderUsesBitLengths reads the sender's length octet directly,
// independently of ParseEVPN's interpretation of that field.
// RFC 7432 Section 10: "For ARP and ND purposes, the IP Address Length field MUST
// be set to 32 for an IPv4 address or 128 for an IPv6 address."
// RFC requirement: RFC7432-10-1 positive -- Type 2 advertisements encode IPv4 length 32 and IPv6 length 128, followed by exactly the corresponding address octets.
func TestRFC7432MACIPSenderUsesBitLengths(t *testing.T) {
	for _, address := range []string{"192.0.2.1", "2001:db8::1"} {
		t.Run(address, func(t *testing.T) {
			ip := netip.MustParseAddr(address)
			// RFC 7432 Section 7.2: retain the mandatory first label field.
			route := NewEVPNType2(RouteDistinguisher{}, [10]byte{}, 0, [6]byte{0, 1, 2, 3, 4, 5}, ip, []uint32{100})
			var wire [128]byte
			n := route.WriteTo(wire[:], 0)
			if wire[31] != byte(ip.BitLen()) {
				t.Fatalf("IP Address Length = %d, want %d bits", wire[31], ip.BitLen())
			}
			addressEnd := 32 + ip.BitLen()/8
			if n != addressEnd+3 {
				t.Fatalf("NLRI length = %d, want %d including Label1", n, addressEnd+3)
			}
			if !bytes.Equal(wire[32:addressEnd], ip.AsSlice()) {
				t.Fatalf("address payload = %x, want %x", wire[32:addressEnd], ip.AsSlice())
			}
			if !bytes.Equal(wire[addressEnd:n], []byte{0, 6, 0x41}) {
				t.Fatalf("Label1 payload = %x, want label 100", wire[addressEnd:n])
			}
		})
	}
}
