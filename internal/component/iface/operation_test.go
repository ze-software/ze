package iface

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tx "github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestIfaceOperationDecomposerAddressAddRemove verifies that iface owns
// decomposition of address-only config changes.
//
// VALIDATES: interface active/candidate roots decompose to add-address and remove-address operations.
// PREVENTS: Generic transaction code parsing interface schema semantics.
func TestIfaceOperationDecomposerAddressAddRemove(t *testing.T) {
	active := `{"interface":{"backend":"test","dummy":{"dum0":{"unit":{"default":{"ipv4":{"address":"10.0.0.1/24"}}}}}}}`
	candidate := `{"interface":{"backend":"test","dummy":{"dum0":{"unit":{"default":{"ipv4":{"address":"10.0.0.2/24"}}}}}}}`

	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-iface-op",
		Root:          configRootInterface,
		ActiveRoot:    active,
		CandidateRoot: candidate,
		Diff: tx.DiffSection{
			Root:    configRootInterface,
			Added:   `{"interface/dummy/dum0/unit/default/ipv4/address/1":"10.0.0.2/24"}`,
			Removed: `{"interface/dummy/dum0/unit/default/ipv4/address/0":"10.0.0.1/24"}`,
		},
	})
	require.NoError(t, err)
	require.Len(t, ops, 2)

	assert.Equal(t, operationAddAddress, ops[0].Type)
	assert.Equal(t, tx.VerbCreate, ops[0].Verb, "the engine orders by the verb, so every emitted operation carries one")
	assert.Equal(t, "interface", ops[0].Owner)
	assert.Equal(t, tx.ResourceAddress, ops[0].Target.Kind)
	assert.Equal(t, "dum0", ops[0].Target.Interface)
	assert.Equal(t, "10.0.0.2/24", ops[0].Params.CIDR)

	assert.Equal(t, operationRemoveAddress, ops[1].Type)
	assert.Equal(t, tx.VerbDestroy, ops[1].Verb)
	assert.Equal(t, "10.0.0.1/24", ops[1].Params.CIDR)
}

// TestIfaceOperationDecomposerUnsupportedDiffFallsBack verifies that iface does
// not force the operation path for changes it cannot express as operations.
//
// VALIDATES: Non-interface, non-address changes produce no operations for legacy fallback.
// PREVENTS: Partial operation decomposition silently dropping unsupported interface changes.
func TestIfaceOperationDecomposerUnsupportedDiffFallsBack(t *testing.T) {
	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		Root:          configRootInterface,
		ActiveRoot:    `{"interface":{"backend":"test"}}`,
		CandidateRoot: `{"interface":{"backend":"linux"}}`,
		Diff: tx.DiffSection{
			Root:    configRootInterface,
			Changed: `{"interface/backend":{"old":"test","new":"linux"}}`,
		},
	})
	require.NoError(t, err)
	assert.Empty(t, ops)
}

// TestIfaceOperationDecomposerNewInterface verifies that creating a managed
// interface produces ADD_INTERFACE followed by ADD_ADDRESS operations.
//
// VALIDATES: new dummy interface decomposes to ADD_INTERFACE + ADD_ADDRESS.
// PREVENTS: constraint rule iface-add-interface-before-address having no operations to match.
func TestIfaceOperationDecomposerNewInterface(t *testing.T) {
	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-iface-new",
		Root:          configRootInterface,
		ActiveRoot:    `{"interface":{"backend":"test"}}`,
		CandidateRoot: `{"interface":{"backend":"test","dummy":{"dum0":{"unit":{"default":{"ipv4":{"address":"10.0.0.1/24"}}}}}}}`,
		Diff: tx.DiffSection{
			Root:  configRootInterface,
			Added: `{"interface/dummy/dum0":{}}`,
		},
	})
	require.NoError(t, err)
	require.Len(t, ops, 2)

	assert.Equal(t, operationAddInterface, ops[0].Type)
	assert.Equal(t, "dum0", ops[0].Target.Name)
	assert.Equal(t, "dummy", ops[0].Params.Property)

	assert.Equal(t, operationAddAddress, ops[1].Type)
	assert.Equal(t, "dum0", ops[1].Target.Interface)
}

// TestIfaceOperationDecomposerDeleteInterface verifies that deleting a managed
// interface produces REMOVE_ADDRESS followed by REMOVE_INTERFACE operations.
//
// VALIDATES: deleted dummy interface decomposes to REMOVE_ADDRESS + REMOVE_INTERFACE.
// PREVENTS: constraint rule iface-remove-address-before-interface having no operations to match.
func TestIfaceOperationDecomposerDeleteInterface(t *testing.T) {
	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-iface-del",
		Root:          configRootInterface,
		ActiveRoot:    `{"interface":{"backend":"test","dummy":{"dum0":{"unit":{"default":{"ipv4":{"address":"10.0.0.1/24"}}}}}}}`,
		CandidateRoot: `{"interface":{"backend":"test"}}`,
		Diff: tx.DiffSection{
			Root:    configRootInterface,
			Removed: `{"interface/dummy/dum0":{}}`,
		},
	})
	require.NoError(t, err)
	require.Len(t, ops, 2)

	assert.Equal(t, operationRemoveAddress, ops[0].Type)
	assert.Equal(t, "dum0", ops[0].Target.Interface)

	assert.Equal(t, operationRemoveInterface, ops[1].Type)
	assert.Equal(t, "dum0", ops[1].Target.Name)
}

// TestApplyIfaceOperationAddInterfaceJournal verifies ADD_INTERFACE creates
// an interface through the backend and rollback deletes it.
//
// VALIDATES: ADD_INTERFACE operation creates a dummy and rollback removes it.
// PREVENTS: interface creation operations that cannot be rolled back.
func TestApplyIfaceOperationAddInterfaceJournal(t *testing.T) {
	b := &fakeBackend{}
	op := tx.ConfigOperation{
		ID:   "iface-add",
		Type: operationAddInterface,
		Target: tx.ResourceRef{
			Kind: tx.ResourceInterface,
			Name: "dum1",
		},
		Params: tx.ConfigOperationParams{Name: "dum1", Property: "dummy"},
	}

	j, err := applyIfaceOperation(&op, b)
	require.NoError(t, err)
	assert.True(t, b.created["dum1"])

	errs := j.Rollback()
	require.Empty(t, errs)
	assert.True(t, b.deleted["dum1"])
}

// TestIfaceOperationDecomposerUnsupportedTypeFallsBack verifies that creating
// or deleting an unsupported interface type (tunnel, wireguard, xfrm) falls
// back to the legacy apply path instead of producing unrollable operations.
//
// VALIDATES: unsupported interface types do not enter the operation path.
// PREVENTS: REMOVE_INTERFACE for a tunnel producing an unrollable operation.
func TestIfaceOperationDecomposerUnsupportedTypeFallsBack(t *testing.T) {
	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-iface-unsupported",
		Root:          configRootInterface,
		ActiveRoot:    `{"interface":{"backend":"test"}}`,
		CandidateRoot: `{"interface":{"backend":"test"}}`,
		Diff: tx.DiffSection{
			Root:    configRootInterface,
			Removed: `{"interface/tunnel/tun0":{}}`,
		},
	})
	require.NoError(t, err)
	assert.Empty(t, ops, "tunnel deletion must fall back to legacy path")
}

// TestIfaceOperationDecomposerMixedDiffFallsBack verifies that a diff with
// both address changes and non-decomposable changes (e.g., MTU) falls back
// to the legacy apply path to avoid losing the non-decomposable changes.
//
// VALIDATES: mixed diffs do not enter the operation path.
// PREVENTS: MTU/MAC/property changes silently dropped when address changes coexist.
func TestIfaceOperationDecomposerMixedDiffFallsBack(t *testing.T) {
	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		Root:          configRootInterface,
		ActiveRoot:    `{"interface":{"backend":"test","dummy":{"dum0":{"unit":{"default":{"ipv4":{"address":"10.0.0.1/24"}}},"mtu":"1500"}}}}`,
		CandidateRoot: `{"interface":{"backend":"test","dummy":{"dum0":{"unit":{"default":{"ipv4":{"address":"10.0.0.2/24"}}},"mtu":"9000"}}}}`,
		Diff: tx.DiffSection{
			Root:    configRootInterface,
			Changed: `{"interface/dummy/dum0/unit/default/ipv4/address/0":{"old":"10.0.0.1/24","new":"10.0.0.2/24"},"interface/dummy/dum0/mtu":{"old":"1500","new":"9000"}}`,
		},
	})
	require.NoError(t, err)
	assert.Empty(t, ops, "mixed address+MTU diff must fall back to legacy path")
}

// TestApplyIfaceOperationAddressJournal verifies address operation handlers
// record exact inverse actions for executor-ordered rollback.
//
// VALIDATES: ADD_ADDRESS operation applies through the backend and rollback removes it.
// PREVENTS: Operation apply succeeding without a usable inverse journal entry.
func TestApplyIfaceOperationAddressJournal(t *testing.T) {
	b := &fakeBackend{}
	b.ensureMaps()
	b.ifaces["dum0"] = fakeIface{name: "dum0", linkType: "dummy"}
	op := tx.ConfigOperation{
		ID:    "addr-add",
		Type:  operationAddAddress,
		Owner: "interface",
		Target: tx.ResourceRef{
			Kind:      tx.ResourceAddress,
			Interface: "dum0",
			Address:   "10.0.0.2/24",
		},
		Params: tx.ConfigOperationParams{Interface: "dum0", CIDR: "10.0.0.2/24"},
	}

	j, err := applyIfaceOperation(&op, b)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.2/24"}, b.addrs["dum0"])

	errs := j.Rollback()
	require.Empty(t, errs)
	assert.Empty(t, b.addrs["dum0"])
}

// TestIfaceConfigOperationDecls verifies the interface plugin declares the
// operation callbacks it wires during Stage 1 registration.
//
// VALIDATES: iface declares interface and address operation support.
// PREVENTS: engine-side exact-or-reject gating from rejecting interface operation commits.
func TestIfaceConfigOperationDecls(t *testing.T) {
	decls := ifaceConfigOperationDecls()
	require.Len(t, decls, 1)
	assert.Equal(t, configRootInterface, decls[0].Root)
	assert.True(t, decls[0].Decompose)
	assert.ElementsMatch(t, []sdk.ConfigOperationType{
		operationAddInterface, operationRemoveInterface,
		operationAddAddress, operationRemoveAddress,
	}, decls[0].Operations)
}

// TestIfaceOperationsDeclareProduceAndConsume verifies that every operation the
// iface decomposer emits declares the resource it owns and the resource it
// needs. Those declarations are the whole of what ordered these operations
// after the five produce/consume constraint rules were deleted: the engine
// derives an edge from a producer and a consumer pair, and this package
// registers no rule naming another root's labels.
//
// VALIDATES: add/remove interface produce the interface; add/remove address produce the address and consume the interface.
// PREVENTS: a decomposer emitting an operation the graph cannot place, which orders as if it depended on nothing.
func TestIfaceOperationsDeclareProduceAndConsume(t *testing.T) {
	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-iface-declares",
		Root:          configRootInterface,
		ActiveRoot:    `{"interface":{"backend":"test"}}`,
		CandidateRoot: `{"interface":{"backend":"test","dummy":{"dum0":{"unit":{"default":{"ipv4":{"address":"10.0.0.1/24"}}}}}}}`,
		Diff: tx.DiffSection{
			Root:  configRootInterface,
			Added: `{"interface/dummy/dum0":{}}`,
		},
	})
	require.NoError(t, err)
	require.Len(t, ops, 2)

	assert.Equal(t, operationAddInterface, ops[0].Type)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceInterface, Name: "dum0"}}, ops[0].Produces,
		"the interface operation owns the interface")
	assert.Empty(t, ops[0].Consumes, "creating an interface needs nothing another operation makes")

	assert.Equal(t, operationAddAddress, ops[1].Type)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceAddress, Address: "10.0.0.1/24"}}, ops[1].Produces,
		"the address operation owns the address")
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceInterface, Name: "dum0"}}, ops[1].Consumes,
		"an address needs the interface it sits on")

	removed, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-iface-declares-remove",
		Root:          configRootInterface,
		ActiveRoot:    `{"interface":{"backend":"test","dummy":{"dum0":{"unit":{"default":{"ipv4":{"address":"10.0.0.1/24"}}}}}}}`,
		CandidateRoot: `{"interface":{"backend":"test"}}`,
		Diff: tx.DiffSection{
			Root:    configRootInterface,
			Removed: `{"interface/dummy/dum0":{}}`,
		},
	})
	require.NoError(t, err)
	require.Len(t, removed, 2)

	assert.Equal(t, operationRemoveAddress, removed[0].Type)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceAddress, Address: "10.0.0.1/24"}}, removed[0].Produces)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceInterface, Name: "dum0"}}, removed[0].Consumes,
		"the destroy ordering runs consumer first, so the address removal declares the interface it needs")

	assert.Equal(t, operationRemoveInterface, removed[1].Type)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceInterface, Name: "dum0"}}, removed[1].Produces)
}

// TestIfaceConstraintRulesStateOnlyWhatNoPairCan verifies this package
// registers exactly the two rules that are not produce/consume facts, and that
// both still produce their edge.
//
// VALIDATES: same-address uniqueness across interfaces, and make-before-break within one interface.
// PREVENTS: the deletion of the five derived rules taking these two with it.
func TestIfaceConstraintRulesStateOnlyWhatNoPairCan(t *testing.T) {
	ops := []tx.ConfigOperation{
		ifaceAddressOperation(operationAddAddress, "dum1", "10.0.0.1/24"),
		ifaceAddressOperation(operationRemoveAddress, "dum1", "10.0.0.9/24"),
		ifaceAddressOperation(operationRemoveAddress, "dum0", "10.0.0.1/24"),
	}

	graph, err := tx.BuildOperationGraph(ops, tx.ConstraintRules())
	require.NoError(t, err)

	assert.True(t, graph.HasEdge("interface-remove-address-dum0-10.0.0.1_24", "interface-add-address-dum1-10.0.0.1_24"),
		"one address is on one interface: the old one goes before the new one arrives")
	assert.True(t, graph.HasEdge("interface-add-address-dum1-10.0.0.1_24", "interface-remove-address-dum1-10.0.0.9_24"),
		"an interface is never left with no address: the new one arrives before the old one goes")
}
