package transaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTopologicalSort verifies that the solver returns operations in an order
// that respects the graph edges, here the ones derived from what each
// operation produces and consumes.
//
// VALIDATES: Topological sort respects add-interface -> add-address -> add-peer.
// PREVENTS: Executor receiving operation order based on input slice order.
func TestTopologicalSort(t *testing.T) {
	t.Parallel()

	graph, err := BuildOperationGraph([]ConfigOperation{
		{ID: "peer-add", Type: testOpAddPeer, Verb: VerbCreate, Target: ResourceRef{Kind: ResourcePeer, Peer: "203.0.113.1"},
			Consumes: []ResourceRef{{Kind: ResourceAddress, Address: "192.0.2.1"}},
			Params:   ConfigOperationParams{Address: "192.0.2.1"}},
		{ID: "addr-add", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1"},
			Produces: []ResourceRef{{Kind: ResourceAddress, Address: "192.0.2.1"}},
			Consumes: []ResourceRef{{Kind: ResourceInterface, Name: "eth0"}}},
		{ID: "iface-add", Type: testOpAddInterface, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"},
			Produces: []ResourceRef{{Kind: ResourceInterface, Name: "eth0"}}},
	}, nil)
	require.NoError(t, err)

	sorted, err := TopologicalSort(graph)
	require.NoError(t, err)
	assert.Equal(t, []string{"iface-add", "addr-add", "peer-add"}, operationIDs(sorted))
}

// TestTopologicalSortCycle verifies that irreconcilable cycles are rejected by
// the solver instead of producing a partial or arbitrary order.
//
// VALIDATES: Cycles in the operation graph return ErrOperationCycle.
// PREVENTS: Executor applying operations when dependencies are impossible.
func TestTopologicalSortCycle(t *testing.T) {
	t.Parallel()

	graph, err := BuildOperationGraph([]ConfigOperation{
		{ID: "addr-add", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1"}},
		{ID: "addr-remove", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1"}},
	}, []ConstraintRule{
		{ID: "test-add-before-remove", Before: OperationSelector{Type: testOpAddAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: testOpRemoveAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameResource},
		{ID: "test-remove-before-add", Before: OperationSelector{Type: testOpRemoveAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: testOpAddAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameResource},
	})
	require.NoError(t, err)

	_, err = TopologicalSort(graph)
	require.ErrorIs(t, err, ErrOperationCycle)
}

// TestTopologicalSortCycleResolution verifies that a two-way IP swap cycle
// is resolved via dual-presence fallback instead of rejecting the commit.
//
// VALIDATES: AC-2: IP swap between two interfaces uses dual-presence fallback.
// PREVENTS: Commit rejection for valid IP swap scenarios.
func TestTopologicalSortCycleResolution(t *testing.T) {
	t.Parallel()

	// IP swap: A currently has 10.0.0.1, B currently has 10.0.0.2.
	// Candidate: A gets 10.0.0.2, B gets 10.0.0.1.
	//
	// R5 (same-address): remove-B-2 before add-A-2, remove-A-1 before add-B-1
	// Make-before-break (same-iface): add-A-2 before remove-A-1, add-B-1 before remove-B-2
	// Cycle: remove-B-2 -> add-A-2 -> remove-A-1 -> add-B-1 -> remove-B-2
	ops := []ConfigOperation{
		{ID: "add-A-2", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.2/32"}},
		{ID: "add-B-1", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.1/32"}},
		{ID: "remove-B-2", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.2/32"}},
		{ID: "remove-A-1", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.1/32"}},
	}
	rules := []ConstraintRule{
		{ID: "R5-remove-before-add-same", Before: OperationSelector{Type: testOpRemoveAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: testOpAddAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameAddress},
		{ID: "add-before-remove-same-iface", Before: OperationSelector{Type: testOpAddAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: testOpRemoveAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameInterface},
	}

	graph, err := BuildOperationGraph(ops, rules)
	require.NoError(t, err)

	sorted, err := TopologicalSort(graph)
	require.NoError(t, err, "IP swap cycle should be resolved via dual-presence fallback")
	require.Len(t, sorted, 4)

	hasDual := false
	for i := range sorted {
		if sorted[i].Params.AllowDual {
			hasDual = true
		}
	}
	assert.True(t, hasDual, "at least one ADD_ADDRESS should have AllowDual set")
}

// TestTopologicalSortThreeWayRotation verifies that a three-way IP rotation
// cycle is resolved via dual-presence fallback.
//
// VALIDATES: AC-9: Three-way IP rotation uses dual-presence fallback.
// PREVENTS: Commit rejection for three-way IP rotation scenarios.
func TestTopologicalSortThreeWayRotation(t *testing.T) {
	t.Parallel()

	// A:1->2, B:2->3, C:3->1
	ops := []ConfigOperation{
		{ID: "add-A-2", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.2/32"}},
		{ID: "add-B-3", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.3/32"}},
		{ID: "add-C-1", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethC", Address: "10.0.0.1/32"}},
		{ID: "remove-A-1", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.1/32"}},
		{ID: "remove-B-2", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.2/32"}},
		{ID: "remove-C-3", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethC", Address: "10.0.0.3/32"}},
	}
	rules := []ConstraintRule{
		{ID: "R5-remove-before-add-same", Before: OperationSelector{Type: testOpRemoveAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: testOpAddAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameAddress},
		{ID: "add-before-remove-same-iface", Before: OperationSelector{Type: testOpAddAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: testOpRemoveAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameInterface},
	}

	graph, err := BuildOperationGraph(ops, rules)
	require.NoError(t, err)

	sorted, err := TopologicalSort(graph)
	require.NoError(t, err, "three-way rotation cycle should be resolved via dual-presence fallback")
	require.Len(t, sorted, 6)

	dualCount := 0
	for i := range sorted {
		if sorted[i].Params.AllowDual {
			dualCount++
		}
	}
	assert.Equal(t, 3, dualCount, "all three ADD_ADDRESS operations should have AllowDual set")
}

// TestTopologicalSortNonAddressCycleFails verifies that cycles involving
// non-address operations are not relaxable and still return ErrOperationCycle.
//
// VALIDATES: Non-address cycles are rejected, not silently relaxed.
// PREVENTS: Dual-presence fallback applied to non-address resource types.
func TestTopologicalSortNonAddressCycleFails(t *testing.T) {
	t.Parallel()

	ops := []ConfigOperation{
		{ID: "add-peer", Type: testOpAddPeer, Verb: VerbCreate, Target: ResourceRef{Kind: ResourcePeer, Peer: "edge"}},
		{ID: "remove-peer", Type: testOpRemovePeer, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourcePeer, Peer: "edge"}},
	}
	rules := []ConstraintRule{
		{ID: "add-before-remove", Before: OperationSelector{Type: testOpAddPeer, ResourceKind: ResourcePeer}, After: OperationSelector{Type: testOpRemovePeer, ResourceKind: ResourcePeer}, Relation: ResourceRelationSameResource},
		{ID: "remove-before-add", Before: OperationSelector{Type: testOpRemovePeer, ResourceKind: ResourcePeer}, After: OperationSelector{Type: testOpAddPeer, ResourceKind: ResourcePeer}, Relation: ResourceRelationSameResource},
	}

	graph, err := BuildOperationGraph(ops, rules)
	require.NoError(t, err)

	_, err = TopologicalSort(graph)
	require.ErrorIs(t, err, ErrOperationCycle)
}

func operationIDs(ops []ConfigOperation) []string {
	ids := make([]string, 0, len(ops))
	for i := range ops {
		ids = append(ids, ops[i].ID)
	}
	return ids
}

// TestTopologicalSortRelaxesCycleByVerbAndKind verifies the address-swap
// relaxation decides on the verb plus the target resource kind, and never on
// the operation label. The operations below carry labels no package in this
// repository names, which is what a root that owns its own vocabulary emits.
//
// VALIDATES: AC-5. The solver orders an operation whose label it does not know.
// PREVENTS: The relaxation working for the two roots whose labels the solver
// was written against, and silently rejecting every other root's swap.
func TestTopologicalSortRelaxesCycleByVerbAndKind(t *testing.T) {
	t.Parallel()

	const (
		bindVIP    ConfigOperationType = "provision-bind-vip"
		releaseVIP ConfigOperationType = "provision-release-vip"
	)

	// The same swap as TestTopologicalSortCycleResolution: ethA and ethB
	// exchange 10.0.0.1 and 10.0.0.2, which is a cycle by construction.
	ops := []ConfigOperation{
		{ID: "bind-A-2", Type: bindVIP, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.2/32"}},
		{ID: "bind-B-1", Type: bindVIP, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.1/32"}},
		{ID: "release-B-2", Type: releaseVIP, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.2/32"}},
		{ID: "release-A-1", Type: releaseVIP, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.1/32"}},
	}
	rules := []ConstraintRule{
		{ID: "release-before-bind-same-address", Before: OperationSelector{Type: releaseVIP, ResourceKind: ResourceAddress}, After: OperationSelector{Type: bindVIP, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameAddress},
		{ID: "bind-before-release-same-iface", Before: OperationSelector{Type: bindVIP, ResourceKind: ResourceAddress}, After: OperationSelector{Type: releaseVIP, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameInterface},
	}

	graph, err := BuildOperationGraph(ops, rules)
	require.NoError(t, err)

	sorted, err := TopologicalSort(graph)
	require.NoError(t, err, "a swap of addresses relaxes whatever the operations are labelled")
	require.Len(t, sorted, 4)

	dual := make([]string, 0, 2)
	for i := range sorted {
		if sorted[i].Params.AllowDual {
			dual = append(dual, sorted[i].ID)
		}
	}
	assert.ElementsMatch(t, []string{"bind-A-2", "bind-B-1"}, dual,
		"dual presence is marked on the address creations, which the verb and the kind identify")
}

// TestTopologicalSortRejectsNonAddressCycle restates the preserved rejection
// against the verb vocabulary: a cycle relaxes only when every member creates
// or destroys a resource of kind address. A cycle over another kind, and a
// cycle over an address that no member creates or destroys, are both rejected.
//
// VALIDATES: "Address-only cross-interface cycles relax. Everything else is
// rejected", now decided by verb and kind.
// PREVENTS: A modification cycle being relaxed because its target happens to
// be an address, which would apply two conflicting modifications at once.
func TestTopologicalSortRejectsNonAddressCycle(t *testing.T) {
	t.Parallel()

	const (
		claim   ConfigOperationType = "provision-claim"
		release ConfigOperationType = "provision-release"
		retune  ConfigOperationType = "provision-retune"
	)

	cases := []struct {
		name string
		ops  []ConfigOperation
	}{
		{
			name: "address kind but neither member creates or destroys",
			ops: []ConfigOperation{
				{ID: "retune-1", Type: retune, Verb: VerbModify, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.1/32"}},
				{ID: "retune-2", Type: retune, Verb: VerbModify, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.1/32"}},
			},
		},
		{
			name: "create and destroy but not an address",
			ops: []ConfigOperation{
				{ID: "claim-1", Type: claim, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceKind("vlan"), Interface: "ethA", Name: "v10"}},
				{ID: "release-1", Type: release, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceKind("vlan"), Interface: "ethB", Name: "v10"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			first := tc.ops[0].Type
			second := tc.ops[1].Type
			rules := []ConstraintRule{
				{ID: "first-before-second", Before: OperationSelector{Type: first, ResourceKind: tc.ops[0].Target.Kind}, After: OperationSelector{Type: second, ResourceKind: tc.ops[1].Target.Kind}, Relation: ResourceRelationAny},
				{ID: "second-before-first", Before: OperationSelector{Type: second, ResourceKind: tc.ops[1].Target.Kind}, After: OperationSelector{Type: first, ResourceKind: tc.ops[0].Target.Kind}, Relation: ResourceRelationAny},
			}

			graph, err := BuildOperationGraph(tc.ops, rules)
			require.NoError(t, err)

			_, err = TopologicalSort(graph)
			require.ErrorIs(t, err, ErrOperationCycle)
		})
	}
}
