// VALIDATES: an MP_REACH_NLRI for VPN-IPv4 (AFI 1, SAFI 128) with an IPv6
// next hop is written in the 24-octet form, and with a link-local address in
// the 48-octet form, each address preceded by an 8-octet Route Distinguisher
// of zero.
// PREVENTS: a writer that leaves the RD octets holding whatever the buffer
// held, or that drops the RD before the link-local address.

package attribute

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC8950VPNIPv6NextHopCarriesZeroRD writes VPN-IPv4 MP_REACH_NLRI
// attributes with an IPv6 next hop and reads the Next Hop field octet by
// octet.
// Method: the buffer is filled with 0xFF before each write, so an RD octet the
// writer does not set reads 0xFF instead of the zero RFC 8950 Section 3
// requires. Two shapes: the global address alone (Length 24: RD, address) and
// the global address followed by the link-local one (Length 48: RD, global,
// RD, link-local). The field is then parsed back to the two addresses.
//
// RFC requirement: RFC8950-3-2 positive -- MPReachNLRI.WriteTo for AFI 1 SAFI 128 with an IPv6 next hop writes Length 24 as a zero 8-octet RD then the address, and with a link-local address Length 48 as zero RD, global, zero RD, link-local, into a buffer pre-filled with 0xFF; parsing returns the same addresses.
func TestRFC8950VPNIPv6NextHopCarriesZeroRD(t *testing.T) {
	t.Parallel()
	global := netip.MustParseAddr("2001:db8::1")
	linkLocal := netip.MustParseAddr("fe80::1")
	zeroRD := make([]byte, RDSize)

	tests := []struct {
		name     string
		hops     []netip.Addr
		wantLen  int
		wantNext [][]byte // the Next Hop field, in order: RD, address, ...
	}{
		{"global only", []netip.Addr{global}, 24, [][]byte{zeroRD, global.AsSlice()}},
		{"global and link-local", []netip.Addr{global, linkLocal}, 48,
			[][]byte{zeroRD, global.AsSlice(), zeroRD, linkLocal.AsSlice()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reach := NewMPReachNLRI(AFIIPv4, SAFIVPN, tt.hops, nil)
			buf := bytes.Repeat([]byte{0xFF}, 128)
			n := reach.WriteTo(buf, 0)

			require.Equal(t, byte(tt.wantLen), buf[3], "Length of Next Hop Address")
			want := bytes.Join(tt.wantNext, nil)
			require.Equal(t, want, buf[4:4+tt.wantLen], "every RD in the Next Hop field must be zero")

			parsed, err := ParseMPReachNLRI(buf[:n])
			require.NoError(t, err)
			require.Equal(t, tt.hops, parsed.NextHops.Slice())
		})
	}
}
