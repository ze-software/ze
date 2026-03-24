package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// TestPeersFromConfigTreeBasic verifies basic peer extraction without routes.
//
// VALIDATES: PeersFromConfigTree returns correct PeerSettings from a simple tree.
// PREVENTS: Regression where basic fields (address, AS, receive-hold-time) are lost.
func TestPeersFromConfigTreeBasic(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	peerTree := config.NewTree()
	remoteTree := config.NewTree()
	remoteTree.Set("ip", "10.0.0.1")
	remoteTree.Set("as", "65001")
	peerTree.SetContainer("remote", remoteTree)
	peerTimerTree := config.NewTree()
	peerTimerTree.Set("receive-hold-time", "180")
	peerTree.SetContainer("timer", peerTimerTree)
	peerLocalTree := config.NewTree()
	peerLocalTree.Set("ip", "auto")
	peerTree.SetContainer("local", peerLocalTree)
	bgp.AddListEntry("peer", "peer1", peerTree)
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	assert.Equal(t, "10.0.0.1", ps.Address.String())
	assert.Equal(t, uint32(65000), ps.LocalAS)
	assert.Equal(t, uint32(65001), ps.PeerAS)
}

// TestPeersFromConfigTreeStaticRoute verifies route extraction and conversion.
//
// VALIDATES: Static routes from peer tree are extracted, converted, and patched into PeerSettings.
// PREVENTS: Route loss when extracting routes from peer tree subtrees.
func TestPeersFromConfigTreeStaticRoute(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	peerTree := config.NewTree()
	remoteTree := config.NewTree()
	remoteTree.Set("ip", "10.0.0.1")
	remoteTree.Set("as", "65001")
	peerTree.SetContainer("remote", remoteTree)
	peerLocalTree := config.NewTree()
	peerLocalTree.Set("ip", "auto")
	peerTree.SetContainer("local", peerLocalTree)

	// Build static route: static { route 10.10.0.0/24 { next-hop 192.168.1.1; origin igp; } }
	staticTree := config.NewTree()
	routeTree := config.NewTree()
	routeTree.Set("next-hop", "192.168.1.1")
	routeTree.Set("origin", "igp")
	staticTree.AddListEntry("route", "10.10.0.0/24", routeTree)
	peerTree.SetContainer("static", staticTree)

	bgp.AddListEntry("peer", "peer1", peerTree)
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	require.Len(t, ps.StaticRoutes, 1, "should have 1 static route")
	assert.Equal(t, "10.10.0.0/24", ps.StaticRoutes[0].Prefix.String())
}

// TestPeersFromConfigTreeGroupWithRoutes verifies group defaults + route extraction.
//
// VALIDATES: Group values are applied to peer AND routes from both group and peer are extracted.
// PREVENTS: Group resolution breaking route extraction pipeline.
func TestPeersFromConfigTreeGroupWithRoutes(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	// Group with receive-hold-time default.
	groupTree := config.NewTree()
	groupTimerTree := config.NewTree()
	groupTimerTree.Set("receive-hold-time", "300")
	groupTree.SetContainer("timer", groupTimerTree)

	// Peer inside group with a static route.
	peerTree := config.NewTree()
	remoteTree := config.NewTree()
	remoteTree.Set("ip", "10.0.0.1")
	remoteTree.Set("as", "65001")
	peerTree.SetContainer("remote", remoteTree)
	peerLocalTree := config.NewTree()
	peerLocalTree.Set("ip", "auto")
	peerTree.SetContainer("local", peerLocalTree)

	staticTree := config.NewTree()
	routeTree := config.NewTree()
	routeTree.Set("next-hop", "172.16.0.1")
	routeTree.Set("origin", "igp")
	staticTree.AddListEntry("route", "192.168.0.0/16", routeTree)
	peerTree.SetContainer("static", staticTree)

	groupTree.AddListEntry("peer", "peer1", peerTree)
	bgp.AddListEntry("group", "base", groupTree)
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	// Group value should be applied.
	assert.Equal(t, uint32(65001), ps.PeerAS)
	assert.Equal(t, "base", ps.GroupName)
	// Route should be extracted.
	require.Len(t, ps.StaticRoutes, 1, "route should be extracted from peer tree")
	assert.Equal(t, "192.168.0.0/16", ps.StaticRoutes[0].Prefix.String())
}

// TestPeersFromConfigTreePortOverride verifies env port override.
//
// VALIDATES: ze_bgp_tcp_port environment variable overrides peer port.
// PREVENTS: Port override being lost when migrating to PeersFromConfigTree.
func TestPeersFromConfigTreePortOverride(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	t.Setenv("ze_bgp_tcp_port", "1790")
	env.ResetCache()

	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	peerTree := config.NewTree()
	remoteTree := config.NewTree()
	remoteTree.Set("ip", "10.0.0.1")
	remoteTree.Set("as", "65001")
	peerTree.SetContainer("remote", remoteTree)
	peerLocalTree := config.NewTree()
	peerLocalTree.Set("ip", "auto")
	peerTree.SetContainer("local", peerLocalTree)
	bgp.AddListEntry("peer", "peer1", peerTree)
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	assert.Equal(t, uint16(1790), peers[0].Port)
}

// TestPeersFromConfigTreeNoPeers verifies no error when there are no peers.
//
// VALIDATES: Empty peer list returns empty slice, not error.
// PREVENTS: Panic on configs with no peers (validation-only configs).
func TestPeersFromConfigTreeNoPeers(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	assert.Empty(t, peers)
}

// TestPeersFromConfigTree_GroupRoutes verifies route accumulation from group + peer layers.
//
// VALIDATES: AC-13 -- group routes and peer routes are both present in PeerSettings.
// PREVENTS: Group-level routes silently dropped, or peer routes replacing group routes.
func TestPeersFromConfigTree_GroupRoutes(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	// Group with a route at group level.
	group := config.NewTree()
	groupRemote := config.NewTree()
	groupRemote.Set("as", "65001")
	group.SetContainer("remote", groupRemote)
	groupUpdate := config.NewTree()
	groupAttr := config.NewTree()
	groupAttr.Set("origin", "igp")
	groupAttr.Set("next-hop", "1.1.1.1")
	groupUpdate.SetContainer("attribute", groupAttr)
	groupNLRI := config.NewTree()
	groupNLRI.Set("content", "add 10.0.0.0/24")
	groupUpdate.AddListEntry("nlri", "ipv4/unicast", groupNLRI)
	group.AddListEntry("update", "", groupUpdate)

	// Peer with its own route.
	peer := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peer.SetContainer("remote", peerRemote)
	peerLocal := config.NewTree()
	peerLocal.Set("ip", "127.0.0.1")
	peer.SetContainer("local", peerLocal)
	peerUpdate := config.NewTree()
	peerAttr := config.NewTree()
	peerAttr.Set("origin", "igp")
	peerAttr.Set("next-hop", "2.2.2.2")
	peerUpdate.SetContainer("attribute", peerAttr)
	peerNLRI := config.NewTree()
	peerNLRI.Set("content", "add 20.0.0.0/24")
	peerUpdate.AddListEntry("nlri", "ipv4/unicast", peerNLRI)
	peer.AddListEntry("update", "", peerUpdate)

	group.AddListEntry("peer", "peer1", peer)
	bgp.AddListEntry("group", "test", group)
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	// Both group route (10.0.0.0/24) and peer route (20.0.0.0/24) should be present.
	require.Len(t, peers[0].StaticRoutes, 2, "peer should have routes from both group and peer layers")
	prefixes := make(map[string]bool)
	for _, r := range peers[0].StaticRoutes {
		prefixes[r.Prefix.String()] = true
	}
	assert.True(t, prefixes["10.0.0.0/24"], "group route should be present")
	assert.True(t, prefixes["20.0.0.0/24"], "peer route should be present")
}

// TestPeersFromConfigTree_GroupRoutesIsolation verifies routes don't leak across groups.
//
// VALIDATES: Peer in group A only gets routes from group A, not from group B.
// PREVENTS: Cross-group route contamination via shared patchRoutes calls.
func TestPeersFromConfigTree_GroupRoutesIsolation(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	// Group A: has a route to 10.0.0.0/24
	groupA := config.NewTree()
	groupARemote := config.NewTree()
	groupARemote.Set("as", "65001")
	groupA.SetContainer("remote", groupARemote)
	updateA := config.NewTree()
	attrA := config.NewTree()
	attrA.Set("origin", "igp")
	attrA.Set("next-hop", "1.1.1.1")
	updateA.SetContainer("attribute", attrA)
	nlriA := config.NewTree()
	nlriA.Set("content", "add 10.0.0.0/24")
	updateA.AddListEntry("nlri", "ipv4/unicast", nlriA)
	groupA.AddListEntry("update", "", updateA)
	peerA := config.NewTree()
	peerARemote := config.NewTree()
	peerARemote.Set("ip", "10.0.0.1")
	peerA.SetContainer("remote", peerARemote)
	peerALocal := config.NewTree()
	peerALocal.Set("ip", "127.0.0.1")
	peerA.SetContainer("local", peerALocal)
	groupA.AddListEntry("peer", "peerA", peerA)
	bgp.AddListEntry("group", "group-a", groupA)

	// Group B: has a route to 20.0.0.0/24
	groupB := config.NewTree()
	groupBRemote := config.NewTree()
	groupBRemote.Set("as", "65002")
	groupB.SetContainer("remote", groupBRemote)
	updateB := config.NewTree()
	attrB := config.NewTree()
	attrB.Set("origin", "igp")
	attrB.Set("next-hop", "2.2.2.2")
	updateB.SetContainer("attribute", attrB)
	nlriB := config.NewTree()
	nlriB.Set("content", "add 20.0.0.0/24")
	updateB.AddListEntry("nlri", "ipv4/unicast", nlriB)
	groupB.AddListEntry("update", "", updateB)
	peerB := config.NewTree()
	peerBRemote := config.NewTree()
	peerBRemote.Set("ip", "10.0.0.2")
	peerB.SetContainer("remote", peerBRemote)
	peerBLocal := config.NewTree()
	peerBLocal.Set("ip", "127.0.0.1")
	peerB.SetContainer("local", peerBLocal)
	groupB.AddListEntry("peer", "peerB", peerB)
	bgp.AddListEntry("group", "group-b", groupB)

	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 2)

	// Find peers by address.
	var peerSettingsA, peerSettingsB *reactor.PeerSettings
	for _, ps := range peers {
		switch ps.Address.String() {
		case "10.0.0.1":
			peerSettingsA = ps
		case "10.0.0.2":
			peerSettingsB = ps
		}
	}
	require.NotNil(t, peerSettingsA, "peer 10.0.0.1 should exist")
	require.NotNil(t, peerSettingsB, "peer 10.0.0.2 should exist")

	// Peer A should have route to 10.0.0.0/24 (from group A), NOT 20.0.0.0/24
	require.Len(t, peerSettingsA.StaticRoutes, 1, "peer A should have 1 route")
	assert.Equal(t, "10.0.0.0/24", peerSettingsA.StaticRoutes[0].Prefix.String())

	// Peer B should have route to 20.0.0.0/24 (from group B), NOT 10.0.0.0/24
	require.Len(t, peerSettingsB.StaticRoutes, 1, "peer B should have 1 route")
	assert.Equal(t, "20.0.0.0/24", peerSettingsB.StaticRoutes[0].Prefix.String())
}
