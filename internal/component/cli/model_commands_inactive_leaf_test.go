package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const modelLeafTestConfig = `bgp {
	session {
		asn {
			local 65000;
		}
	}
	router-id 1.2.3.4;
}
`

func newModelWithLeafConfig(t *testing.T) Model {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(modelLeafTestConfig), 0o600))
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })
	model, err := NewModel(ed)
	require.NoError(t, err)
	return model
}

// TestModelDeactivateLeaf verifies the TUI command no longer rejects a
// leaf path; `deactivate bgp router-id` deactivates the leaf and emits
// a status confirming the path.
//
// VALIDATES: AC-11 -- the previously-rejected leaf path now succeeds.
//
// PREVENTS: regression to the old "cannot deactivate a leaf value, use
// delete instead" behavior at model_commands.go:549.
func TestModelDeactivateLeaf(t *testing.T) {
	model := newModelWithLeafConfig(t)

	result, err := model.cmdDeactivate([]string{"bgp", "router-id"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Deactivated bgp router-id")

	content := model.editor.WorkingContent()
	assert.Contains(t, content, "inactive: router-id")
}

// TestModelActivateLeaf verifies the symmetric path: a previously
// deactivated leaf re-activates cleanly.
func TestModelActivateLeaf(t *testing.T) {
	model := newModelWithLeafConfig(t)

	_, err := model.cmdDeactivate([]string{"bgp", "router-id"})
	require.NoError(t, err)

	result, err := model.cmdActivate([]string{"bgp", "router-id"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Activated bgp router-id")

	content := model.editor.WorkingContent()
	assert.NotContains(t, content, "inactive: router-id")
	assert.Contains(t, content, "router-id 1.2.3.4")
}
