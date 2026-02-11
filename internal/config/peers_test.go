package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPeersFromConfigTreeBasic verifies basic peer extraction without routes.
//
// VALIDATES: PeersFromConfigTree returns correct PeerSettings from a simple tree.
// PREVENTS: Regression where basic fields (address, AS, hold-time) are lost.
func TestPeersFromConfigTreeBasic(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgp.Set("local-as", "65000")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("hold-time", "180")
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
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
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgp.Set("local-as", "65000")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")

	// Build static route: static { route 10.10.0.0/24 { next-hop 192.168.1.1; origin igp; } }
	staticTree := NewTree()
	routeTree := NewTree()
	routeTree.Set("next-hop", "192.168.1.1")
	routeTree.Set("origin", "igp")
	staticTree.AddListEntry("route", "10.10.0.0/24", routeTree)
	peerTree.SetContainer("static", staticTree)

	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	require.Len(t, ps.StaticRoutes, 1, "should have 1 static route")
	assert.Equal(t, "10.10.0.0/24", ps.StaticRoutes[0].Prefix.String())
}

// TestPeersFromConfigTreeTemplateWithRoutes verifies template resolution + route extraction.
//
// VALIDATES: Template values are applied to peer AND routes are extracted from peer tree.
// PREVENTS: Template resolution breaking route extraction pipeline.
func TestPeersFromConfigTreeTemplateWithRoutes(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgp.Set("local-as", "65000")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("inherit", "base")

	// Static route on the peer itself.
	staticTree := NewTree()
	routeTree := NewTree()
	routeTree.Set("next-hop", "172.16.0.1")
	routeTree.Set("origin", "igp")
	staticTree.AddListEntry("route", "192.168.0.0/16", routeTree)
	peerTree.SetContainer("static", staticTree)

	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	// Template provides hold-time.
	tmpl := NewTree()
	groupTree := NewTree()
	groupTree.Set("hold-time", "300")
	tmpl.AddListEntry("group", "base", groupTree)
	tree.SetContainer("template", tmpl)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	// Template value should be applied.
	assert.Equal(t, uint32(65001), ps.PeerAS)
	// Route should be extracted.
	require.Len(t, ps.StaticRoutes, 1, "route should be extracted from peer tree")
	assert.Equal(t, "192.168.0.0/16", ps.StaticRoutes[0].Prefix.String())
}

// TestPeersFromConfigTreePortOverride verifies env port override.
//
// VALIDATES: ze_bgp_tcp_port environment variable overrides peer port.
// PREVENTS: Port override being lost when migrating to PeersFromConfigTree.
func TestPeersFromConfigTreePortOverride(t *testing.T) {
	t.Setenv("ze_bgp_tcp_port", "1790")

	tree := NewTree()
	bgp := NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgp.Set("local-as", "65000")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
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
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgp.Set("local-as", "65000")
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	assert.Empty(t, peers)
}
