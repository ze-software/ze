package bgpconfig

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestParseRouteAttributesRejectsLinkLocalNextHop drives the RFC 2545 Section 3
// next-hop form guard from the config entry point that reaches it, rather than
// from the helper alone.
//
// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop." Ze appends the second
// address itself when the section's condition holds, so a link-local supplied as
// THE next hop has no global address to follow and the config is refused.
//
// VALIDATES: a static route configured with `next-hop fe80::cafe` fails to parse.
// PREVENTS: ze emitting a 16-octet Next Hop field whose only address is
// link-local, which is the shape Section 3 excludes.
func TestParseRouteAttributesRejectsLinkLocalNextHop(t *testing.T) {
	_, err := ParseRouteAttributes(&StaticRouteConfig{
		Prefix:  netip.MustParsePrefix("2001:db8:1::1/128"),
		NextHop: "fe80::cafe",
		Origin:  "igp",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, attribute.ErrLinkLocalNextHop)
	assert.Contains(t, err.Error(), "fe80::cafe")
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
