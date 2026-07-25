package iface

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tx "github.com/ze-software/ze/internal/component/config/transaction"
)

// runSwapOperations decomposes an interface address change the way a SIGHUP
// reload does, orders it through the real constraint rules and solver, and
// returns the operations in the order the executor would apply them.
func runSwapOperations(t *testing.T, active, candidate, changed string) []tx.ConfigOperation {
	t.Helper()
	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-iface-swap",
		Root:          configRootInterface,
		ActiveRoot:    active,
		CandidateRoot: candidate,
		Diff:          tx.DiffSection{Root: configRootInterface, Changed: changed},
	})
	require.NoError(t, err)

	graph, err := tx.BuildOperationGraph(ops, tx.ConstraintRules())
	require.NoError(t, err)
	sorted, err := tx.TopologicalSort(graph)
	require.NoError(t, err)
	return sorted
}

// TestIfaceSameSubnetSwapOrdersAddBeforeRemove pins the make-before-break order
// for a same-subnet address change (10.77.0.1/24 -> 10.77.0.2/24), the shape of
// the reload that left the interface with no address at all.
//
// The ordering itself is correct and deliberate: the new address must be up
// before the old one goes away. What it costs is that the new address becomes a
// Linux SECONDARY of the old one, so the netlink backend has to stop the kernel
// cascading the removal -- see internal/plugins/iface/netlink/addr_primary.go.
// If this order is ever inverted, that guard stops being load-bearing, and the
// inversion must be a deliberate reviewed decision rather than silent drift.
//
// VALIDATES: a same-subnet address change yields ADD_ADDRESS(new) ordered before REMOVE_ADDRESS(old).
// PREVENTS: a silent reordering that changes which address the kernel treats as primary.
func TestIfaceSameSubnetSwapOrdersAddBeforeRemove(t *testing.T) {
	sorted := runSwapOperations(t,
		`{"interface":{"backend":"netlink","dummy":{"zdiag0":{"unit":{"0":{"ipv4":{"address":["10.77.0.1/24"]}}}}}}}`,
		`{"interface":{"backend":"netlink","dummy":{"zdiag0":{"unit":{"0":{"ipv4":{"address":["10.77.0.2/24"]}}}}}}}`,
		`{"interface/dummy/zdiag0/unit/0/ipv4/address/0":{"old":"10.77.0.1/24","new":"10.77.0.2/24"}}`,
	)

	require.Len(t, sorted, 2)
	assert.Equal(t, tx.OperationAddAddress, sorted[0].Type)
	assert.Equal(t, "10.77.0.2/24", sorted[0].Params.CIDR)
	assert.Equal(t, "zdiag0", sorted[0].Target.Interface)
	assert.Equal(t, tx.OperationRemoveAddress, sorted[1].Type)
	assert.Equal(t, "10.77.0.1/24", sorted[1].Params.CIDR)
	assert.Equal(t, "zdiag0", sorted[1].Target.Interface)
}

// TestIfaceSameSubnetSwapAppliesNewAddress drives the ordered operations
// through the operation apply handler and asserts the interface ends up with
// exactly the candidate address set.
//
// VALIDATES: applying the ordered operations for a same-subnet swap leaves only the new address.
// PREVENTS: a decomposition or ordering change dropping the new address on reload.
func TestIfaceSameSubnetSwapAppliesNewAddress(t *testing.T) {
	sorted := runSwapOperations(t,
		`{"interface":{"backend":"netlink","dummy":{"zdiag0":{"unit":{"0":{"ipv4":{"address":["10.77.0.1/24"]}}}}}}}`,
		`{"interface":{"backend":"netlink","dummy":{"zdiag0":{"unit":{"0":{"ipv4":{"address":["10.77.0.2/24"]}}}}}}}`,
		`{"interface/dummy/zdiag0/unit/0/ipv4/address/0":{"old":"10.77.0.1/24","new":"10.77.0.2/24"}}`,
	)

	b := &fakeBackend{}
	b.ensureMaps()
	b.addrs["zdiag0"] = []string{"10.77.0.1/24"}
	for i := range sorted {
		_, err := applyIfaceOperation(&sorted[i], b)
		require.NoErrorf(t, err, "apply %s", sorted[i].ID)
	}

	assert.Equal(t, []string{"10.77.0.2/24"}, b.addrs["zdiag0"])
}
