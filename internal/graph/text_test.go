package graph

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderText_SinglePath verifies rendering of a simple 3-node linear path.
//
// VALIDATES: AC-2 "Graph shows 3 columns (layers), left-to-right flow."
// PREVENTS: Missing nodes or edges in simple case.
func TestRenderText_SinglePath(t *testing.T) {
	g := BuildGraphFromPaths([][]uint32{{100, 200, 300}})
	result := RenderText(g)

	require.NotEmpty(t, result)

	// Should contain all three ASN labels
	assert.Contains(t, result, "AS100")
	assert.Contains(t, result, "AS200")
	assert.Contains(t, result, "AS300")

	// Should contain box-drawing characters
	assert.True(t, strings.ContainsAny(result, "┌┐└┘│─"), "should contain box-drawing characters")

	// Should contain arrow heads
	assert.Contains(t, result, ">", "should contain arrow heads")
}

// TestRenderText_Diamond verifies rendering of a diamond topology.
//
// VALIDATES: AC-3 "Shared AS appears once, edges converge."
// PREVENTS: Duplicate nodes or missing edges in diamond case.
func TestRenderText_Diamond(t *testing.T) {
	// AS100->AS200->AS300 and AS100->AS300
	g := BuildGraphFromPaths([][]uint32{{100, 200, 300}, {100, 300}})
	result := RenderText(g)

	require.NotEmpty(t, result)

	// All three ASNs present
	assert.Contains(t, result, "AS100")
	assert.Contains(t, result, "AS200")
	assert.Contains(t, result, "AS300")

	// AS100 should appear only once in the output (as a label)
	count := strings.Count(result, "AS100")
	assert.Equal(t, 1, count, "AS100 should appear exactly once")
}

// TestRenderText_Empty verifies empty graph produces empty or minimal output.
//
// VALIDATES: AC-7 "No routes match filters" -- empty graph handled gracefully.
// PREVENTS: Crash or garbage output on empty input.
func TestRenderText_Empty(t *testing.T) {
	g := BuildGraphFromPaths([][]uint32{})
	result := RenderText(g)

	// Should be empty
	assert.Empty(t, result, "empty graph should produce empty output")
}

// TestRenderText_MaxNodes verifies graceful handling at node limit.
//
// VALIDATES: AC-8 "Graph with > 100 nodes: graceful limit."
// PREVENTS: Terminal flooding with oversized graph.
func TestRenderText_MaxNodes(t *testing.T) {
	// Build a graph with exactly MaxNodes+1 ASes
	paths := make([][]uint32, MaxNodes+1)
	for i := range paths {
		paths[i] = []uint32{uint32(i + 1), 99999}
	}
	g := BuildGraphFromPaths(paths)
	assert.Greater(t, len(g.Nodes), MaxNodes)

	result := RenderText(g)
	require.NotEmpty(t, result)

	// Should indicate the graph is too large
	assert.Contains(t, result, "too many", "should indicate graph is too large")
}

// TestRenderText_SingleNode verifies rendering of a graph with one ASN.
//
// VALIDATES: Edge case -- single node path produces a single box.
// PREVENTS: Index out of range on single-node graph.
func TestRenderText_SingleNode(t *testing.T) {
	g := BuildGraphFromPaths([][]uint32{{100}})
	result := RenderText(g)

	require.NotEmpty(t, result)
	assert.Contains(t, result, "AS100")
	assert.True(t, strings.ContainsAny(result, "┌┐└┘"), "should contain box corners")
}

// TestRenderText_ASPrepending verifies AS prepending is collapsed.
//
// VALIDATES: AC-4 "Consecutive duplicates collapsed, AS100 appears once."
// PREVENTS: AS prepending creating duplicate nodes.
func TestRenderText_ASPrepending(t *testing.T) {
	g := BuildGraphFromPaths([][]uint32{{100, 100, 200}})
	result := RenderText(g)

	require.NotEmpty(t, result)
	// AS100 should appear exactly once as a label
	count := strings.Count(result, "AS100")
	assert.Equal(t, 1, count, "AS100 should appear exactly once after dedup")
}

// TestRenderText_TwoSourcesOneOrigin verifies two source ASes converging.
//
// VALIDATES: AC-1 "Output contains box-drawing characters and both ASN labels."
// PREVENTS: Missing nodes when multiple source ASes exist.
func TestRenderText_TwoSourcesOneOrigin(t *testing.T) {
	g := BuildGraphFromPaths([][]uint32{{100, 300}, {200, 300}})
	result := RenderText(g)

	require.NotEmpty(t, result)
	assert.Contains(t, result, "AS100")
	assert.Contains(t, result, "AS200")
	assert.Contains(t, result, "AS300")
}

// TestRenderText_ExactMaxNodes verifies rendering at exactly the node limit.
//
// VALIDATES: Boundary test -- exactly MaxNodes nodes should render successfully.
// PREVENTS: Off-by-one error at the limit boundary.
func TestRenderText_ExactMaxNodes(t *testing.T) {
	// Build a graph with exactly MaxNodes ASes
	paths := make([][]uint32, MaxNodes-1) // -1 because origin adds 1
	for i := range paths {
		paths[i] = []uint32{uint32(i + 1), 99999}
	}
	g := BuildGraphFromPaths(paths)
	assert.LessOrEqual(t, len(g.Nodes), MaxNodes)

	result := RenderText(g)
	require.NotEmpty(t, result)
	// Should NOT contain the "too many" message
	assert.NotContains(t, result, "too many")
}
