package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/rib/routetype"
)

// RFC requirement: RFC7999-3.3-2 positive -- both conditions of the sentence
// hold, and the announcement is accepted and honored: on a session that agreed
// to honor BLACKHOLE and is authorized for 10.0.0.0/24, a BLACKHOLE
// announcement of the EQUAL prefix 10.0.0.0/24 becomes a Blackhole route on
// the event-bus rail and in the Loc-RIB.
//
// VALIDATES: RFC 7999 Section 3.3 "covered by an equal or shorter prefix", equal case.
// PREVENTS: a coverage test that accepts only strictly longer prefixes.
func TestRFC7999BlackholeHonoredForTheEqualAuthorizedPrefix(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r, loc := blackholeRIB(t, peer, blackholeConfig{
		communities: agreedBlackhole,
		authorized:  []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})

	change, ok := announce(t, r, peer,
		ipv4Prefix(24, 10, 0, 0),
		blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue))

	require.True(t, ok, "best-path change not detected")
	assert.Equal(t, routetype.Blackhole, change.RouteType)
	assert.Equal(t, routetype.Blackhole, blackholeLocRIBType(t, loc, netip.MustParsePrefix("10.0.0.0/24")))
}

// RFC requirement: RFC7999-3.3-2 negative -- the session agreed to honor
// BLACKHOLE, but the announced prefix is covered by no equal or shorter
// authorized prefix, so the announcement is not honored: with 10.0.0.0/24
// authorized, 198.51.100.1/32 (outside it) and 10.0.0.0/16 (a SHORTER prefix
// that contains it) each stay ordinary routes on both rails.
//
// VALIDATES: RFC 7999 Section 3.3 first condition, with the second condition held true.
// PREVENTS: an agreed session blackholing space it is not authorized for.
func TestRFC7999BlackholeNotHonoredOutsideTheAuthorizedPrefix(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	cases := []struct {
		name string
		nlri []byte
		pfx  netip.Prefix
	}{
		{"outside", ipv4Prefix(32, 198, 51, 100, 1), netip.MustParsePrefix("198.51.100.1/32")},
		{"shorter than the authorization", ipv4Prefix(16, 10, 0), netip.MustParsePrefix("10.0.0.0/16")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, loc := blackholeRIB(t, peer, blackholeConfig{
				communities: agreedBlackhole,
				authorized:  []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
			})

			change, ok := announce(t, r, peer, tc.nlri, blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue))

			require.True(t, ok, "best-path change not detected")
			assert.Equal(t, routetype.Type(0), change.RouteType)
			assert.Equal(t, routetype.Type(0), blackholeLocRIBType(t, loc, tc.pfx))
		})
	}
}
