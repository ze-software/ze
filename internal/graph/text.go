// Design: docs/architecture/web-interface.md -- Unicode box-drawing text renderer for AS topology
// Related: graph.go -- Graph data model and construction
// Related: layout.go -- Layered graph layout algorithm

package graph

import (
	"fmt"
	"strings"
)

// RenderText renders the graph as Unicode box-drawing art on a character grid.
// Returns an empty string for empty graphs. Returns a message for oversized graphs.
func RenderText(g *Graph) string {
	if len(g.Nodes) == 0 {
		return ""
	}

	if len(g.Nodes) > MaxNodes {
		return fmt.Sprintf("graph too many nodes (%d, limit %d)\n", len(g.Nodes), MaxNodes)
	}

	cfg := TextConfig()
	layout := ComputeLayout(g, cfg)

	// Allocate character grid.
	grid := newGrid(layout.Width, layout.Height)

	// Draw nodes (boxes).
	for _, n := range g.Nodes {
		pos, ok := layout.Positions[n.ASN]
		if !ok {
			continue
		}
		drawBox(grid, pos, FormatNodeLabel(n))
	}

	// Draw edges.
	for _, e := range g.Edges {
		fromPos, fromOK := layout.Positions[e.FromASN]
		toPos, toOK := layout.Positions[e.ToASN]
		if !fromOK || !toOK {
			continue
		}
		drawEdge(grid, fromPos, toPos)
	}

	return grid.String()
}

// grid is a 2D character grid for text rendering.
type grid struct {
	cells  [][]rune
	width  int
	height int
}

func newGrid(width, height int) *grid {
	// Ensure minimum dimensions.
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	cells := make([][]rune, height)
	for i := range cells {
		cells[i] = make([]rune, width)
		for j := range cells[i] {
			cells[i][j] = ' '
		}
	}
	return &grid{cells: cells, width: width, height: height}
}

func (g *grid) set(x, y int, ch rune) {
	if y >= 0 && y < g.height && x >= 0 && x < g.width {
		g.cells[y][x] = ch
	}
}

func (g *grid) get(x, y int) rune {
	if y >= 0 && y < g.height && x >= 0 && x < g.width {
		return g.cells[y][x]
	}
	return ' '
}

func (g *grid) String() string {
	var sb strings.Builder
	for _, row := range g.cells {
		line := strings.TrimRight(string(row), " ")
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// drawBox draws a Unicode box at the given position with the label centered.
// Box structure (3 rows):
//
//	┌─────────┐
//	│  AS100  │
//	└─────────┘
func drawBox(g *grid, pos Position, label string) {
	x, y, w := pos.X, pos.Y, pos.Width

	// Truncate label if needed.
	innerW := max(w-2, 0) // minus left and right border
	if len(label) > innerW {
		if innerW > 1 {
			label = label[:innerW-1] + "\u2026"
		} else {
			label = label[:innerW]
		}
	}

	// Top border.
	g.set(x, y, '\u250C') // ┌
	for i := 1; i < w-1; i++ {
		g.set(x+i, y, '\u2500') // ─
	}
	g.set(x+w-1, y, '\u2510') // ┐

	// Middle row with label.
	midY := y + 1
	g.set(x, midY, '\u2502') // │
	// Center the label.
	padLeft := (innerW - len(label)) / 2
	for i := 1; i < w-1; i++ {
		labelIdx := i - 1 - padLeft
		if labelIdx >= 0 && labelIdx < len(label) {
			g.set(x+i, midY, rune(label[labelIdx]))
		} else {
			g.set(x+i, midY, ' ')
		}
	}
	g.set(x+w-1, midY, '\u2502') // │

	// Bottom border.
	botY := y + 2
	g.set(x, botY, '\u2514') // └
	for i := 1; i < w-1; i++ {
		g.set(x+i, botY, '\u2500') // ─
	}
	g.set(x+w-1, botY, '\u2518') // ┘
}

// setIfEmpty writes a character only if the cell is currently a space.
// This prevents edges from overwriting box characters.
func (g *grid) setIfEmpty(x, y int, ch rune) {
	if g.get(x, y) == ' ' {
		g.set(x, y, ch)
	}
}

// drawEdge draws a horizontal edge from the right side of fromPos to the left side of toPos.
// If they are at the same Y, it draws a straight horizontal arrow.
// If at different Y, it draws an L-shaped path: horizontal, vertical, horizontal.
// Edge characters never overwrite existing box characters.
func drawEdge(g *grid, fromPos, toPos Position) {
	// Edge starts from the right side of source node, middle row.
	fromX := fromPos.X + fromPos.Width
	fromY := fromPos.Y + 1 // middle row of the 3-row box

	// Edge ends at the left side of destination node, middle row.
	toX := toPos.X - 1
	toY := toPos.Y + 1

	if fromY == toY {
		// Straight horizontal edge.
		for x := fromX; x < toX; x++ {
			g.setIfEmpty(x, fromY, '\u2500') // ─
		}
		g.setIfEmpty(toX, toY, '>') // arrow head
		return
	}

	// L-shaped edge: horizontal from source, turn, vertical, turn, horizontal to dest.
	// Place the vertical segment right after the source node.
	midX := fromX + 1
	if midX >= toX {
		midX = fromX
	}

	// Horizontal segment from source to midX.
	for x := fromX; x < midX; x++ {
		g.setIfEmpty(x, fromY, '\u2500') // ─
	}

	// Vertical segment.
	if fromY < toY {
		g.setIfEmpty(midX, fromY, '\u2510') // ┐ (turn down)
		for y := fromY + 1; y < toY; y++ {
			g.setIfEmpty(midX, y, '\u2502') // │
		}
		g.setIfEmpty(midX, toY, '\u2514') // └ (turn right)
	} else {
		g.setIfEmpty(midX, fromY, '\u2518') // ┘ (turn up)
		for y := fromY - 1; y > toY; y-- {
			g.setIfEmpty(midX, y, '\u2502') // │
		}
		g.setIfEmpty(midX, toY, '\u250C') // ┌ (turn right)
	}

	// Horizontal segment from midX to destination.
	for x := midX + 1; x < toX; x++ {
		g.setIfEmpty(x, toY, '\u2500') // ─
	}
	g.setIfEmpty(toX, toY, '>') // arrow head
}
