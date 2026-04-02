package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeLayoutSVG verifies SVG layout produces correct pixel positions.
//
// VALIDATES: SVG layout matches current lg behavior with pixel-based constants.
// PREVENTS: Layout extraction breaking existing SVG rendering.
func TestComputeLayoutSVG(t *testing.T) {
	g := BuildGraphFromPaths([][]uint32{{100, 200, 300}})
	layout := ComputeLayout(g, SVGConfig())
	require.NotNil(t, layout)

	// Should have positions for all 3 nodes
	assert.Len(t, layout.Positions, 3)

	// Verify all nodes have positions
	_, has100 := layout.Positions[100]
	_, has200 := layout.Positions[200]
	_, has300 := layout.Positions[300]
	assert.True(t, has100, "AS100 should have position")
	assert.True(t, has200, "AS200 should have position")
	assert.True(t, has300, "AS300 should have position")

	// Source (layer 2) should be left of intermediate (layer 1), which is left of origin (layer 0)
	assert.Less(t, layout.Positions[100].X, layout.Positions[200].X, "source should be left of intermediate")
	assert.Less(t, layout.Positions[200].X, layout.Positions[300].X, "intermediate should be left of origin")

	// All Y should be the same (same row for linear path)
	assert.Equal(t, layout.Positions[100].Y, layout.Positions[200].Y, "same row")
	assert.Equal(t, layout.Positions[200].Y, layout.Positions[300].Y, "same row")

	// Width and Height should be positive
	assert.Greater(t, layout.Width, 0)
	assert.Greater(t, layout.Height, 0)
}

// TestComputeLayoutText verifies text layout produces correct character-unit positions.
//
// VALIDATES: Text layout uses character-unit constants and produces valid positions.
// PREVENTS: Text layout using pixel values.
func TestComputeLayoutText(t *testing.T) {
	g := BuildGraphFromPaths([][]uint32{{100, 200, 300}})
	layout := ComputeLayout(g, TextConfig())
	require.NotNil(t, layout)

	assert.Len(t, layout.Positions, 3)

	// Source (layer 2) should be left of origin (layer 0)
	assert.Less(t, layout.Positions[100].X, layout.Positions[300].X)

	// Text dimensions should be much smaller than SVG (character units vs pixels)
	svgLayout := ComputeLayout(g, SVGConfig())
	assert.Less(t, layout.Width, svgLayout.Width, "text layout should be narrower than SVG")
	assert.Less(t, layout.Height, svgLayout.Height, "text layout should be shorter than SVG")

	// Height should be at least nodeHeight (character units)
	textCfg := TextConfig()
	assert.GreaterOrEqual(t, layout.Height, textCfg.NodeHeight+2*textCfg.Padding)
}

// TestComputeLayoutEmpty verifies empty graph produces empty layout.
//
// VALIDATES: AC-7 "No routes match filters" -- empty graph handled gracefully.
// PREVENTS: Nil pointer dereference on empty graph.
func TestComputeLayoutEmpty(t *testing.T) {
	g := BuildGraphFromPaths([][]uint32{})
	layout := ComputeLayout(g, TextConfig())
	require.NotNil(t, layout)
	assert.Empty(t, layout.Positions)
}

// TestComputeLayoutMultiRow verifies nodes in the same layer get different Y positions.
//
// VALIDATES: Multiple source ASes are stacked vertically in the same column.
// PREVENTS: Overlapping node positions.
func TestComputeLayoutMultiRow(t *testing.T) {
	// Two source ASes, one origin
	g := BuildGraphFromPaths([][]uint32{{100, 300}, {200, 300}})
	layout := ComputeLayout(g, TextConfig())

	// AS100 and AS200 are both at layer 1, should have different Y
	pos100 := layout.Positions[100]
	pos200 := layout.Positions[200]
	assert.NotEqual(t, pos100.Y, pos200.Y, "same-layer nodes should be at different Y")
}
