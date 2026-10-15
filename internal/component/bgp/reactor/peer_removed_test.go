package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
)

// newRemovalTestReactor builds a minimal reactor sufficient to exercise peer
// removal (peers map, listeners, clock, and a non-nil config so peerListenPort
// does not dereference nil). eventDispatcher is nil, so RemovePeer exercises the
// nil-dispatcher guard without needing a plugin server.
func newRemovalTestReactor() *Reactor {
	return &Reactor{
		peers:     make(map[netip.AddrPort]*Peer),
		listeners: make(map[string]*Listener),
		clock:     clock.RealClock{},
		config:    &Config{},
	}
}

func insertTestPeer(r *Reactor, addr netip.Addr) {
	settings := NewPeerSettings(addr, 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	peer.SetReactor(r)
	r.peers[peerKeyFromAddrPort(addr, DefaultBGPPort)] = peer
}

// TestDoRemovePeerReturnsRemovedIdentity verifies the locked removal work drops
// the peer from the map and returns its identity, which RemovePeer then hands to
// plugins via OnPeerStateChange(SessionStateDown, ReasonPeerRemoved).
//
// VALIDATES: doRemovePeer unregisters the peer and returns a *plugin.PeerInfo
// carrying the removed peer's address/AS for the removal notification.
// PREVENTS: the removal event being emitted with the wrong (or empty) peer,
// which would leave the real peer's per-peer plugin metrics orphaned.
func TestDoRemovePeerReturnsRemovedIdentity(t *testing.T) {
	r := newRemovalTestReactor()
	addr := netip.MustParseAddr("192.0.2.7")
	insertTestPeer(r, addr)

	removed, _, err := r.doRemovePeer(addr, message.NotifyCeasePeerDeconfigured)
	require.NoError(t, err)
	require.NotNil(t, removed)
	assert.Equal(t, addr, removed.Address, "removed identity must be the removed peer")
	assert.Equal(t, uint32(65001), removed.PeerAS)
	assert.Equal(t, uint32(65000), removed.LocalAS)

	_, exists := r.peers[peerKeyFromAddrPort(addr, DefaultBGPPort)]
	assert.False(t, exists, "peer must be unregistered from the reactor")
}

// TestRemovePeerNilDispatcherNoPanic verifies RemovePeer removes the peer and
// returns nil even when no event dispatcher is wired (metrics/telemetry off),
// i.e. the removal-notification path is guarded.
//
// VALIDATES: RemovePeer's `removed != nil && r.eventDispatcher != nil` guard.
// PREVENTS: a nil-dispatcher panic when a peer is removed on a build without the
// plugin event pipeline attached.
func TestRemovePeerNilDispatcherNoPanic(t *testing.T) {
	r := newRemovalTestReactor()
	addr := netip.MustParseAddr("192.0.2.8")
	insertTestPeer(r, addr)

	require.NotPanics(t, func() {
		require.NoError(t, r.RemovePeer(addr))
	})
	_, exists := r.peers[peerKeyFromAddrPort(addr, DefaultBGPPort)]
	assert.False(t, exists, "peer must be unregistered")
}

// TestRemovePeerUnknownReturnsError verifies removing an unknown peer is a clean
// error and emits nothing.
//
// VALIDATES: RemovePeer returns ErrPeerNotFound and does not notify for a peer
// that was never present.
// PREVENTS: a spurious removal event (and plugin cleanup) for a non-existent peer.
func TestRemovePeerUnknownReturnsError(t *testing.T) {
	r := newRemovalTestReactor()
	err := r.RemovePeer(netip.MustParseAddr("192.0.2.9"))
	assert.ErrorIs(t, err, ErrPeerNotFound)
}

// recordFilterRemovals registers a filter whose only contribution is a
// PeerRemoved hook and returns the peers it was told about. The filter
// registry is restored when the test ends.
func recordFilterRemovals(t *testing.T) *[]filterapi.PeerFilterInfo {
	t.Helper()
	snap := filterapi.Snapshot()
	t.Cleanup(func() { filterapi.Restore(snap) })
	var seen []filterapi.PeerFilterInfo
	require.NoError(t, filterapi.Register(filterapi.Filter{
		Name:        "removal-recorder",
		PeerRemoved: func(peer filterapi.PeerFilterInfo) { seen = append(seen, peer) },
	}))
	return &seen
}

// TestRemovePeerNotifiesFilters checks that `delete bgp peer` tells the filters.
//
// VALIDATES: RemovePeer calls every registered Filter.PeerRemoved once, with
// the removed peer's address, name and group, before it returns.
// PREVENTS: per-peer filter state (bgp-role's learned role) outliving a peer
// the state event never told its plugin about, and being read by a peer
// created again under the same name.
func TestRemovePeerNotifiesFilters(t *testing.T) {
	seen := recordFilterRemovals(t)
	r := newRemovalTestReactor()
	addr := netip.MustParseAddr("192.0.2.9")
	settings := NewPeerSettings(addr, 65000, 65001, 0x01010101)
	settings.Name = "peer-192.0.2.9"
	settings.GroupName = "edge"
	peer := NewPeer(settings)
	peer.SetReactor(r)
	r.peers[peerKeyFromAddrPort(addr, DefaultBGPPort)] = peer

	require.NoError(t, r.RemovePeer(addr))

	require.Len(t, *seen, 1, "the removal must reach the filter exactly once")
	assert.Equal(t, filterapi.PeerFilterInfo{Address: addr, Name: "peer-192.0.2.9", GroupName: "edge"}, (*seen)[0])
}

// TestRemoveDynamicPeerNotifiesFilters checks the dynamic-member removal path.
//
// VALIDATES: removeDynamicPeer calls Filter.PeerRemoved with the member's
// address, generated name and group.
// PREVENTS: a dynamic member's learned role living as long as its group, after
// the member itself is gone.
func TestRemoveDynamicPeerNotifiesFilters(t *testing.T) {
	seen := recordFilterRemovals(t)
	r := newRemovalTestReactor()
	addr := netip.MustParseAddr("192.0.2.10")
	settings := NewPeerSettings(addr, 65000, 65001, 0x01010101)
	settings.Name = "dyn-192.0.2.10"
	settings.GroupName = "dynamic-edge"
	peer := NewPeer(settings)
	peer.SetReactor(r)
	r.peers[settings.PeerKey()] = peer

	r.mu.Lock()
	r.removeDynamicPeer(peer, message.NotifyCeasePeerDeconfigured)
	r.mu.Unlock()

	require.Len(t, *seen, 1, "the removal must reach the filter exactly once")
	assert.Equal(t, filterapi.PeerFilterInfo{Address: addr, Name: "dyn-192.0.2.10", GroupName: "dynamic-edge"}, (*seen)[0])
}
