package transaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTopologicalSort verifies that the solver returns operations in an order
// that respects graph edges produced by constraint rules.
//
// VALIDATES: Topological sort respects ADD_INTERFACE -> ADD_ADDRESS -> ADD_PEER.
// PREVENTS: Executor receiving operation order based on input slice order.
func TestTopologicalSort(t *testing.T) {
	t.Parallel()

	graph, err := BuildOperationGraph([]ConfigOperation{
		{ID: "peer-add", Type: OperationAddPeer, Target: ResourceRef{Kind: ResourcePeer, Peer: "203.0.113.1"}, Params: ConfigOperationParams{Address: "192.0.2.1"}},
		{ID: "addr-add", Type: OperationAddAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1"}},
		{ID: "iface-add", Type: OperationAddInterface, Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"}},
	}, []ConstraintRule{
		{ID: "O1", Before: OperationSelector{Type: OperationAddInterface, ResourceKind: ResourceInterface}, After: OperationSelector{Type: OperationAddAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationInterfaceAddress},
		{ID: "O2", Before: OperationSelector{Type: OperationAddAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: OperationAddPeer, ResourceKind: ResourcePeer}, Relation: ResourceRelationAddressUsedBy},
	})
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
		{ID: "addr-add", Type: OperationAddAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1"}},
		{ID: "addr-remove", Type: OperationRemoveAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1"}},
	}, []ConstraintRule{
		{ID: "test-add-before-remove", Before: OperationSelector{Type: OperationAddAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: OperationRemoveAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameResource},
		{ID: "test-remove-before-add", Before: OperationSelector{Type: OperationRemoveAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: OperationAddAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameResource},
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
		{ID: "add-A-2", Type: OperationAddAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.2/32"}},
		{ID: "add-B-1", Type: OperationAddAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.1/32"}},
		{ID: "remove-B-2", Type: OperationRemoveAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.2/32"}},
		{ID: "remove-A-1", Type: OperationRemoveAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.1/32"}},
	}
	rules := []ConstraintRule{
		{ID: "R5-remove-before-add-same", Before: OperationSelector{Type: OperationRemoveAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: OperationAddAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameAddress},
		{ID: "add-before-remove-same-iface", Before: OperationSelector{Type: OperationAddAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: OperationRemoveAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationInterfaceAddress},
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
		{ID: "add-A-2", Type: OperationAddAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.2/32"}},
		{ID: "add-B-3", Type: OperationAddAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.3/32"}},
		{ID: "add-C-1", Type: OperationAddAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethC", Address: "10.0.0.1/32"}},
		{ID: "remove-A-1", Type: OperationRemoveAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.1/32"}},
		{ID: "remove-B-2", Type: OperationRemoveAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.2/32"}},
		{ID: "remove-C-3", Type: OperationRemoveAddress, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethC", Address: "10.0.0.3/32"}},
	}
	rules := []ConstraintRule{
		{ID: "R5-remove-before-add-same", Before: OperationSelector{Type: OperationRemoveAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: OperationAddAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationSameAddress},
		{ID: "add-before-remove-same-iface", Before: OperationSelector{Type: OperationAddAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: OperationRemoveAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationInterfaceAddress},
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
		{ID: "add-peer", Type: OperationAddPeer, Target: ResourceRef{Kind: ResourcePeer, Peer: "edge"}},
		{ID: "remove-peer", Type: OperationRemovePeer, Target: ResourceRef{Kind: ResourcePeer, Peer: "edge"}},
	}
	rules := []ConstraintRule{
		{ID: "add-before-remove", Before: OperationSelector{Type: OperationAddPeer, ResourceKind: ResourcePeer}, After: OperationSelector{Type: OperationRemovePeer, ResourceKind: ResourcePeer}, Relation: ResourceRelationSameResource},
		{ID: "remove-before-add", Before: OperationSelector{Type: OperationRemovePeer, ResourceKind: ResourcePeer}, After: OperationSelector{Type: OperationAddPeer, ResourceKind: ResourcePeer}, Relation: ResourceRelationSameResource},
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
