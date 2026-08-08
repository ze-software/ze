package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
)

// VALIDATES: the single-route announce rail refuses a next hop it cannot encode,
// instead of panicking (IPv4 branch) or filling the declared octets with an
// address that names a different host (IPv6 branch).
// PREVENTS: netip.Addr.As4 panicking inside writeNextHopAttr, and MP_REACH_NLRI
// reaching a peer with ::ffff:a.b.c.d or :: in the field RFC 2545 Section 3
// reserves for the global IPv6 address of the next hop.

// v6NLRIDb81 is 2001:db8:1::/64 on the wire: prefix length 0x40 followed by the
// eight significant octets.
var v6NLRIDb81 = []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00}

// TestSendAnnounceRefusesUnusableIPv4NextHop drives the announce rail from its
// entry point with a next hop that cannot fill the four octets the NEXT_HOP
// attribute declares.
//
// RFC 4271 Section 5.1.3 gives NEXT_HOP a fixed length of four octets holding an
// IPv4 address. Before this guard writeNextHopAttr (reactor_wire.go) wrote the
// length octet 4 and then called netip.Addr.As4, which panics for the zero Addr
// and for any IPv6 address that is not IPv4-mapped, so the daemon crashed on the
// branch that was meant to encode the attribute.
func TestSendAnnounceRefusesUnusableIPv4NextHop(t *testing.T) {
	cases := []struct {
		name    string
		nextHop netip.Addr
	}{
		{"unset", netip.Addr{}},
		{"ipv6", netip.MustParseAddr("2001:db8::1")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer, conn := newAnnouncePeer(t, "::1")
			route := bgptypes.RouteSpec{
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: bgptypes.NewNextHopExplicit(tc.nextHop),
			}

			err := peer.SendAnnounce(route, 65000)

			require.ErrorIs(t, err, ErrNextHopUnencodable,
				"RFC 4271 Section 5.1.3: a NEXT_HOP that cannot fill its four declared octets is refused, not written")
			assert.Empty(t, conn.written(), "a refused announce puts no bytes on the wire")
		})
	}
}

// TestSendAnnounceRefusesUnusableIPv6NextHop is the case a peer ACCEPTS, which is
// why it is the more expensive of the two.
//
// RFC 2545 Section 3 names the first sixteen octets of the Next Hop field "the
// global IPv6 address of the next hop". netip.Addr.As16 renders 192.0.2.1 as
// ::ffff:192.0.2.1 and the zero Addr as ::, and both fill the declared sixteen
// octets. The message is therefore well formed, the peer raises no NOTIFICATION,
// and it installs a route toward a host that does not exist.
//
// RFC requirement: RFC2545-3-1 negative -- an address that is not the global IPv6
// address of the next hop never reaches that field.
func TestSendAnnounceRefusesUnusableIPv6NextHop(t *testing.T) {
	cases := []struct {
		name     string
		nextHop  netip.Addr
		wrongEnc string // what the unguarded encoder put in the 16-octet slot
	}{
		{"ipv4", netip.MustParseAddr("192.0.2.1"), "::ffff:192.0.2.1"},
		{"unset", netip.Addr{}, "::"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer, conn := newAnnouncePeer(t, "::1")
			route := bgptypes.RouteSpec{
				Prefix:  netip.MustParsePrefix("2001:db8:1::/64"),
				NextHop: bgptypes.NewNextHopExplicit(tc.nextHop),
			}

			err := peer.SendAnnounce(route, 65000)

			require.ErrorIs(t, err, ErrNextHopUnencodable,
				"RFC 2545 Section 3: the global IPv6 address of the next hop is not something the encoder may invent")
			assert.NotContains(t, string(conn.written()), string(mpReachIPv6Attr(t, v6NLRIDb81, tc.wrongEnc)),
				"a peer accepts this attribute and installs an unusable route")
			assert.Empty(t, conn.written(), "a refused announce puts no bytes on the wire")
		})
	}
}

// TestSessionSendAnnounceRefusesNonLinkLocalSecondAddress covers the other half
// of the same field, from the exported entry point that takes it as a parameter.
//
// Peer.SendAnnounce cannot reach this case: linkLocalNextHopFor (link_scope.go)
// returns the zero Addr for everything that is not link-local unicast.
// Session.SendAnnounce takes the address from its caller, so the guard has to sit
// where the octets are written.
//
// RFC requirement: RFC2545-3-1 negative -- the second address of the 32-octet form
// is "the link-local IPv6 address of the next hop", so nothing else may fill it.
//
// RFC requirement: RFC2545-3-2 negative -- the length octet 32 is not written for
// a second address the section does not permit there.
func TestSessionSendAnnounceRefusesNonLinkLocalSecondAddress(t *testing.T) {
	peer, conn := newAnnouncePeer(t, "::1")
	peer.mu.RLock()
	session := peer.session
	peer.mu.RUnlock()

	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("2001:db8:1::/64"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	err := session.SendAnnounce(route, netip.MustParseAddr("2001:db8::2"), 65000, false, true, false)

	require.ErrorIs(t, err, ErrNextHopUnencodable,
		"RFC 2545 Section 3: only a link-local address belongs in the second slot")
	assert.NotContains(t, string(conn.written()), string(mpReachIPv6Attr(t, v6NLRIDb81, "2001:db8::1", "2001:db8::2")),
		"a 32-octet field whose second address is global satisfies the length octet and breaks the sentence it encodes")
	assert.Empty(t, conn.written(), "a refused announce puts no bytes on the wire")
}

// TestSessionSendAnnounceAcceptsLinkLocalSecondAddress is the positive polarity of
// the guard above: the form Section 3 does permit still goes out unchanged.
//
// RFC requirement: RFC2545-3-1 positive -- the global IPv6 address of the next hop
// followed by the link-local IPv6 address of the next hop.
//
// RFC requirement: RFC2545-3-2 positive -- the length octet is 32 when the second
// address is included.
func TestSessionSendAnnounceAcceptsLinkLocalSecondAddress(t *testing.T) {
	peer, conn := newAnnouncePeer(t, "::1")
	peer.mu.RLock()
	session := peer.session
	peer.mu.RUnlock()

	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("2001:db8:1::/64"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	require.NoError(t, session.SendAnnounce(route, netip.MustParseAddr("fe80::2"), 65000, false, true, false))

	assert.Contains(t, string(conn.written()), string(mpReachIPv6Attr(t, v6NLRIDb81, "2001:db8::1", "fe80::2")),
		"RFC 2545 Section 3: global address first, link-local second, length octet 0x20")
}
