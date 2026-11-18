// Design: docs/architecture/config/change-file-structural-ops.md -- raw review records refuse invalid data.
package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestRenderPendingChangesRefusesInvalidRecords sends unvalidated adapter data
// to the public renderer. Refusal must return no partial review, panic or value
// in its diagnostic, including when a useful record precedes the invalid one.
// MUTATION: panic or return success for an unknown kind; the unknown-kind row
// must fail. Removing address validation must fail the corresponding rows.
func TestRenderPendingChangesRefusesInvalidRecords(t *testing.T) {
	schema, _ := secretCLISchemaAndTree(true)
	valid := contract.PendingChange{
		Kind: contract.PendingChangeSet, Path: "environment api-server token",
		Previous: storedCLISecret, Value: rotatedCLISecret,
	}
	for _, tc := range []struct {
		name   string
		schema *config.Schema
		change contract.PendingChange
	}{
		{name: "zero record", schema: schema},
		{name: "unknown kind", schema: schema, change: contract.PendingChange{
			Kind: contract.PendingChangeKind(rotatedCLISecret), Path: valid.Path,
		}},
		{name: "malformed address", schema: schema, change: contract.PendingChange{
			Kind: contract.PendingChangeSet, Path: `environment "` + rotatedCLISecret,
		}},
		{name: "unknown address", schema: schema, change: contract.PendingChange{
			Kind: contract.PendingChangeSet, Path: "environment " + rotatedCLISecret,
		}},
		{name: "missing schema", change: valid},
		{name: "missing rename source", schema: schema, change: contract.PendingChange{
			Kind: contract.PendingChangeRename, Path: valid.Path, NewPath: valid.Path,
		}},
		{name: "missing copy target", schema: schema, change: contract.PendingChange{
			Kind: contract.PendingChangeCopy, Path: valid.Path, OldPath: valid.Path,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			review, err := RenderPendingChanges(tc.schema, []contract.PendingChange{valid, tc.change})
			require.Error(t, err)
			var refusal *opererr.Error
			require.ErrorAs(t, err, &refusal)
			assert.Empty(t, review, "a partial review cannot imply complete intent")
			assert.NotContains(t, err.Error(), storedCLISecret)
			assert.NotContains(t, err.Error(), rotatedCLISecret)
		})
	}
}

// TestRenderPendingChangesMasksAddressedValues retains both sides of a real
// rotation and proves raw records are unchanged by schema-based masking.
func TestRenderPendingChangesMasksAddressedValues(t *testing.T) {
	schema, _ := secretCLISchemaAndTree(true)
	change := contract.PendingChange{
		Kind: contract.PendingChangeSet, Path: "environment api-server token",
		Previous: storedCLISecret, Value: rotatedCLISecret,
	}
	changes := []contract.PendingChange{change}
	review, err := RenderPendingChanges(schema, changes)
	require.NoError(t, err)
	assert.Equal(t, "- environment api-server token "+config.SecretDataPlaceholder+
		"\n+ environment api-server token "+config.SecretDataPlaceholder+"\n", review)
	assert.Equal(t, change, changes[0])
	assert.NotContains(t, review, storedCLISecret)
	assert.NotContains(t, review, rotatedCLISecret)
}
