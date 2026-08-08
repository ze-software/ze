package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// TestBuildRIBRouteUpdate_RefusesANextHopWithNoWireForm covers the RIB-replay
// rail's half of the MP_REACH next-hop guard.
//
// The rail reaches the wire through announceAttrs.add -> WriteToWithContext ->
// WriteTo, never through CheckedWriteTo, so attribute.ValidateNextHops was never
// asked. Until the guard below existed it was protected only by an accident: the
// length arithmetic disagreed with the write for the zero netip.Addr (Len said 26,
// WriteTo wrote 10), and announceAttrs.add refuses a plan whose size query and
// write disagree. Deriving both from netip.Addr.AsSlice made them agree at zero,
// which silenced the accidental refusal and left the rail emitting an MP_REACH
// whose Length of Next Hop Network Address octet is 0x00.
//
// Measured on this rail before the guard existed, for an IPv6 unicast route whose
// next hop is the zero netip.Addr. The whole attribute block it returned:
//
//	40 01 01 00                          ORIGIN = IGP
//	40 02 06 02 01 00 00 fd e8           AS_PATH = [65000]
//	80 0e 0a 00 02 01 00 00 20 20 01 0d b8
//	         └ MP_REACH value: AFI=0x0002 SAFI=0x01 NHLen=0x00
//	           Reserved=0x00 NLRI=20 20 01 0d b8 (2001:db8::/32)
//
// attribute.ValidNextHopLens admits no zero length for any AFI/SAFI pair, and
// validateMPReachNextHop (message/rfc7606.go) turns exactly that octet into
// RFC7606ActionSessionReset, so the rail was trading a dropped route for a reset
// session. The IPv4 arm was refused even then, but by the length check and under
// the oversize line, telling the operator to reduce attributes on a route whose
// attributes were fine; it now has its own cause.
//
// VALIDATES: an IPv6 route whose next hop has no wire form produces no UPDATE at
// all, and an announceable one still builds with a 16-octet next-hop field.
// PREVENTS: the queued/initial-sync rail emitting a Length of Next Hop Network
// Address octet of 0x00 (attribute.ErrUnencodableNextHop).
//
// RFC requirement: RFC4760-3-2 negative -- the Length of Next Hop Network Address
// field is what identifies the next hop's network-layer protocol, and a zero there
// identifies none. This drives the RIB-replay entry point rather than the
// attribute, because that rail is the one with no checked write between it and the
// socket (internal/component/bgp/reactor/peer_rib_routes.go, buildRIBRouteUpdate).
func TestBuildRIBRouteUpdate_RefusesANextHopWithNoWireForm(t *testing.T) {
	build := func(t *testing.T, fam family.Family, prefix string, nextHop netip.Addr) *message.Update {
		t.Helper()
		route := rib.NewRoute(nlri.NewINET(fam, netip.MustParsePrefix(prefix), 0), nextHop, nil)
		return buildRIBRouteUpdate(make([]byte, message.MaxMsgLen), route, 65000,
			false /*eBGP*/, true /*asn4*/, false /*addPath*/)
	}

	t.Run("ipv6 unicast is refused", func(t *testing.T) {
		assert.Nil(t, build(t, family.IPv6Unicast, "2001:db8::/32", netip.Addr{}),
			"no UPDATE may be produced for a next hop with no wire form")
	})

	t.Run("ipv4 unicast is refused", func(t *testing.T) {
		// RFC 4271 Section 5.1.3 makes NEXT_HOP well-known mandatory, so the IPv4
		// remedy is a refusal too: this rail builds over an EMPTY base, so skipping
		// the contribution would emit an UPDATE with no NEXT_HOP at all, which RFC
		// 7606 Section 3(d) makes treat-as-withdraw at the receiver.
		assert.Nil(t, build(t, family.IPv4Unicast, "10.0.0.0/24", netip.Addr{}),
			"no UPDATE may be produced for a next hop with no wire form")
	})

	t.Run("ipv6 unicast with a real next hop is built", func(t *testing.T) {
		update := build(t, family.IPv6Unicast, "2001:db8::/32", netip.MustParseAddr("2001:db8::1"))
		require.NotNil(t, update)

		_, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
		require.True(t, found, "MP_REACH_NLRI must be present")
		require.GreaterOrEqual(t, len(value), 4)
		assert.Equal(t, byte(16), value[3], "Length of Next Hop Network Address")

		parsed, err := attribute.ParseMPReachNLRI(value)
		require.NoError(t, err)
		require.NoError(t, parsed.ValidateNextHops())
	})

	t.Run("ipv4 unicast with a real next hop is built", func(t *testing.T) {
		update := build(t, family.IPv4Unicast, "10.0.0.0/24", netip.MustParseAddr("192.0.2.1"))
		require.NotNil(t, update)

		_, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
		require.True(t, found, "NEXT_HOP must be present")
		assert.Equal(t, []byte{192, 0, 2, 1}, value)
	})
}
