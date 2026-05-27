package rpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigOperationMarshal verifies operation callback payloads use stable
// kebab-case JSON keys and self-contained value fields.
//
// VALIDATES: External operation callbacks have a concrete wire contract.
// PREVENTS: In-process-only operation values leaking across the plugin boundary.
func TestConfigOperationMarshal(t *testing.T) {
	t.Parallel()

	input := ConfigOperationApplyInput{
		TransactionID: "tx-1",
		Operation: ConfigOperation{
			ID:     "op-1",
			Root:   "interface",
			Owner:  "interface",
			Type:   OperationAddAddress,
			Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "10.0.0.1/32"},
			Params: ConfigOperationParams{Interface: "eth0", CIDR: "10.0.0.1/32", AllowDual: true},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "transaction-id")
	assert.Contains(t, raw, "operation")

	operation, ok := raw["operation"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "add-address", operation["type"])
	assert.Contains(t, operation, "target")
	assert.Contains(t, operation, "params")

	var decoded ConfigOperationApplyInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, OperationAddAddress, decoded.Operation.Type)
	assert.Equal(t, "eth0", decoded.Operation.Target.Interface)
	assert.True(t, decoded.Operation.Params.AllowDual)
}

// TestDeclareRegistrationConfigOperationsMarshal verifies Stage 1 can declare
// operation callback support for external plugins.
//
// VALIDATES: External operation callbacks are mandatory v1 capability data.
// PREVENTS: Operation support being invisible to the engine at startup.
func TestDeclareRegistrationConfigOperationsMarshal(t *testing.T) {
	t.Parallel()

	input := DeclareRegistrationInput{
		WantsConfig: []string{"interface"},
		ConfigOperations: []ConfigOperationDecl{
			{Root: "interface", Decompose: true, Operations: []ConfigOperationType{OperationAddAddress, OperationRemoveAddress}},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)
	assert.Contains(t, string(data), "config-operations")

	var decoded DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Len(t, decoded.ConfigOperations, 1)
	assert.Equal(t, "interface", decoded.ConfigOperations[0].Root)
	assert.True(t, decoded.ConfigOperations[0].Decompose)
	assert.Equal(t, []ConfigOperationType{OperationAddAddress, OperationRemoveAddress}, decoded.ConfigOperations[0].Operations)
}
