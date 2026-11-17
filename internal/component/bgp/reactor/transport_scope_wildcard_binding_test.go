// Design: docs/architecture/bgp/interface-event-reactions.md -- shared socket ownership
// Related: transport_scope_final_alias_test.go -- bound-key lifetime boundaries
// Related: reactor_connection.go -- attribution to a binding's current owners

package reactor

import (
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScopedTransportReadFailureKeepsMixedWildcardOwners removes each of the
// two declarations sharing one translated binding first, then removes its
// survivor. Only the final removal may close the original socket and map entry.
func TestScopedTransportReadFailureKeepsMixedWildcardOwners(t *testing.T) {
	for _, removeIPv4First := range []bool{true, false} {
		name := "remove-ipv6-first"
		if removeIPv4First {
			name = "remove-ipv4-first"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newMixedWildcardBinding(t)
			first, second := fixture.ipv6, fixture.ipv4
			if removeIPv4First {
				first, second = fixture.ipv4, fixture.ipv6
			}

			require.NoError(t, fixture.reactor.RemovePeer(first.Address))
			assert.NotContains(t, fixture.reactor.peers, first.PeerKey())
			require.Len(t, fixture.reactor.peers, 1)
			assert.Same(t, second, fixture.reactor.peers[second.PeerKey()].Settings())
			require.Len(t, fixture.reactor.listeners, 1, "the other declaration still owns this socket")
			assert.Same(t, fixture.listener, fixture.reactor.listeners[fixture.key])
			assert.Zero(t, fixture.socket.closes)

			require.NoError(t, fixture.reactor.RemovePeer(second.Address))
			assert.Empty(t, fixture.reactor.peers)
			assert.Empty(t, fixture.reactor.listeners, "release the original configured entry")
			assert.Equal(t, 1, fixture.socket.closes, "close the shared socket only once")
		})
	}
}

// TestScopedTransportReadFailureAttributesMixedWildcardBinding sends useful
// connections from both configured owners and refuses a foreign remote before
// repeating useful attribution to the surviving owner after either removal.
func TestScopedTransportReadFailureAttributesMixedWildcardBinding(t *testing.T) {
	for _, removeIPv4First := range []bool{true, false} {
		name := "surviving-ipv4"
		if removeIPv4First {
			name = "surviving-ipv6"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newMixedWildcardBinding(t)
			accept := func(local string, expected *PeerSettings) {
				var accepted *PeerSettings
				fixture.reactor.connCallback = func(_ net.Conn, settings *PeerSettings) {
					accepted = settings
				}
				conn := dynamicInboundConnection(t, local, expected.Address.String())
				conn.local.Port = int(fixture.ipv4.LocalPort)
				fixture.reactor.handleListenerConnection(conn, fixture.ipv4.LocalAddress,
					int(fixture.ipv4.LocalPort))
				assert.Same(t, expected, accepted, "attribute by the remote peer on the shared binding")
				assert.False(t, conn.closed)
			}
			accept("127.0.0.1", fixture.ipv4)
			accept("::1", fixture.ipv6)

			var accepted *PeerSettings
			fixture.reactor.connCallback = func(_ net.Conn, settings *PeerSettings) {
				accepted = settings
			}
			foreign := dynamicInboundConnection(t, "::1", "2001:db8::99")
			foreign.local.Port = int(fixture.ipv4.LocalPort)
			fixture.reactor.handleListenerConnection(foreign, fixture.ipv4.LocalAddress,
				int(fixture.ipv4.LocalPort))
			assert.Nil(t, accepted, "a second declaration must not grant the first peer's policy")
			assert.True(t, foreign.closed)

			removed, survivor, local := fixture.ipv6, fixture.ipv4, "127.0.0.1"
			if removeIPv4First {
				removed, survivor, local = fixture.ipv4, fixture.ipv6, "::1"
			}
			require.NoError(t, fixture.reactor.RemovePeer(removed.Address))
			require.Len(t, fixture.reactor.listeners, 1, "the surviving peer keeps its binding")
			assert.Same(t, fixture.listener, fixture.reactor.listeners[fixture.key])
			assert.Zero(t, fixture.socket.closes)
			accept(local, survivor)
		})
	}
}

// TestScopedTransportReadFailureKeepsMappedWildcardGroupBinding removes the
// IPv6 peer from a translated socket while its IPv4 wildcard group remains.
// Canonical and mapped group declarations must keep the same useful binding.
func TestScopedTransportReadFailureKeepsMappedWildcardGroupBinding(t *testing.T) {
	for _, local := range []string{"0.0.0.0", "::ffff:0.0.0.0"} {
		t.Run(local, func(t *testing.T) {
			r := scopeReadFailureReactor(true)
			original := NewPeerSettings(netip.MustParseAddr("127.0.0.2"), 65000, 65001, 0x01020304)
			original.LocalAddress = netip.IPv4Unspecified()
			original.LocalPort = scopeAliasPort
			original.Connection = ConnectionPassive
			require.NoError(t, r.AddPeer(original))
			peer := NewPeerSettings(netip.MustParseAddr("2001:db8::2"), 65000, 65001, 0x01020304)
			peer.LocalAddress = netip.IPv6Unspecified()
			peer.LocalPort = scopeAliasPort
			peer.Connection = ConnectionPassive
			require.NoError(t, r.AddPeer(peer))
			group := newTestDynamicGroup("mixed-mapped-wildcard", []string{"127.0.0.0/24"}, 10)
			group.Settings.LocalAddress = netip.MustParseAddr(local)
			group.Settings.LocalPort = peer.LocalPort
			group.Settings.Connection = ConnectionPassive
			r.SetDynamicGroups([]*DynamicGroupConfig{group})
			configuredLocal := group.Settings.LocalAddress

			boundKey := netip.AddrPortFrom(netip.IPv4Unspecified(), peer.LocalPort).String()
			bound := NewListener(boundKey)
			socket := &scopeReadFailureListener{address: net.TCPAddrFromAddrPort(
				netip.AddrPortFrom(netip.IPv6Unspecified(), peer.LocalPort))}
			bound.listener = socket
			r.listeners[boundKey] = bound
			r.mu.RLock()
			lookupKey, reused, found := r.listenerForAddressPort(peer.LocalAddress, int(peer.LocalPort))
			r.mu.RUnlock()
			require.True(t, found)
			require.Equal(t, boundKey, lookupKey)
			require.Same(t, bound, reused)

			require.NoError(t, r.RemovePeer(original.Address))
			require.Len(t, r.peers, 1)
			assert.Same(t, peer, r.peers[peer.PeerKey()].Settings())
			require.Len(t, r.listeners, 1)
			assert.Same(t, bound, r.listeners[boundKey])
			assert.Zero(t, socket.closes)

			require.NoError(t, r.RemovePeer(peer.Address))
			assert.Empty(t, r.peers)
			require.Len(t, r.dynamicGroups, 1)
			assert.Same(t, group, r.dynamicGroups[0])
			assert.Equal(t, configuredLocal, group.Settings.LocalAddress)
			require.Len(t, r.listeners, 1, "the mapped declaration still owns the original socket")
			assert.Same(t, bound, r.listeners[boundKey])
			assert.Zero(t, socket.closes)

			var accepted *PeerSettings
			r.connCallback = func(_ net.Conn, settings *PeerSettings) { accepted = settings }
			conn := dynamicInboundConnection(t, "127.0.0.1", "127.0.0.4")
			conn.local.Port = int(peer.LocalPort)
			r.handleListenerConnection(conn, netip.IPv4Unspecified(), int(peer.LocalPort))
			require.NotNil(t, accepted)
			assert.True(t, accepted.IsDynamic)
			assert.Equal(t, group.GroupName, accepted.GroupName)
			assert.EqualValues(t, 1, group.ActivePeers.Load())
			assert.False(t, conn.closed)
			assert.Zero(t, socket.closes)
		})
	}
}

// TestScopedTransportReadFailureKeepsCanonicalGroupOnMappedBinding keeps a
// literal mapped listener key after a group reload. Both its literal owner and
// a canonical replacement must survive the IPv6 co-owner's removal.
func TestScopedTransportReadFailureKeepsCanonicalGroupOnMappedBinding(t *testing.T) {
	for _, local := range []string{"0.0.0.0", "::ffff:0.0.0.0"} {
		t.Run(local, func(t *testing.T) {
			r := scopeReadFailureReactor(true)
			mapped := netip.MustParseAddr("::ffff:0.0.0.0")
			original := newTestDynamicGroup("literal-mapped-wildcard", []string{"127.0.0.0/24"}, 10)
			original.Settings.LocalAddress = mapped
			original.Settings.LocalPort = scopeAliasPort
			original.Settings.Connection = ConnectionPassive
			r.SetDynamicGroups([]*DynamicGroupConfig{original})
			r.mu.RLock()
			specs := r.listenerSpecs()
			r.mu.RUnlock()
			require.Contains(t, specs, listenerSpec{addr: mapped, port: scopeAliasPort})

			boundKey := netip.AddrPortFrom(mapped, scopeAliasPort).String()
			bound := NewListener(boundKey)
			socket := &scopeReadFailureListener{address: net.TCPAddrFromAddrPort(
				netip.AddrPortFrom(netip.IPv6Unspecified(), scopeAliasPort))}
			bound.listener = socket
			r.listeners[boundKey] = bound
			peer := NewPeerSettings(netip.MustParseAddr("2001:db8::2"), 65000, 65001, 0x01020304)
			peer.LocalAddress = netip.IPv6Unspecified()
			peer.LocalPort = scopeAliasPort
			peer.Connection = ConnectionPassive
			require.NoError(t, r.AddPeer(peer))

			group := newTestDynamicGroup(original.GroupName, []string{"127.0.0.0/24"}, 10)
			group.Settings.LocalAddress = netip.MustParseAddr(local)
			group.Settings.LocalPort = scopeAliasPort
			group.Settings.Connection = ConnectionPassive
			r.SetDynamicGroups([]*DynamicGroupConfig{group})
			configuredLocal := group.Settings.LocalAddress

			require.NoError(t, r.RemovePeer(peer.Address))
			assert.Empty(t, r.peers)
			require.Len(t, r.dynamicGroups, 1)
			assert.Same(t, group, r.dynamicGroups[0])
			assert.Equal(t, configuredLocal, group.Settings.LocalAddress)
			require.Len(t, r.listeners, 1, "a group reload must not lose the stored declaration's identity")
			assert.Same(t, bound, r.listeners[boundKey])
			assert.Zero(t, socket.closes)

			var accepted *PeerSettings
			r.connCallback = func(_ net.Conn, settings *PeerSettings) { accepted = settings }
			conn := dynamicInboundConnection(t, "127.0.0.1", "127.0.0.4")
			conn.local.Port = scopeAliasPort
			r.handleListenerConnection(conn, mapped, scopeAliasPort)
			require.NotNil(t, accepted)
			assert.True(t, accepted.IsDynamic)
			assert.Equal(t, group.GroupName, accepted.GroupName)
			assert.EqualValues(t, 1, group.ActivePeers.Load())
			assert.False(t, conn.closed)
			assert.Zero(t, socket.closes)
		})
	}
}

// TestScopedTransportReadFailureReusesHistoricalMappedDeclaration adds a
// canonical static peer after the listener pass. It must reuse the old mapped
// declaration's socket, retain useful attribution and release its original key.
func TestScopedTransportReadFailureReusesHistoricalMappedDeclaration(t *testing.T) {
	fixture := newHistoricalMappedWildcardBinding(t)
	r := fixture.reactor
	r.SetListenerFactory(startupFailingListenerFactory{
		err: errors.New("fixture: an existing listener must be reused"),
	})
	r.listenersStarted = true
	peer := NewPeerSettings(netip.MustParseAddr("127.0.0.2"), 65000, 65001, 0x01020304)
	peer.LocalAddress = netip.IPv4Unspecified()
	peer.LocalPort = scopeAliasPort
	peer.Connection = ConnectionPassive
	require.NoError(t, r.AddPeer(peer), "the historical declaration must prevent a second bind")
	require.Len(t, r.listeners, 1)
	assert.Same(t, fixture.listener, r.listeners[fixture.key])
	assert.Zero(t, fixture.socket.closes)

	var accepted *PeerSettings
	r.connCallback = func(_ net.Conn, settings *PeerSettings) { accepted = settings }
	conn := dynamicInboundConnection(t, "127.0.0.1", peer.Address.String())
	conn.local.Port = scopeAliasPort
	r.handleListenerConnection(conn, fixture.group.Settings.LocalAddress, scopeAliasPort)
	assert.Same(t, peer, accepted)
	assert.False(t, conn.closed)

	r.SetDynamicGroups(nil)
	require.Len(t, r.peers, 1)
	assert.Same(t, peer, r.peers[peer.PeerKey()].Settings())
	require.Len(t, r.listeners, 1)
	assert.Same(t, fixture.listener, r.listeners[fixture.key])
	assert.Zero(t, fixture.socket.closes)
	require.NoError(t, r.RemovePeer(peer.Address))
	assert.Empty(t, r.peers)
	assert.Empty(t, r.listeners, "the last canonical owner releases the historical map entry")
	assert.Equal(t, 1, fixture.socket.closes)
}

// TestScopedTransportReadFailureReleasesHistoricalMappedMember exercises the
// group-install bookkeeping primitive while a published canonical member still
// owns its old socket. Final RemovePeer must release that historical entry.
func TestScopedTransportReadFailureReleasesHistoricalMappedMember(t *testing.T) {
	fixture := newHistoricalMappedWildcardBinding(t)
	r := fixture.reactor
	group := newTestDynamicGroup(fixture.group.GroupName, []string{"127.0.0.0/24"}, 10)
	group.Settings.LocalAddress = netip.IPv4Unspecified()
	group.Settings.LocalPort = scopeAliasPort
	group.Settings.Connection = ConnectionPassive
	r.SetDynamicGroups([]*DynamicGroupConfig{group})
	remote := netip.MustParseAddr("127.0.0.4")
	local := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), scopeAliasPort)
	member := r.tryCreateDynamicPeerAt(remote, local)
	require.NotNil(t, member, "a canonical wildcard admits a useful member")
	require.True(t, member.Settings().IsDynamic)
	require.Equal(t, netip.IPv4Unspecified(), member.Settings().LocalAddress)
	assert.EqualValues(t, 1, group.ActivePeers.Load())
	require.Len(t, r.listeners, 1)
	assert.Same(t, fixture.listener, r.listeners[fixture.key])
	assert.Zero(t, fixture.socket.closes)

	// This deliberately tests the shared install primitive, not the journal's
	// separate removal of members whose group disappears.
	r.SetDynamicGroups(nil)
	require.Len(t, r.peers, 1)
	assert.Same(t, member, r.peers[member.Settings().PeerKey()])
	var accepted *PeerSettings
	r.connCallback = func(_ net.Conn, settings *PeerSettings) { accepted = settings }
	conn := dynamicInboundConnection(t, local.Addr().String(), remote.String())
	conn.local.Port = scopeAliasPort
	r.handleListenerConnection(conn, fixture.group.Settings.LocalAddress, scopeAliasPort)
	assert.Same(t, member.Settings(), accepted)
	assert.False(t, conn.closed)
	assert.Zero(t, fixture.socket.closes)

	require.NoError(t, r.RemovePeer(remote))
	assert.Empty(t, r.peers)
	assert.Empty(t, r.listeners, "the last member releases the original mapped-key socket")
	assert.Equal(t, 1, fixture.socket.closes)
}

type historicalMappedWildcardBinding struct {
	reactor  *Reactor
	group    *DynamicGroupConfig
	listener *Listener
	socket   *scopeReadFailureListener
	key      string
}

// newHistoricalMappedWildcardBinding retains a real group declaration's literal
// key and the translated endpoint, without opening a host socket or a worker.
func newHistoricalMappedWildcardBinding(t *testing.T) historicalMappedWildcardBinding {
	t.Helper()
	r := scopeReadFailureReactor(true)
	mapped := netip.MustParseAddr("::ffff:0.0.0.0")
	group := newTestDynamicGroup("historical-mapped-wildcard", []string{"127.0.0.0/24"}, 10)
	group.Settings.LocalAddress = mapped
	group.Settings.LocalPort = scopeAliasPort
	group.Settings.Connection = ConnectionPassive
	r.SetDynamicGroups([]*DynamicGroupConfig{group})
	r.mu.RLock()
	specs := r.listenerSpecs()
	r.mu.RUnlock()
	require.Contains(t, specs, listenerSpec{addr: mapped, port: scopeAliasPort})
	key := netip.AddrPortFrom(mapped, scopeAliasPort).String()
	listener := NewListener(key)
	socket := &scopeReadFailureListener{address: net.TCPAddrFromAddrPort(
		netip.AddrPortFrom(netip.IPv6Unspecified(), scopeAliasPort))}
	listener.listener = socket
	r.listeners[key] = listener
	return historicalMappedWildcardBinding{
		reactor: r, group: group, listener: listener, socket: socket, key: key,
	}
}

type mixedWildcardBinding struct {
	reactor  *Reactor
	ipv4     *PeerSettings
	ipv6     *PeerSettings
	listener *Listener
	socket   *scopeReadFailureListener
	key      string
}

// newMixedWildcardBinding publishes two real AddPeer declarations and the
// single translated socket the existing listener lookup permits them to share.
// It starts no host listener or accept goroutine.
func newMixedWildcardBinding(t *testing.T) mixedWildcardBinding {
	t.Helper()
	r := scopeReadFailureReactor(true)
	ipv4 := NewPeerSettings(netip.MustParseAddr("127.0.0.2"), 65000, 65001, 0x01020304)
	ipv4.LocalAddress = netip.IPv4Unspecified()
	ipv4.LocalPort = scopeAliasPort
	ipv4.Connection = ConnectionPassive
	require.NoError(t, r.AddPeer(ipv4))

	key := netip.AddrPortFrom(ipv4.LocalAddress, ipv4.LocalPort).String()
	bound := NewListener(key)
	socket := &scopeReadFailureListener{address: net.TCPAddrFromAddrPort(
		netip.AddrPortFrom(netip.IPv6Unspecified(), ipv4.LocalPort))}
	bound.listener = socket
	r.listeners[key] = bound

	ipv6 := NewPeerSettings(netip.MustParseAddr("2001:db8::2"), 65000, 65001, 0x01020304)
	ipv6.LocalAddress = netip.IPv6Unspecified()
	ipv6.LocalPort = ipv4.LocalPort
	ipv6.Connection = ConnectionPassive
	require.NoError(t, r.AddPeer(ipv6))
	r.mu.RLock()
	lookupKey, reused, found := r.listenerForAddressPort(ipv6.LocalAddress, int(ipv6.LocalPort))
	r.mu.RUnlock()
	require.True(t, found, "the actual bound endpoint permits this second declaration")
	require.Equal(t, key, lookupKey)
	require.Same(t, bound, reused)
	require.Len(t, r.listeners, 1)

	return mixedWildcardBinding{
		reactor: r, ipv4: ipv4, ipv6: ipv6, listener: bound, socket: socket, key: key,
	}
}
