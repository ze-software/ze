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

// TestDoRemovePeerNotifiesFiltersBeforeCease checks when `delete bgp peer`
// tells the filters: inside the locked removal, before the Cease can run.
//
// VALIDATES: doRemovePeer calls Filter.PeerRemoved with r.mu held, and returns
// with the hook already run while the stop that sends the Cease is still owed
// by the caller.
// PREVENTS: the hook running after a Cease blocked on a full send buffer, and
// deleting the fresh role of a peer created again at the address whose OPEN
// arrived during that block.
// DISCRIMINATION: moving the call back after stop.run() in removePeer leaves
// the recorder empty here, because this test never runs the stop.
func TestDoRemovePeerNotifiesFiltersBeforeCease(t *testing.T) {
	r := newRemovalTestReactor()
	snap := filterapi.Snapshot()
	t.Cleanup(func() { filterapi.Restore(snap) })
	var seen []filterapi.PeerFilterInfo
	lockHeld := false
	require.NoError(t, filterapi.Register(filterapi.Filter{
		Name: "removal-order-recorder",
		PeerRemoved: func(peer filterapi.PeerFilterInfo) {
			seen = append(seen, peer)
			if r.mu.TryLock() {
				r.mu.Unlock()
				return
			}
			lockHeld = true
		},
	}))
	addr := netip.MustParseAddr("192.0.2.11")
	insertTestPeer(r, addr)

	removed, stop, err := r.doRemovePeer(addr, message.NotifyCeasePeerDeconfigured)
	require.NoError(t, err)
	require.NotNil(t, removed)
	require.NotNil(t, stop.peer, "the Cease is still owed by the caller")

	require.Len(t, seen, 1, "the filters must be told before the Cease is sent")
	assert.Equal(t, addr, seen[0].Address)
	assert.True(t, lockHeld, "the hook must run in the critical section that deletes the peer")
}

// TestReloadRemoveNotifiesFilters checks the reload's removal of a peer the
// new configuration drops.
//
// VALIDATES: reconcilePeersJournaled's remove step calls Filter.PeerRemoved
// with the dropped peer's address, name and group.
// PREVENTS: a peer a commit removes keeping its filter state until a prune
// that only some filters run.
func TestReloadRemoveNotifiesFilters(t *testing.T) {
	seen := recordFilterRemovals(t)
	r := New(&Config{})
	adapter := &reactorAPIAdapter{r: r}
	dropped := NewPeerSettings(mustParseAddr("192.0.2.12"), 65000, 65001, 0)
	dropped.Name = "peerA"
	dropped.GroupName = "edge"
	require.NoError(t, r.AddPeer(dropped))

	require.NoError(t, adapter.reconcilePeersJournaled(nil, Globals{}, dynamicGroupsNotRead, "test", &testJournal{}))

	require.Len(t, *seen, 1, "the reload's removal must reach the filter exactly once")
	assert.Equal(t, filterapi.PeerFilterInfo{Address: dropped.Address, Name: "peerA", GroupName: "edge"}, (*seen)[0])
}

// TestReloadReplacedDynamicPeerNotifiesFilters covers the leak the reload
// path held: a dynamic member replaced by a configured peer at its address.
//
// VALIDATES: the reload tells the filters the member left, under the member's
// own name and group, so its learned role does not outlive it.
// PREVENTS: the member's role staying in bgp-role under a name key no peer
// will ever use again.
func TestReloadReplacedDynamicPeerNotifiesFilters(t *testing.T) {
	seen := recordFilterRemovals(t)
	addr := netip.MustParseAddr("185.1.69.43")
	configured := NewPeerSettings(addr, 65000, 65010, 0x01020304)
	configured.Connection = ConnectionPassive
	configured.Name = "ix-peer-2"

	r, member := newDynamicReloadReactor(t, dynamicReloadGroup(t), addr, []*PeerSettings{configured})
	memberSettings := member.Settings()
	require.True(t, memberSettings.IsDynamic)

	require.NoError(t, (&reactorAPIAdapter{r: r}).Reload())

	want := filterapi.PeerFilterInfo{Address: addr, Name: memberSettings.Name, GroupName: memberSettings.GroupName}
	require.Len(t, *seen, 1, "the replaced member's removal must reach the filter exactly once")
	assert.Equal(t, want, (*seen)[0])
}

// TestReloadAddUndoNotifiesFilters checks the undo of a reload's add (AC-3).
//
// VALIDATES: rolling back a reload that added a peer calls Filter.PeerRemoved
// for that peer, and the forward add does not.
// PREVENTS: state an OPEN left in the filters for a peer the rollback removed
// surviving the rollback.
func TestReloadAddUndoNotifiesFilters(t *testing.T) {
	seen := recordFilterRemovals(t)
	r := New(&Config{})
	adapter := &reactorAPIAdapter{r: r}
	added := NewPeerSettings(mustParseAddr("192.0.2.13"), 65000, 65003, 0)
	added.Name = "peerC"
	added.GroupName = "core"

	j := &testJournal{}
	require.NoError(t, adapter.reconcilePeersJournaled([]*PeerSettings{added}, Globals{}, dynamicGroupsNotRead, "test", j))
	require.Empty(t, *seen, "an add tells the filters nothing")

	j.rollback()

	require.Len(t, *seen, 1, "the undo of the add must reach the filter exactly once")
	assert.Equal(t, filterapi.PeerFilterInfo{Address: added.Address, Name: "peerC", GroupName: "core"}, (*seen)[0])
}
