// Design: docs/architecture/bgp/interface-event-reactions.md -- listener lifetime
// Related: transport_scope_read_failure_test.go -- immutable interface read faults
// Related: reactor_peers.go -- final claimant removal

package reactor

import (
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScopedTransportReadFailureReleasesFinalAliasedOwner removes both owners
// through RemovePeer while the interface table stays unreadable. The first
// removal must retain the socket; the final removal must close its original key.
func TestScopedTransportReadFailureReleasesFinalAliasedOwner(t *testing.T) {
	r := scopeReadFailureReactor(true)
	first := scopeAliasSettings("if0", "if0", scopeAliasPort)
	first.LocalPort = scopeAliasPort
	second := scopeAliasSettings("if0", "7", DefaultBGPPort)
	second.LocalPort = first.LocalPort
	second.Address = netip.MustParseAddr("fe80::3%if0")
	firstKey, secondKey := first.PeerKey(), second.PeerKey()
	r.peers[firstKey] = NewPeer(first)
	r.peers[secondKey] = NewPeer(second)

	boundAddress := netip.AddrPortFrom(first.LocalAddress, first.LocalPort)
	boundKey := boundAddress.String()
	bound := NewListener(boundKey)
	socket := &scopeReadFailureListener{address: net.TCPAddrFromAddrPort(boundAddress)}
	bound.listener = socket
	r.listeners[boundKey] = bound

	require.NoError(t, r.RemovePeer(first.Address))
	assert.NotContains(t, r.peers, firstKey)
	require.Len(t, r.peers, 1)
	assert.Same(t, second, r.peers[secondKey].Settings())
	require.Len(t, r.listeners, 1)
	assert.Same(t, bound, r.listeners[boundKey], "keep the bound spelling and socket while an alias may own it")
	assert.Zero(t, socket.closes)

	require.NoError(t, r.RemovePeer(second.Address))
	assert.Empty(t, r.peers)
	assert.Empty(t, r.listeners, "the final alias removal must release the original listener entry")
	assert.Equal(t, 1, socket.closes, "release the ownerless socket exactly once")
}

// TestScopedTransportReadFailurePreservesDistinctInterfaceListener keeps a
// third peer on another interface while the aliases leave. Unknown ownership
// must retain both sockets and useful exact attribution for the remaining peer.
func TestScopedTransportReadFailurePreservesDistinctInterfaceListener(t *testing.T) {
	r := scopeReadFailureReactor(true)
	first := scopeAliasSettings("if0", "if0", scopeAliasPort)
	first.LocalPort = scopeAliasPort
	second := scopeAliasSettings("if0", "7", DefaultBGPPort)
	second.LocalPort = first.LocalPort
	second.Address = netip.MustParseAddr("fe80::3%if0")
	other := scopeAliasSettings("if1", "if1", DefaultBGPPort)
	other.LocalPort = first.LocalPort
	other.Address = netip.MustParseAddr("fe80::4%if1")
	otherKey := other.PeerKey()
	r.peers[first.PeerKey()] = NewPeer(first)
	r.peers[second.PeerKey()] = NewPeer(second)
	r.peers[otherKey] = NewPeer(other)

	boundAddress := netip.AddrPortFrom(first.LocalAddress, first.LocalPort)
	boundKey := boundAddress.String()
	bound := NewListener(boundKey)
	socket := &scopeReadFailureListener{address: net.TCPAddrFromAddrPort(boundAddress)}
	bound.listener = socket
	r.listeners[boundKey] = bound
	otherAddress := netip.AddrPortFrom(other.LocalAddress, other.LocalPort)
	otherBoundKey := otherAddress.String()
	otherBound := NewListener(otherBoundKey)
	otherSocket := &scopeReadFailureListener{address: net.TCPAddrFromAddrPort(otherAddress)}
	otherBound.listener = otherSocket
	r.listeners[otherBoundKey] = otherBound

	require.NoError(t, r.RemovePeer(first.Address))
	require.Len(t, r.listeners, 2)
	assert.Same(t, bound, r.listeners[boundKey])
	assert.Same(t, otherBound, r.listeners[otherBoundKey])
	assert.Zero(t, socket.closes)
	assert.Zero(t, otherSocket.closes)

	require.NoError(t, r.RemovePeer(second.Address))
	require.Len(t, r.peers, 1)
	assert.Same(t, other, r.peers[otherKey].Settings())
	require.Len(t, r.listeners, 2, "an unreadable table cannot prove either remaining scope unclaimed")
	assert.Same(t, bound, r.listeners[boundKey])
	assert.Same(t, otherBound, r.listeners[otherBoundKey])
	assert.Zero(t, socket.closes)
	assert.Zero(t, otherSocket.closes)

	var accepted *PeerSettings
	r.connCallback = func(_ net.Conn, settings *PeerSettings) { accepted = settings }
	conn := dynamicInboundConnection(t, other.LocalAddress.String(), other.Address.String())
	conn.local.Port = int(other.LocalPort)
	r.handleListenerConnection(conn, other.LocalAddress, int(other.LocalPort))
	assert.Same(t, other, accepted, "the surviving interface must retain its own useful attribution")
	assert.False(t, conn.closed)
	assert.Zero(t, otherSocket.closes)
}

// TestScopedTransportReadFailureRemovesOnlyTheAffectedListener checks the exact
// simulation-key path and a TCP binding. Removing either must leave a different
// listen port and the global listener unchanged, with the other peer usable.
func TestScopedTransportReadFailureRemovesOnlyTheAffectedListener(t *testing.T) {
	for _, tcp := range []bool{false, true} {
		name := "simulation"
		if tcp {
			name = "tcp"
		}
		t.Run(name, func(t *testing.T) {
			r := scopeReadFailureReactor(true)
			first := scopeAliasSettings("if0", "if0", scopeAliasPort)
			first.LocalPort = scopeAliasPort
			other := scopeAliasSettings("if0", "if0", DefaultBGPPort)
			other.LocalPort = first.LocalPort + 1
			other.Address = netip.MustParseAddr("fe80::5%if0")
			r.peers[first.PeerKey()] = NewPeer(first)
			r.peers[other.PeerKey()] = NewPeer(other)

			boundAddress := netip.AddrPortFrom(first.LocalAddress, first.LocalPort)
			boundKey := boundAddress.String()
			bound := NewListener(boundKey)
			socket := &scopeReadFailureListener{address: net.TCPAddrFromAddrPort(boundAddress)}
			if tcp {
				bound.listener = socket
			}
			r.listeners[boundKey] = bound
			otherAddress := netip.AddrPortFrom(other.LocalAddress, other.LocalPort)
			otherKey := otherAddress.String()
			otherBound := NewListener(otherKey)
			otherSocket := &scopeReadFailureListener{address: net.TCPAddrFromAddrPort(otherAddress)}
			otherBound.listener = otherSocket
			r.listeners[otherKey] = otherBound
			global := NewListener(":179")
			globalSocket := &scopeReadFailureListener{address: &net.TCPAddr{Port: DefaultBGPPort}}
			global.listener = globalSocket
			r.listener = global

			require.NoError(t, r.RemovePeer(first.Address))
			assert.NotContains(t, r.peers, first.PeerKey())
			require.Len(t, r.peers, 1)
			assert.Same(t, other, r.peers[other.PeerKey()].Settings())
			require.Len(t, r.listeners, 1)
			assert.Same(t, otherBound, r.listeners[otherKey])
			assert.Zero(t, otherSocket.closes)
			assert.Same(t, global, r.listener)
			assert.Zero(t, globalSocket.closes)
			if tcp {
				assert.Equal(t, 1, socket.closes)
			}

			var accepted *PeerSettings
			r.connCallback = func(_ net.Conn, settings *PeerSettings) { accepted = settings }
			conn := dynamicInboundConnection(t, other.LocalAddress.String(), other.Address.String())
			conn.local.Port = int(other.LocalPort)
			r.handleListenerConnection(conn, other.LocalAddress, int(other.LocalPort))
			assert.Same(t, other, accepted)
			assert.False(t, conn.closed)
		})
	}
}

// TestScopedTransportReadFailureRemovesMappedIPv4Binding uses AddPeer's
// normalized settings and a TCPAddr's sixteen-byte IPv4 representation. The
// binding must close only when no peer remains on that IPv4 address and port.
func TestScopedTransportReadFailureRemovesMappedIPv4Binding(t *testing.T) {
	for _, survivor := range []bool{false, true} {
		name := "last-owner"
		if survivor {
			name = "surviving-peer"
		}
		t.Run(name, func(t *testing.T) {
			r := scopeReadFailureReactor(true)
			first := NewPeerSettings(netip.MustParseAddr("127.0.0.2"), 65000, 65001, 0x01020304)
			first.LocalAddress = netip.MustParseAddr("::ffff:127.0.0.1")
			first.LocalPort = scopeAliasPort
			first.Connection = ConnectionPassive
			require.NoError(t, r.AddPeer(first))
			require.Equal(t, netip.MustParseAddr("127.0.0.1"), first.LocalAddress)
			var second *PeerSettings
			if survivor {
				second = NewPeerSettings(netip.MustParseAddr("127.0.0.3"), 65000, 65001, 0x01020304)
				second.LocalAddress = first.LocalAddress
				second.LocalPort = first.LocalPort
				second.Connection = ConnectionPassive
				require.NoError(t, r.AddPeer(second))
			}

			boundKey := netip.AddrPortFrom(first.LocalAddress, first.LocalPort).String()
			bound := NewListener(boundKey)
			socket := &scopeReadFailureListener{address: &net.TCPAddr{
				IP: net.ParseIP("127.0.0.1"), Port: int(first.LocalPort),
			}}
			require.True(t, socket.address.AddrPort().Addr().Is4In6(), "exercise the real TCPAddr representation")
			bound.listener = socket
			r.listeners[boundKey] = bound

			require.NoError(t, r.RemovePeer(first.Address))
			assert.NotContains(t, r.peers, first.PeerKey())
			if !survivor {
				assert.Empty(t, r.peers)
				assert.Empty(t, r.listeners, "mapped socket address must not hide the ownerless binding")
				assert.Equal(t, 1, socket.closes)
				return
			}
			require.Len(t, r.peers, 1)
			assert.Same(t, second, r.peers[second.PeerKey()].Settings())
			require.Len(t, r.listeners, 1)
			assert.Same(t, bound, r.listeners[boundKey])
			assert.Zero(t, socket.closes)
			var accepted *PeerSettings
			r.connCallback = func(_ net.Conn, settings *PeerSettings) { accepted = settings }
			conn := dynamicInboundConnection(t, second.LocalAddress.String(), second.Address.String())
			conn.local.Port = int(second.LocalPort)
			r.handleListenerConnection(conn, second.LocalAddress, int(second.LocalPort))
			assert.Same(t, second, accepted)
			assert.False(t, conn.closed)
		})
	}
}

// TestScopedTransportReadFailureKeepsMappedDynamicGroupListener removes the
// final static owner of a mapped TCP binding. A canonical or mapped group still
// owns the socket and must receive a useful new connection through that socket.
func TestScopedTransportReadFailureKeepsMappedDynamicGroupListener(t *testing.T) {
	for _, local := range []string{"127.0.0.1", "::ffff:127.0.0.1"} {
		t.Run(local, func(t *testing.T) {
			r := scopeReadFailureReactor(true)
			first := NewPeerSettings(netip.MustParseAddr("127.0.0.2"), 65000, 65001, 0x01020304)
			first.LocalAddress = netip.MustParseAddr("127.0.0.1")
			first.LocalPort = scopeAliasPort
			first.Connection = ConnectionPassive
			require.NoError(t, r.AddPeer(first))
			group := newTestDynamicGroup("mapped-co-owner", []string{"127.0.0.0/24"}, 10)
			group.Settings.LocalAddress = netip.MustParseAddr(local)
			group.Settings.LocalPort = first.LocalPort
			group.Settings.Connection = ConnectionPassive
			r.SetDynamicGroups([]*DynamicGroupConfig{group})
			configuredLocal := group.Settings.LocalAddress

			boundKey := netip.AddrPortFrom(first.LocalAddress, first.LocalPort).String()
			bound := NewListener(boundKey)
			socket := &scopeReadFailureListener{address: &net.TCPAddr{
				IP: net.ParseIP("127.0.0.1"), Port: int(first.LocalPort),
			}}
			require.True(t, socket.address.AddrPort().Addr().Is4In6())
			bound.listener = socket
			r.listeners[boundKey] = bound

			require.NoError(t, r.RemovePeer(first.Address))
			assert.Empty(t, r.peers)
			require.Len(t, r.dynamicGroups, 1)
			assert.Same(t, group, r.dynamicGroups[0])
			assert.Equal(t, configuredLocal, group.Settings.LocalAddress, "comparison must preserve the declaration")
			require.Len(t, r.listeners, 1)
			assert.Same(t, bound, r.listeners[boundKey])
			assert.Zero(t, socket.closes)

			var accepted *PeerSettings
			r.connCallback = func(_ net.Conn, settings *PeerSettings) { accepted = settings }
			conn := dynamicInboundConnection(t, first.LocalAddress.String(), "127.0.0.4")
			conn.local.Port = int(first.LocalPort)
			r.handleListenerConnection(conn, first.LocalAddress, int(first.LocalPort))
			require.NotNil(t, accepted)
			assert.True(t, accepted.IsDynamic)
			assert.Equal(t, group.GroupName, accepted.GroupName)
			assert.EqualValues(t, 1, group.ActivePeers.Load())
			assert.False(t, conn.closed)
			assert.Zero(t, socket.closes)
		})
	}
}

// TestScopedTransportReadFailureReleasesTranslatedWildcardBinding preserves
// the configured IPv4 wildcard key when generic TCP reports an IPv6 wildcard
// binding. Only its final configured owner may release the original entry.
func TestScopedTransportReadFailureReleasesTranslatedWildcardBinding(t *testing.T) {
	for _, survivor := range []bool{false, true} {
		name := "last-owner"
		if survivor {
			name = "surviving-peer"
		}
		t.Run(name, func(t *testing.T) {
			r := scopeReadFailureReactor(true)
			first := NewPeerSettings(netip.MustParseAddr("127.0.0.2"), 65000, 65001, 0x01020304)
			first.LocalAddress = netip.IPv4Unspecified()
			first.LocalPort = scopeAliasPort
			first.Connection = ConnectionPassive
			require.NoError(t, r.AddPeer(first))
			var second *PeerSettings
			if survivor {
				second = NewPeerSettings(netip.MustParseAddr("127.0.0.3"), 65000, 65001, 0x01020304)
				second.LocalAddress = first.LocalAddress
				second.LocalPort = first.LocalPort
				second.Connection = ConnectionPassive
				require.NoError(t, r.AddPeer(second))
			}

			boundKey := netip.AddrPortFrom(first.LocalAddress, first.LocalPort).String()
			require.Equal(t, "0.0.0.0:2179", boundKey)
			bound := NewListener(boundKey)
			socket := &scopeReadFailureListener{address: net.TCPAddrFromAddrPort(
				netip.AddrPortFrom(netip.IPv6Unspecified(), first.LocalPort))}
			require.Equal(t, netip.IPv6Unspecified(), socket.address.AddrPort().Addr())
			bound.listener = socket
			r.listeners[boundKey] = bound

			require.NoError(t, r.RemovePeer(first.Address))
			assert.NotContains(t, r.peers, first.PeerKey())
			if !survivor {
				assert.Empty(t, r.peers)
				assert.Empty(t, r.listeners, "the configured wildcard key remains authoritative after bind translation")
				assert.Equal(t, 1, socket.closes)
				return
			}
			require.Len(t, r.peers, 1)
			assert.Same(t, second, r.peers[second.PeerKey()].Settings())
			require.Len(t, r.listeners, 1)
			assert.Same(t, bound, r.listeners[boundKey])
			assert.Zero(t, socket.closes)
			var accepted *PeerSettings
			r.connCallback = func(_ net.Conn, settings *PeerSettings) { accepted = settings }
			conn := dynamicInboundConnection(t, "127.0.0.1", second.Address.String())
			conn.local.Port = int(second.LocalPort)
			r.handleListenerConnection(conn, first.LocalAddress, int(first.LocalPort))
			assert.Same(t, second, accepted)
			assert.False(t, conn.closed)
		})
	}
}
