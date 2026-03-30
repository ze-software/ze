// Design: docs/architecture/web-interface.md -- Layered graph layout algorithm
// Related: graph.go -- Graph data model and construction
// Related: handler_graph.go -- Graph HTTP handler

package lg

import (
	"fmt"
	"html/template"
	"sort"
	"strings"
)

// Layout holds computed positions for graph nodes.
type Layout struct {
	Positions map[uint32]Position // ASN -> position
	Width     int                 // Total SVG width
	Height    int                 // Total SVG height
}

// Position holds the x,y coordinates and dimensions for a graph node.
type Position struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Layout constants.
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
func computeLayout(g *Graph) *Layout {
	if len(g.Nodes) == 0 {
		return &Layout{Positions: make(map[uint32]Position)}
	}

	// Group nodes by layer.
	layers := make(map[int][]GraphNode)
	maxLayer := 0
	for _, n := range g.Nodes {
		layers[n.Layer] = append(layers[n.Layer], n)
		if n.Layer > maxLayer {
			maxLayer = n.Layer
		}
	}

	// Sort nodes within each layer by ASN for deterministic layout.
	for layer := range layers {
		sort.Slice(layers[layer], func(i, j int) bool {
			return layers[layer][i].ASN < layers[layer][j].ASN
		})
	}

	positions := make(map[uint32]Position)

	// Compute column widths based on label text.
	colWidths := make(map[int]int)
	for layer, nodes := range layers {
		maxW := nodeMinWidth
		for _, n := range nodes {
			label := formatNodeLabel(n)
			w := min(len(label)*charWidthApprox+nodePadding*2, nodeMaxWidth)
			if w > maxW {
				maxW = w
			}
		}
		colWidths[layer] = maxW
	}

	// Compute x positions (left-to-right, highest layer first).
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

	// Compute y positions within each layer.
	maxHeight := 0
	for layer, nodes := range layers {
		for i, n := range nodes {
			y := graphPadding + i*(nodeHeight+verticalGap)
			w := colWidths[layer]
			positions[n.ASN] = Position{
				X:      colX[layer],
				Y:      y,
				Width:  w,
				Height: nodeHeight,
			}
			bottom := y + nodeHeight
			if bottom > maxHeight {
				maxHeight = bottom
			}
		}
	}

	totalHeight := maxHeight + graphPadding

	return &Layout{
		Positions: positions,
		Width:     totalWidth,
		Height:    totalHeight,
	}
}

// formatNodeLabel returns the display label for a graph node.
func formatNodeLabel(n GraphNode) string {
	if n.Name != "" {
		return fmt.Sprintf("AS%d %s", n.ASN, n.Name)
	}
	return fmt.Sprintf("AS%d", n.ASN)
}

// renderGraphSVG renders the graph as an SVG string.
func renderGraphSVG(g *Graph, layout *Layout) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		layout.Width, layout.Height, layout.Width, layout.Height)
	sb.WriteString("\n")

	// Style.
	sb.WriteString(`<style>
.node rect { fill: #f0f4f8; stroke: #4a90d9; stroke-width: 2; rx: 6; }
.node rect:hover { fill: #dce8f5; stroke: #2a6cb8; }
.node text { font-family: monospace; font-size: 12px; fill: #333; }
.node .asn { font-weight: bold; }
.edge line { stroke: #999; stroke-width: 1.5; }
.edge polygon { fill: #999; }
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

		fmt.Fprintf(&sb, `<g class="edge"><line x1="%d" y1="%d" x2="%d" y2="%d"/>`, x1, y1, x2, y2)

		// Arrow head at destination.
		arrowSize := 8
		fmt.Fprintf(&sb, `<polygon points="%d,%d %d,%d %d,%d"/>`,
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
			label = label[:maxChars-1] + "\u2026"
		}

		sb.WriteString(`<g class="node">`)
		fmt.Fprintf(&sb, `<rect x="%d" y="%d" width="%d" height="%d"/>`,
			pos.X, pos.Y, pos.Width, pos.Height)
		fmt.Fprintf(&sb, `<text x="%d" y="%d" class="asn">%s</text>`,
			pos.X+nodePadding, pos.Y+nodeHeight/2+fontSize/3,
			template.HTMLEscapeString(label))
		fmt.Fprintf(&sb, `<title>%s</title>`,
			template.HTMLEscapeString(fmt.Sprintf("AS%d%s", n.ASN, tooltipName(n.Name))))
		sb.WriteString("</g>\n")
	}

	sb.WriteString("</svg>")
	return sb.String()
}

// tooltipName formats the name portion of a tooltip.
func tooltipName(name string) string {
	if name == "" {
		return ""
	}
	return " - " + name
}
