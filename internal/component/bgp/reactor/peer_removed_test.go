package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
	assert.Equal(t, rpc.RemoteASOf(65001), removed.PeerAS)
	assert.Equal(t, uint32(65000), removed.LocalAS)

	_, exists := r.peers[peerKeyFromAddrPort(addr, DefaultBGPPort)]
	assert.False(t, exists, "peer must be unregistered from the reactor")
}

// TestDoRemovePeerNamesTheSessionGeneration verifies the removal down event
// names the session it ends.
//
// GOAL: `delete bgp peer` tells every process the peer is down through the
// PeerInfo doRemovePeer builds (removePeer hands it to OnPeerStateChange
// unchanged), and the index then drops the peer, so the FSM teardown's own
// down finds no receiver. A forwarding plugin stamps its departure with the
// generation this down carries, and the engine refuses a departure that names
// generation 0.
// METHOD: give the peer a live forward generation, remove it, and read the
// PeerInfo the removal answers.
//
// VALIDATES: removed.SessionGeneration is above the generation of the session
// the removal ends, so the departure covers every route that session wrote,
// and is the peer's forwardGeneration after the removal.
// PREVENTS: a removal down carrying generation 0, which made bgp-rs log
// "recovery-generation 0 names no session", send no WITHDRAW, and keep the
// deleted peer's routes on every other peer.
func TestDoRemovePeerNamesTheSessionGeneration(t *testing.T) {
	r := newRemovalTestReactor()
	addr := netip.MustParseAddr("192.0.2.9")
	insertTestPeer(r, addr)
	peer := r.peers[peerKeyFromAddrPort(addr, DefaultBGPPort)]
	peer.advanceForwardGeneration()
	generation := peer.forwardGeneration.Load()

	removed, _, err := r.doRemovePeer(addr, message.NotifyCeasePeerDeconfigured)
	require.NoError(t, err)
	require.NotNil(t, removed)
	assert.Greater(t, removed.SessionGeneration, generation,
		"the removal down names a generation above the session it ends, never 0")
	assert.Equal(t, peer.forwardGeneration.Load(), removed.SessionGeneration,
		"the removal down names the generation the removal gave the peer")
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
