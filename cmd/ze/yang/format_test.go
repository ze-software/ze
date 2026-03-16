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

// PREVENTS: Config nodes leaking into config-only view or vice versa.
func TestTreeFormatTextFilterConfig(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	var buf bytes.Buffer
	err = FormatTreeText(&buf, root, SourceConfig)
	require.NoError(t, err)

	out := buf.String()
	// Should contain config nodes like bgp.
	assert.Contains(t, out, "bgp", "config filter should show bgp")
	// Should not contain command-only nodes like "summary" (the BGP summary RPC).
	// Note: "cache" exists as both command and config (environment > cache), so we
	// check "summary" which is command-only.
	assert.NotContains(t, out, "summary", "config filter should hide command-only nodes")
}

// PREVENTS: JSON collision output broken when no collisions found.
func TestCollisionsFormatJSONEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := FormatCollisionsJSON(&buf, nil)
	require.NoError(t, err)

	var result struct {
		Collisions []json.RawMessage `json:"collisions"`
		Summary    struct {
			TotalGroups   int `json:"total-groups"`
			TotalAffected int `json:"total-affected"`
		} `json:"summary"`
	}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err, "empty collision list should produce valid JSON")
	assert.Empty(t, result.Collisions)
	assert.Equal(t, 0, result.Summary.TotalGroups)
}

// PREVENTS: Text collision output broken when no collisions found.
func TestCollisionsFormatTextEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := FormatCollisionsText(&buf, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No prefix collisions found")
}

// PREVENTS: Constraint annotations missing for mandatory fields.
func TestFormatConstraintsMandatory(t *testing.T) {
	node := &AnalysisNode{Mandatory: true}
	assert.Equal(t, "[mandatory]", formatConstraints(node))
}

// PREVENTS: Constraint annotations missing for default values.
func TestFormatConstraintsDefault(t *testing.T) {
	node := &AnalysisNode{Default: "90"}
	assert.Equal(t, "[default: 90]", formatConstraints(node))
}

// PREVENTS: Constraint annotations missing for range values.
func TestFormatConstraintsRange(t *testing.T) {
	node := &AnalysisNode{Range: "0..65535"}
	assert.Equal(t, "[0..65535]", formatConstraints(node))
}

// PREVENTS: Multiple constraints not combined correctly.
func TestFormatConstraintsCombined(t *testing.T) {
	node := &AnalysisNode{Mandatory: true, Default: "90", Range: "0..65535"}
	result := formatConstraints(node)
	assert.Contains(t, result, "[mandatory]")
	assert.Contains(t, result, "[default: 90]")
	assert.Contains(t, result, "[0..65535]")
}

// PREVENTS: Empty constraints producing non-empty string.
func TestFormatConstraintsEmpty(t *testing.T) {
	node := &AnalysisNode{}
	assert.Equal(t, "", formatConstraints(node))
}
