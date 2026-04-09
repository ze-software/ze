package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// buildMinimalPeer creates a minimal peer tree with the new container structure.
func buildMinimalPeer(remoteIP, remoteAS, localIP string) *config.Tree {
	peerTree := config.NewTree()

	connTree := config.NewTree()
	connRemote := config.NewTree()
	connRemote.Set("ip", remoteIP)
	connTree.SetContainer("remote", connRemote)
	connLocal := config.NewTree()
	connLocal.Set("ip", localIP)
	connTree.SetContainer("local", connLocal)
	peerTree.SetContainer("connection", connTree)

	sessionTree := config.NewTree()
	asnTree := config.NewTree()
	asnTree.Set("remote", remoteAS)
	sessionTree.SetContainer("asn", asnTree)
	peerTree.SetContainer("session", sessionTree)

	return peerTree
}

// buildBGPBlock creates a minimal BGP block with router-id 1.2.3.4 and local AS 65000.
func buildBGPBlock() *config.Tree {
	bgp := config.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	sessionTree := config.NewTree()
	asnTree := config.NewTree()
	asnTree.Set("local", "65000")
	sessionTree.SetContainer("asn", asnTree)
	bgp.SetContainer("session", sessionTree)
	return bgp
}

// TestPeersFromConfigTreeBasic verifies basic peer extraction without routes.
//
// VALIDATES: PeersFromConfigTree returns correct PeerSettings from a simple tree.
// PREVENTS: Regression where basic fields (address, AS, receive-hold-time) are lost.
func TestPeersFromConfigTreeBasic(t *testing.T) {
	tree := config.NewTree()
	bgp := buildBGPBlock()

	peerTree := buildMinimalPeer("10.0.0.1", "65001", "auto")
	peerTimerTree := config.NewTree()
	peerTimerTree.Set("receive-hold-time", "180")
	peerTree.SetContainer("timer", peerTimerTree)

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
	bgp := buildBGPBlock()

	peerTree := buildMinimalPeer("10.0.0.1", "65001", "auto")

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
	bgp := buildBGPBlock()

	// Group with receive-hold-time default and remote AS.
	groupTree := config.NewTree()
	groupTimerTree := config.NewTree()
	groupTimerTree.Set("receive-hold-time", "300")
	groupTree.SetContainer("timer", groupTimerTree)
	groupSession := config.NewTree()
	groupASN := config.NewTree()
	groupASN.Set("remote", "65001")
	groupSession.SetContainer("asn", groupASN)
	groupTree.SetContainer("session", groupSession)

	// Peer inside group with a static route.
	peerTree := buildMinimalPeer("10.0.0.1", "65001", "auto")

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

// TestPeersFromConfigTreeNoPeers verifies no error when there are no peers.
//
// VALIDATES: Empty peer list returns empty slice, not error.
// PREVENTS: Panic on configs with no peers (validation-only configs).
func TestPeersFromConfigTreeNoPeers(t *testing.T) {
	tree := config.NewTree()
	bgp := buildBGPBlock()
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
	bgp := buildBGPBlock()

	// Group with a route at group level and remote AS default.
	group := config.NewTree()
	groupSession := config.NewTree()
	groupASN := config.NewTree()
	groupASN.Set("remote", "65001")
	groupSession.SetContainer("asn", groupASN)
	group.SetContainer("session", groupSession)

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
	peer := buildMinimalPeer("10.0.0.1", "65001", "127.0.0.1")

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
	bgp := buildBGPBlock()

	// Group A: has a route to 10.0.0.0/24
	groupA := config.NewTree()
	groupASession := config.NewTree()
	groupAASN := config.NewTree()
	groupAASN.Set("remote", "65001")
	groupASession.SetContainer("asn", groupAASN)
	groupA.SetContainer("session", groupASession)
	updateA := config.NewTree()
	attrA := config.NewTree()
	attrA.Set("origin", "igp")
	attrA.Set("next-hop", "1.1.1.1")
	updateA.SetContainer("attribute", attrA)
	nlriA := config.NewTree()
	nlriA.Set("content", "add 10.0.0.0/24")
	updateA.AddListEntry("nlri", "ipv4/unicast", nlriA)
	groupA.AddListEntry("update", "", updateA)
	peerA := buildMinimalPeer("10.0.0.1", "65001", "127.0.0.1")
	groupA.AddListEntry("peer", "peerA", peerA)
	bgp.AddListEntry("group", "group-a", groupA)

	// Group B: has a route to 20.0.0.0/24
	groupB := config.NewTree()
	groupBSession := config.NewTree()
	groupBASN := config.NewTree()
	groupBASN.Set("remote", "65002")
	groupBSession.SetContainer("asn", groupBASN)
	groupB.SetContainer("session", groupBSession)
	updateB := config.NewTree()
	attrB := config.NewTree()
	attrB.Set("origin", "igp")
	attrB.Set("next-hop", "2.2.2.2")
	updateB.SetContainer("attribute", attrB)
	nlriB := config.NewTree()
	nlriB.Set("content", "add 20.0.0.0/24")
	updateB.AddListEntry("nlri", "ipv4/unicast", nlriB)
	groupB.AddListEntry("update", "", updateB)
	peerB := buildMinimalPeer("10.0.0.2", "65002", "127.0.0.1")
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

// TestLoopDetectionConfigExtraction verifies loop-detection policy settings are applied to peers.
//
// VALIDATES: PeersFromConfigTree extracts allow-own-as from policy > loop-detection entries
//
//	and applies them to peers whose import filter chains reference the filter name.
//
// PREVENTS: Loop detection settings silently ignored, leaving peers with default (0) values.
func TestLoopDetectionConfigExtraction(t *testing.T) {
	tree := config.NewTree()
	bgp := buildBGPBlock()

	// Add a policy section with a loop-detection filter entry.
	policy := config.NewTree()
	ldEntry := config.NewTree()
	ldEntry.Set("allow-own-as", "2")
	policy.AddListEntry("loop-detection", "my-loop-filter", ldEntry)
	bgp.SetContainer("policy", policy)

	// Create a peer with an import filter chain referencing the loop-detection entry.
	peerTree := buildMinimalPeer("10.0.0.1", "65001", "auto")
	filterTree := config.NewTree()
	filterTree.SetSlice("import", []string{"my-loop-filter"})
	peerTree.SetContainer("filter", filterTree)

	bgp.AddListEntry("peer", "peer1", peerTree)
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	assert.Equal(t, uint8(2), ps.LoopAllowOwnAS, "allow-own-as should be extracted from loop-detection policy")
}

// TestLoopDetectionClusterID verifies cluster-id extraction from loop-detection policy.
//
// VALIDATES: PeersFromConfigTree extracts cluster-id from policy > loop-detection and
//
//	applies it to peers whose import filter chains reference the filter name.
//
// PREVENTS: Cluster-id override silently ignored, using router-id instead.
func TestLoopDetectionClusterID(t *testing.T) {
	tree := config.NewTree()
	bgp := buildBGPBlock()

	// Policy with loop-detection entry including cluster-id.
	policy := config.NewTree()
	ldEntry := config.NewTree()
	ldEntry.Set("cluster-id", "10.0.0.1")
	policy.AddListEntry("loop-detection", "ld-with-cluster", ldEntry)
	bgp.SetContainer("policy", policy)

	// Peer referencing the filter.
	peerTree := buildMinimalPeer("10.0.0.2", "65001", "auto")
	filterTree := config.NewTree()
	filterTree.SetSlice("import", []string{"ld-with-cluster"})
	peerTree.SetContainer("filter", filterTree)

	bgp.AddListEntry("peer", "peer1", peerTree)
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	// 10.0.0.1 = 0x0A000001
	assert.Equal(t, uint32(0x0A000001), ps.LoopClusterID, "cluster-id should be extracted from loop-detection policy")
}

// TestLoopDetectionUnreferencedFilter verifies unreferenced loop-detection entries are not applied.
//
// VALIDATES: Loop detection settings are only applied to peers that reference the filter by name.
// PREVENTS: Global application of loop-detection settings to all peers regardless of filter chain.
func TestLoopDetectionUnreferencedFilter(t *testing.T) {
	tree := config.NewTree()
	bgp := buildBGPBlock()

	// Policy with loop-detection entry.
	policy := config.NewTree()
	ldEntry := config.NewTree()
	ldEntry.Set("allow-own-as", "3")
	policy.AddListEntry("loop-detection", "unused-filter", ldEntry)
	bgp.SetContainer("policy", policy)

	// Peer without any filter chain referencing the loop-detection entry.
	peerTree := buildMinimalPeer("10.0.0.1", "65001", "auto")
	bgp.AddListEntry("peer", "peer1", peerTree)
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	assert.Equal(t, uint8(0), ps.LoopAllowOwnAS, "unreferenced filter should not affect peer settings")
}
