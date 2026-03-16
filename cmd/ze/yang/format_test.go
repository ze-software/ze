package yang

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: AC-1 -- text output includes collision info.
// PREVENTS: Collision report missing key information.
func TestTreeFormatText(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	var buf bytes.Buffer
	err = FormatTreeText(&buf, root, "")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "bgp", "should contain bgp config node")
	assert.Contains(t, out, SourceConfig, "should show config source tag")
	assert.Contains(t, out, SourceCommand, "should show command source tag")
	assert.Contains(t, out, "mandatory", "should show mandatory constraint for required fields like router-id")
}

// VALIDATES: AC-6 -- JSON tree output has correct structure.
// PREVENTS: JSON output failing to parse.
func TestTreeFormatJSON(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	var buf bytes.Buffer
	err = FormatTreeJSON(&buf, root, "")
	require.NoError(t, err)

	// Verify valid JSON.
	var nodes []json.RawMessage
	err = json.Unmarshal(buf.Bytes(), &nodes)
	require.NoError(t, err, "output should be valid JSON array")
	assert.NotEmpty(t, nodes, "should have tree nodes")
}

// VALIDATES: AC-5 -- --commands filter shows only command nodes.
// PREVENTS: Config nodes leaking into command-only view.
func TestTreeFormatTextFilterCommands(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	var buf bytes.Buffer
	err = FormatTreeText(&buf, root, FilterCommands)
	require.NoError(t, err)

	out := buf.String()
	// Should not contain config-only nodes like router-id.
	assert.NotContains(t, out, "router-id", "config-only nodes should be filtered out")
}

// VALIDATES: AC-2 -- JSON collision output has correct structure.
// PREVENTS: JSON collision output failing to parse.
func TestCollisionsFormatJSON(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	groups := CollectCollisions(root, 1)

	var buf bytes.Buffer
	err = FormatCollisionsJSON(&buf, groups)
	require.NoError(t, err)

	var result struct {
		Collisions []json.RawMessage `json:"collisions"`
		Summary    struct {
			TotalGroups   int `json:"total-groups"`
			TotalAffected int `json:"total-affected"`
		} `json:"summary"`
	}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err, "output should be valid JSON")
	assert.NotEmpty(t, result.Collisions, "should have collision groups")
	assert.Greater(t, result.Summary.TotalGroups, 0)
	assert.Greater(t, result.Summary.TotalAffected, 0)
}
