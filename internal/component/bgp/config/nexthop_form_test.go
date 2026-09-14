package bgpconfig

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseRouteAttributesAcceptsLinkLocalNextHop drives the config entry point
// for the one next-hop form the session rather than the config decides.
//
// draft-ietf-idr-linklocal-capability Section 3 defines a Next Hop field of 16
// octets holding one IPv6 Link-Local address, which RFC 2545 Section 3 has no
// form for. One static route is advertised to many peers and only a session that
// negotiated capability 77 may carry that form, so this parser keeps the address
// and Peer.resolveNextHop (../reactor/peer.go) answers per peer.
//
// VALIDATES: a static route configured with `next-hop fe80::cafe` parses, and
// the address reaches the attributes unchanged.
// PREVENTS: the config refusing at parse time what a capable session may send,
// which is how ze came to advertise capability 77 and be unable to produce the
// form behind it.
func TestParseRouteAttributesAcceptsLinkLocalNextHop(t *testing.T) {
	attrs, err := ParseRouteAttributes(&StaticRouteConfig{
		Prefix:  netip.MustParsePrefix("2001:db8:1::1/128"),
		NextHop: "fe80::cafe",
		Origin:  "igp",
	})

	require.NoError(t, err)
	assert.Equal(t, netip.MustParseAddr("fe80::cafe"), attrs.NextHop)
}

// TestParseRouteAttributesAcceptsGlobalNextHop is the other side of the guard.
//
// VALIDATES: a global IPv6 next hop, an IPv4 next hop, and `self` all parse.
func TestParseRouteAttributesAcceptsGlobalNextHop(t *testing.T) {
	for _, nextHop := range []string{"2001:db8::ffff", "::1", "192.0.2.1", "self", ""} {
		t.Run(nextHop, func(t *testing.T) {
			_, err := ParseRouteAttributes(&StaticRouteConfig{
				Prefix:  netip.MustParsePrefix("2001:db8:1::1/128"),
				NextHop: nextHop,
				Origin:  "igp",
			})
			assert.NoError(t, err)
		})
	}
}
