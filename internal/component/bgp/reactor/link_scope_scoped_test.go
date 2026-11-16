// Design: docs/architecture/bgp/interface-event-reactions.md -- scoped peer subnet refresh
// Related: link_scope.go -- immutable comparison snapshot, separate from transport
// Related: link_scope_fixture_test.go -- established session with scoped TCP endpoint

package reactor

import (
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLinkScopeScopedPeerMembership checks both membership facts from the real
// constructor. The captured comparison address must match an unscoped wire
// address without changing the address value the caller uses for transport.
func TestLinkScopeScopedPeerMembership(t *testing.T) {
	tests := []struct {
		name      string
		connected []netip.Prefix
		peer      netip.Addr
		wire      netip.Addr
		onLink    bool
		joint     bool
	}{
		{"scoped-link-local", []netip.Prefix{netip.MustParsePrefix("fe80::1/64")}, netip.MustParseAddr("fe80::2%if0"), netip.MustParseAddr("fe80::9"), true, true},
		{"scoped-global", []netip.Prefix{netip.MustParsePrefix("2001:db8:1::1/64")}, netip.MustParseAddr("2001:db8:1::2%if0"), netip.MustParseAddr("2001:db8:1::9"), true, true},
		{"unscoped-link-local", []netip.Prefix{netip.MustParsePrefix("fe80::1/64")}, netip.MustParseAddr("fe80::2"), netip.MustParseAddr("fe80::9"), true, true},
		{"native-ipv4", []netip.Prefix{netip.MustParsePrefix("192.0.2.1/24")}, netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.9"), true, true},
		{"mapped-ipv4-peer", []netip.Prefix{netip.MustParsePrefix("192.0.2.1/24")}, netip.MustParseAddr("::ffff:192.0.2.2"), netip.MustParseAddr("192.0.2.9"), true, true},
		{"scoped-peer-outside", []netip.Prefix{netip.MustParsePrefix("fe80::1/128")}, netip.MustParseAddr("fe80::2%if0"), netip.MustParseAddr("fe80::1"), false, false},
		{"scoped-peer-wire-outside", []netip.Prefix{netip.MustParsePrefix("fe80::1/64")}, netip.MustParseAddr("fe80::2%if0"), netip.MustParseAddr("2001:db8:1::9"), true, false},
		{"separate-connected-prefixes", []netip.Prefix{netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("2001:db8:1::1/64")}, netip.MustParseAddr("fe80::2%if0"), netip.MustParseAddr("2001:db8:1::9"), true, false},
		{"no-connected-prefixes", nil, netip.MustParseAddr("fe80::2%if0"), netip.MustParseAddr("fe80::9"), false, false},
		{"invalid-peer", []netip.Prefix{netip.MustParsePrefix("fe80::1/64")}, netip.Addr{}, netip.MustParseAddr("fe80::9"), false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scope := newLinkScopeWith(test.connected, neighborView{}, test.peer)
			assert.Equal(t, test.peer.Unmap().WithZone(""), scope.peer,
				"the snapshot holds address identity rather than a TCP zone")
			assert.Equal(t, test.onLink, scope.peerOnLink)
			assert.Equal(t, test.joint, scope.sharesNextHopSubnet(test.wire),
				"the peer and wire address must share one connected prefix")
		})
	}
}

// TestLinkScopeScopedPeerRefresh follows addition, removal and restoration of a
// link-local prefix using the established scoped-session fixture. Only subnet
// comparisons change: settings and the actual remote TCP endpoint retain if0.
func TestLinkScopeScopedPeerRefresh(t *testing.T) {
	topology := ls71PairScopedTopology(true)
	f := ls71PairFixture(t, ls71PairModes()[3], topology)
	conn, ok := f.destination.session.conn.(*ls71PairScopedConn)
	require.True(t, ok)
	remote := *conn.remote
	settingsAddr := f.destination.Settings().Address
	sessionAddr := f.destination.session.settings.Address
	neighborSnapshot := f.destination.llScope.Load().neighbors
	states := []struct {
		name      string
		connected []netip.Prefix
		onLink    bool
	}{
		{"prefix-present", topology.connected, true},
		{"prefix-removed", []netip.Prefix{netip.MustParsePrefix("2001:db8:1::1/64")}, false},
		{"prefix-restored", topology.connected, true},
	}
	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			f.destination.refreshLinkScopeWith(state.connected, neighborSnapshot)
			scope := f.destination.llScope.Load()
			require.NotNil(t, scope)
			assert.Equal(t, netip.MustParseAddr("fe80::2"), scope.peer)
			assert.Equal(t, state.onLink, scope.peerOnLink)
			assert.Equal(t, state.onLink, destOnLink(f.destination))
			assert.Equal(t, state.onLink, scope.sharesNextHopSubnet(netip.MustParseAddr("fe80::9")))
			assert.False(t, scope.sharesNextHopSubnet(netip.MustParseAddr("2001:db8:1::9")),
				"separate peer and wire prefixes do not establish joint membership")
			assert.Equal(t, settingsAddr, f.destination.Settings().Address)
			assert.Equal(t, sessionAddr, f.destination.session.settings.Address)
			assert.Equal(t, "if0", f.destination.Settings().Address.Zone())
			assert.Equal(t, "if0", f.destination.session.settings.Address.Zone())
			assert.Same(t, conn, f.destination.session.conn)
			assert.Equal(t, remote, *conn.remote)
			endpoint, ok := conn.RemoteAddr().(*net.TCPAddr)
			require.True(t, ok)
			assert.Equal(t, "if0", endpoint.Zone)
			assert.Equal(t, settingsAddr, endpoint.AddrPort().Addr())
		})
	}
}
