package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/rib/routetype"
)

// TestRFC7999BlackholeHonoredOnAnEqualAuthorizedPrefix announces BLACKHOLE on
// exactly the prefix the neighbor is authorized for.
//
// VALIDATES: RFC 7999 Section 3.3 -- "The announced prefix is covered by an
// equal or shorter prefix that the neighboring network is authorized to
// advertise." The EQUAL case: authorization 10.0.0.0/24, announcement
// 10.0.0.0/24 with 65535:666, honored on both rails.
// PREVENTS: a strictly-shorter comparison that refuses the equal prefix, which
// the /24-over-/32 units cannot see.
//
// RFC requirement: RFC7999-3.3-1 positive -- equal-prefix cover: with BLACKHOLE agreed and 10.0.0.0/24 authorized, an announcement of 10.0.0.0/24 carrying 65535:666 becomes routetype.Blackhole on the best-path change and in the Loc-RIB.
func TestRFC7999BlackholeHonoredOnAnEqualAuthorizedPrefix(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r, loc := blackholeRIB(t, peer, blackholeConfig{
		communities: agreedBlackhole,
		authorized:  []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})

	change, ok := announce(t, r, peer,
		ipv4Prefix(24, 10, 0, 0),
		blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue))

	require.True(t, ok, "best-path change not detected")
	assert.Equal(t, routetype.Blackhole, change.RouteType, "an equal authorized prefix covers the announcement")
	assert.Equal(t, routetype.Blackhole, blackholeLocRIBType(t, loc, netip.MustParsePrefix("10.0.0.0/24")))
}

// TestRFC7999BlackholeRefusedUnderALongerAuthorizedPrefix announces BLACKHOLE
// on a prefix wider than the only authorization that holds its address.
//
// VALIDATES: RFC 7999 Section 3.3 -- the authorized prefix must be "equal or
// shorter". Authorization 10.0.0.0/25 holds the address 10.0.0.0 but is LONGER
// than the announced 10.0.0.0/24, so it does not cover it and the announcement
// is not honored.
// PREVENTS: a cover test that reads only address containment, so a neighbor
// authorized for half a block blackholes the whole block.
//
// RFC requirement: RFC7999-3.3-1 negative -- longer-prefix authorization is no cover: with BLACKHOLE agreed and only 10.0.0.0/25 authorized, an announcement of 10.0.0.0/24 carrying 65535:666 stays route type 0 (forwarding) on the best-path change and in the Loc-RIB.
func TestRFC7999BlackholeRefusedUnderALongerAuthorizedPrefix(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r, loc := blackholeRIB(t, peer, blackholeConfig{
		communities: agreedBlackhole,
		authorized:  []netip.Prefix{netip.MustParsePrefix("10.0.0.0/25")},
	})

	change, ok := announce(t, r, peer,
		ipv4Prefix(24, 10, 0, 0),
		blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue))

	require.True(t, ok, "best-path change not detected")
	assert.Equal(t, routetype.Type(0), change.RouteType, "a /25 authorization does not cover a /24 announcement")
	assert.Equal(t, routetype.Type(0), blackholeLocRIBType(t, loc, netip.MustParsePrefix("10.0.0.0/24")))
}
