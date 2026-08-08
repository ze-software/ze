package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLinkScopeLinkLocalNextHop drives RFC 2545 Section 3's inclusion condition
// through every combination that decides it.
//
// RFC 2545 Section 3: "The link-local address shall be included in the Next Hop
// field if and only if the BGP speaker shares a common subnet with the entity
// identified by the global IPv6 address carried in the Network Address of Next
// Hop field and the peer the route is being advertised to."
//
// VALIDATES: the link-local is returned only when both halves hold; every other
// row returns the zero Addr, which is the "in all other cases" branch.
func TestLinkScopeLinkLocalNextHop(t *testing.T) {
	connected := []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}
	linkLocal := netip.MustParseAddr("fe80::1")
	onLink := netip.MustParseAddr("2001:db8:1::ffff")
	offLink := netip.MustParseAddr("2001:db8:9::ffff")

	tests := []struct {
		name       string
		scope      *linkScope
		configured netip.Addr
		nextHop    netip.Addr
		want       netip.Addr
	}{
		{
			name:       "both halves hold",
			scope:      &linkScope{connected: connected, peerOnLink: true},
			configured: linkLocal,
			nextHop:    onLink,
			want:       linkLocal,
		},
		{
			name:       "next-hop entity off link",
			scope:      &linkScope{connected: connected, peerOnLink: true},
			configured: linkLocal,
			nextHop:    offLink,
			want:       netip.Addr{},
		},
		{
			name:       "peer off link",
			scope:      &linkScope{connected: connected, peerOnLink: false},
			configured: linkLocal,
			nextHop:    onLink,
			want:       netip.Addr{},
		},
		{
			name:       "no link-local configured",
			scope:      &linkScope{connected: connected, peerOnLink: true},
			configured: netip.Addr{},
			nextHop:    onLink,
			want:       netip.Addr{},
		},
		{
			name:       "configured address is not link-local",
			scope:      &linkScope{connected: connected, peerOnLink: true},
			configured: netip.MustParseAddr("2001:db8:1::2"),
			nextHop:    onLink,
			want:       netip.Addr{},
		},
		{
			name:       "nil scope reads no interface table",
			scope:      nil,
			configured: linkLocal,
			nextHop:    onLink,
			want:       netip.Addr{},
		},
		{
			// RFC 2545 Section 3 names the FIRST address "the global IPv6 address
			// of the next hop". A link-local there is the shape the section
			// excludes, and appending a second address to it would leave a
			// conformant length octet over a non-conformant field.
			name:       "global next hop is itself link-local",
			scope:      &linkScope{connected: append(connected, netip.MustParsePrefix("fe80::/64")), peerOnLink: true},
			configured: linkLocal,
			nextHop:    netip.MustParseAddr("fe80::beef"),
			want:       netip.Addr{},
		},
		{
			// The second address is an IPv6 link-local one, so it can only follow
			// an IPv6 global address. RFC 8950 carries an IPv4 NLRI behind an IPv6
			// next hop, never the reverse.
			name:       "global next hop is IPv4",
			scope:      &linkScope{connected: append(connected, netip.MustParsePrefix("192.0.2.0/24")), peerOnLink: true},
			configured: linkLocal,
			nextHop:    netip.MustParseAddr("192.0.2.1"),
			want:       netip.Addr{},
		},
		{
			name:       "global next hop unset",
			scope:      &linkScope{connected: connected, peerOnLink: true},
			configured: linkLocal,
			nextHop:    netip.Addr{},
			want:       netip.Addr{},
		},
		{
			name:       "empty connected set",
			scope:      &linkScope{connected: nil, peerOnLink: true},
			configured: linkLocal,
			nextHop:    onLink,
			want:       netip.Addr{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.linkLocalNextHop(tt.configured, tt.nextHop)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNewLinkScopeAnswersPeerHalfFromHost verifies newLinkScope reads the host
// interface table and settles the peer half of the Section 3 condition.
//
// VALIDATES: the loopback peer is on link; an address on no local subnet is not.
func TestNewLinkScopeAnswersPeerHalfFromHost(t *testing.T) {
	scope := newLinkScope(netip.MustParseAddr("::1"))
	require.NotNil(t, scope)
	require.NotEmpty(t, scope.connected, "host reports no interface addresses")
	assert.True(t, scope.peerOnLink, "loopback peer should share the loopback subnet")

	off := newLinkScope(netip.MustParseAddr("2001:db8:dead:beef::1"))
	assert.False(t, off.peerOnLink, "documentation prefix should not be locally connected")
}

// TestPeerLinkLocalNextHopForBeforeRefresh verifies a peer whose link scope has
// never been refreshed appends no link-local.
//
// VALIDATES: the unproven condition denies. Section 3's "if and only if" makes an
// unread interface table a false condition, not a permissive one.
func TestPeerLinkLocalNextHopForBeforeRefresh(t *testing.T) {
	peer := NewPeer(&PeerSettings{
		Address:      netip.MustParseAddr("::1"),
		LocalAddress: netip.MustParseAddr("::1"),
		LinkLocal:    netip.MustParseAddr("fe80::1"),
	})

	assert.False(t, peer.linkLocalNextHopFor(netip.MustParseAddr("::1")).IsValid())

	peer.refreshLinkScope()
	assert.Equal(t, netip.MustParseAddr("fe80::1"),
		peer.linkLocalNextHopFor(netip.MustParseAddr("::1")),
		"loopback next hop and loopback peer both sit on the local ::1/128 subnet")
}
