package reactor

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// testPeerWithCtxID creates a minimal Peer with the given sendCtxID for update group tests.
func testPeerWithCtxID(addr netip.Addr, ctxID bgpctx.ContextID) *Peer {
	ps := &PeerSettings{
		Address: addr,
		Port:    DefaultBGPPort,
		LocalAS: 65000,
		PeerAS:  65000,
	}
	p := &Peer{
		settings:   ps,
		addrString: addr.String(),
	}
	p.sendCtxID = ctxID
	return p
}

// VALIDATES: GroupKey equality -- same ctxID+policy = same key.
// PREVENTS: Two peers with identical encoding context ending up in different groups.
func TestUpdateGroupKey(t *testing.T) {
	k1 := GroupKey{CtxID: 1, PolicyKey: 0}
	k2 := GroupKey{CtxID: 1, PolicyKey: 0}
	k3 := GroupKey{CtxID: 2, PolicyKey: 0}
	k4 := GroupKey{CtxID: 1, PolicyKey: 1}

	assert.Equal(t, k1, k2, "same ctxID+policy should be equal")
	assert.NotEqual(t, k1, k3, "different ctxID should not be equal")
	assert.NotEqual(t, k1, k4, "different policy should not be equal")

	// Verify usable as map key
	m := make(map[GroupKey]bool)
	m[k1] = true
	assert.True(t, m[k2], "same key should find entry in map")
	assert.False(t, m[k3], "different key should not find entry in map")
}

// VALIDATES: Add peer to index, remove peer, verify membership.
// PREVENTS: Peer not tracked after Add, or still tracked after Remove.
func TestUpdateGroupAddRemove(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	peer := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 5)
	idx.Add(peer)

	key := GroupKey{CtxID: 5, PolicyKey: 0}
	group, ok := idx.groups[key]
	require.True(t, ok, "group should exist after Add")
	require.Len(t, group.Members, 1, "group should have 1 member")
	assert.Equal(t, peer, group.Members[0])

	idx.Remove(peer)
	_, ok = idx.groups[key]
	assert.False(t, ok, "group should be deleted after removing last member")
}

// VALIDATES: Two peers with same sendCtxID join the same group.
// PREVENTS: Peers with identical encoding being placed in separate groups.
func TestUpdateGroupFormation(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	peer1 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 7)
	peer2 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.2"), 7)

	idx.Add(peer1)
	idx.Add(peer2)

	key := GroupKey{CtxID: 7, PolicyKey: 0}
	group, ok := idx.groups[key]
	require.True(t, ok, "group should exist")
	assert.Len(t, group.Members, 2, "both peers should be in same group")
}

// VALIDATES: Two peers with different sendCtxID get separate groups.
// PREVENTS: Peers with different encoding contexts sharing a group (would produce wrong wire bytes).
func TestUpdateGroupDifferentContexts(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	peer1 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 3)
	peer2 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.2"), 4)

	idx.Add(peer1)
	idx.Add(peer2)

	assert.Len(t, idx.groups, 2, "different ctxIDs should produce separate groups")

	key3 := GroupKey{CtxID: 3, PolicyKey: 0}
	key4 := GroupKey{CtxID: 4, PolicyKey: 0}
	assert.Len(t, idx.groups[key3].Members, 1)
	assert.Len(t, idx.groups[key4].Members, 1)
}

// VALIDATES: Last peer removed from a group deletes the group entry.
// PREVENTS: Empty groups accumulating in the index (memory leak).
func TestUpdateGroupTeardown(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	peer1 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 9)
	peer2 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.2"), 9)

	idx.Add(peer1)
	idx.Add(peer2)
	assert.Len(t, idx.groups, 1, "both in same group")

	idx.Remove(peer1)
	key := GroupKey{CtxID: 9, PolicyKey: 0}
	group, ok := idx.groups[key]
	require.True(t, ok, "group should still exist with one member")
	assert.Len(t, group.Members, 1, "one member remaining")
	assert.Equal(t, peer2, group.Members[0])

	idx.Remove(peer2)
	assert.Len(t, idx.groups, 0, "group should be deleted when empty")
}

// VALIDATES: Peer moves between groups when sendCtxID changes (renegotiation).
// PREVENTS: Stale group membership after capability renegotiation.
func TestUpdateGroupRenegotiation(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	peer := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 10)
	idx.Add(peer)

	key10 := GroupKey{CtxID: 10, PolicyKey: 0}
	require.Len(t, idx.groups[key10].Members, 1)

	// Simulate renegotiation: remove from old group, change ctxID, add to new group
	idx.Remove(peer)
	peer.sendCtxID = 20
	idx.Add(peer)

	assert.Len(t, idx.groups, 1, "should have exactly one group")
	_, ok := idx.groups[key10]
	assert.False(t, ok, "old group should be gone")

	key20 := GroupKey{CtxID: 20, PolicyKey: 0}
	group, ok := idx.groups[key20]
	require.True(t, ok, "new group should exist")
	assert.Len(t, group.Members, 1)
}

// VALIDATES: Default env var = true means groups are enabled.
// PREVENTS: Update groups silently disabled when env var is not set.
func TestUpdateGroupEnvVarDefault(t *testing.T) {
	// Ensure no override; empty value makes GetBool return default (true).
	env.ResetCache()
	defer env.ResetCache()

	idx := NewUpdateGroupIndexFromEnv()
	assert.True(t, idx.Enabled(), "default should be enabled")
}

// VALIDATES: Env var false = groups disabled, Add is no-op.
// PREVENTS: Update groups operating when explicitly disabled by user.
func TestUpdateGroupDisabledByEnv(t *testing.T) {
	// Use env.SetBool with dot-notation key (normalize maps dots to underscores,
	// but NOT hyphens, so os.Setenv with uppercase underscored key won't match).
	err := env.SetBool("ze.bgp.reactor.update-groups", false)
	require.NoError(t, err)
	defer func() {
		_ = env.Set("ze.bgp.reactor.update-groups", "")
		env.ResetCache()
	}()

	idx := NewUpdateGroupIndexFromEnv()
	assert.False(t, idx.Enabled(), "should be disabled when env var is false")

	peer := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 5)
	idx.Add(peer)
	assert.Len(t, idx.groups, 0, "Add should be no-op when disabled")

	groups := idx.GroupsForPeers([]*Peer{peer})
	assert.Nil(t, groups, "GroupsForPeers should return nil when disabled")
}

// VALIDATES: All-unique-context peers: N peers = N groups (no false sharing).
// PREVENTS: Unrelated peers being grouped together due to hash collisions or bugs.
func TestUpdateGroupNoRegression(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	peers := make([]*Peer, 10)
	for i := range peers {
		addr := netip.AddrFrom4([4]byte{10, 0, 0, byte(i + 1)})
		peers[i] = testPeerWithCtxID(addr, bgpctx.ContextID(i+1))
		idx.Add(peers[i])
	}

	assert.Len(t, idx.groups, 10, "N unique contexts should produce N groups")

	groups := idx.GroupsForPeers(peers)
	assert.Len(t, groups, 10, "GroupsForPeers should return N groups")
}

// VALIDATES: GroupsForPeers returns correctly grouped peers.
// PREVENTS: GroupsForPeers returning wrong membership or missing peers.
func TestUpdateGroupGroupsForPeers(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	// 3 peers: two with ctxID=5, one with ctxID=6
	peer1 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 5)
	peer2 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.2"), 5)
	peer3 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.3"), 6)

	idx.Add(peer1)
	idx.Add(peer2)
	idx.Add(peer3)

	// Query with a subset (peer1 and peer3 only)
	groups := idx.GroupsForPeers([]*Peer{peer1, peer3})
	require.Len(t, groups, 2, "should get 2 groups for 2 different ctxIDs")

	// Verify group contents
	foundCtx5 := false
	foundCtx6 := false
	for _, g := range groups {
		if g.Key.CtxID == 5 {
			foundCtx5 = true
			assert.Len(t, g.Members, 1, "only peer1 from the query subset is in ctxID=5")
		}
		if g.Key.CtxID == 6 {
			foundCtx6 = true
			assert.Len(t, g.Members, 1, "peer3 should be in ctxID=6")
		}
	}
	assert.True(t, foundCtx5, "should have group for ctxID=5")
	assert.True(t, foundCtx6, "should have group for ctxID=6")
}

// VALIDATES: Peer with ctxID=0 (not established) is not grouped.
// PREVENTS: Unestablished peers contaminating update groups.
func TestUpdateGroupZeroContextID(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	peer := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 0)
	idx.Add(peer)

	assert.Len(t, idx.groups, 0, "ctxID=0 peer should not be added to any group")
}

// VALIDATES: Double-Remove safety -- peer.updateGroupKey cleared on first Remove.
// PREVENTS: Panic or corruption on redundant Remove calls.
func TestUpdateGroupDoubleRemove(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	peer := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 11)
	idx.Add(peer)

	key := GroupKey{CtxID: 11, PolicyKey: 0}
	require.Len(t, idx.groups[key].Members, 1, "peer should be in group after Add")

	idx.Remove(peer)
	assert.Len(t, idx.groups, 0, "group should be gone after first Remove")

	// Second Remove must not panic or corrupt state.
	assert.NotPanics(t, func() { idx.Remove(peer) }, "double Remove must not panic")
	assert.Len(t, idx.groups, 0, "index should still be empty after double Remove")
	assert.Equal(t, GroupKey{}, peer.updateGroupKey, "updateGroupKey should be zero after Remove")
}

// VALIDATES: Re-establishment with identical capabilities works.
// PREVENTS: Stale state from previous session interfering with new group membership.
func TestUpdateGroupReestablishSameCtxID(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	peer := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 12)
	idx.Add(peer)

	key := GroupKey{CtxID: 12, PolicyKey: 0}
	require.Len(t, idx.groups[key].Members, 1)

	// Simulate session teardown + re-establishment with same ctxID.
	idx.Remove(peer)
	assert.Len(t, idx.groups, 0, "group should be gone after Remove")

	// Re-add with same ctxID (same capabilities negotiated again).
	peer.sendCtxID = 12
	idx.Add(peer)

	group, ok := idx.groups[key]
	require.True(t, ok, "group should exist after re-Add")
	assert.Len(t, group.Members, 1, "should have exactly one member after re-establishment")
	assert.Equal(t, peer, group.Members[0], "member should be the re-added peer")
}

// VALIDATES: Mutex protects concurrent access correctly.
// PREVENTS: Data races between concurrent peer lifecycle events.
func TestUpdateGroupConcurrentAddRemove(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	const numPeers = 100
	peers := make([]*Peer, numPeers)
	for i := range peers {
		addr := netip.AddrFrom4([4]byte{10, 0, byte(i / 256), byte(i % 256)})
		// Use a small set of ctxIDs to force contention on the same groups.
		ctxID := bgpctx.ContextID((i % 5) + 1)
		peers[i] = testPeerWithCtxID(addr, ctxID)
	}

	var wg sync.WaitGroup

	// Launch goroutines that Add all peers concurrently.
	for i := range peers {
		wg.Add(1)
		go func(p *Peer) {
			defer wg.Done()
			idx.Add(p)
		}(peers[i])
	}
	wg.Wait()

	// All peers should be in groups -- 5 groups with 20 members each.
	assert.Len(t, idx.groups, 5, "should have 5 groups after concurrent Add")
	total := 0
	for _, g := range idx.groups {
		total += len(g.Members)
	}
	assert.Equal(t, numPeers, total, "total members should equal number of peers added")

	// Launch goroutines that Remove all peers concurrently.
	for i := range peers {
		wg.Add(1)
		go func(p *Peer) {
			defer wg.Done()
			idx.Remove(p)
		}(peers[i])
	}
	wg.Wait()

	assert.Len(t, idx.groups, 0, "all groups should be empty after concurrent Remove")
}

// VALIDATES: Empty input handled correctly.
// PREVENTS: Nil pointer dereference on empty peer list.
func TestUpdateGroupGroupsForPeersEmpty(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	// Add a peer so the index is non-empty, then query with empty slice.
	peer := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 13)
	idx.Add(peer)

	groups := idx.GroupsForPeers([]*Peer{})
	assert.Nil(t, groups, "GroupsForPeers with empty slice should return nil")
}

// VALIDATES: Unestablished peers filtered correctly.
// PREVENTS: Groups created for unestablished peers.
func TestUpdateGroupGroupsForPeersAllZeroCtxID(t *testing.T) {
	idx := NewUpdateGroupIndex(true)

	// Create peers with ctxID=0 (not established).
	peer1 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.1"), 0)
	peer2 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.2"), 0)
	peer3 := testPeerWithCtxID(netip.MustParseAddr("10.0.0.3"), 0)

	groups := idx.GroupsForPeers([]*Peer{peer1, peer2, peer3})
	assert.Nil(t, groups, "GroupsForPeers with all ctxID=0 peers should return nil")
}
