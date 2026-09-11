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
	require.Len(t, ops, 3)

	assert.Equal(t, operationAddAddress, ops[0].Type)
	assert.Equal(t, tx.VerbCreate, ops[0].Verb, "the engine orders by the verb, so every emitted operation carries one")
	assert.Equal(t, "interface", ops[0].Owner)
	assert.Equal(t, tx.ResourceAddress, ops[0].Target.Kind)
	assert.Equal(t, "dum0", ops[0].Target.Interface)
	assert.Equal(t, "10.0.0.2/24", ops[0].Params.CIDR)

	assert.Equal(t, operationRemoveAddress, ops[1].Type)
	assert.Equal(t, tx.VerbDestroy, ops[1].Verb)
	assert.Equal(t, "10.0.0.1/24", ops[1].Params.CIDR)

	assert.Equal(t, operationConfigureIfaces, ops[2].Type,
		"the root covers itself whole, so the configure operation closes every decomposition")
}

// TestIfaceOperationDecomposerBackendChangeRidesTheConfigureOperation verifies
// that a change this package has no resource primitive for still reaches the
// component, as the one operation that applies the config whole.
//
// It used to answer nothing at all, which left the participant uncovered and
// the core synthesized a coarse section apply for it. That answer was correct
// for this diff and wrong for a diff that also moved an address, and the two
// could not be told apart from outside this function.
//
// VALIDATES: a backend change decomposes to the configure operation alone.
// PREVENTS: a root that answers nothing, which the core reads as a root that
// disturbs nothing.
func TestIfaceOperationDecomposerBackendChangeRidesTheConfigureOperation(t *testing.T) {
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
	require.Len(t, ops, 1)
	assert.Equal(t, operationConfigureIfaces, ops[0].Type)
	assert.Equal(t, tx.VerbModify, ops[0].Verb)
}

// TestIfaceOperationDecomposerNoDiffDecomposesNothing verifies that a root
// asked with an empty diff answers with no operation.
//
// The planner asks every root that decomposes a second time once an address is
// disturbed, and this root is asked then even when its own config did not
// change (bindingRoots in internal/component/plugin/server/reload_tx.go).
//
// VALIDATES: an empty diff produces no operation, the configure operation included.
// PREVENTS: a reload applying an interface section no commit changed.
func TestIfaceOperationDecomposerNoDiffDecomposesNothing(t *testing.T) {
	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		Root:               configRootInterface,
		ActiveRoot:         `{"interface":{"backend":"test"}}`,
		CandidateRoot:      `{"interface":{"backend":"test"}}`,
		Diff:               tx.DiffSection{Root: configRootInterface},
		DisturbedAddresses: []string{"10.0.0.1"},
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
	require.Len(t, ops, 3)

	assert.Equal(t, operationAddInterface, ops[0].Type)
	assert.Equal(t, "dum0", ops[0].Target.Name)
	assert.Equal(t, "dummy", ops[0].Params.Property)

	assert.Equal(t, operationAddAddress, ops[1].Type)
	assert.Equal(t, "dum0", ops[1].Target.Interface)

	assert.Equal(t, operationConfigureIfaces, ops[2].Type)
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
	require.Len(t, ops, 3)

	assert.Equal(t, operationRemoveAddress, ops[0].Type)
	assert.Equal(t, "dum0", ops[0].Target.Interface)

	assert.Equal(t, operationRemoveInterface, ops[1].Type)
	assert.Equal(t, "dum0", ops[1].Target.Name)

	assert.Equal(t, operationConfigureIfaces, ops[2].Type)
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

// TestIfaceOperationDecomposerUnsupportedTypeRidesTheConfigureOperation
// verifies that an interface type this package has no create primitive for
// (tunnel, wireguard, xfrm) is applied by the configure operation instead of
// an unrollable interface operation.
//
// VALIDATES: a tunnel deletion produces no interface operation and rides the
// configure operation instead.
// PREVENTS: REMOVE_INTERFACE for a tunnel producing an unrollable operation,
// and a tunnel edit taking the whole root's address operations down with it.
func TestIfaceOperationDecomposerUnsupportedTypeRidesTheConfigureOperation(t *testing.T) {
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
	require.Len(t, ops, 1)
	assert.Equal(t, operationConfigureIfaces, ops[0].Type,
		"the configure operation deletes the tunnel, because this package has no primitive for one")
}

// TestIfaceOperationDecomposerMixedDiffStillMovesTheAddress is the defect this
// decomposition was rewritten for. A commit that edits an MTU AND moves an
// address between interfaces used to produce no operation at all, because one
// key the decomposer had no primitive for refused the whole root. The core
// reads the disturbed address set out of the planned operations, so a plan
// with no address destroy in it says nothing is disturbed, and every peer
// bound to that address stayed up while the coarse section apply moved the
// address underneath it.
//
// VALIDATES: a mixed diff emits the address destroy and create, and the
// configure operation that carries the MTU.
// PREVENTS: a binder left bound to an address that is being removed and added
// again, which is the failure phase 2 of the requirement exists to prevent.
func TestIfaceOperationDecomposerMixedDiffStillMovesTheAddress(t *testing.T) {
	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		Root:          configRootInterface,
		ActiveRoot:    `{"interface":{"backend":"test","dummy":{"dum0":{"mtu":"1500","unit":{"default":{"ipv4":{"address":"10.0.0.1/24"}}}},"dum1":{}}}}`,
		CandidateRoot: `{"interface":{"backend":"test","dummy":{"dum0":{"mtu":"9000"},"dum1":{"unit":{"default":{"ipv4":{"address":"10.0.0.1/24"}}}}}}}`,
		Diff: tx.DiffSection{
			Root:    configRootInterface,
			Added:   `{"interface/dummy/dum1/unit/default/ipv4/address/0":"10.0.0.1/24"}`,
			Removed: `{"interface/dummy/dum0/unit/default/ipv4/address/0":"10.0.0.1/24"}`,
			Changed: `{"interface/dummy/dum0/mtu":{"old":"1500","new":"9000"}}`,
		},
	})
	require.NoError(t, err)
	require.Len(t, ops, 3)

	assert.Equal(t, operationAddAddress, ops[0].Type)
	assert.Equal(t, "dum1", ops[0].Target.Interface)
	assert.Equal(t, "10.0.0.1/24", ops[0].Params.CIDR)

	assert.Equal(t, operationRemoveAddress, ops[1].Type)
	assert.Equal(t, tx.VerbDestroy, ops[1].Verb,
		"the destroy is what the core reads the disturbed address out of")
	assert.Equal(t, "dum0", ops[1].Target.Interface)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceAddress, Address: "10.0.0.1/24"}}, ops[1].Produces)

	assert.Equal(t, operationConfigureIfaces, ops[2].Type,
		"the MTU still reaches the component, on the operation that applies the config whole")

	assert.Equal(t, []string{"10.0.0.1"}, tx.DisturbedAddresses(ops),
		"the core establishes the disturbance, so every binder bound to it is stopped")
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
		operationConfigureIfaces,
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
	require.Len(t, ops, 3)

	assert.Equal(t, operationAddInterface, ops[0].Type)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceInterface, Name: "dum0"}}, ops[0].Produces,
		"the interface operation owns the interface")
	assert.Empty(t, ops[0].Consumes, "creating an interface needs nothing another operation makes")

	assert.Equal(t, operationAddAddress, ops[1].Type)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceAddress, Address: "10.0.0.1/24"}}, ops[1].Produces,
		"the address operation owns the address")
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceInterface, Name: "dum0"}}, ops[1].Consumes,
		"an address needs the interface it sits on")

	assert.Equal(t, operationConfigureIfaces, ops[2].Type)
	assert.Empty(t, ops[2].Produces, "the configure operation owns no resource")
	assert.Empty(t, ops[2].Consumes, "so it earns no derived edge, and two placement rules order it")

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
	require.Len(t, removed, 3)

	assert.Equal(t, operationRemoveAddress, removed[0].Type)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceAddress, Address: "10.0.0.1/24"}}, removed[0].Produces)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceInterface, Name: "dum0"}}, removed[0].Consumes,
		"the destroy ordering runs consumer first, so the address removal declares the interface it needs")

	assert.Equal(t, operationRemoveInterface, removed[1].Type)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceInterface, Name: "dum0"}}, removed[1].Produces)

	assert.Equal(t, operationConfigureIfaces, removed[2].Type)
}

// TestIfaceConstraintRulesStateOnlyWhatNoPairCan verifies this package
// registers only rules no produce and consume pair can state, and that each
// still produces its edge.
//
// Two state a fact about two operations over DIFFERENT resources. Two more
// place the configure operation, which owns no resource at all and therefore
// earns no derived edge of its own.
//
// VALIDATES: every address removal is ordered before every address addition,
// which is phases 3 and 4 of the owner's apply order, and the configure
// operation after every operation that moves an address or an interface.
// PREVENTS: a return to make-before-break, and a configure operation that
// applies the end state while an address is still waiting to be removed.
func TestIfaceConstraintRulesStateOnlyWhatNoPairCan(t *testing.T) {
	ops := []tx.ConfigOperation{
		ifaceAddressOperation(operationAddAddress, "dum1", "10.0.0.1/24"),
		ifaceAddressOperation(operationRemoveAddress, "dum1", "10.0.0.9/24"),
		ifaceAddressOperation(operationRemoveAddress, "dum0", "10.0.0.1/24"),
		ifaceInterfaceOperation(operationAddInterface, "dum1", zeTypeDummy),
		ifaceConfigureOperation(),
	}

	graph, err := tx.BuildOperationGraph(ops, tx.ConstraintRules())
	require.NoError(t, err)

	assert.True(t, graph.HasEdge("interface-remove-address-dum0-10.0.0.1_24", "interface-add-address-dum1-10.0.0.1_24"),
		"one address is on one interface: the old one goes before the new one arrives")
	assert.True(t, graph.HasEdge("interface-remove-address-dum1-10.0.0.9_24", "interface-add-address-dum1-10.0.0.1_24"),
		"phases 3 and 4: every address the commit removes goes before any address it adds, on one interface as across two")

	configure := ifaceConfigureOperation().ID
	assert.True(t, graph.HasEdge("interface-remove-address-dum0-10.0.0.1_24", configure),
		"the configure operation applies the end state, so it follows every address destroy")
	assert.True(t, graph.HasEdge("interface-add-address-dum1-10.0.0.1_24", configure),
		"and every address create")
	assert.True(t, graph.HasEdge("interface-add-dum1", configure),
		"and every interface operation")
	assert.False(t, graph.HasEdge(configure, configure),
		"it orders nothing after itself, which is what keeps it out of every cycle")
}

// TestIfaceOperationDecomposerRefusesADiffItCannotParse verifies that a diff
// section which will not unmarshal aborts the decomposition instead of reading
// as a root that changed nothing.
//
// The two answers are the same value and mean opposite things. "Nothing
// changed" drops every address operation this root owns, so the core reads the
// commit as quiet, no binder stops, and the section apply moves the address
// under a running session -- the failure the decomposer was rewritten to
// prevent. No first-party producer can emit one, which is why the shape is
// what this test fences (ai/rules/principles.md).
//
// VALIDATES: a parse failure is answered as a failure.
// PREVENTS: a malformed diff spelled exactly like a quiet commit.
func TestIfaceOperationDecomposerRefusesADiffItCannotParse(t *testing.T) {
	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-iface-bad-diff",
		Root:          configRootInterface,
		ActiveRoot:    `{"interface":{"backend":"test","dummy":{"dum0":{"unit":{"default":{"ipv4":{"address":"10.0.0.1/24"}}}}}}}`,
		CandidateRoot: `{"interface":{"backend":"test"}}`,
		Diff:          tx.DiffSection{Root: configRootInterface, Changed: `{"interface/dummy/dum0"`},
	})
	require.Error(t, err, "a diff section that will not parse must not be answered as no change")
	assert.Contains(t, err.Error(), "decompose diff")
	assert.Nil(t, ops)
}
