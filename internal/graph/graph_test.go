package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeduplicateASPath verifies consecutive ASN deduplication (AS prepending collapse).
//
// VALIDATES: AC-4 "Consecutive duplicates collapsed, AS100 appears once."
// PREVENTS: AS prepending inflating the graph with duplicate nodes.
func TestDeduplicateASPath(t *testing.T) {
	tests := []struct {
		name string
		path []uint32
		want []uint32
	}{
		{"no duplicates", []uint32{100, 200, 300}, []uint32{100, 200, 300}},
		{"leading prepend", []uint32{100, 100, 200}, []uint32{100, 200}},
		{"middle prepend", []uint32{100, 200, 200, 300}, []uint32{100, 200, 300}},
		{"all same", []uint32{100, 100, 100}, []uint32{100}},
		{"single ASN", []uint32{100}, []uint32{100}},
		{"empty path", []uint32{}, nil},
		{"nil path", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeduplicateASPath(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestBuildGraphFromPaths verifies graph construction from AS path slices.
//
// VALIDATES: AC-2 "Graph shows 3 columns (layers), left-to-right flow."
// PREVENTS: Missing nodes or edges when building graph from multiple AS paths.
func TestBuildGraphFromPaths(t *testing.T) {
	tests := []struct {
		name      string
		paths     [][]uint32
		wantNodes int
		wantEdges int
	}{
		{
			name:      "single path 3 hops",
			paths:     [][]uint32{{100, 200, 300}},
			wantNodes: 3,
			wantEdges: 2,
		},
		{
			name:      "two paths sharing origin",
			paths:     [][]uint32{{100, 200, 300}, {150, 300}},
			wantNodes: 4,
			wantEdges: 3,
		},
		{
			name: "diamond topology",
			// AS100 -> AS200 -> AS300, AS100 -> AS300
			paths:     [][]uint32{{100, 200, 300}, {100, 300}},
			wantNodes: 3,
			wantEdges: 3,
		},
		{
			name:      "empty paths",
			paths:     [][]uint32{},
			wantNodes: 0,
			wantEdges: 0,
		},
		{
			name:      "single ASN path",
			paths:     [][]uint32{{100}},
			wantNodes: 1,
			wantEdges: 0,
		},
		{
			name:      "with AS prepending",
			paths:     [][]uint32{{100, 100, 200}},
			wantNodes: 2,
			wantEdges: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := BuildGraphFromPaths(tt.paths)
			require.NotNil(t, g)
			assert.Len(t, g.Nodes, tt.wantNodes)
			assert.Len(t, g.Edges, tt.wantEdges)
		})
	}
}

// TestBuildGraphFromPaths_Layers verifies layer assignment.
//
// VALIDATES: AC-2 "Graph shows 3 columns (layers)."
// PREVENTS: Origin AS getting wrong layer depth.
func TestBuildGraphFromPaths_Layers(t *testing.T) {
	// Path: AS100 -> AS200 -> AS300 (origin)
	g := BuildGraphFromPaths([][]uint32{{100, 200, 300}})
	require.Len(t, g.Nodes, 3)

	layerMap := make(map[uint32]int)
	for _, n := range g.Nodes {
		layerMap[n.ASN] = n.Layer
	}

	// Layer 0 = origin (last AS in path), higher = toward source
	assert.Equal(t, 0, layerMap[300], "origin should be layer 0")
	assert.Equal(t, 1, layerMap[200], "intermediate should be layer 1")
	assert.Equal(t, 2, layerMap[100], "source should be layer 2")
}

// TestBuildGraphFromPaths_Diamond verifies shared intermediate node appears once.
//
// VALIDATES: AC-3 "Shared AS appears once, edges converge."
// PREVENTS: Duplicate nodes for the same ASN.
func TestBuildGraphFromPaths_Diamond(t *testing.T) {
	// Two paths converge: AS100->AS200->AS300, AS100->AS300
	g := BuildGraphFromPaths([][]uint32{{100, 200, 300}, {100, 300}})

	// Count unique ASNs
	asnSet := make(map[uint32]bool)
	for _, n := range g.Nodes {
		asnSet[n.ASN] = true
	}
	assert.Len(t, asnSet, 3, "diamond should have 3 unique ASNs")

	// Verify edges: 100->200, 200->300, 100->300
	edgeSet := make(map[[2]uint32]bool)
	for _, e := range g.Edges {
		edgeSet[[2]uint32{e.FromASN, e.ToASN}] = true
	}
	assert.True(t, edgeSet[[2]uint32{100, 200}], "edge 100->200")
	assert.True(t, edgeSet[[2]uint32{200, 300}], "edge 200->300")
	assert.True(t, edgeSet[[2]uint32{100, 300}], "edge 100->300")
}

// TestBuildGraphFromPaths_MaxASN verifies max uint32 ASN is handled.
//
// VALIDATES: boundary test for ASN 4294967295.
// PREVENTS: uint32 overflow in ASN handling.
func TestBuildGraphFromPaths_MaxASN(t *testing.T) {
	g := BuildGraphFromPaths([][]uint32{{4294967295, 1}})
	require.Len(t, g.Nodes, 2)
	asnSet := make(map[uint32]bool)
	for _, n := range g.Nodes {
		asnSet[n.ASN] = true
	}
	assert.True(t, asnSet[4294967295])
	assert.True(t, asnSet[1])
}

// TestBuildGraphFromPaths_Deterministic verifies output is sorted deterministically.
//
// VALIDATES: Deterministic output for test stability.
// PREVENTS: Map iteration order causing flaky tests.
func TestBuildGraphFromPaths_Deterministic(t *testing.T) {
	paths := [][]uint32{{100, 200, 300}, {150, 300}}

	// Run twice and compare
	g1 := BuildGraphFromPaths(paths)
	g2 := BuildGraphFromPaths(paths)

	require.Len(t, g1.Nodes, len(g2.Nodes))
	for i := range g1.Nodes {
		assert.Equal(t, g1.Nodes[i].ASN, g2.Nodes[i].ASN)
		assert.Equal(t, g1.Nodes[i].Layer, g2.Nodes[i].Layer)
	}
	require.Len(t, g1.Edges, len(g2.Edges))
	for i := range g1.Edges {
		assert.Equal(t, g1.Edges[i].FromASN, g2.Edges[i].FromASN)
		assert.Equal(t, g1.Edges[i].ToASN, g2.Edges[i].ToASN)
	}
}
