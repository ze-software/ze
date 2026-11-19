package fixture

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chaosscenario "github.com/ze-software/ze/internal/chaos/scenario"
)

// The two graphs in the text forms renderGraphText and renderNextHopGraphText
// write (internal/component/lg/graph.go, graph_nexthop.go): a node line with
// its layer, an AS name between the AS and the layer, `egress` and `name=` on
// a next-hop node of an address, `outside` on a next-hop node of an AS, then
// the edges.
const (
	lgSimulatedASPathSample = "mode aspath\n" +
		"node AS65101 Transit A layer=1\n" +
		"node AS65201 layer=0\n" +
		"edge AS65101 -> AS65201\n"
	lgSimulatedNextHopSample = "mode nexthop\n" +
		"node 198.51.100.3 layer=1\n" +
		"node 198.51.100.2 layer=0 egress name=border\n" +
		"node AS65102 layer=0 outside\n" +
		"edge 198.51.100.3 -> 198.51.100.2\n"
)

// lgNextHopUnchangedTopology answers the model of the hand-written network.
func lgNextHopUnchangedTopology(t *testing.T) *chaosscenario.Topology {
	t.Helper()
	topology, err := chaosscenario.NewTopology(lgNextHopUnchangedSpec())
	require.NoError(t, err)
	return topology
}

// VALIDATES: the reader takes the nodes, the egress marks and the edges of
// both text forms, whatever follows a node's name, and sorts them.
// PREVENTS: an AS name or a `name=` read as part of the node.
func TestLGSimulatedReadGraphReadsBothModes(t *testing.T) {
	asPath, err := lgSimulatedReadGraph(lgSimulatedASPathSample, lgSimulatedGraphASPath)
	require.NoError(t, err)
	assert.Equal(t, lgSimulatedGraphText{
		nodes: []string{"AS65101", "AS65201"},
		edges: []string{"AS65101 -> AS65201"},
	}, asPath)

	nextHop, err := lgSimulatedReadGraph(lgSimulatedNextHopSample, lgSimulatedGraphNextHop)
	require.NoError(t, err)
	assert.Equal(t, lgSimulatedGraphText{
		nodes:   []string{"198.51.100.2", "198.51.100.3", "AS65102"},
		egress:  []string{"198.51.100.2"},
		outside: []string{"AS65102"},
		edges:   []string{"198.51.100.3 -> 198.51.100.2"},
	}, nextHop)
}

// VALIDATES: a text of another mode, the empty answer, an unknown line and a
// malformed edge are each refused.
// PREVENTS: "No routes found" or an error page read as a graph with no node,
// which would compare equal to a model that expects none.
func TestLGSimulatedReadGraphRefusesWhatIsNotTheGraph(t *testing.T) {
	_, err := lgSimulatedReadGraph(lgSimulatedGraphNoRoutes, lgSimulatedGraphASPath)
	require.ErrorContains(t, err, "does not start with")

	_, err = lgSimulatedReadGraph(lgSimulatedNextHopSample, lgSimulatedGraphASPath)
	require.ErrorContains(t, err, "does not start with")

	_, err = lgSimulatedReadGraph("mode aspath\ntrace AS65101\n", lgSimulatedGraphASPath)
	require.ErrorContains(t, err, "no node and no edge")

	_, err = lgSimulatedReadGraph("mode aspath\nedge AS65101 AS65201\n", lgSimulatedGraphASPath)
	require.ErrorContains(t, err, "is not `edge <from> -> <to>`")
}

// VALIDATES: the graphs expected of a network are the model's announcements:
// for the prefix whose next hop a border router keeps, every router points at
// the neighbor, which is outside the AS and so is the node of its AS, and no
// router is egress; for the prefix the other border router rewrites, that
// router is egress, the two others point at it, and the neighbor that
// announces the prefix is the node of its AS. A neighbor's address is no node.
// PREVENTS: an expectation that states the next-hop-self shape for every
// prefix, which a next-hop graph of the kept prefix would never match, and
// one that expects a speaker outside the AS by its address.
func TestLGSimulatedGraphWantFollowsTheModel(t *testing.T) {
	topology := lgNextHopUnchangedTopology(t)

	assert.Equal(t, lgSimulatedGraphText{
		nodes:   []string{"198.51.100.1", "198.51.100.2", "198.51.100.3", "AS65101"},
		outside: []string{"AS65101"},
		edges: []string{
			"198.51.100.1 -> AS65101", "198.51.100.2 -> AS65101", "198.51.100.3 -> AS65101",
		},
	}, lgSimulatedNextHopWant(topology, lgNextHopUnchangedPrefixKept))
	assert.Equal(t, lgSimulatedGraphText{
		nodes:   []string{"198.51.100.1", "198.51.100.2", "198.51.100.3", "AS65102"},
		egress:  []string{"198.51.100.2"},
		outside: []string{"AS65102"},
		edges:   []string{"198.51.100.1 -> 198.51.100.2", "198.51.100.3 -> 198.51.100.2"},
	}, lgSimulatedNextHopWant(topology, lgNextHopUnchangedPrefixSelf))

	assert.Equal(t, lgSimulatedGraphText{
		nodes: []string{"AS65101", "AS65201"},
		edges: []string{"AS65101 -> AS65201"},
	}, lgSimulatedASPathWant(topology, lgNextHopUnchangedPrefixKept))
	assert.Equal(t, lgSimulatedGraphText{nodes: []string{"AS65102"}}, lgSimulatedASPathWant(topology, lgNextHopUnchangedPrefixSelf))
}

// VALIDATES: a graph that differs from the model's in a node, an egress mark
// or an edge is named as that difference.
// PREVENTS: a comparison that passes a graph with a missing edge.
func TestLGSimulatedGraphProblemNamesTheDifference(t *testing.T) {
	want := lgSimulatedNextHopWant(lgNextHopUnchangedTopology(t), lgNextHopUnchangedPrefixSelf)
	assert.Empty(t, lgSimulatedGraphProblem(want, want))

	fewer := want
	fewer.nodes = want.nodes[1:]
	assert.Contains(t, lgSimulatedGraphProblem(fewer, want), "holds the nodes")

	unmarked := want
	unmarked.egress = nil
	assert.Contains(t, lgSimulatedGraphProblem(unmarked, want), "as egress")

	// A looking glass that drew the neighbor as an exit of the AS: the node
	// is there, and it carries no outside mark.
	inside := want
	inside.outside = nil
	assert.Contains(t, lgSimulatedGraphProblem(inside, want), "as outside")

	unlinked := want
	unlinked.edges = want.edges[:1]
	assert.Contains(t, lgSimulatedGraphProblem(unlinked, want), "holds the edges")
}

// VALIDATES: A-10 -- the lines written by hand for the hand-written network
// are what the model computes for it, for every router and both prefixes, and
// the network holds a walk that leaves toward an external address beside one
// that ends on a router's own address.
// PREVENTS: a hand check and a model that disagree before any network runs,
// and a spec the model refuses.
func TestLGNextHopUnchangedHandCheckedAgreesWithTheModel(t *testing.T) {
	topology := lgNextHopUnchangedTopology(t)
	ends := map[chaosscenario.TraceEnd]int{}

	for prefix, lines := range lgNextHopUnchangedHandChecked() {
		require.Len(t, lines, 3)
		for router, line := range lines {
			model, err := topology.Trace(netip.MustParseAddr(router), prefix)
			require.NoError(t, err)
			want, err := lgSimulatedTraceWant(model)
			require.NoError(t, err)
			assert.Equal(t, want, line, "prefix %s from %s", prefix, router)
			ends[model.End]++
		}
	}
	assert.Equal(t, 3, ends[chaosscenario.TraceEndExitExternalAddress])
	assert.Equal(t, 3, ends[chaosscenario.TraceEndExitOwnAddress])
}

// lgNextHopUnchangedKeptText is the trace graph of the kept prefix as the looking
// glass must write it: every router leaves toward the neighbor's address,
// which each walk's line states and no node carries.
const lgNextHopUnchangedKeptText = "mode trace\n" +
	"node router 198.51.100.1 exit\n" +
	"node router 198.51.100.2 exit\n" +
	"node router 198.51.100.3 exit\n" +
	"node AS65101\n" +
	"node AS65201\n" +
	"trace ipv4/unicast 198.18.0.0/24 198.51.100.1 routers=198.51.100.1 end=exit-external-address" +
	" next-hop=203.0.113.1 as-path=65101,65201 says: leaves the AS toward 203.0.113.1\n" +
	"trace ipv4/unicast 198.18.0.0/24 198.51.100.2 routers=198.51.100.2 end=exit-external-address" +
	" next-hop=203.0.113.1 as-path=65101,65201 says: leaves the AS toward 203.0.113.1\n" +
	"trace ipv4/unicast 198.18.0.0/24 198.51.100.3 routers=198.51.100.3 end=exit-external-address" +
	" next-hop=203.0.113.1 as-path=65101,65201 says: leaves the AS toward 203.0.113.1\n"

// VALIDATES: the hand check accepts the kept prefix's graph, and refuses one
// where the border router's walk ends on its own address, one with a router's
// walk missing, and a text that is no trace graph.
// PREVENTS: a looking glass that reads the kept next hop as next-hop-self
// passing the network run.
func TestLGNextHopUnchangedTraceProblemRefusesTheRewrittenNextHop(t *testing.T) {
	want := lgNextHopUnchangedHandChecked()[lgNextHopUnchangedPrefixKept]
	assert.Empty(t, lgNextHopUnchangedTraceProblem(lgNextHopUnchangedKeptText, lgNextHopUnchangedPrefixKept, want))

	rewritten := strings.Replace(lgNextHopUnchangedKeptText,
		"198.51.100.1 routers=198.51.100.1 end=exit-external-address next-hop=203.0.113.1",
		"198.51.100.1 routers=198.51.100.1 end=exit-own-address next-hop=198.51.100.1", 1)
	require.NotEqual(t, lgNextHopUnchangedKeptText, rewritten)
	assert.Contains(t, lgNextHopUnchangedTraceProblem(rewritten, lgNextHopUnchangedPrefixKept, want),
		`the trace from 198.51.100.1 ends as "exit-own-address"`)

	lines := strings.SplitAfter(lgNextHopUnchangedKeptText, "\n")
	missing := strings.Join(lines[:len(lines)-2], "")
	assert.Contains(t, lgNextHopUnchangedTraceProblem(missing, lgNextHopUnchangedPrefixKept, want), "holds 2 traces")

	assert.Contains(t, lgNextHopUnchangedTraceProblem(lgSimulatedGraphNoRoutes, lgNextHopUnchangedPrefixKept, want), "does not start with")
}
