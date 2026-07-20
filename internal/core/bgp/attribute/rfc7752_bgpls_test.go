// RFC: rfc/short/rfc7752.md — BGP-LS next-hop encoding (§3.4)
//
// RFC 7752 Section 3.4 defers the next-hop entirely to RFC 4760: a BGP-LS
// MP_REACH_NLRI carries AFI 16388, SAFI 71, the next-hop length, the next-hop
// address, the reserved octet set to zero, and then the Link-State NLRI.

package attribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRFC7752BGPLSNextHopFollowsRFC4760 pins the MP_REACH_NLRI layout ze emits
// for the BGP-LS family. The encoder is family-agnostic (mpnlri.go:154), so the
// obligation is met by the same code path that serves every other family.
//
// VALIDATES: AFI/SAFI/next-hop-length/next-hop/reserved ordering for AFI 16388.
// PREVENTS: a BGP-LS-specific next-hop shortcut diverging from RFC 4760.
func TestRFC7752BGPLSNextHopFollowsRFC4760(t *testing.T) {
	// RFC requirement: RFC7752-3.4-1 positive -- the BGP-LS next-hop is encoded as RFC 4760 Section 3 specifies: length octet, address, then the zero reserved octet before the NLRI (§3.4)
	nlri := []byte{0x00, 0x01, 0x00, 0x09, 0x02, 0, 0, 0, 0, 0, 0, 0, 0}

	t.Run("ipv4 next-hop", func(t *testing.T) {
		mp := NewMPReachNLRI(AFI(16388), SAFI(71), []netip.Addr{netip.MustParseAddr("192.0.2.1")}, nlri)
		buf := make([]byte, mp.Len())
		n := mp.WriteTo(buf, 0)
		require.Equal(t, mp.Len(), n)

		assert.Equal(t, []byte{0x40, 0x04}, buf[0:2], "AFI 16388")
		assert.Equal(t, byte(71), buf[2], "SAFI 71")
		assert.Equal(t, byte(4), buf[3], "next-hop length")
		assert.Equal(t, []byte{192, 0, 2, 1}, buf[4:8], "next-hop address")
		assert.Equal(t, byte(0), buf[8], "RFC 4760 reserved octet is zero")
		assert.Equal(t, nlri, buf[9:n], "NLRI follows the reserved octet")

		back, err := ParseMPReachNLRI(buf[:n])
		require.NoError(t, err)
		assert.Equal(t, AFI(16388), back.AFI)
		assert.Equal(t, SAFI(71), back.SAFI)
		assert.Equal(t, nlri, back.NLRI)
	})

	t.Run("ipv6 next-hop", func(t *testing.T) {
		mp := NewMPReachNLRI(AFI(16388), SAFI(71), []netip.Addr{netip.MustParseAddr("2001:db8::1")}, nlri)
		buf := make([]byte, mp.Len())
		n := mp.WriteTo(buf, 0)
		require.Equal(t, mp.Len(), n)

		assert.Equal(t, byte(16), buf[3], "next-hop length")
		assert.Equal(t, byte(0), buf[20], "RFC 4760 reserved octet is zero")
		assert.Equal(t, nlri, buf[21:n])
	})
}
