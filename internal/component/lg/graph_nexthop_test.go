package lg

import (
	"strings"
	"testing"
)

func TestBuildNextHopGraphSimple(t *testing.T) {
	// VALIDATES: linear forwarding -- peer forwards to egress.
	// PREVENTS: missing egress detection or wrong layer assignment.
	routes := []any{
		map[string]any{"peer-address": "10.0.1.1", "next-hop": "10.0.5.1"},
		map[string]any{"peer-address": "10.0.5.1", "next-hop": "10.0.5.1"},
	}

	g := buildNextHopGraph(routes)

	if len(g.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(g.Nodes))
	}
	if len(g.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(g.Edges))
	}

	for _, n := range g.Nodes {
		switch n.Address {
		case "10.0.5.1":
			if !n.Egress {
				t.Error("10.0.5.1 should be egress")
			}
			if n.Layer != 0 {
				t.Errorf("egress layer = %d, want 0", n.Layer)
			}
		case "10.0.1.1":
			if n.Egress {
				t.Error("10.0.1.1 should not be egress")
			}
			if n.Layer != 1 {
				t.Errorf("non-egress layer = %d, want 1", n.Layer)
			}
		}
	}

	e := g.Edges[0]
	if e.From != "10.0.1.1" || e.To != "10.0.5.1" {
		t.Errorf("edge = %s -> %s, want 10.0.1.1 -> 10.0.5.1", e.From, e.To)
	}
}

func TestBuildNextHopGraphMultipleEgress(t *testing.T) {
	// VALIDATES: traffic split -- multiple egress points with different forwarding sets.
	// PREVENTS: wrong layer assignment when multiple egress exist.
	routes := []any{
		map[string]any{"peer-address": "10.0.5.1", "next-hop": "10.0.5.1"},
		map[string]any{"peer-address": "10.0.6.1", "next-hop": "10.0.6.1"},
		map[string]any{"peer-address": "10.0.1.1", "next-hop": "10.0.5.1"},
		map[string]any{"peer-address": "10.0.1.2", "next-hop": "10.0.5.1"},
		map[string]any{"peer-address": "10.0.2.1", "next-hop": "10.0.6.1"},
	}

	g := buildNextHopGraph(routes)

	if len(g.Nodes) != 5 {
		t.Fatalf("nodes = %d, want 5", len(g.Nodes))
	}
	if len(g.Edges) != 3 {
		t.Fatalf("edges = %d, want 3", len(g.Edges))
	}

	egressCount := 0
	for _, n := range g.Nodes {
		if n.Egress {
			egressCount++
			if n.Layer != 0 {
				t.Errorf("egress %s layer = %d, want 0", n.Address, n.Layer)
			}
		} else if n.Layer != 1 {
			t.Errorf("non-egress %s layer = %d, want 1", n.Address, n.Layer)
		}
	}
	if egressCount != 2 {
		t.Errorf("egress count = %d, want 2", egressCount)
	}
}

func TestBuildNextHopGraphEmpty(t *testing.T) {
	// VALIDATES: nil routes produce empty graph.
	g := buildNextHopGraph(nil)

	if len(g.Nodes) != 0 {
		t.Errorf("nodes = %d, want 0", len(g.Nodes))
	}
	if len(g.Edges) != 0 {
		t.Errorf("edges = %d, want 0", len(g.Edges))
	}
}

func TestBuildNextHopGraphMissingFields(t *testing.T) {
	// VALIDATES: routes missing peer-address or next-hop are skipped.
	// PREVENTS: panic on incomplete route data.
	routes := []any{
		map[string]any{"next-hop": "10.0.5.1"},
		map[string]any{"peer-address": "10.0.1.1"},
		map[string]any{"peer-address": "10.0.1.1", "next-hop": "10.0.5.1"},
		map[string]any{"peer-address": "10.0.5.1", "next-hop": "10.0.5.1"},
	}

	g := buildNextHopGraph(routes)

	if len(g.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2 (invalid routes skipped)", len(g.Nodes))
	}
	if len(g.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(g.Edges))
	}
}

func TestBuildNextHopGraphDeterministic(t *testing.T) {
	// VALIDATES: output is sorted and deterministic.
	// PREVENTS: flaky text assertions in .ci tests.
	routes := []any{
		map[string]any{"peer-address": "10.0.3.1", "next-hop": "10.0.5.1"},
		map[string]any{"peer-address": "10.0.1.1", "next-hop": "10.0.5.1"},
		map[string]any{"peer-address": "10.0.2.1", "next-hop": "10.0.5.1"},
		map[string]any{"peer-address": "10.0.5.1", "next-hop": "10.0.5.1"},
	}

	g1 := buildNextHopGraph(routes)
	g2 := buildNextHopGraph(routes)

	text1 := renderNextHopGraphText(g1)
	text2 := renderNextHopGraphText(g2)
	if text1 != text2 {
		t.Errorf("non-deterministic output:\n%s\nvs\n%s", text1, text2)
	}

	// Nodes should be sorted: layer desc, then address asc.
	if g1.Nodes[0].Address != "10.0.1.1" {
		t.Errorf("first node = %s, want 10.0.1.1 (highest layer, lowest addr)", g1.Nodes[0].Address)
	}
	if g1.Nodes[len(g1.Nodes)-1].Address != "10.0.5.1" {
		t.Errorf("last node = %s, want 10.0.5.1 (egress, layer 0)", g1.Nodes[len(g1.Nodes)-1].Address)
	}
}

func TestBuildNextHopGraphEgressOnly(t *testing.T) {
	// VALIDATES: single egress node with no forwarding produces zero edges.
	routes := []any{
		map[string]any{"peer-address": "10.0.5.1", "next-hop": "10.0.5.1"},
	}

	g := buildNextHopGraph(routes)

	if len(g.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(g.Nodes))
	}
	if len(g.Edges) != 0 {
		t.Fatalf("edges = %d, want 0", len(g.Edges))
	}
	if !g.Nodes[0].Egress {
		t.Error("single node should be egress")
	}
}

func TestExtractString(t *testing.T) {
	// VALIDATES: string extraction from route maps.
	tests := []struct {
		name string
		in   map[string]any
		key  string
		want string
	}{
		{"present", map[string]any{"k": "v"}, "k", "v"},
		{"missing", map[string]any{}, "k", ""},
		{"wrong type", map[string]any{"k": 42}, "k", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractString(tt.in, tt.key); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderNextHopGraphText(t *testing.T) {
	// VALIDATES: text output contains mode, nodes with egress flag, and edges.
	g := &NextHopGraph{
		Nodes: []NextHopNode{
			{Address: "10.0.1.1", Layer: 1},
			{Address: "10.0.5.1", Layer: 0, Egress: true},
		},
		Edges: []NextHopEdge{
			{From: "10.0.1.1", To: "10.0.5.1"},
		},
	}

	text := renderNextHopGraphText(g)

	if !strings.Contains(text, "mode nexthop") {
		t.Error("should contain mode nexthop")
	}
	if !strings.Contains(text, "node 10.0.5.1 layer=0 egress") {
		t.Errorf("should contain egress node, got:\n%s", text)
	}
	if !strings.Contains(text, "node 10.0.1.1 layer=1\n") {
		t.Errorf("should contain non-egress node, got:\n%s", text)
	}
	if !strings.Contains(text, "edge 10.0.1.1 -> 10.0.5.1") {
		t.Error("should contain edge")
	}
}

func TestRenderNextHopGraphTextWithName(t *testing.T) {
	// VALIDATES: text output includes name when present.
	g := &NextHopGraph{
		Nodes: []NextHopNode{
			{Address: "10.0.5.1", Layer: 0, Egress: true, Name: "slo-1"},
		},
	}

	text := renderNextHopGraphText(g)

	if !strings.Contains(text, "name=slo-1") {
		t.Errorf("should contain name, got:\n%s", text)
	}
}

func TestNextHopLayoutLayers(t *testing.T) {
	// VALIDATES: egress nodes at right, forwarding nodes at left.
	g := &NextHopGraph{
		Nodes: []NextHopNode{
			{Address: "10.0.1.1", Layer: 1},
			{Address: "10.0.5.1", Layer: 0, Egress: true},
		},
		Edges: []NextHopEdge{
			{From: "10.0.1.1", To: "10.0.5.1"},
		},
	}

	layout := computeNextHopLayout(g)

	pos1 := layout.Positions["10.0.1.1"]
	pos5 := layout.Positions["10.0.5.1"]
	if pos1.X >= pos5.X {
		t.Errorf("forwarding node X=%d should be left of egress X=%d", pos1.X, pos5.X)
	}
}

func TestNextHopLayoutSameLayerVertical(t *testing.T) {
	// VALIDATES: nodes in same layer stacked vertically.
	g := &NextHopGraph{
		Nodes: []NextHopNode{
			{Address: "10.0.1.1", Layer: 1},
			{Address: "10.0.1.2", Layer: 1},
			{Address: "10.0.5.1", Layer: 0, Egress: true},
		},
		Edges: []NextHopEdge{
			{From: "10.0.1.1", To: "10.0.5.1"},
			{From: "10.0.1.2", To: "10.0.5.1"},
		},
	}

	layout := computeNextHopLayout(g)

	pos1 := layout.Positions["10.0.1.1"]
	pos2 := layout.Positions["10.0.1.2"]
	if pos1.X != pos2.X {
		t.Error("same-layer nodes should have same X")
	}
	if pos1.Y == pos2.Y {
		t.Error("same-layer nodes should have different Y")
	}
}

func TestNextHopLayoutEmpty(t *testing.T) {
	// VALIDATES: empty graph returns empty layout.
	layout := computeNextHopLayout(&NextHopGraph{})

	if len(layout.Positions) != 0 {
		t.Errorf("positions = %d, want 0", len(layout.Positions))
	}
}

func TestRenderNextHopGraphSVG(t *testing.T) {
	// VALIDATES: valid SVG with egress styling.
	g := &NextHopGraph{
		Nodes: []NextHopNode{
			{Address: "10.0.1.1", Layer: 1},
			{Address: "10.0.5.1", Layer: 0, Egress: true},
		},
		Edges: []NextHopEdge{
			{From: "10.0.1.1", To: "10.0.5.1"},
		},
	}

	layout := computeNextHopLayout(g)
	svg := renderNextHopGraphSVG(g, layout)

	if !strings.HasPrefix(svg, "<svg") {
		t.Error("SVG should start with <svg")
	}
	if !strings.HasSuffix(svg, "</svg>") {
		t.Error("SVG should end with </svg>")
	}
	if !strings.Contains(svg, "10.0.1.1") {
		t.Error("SVG should contain forwarding node address")
	}
	if !strings.Contains(svg, "10.0.5.1") {
		t.Error("SVG should contain egress node address")
	}
	if !strings.Contains(svg, "egress") {
		t.Error("SVG should contain egress class")
	}
}

func TestFormatNextHopNodeLabel(t *testing.T) {
	// VALIDATES: label formatting with and without name.
	if got := formatNextHopNodeLabel(NextHopNode{Address: "10.0.1.1"}); got != "10.0.1.1" {
		t.Errorf("no name: got %q, want 10.0.1.1", got)
	}
	if got := formatNextHopNodeLabel(NextHopNode{Address: "10.0.1.1", Name: "th-1"}); got != "th-1 10.0.1.1" {
		t.Errorf("with name: got %q, want 'th-1 10.0.1.1'", got)
	}
}

func TestNextHopTooltipName(t *testing.T) {
	// VALIDATES: tooltip formatting.
	if got := nextHopTooltipName(""); got != "" {
		t.Errorf("empty: got %q", got)
	}
	if got := nextHopTooltipName("th-1"); got != " - th-1" {
		t.Errorf("non-empty: got %q, want ' - th-1'", got)
	}
}
