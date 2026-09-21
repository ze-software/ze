package wireu

import (
	"net/netip"
	"testing"
)

// RFC 4684 Section 4: for the Route Target membership family "The Next Hop field of
// MP_REACH_NLRI attribute shall be interpreted as an IPv4 address whenever the length
// of NextHop address is 4 octets, and as a IPv6 address whenever the length of the
// NextHop address is 16 octets". MPReachWire.NextHop decides by the length alone.

// rtcReach builds an MP_REACH_NLRI value for AFI 1, SAFI 132 (RTC) with the given next
// hop and one all-zero RTC NLRI (length 0: the default route target).
func rtcReach(nextHop []byte) MPReachWire {
	v := []byte{0x00, 0x01, 132, byte(len(nextHop))}
	v = append(v, nextHop...)
	return append(v, 0x00, 0x00)
}

// TestRFC4684RTCNextHopByLength pins the address family of the RTC next hop.
//
// VALIDATES: RFC4684-4-1, a 4-octet next hop is IPv4 and a 16-octet one is IPv6, on
// the RTC family whose AFI says nothing about the next hop.
// PREVENTS: reading the next hop's family from the AFI, which is always 1 for RTC.
func TestRFC4684RTCNextHopByLength(t *testing.T) {
	t.Run("4 octets is IPv4", func(t *testing.T) {
		// RFC requirement: RFC4684-4-1 positive -- a 4-octet RTC next hop decodes as the IPv4 address it spells (§4).
		// RFC requirement: RFC4684-4-1 negative -- a 4-octet RTC next hop is never decoded as an IPv6 address (§4).
		got := rtcReach([]byte{192, 0, 2, 1}).NextHop()
		if want := netip.MustParseAddr("192.0.2.1"); got != want {
			t.Fatalf("NextHop() = %v, want %v", got, want)
		}
		if got.Is6() {
			t.Fatalf("a 4-octet next hop decoded as IPv6: %v", got)
		}
	})

	t.Run("16 octets is IPv6", func(t *testing.T) {
		// RFC requirement: RFC4684-4-1 positive -- a 16-octet RTC next hop decodes as the IPv6 address it spells, under the same AFI 1 (§4).
		// RFC requirement: RFC4684-4-1 negative -- a 16-octet RTC next hop is never decoded as an IPv4 address (§4).
		want := netip.MustParseAddr("2001:db8::1")
		got := rtcReach(want.AsSlice()).NextHop()
		if got != want {
			t.Fatalf("NextHop() = %v, want %v", got, want)
		}
		if got.Is4() {
			t.Fatalf("a 16-octet next hop decoded as IPv4: %v", got)
		}
	})
}
