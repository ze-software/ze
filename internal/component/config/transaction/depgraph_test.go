package transaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The operations the two decomposers emit, spelled here because this package
// cannot import them: iface and bgp both import the transaction package. The
// real declarations are pinned where they are written, by
// TestIfaceOperationsDeclareProduceAndConsume and
// TestBGPOperationsDeclareConsumeAddress, so a decomposer that stops declaring
// a resource reddens its own package rather than only this fixture.

func fixtureAddInterface(name string) ConfigOperation {
	return ConfigOperation{
		ID: "interface-add-" + name, Root: "interface", Owner: "interface",
		Type: testOpAddInterface, Verb: VerbCreate,
		Target:   ResourceRef{Kind: ResourceInterface, Name: name},
		Produces: []ResourceRef{{Kind: ResourceInterface, Name: name}},
		Params:   ConfigOperationParams{Name: name, Property: "dummy"},
	}
}

func fixtureRemoveInterface(name string) ConfigOperation {
	return ConfigOperation{
		ID: "interface-remove-" + name, Root: "interface", Owner: "interface",
		Type: testOpRemoveInterface, Verb: VerbDestroy,
		Target:   ResourceRef{Kind: ResourceInterface, Name: name},
		Produces: []ResourceRef{{Kind: ResourceInterface, Name: name}},
		Params:   ConfigOperationParams{Name: name, Property: "dummy"},
	}
}

func fixtureAddAddress(ifaceName, cidr string) ConfigOperation {
	return ConfigOperation{
		ID: "interface-add-address-" + ifaceName + "-" + cidr, Root: "interface", Owner: "interface",
		Type: testOpAddAddress, Verb: VerbCreate,
		Target:   ResourceRef{Kind: ResourceAddress, Interface: ifaceName, Address: cidr},
		Produces: []ResourceRef{{Kind: ResourceAddress, Address: cidr}},
		Consumes: []ResourceRef{{Kind: ResourceInterface, Name: ifaceName}},
		Params:   ConfigOperationParams{Interface: ifaceName, CIDR: cidr},
	}
}

func fixtureRemoveAddress(ifaceName, cidr string) ConfigOperation {
	return ConfigOperation{
		ID: "interface-remove-address-" + ifaceName + "-" + cidr, Root: "interface", Owner: "interface",
		Type: testOpRemoveAddress, Verb: VerbDestroy,
		Target:   ResourceRef{Kind: ResourceAddress, Interface: ifaceName, Address: cidr},
		Produces: []ResourceRef{{Kind: ResourceAddress, Address: cidr}},
		Consumes: []ResourceRef{{Kind: ResourceInterface, Name: ifaceName}},
		Params:   ConfigOperationParams{Interface: ifaceName, CIDR: cidr},
	}
}

func fixturePeer(id string, opType ConfigOperationType, verb OperationVerb, name, localAddress string) ConfigOperation {
	return ConfigOperation{
		ID: id, Root: "bgp", Owner: "bgp",
		Type: opType, Verb: verb,
		Target:   ResourceRef{Kind: ResourcePeer, Peer: name, Address: localAddress},
		Produces: []ResourceRef{{Kind: ResourcePeer, Peer: name}},
		Consumes: []ResourceRef{{Kind: ResourceAddress, Address: localAddress}},
		Params:   ConfigOperationParams{Peer: name, Address: localAddress},
	}
}

// The one rule that survived the derivation. It states no produce and consume
// fact: it relates two operations over DIFFERENT resources, which no pair of
// declarations can express. Every address this commit removes leaves before
// any address this commit adds arrives, which is phases 3 and 4 of the
// requirement (docs/architecture/config/apply-ordering.md).
func survivingIfaceRules() []ConstraintRule {
	return []ConstraintRule{{
		ID:       "iface-remove-address-before-add-address",
		Before:   OperationSelector{Type: testOpRemoveAddress, ResourceKind: ResourceAddress},
		After:    OperationSelector{Type: testOpAddAddress, ResourceKind: ResourceAddress},
		Relation: ResourceRelationAny,
	}}
}

func graphEdgePairs(t *testing.T, graph *OperationGraph) []string {
	t.Helper()
	pairs := make([]string, 0, len(graph.edges))
	for _, edge := range graph.edges {
		pairs = append(pairs, edge.FromID+" -> "+edge.ToID)
	}
	return pairs
}

// TestBuildOperationGraphDerivesProducerBeforeConsumer verifies that a create
// runs before every operation consuming what it produces, with no constraint
// rule registered.
//
// It replaces TestDependencyGraphSimple, which asserted the same three facts
// over the two hand-written rules this spec deleted. The chain and the
// no-transitive-edge assertion are unchanged; the mechanism under them is the
// declaration each operation carries.
//
// VALIDATES: add-interface -> add-address -> add-peer from Produces and Consumes alone.
// PREVENTS: a root joining the ordering only by registering a rule that names another root's labels.
func TestBuildOperationGraphDerivesProducerBeforeConsumer(t *testing.T) {
	t.Parallel()

	ops := []ConfigOperation{
		fixturePeer("peer-add", testOpAddPeer, VerbCreate, "203.0.113.1", "192.0.2.1"),
		fixtureAddAddress("eth0", "192.0.2.1/32"),
		fixtureAddInterface("eth0"),
	}

	graph, err := BuildOperationGraph(ops, nil)
	require.NoError(t, err)

	assert.True(t, graph.HasEdge("interface-add-eth0", "interface-add-address-eth0-192.0.2.1/32"), "interface must be created before its address")
	assert.True(t, graph.HasEdge("interface-add-address-eth0-192.0.2.1/32", "peer-add"), "address must exist before peer uses it")
	assert.False(t, graph.HasEdge("interface-add-eth0", "peer-add"), "the derivation must not add a transitive edge")
}

// TestBuildOperationGraphDerivesConsumerBeforeProducerOnDestroy verifies the
// mirror of the create ordering: a destroy that consumes a resource runs before
// the destroy of the operation that produces it.
//
// VALIDATES: remove-peer -> remove-address and remove-address -> remove-interface.
// PREVENTS: an address removed while a peer still binds it, or an interface deleted under its address.
func TestBuildOperationGraphDerivesConsumerBeforeProducerOnDestroy(t *testing.T) {
	t.Parallel()

	ops := []ConfigOperation{
		fixtureRemoveInterface("eth0"),
		fixtureRemoveAddress("eth0", "192.0.2.1/32"),
		fixturePeer("peer-remove", testOpRemovePeer, VerbDestroy, "203.0.113.1", "192.0.2.1"),
	}

	graph, err := BuildOperationGraph(ops, nil)
	require.NoError(t, err)

	assert.True(t, graph.HasEdge("peer-remove", "interface-remove-address-eth0-192.0.2.1/32"), "the peer must go before the address it binds")
	assert.True(t, graph.HasEdge("interface-remove-address-eth0-192.0.2.1/32", "interface-remove-eth0"), "the address must go before the interface holding it")
	assert.False(t, graph.HasEdge("interface-remove-address-eth0-192.0.2.1/32", "peer-remove"), "the destroy ordering runs consumer first, never producer first")
}

// TestBuildOperationGraphNoDerivedEdgeAcrossDifferentResources verifies that a
// produce and a consume of two different resources relate two operations not at
// all, whatever else they share.
//
// VALIDATES: identity, not kind, is what pairs a producer with a consumer.
// PREVENTS: a dense graph in which every address operation orders every peer.
func TestBuildOperationGraphNoDerivedEdgeAcrossDifferentResources(t *testing.T) {
	t.Parallel()

	ops := []ConfigOperation{
		fixtureAddAddress("eth0", "192.0.2.1/32"),
		fixturePeer("peer-add", testOpAddPeer, VerbCreate, "203.0.113.1", "198.51.100.7"),
		fixtureAddInterface("eth1"),
	}

	graph, err := BuildOperationGraph(ops, nil)
	require.NoError(t, err)

	assert.Empty(t, graphEdgePairs(t, graph), "no two operations here name one resource, so no edge is owed")
}

// TestBuildOperationGraphUnknownResourceKindOrders verifies that a resource
// kind no constant in this package names still pairs a producer with a
// consumer (A-4). The kinds are open because the identity of a kind the switch
// does not know comes from its default branch.
//
// VALIDATES: a root can declare a resource the engine never heard of and still be ordered.
// PREVENTS: the vocabulary closing again around the kinds two components happen to use.
func TestBuildOperationGraphUnknownResourceKindOrders(t *testing.T) {
	t.Parallel()

	const wireguardTunnel ResourceKind = "wireguard-tunnel"

	ops := []ConfigOperation{
		{
			ID: "vpn-start", Verb: VerbCreate, Owner: "vpn",
			Target:   ResourceRef{Kind: wireguardTunnel, Name: "wg0"},
			Consumes: []ResourceRef{{Kind: wireguardTunnel, Name: "wg0"}},
		},
		{
			ID: "tunnel-create", Verb: VerbCreate, Owner: "vpn",
			Target:   ResourceRef{Kind: wireguardTunnel, Name: "wg0"},
			Produces: []ResourceRef{{Kind: wireguardTunnel, Name: "wg0"}},
		},
		{
			ID: "tunnel-create-other", Verb: VerbCreate, Owner: "vpn",
			Target:   ResourceRef{Kind: wireguardTunnel, Name: "wg1"},
			Produces: []ResourceRef{{Kind: wireguardTunnel, Name: "wg1"}},
		},
	}

	graph, err := BuildOperationGraph(ops, nil)
	require.NoError(t, err)

	assert.True(t, graph.HasEdge("tunnel-create", "vpn-start"), "an unnamed kind is matched by identity like every other")
	assert.False(t, graph.HasEdge("tunnel-create-other", "vpn-start"), "a second resource of that kind is a different resource")
}

// TestBuildOperationGraphBlankResourceMatchesNothing verifies that a declared
// resource carrying no identifying value orders nothing, rather than matching
// every resource in the transaction.
//
// ValidateOperations refuses such an operation before a transaction reaches the
// graph. This is the second half of that pair, and it is the half that holds
// when a caller builds a graph without going through the planner.
//
// VALIDATES: an empty identity is not a wildcard.
// PREVENTS: a hostile plugin ordering itself against every operation by declaring a blank resource.
func TestBuildOperationGraphBlankResourceMatchesNothing(t *testing.T) {
	t.Parallel()

	ops := []ConfigOperation{
		fixtureAddAddress("eth0", "192.0.2.1/32"),
		fixtureAddInterface("eth0"),
		{
			ID: "hostile-consume-everything", Verb: VerbCreate, Owner: "hostile",
			Target:   ResourceRef{Kind: ResourcePeer, Peer: "hostile"},
			Consumes: []ResourceRef{{Kind: ResourceAddress}, {}},
		},
		{
			ID: "hostile-produce-everything", Verb: VerbCreate, Owner: "hostile",
			Target:   ResourceRef{Kind: ResourcePeer, Peer: "hostile"},
			Produces: []ResourceRef{{Kind: ResourceInterface}},
		},
	}

	graph, err := BuildOperationGraph(ops, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"interface-add-eth0 -> interface-add-address-eth0-192.0.2.1/32"}, graphEdgePairs(t, graph),
		"the blank entries must earn no edge at either end")
}

// TestBuildOperationGraphDerivedEdgesMatchDeletedRules verifies that the
// derivation reproduces, on the operations the iface and bgp decomposers emit,
// the edges the nine deleted constraint rules produced.
//
// The `rules` column of each case is MEASURED, not reasoned: the graph was
// built with the nine rules as they stood before this spec deleted them, over
// these same operations, and the pairs it printed are recorded here. The
// evidence is in the phase's scratch log, `probe-raw.log`.
//
// The `derived` column is the two edges the derivation adds. Neither replaces a
// measured edge, and each one is an edge a deleted rule's own ID promised and
// its body never produced. They are the whole difference between the two edge
// sets:
//
//   - `iface-remove-address-before-interface` produced NOTHING. It related the
//     two operations through the interface an ADDRESS names, and an interface
//     operation carries its name in Target.Name, so the right-hand side of that
//     comparison was empty for every pair the rule ever saw.
//   - `bgp-add-address-before-peer` selected the `add-peer` label, so a
//     modify-peer binding the same address got no edge, though it binds the
//     address exactly as a create does.
//
// VALIDATES: no edge the deleted rules produced is lost, and every edge gained is named.
// PREVENTS: the derivation being taken for a simplification while it silently drops an ordering.
func TestBuildOperationGraphDerivedEdgesMatchDeletedRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		ops     []ConfigOperation
		rules   []string
		derived []string
	}{
		{
			name: "iface-create",
			ops:  []ConfigOperation{fixtureAddInterface("dum0"), fixtureAddAddress("dum0", "10.0.0.1/24")},
			rules: []string{
				"interface-add-dum0 -> interface-add-address-dum0-10.0.0.1/24",
			},
		},
		{
			name:  "iface-delete",
			ops:   []ConfigOperation{fixtureRemoveAddress("dum0", "10.0.0.1/24"), fixtureRemoveInterface("dum0")},
			rules: nil,
			derived: []string{
				"interface-remove-address-dum0-10.0.0.1/24 -> interface-remove-dum0",
			},
		},
		{
			name: "iface-renumber-one-interface",
			ops:  []ConfigOperation{fixtureAddAddress("dum0", "10.0.0.2/24"), fixtureRemoveAddress("dum0", "10.0.0.1/24")},
			rules: []string{
				"interface-remove-address-dum0-10.0.0.1/24 -> interface-add-address-dum0-10.0.0.2/24",
			},
		},
		{
			name: "iface-move-address-between-interfaces",
			ops:  []ConfigOperation{fixtureAddAddress("dum1", "10.0.0.1/24"), fixtureRemoveAddress("dum0", "10.0.0.1/24")},
			rules: []string{
				"interface-remove-address-dum0-10.0.0.1/24 -> interface-add-address-dum1-10.0.0.1/24",
			},
		},
		{
			name: "bgp-peer-local-address-change",
			ops: []ConfigOperation{
				fixtureAddAddress("dum0", "192.0.2.2/32"),
				fixtureRemoveAddress("dum0", "192.0.2.1/32"),
				fixturePeer("bgp-remove-peer-edge", testOpRemovePeer, VerbDestroy, "edge", "192.0.2.1"),
				fixturePeer("bgp-add-peer-edge", testOpAddPeer, VerbCreate, "edge", "192.0.2.2"),
			},
			rules: []string{
				"interface-remove-address-dum0-192.0.2.1/32 -> interface-add-address-dum0-192.0.2.2/32",
				"interface-add-address-dum0-192.0.2.2/32 -> bgp-add-peer-edge",
				"bgp-remove-peer-edge -> interface-remove-address-dum0-192.0.2.1/32",
			},
		},
		{
			name: "bgp-modify-peer-while-its-address-moves",
			ops: []ConfigOperation{
				fixtureAddAddress("dum1", "192.0.2.1/32"),
				fixtureRemoveAddress("dum0", "192.0.2.1/32"),
				fixturePeer("bgp-modify-peer-edge", testOpModifyPeer, VerbModify, "edge", "192.0.2.1"),
			},
			rules: []string{
				"interface-remove-address-dum0-192.0.2.1/32 -> interface-add-address-dum1-192.0.2.1/32",
			},
			derived: []string{
				"interface-add-address-dum1-192.0.2.1/32 -> bgp-modify-peer-edge",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			graph, err := BuildOperationGraph(tc.ops, survivingIfaceRules())
			require.NoError(t, err)
			got := graphEdgePairs(t, graph)

			for _, want := range tc.rules {
				assert.Contains(t, got, want, "an edge the deleted rules produced is missing: that is a regression, not a simplification")
			}
			assert.ElementsMatch(t, append(append([]string{}, tc.rules...), tc.derived...), got,
				"the derivation produced an edge no deleted rule produced and this case does not name")
		})
	}
}

// TestBuildOperationGraphRejectsDuplicateOperationID verifies the graph refuses
// two operations sharing one id, because every edge names its ends by id.
//
// VALIDATES: duplicate ids abort the build.
// PREVENTS: an edge silently attaching to whichever operation the map kept.
func TestBuildOperationGraphRejectsDuplicateOperationID(t *testing.T) {
	t.Parallel()

	ops := []ConfigOperation{fixtureAddInterface("dum0"), fixtureAddInterface("dum0")}

	_, err := BuildOperationGraph(ops, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate operation id")
}
