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

// TestValidateOperationsRefusesBlankResource verifies that an operation
// declaring a resource with no identity is refused before it reaches the graph,
// and that the refusal names the plugin, the root, the operation and the
// declaration without quoting a single parameter.
//
// The entry is attacker-influenced: an operation crosses a JSON boundary from a
// plugin process. An entry naming nothing cannot be matched against any other
// operation's declaration, so accepting it would leave the operation ordered
// against nothing while looking ordered.
//
// VALIDATES: a blank Produces or Consumes entry aborts the transaction.
// PREVENTS: a blank entry read as "any resource", and config values in an error line.
func TestValidateOperationsRefusesBlankResource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		op   ConfigOperation
	}{
		{
			name: "produces nothing identifiable",
			op: ConfigOperation{
				ID: "op-1", Root: "vpn", Owner: "vpn-plugin", Type: testOpAddInterface, Verb: VerbCreate,
				Produces: []ResourceRef{{Kind: ResourceInterface}},
				Params:   ConfigOperationParams{Value: "the-pre-shared-key"},
			},
		},
		{
			name: "consumes an entry with no kind at all",
			op: ConfigOperation{
				ID: "op-1", Root: "vpn", Owner: "vpn-plugin", Type: testOpAddInterface, Verb: VerbCreate,
				Consumes: []ResourceRef{{Name: "eth0"}},
				Params:   ConfigOperationParams{Value: "the-pre-shared-key"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateOperations([]ConfigOperation{tc.op})
			require.ErrorIs(t, err, ErrOperationBlankResource)
			assert.Contains(t, err.Error(), "vpn-plugin")
			assert.Contains(t, err.Error(), "op-1")
			assert.NotContains(t, err.Error(), "the-pre-shared-key", "the message names the operation, never its params")
		})
	}
}

// TestValidateOperationsAcceptsDeclaredResources verifies the validator passes
// an operation whose declarations name real resources, so the refusal above is
// about the blank entry and not about declaring one at all.
//
// VALIDATES: a declared producer and consumer pair is accepted.
// PREVENTS: a fail-closed check that closes on everything.
func TestValidateOperationsAcceptsDeclaredResources(t *testing.T) {
	t.Parallel()

	err := ValidateOperations([]ConfigOperation{{
		ID: "op-1", Root: "interface", Owner: "interface", Type: testOpAddAddress, Verb: VerbCreate,
		Produces: []ResourceRef{{Kind: ResourceAddress, Address: "10.0.0.1/24"}},
		Consumes: []ResourceRef{{Kind: ResourceInterface, Name: "dum0"}},
	}})
	require.NoError(t, err)
}
