// Design: docs/architecture/web-interface.md -- Layered graph layout algorithm
// Related: graph.go -- Graph data model and construction
// Related: layout_nexthop.go -- Next-hop graph layout (parallel implementation)
// Related: handler_graph.go -- Graph HTTP handler

package lg

import (
	"fmt"
	"html/template"

	"github.com/ze-software/ze/internal/core/bgp/asn"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/graph"
)

// Layout is a type alias for the shared layout type.
type Layout = graph.Layout

// Position is a type alias for the shared position type.
type Position = graph.Position

// Layout constants for SVG rendering (used by AS path and next-hop graph renderers).
const (
	nodeHeight      = 40
	nodeMinWidth    = 80
	nodeMaxWidth    = 200
	nodePadding     = 16
	horizontalGap   = 120
	verticalGap     = 60
	graphPadding    = 20
	fontSize        = 12
	charWidthApprox = 7 // approximate monospace character width at 12px
)

// computeLayout assigns x,y positions to each graph node using a layered layout.
// Layers are arranged left-to-right: source ASes at left, origin at right.
//
// A node is as wide as its visible label, the AS number alone. The AS name is
// shown on mouse-over only (renderGraphSVG), so the shared layout, which sizes
// a node by the label with the name, is given the nodes without their names.
func computeLayout(g *Graph) *Layout {
	unnamed := Graph{Nodes: make([]GraphNode, len(g.Nodes)), Edges: g.Edges}
	for i, node := range g.Nodes {
		node.Name = ""
		unnamed.Nodes[i] = node
	}
	return graph.ComputeLayout(&unnamed, graph.SVGConfig())
}

// formatNodeLabel returns the visible label of a graph node: "AS" and the
// number, whether or not the AS has a name. The name is the node's tooltip
// (asNodeTooltip), never drawn as text.
func formatNodeLabel(n GraphNode) string {
	return graph.FormatNodeLabel(GraphNode{ASN: n.ASN})
}

// renderGraphSVG renders the graph as an SVG string.
func renderGraphSVG(g *Graph, layout *Layout) string {
	var sb textbuf.Buffer

	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, //nolint:errcheck // report output
		layout.Width, layout.Height, layout.Width, layout.Height)
	sb.WriteString("\n")

	// Style uses CSS variables from the page for dark mode support.
	sb.WriteString(`<style>
.node rect { fill: var(--node-fill, #f0f4f8); stroke: var(--node-stroke, #4a90d9); stroke-width: 2; rx: 6; }
.node rect:hover { fill: var(--node-fill-hover, #dce8f5); stroke: var(--node-stroke-hover, #2a6cb8); }
.node text { font-family: monospace; font-size: 12px; fill: var(--node-text, #333); }
.node .asn { font-weight: bold; }
.edge line { stroke: var(--edge-stroke, #999); stroke-width: 1.5; }
.edge polygon { fill: var(--edge-stroke, #999); }
</style>`)
	sb.WriteString("\n")

	// Render edges first (behind nodes).
	for _, e := range g.Edges {
		fromPos, fromOK := layout.Positions[e.FromASN]
		toPos, toOK := layout.Positions[e.ToASN]
		if !fromOK || !toOK {
			continue
		}

		// Edge from right side of source to left side of destination.
		x1 := fromPos.X + fromPos.Width
		y1 := fromPos.Y + fromPos.Height/2
		x2 := toPos.X
		y2 := toPos.Y + toPos.Height/2

		fmt.Fprintf(&sb, `<g class="edge"><line x1="%d" y1="%d" x2="%d" y2="%d"/>`, x1, y1, x2, y2) //nolint:errcheck // report output

		// Arrow head at destination.
		arrowSize := 8
		fmt.Fprintf(&sb, `<polygon points="%d,%d %d,%d %d,%d"/>`, //nolint:errcheck // report output
			x2, y2,
			x2-arrowSize, y2-arrowSize/2,
			x2-arrowSize, y2+arrowSize/2)
		sb.WriteString("</g>\n")
	}

	// Render nodes.
	for _, n := range g.Nodes {
		pos, ok := layout.Positions[n.ASN]
		if !ok {
			continue
		}

		label := formatNodeLabel(n)
		// Truncate label if too long for node width.
		maxChars := (pos.Width - nodePadding*2) / charWidthApprox
		if maxChars > 0 && len(label) > maxChars {
			var tb textbuf.Buffer
			label = tb.Str(label[:maxChars-1]).Str("\u2026").String()
		}

		sb.WriteString(`<g class="node">`)
		fmt.Fprintf(&sb, `<rect x="%d" y="%d" width="%d" height="%d"/>`, //nolint:errcheck // report output
			pos.X, pos.Y, pos.Width, pos.Height)
		fmt.Fprintf(&sb, `<text x="%d" y="%d" class="asn">%s</text>`, //nolint:errcheck // report output
			pos.X+nodePadding, pos.Y+nodeHeight/2+fontSize/3,
			template.HTMLEscapeString(label))
		if tooltip := asNodeTooltip(n.ASN, n.Name); tooltip != "" {
			fmt.Fprintf(&sb, `<title>%s</title>`, template.HTMLEscapeString(tooltip)) //nolint:errcheck // buffer output
		}
		sb.WriteString("</g>\n")
	}

	sb.WriteString("</svg>")
	return sb.String()
}

// asNodeTooltip is the mouse-over text of the node of an AS in any of the
// three graphs: "AS<n> - <name>", the number in the configured notation. It is
// "" for an AS with no name, whether the resolve command named nothing or
// failed (as_names.go), and the caller then writes no <title> at all: the
// visible label already says "AS<n>", and an empty tooltip says nothing.
func asNodeTooltip(number uint32, name string) string {
	if name == "" {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str("AS").Str(asn.Text(number, asn.Configured())).Str(nextHopTooltipName(name)).String()
}
