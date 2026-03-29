package reactor

import (
	"net/netip"
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
