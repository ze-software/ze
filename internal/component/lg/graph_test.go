package lg

import (
	"strings"
	"testing"
)

func TestBuildGraphSinglePath(t *testing.T) {
	// VALIDATES: AC-5 from lg-4 -- linear graph from single AS path.
	// PREVENTS: graph construction failing on simple input.
	routes := []any{
		map[string]any{
			"as-path": []any{float64(65001), float64(65002), float64(65003)},
		},
	}

	g := buildGraph(routes)

	if len(g.Nodes) != 3 {
		t.Errorf("nodes = %d, want 3", len(g.Nodes))
	}
	if len(g.Edges) != 2 {
		t.Errorf("edges = %d, want 2", len(g.Edges))
	}
}

func TestBuildGraphMultiplePaths(t *testing.T) {
	// VALIDATES: AC-6 from lg-4 -- branching graph from multiple paths.
	routes := []any{
		map[string]any{
			"as-path": []any{float64(65001), float64(65002), float64(65003)},
		},
		map[string]any{
			"as-path": []any{float64(65004), float64(65002), float64(65003)},
		},
	}

	g := buildGraph(routes)

	// 4 unique ASes: 65001, 65002, 65003, 65004.
	if len(g.Nodes) != 4 {
		t.Errorf("nodes = %d, want 4", len(g.Nodes))
	}
	// 3 edges: 65001->65002, 65002->65003, 65004->65002.
	if len(g.Edges) != 3 {
		t.Errorf("edges = %d, want 3", len(g.Edges))
	}
}

func TestBuildGraphPrepending(t *testing.T) {
	// VALIDATES: AC-7 from lg-4 -- AS prepending collapsed to single node.
	routes := []any{
		map[string]any{
			"as-path": []any{float64(65001), float64(65002), float64(65002), float64(65003)},
		},
	}

	g := buildGraph(routes)

	// 3 unique ASes (65002 appears once despite prepending).
	if len(g.Nodes) != 3 {
		t.Errorf("nodes = %d, want 3", len(g.Nodes))
	}
	if len(g.Edges) != 2 {
		t.Errorf("edges = %d, want 2", len(g.Edges))
	}
}

func TestBuildGraphEmpty(t *testing.T) {
	// VALIDATES: AC-8 from lg-4 -- no routes produces empty graph.
	g := buildGraph(nil)

	if len(g.Nodes) != 0 {
		t.Errorf("nodes = %d, want 0", len(g.Nodes))
	}
	if len(g.Edges) != 0 {
		t.Errorf("edges = %d, want 0", len(g.Edges))
	}
}

func TestDeduplicateASPath(t *testing.T) {
	// VALIDATES: AS prepending removal.
	tests := []struct {
		name string
		in   []uint32
		want []uint32
	}{
		{"no dups", []uint32{1, 2, 3}, []uint32{1, 2, 3}},
		{"consecutive dups", []uint32{1, 2, 2, 3}, []uint32{1, 2, 3}},
		{"all same", []uint32{1, 1, 1}, []uint32{1}},
		{"empty", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateASPath(tt.in)
			if len(got) != len(tt.want) {
				t.Errorf("len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLayoutLayers(t *testing.T) {
	// VALIDATES: AC-3 from lg-4 -- nodes arranged by hop depth.
	g := &Graph{
		Nodes: []GraphNode{
			{ASN: 65001, Layer: 2},
			{ASN: 65002, Layer: 1},
			{ASN: 65003, Layer: 0},
		},
		Edges: []GraphEdge{
			{FromASN: 65001, ToASN: 65002},
			{FromASN: 65002, ToASN: 65003},
		},
	}

	layout := computeLayout(g)

	// Source (layer 2) should be leftmost, origin (layer 0) rightmost.
	pos1 := layout.Positions[65001]
	pos3 := layout.Positions[65003]
	if pos1.X >= pos3.X {
		t.Errorf("source AS (layer 2) X=%d should be left of origin AS (layer 0) X=%d", pos1.X, pos3.X)
	}
}

func TestLayoutPositions(t *testing.T) {
	// VALIDATES: AC-3 from lg-4 -- nodes within same layer spaced vertically.
	g := &Graph{
		Nodes: []GraphNode{
			{ASN: 65001, Layer: 1},
			{ASN: 65002, Layer: 1},
			{ASN: 65003, Layer: 0},
		},
		Edges: []GraphEdge{
			{FromASN: 65001, ToASN: 65003},
			{FromASN: 65002, ToASN: 65003},
		},
	}

	layout := computeLayout(g)

	pos1 := layout.Positions[65001]
	pos2 := layout.Positions[65002]
	if pos1.Y == pos2.Y {
		t.Error("two nodes in same layer should have different Y positions")
	}
	if pos1.X != pos2.X {
		t.Error("two nodes in same layer should have same X position")
	}
}

func TestRenderSVG(t *testing.T) {
	// VALIDATES: AC-9 from lg-4 -- valid SVG output.
	g := &Graph{
		Nodes: []GraphNode{
			{ASN: 65001, Layer: 1},
			{ASN: 65002, Layer: 0},
		},
		Edges: []GraphEdge{
			{FromASN: 65001, ToASN: 65002},
		},
	}

	layout := computeLayout(g)
	svg := renderGraphSVG(g, layout)

	if !strings.HasPrefix(svg, "<svg") {
		t.Error("SVG should start with <svg")
	}
	if !strings.HasSuffix(svg, "</svg>") {
		t.Error("SVG should end with </svg>")
	}
	if !strings.Contains(svg, "AS65001") {
		t.Error("SVG should contain AS65001")
	}
	if !strings.Contains(svg, "AS65002") {
		t.Error("SVG should contain AS65002")
	}
}

func TestRenderSVGWithNames(t *testing.T) {
	// VALIDATES: AC-2 from lg-4 -- ASN names in node labels.
	g := &Graph{
		Nodes: []GraphNode{
			{ASN: 65001, Name: "Example Corp", Layer: 0},
		},
	}

	layout := computeLayout(g)
	svg := renderGraphSVG(g, layout)

	if !strings.Contains(svg, "Example Corp") {
		t.Error("SVG should contain organization name")
	}
}

func TestExtractASPath(t *testing.T) {
	// VALIDATES: AS path extraction from route map with type handling.
	// PREVENTS: panic or wrong ASN values for various input types.
	tests := []struct {
		name string
		in   map[string]any
		want []uint32
	}{
		{"valid float64", map[string]any{"as-path": []any{float64(65001), float64(65002)}}, []uint32{65001, 65002}},
		{"missing key", map[string]any{}, nil},
		{"non-array value", map[string]any{"as-path": "not-an-array"}, nil},
		{"empty array", map[string]any{"as-path": []any{}}, nil},
		{"string element skipped", map[string]any{"as-path": []any{float64(65001), "bad", float64(65003)}}, []uint32{65001, 65003}},
		{"negative float64 skipped", map[string]any{"as-path": []any{float64(-1), float64(65001)}}, []uint32{65001}},
		{"overflow float64 skipped", map[string]any{"as-path": []any{float64(5000000000), float64(65001)}}, []uint32{65001}},
		{"int type", map[string]any{"as-path": []any{int(65001)}}, []uint32{65001}},
		{"int64 type", map[string]any{"as-path": []any{int64(65001)}}, []uint32{65001}},
		{"negative int skipped", map[string]any{"as-path": []any{int(-1)}}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractASPath(tt.in)
			if len(got) != len(tt.want) {
				t.Errorf("len = %d, want %d (got %v)", len(got), len(tt.want), got)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFormatNodeLabel(t *testing.T) {
	// VALIDATES: node label formatting with and without name.
	if got := formatNodeLabel(GraphNode{ASN: 65001}); got != "AS65001" {
		t.Errorf("no name: got %q, want AS65001", got)
	}
	if got := formatNodeLabel(GraphNode{ASN: 65001, Name: "Acme"}); got != "AS65001 Acme" {
		t.Errorf("with name: got %q, want 'AS65001 Acme'", got)
	}
}

func TestTooltipName(t *testing.T) {
	// VALIDATES: tooltip name formatting.
	if got := tooltipName(""); got != "" {
		t.Errorf("empty: got %q, want empty", got)
	}
	if got := tooltipName("Acme Corp"); got != " - Acme Corp" {
		t.Errorf("non-empty: got %q, want ' - Acme Corp'", got)
	}
}
