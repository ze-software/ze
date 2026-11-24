// Design: docs/architecture/wire/nlri-bgpls.md -- BGP-LS and BGP-LS-VPN MP_REACH next-hop framing
// RFC 7752 Section 3.4 -- see rfc/short/rfc7752.md.
// RFC 9552 Section 5.5 -- see rfc/short/rfc9552.md.

package ls_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC9552BGPLSNextHopFramingPositive verifies RFC 7752 Section 3.4 and
// RFC 9552 Section 5.5 MP_REACH_NLRI next-hop encoding and decoding across
// SAFI 71 (4/16/32) and SAFI 72 (12/24/48 with per-address zero RD).
//
// VALIDATES: SAFI 71 and SAFI 72 next-hop lengths, zero-RD framing, reserved
// octet, and round-trip address recovery.
// PREVENTS: BGP-LS or BGP-LS-VPN emitting or decoding malformed next-hop wire layouts.
func TestRFC9552BGPLSNextHopFramingPositive(t *testing.T) {
	// RFC requirement: RFC7752-3.4-1 positive -- BGP-LS (SAFI 71) and BGP-LS-VPN (SAFI 72) encode and decode RFC 4760 MP_REACH next-hop fields in 4/16/32 and 12/24/48 per-address zero-RD forms (§3.4)
	// RFC requirement: RFC9552-5.5-1 positive -- BGP-LS (SAFI 71) and BGP-LS-VPN (SAFI 72) encode and decode RFC 4760 MP_REACH next-hop fields in 4/16/32 and 12/24/48 per-address zero-RD forms (§5.5)
	t.Parallel()
	for _, tc := range lsNextHopWireCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mp := attribute.NewMPReachNLRI(attribute.AFI(tc.family.AFI),
				attribute.SAFI(tc.family.SAFI), tc.nextHops, tc.nlri)
			buf := make([]byte, mp.Len())
			n := mp.WriteTo(buf, 0)
			require.Equal(t, mp.Len(), n)
			checkLSNextHopWire(t, buf[:n], tc)

			back, err := attribute.ParseMPReachNLRI(buf[:n])
			require.NoError(t, err)
			assert.Equal(t, tc.nextHops, back.NextHops.Slice())
			assert.Equal(t, tc.nlri, back.NLRI)
		})
	}
}

// TestRFC9552BGPLSNextHopFramingRejectsInvalidLengths verifies that malformed
// next-hop field widths (including the non-standard 40-octet single-RD pair for
// SAFI 72 and unprefixed/wrong widths for SAFI 71 and SAFI 72) are rejected.
//
// VALIDATES: ParseMPReachNLRI and MPNextHopProfile.AdmitsNextHopOctets reject
// invalid BGP-LS and BGP-LS-VPN next-hop lengths.
// PREVENTS: malformed or non-standard 40-octet SAFI 72 next-hop layouts being accepted.
func TestRFC9552BGPLSNextHopFramingRejectsInvalidLengths(t *testing.T) {
	// RFC requirement: RFC7752-3.4-1 negative -- invalid BGP-LS (SAFI 71) and BGP-LS-VPN (SAFI 72) next-hop field lengths (including 40-octet single-RD pairs and unprefixed SAFI 72 widths) are rejected (§3.4)
	// RFC requirement: RFC9552-5.5-1 negative -- invalid BGP-LS (SAFI 71) and BGP-LS-VPN (SAFI 72) next-hop field lengths (including 40-octet single-RD pairs and unprefixed SAFI 72 widths) are rejected (§5.5)
	t.Parallel()
	cases := []struct {
		name   string
		family family.Family
		nhLen  int
	}{
		{"ls72_reject_4_unprefixed_v4", ls.BGPLSVPNFamily, 4},
		{"ls72_reject_8_rd_only", ls.BGPLSVPNFamily, 8},
		{"ls72_reject_16_unprefixed_v6", ls.BGPLSVPNFamily, 16},
		{"ls72_reject_32_unprefixed_pair", ls.BGPLSVPNFamily, 32},
		{"ls72_reject_40_single_rd_pair", ls.BGPLSVPNFamily, 40},
		{"ls71_reject_8", ls.BGPLSFamily, 8},
		{"ls71_reject_12_rd_framed", ls.BGPLSFamily, 12},
		{"ls71_reject_24_rd_framed", ls.BGPLSFamily, 24},
		{"ls71_reject_48_rd_framed", ls.BGPLSFamily, 48},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prof := attribute.MPNextHopProfile(attribute.AFI(tc.family.AFI), attribute.SAFI(tc.family.SAFI))
			assert.False(t, prof.AdmitsNextHopOctets(tc.nhLen), "profile must reject width %d", tc.nhLen)
			assert.False(t, prof.AdmitsNextHopOctets(0), "profile must reject zero width")

			raw := make([]byte, 5+tc.nhLen)
			raw[0], raw[1] = 0x40, 0x04
			raw[2] = byte(tc.family.SAFI)
			raw[3] = byte(tc.nhLen)
			_, err := attribute.ParseMPReachNLRI(raw)
			require.ErrorIs(t, err, attribute.ErrInvalidNextHopLen)
		})
	}

	// Control: confirm valid SAFI 71 and SAFI 72 widths remain admitted so the
	// negative cases cannot pass via blanket rejection.
	p71 := attribute.MPNextHopProfile(attribute.AFI(ls.BGPLSFamily.AFI), attribute.SAFI(ls.BGPLSFamily.SAFI))
	for _, w := range []int{4, 16, 32} {
		assert.True(t, p71.AdmitsNextHopOctets(w), "SAFI 71 must admit width %d", w)
	}
	p72 := attribute.MPNextHopProfile(attribute.AFI(ls.BGPLSVPNFamily.AFI), attribute.SAFI(ls.BGPLSVPNFamily.SAFI))
	for _, w := range []int{12, 24, 48} {
		assert.True(t, p72.AdmitsNextHopOctets(w), "SAFI 72 must admit width %d", w)
	}
	// Ensure valid single IPv4 SAFI 72 decode still succeeds.
	valid := make([]byte, 5+12)
	valid[0], valid[1], valid[2], valid[3] = 0x40, 0x04, byte(ls.BGPLSVPNFamily.SAFI), 12
	copy(valid[4+8:4+12], netip.MustParseAddr("192.0.2.1").AsSlice())
	parsed, err := attribute.ParseMPReachNLRI(valid)
	require.NoError(t, err)
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("192.0.2.1")}, parsed.NextHops.Slice())
}
