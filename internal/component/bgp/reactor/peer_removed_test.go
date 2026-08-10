package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	removed, err := r.doRemovePeer(addr)
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
