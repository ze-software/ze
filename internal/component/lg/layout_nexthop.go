// Design: docs/architecture/web-interface.md -- Next-hop graph layout and SVG rendering
// Related: graph_nexthop.go -- Next-hop graph data model
// Related: layout.go -- AS path graph layout (parallel implementation)
// Related: handler_graph.go -- Graph HTTP handler

package lg

import (
	"fmt"
	"html/template"
	"sort"
	"strings"
)

// NextHopLayout holds computed positions for next-hop graph nodes.
type NextHopLayout struct {
	Positions map[string]Position // Address -> position.
	Width     int
	Height    int
}

// computeNextHopLayout assigns x,y positions using a layered layout.
// Layers left-to-right: forwarding routers at left, egress at right.
func computeNextHopLayout(g *NextHopGraph) *NextHopLayout {
	if len(g.Nodes) == 0 {
		return &NextHopLayout{Positions: make(map[string]Position)}
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

	return &NextHopLayout{
		Positions: positions,
		Width:     totalWidth,
		Height:    maxHeight + graphPadding,
	}
}

// formatNextHopNodeLabel returns the display label for a next-hop graph node.
func formatNextHopNodeLabel(n NextHopNode) string {
	if n.Name != "" {
		return n.Name + " " + n.Address
	}
	return n.Address
}

// renderNextHopGraphSVG renders the next-hop graph as an SVG string.
func renderNextHopGraphSVG(g *NextHopGraph, layout *NextHopLayout) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		layout.Width, layout.Height, layout.Width, layout.Height)
	sb.WriteString("\n")

	sb.WriteString(`<style>
.node rect { fill: var(--node-fill, #f0f4f8); stroke: var(--node-stroke, #4a90d9); stroke-width: 2; rx: 6; }
.node rect:hover { fill: var(--node-fill-hover, #dce8f5); stroke: var(--node-stroke-hover, #2a6cb8); }
.node text { font-family: monospace; font-size: 12px; fill: var(--node-text, #333); }
.node .label { font-weight: bold; }
.egress rect { fill: var(--egress-fill, #e8f5e9); stroke: var(--egress-stroke, #4caf50); }
.egress rect:hover { fill: var(--egress-fill-hover, #c8e6c9); stroke: var(--egress-stroke-hover, #388e3c); }
.edge line { stroke: var(--edge-stroke, #999); stroke-width: 1.5; }
.edge polygon { fill: var(--edge-stroke, #999); }
</style>`)
	sb.WriteString("\n")

	// Render edges first (behind nodes).
	for _, e := range g.Edges {
		fromPos, fromOK := layout.Positions[e.From]
		toPos, toOK := layout.Positions[e.To]
		if !fromOK || !toOK {
			continue
		}

		x1 := fromPos.X + fromPos.Width
		y1 := fromPos.Y + fromPos.Height/2
		x2 := toPos.X
		y2 := toPos.Y + toPos.Height/2

		fmt.Fprintf(&sb, `<g class="edge"><line x1="%d" y1="%d" x2="%d" y2="%d"/>`, x1, y1, x2, y2)

		arrowSize := 8
		fmt.Fprintf(&sb, `<polygon points="%d,%d %d,%d %d,%d"/>`,
			x2, y2,
			x2-arrowSize, y2-arrowSize/2,
			x2-arrowSize, y2+arrowSize/2)
		sb.WriteString("</g>\n")
	}

	// Render nodes.
	for _, n := range g.Nodes {
		pos, ok := layout.Positions[n.Address]
		if !ok {
			continue
		}

		class := "node"
		if n.Egress {
			class = "node egress"
		}

		label := formatNextHopNodeLabel(n)
		maxChars := (pos.Width - nodePadding*2) / charWidthApprox
		if maxChars > 0 && len(label) > maxChars {
			label = label[:maxChars-1] + "\u2026"
		}

		fmt.Fprintf(&sb, `<g class="%s">`, class)
		fmt.Fprintf(&sb, `<rect x="%d" y="%d" width="%d" height="%d"/>`,
			pos.X, pos.Y, pos.Width, pos.Height)
		fmt.Fprintf(&sb, `<text x="%d" y="%d" class="label">%s</text>`,
			pos.X+nodePadding, pos.Y+nodeHeight/2+fontSize/3,
			template.HTMLEscapeString(label))
		fmt.Fprintf(&sb, `<title>%s%s</title>`,
			template.HTMLEscapeString(n.Address),
			template.HTMLEscapeString(nextHopTooltipName(n.Name)))
		sb.WriteString("</g>\n")
	}

	sb.WriteString("</svg>")
	return sb.String()
}

// nextHopTooltipName formats the name portion of a next-hop tooltip.
func nextHopTooltipName(name string) string {
	if name == "" {
		return ""
	}
	return " - " + name
}
