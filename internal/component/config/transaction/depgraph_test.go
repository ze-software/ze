package transaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDependencyGraphSimple verifies that data constraint rules produce only
// the resource-related edges needed by the operation solver.
//
// VALIDATES: O1 and O2 style rules produce ADD_INTERFACE -> ADD_ADDRESS -> ADD_PEER.
// PREVENTS: The transaction package hardcoding iface or BGP ordering logic.
func TestDependencyGraphSimple(t *testing.T) {
	t.Parallel()

	ops := []ConfigOperation{
		{
			ID:   "peer-add",
			Type: OperationAddPeer,
			Target: ResourceRef{
				Kind: ResourcePeer,
				Peer: "203.0.113.1",
			},
			Params: ConfigOperationParams{Address: "192.0.2.1"},
		},
		{
			ID:   "addr-add",
			Type: OperationAddAddress,
			Target: ResourceRef{
				Kind:      ResourceAddress,
				Interface: "eth0",
				Address:   "192.0.2.1/32",
			},
		},
		{
			ID:   "iface-add",
			Type: OperationAddInterface,
			Target: ResourceRef{
				Kind: ResourceInterface,
				Name: "eth0",
			},
		},
	}
	rules := []ConstraintRule{
		{
			ID:       "O1",
			Before:   OperationSelector{Type: OperationAddInterface, ResourceKind: ResourceInterface},
			After:    OperationSelector{Type: OperationAddAddress, ResourceKind: ResourceAddress},
			Relation: ResourceRelationInterfaceAddress,
		},
		{
			ID:       "O2",
			Before:   OperationSelector{Type: OperationAddAddress, ResourceKind: ResourceAddress},
			After:    OperationSelector{Type: OperationAddPeer, ResourceKind: ResourcePeer},
			Relation: ResourceRelationAddressUsedBy,
		},
	}

	graph, err := BuildOperationGraph(ops, rules)
	require.NoError(t, err)

	assert.True(t, graph.HasEdge("iface-add", "addr-add"), "interface must be created before its address")
	assert.True(t, graph.HasEdge("addr-add", "peer-add"), "address must exist before peer uses it")
	assert.False(t, graph.HasEdge("iface-add", "peer-add"), "rules should not add transitive edges directly")
}
