package lg

import (
	"net/http"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/graph"
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
	// VALIDATES: AS prepending removal via shared graph package.
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
			got := graph.DeduplicateASPath(tt.in)
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
	// VALIDATES: AC-2 from lg-4 -- the ASN name reaches the drawing, as the
	// node's mouse-over <title> (TestASPathGraphNamesOnMouseOver).
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
	// VALIDATES: the visible label of a node is "AS" and the number, with a
	// name or without one: the name is on mouse-over only (asNodeTooltip).
	if got := formatNodeLabel(GraphNode{ASN: 65001}); got != "AS65001" {
		t.Errorf("no name: got %q, want AS65001", got)
	}
	if got := formatNodeLabel(GraphNode{ASN: 65001, Name: "Acme"}); got != "AS65001" {
		t.Errorf("with name: got %q, want AS65001", got)
	}
}

func TestASNodeTooltip(t *testing.T) {
	// VALIDATES: the mouse-over text of an AS node is "AS<n> - <name>", and
	// "" for an AS with no name, so the writer draws no empty <title>.
	if got := asNodeTooltip(65001, ""); got != "" {
		t.Errorf("no name: got %q, want empty", got)
	}
	if got := asNodeTooltip(65001, "Acme Corp"); got != "AS65001 - Acme Corp" {
		t.Errorf("with name: got %q, want 'AS65001 - Acme Corp'", got)
	}
}

func TestRenderGraphText(t *testing.T) {
	// VALIDATES: deterministic text output for AS-path graph.
	g := &Graph{
		Nodes: []GraphNode{
			{ASN: 65001, Layer: 1},
			{ASN: 65002, Layer: 0, Name: "Example"},
		},
		Edges: []GraphEdge{
			{FromASN: 65001, ToASN: 65002},
		},
	}

	text := renderGraphText(g)

	if !strings.Contains(text, "mode aspath") {
		t.Error("should contain mode aspath")
	}
	if !strings.Contains(text, "node AS65001 layer=1") {
		t.Errorf("should contain AS65001 node, got:\n%s", text)
	}
	if !strings.Contains(text, "node AS65002 Example layer=0") {
		t.Errorf("should contain AS65002 with name, got:\n%s", text)
	}
	if !strings.Contains(text, "edge AS65001 -> AS65002") {
		t.Error("should contain edge")
	}
}

// TestGraphSendsCoveringForBareAddress proves the graph asks bgp-rib what the
// search table asked for the same prefix field, so it draws the routes the
// table lists.
//
// VALIDATES: a bare address is sent as `covering <address>` and draws the AS
// path of the received route containing it (65001 then 65002); an address no
// received route contains draws nothing; a prefix with its length is sent as
// `exact <prefix>` and draws that prefix alone, so 10.0.0.0/2 draws nothing
// while the text filter would have drawn 10.0.0.0/24; an IPv4 length of 33
// answers 400 and sends nothing.
// PREVENTS: an empty graph under a full result table
// (spec-lg-search-matches-what-the-pages-promise AC-4).
func TestGraphSendsCoveringForBareAddress(t *testing.T) {
	cases := []struct {
		name, prefix, command, body string
	}{
		{"bare address", "10.0.0.1", "show bgp rib received covering 10.0.0.1", "edge AS65001 -> AS65002"},
		{"address no received route contains", "192.0.2.1", "show bgp rib received covering 192.0.2.1", "No routes found"},
		{"prefix with its length", "10.0.0.0/24", "show bgp rib received exact 10.0.0.0/24", "edge AS65001 -> AS65002"},
		{"prefix whose text starts the held one", "10.0.0.0/2", "show bgp rib received exact 0.0.0.0/2", "No routes found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, commands := lookupRequest(t, "/lg/graph?format=text&prefix="+tc.prefix, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
			}
			rib := ribCommands(commands)
			if len(rib) != 1 || rib[0] != tc.command {
				t.Fatalf("rib commands = %q, want [%q]", rib, tc.command)
			}
			if !strings.Contains(rec.Body.String(), tc.body) {
				t.Errorf("graph does not hold %q:\n%s", tc.body, rec.Body.String())
			}
		})
	}

	t.Run("ipv4 length past 32", func(t *testing.T) {
		rec, commands := lookupRequest(t, "/lg/graph?format=text&prefix=10.0.0.0/33", "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
		}
		if rib := ribCommands(commands); len(rib) != 0 {
			t.Fatalf("an invalid prefix reached bgp-rib: %q", rib)
		}
		if !strings.Contains(rec.Body.String(), "an IPv4 prefix length is 0 to 32") {
			t.Errorf("refusal does not give the bound: %s", rec.Body.String())
		}
	})
}
