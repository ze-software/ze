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
			scope:      newLinkScopeFrom(connected, onLink),
			configured: linkLocal,
			nextHop:    onLink,
			want:       linkLocal,
		},
		{
			name:       "next-hop entity off link",
			scope:      newLinkScopeFrom(connected, onLink),
			configured: linkLocal,
			nextHop:    offLink,
			want:       netip.Addr{},
		},
		{
			name:       "peer off link",
			scope:      newLinkScopeFrom(connected, offLink),
			configured: linkLocal,
			nextHop:    onLink,
			want:       netip.Addr{},
		},
		{
			name:       "no link-local configured",
			scope:      newLinkScopeFrom(connected, onLink),
			configured: netip.Addr{},
			nextHop:    onLink,
			want:       netip.Addr{},
		},
		{
			name:       "configured address is not link-local",
			scope:      newLinkScopeFrom(connected, onLink),
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
			scope:      newLinkScopeFrom(append(connected, netip.MustParsePrefix("fe80::/64")), onLink),
			configured: linkLocal,
			nextHop:    netip.MustParseAddr("fe80::beef"),
			want:       netip.Addr{},
		},
		{
			// The second address is an IPv6 link-local one, so it can only follow
			// an IPv6 global address. RFC 8950 carries an IPv4 NLRI behind an IPv6
			// next hop, never the reverse.
			name:       "global next hop is IPv4",
			scope:      newLinkScopeFrom(append(connected, netip.MustParsePrefix("192.0.2.0/24")), onLink),
			configured: linkLocal,
			nextHop:    netip.MustParseAddr("192.0.2.1"),
			want:       netip.Addr{},
		},
		{
			name:       "global next hop unset",
			scope:      newLinkScopeFrom(connected, onLink),
			configured: linkLocal,
			nextHop:    netip.Addr{},
			want:       netip.Addr{},
		},
		{
			name:       "empty connected set",
			scope:      newLinkScopeFrom(nil, onLink),
			configured: linkLocal,
			nextHop:    onLink,
			want:       netip.Addr{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The global names the speaker in every row: these rows drive the
			// Section 3 subnet condition, TestNextHopOwnersClassify drives whose
			// address the global is.
			got := tt.scope.linkLocalNextHop(tt.configured, tt.nextHop, nextHopRouterSpeaker)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNextHopOwnersClassify drives the router classification that decides
// whether the speaker's own Link-Local may follow a global next hop.
//
// VALIDATES: the session endpoint, the configured local address and any held
// interface address name the speaker; every other address, the peer's own
// included, is a third party, and only the speaker class gets the own
// Link-Local from linkLocalNextHop.
// PREVENTS: the own Link-Local paired with another router's global.
func TestNextHopOwnersClassify(t *testing.T) {
	owners := nextHopOwners{
		endpoint:   netip.MustParseAddr("2001:db8:1::1"),
		configured: netip.MustParseAddr("2001:db8:1::5"),
		held:       []netip.Prefix{netip.MustParsePrefix("2001:db8:2::7/64")},
	}
	scope := newLinkScopeFrom([]netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64"), netip.MustParsePrefix("2001:db8:2::/64")}, netip.MustParseAddr("2001:db8:1::2"))
	linkLocal := netip.MustParseAddr("fe80::1")

	tests := []struct {
		name   string
		owners nextHopOwners
		global string
		want   nextHopRouter
		wantLL bool
	}{
		{"session endpoint", owners, "2001:db8:1::1", nextHopRouterSpeaker, true},
		{"configured local address", owners, "2001:db8:1::5", nextHopRouterSpeaker, true},
		{"address on another interface", owners, "2001:db8:2::7", nextHopRouterSpeaker, false},
		{"peer's own address", owners, "2001:db8:1::2", nextHopRouterThirdParty, false},
		{"another router on the shared link", owners, "2001:db8:1::99", nextHopRouterThirdParty, false},
		{"unset global", owners, "", nextHopRouterUnspecified, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var global netip.Addr
			if tt.global != "" {
				global = netip.MustParseAddr(tt.global)
			}
			router := tt.owners.classify(global)
			assert.Equal(t, tt.want, router)
			got := scope.linkLocalNextHop(linkLocal, global, router)
			if tt.wantLL {
				assert.Equal(t, linkLocal, got, "own Link-Local follows a speaker global on the common subnet")
				return
			}
			assert.False(t, got.IsValid(), "no own Link-Local for a third party or a different subnet")
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

	assert.False(t, peer.linkLocalNextHopFor(nil, netip.MustParseAddr("::1")).IsValid())

	peer.refreshLinkScope()
	assert.Equal(t, netip.MustParseAddr("fe80::1"),
		peer.linkLocalNextHopFor(nil, netip.MustParseAddr("::1")),
		"loopback next hop and loopback peer both sit on the local ::1/128 subnet")
}
