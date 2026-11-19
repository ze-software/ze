// Design: docs/architecture/web-interface.md -- Next-hop graph layout and SVG rendering
// Related: graph_nexthop.go -- Next-hop graph data model
// Related: layout.go -- AS path graph layout (parallel implementation)
// Related: handler_graph.go -- Graph HTTP handler
// Related: layout_trace.go -- Trace graph layout, drawn with this file's writers

package lg

import (
	"fmt"
	"html/template"
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// nextHopLayout holds computed positions for next-hop graph nodes.
type nextHopLayout struct {
	Positions map[string]Position // Address -> position.
	Width     int
	Height    int
}

// computeNextHopLayout assigns x,y positions using a layered layout.
// Layers left-to-right: forwarding routers at left, egress at right.
func computeNextHopLayout(g *nextHopGraph) *nextHopLayout {
	if len(g.Nodes) == 0 {
		return &nextHopLayout{Positions: make(map[string]Position)}
	}

	// Group nodes by layer.
	layers := make(map[int][]NextHopNode)
	maxLayer := 0
	for _, n := range g.Nodes {
		layers[n.Layer] = append(layers[n.Layer], n)
		if n.Layer > maxLayer {
			maxLayer = n.Layer
		}
	}

	// Sort within each layer by address for deterministic layout.
	for layer := range layers {
		sort.Slice(layers[layer], func(i, j int) bool {
			return layers[layer][i].Address < layers[layer][j].Address
		})
	}

	positions := make(map[string]Position)

	// Compute column widths based on label text.
	colWidths := make(map[int]int)
	for layer, nodes := range layers {
		maxW := nodeMinWidth
		for _, n := range nodes {
			label := formatNextHopNodeLabel(n)
			w := min(len(label)*charWidthApprox+nodePadding*2, nodeMaxWidth)
			if w > maxW {
				maxW = w
			}
		}
		colWidths[layer] = maxW
	}

	// X positions: highest layer (source) at left, layer 0 (egress) at right.
	colX := make(map[int]int)
	x := graphPadding
	for layer := maxLayer; layer >= 0; layer-- {
		colX[layer] = x
		w, ok := colWidths[layer]
		if !ok {
			w = nodeMinWidth
		}
		x += w + horizontalGap
	}
	totalWidth := x - horizontalGap + graphPadding

	// Y positions within each layer.
	maxHeight := 0
	for layer, nodes := range layers {
		for i, n := range nodes {
			y := graphPadding + i*(nodeHeight+verticalGap)
			w := colWidths[layer]
			positions[n.Address] = Position{
				X:      colX[layer],
				Y:      y,
				Width:  w,
				Height: nodeHeight,
			}
			if bottom := y + nodeHeight; bottom > maxHeight {
				maxHeight = bottom
			}
		}
	}

	return &nextHopLayout{
		Positions: positions,
		Width:     totalWidth,
		Height:    maxHeight + graphPadding,
	}
}

// formatNextHopNodeLabel returns the display label for a next-hop graph node.
func formatNextHopNodeLabel(n NextHopNode) string {
	if n.Name != "" {
		var tb textbuf.Buffer
		return tb.Str(n.Name).Byte(' ').Str(n.Address).String()
	}
	return n.Address
}

// routerGraphStyle is the style block of a drawing whose nodes are routers:
// the next-hop graph and the trace graph (layout_trace.go). A router
// that is an exit carries the class "egress" beside "node".
const routerGraphStyle = `<style>
.node rect { fill: var(--node-fill, #f0f4f8); stroke: var(--node-stroke, #4a90d9); stroke-width: 2; rx: 6; }
.node rect:hover { fill: var(--node-fill-hover, #dce8f5); stroke: var(--node-stroke-hover, #2a6cb8); }
.node text { font-family: monospace; font-size: 12px; fill: var(--node-text, #333); }
.node .label { font-weight: bold; }
.egress rect { fill: var(--egress-fill, #e8f5e9); stroke: var(--egress-stroke, #4caf50); }
.egress rect:hover { fill: var(--egress-fill-hover, #c8e6c9); stroke: var(--egress-stroke-hover, #388e3c); }
.edge line { stroke: var(--edge-stroke, #999); stroke-width: 1.5; }
.edge polygon { fill: var(--edge-stroke, #999); }
</style>`

// Classes of a router node (routerGraphStyle).
const (
	routerNodeClass       = "node"
	routerNodeEgressClass = "node egress"
)

// renderNextHopGraphSVG renders the next-hop graph as an SVG string.
func renderNextHopGraphSVG(g *nextHopGraph, layout *nextHopLayout) string {
	var sb textbuf.Buffer

	writeRouterGraphOpen(&sb, layout.Width, layout.Height)

	// Render edges first (behind nodes).
	for _, e := range g.Edges {
		fromPos, fromOK := layout.Positions[e.From]
		toPos, toOK := layout.Positions[e.To]
		if !fromOK || !toOK {
			continue
		}
		writeRouterGraphEdge(&sb, fromPos, toPos)
	}

	// Render nodes.
	for _, n := range g.Nodes {
		pos, ok := layout.Positions[n.Address]
		if !ok {
			continue
		}

		class := routerNodeClass
		if n.Egress {
			class = routerNodeEgressClass
		}

		var tooltip textbuf.Buffer
		tooltip.Str(n.Address).Str(nextHopTooltipName(n.Name))
		writeRouterGraphNode(&sb, class, formatNextHopNodeLabel(n), tooltip.String(), pos)
	}

	writeRouterGraphClose(&sb)
	return sb.String()
}

// writeRouterGraphOpen writes the opening <svg> tag of a drawing of the given
// size and routerGraphStyle. The caller MUST call writeRouterGraphClose after
// the last element.
func writeRouterGraphOpen(sb *textbuf.Buffer, width, height int) {
	fmt.Fprintf(sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, //nolint:errcheck // buffer output
		width, height, width, height)
	sb.WriteString("\n")
	sb.WriteString(routerGraphStyle)
	sb.WriteString("\n")
}

// writeRouterGraphClose closes the drawing. It MUST be called after
// writeRouterGraphOpen, once, as the last write.
func writeRouterGraphClose(sb *textbuf.Buffer) {
	sb.WriteString("</svg>")
}

// routerGraphLine is one line of text under the nodes of a drawing: the
// baseline a layout pass computed for it, and its text.
type routerGraphLine struct {
	Y    int
	Text string
}

// writeRouterGraphLines writes a group of text lines under the nodes, each at
// the left padding and at its own baseline. The text is escaped here. A line
// carries its own font and takes the page's text color, as the empty graph
// does (graph_empty.templ), because it is outside any node.
//
// The writer is here, with the other writers of a router drawing, so that the
// one file that builds this markup in Go stays the one the markup guard names
// (markup_check_test.go, lgMarkupExempt).
func writeRouterGraphLines(sb *textbuf.Buffer, class string, lines []routerGraphLine) {
	fmt.Fprintf(sb, `<g class="%s">`, class) //nolint:errcheck // buffer output
	sb.WriteString("\n")
	for _, line := range lines {
		fmt.Fprintf(sb, `<text x="%d" y="%d" font-family="monospace" font-size="%d" fill="currentColor">%s</text>`, //nolint:errcheck // buffer output
			graphPadding, line.Y, fontSize, template.HTMLEscapeString(line.Text))
		sb.WriteString("\n")
	}
	sb.WriteString("</g>\n")
}

// writeRouterGraphEdge writes one arrow from the right side of a node to the
// left side of another.
func writeRouterGraphEdge(sb *textbuf.Buffer, from, to Position) {
	x1 := from.X + from.Width
	y1 := from.Y + from.Height/2
	x2 := to.X
	y2 := to.Y + to.Height/2

	fmt.Fprintf(sb, `<g class="edge"><line x1="%d" y1="%d" x2="%d" y2="%d"/>`, x1, y1, x2, y2) //nolint:errcheck // buffer output

	arrowSize := 8
	fmt.Fprintf(sb, `<polygon points="%d,%d %d,%d %d,%d"/>`, //nolint:errcheck // buffer output
		x2, y2,
		x2-arrowSize, y2-arrowSize/2,
		x2-arrowSize, y2+arrowSize/2)
	sb.WriteString("</g>\n")
}

// writeRouterGraphNode writes one node: its box, its label cut to the box, and
// its tooltip. label and tooltip are escaped here, so a caller passes operator
// text (a range name, an AS name) as it is.
func writeRouterGraphNode(sb *textbuf.Buffer, class, label, tooltip string, pos Position) {
	maxChars := (pos.Width - nodePadding*2) / charWidthApprox
	if maxChars > 0 && len(label) > maxChars {
		var tb textbuf.Buffer
		label = tb.Str(label[:maxChars-1]).Str("\u2026").String()
	}

	fmt.Fprintf(sb, `<g class="%s">`, class)                        //nolint:errcheck // buffer output
	fmt.Fprintf(sb, `<rect x="%d" y="%d" width="%d" height="%d"/>`, //nolint:errcheck // buffer output
		pos.X, pos.Y, pos.Width, pos.Height)
	fmt.Fprintf(sb, `<text x="%d" y="%d" class="label">%s</text>`, //nolint:errcheck // buffer output
		pos.X+nodePadding, pos.Y+nodeHeight/2+fontSize/3,
		template.HTMLEscapeString(label))
	fmt.Fprintf(sb, `<title>%s</title>`, template.HTMLEscapeString(tooltip)) //nolint:errcheck // buffer output
	sb.WriteString("</g>\n")
}

// nextHopTooltipName formats the name portion of a next-hop tooltip.
func nextHopTooltipName(name string) string {
	if name == "" {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str(" - ").Str(name).String()
}
