package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func graphTestSchema(t *testing.T) *Schema {
	t.Helper()
	schema, err := YANGSchema()
	require.NoError(t, err)
	return schema
}

func findNode(g *Graph, id string) *GraphNode {
	for i := range g.Nodes {
		if g.Nodes[i].ID == id {
			return &g.Nodes[i]
		}
	}
	return nil
}

func hasEdge(g *Graph, from, to string, kind GraphEdgeKind) bool {
	for _, e := range g.Edges {
		if e.From == from && e.To == to && e.Kind == kind {
			return true
		}
	}
	return false
}

func TestBuildGraph_SectionNodes(t *testing.T) {
	schema := graphTestSchema(t)
	tree := NewTree()
	tree.SetContainer("bgp", NewTree())
	tree.SetContainer("web", NewTree())

	g := BuildGraph(tree, schema)

	assert.NotNil(t, findNode(g, "section/bgp"))
	assert.NotNil(t, findNode(g, "section/web"))
}

func TestBuildGraph_StandalonePeers(t *testing.T) {
	schema := graphTestSchema(t)
	tree := NewTree()

	bgp := NewTree()
	peer1 := NewTree()
	bgp.AddListEntry("peer", "upstream1", peer1)
	tree.SetContainer("bgp", bgp)

	g := BuildGraph(tree, schema)

	assert.NotNil(t, findNode(g, "peer/upstream1"))
	assert.True(t, hasEdge(g, "section/bgp", "peer/upstream1", EdgeContains))
}

func TestBuildGraph_GroupInheritance(t *testing.T) {
	schema := graphTestSchema(t)
	tree := NewTree()

	bgp := NewTree()
	group := NewTree()
	peer := NewTree()
	group.AddListEntry("peer", "peer-a", peer)
	bgp.AddListEntry("group", "transit", group)
	tree.SetContainer("bgp", bgp)

	g := BuildGraph(tree, schema)

	assert.NotNil(t, findNode(g, "group/transit"))
	assert.NotNil(t, findNode(g, "peer/peer-a"))
	assert.True(t, hasEdge(g, "section/bgp", "group/transit", EdgeContains))
	assert.True(t, hasEdge(g, "peer/peer-a", "group/transit", EdgeInherits))
}

func TestBuildGraph_ProcessBindings(t *testing.T) {
	schema := graphTestSchema(t)
	tree := NewTree()

	bgp := NewTree()
	peer := NewTree()
	proc := NewTree()
	proc.Set("run", "/usr/local/bin/my-filter")
	peer.GetOrCreateContainer("attach").AddListEntry("process", "my-filter", proc)
	bgp.AddListEntry("peer", "peer-x", peer)
	tree.SetContainer("bgp", bgp)

	g := BuildGraph(tree, schema)

	assert.NotNil(t, findNode(g, "plugin/my-filter"))
	assert.True(t, hasEdge(g, "peer/peer-x", "plugin/my-filter", EdgeProcessBinds))
}

func TestBuildGraph_AuthzProfileReferences(t *testing.T) {
	schema := graphTestSchema(t)
	tree := NewTree()

	sys := NewTree()
	authz := NewTree()
	profile := NewTree()
	authz.AddListEntry("profile", "admin", profile)
	sys.SetContainer("authorization", authz)

	auth := NewTree()
	user := NewTree()
	user.SetSlice("profile", []string{"admin"})
	auth.AddListEntry("user", "alice", user)
	sys.SetContainer("authentication", auth)

	tree.SetContainer("system", sys)

	g := BuildGraph(tree, schema)

	assert.NotNil(t, findNode(g, "profile/admin"))
	assert.NotNil(t, findNode(g, "user/alice"))
	assert.True(t, hasEdge(g, "section/system", "profile/admin", EdgeContains))
	assert.True(t, hasEdge(g, "section/system", "user/alice", EdgeContains))
	assert.True(t, hasEdge(g, "user/alice", "profile/admin", EdgeReferences))
}

func TestBuildGraph_ListenerEndpoints(t *testing.T) {
	schema := graphTestSchema(t)
	tree := NewTree()

	env := NewTree()
	web := NewTree()
	web.Set("enabled", "true")
	srv := NewTree()
	srv.Set("ip", "0.0.0.0")
	srv.Set("port", "8443")
	web.AddListEntry("server", "default", srv)
	env.SetContainer("web", web)
	tree.SetContainer("environment", env)

	g := BuildGraph(tree, schema)

	n := findNode(g, "listener/web default")
	assert.NotNil(t, n)
	assert.True(t, hasEdge(g, "section/web", "listener/web default", EdgeListensOn),
		"web section should own its listener")
}

func TestBuildGraph_PluginRegistryEdges(t *testing.T) {
	schema := graphTestSchema(t)
	tree := NewTree()
	tree.SetContainer("bgp", NewTree())

	g := BuildGraph(tree, schema)

	assert.NotNil(t, findNode(g, "plugin/bgp"), "bgp plugin should exist from registry")
	assert.True(t, hasEdge(g, "plugin/bgp", "section/bgp", EdgeConfigRoot))
}

func TestBuildGraph_NoDuplicateNodes(t *testing.T) {
	schema := graphTestSchema(t)
	tree := NewTree()

	bgp := NewTree()
	group := NewTree()
	p1 := NewTree()
	proc1 := NewTree()
	proc1.Set("use", "bgp-rib")
	p1.GetOrCreateContainer("attach").AddListEntry("process", "bgp-rib", proc1)
	group.AddListEntry("peer", "peer1", p1)

	p2 := NewTree()
	proc2 := NewTree()
	proc2.Set("use", "bgp-rib")
	p2.GetOrCreateContainer("attach").AddListEntry("process", "bgp-rib", proc2)
	group.AddListEntry("peer", "peer2", p2)
	bgp.AddListEntry("group", "grp", group)
	tree.SetContainer("bgp", bgp)

	g := BuildGraph(tree, schema)

	count := 0
	for _, n := range g.Nodes {
		if n.ID == "plugin/bgp-rib" {
			count++
		}
	}
	assert.Equal(t, 1, count, "plugin/bgp-rib should appear exactly once")
}

func TestGraphAddressNodes(t *testing.T) {
	schema := graphTestSchema(t)
	tree := NewTree()

	// Build interface section with a dummy interface that has an address.
	iface := NewTree()
	dum0 := NewTree()
	unit := NewTree()
	ipv4 := NewTree()
	ipv4.SetSlice("address", []string{"10.0.0.1/32"})
	unit.SetContainer("ipv4", ipv4)
	dum0.AddListEntry("unit", "default", unit)
	iface.AddListEntry("dummy", "dum0", dum0)
	tree.SetContainer("interface", iface)

	// Build a BGP peer whose connection > local > ip matches the address.
	bgp := NewTree()
	peer := NewTree()
	conn := NewTree()
	local := NewTree()
	local.Set("ip", "10.0.0.1")
	conn.SetContainer("local", local)
	peer.SetContainer("connection", conn)
	bgp.AddListEntry("peer", "upstream1", peer)
	tree.SetContainer("bgp", bgp)

	g := BuildGraph(tree, schema)

	// Verify the address node exists with the correct kind.
	addrNode := findNode(g, "address/dum0/10.0.0.1/32")
	require.NotNil(t, addrNode, "address/dum0/10.0.0.1/32 node must exist")
	assert.Equal(t, NodeAddress, addrNode.Kind)

	// Verify EdgeContains from section/interface to the address node.
	assert.True(t, hasEdge(g, "section/interface", "address/dum0/10.0.0.1/32", EdgeContains),
		"section/interface should contain the address node")

	// Verify EdgeUsesAddress from the peer to the address node.
	assert.True(t, hasEdge(g, "peer/upstream1", "address/dum0/10.0.0.1/32", EdgeUsesAddress),
		"peer should have uses-address edge to matching address node")
}

func TestBuildGraph_EmptyConfig(t *testing.T) {
	schema := graphTestSchema(t)
	tree := NewTree()

	g := BuildGraph(tree, schema)

	assert.NotNil(t, g)
	// Empty tree has no explicit sections, but plugin registry edges create
	// implicit section nodes (e.g., section/bgp from the bgp plugin's config root).
	// Verify the graph is self-consistent: no edge references a missing node.
	nodeIDs := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeIDs[n.ID] = true
	}
	for _, e := range g.Edges {
		assert.True(t, nodeIDs[e.From], "edge from %s has no node", e.From)
		assert.True(t, nodeIDs[e.To], "edge to %s has no node", e.To)
	}
}
