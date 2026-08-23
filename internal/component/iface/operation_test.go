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

	assert.Equal(t, tx.OperationAddAddress, ops[0].Type)
	assert.Equal(t, "interface", ops[0].Owner)
	assert.Equal(t, tx.ResourceAddress, ops[0].Target.Kind)
	assert.Equal(t, "dum0", ops[0].Target.Interface)
	assert.Equal(t, "10.0.0.2/24", ops[0].Params.CIDR)

	assert.Equal(t, tx.OperationRemoveAddress, ops[1].Type)
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

	assert.Equal(t, tx.OperationAddInterface, ops[0].Type)
	assert.Equal(t, "dum0", ops[0].Target.Name)
	assert.Equal(t, "dummy", ops[0].Params.Property)

	assert.Equal(t, tx.OperationAddAddress, ops[1].Type)
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

	assert.Equal(t, tx.OperationRemoveAddress, ops[0].Type)
	assert.Equal(t, "dum0", ops[0].Target.Interface)

	assert.Equal(t, tx.OperationRemoveInterface, ops[1].Type)
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
		Type: tx.OperationAddInterface,
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
		Type:  tx.OperationAddAddress,
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
		sdk.OperationAddInterface, sdk.OperationRemoveInterface,
		sdk.OperationAddAddress, sdk.OperationRemoveAddress,
	}, decls[0].Operations)
}
