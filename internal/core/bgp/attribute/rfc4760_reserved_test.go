package attribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Reserved octet is the one MP_REACH_NLRI field whose conforming value is the
// value a Go slice already holds. A test that encodes into a fresh make([]byte, n)
// therefore asserts nothing about it: delete the write and the assertion still
// reads zero. These tests encode into a buffer that is poisoned first and reused
// after, which is the shape the announce rails use.
//
// announcePlanPool pools each plan with its scratch region
// (internal/component/bgp/reactor/announce_build.go, announcePlan.scratch), release
// does not clear that region, and add writes through
// WriteToWithContext(p.scratch, p.used, ...) at a non-zero offset. So the octet
// under Reserved on a real send is whatever the previous announce left there.
//
// mpReachPoison is declared in mpnlri_nexthop_wire_test.go, which poisons for the
// same reason: one value for one concept across the package.

// TestRFC4760ReservedIsWrittenNotInherited encodes MP_REACH_NLRI over bytes that
// are not zero, so the Reserved octet can only read zero because the encoder put
// it there.
//
// VALIDATES: for every next-hop form the encoder supports, and at a non-zero
// offset, the Reserved octet is 0 and no octet inside the span Len() promised was
// left at the poison value.
//
// PREVENTS: a pooled announce buffer leaking the previous UPDATE's octet into the
// Reserved position, where a receiver reads it as the start of the NLRI field.
//
// RFC requirement: RFC4760-3-1 positive -- RFC 4760 Section 3: "Reserved: A 1 octet
// field that MUST be set to 0, and SHOULD be ignored upon receipt." The octet is
// set to 0 by MPReachNLRI.WriteTo rather than inherited from the buffer: with the
// destination poisoned to 0xAA and the attribute written at a non-zero offset, the
// Reserved position reads 0x00 for every supported next-hop form
// (internal/core/bgp/attribute/mpnlri.go, MPReachNLRI.WriteTo).
func TestRFC4760ReservedIsWrittenNotInherited(t *testing.T) {
	t.Parallel()

	v4 := netip.MustParseAddr("10.0.0.1")
	v6 := netip.MustParseAddr("2001:db8::1")
	ll := netip.MustParseAddr("fe80::1")

	// 2001:db8::/32 and 10.0.0.0/24 in RFC 4271 Section 4.3 prefix encoding.
	nlri6 := []byte{0x20, 0x20, 0x01, 0x0d, 0xb8}
	nlri4 := []byte{0x18, 0x0a, 0x00, 0x00}

	tests := []struct {
		name      string
		afi       AFI
		safi      SAFI
		nextHops  []netip.Addr
		nlri      []byte
		wantNHLen int
	}{
		{"ipv4 unicast", AFIIPv4, SAFIUnicast, []netip.Addr{v4}, nlri4, 4},
		{"ipv6 unicast", AFIIPv6, SAFIUnicast, []netip.Addr{v6}, nlri6, 16},
		// RFC 2545 Section 3: global followed by link-local.
		{"ipv6 global and link-local", AFIIPv6, SAFIUnicast, []netip.Addr{v6, ll}, nlri6, 32},
		// RFC 5549 Section 3: an IPv6 next hop for IPv4 NLRI.
		{"ipv4 over an ipv6 next hop", AFIIPv4, SAFIUnicast, []netip.Addr{v6}, nlri4, 16},
		// RFC 4364 Section 4.3.4: an 8-octet zero RD in front of each address.
		{"vpn ipv4", AFIIPv4, SAFIVPN, []netip.Addr{v4}, nlri4, 12},
		{"vpn ipv6", AFIIPv6, SAFIVPN, []netip.Addr{v6}, nlri6, 24},
	}

	// The announce rails write at p.used, never at zero. An encoder that put the
	// Reserved octet at an absolute position rather than a relative one passes
	// every offset-zero test and fails here.
	const off = 7

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := NewMPReachNLRI(tt.afi, tt.safi, tt.nextHops, tt.nlri)

			buf := make([]byte, 128)
			for i := range buf {
				buf[i] = mpReachPoison
			}
			n := m.WriteTo(buf, off)

			// AFI(2) + SAFI(1) + NHLen(1) + NextHop + Reserved(1) + NLRI.
			wantLen := 2 + 1 + 1 + tt.wantNHLen + 1 + len(tt.nlri)
			require.Equal(t, wantLen, n, "octets written")

			reserved := off + 4 + tt.wantNHLen
			assert.Equal(t, byte(0), buf[reserved],
				"RFC 4760 Section 3: Reserved MUST be set to 0, not inherited from the buffer")
			assert.NotContains(t, buf[off:off+n], byte(mpReachPoison),
				"an octet inside the declared span was never written")

			// A receiver's own arithmetic over the bytes just produced.
			parsed, err := ParseMPReachNLRI(buf[off : off+n])
			require.NoError(t, err)
			assert.Equal(t, tt.nlri, parsed.NLRI, "the NLRI a receiver recovers")
		})
	}
}

// TestRFC4760ReservedSurvivesBufferReuse re-encodes into a buffer that already
// holds a longer attribute, so a previous encode's own octet is what sits under
// the new Reserved position.
//
// Poison proves the writer touched the octet. Reuse proves it wrote the value the
// RFC asks for on the buffer shape production actually uses: the first attribute's
// next hop leaves 0x11 at offset 8, so an encoder that skipped the write would
// emit 0x11 and a receiver would take its prefixes one octet further on.
//
// VALIDATES: after a 16-octet next-hop encode, a 4-octet next-hop encode into the
// same buffer at the same offset reads Reserved 0x00.
//
// PREVENTS: the pooled announce scratch region carrying a previous UPDATE's octet
// into the Reserved field.
//
// RFC requirement: RFC4760-3-1 positive -- the Reserved octet is 0 even when the
// destination octet held a non-zero value left by the previous encode, so the
// "MUST be set to 0" of RFC 4760 Section 3 holds on a reused buffer
// (internal/core/bgp/attribute/mpnlri.go, MPReachNLRI.WriteTo).
func TestRFC4760ReservedSurvivesBufferReuse(t *testing.T) {
	t.Parallel()

	nlri4 := []byte{0x18, 0x0a, 0x00, 0x00}
	buf := make([]byte, 128)

	// First encode: a 16-octet IPv6 next hop for IPv4 NLRI (RFC 5549 Section 3).
	// Its next-hop octets run from offset 4 to offset 19, so its fifth octet, 0x11,
	// lands at offset 8, where the next encode's Reserved octet goes.
	firstHop := netip.MustParseAddr("2001:db8:1122:3344:5566:7788:99aa:bbcc")
	first := NewMPReachNLRI(AFIIPv4, SAFIUnicast, []netip.Addr{firstHop}, nlri4)
	require.Positive(t, first.WriteTo(buf, 0))
	require.Equal(t, byte(0x11), buf[8],
		"the first encode must leave a non-zero octet where the second encode's Reserved lands")

	// Second encode: a 4-octet IPv4 next hop, so Reserved lands at offset 4+4.
	second := NewMPReachNLRI(AFIIPv4, SAFIUnicast, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nlri4)
	n := second.WriteTo(buf, 0)

	assert.Equal(t, byte(0), buf[8],
		"RFC 4760 Section 3: Reserved MUST be set to 0 over a reused buffer, not left as the previous encode's octet")

	parsed, err := ParseMPReachNLRI(buf[:n])
	require.NoError(t, err)
	assert.Equal(t, nlri4, parsed.NLRI, "the NLRI a receiver recovers after the reuse")
}
