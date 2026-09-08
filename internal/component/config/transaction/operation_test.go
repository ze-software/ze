package transaction

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Operation labels for the tests in this package. This package names no label
// of its own: a label belongs to the component that emits it, so a test that
// needs one spells its own. These are the spellings iface and bgp use, which
// keeps the fixtures readable beside the real decomposers.
const (
	testOpAddInterface    ConfigOperationType = "add-interface"
	testOpRemoveInterface ConfigOperationType = "remove-interface"
	testOpAddAddress      ConfigOperationType = "add-address"
	testOpRemoveAddress   ConfigOperationType = "remove-address"
	testOpAddPeer         ConfigOperationType = "add-peer"
	testOpRemovePeer      ConfigOperationType = "remove-peer"
	testOpModifyPeer      ConfigOperationType = "modify-peer"
	testOpAddTunnel       ConfigOperationType = "add-tunnel"
	testOpSetProperty     ConfigOperationType = "set-property"
)

// TestRegisterOperationDecomposer verifies component-owned config operation
// decomposers are registered by root and returned through the transaction
// registry.
//
// VALIDATES: Component-owned decomposition via registry.
// PREVENTS: Centralized transaction code owning iface/BGP/static semantics.
func TestRegisterOperationDecomposer(t *testing.T) {
	t.Parallel()

	root := "test-root-decomposer"
	called := false
	fn := OperationDecomposer(func(_ context.Context, req DecomposeRequest) ([]ConfigOperation, error) {
		called = true
		assert.Equal(t, root, req.Root)
		return []ConfigOperation{{ID: "op-1", Root: root, Type: testOpAddInterface, Verb: VerbCreate}}, nil
	})

	require.NoError(t, RegisterOperationDecomposer(root, fn))
	got, ok := OperationDecomposerFor(root)
	require.True(t, ok)
	ops, err := got(context.Background(), DecomposeRequest{Root: root})
	require.NoError(t, err)
	require.True(t, called)
	require.Len(t, ops, 1)
	assert.Equal(t, testOpAddInterface, ops[0].Type)
}

// TestRegisterOperationDecomposerDuplicate verifies duplicate root ownership is
// rejected so two components cannot both claim semantic decomposition for the
// same config root.
//
// VALIDATES: One owner per config root for operation decomposition.
// PREVENTS: Ambiguous operation decomposition ownership.
func TestRegisterOperationDecomposerDuplicate(t *testing.T) {
	t.Parallel()

	root := "test-root-duplicate"
	fn := OperationDecomposer(func(context.Context, DecomposeRequest) ([]ConfigOperation, error) {
		return nil, nil
	})

	require.NoError(t, RegisterOperationDecomposer(root, fn))
	err := RegisterOperationDecomposer(root, fn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

// TestRegisterConstraintRule verifies constraint rules are registered as data
// and returned deterministically.
//
// VALIDATES: Constraint rules are data, not hardcoded in the orchestrator.
// PREVENTS: Hidden cross-component ordering embedded in transaction code.
func TestRegisterConstraintRule(t *testing.T) {
	t.Parallel()

	rule := ConstraintRule{
		ID:          "test-rule-add-interface-before-address",
		Description: "interface before address",
		Before:      OperationSelector{Type: testOpAddInterface, ResourceKind: ResourceInterface},
		After:       OperationSelector{Type: testOpAddAddress, ResourceKind: ResourceAddress},
	}

	require.NoError(t, RegisterConstraintRule(rule))
	rules := ConstraintRules()
	assert.Contains(t, rules, rule)
}
