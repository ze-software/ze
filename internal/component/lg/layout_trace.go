// Design: docs/architecture/web-interface.md -- Trace graph layout and SVG rendering
// Related: graph_trace.go -- Trace graph data model, walk and text form
// Related: layout_nexthop.go -- The SVG writers and the style this drawing uses
// Related: layout.go -- The node and gap sizes shared by the three drawings
// Related: handler_graph.go -- Graph HTTP handler

package lg

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The classes of the two groups of text under the drawing: the line of each
// walk, and traceNote. A reader of the SVG finds each by its class.
const (
	traceEndsClass = "trace-ends"
	traceNoteClass = "trace-note"
)

// traceLineHeight is the distance between two lines of the text under the
// drawing, in pixels.
const traceLineHeight = 18

// traceLayout holds where the trace graph draws each node, and the
// text it writes under the nodes. Only computeTraceLayout builds one.
type traceLayout struct {
	Addresses map[netip.Addr]Position // Routers of the AS and the next hops of traceGraph.Neighbors.
	ASes      map[uint32]Position
	// Ends holds one line per walk, in the order of traceGraph.Walks: the
	// prefix, the starting router and the sentence of the walk's end. The
	// drawing shows where a walk went, and only this line says why it ended.
	// Past traceWalksListedMax walks the last line says how many have none.
	Ends []string
	// LinesTop is the y the text starts under. Line i is written at
	// LinesTop + (i+1)*traceLineHeight.
	LinesTop int
	Width    int
	Height   int
}

// lineY answers the baseline of line i of the text under the drawing.
func (l *traceLayout) lineY(i int) int {
	return l.LinesTop + (i+1)*traceLineHeight
}

// computeTraceLayout places the nodes in columns, left to right in the
// direction of the walk: a router of the AS in the column of the furthest
// place a walk reaches it at, then the next hops of traceGraph.Neighbors, then
// each AS in the column of its furthest place in an exit route's path, the
// origin AS last.
//
// A router's column is below the number of routers and an AS's below the
// longest path, so the columns are bounded by the node cap the handler applied
// (traceGraph.undrawn). Two routers that name each other share a column, and
// the line of each walk says that it would loop.
func computeTraceLayout(g *traceGraph, names traceNames) *traceLayout {
	routerColumn, routerColumns := traceRouterColumns(g)
	asFirst := routerColumns
	if len(g.Neighbors) > 0 {
		asFirst++
	}
	asColumn, columns := traceASColumns(g, asFirst)

	widths := make([]int, columns)
	for i := range widths {
		widths[i] = nodeMinWidth
	}
	widen := func(column int, label string) {
		widths[column] = max(widths[column], min(len(label)*charWidthApprox+nodePadding*2, nodeMaxWidth))
	}
	for _, router := range g.Routers {
		widen(routerColumn[router.Address], traceAddressLabel(router.Address, names))
	}
	for _, neighbor := range g.Neighbors {
		widen(routerColumns, traceAddressLabel(neighbor, names))
	}
	for _, number := range g.ASes {
		widen(asColumn[number], traceASLabel(number))
	}

	lefts := make([]int, columns)
	width := graphPadding
	for i := range lefts {
		lefts[i] = width
		width += widths[i] + horizontalGap
	}
	if columns > 0 {
		width -= horizontalGap
	}
	width += graphPadding

	// Each column is filled top to bottom in the graph's own order, which is
	// sorted, so two layouts of the same graph are equal.
	rows := make([]int, columns)
	nodesBottom := graphPadding
	place := func(column int) Position {
		y := graphPadding + rows[column]*(nodeHeight+verticalGap)
		rows[column]++
		nodesBottom = max(nodesBottom, y+nodeHeight)
		return Position{X: lefts[column], Y: y, Width: widths[column], Height: nodeHeight}
	}

	layout := &traceLayout{
		Addresses: make(map[netip.Addr]Position, len(g.Routers)+len(g.Neighbors)),
		ASes:      make(map[uint32]Position, len(g.ASes)),
		Ends:      traceEndLines(g),
	}
	for _, router := range g.Routers {
		layout.Addresses[router.Address] = place(routerColumn[router.Address])
	}
	for _, neighbor := range g.Neighbors {
		layout.Addresses[neighbor] = place(routerColumns)
	}
	for _, number := range g.ASes {
		layout.ASes[number] = place(asColumn[number])
	}

	lineLengthMax := len(traceNote)
	for _, line := range layout.Ends {
		lineLengthMax = max(lineLengthMax, len(line))
	}

	layout.LinesTop = nodesBottom + graphPadding
	layout.Width = max(width, graphPadding*2+lineLengthMax*charWidthApprox)
	// The lines of the walks, then the note.
	layout.Height = layout.lineY(len(layout.Ends)) + graphPadding
	return layout
}

// traceRouterColumns answers the column of each router, and how many columns
// the routers take: a router sits at the furthest place any walk reaches it
// at, the starting router of a walk being place zero.
func traceRouterColumns(g *traceGraph) (column map[netip.Addr]int, columns int) {
	column = make(map[netip.Addr]int, len(g.Routers))
	for _, walk := range g.Walks {
		for place, router := range walk.Routers {
			column[router] = max(column[router], place)
			columns = max(columns, place+1)
		}
	}
	return column, columns
}

// traceASColumns answers the column of each AS, and how many columns the
// whole drawing takes. The first AS of a path sits in column first, and each
// step of the path one column further. A prepended AS is one step, as it is
// one node (traceASChain).
func traceASColumns(g *traceGraph, first int) (column map[uint32]int, columns int) {
	column = make(map[uint32]int, len(g.ASes))
	columns = first
	for _, walk := range g.Walks {
		place := first
		for i, number := range walk.ASPath {
			if i > 0 && walk.ASPath[i-1] != number {
				place++
			}
			column[number] = max(column[number], place)
			columns = max(columns, place+1)
		}
	}
	return column, columns
}

// traceEndLines answers the line of each listed walk, in the order of g.Walks,
// then the line that counts the walks left unlisted when there are any
// (traceGraph.listedWalks).
func traceEndLines(g *traceGraph) []string {
	listed, unlistedCount := g.listedWalks()
	lines := make([]string, 0, len(listed)+1)
	for _, walk := range listed {
		var line textbuf.Buffer
		line.Str(walk.Prefix.prefix).Str(" from ").Addr(walk.Routers[0]).Str(": ").Str(walk.statement())
		lines = append(lines, line.String())
	}
	if unlistedCount > 0 {
		lines = append(lines, traceUnlistedSentence(unlistedCount))
	}
	return lines
}

// traceAddressLabel is the label of a router or a next hop of
// traceGraph.Neighbors: its
// address, after the operator's name for it when there is one, as the next-hop
// graph writes it (formatNextHopNodeLabel).
func traceAddressLabel(address netip.Addr, names traceNames) string {
	return formatNextHopNodeLabel(NextHopNode{Address: address.String(), Name: names.ofAddress(address)})
}

// traceASLabel is the visible label of an AS: "AS" and its number in the
// configured notation, whether or not the AS has a name. The name is the
// node's tooltip (asNodeTooltip), so it never widens the node
// (computeTraceLayout sizes a column by this label).
func traceASLabel(number uint32) string {
	var label textbuf.Buffer
	return label.Str("AS").Str(asn.Text(number, asn.Configured())).String()
}

// renderTraceGraphSVG renders the trace graph: the hops inside the AS,
// the edge from each exit to the first AS of its route, the AS chains, then
// under them the line of every walk and traceNote.
//
// layout MUST be computeTraceLayout of the same g and names: every node is
// looked up in it.
func renderTraceGraphSVG(g *traceGraph, layout *traceLayout, names traceNames) string {
	var svg textbuf.Buffer

	writeRouterGraphOpen(&svg, layout.Width, layout.Height)

	// Edges first, behind the nodes.
	for _, edge := range g.HopEdges {
		writeRouterGraphEdge(&svg, layout.Addresses[edge.From], layout.Addresses[edge.To])
	}
	for _, edge := range g.EntryEdges {
		writeRouterGraphEdge(&svg, layout.Addresses[edge.From], layout.ASes[edge.AS])
	}
	for _, edge := range g.ASEdges {
		writeRouterGraphEdge(&svg, layout.ASes[edge.From], layout.ASes[edge.To])
	}

	for _, router := range g.Routers {
		class := routerNodeClass
		if router.Exit {
			// The exit is marked as the next-hop graph marks egress.
			class = routerNodeEgressClass
		}
		writeTraceAddressNode(&svg, class, router.Address, layout, names)
	}
	for _, neighbor := range g.Neighbors {
		writeTraceAddressNode(&svg, routerNodeClass, neighbor, layout, names)
	}
	for _, number := range g.ASes {
		writeRouterGraphNode(&svg, routerNodeClass, traceASLabel(number), asNodeTooltip(number, names.ofAS(number)), layout.ASes[number])
	}

	ends := make([]routerGraphLine, 0, len(layout.Ends))
	for i, line := range layout.Ends {
		ends = append(ends, routerGraphLine{Y: layout.lineY(i), Text: line})
	}
	writeRouterGraphLines(&svg, traceEndsClass, ends)
	writeRouterGraphLines(&svg, traceNoteClass, []routerGraphLine{
		{Y: layout.lineY(len(layout.Ends)), Text: traceNote},
	})
	writeRouterGraphClose(&svg)
	return svg.String()
}

// writeTraceAddressNode writes the node of a router or of a next hop of
// traceGraph.Neighbors.
func writeTraceAddressNode(svg *textbuf.Buffer, class string, address netip.Addr, layout *traceLayout, names traceNames) {
	var tooltip textbuf.Buffer
	tooltip.Addr(address).Str(nextHopTooltipName(names.ofAddress(address)))
	writeRouterGraphNode(svg, class, traceAddressLabel(address, names), tooltip.String(), layout.Addresses[address])
}
