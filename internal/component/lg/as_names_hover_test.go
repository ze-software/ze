package lg

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// The owner's order for the looking glass: wherever it shows an AS, the
// visible text is "AS" and the number, and the AS name, when the resolve
// command gives one, is shown on mouse-over only. A drawing carries it in the
// node's <title>, a page in the title attribute of the element that shows the
// AS. An AS with no name has no tooltip at all, never an empty one. The text
// forms are read by scripts and keep the name exactly as before.

// hoverTestName is an AS name with markup in it, so each test also proves the
// name is escaped where it is written.
const hoverTestName = "Transit <A>"

// hoverTestRender renders a templ component to a string.
func hoverTestRender(t *testing.T, component templ.Component) string {
	t.Helper()
	var html strings.Builder
	if err := component.Render(context.Background(), &html); err != nil {
		t.Fatalf("render: %v", err)
	}
	return html.String()
}

// hoverTestHolds fails the test for each part the answer does not hold.
func hoverTestHolds(t *testing.T, answer string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(answer, part) {
			t.Errorf("the answer does not hold %q:\n%s", part, answer)
		}
	}
}

// hoverTestNameOnce fails the test unless the escaped name appears exactly
// once, inside the tooltip: never in a visible label, never unescaped.
func hoverTestNameOnce(t *testing.T, answer string) {
	t.Helper()
	if got := strings.Count(answer, "Transit &lt;A&gt;"); got != 1 {
		t.Errorf("the escaped AS name appears %d times, want once, in the tooltip:\n%s", got, answer)
	}
	if strings.Contains(answer, "<A>") {
		t.Errorf("the AS name reached the answer unescaped:\n%s", answer)
	}
}

// VALIDATES: the AS path graph labels a named AS "AS<n>" and puts its name in
// <title> only, writes no <title> for an AS with no name, and sizes the nodes
// by the visible label alone: a name does not move or widen any node.
// PREVENTS: a name drawn as node text, an empty tooltip, a node widened by a
// name nobody sees.
func TestASPathGraphNamesOnMouseOver(t *testing.T) {
	named := &Graph{
		Nodes: []GraphNode{
			{ASN: 65001, Name: hoverTestName, Layer: 1},
			{ASN: 65002, Layer: 0},
		},
		Edges: []GraphEdge{{FromASN: 65001, ToASN: 65002}},
	}
	unnamed := &Graph{
		Nodes: []GraphNode{{ASN: 65001, Layer: 1}, {ASN: 65002, Layer: 0}},
		Edges: named.Edges,
	}

	layout := computeLayout(named)
	plain := computeLayout(unnamed)
	if layout.Width != plain.Width || layout.Height != plain.Height {
		t.Errorf("the named drawing is %dx%d, the unnamed %dx%d: a name changed the size",
			layout.Width, layout.Height, plain.Width, plain.Height)
	}
	for number, position := range plain.Positions {
		if got := layout.Positions[number]; got != position {
			t.Errorf("AS%d at %+v with its name, %+v without: a name moved the node", number, got, position)
		}
	}
	// "AS65001" is 7 characters at 7 pixels plus 16 of padding on both sides.
	if got := layout.Positions[65001].Width; got != 81 {
		t.Errorf("AS65001 is %d wide, want 81, the width of its visible label", got)
	}

	svg := renderGraphSVG(named, layout)
	hoverTestHolds(t, svg,
		`class="asn">AS65001</text><title>AS65001 - Transit &lt;A&gt;</title></g>`,
		`class="asn">AS65002</text></g>`,
	)
	hoverTestNameOnce(t, svg)
	if strings.Contains(svg, "<title>AS65002") {
		t.Errorf("the AS with no name has a tooltip:\n%s", svg)
	}
}

// VALIDATES: the next-hop graph labels an AS outside "AS<n>", with its name in
// <title> only, and writes no <title> for an outside AS with no name. A router
// keeps its tooltip, the address.
// PREVENTS: an outside node widened by its name, an empty tooltip.
func TestNextHopGraphOutsideNamesOnMouseOver(t *testing.T) {
	g := &nextHopGraph{
		Nodes: []NextHopNode{
			{Address: "198.51.100.1", Layer: 1},
			{Address: "AS65101", Layer: 0, Outside: true, AS: 65101, Name: hoverTestName},
			{Address: "AS65102", Layer: 0, Outside: true, AS: 65102},
		},
		Edges: []NextHopEdge{{From: "198.51.100.1", To: "AS65101"}},
	}

	layout := computeNextHopLayout(g)
	if got := layout.Positions["AS65101"].Width; got != 81 {
		t.Errorf("AS65101 is %d wide, want 81, the width of its visible label", got)
	}

	svg := renderNextHopGraphSVG(g, layout)
	hoverTestHolds(t, svg,
		`class="label">AS65101</text><title>AS65101 - Transit &lt;A&gt;</title></g>`,
		`class="label">AS65102</text></g>`,
		`class="label">198.51.100.1</text><title>198.51.100.1</title></g>`,
	)
	hoverTestNameOnce(t, svg)
}

// VALIDATES: the trace graph labels an AS "AS<n>", with its name in <title>
// only, writes no <title> for an AS with no name, and sizes the AS columns by
// the visible label alone.
// PREVENTS: an AS node widened by its name, an empty tooltip.
func TestTraceGraphASNamesOnMouseOver(t *testing.T) {
	routers := traceTestSummary(t, traceTestRouter("198.51.100.1"))
	g := buildTraceGraph(traceTestRoutes(t, traceTestRow("198.51.100.1", "198.51.100.1", 65101, 64500)), routers)
	names := traceNames{as: func(number uint32) string {
		if number == 65101 {
			return hoverTestName
		}
		return ""
	}}

	layout := computeTraceLayout(g, names)
	plain := computeTraceLayout(g, traceNames{})
	for number, position := range plain.ASes {
		if got := layout.ASes[number]; got != position {
			t.Errorf("AS%d at %+v with its name, %+v without: a name moved the node", number, got, position)
		}
	}
	if layout.Width != plain.Width {
		t.Errorf("width %d with the name, %d without", layout.Width, plain.Width)
	}

	svg := renderTraceGraphSVG(g, layout, names)
	hoverTestHolds(t, svg,
		`class="label">AS65101</text><title>AS65101 - Transit &lt;A&gt;</title></g>`,
		`class="label">AS64500</text></g>`,
		`class="label">198.51.100.1</text><title>198.51.100.1</title></g>`,
	)
	hoverTestNameOnce(t, svg)
}

// VALIDATES: the text forms keep the name as they wrote it before the name
// moved to mouse-over: the AS path node line carries it after the number
// (TestRenderGraphText), the trace adds name= (TestTraceGraphTextIsOrdered's
// "node AS65101 name=Transit A"), and the next-hop graph writes an AS outside
// with no name (TestNextHopGraphDrawsOutsidePeersByAS's "node AS65101 layer=0
// outside").
// PREVENTS: a script that reads a text form seeing a changed line.
func TestASNameTextFormsUnchanged(t *testing.T) {
	aspath := renderGraphText(&Graph{Nodes: []GraphNode{{ASN: 65002, Layer: 0, Name: "Example"}}})
	hoverTestHolds(t, aspath, "node AS65002 Example layer=0\n")

	nexthop := renderNextHopGraphText(&nextHopGraph{Nodes: []NextHopNode{
		{Address: "AS65101", Layer: 0, Outside: true, AS: 65101, Name: "Transit A"},
	}})
	hoverTestHolds(t, nexthop, "node AS65101 layer=0 outside\n")

	routers := traceTestSummary(t, traceTestRouter("198.51.100.1"))
	g := buildTraceGraph(traceTestRoutes(t, traceTestRow("198.51.100.1", "198.51.100.1", 65101, 64500)), routers)
	trace := renderTraceGraphText(g, traceNames{as: func(number uint32) string {
		if number == 65101 {
			return "Transit A"
		}
		return ""
	}})
	hoverTestHolds(t, trace, "node AS65101 name=Transit A\n", "node AS64500\n")
}

// VALIDATES: the peer tables show the bare number in the Remote AS column, as
// before, with the AS name as the cell's title, and a cell with no title for
// an AS with no name; the peer header shows "AS <n>" with the name as title.
// PREVENTS: a name drawn as text beside the number, an empty title attribute.
func TestPeerPagesASNamesOnMouseOver(t *testing.T) {
	table := hoverTestRender(t, peersTableBody([]peerRow{
		{Address: "192.0.2.1", RemoteAS: "64500", RemoteASName: hoverTestName},
		{Address: "2001:db8::1", RemoteAS: "64501"},
	}))
	hoverTestHolds(t, table,
		`<td title="Transit &lt;A&gt;">64500</td>`,
		`<td>64501</td>`,
	)
	hoverTestNameOnce(t, table)

	bmp := hoverTestRender(t, bmpPeersContent([]bmpPeerRow{
		{Router: "198.51.100.7", PeerAS: "64502", PeerASName: hoverTestName},
		{Router: "198.51.100.8", PeerAS: "64503"},
	}))
	hoverTestHolds(t, bmp,
		`<td title="Transit &lt;A&gt;">64502</td>`,
		`<td>64503</td>`,
	)
	hoverTestNameOnce(t, bmp)

	header := hoverTestRender(t, peerRoutesContent(peerRoutesView{
		Address: "192.0.2.1",
		Peer:    &peerInfoRow{State: "established", RemoteAS: "64500", RemoteASName: hoverTestName},
	}))
	hoverTestHolds(t, header, `<span title="Transit &lt;A&gt;">AS 64500</span>`)
	hoverTestNameOnce(t, header)

	unnamed := hoverTestRender(t, peerRoutesContent(peerRoutesView{
		Address: "192.0.2.1",
		Peer:    &peerInfoRow{State: "established", RemoteAS: "64500"},
	}))
	hoverTestHolds(t, unnamed, `<span>AS 64500</span>`)
	if strings.Contains(unnamed, "title=\"\"") {
		t.Errorf("the header of an AS with no name has an empty title:\n%s", unnamed)
	}
}
