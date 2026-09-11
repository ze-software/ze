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
// The order is INVERTED here, and this is the deliberate reviewed decision the
// previous version of this comment demanded. The owner stated the apply order on
// 2026-09-11 and it removes before it adds: "remove all IP deconfigured", then
// "add all IP moved or added" (docs/architecture/config/apply-ordering.md,
// quoted verbatim). The binder bound to the address is stopped first, so the
// window with no address on the interface costs nothing that is still running.
//
// What the old order cost is now gone with it: the new address is no longer
// added while the old one is present, so it never becomes a Linux SECONDARY of
// it, and the kernel has no removal to cascade. The guard in
// internal/plugins/iface/netlink/addr_primary.go stays for the RECONCILE path,
// which still adds before it removes; the ordered path no longer reaches it.
//
// VALIDATES: a same-subnet address change yields REMOVE_ADDRESS(old) ordered before ADD_ADDRESS(new).
// PREVENTS: a silent return to make-before-break, which the requirement does not ask for.
func TestIfaceSameSubnetSwapOrdersRemoveBeforeAdd(t *testing.T) {
	sorted := runSwapOperations(t,
		`{"interface":{"backend":"netlink","dummy":{"zdiag0":{"unit":{"0":{"ipv4":{"address":"10.77.0.1/24"}}}}}}}`,
		`{"interface":{"backend":"netlink","dummy":{"zdiag0":{"unit":{"0":{"ipv4":{"address":"10.77.0.2/24"}}}}}}}`,
		`{"interface/dummy/zdiag0/unit/0/ipv4/address/0":{"old":"10.77.0.1/24","new":"10.77.0.2/24"}}`,
	)

	require.Len(t, sorted, 3)
	assert.Equal(t, operationConfigureIfaces, sorted[2].Type,
		"the configure operation runs last, once every address is where the plan leaves it")
	assert.Equal(t, operationRemoveAddress, sorted[0].Type)
	assert.Equal(t, "10.77.0.1/24", sorted[0].Params.CIDR)
	assert.Equal(t, "zdiag0", sorted[0].Target.Interface)
	assert.Equal(t, operationAddAddress, sorted[1].Type)
	assert.Equal(t, "10.77.0.2/24", sorted[1].Params.CIDR)
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
		`{"interface":{"backend":"netlink","dummy":{"zdiag0":{"unit":{"0":{"ipv4":{"address":"10.77.0.1/24"}}}}}}}`,
		`{"interface":{"backend":"netlink","dummy":{"zdiag0":{"unit":{"0":{"ipv4":{"address":"10.77.0.2/24"}}}}}}}`,
		`{"interface/dummy/zdiag0/unit/0/ipv4/address/0":{"old":"10.77.0.1/24","new":"10.77.0.2/24"}}`,
	)

	b := &fakeBackend{}
	b.ensureMaps()
	b.ifaces["zdiag0"] = fakeIface{name: "zdiag0", linkType: "dummy"}
	b.addrs["zdiag0"] = []string{"10.77.0.1/24"}
	for i := range sorted {
		// The configure operation applies the pending config, which the
		// plugin closure holds and this test has no verify phase to fill
		// (register.go, applyPendingConfig). What it would do to the
		// addresses is what the two operations below have already done.
		if sorted[i].Type == operationConfigureIfaces {
			continue
		}
		_, err := applyIfaceOperation(&sorted[i], b)
		require.NoErrorf(t, err, "apply %s", sorted[i].ID)
	}

	assert.Equal(t, []string{"10.77.0.2/24"}, b.addrs["zdiag0"])
}
