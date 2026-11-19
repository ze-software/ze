// Related: graph_nexthop.go -- nextHopOutsideFrom and the node of an AS outside the local one
// Related: graph_trace.go -- the exit toward an address outside the AS
// Related: graph_trace_mode_test.go -- traceModeEngine and traceModeRequest, the engine these tests ask
// Related: graph_golden_test.go -- the two networks, and the whole drawings captured

package lg

import (
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

// The rule these tests pin, in the owner's words: "for IBGP we have the IP,
// for outside only the ASN". A router of the AS is drawn by its address. A
// peer outside the AS is drawn by its AS number only, in the next-hop graph
// and in the trace.

// outsideTestRoutersOnly answers the engine's rows over a summary that holds
// the four routers and no external session: the rows of 203.0.113.x are then
// rows of a peer that holds no session, as a lab injects them.
func outsideTestRoutersOnly(engine traceModeEngine) traceModeEngine {
	engine.summary = traceTestSummaryText(
		traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.2"),
		traceTestRouter("198.51.100.3"), traceTestRouter("198.51.100.4"),
	)
	return engine
}

// outsideTestAnswer asks one graph and answers its body. The request MUST be
// answered 200.
func outsideTestAnswer(t *testing.T, engine traceModeEngine, query string) string {
	t.Helper()
	rec, _ := traceModeRequest(t, engine, query)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: status = %d, want 200; body %s", query, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// VALIDATES: the next-hop graph of a prefix with two exits draws each external
// peer as its AS, marked `outside`, and each router of the AS by its address,
// from ONE RIB query and the peer summary. The address of an external peer is
// nowhere in the text form or the drawing.
// PREVENTS: a neighbor outside the AS drawn as a bare address, which nothing
// tells apart from a router of the AS.
func TestNextHopGraphDrawsOutsidePeersByAS(t *testing.T) {
	rec, commands := traceModeRequest(t, lgGraphTwoExits(), "&mode=nexthop&format=text")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	// One RIB query, the peer summary, then one name lookup per outside AS,
	// asked once each through the resolve command (as_names.go).
	if want := []string{traceModeRIBCommand, cmdBGPOverview, "resolve cymru asn-name 65101", "resolve cymru asn-name 65102"}; strings.Join(commands, "|") != strings.Join(want, "|") {
		t.Fatalf("commands = %q, want %q", commands, want)
	}

	want := "mode nexthop\n" +
		"node 198.51.100.4 layer=2\n" +
		"node 198.51.100.3 layer=1\n" +
		"node 198.51.100.1 layer=0 egress\n" +
		"node 198.51.100.2 layer=0 egress\n" +
		"node AS65101 layer=0 outside\n" +
		"node AS65102 layer=0 outside\n" +
		"edge 198.51.100.3 -> 198.51.100.1\n" +
		"edge 198.51.100.4 -> 198.51.100.3\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}

	drawing := outsideTestAnswer(t, lgGraphTwoExits(), "&mode=nexthop")
	for _, part := range []string{
		outsideNodeStyle,
		`<g class="node outside">`,
		// The AS name is the one the AS path graph shows, escaped.
		"<title>AS65101 Transit &lt;A&gt;</title>",
		"<title>AS65102</title>",
		`<g class="node egress">`,
	} {
		if !strings.Contains(drawing, part) {
			t.Errorf("the drawing does not hold %q:\n%s", part, drawing)
		}
	}
	if got := strings.Count(drawing, `<g class="node outside">`); got != 2 {
		t.Errorf("%d nodes are drawn outside, want the two ASes", got)
	}
	if got := strings.Count(drawing, `<g class="node egress">`); got != 2 {
		t.Errorf("%d nodes are marked egress, want the two exit routers and no AS", got)
	}
	for name, answer := range map[string]string{"text": rec.Body.String(), "drawing": drawing} {
		if strings.Contains(answer, "203.0.113.") {
			t.Errorf("the %s holds the address of a peer outside the AS:\n%s", name, answer)
		}
	}
}

// VALIDATES: two external peers of one AS are ONE node, and the edges of the
// routers that point at either of them merge on it.
// PREVENTS: one node per external session, which draws one AS twice.
func TestNextHopGraphMergesExternalPeersOfOneAS(t *testing.T) {
	engine := traceModeEngine{
		summary: traceTestSummaryText(
			traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.2"),
			traceTestPeer{address: "203.0.113.1", remoteAS: 65101},
			traceTestPeer{address: "203.0.113.2", remoteAS: 65101},
		),
		rows: []string{
			traceTestRow("198.51.100.1", "203.0.113.1", 65101, 64500),
			traceTestRow("198.51.100.2", "203.0.113.2", 65101, 64500),
			traceTestRow("203.0.113.1", "203.0.113.1", 65101, 64500),
			traceTestRow("203.0.113.2", "203.0.113.2", 65101, 64500),
		},
	}

	want := "mode nexthop\n" +
		"node 198.51.100.1 layer=1\n" +
		"node 198.51.100.2 layer=1\n" +
		"node AS65101 layer=0 outside\n" +
		"edge 198.51.100.1 -> AS65101\n" +
		"edge 198.51.100.2 -> AS65101\n"
	if got := outsideTestAnswer(t, engine, "&mode=nexthop&format=text"); got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}

	// A router that keeps the neighbor's next hop is no exit by its own
	// address, so nothing is marked egress: the AS is where the routes leave.
	drawing := outsideTestAnswer(t, engine, "&mode=nexthop")
	if got := strings.Count(drawing, `<g class="node outside">`); got != 1 {
		t.Errorf("%d nodes are drawn outside, want one for the one AS:\n%s", got, drawing)
	}
	if strings.Contains(drawing, `<g class="node egress">`) {
		t.Errorf("a node is marked egress, and no router announces its own address:\n%s", drawing)
	}
}

// VALIDATES: the guard -- an address that is no peer of the summary stays an
// address, exactly as before the rule: the same rows over a summary with no
// external session draw every node by address with today's marks and names,
// and the drawing holds no outside node and no second style block.
// PREVENTS: an AS derived for an address that holds no session with Ze, and a
// drawing of addresses alone whose bytes changed (the Linux lab's captures).
//
// The whole bytes are pinned by the two `-no-session` captures of
// TestLGGraphGoldenOutput, which equal the captures the same rows produced
// before the rule.
func TestNextHopGraphKeepsAnAddressThatIsNoPeer(t *testing.T) {
	engine := outsideTestRoutersOnly(lgGraphTwoExits())

	want := "mode nexthop\n" +
		"node 198.51.100.4 layer=2\n" +
		"node 198.51.100.3 layer=1\n" +
		"node 198.51.100.1 layer=0 egress\n" +
		"node 198.51.100.2 layer=0 egress\n" +
		"node 203.0.113.1 layer=0 egress name=ixp-lan-a\n" +
		"node 203.0.113.2 layer=0 egress name=ixp-lan-a\n" +
		"edge 198.51.100.3 -> 198.51.100.1\n" +
		"edge 198.51.100.4 -> 198.51.100.3\n"
	if got := outsideTestAnswer(t, engine, "&mode=nexthop&format=text"); got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}

	drawing := outsideTestAnswer(t, engine, "&mode=nexthop")
	if strings.Contains(drawing, "outside") {
		t.Errorf("a drawing of addresses alone names the outside:\n%s", drawing)
	}
	if got := strings.Count(drawing, "<style>"); got != 1 {
		t.Errorf("the drawing holds %d style blocks, want the one of a router drawing", got)
	}
	for _, part := range []string{
		"<title>203.0.113.1 - ixp-lan-a</title>",
		"<title>203.0.113.2 - ixp-lan-a</title>",
	} {
		if !strings.Contains(drawing, part) {
			t.Errorf("the drawing does not hold %q:\n%s", part, drawing)
		}
	}
	if got := strings.Count(drawing, `<g class="node egress">`); got != 4 {
		t.Errorf("%d nodes are marked egress, want the four that announce their own address", got)
	}
}

// VALIDATES: the next-hop mode refuses what the trace mode refuses: a failed
// summary with the failure's status, and a summary that cannot say which peers
// are outside the AS (no peer-type, or an external peer with no AS).
// PREVENTS: a graph that silently draws the outside by address again because
// the summary could not be read.
func TestNextHopGraphModeRefusals(t *testing.T) {
	t.Run("the peer summary fails", func(t *testing.T) {
		engine := lgGraphTwoExits()
		engine.failing = cmdBGPOverview
		rec, _ := traceModeRequest(t, engine, "&mode=nexthop&format=text")
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502; body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), cmdBGPOverview+": "+failureEngineError) {
			t.Errorf("the refusal does not name the command and the failure: %s", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "mode nexthop") {
			t.Errorf("a graph was answered over a failed summary: %s", rec.Body.String())
		}
	})

	unreadable := map[string]struct {
		summary string
		says    string
	}{
		"a peer of the summary names no peer-type": {
			`{"router-id":"10.0.0.1","local-as":0,"peers":[{"address":"198.51.100.1","remote-as":65000}]}`,
			"peer-type",
		},
		"an external peer carries no AS": {
			`{"router-id":"10.0.0.1","local-as":0,"peers":[{"address":"203.0.113.1","peer-type":"external"}]}`,
			"no readable remote-as",
		},
		"an external peer carries no address": {
			`{"router-id":"10.0.0.1","local-as":0,"peers":[{"remote-as":65101,"peer-type":"external"}]}`,
			"is not an IP address",
		},
	}
	for name, tc := range unreadable {
		t.Run(name, func(t *testing.T) {
			engine := lgGraphTwoExits()
			engine.summary = tc.summary
			for _, format := range []string{"&format=text", ""} {
				rec, _ := traceModeRequest(t, engine, "&mode=nexthop"+format)
				if rec.Code != http.StatusServiceUnavailable {
					t.Fatalf("status = %d, want 503; body %s", rec.Code, rec.Body.String())
				}
				for _, part := range []string{"does not say which peers are outside the AS", tc.says, traceSummaryFix} {
					if !strings.Contains(rec.Body.String(), part) {
						t.Errorf("the refusal does not hold %q: %s", part, rec.Body.String())
					}
				}
			}
		})
	}
}

// VALIDATES: a trace that leaves the AS toward an external address draws no
// node for that address: the exit router links straight to the first AS of the
// exit route's path, and the address stays in the walk's `next-hop=` field and
// in its statement under the drawing.
// PREVENTS: an address outside the AS drawn as a node beside the routers.
func TestTraceExitTowardOutsideHasNoAddressNode(t *testing.T) {
	text := outsideTestAnswer(t, traceModeFourRouters(), "&mode=trace&format=text")
	for _, absent := range []string{"node neighbor", "-> 203.0.113.9", "edge 203.0.113.9"} {
		if strings.Contains(text, absent) {
			t.Errorf("the text form holds %q:\n%s", absent, text)
		}
	}
	for _, line := range []string{
		"node router 198.51.100.2 exit\n",
		"node AS65102\n",
		"edge 198.51.100.2 -> AS65102\n",
		"trace ipv4/unicast 198.18.0.0/24 198.51.100.2 routers=198.51.100.2 end=exit-external-address" +
			" next-hop=203.0.113.9 as-path=65102,64500" +
			" says: leaves the AS at 198.51.100.2 toward 203.0.113.9, the address of no router of the AS\n",
	} {
		if !strings.Contains(text, line) {
			t.Errorf("the text form does not hold %q:\n%s", line, text)
		}
	}

	drawing := outsideTestAnswer(t, traceModeFourRouters(), "&mode=trace")
	if strings.Contains(drawing, "<title>203.0.113.9") {
		t.Errorf("the address outside the AS is drawn as a node:\n%s", drawing)
	}
	statement := ">198.18.0.0/24 from 198.51.100.2: leaves the AS at 198.51.100.2 toward 203.0.113.9, the address of no router of the AS<"
	if !strings.Contains(drawing, statement) {
		t.Errorf("the drawing does not state the address the walk leaves toward:\n%s", drawing)
	}
	// The address is in that one line and nowhere else in the drawing.
	if got := strings.Count(drawing, "203.0.113.9"); got != 1 {
		t.Errorf("the drawing holds the outside address %d times, want once, in the walk's line", got)
	}
}

// VALIDATES: a prefix that originates inside the AS at a router whose next hop
// is no router of the AS keeps that next hop as a node, named from the named
// ranges, in the text form and the drawing. The AS path is empty, so there is
// no AS to draw the address as, and nothing is derived for it.
// PREVENTS: the outside rule removing an address that has no AS to stand for
// it, and a node that carries no name from the named ranges.
func TestTraceOriginInsideKeepsItsNextHopNode(t *testing.T) {
	engine := traceModeEngine{
		summary: traceTestSummaryText(traceTestRouter("198.51.100.1")),
		rows:    []string{traceTestRow("198.51.100.1", "203.0.113.9")},
	}

	want := "mode trace\n" +
		"node router 198.51.100.1 origin\n" +
		"node neighbor 203.0.113.9 name=ixp-lan-a\n" +
		"edge 198.51.100.1 -> 203.0.113.9\n" +
		"trace ipv4/unicast 198.18.0.0/24 198.51.100.1 routers=198.51.100.1 end=origin-inside" +
		" next-hop=203.0.113.9" +
		" says: ends at 198.51.100.1: its route has an empty AS path, so the prefix originates inside the AS at that router\n"
	if got := outsideTestAnswer(t, engine, "&mode=trace&format=text"); got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}

	drawing := outsideTestAnswer(t, engine, "&mode=trace")
	if !strings.Contains(drawing, "<title>203.0.113.9 - ixp-lan-a</title>") {
		t.Errorf("the drawing does not hold the next hop as a named node:\n%s", drawing)
	}
}

// VALIDATES: the layout still places a node of traceGraph.Neighbors in a
// column of its own, right of every router and left of the first AS: .3
// originates the prefix with a next hop that is no router, beside the walk of
// .4 through the exit .1 and its AS path. Every position is exact.
// PREVENTS: the neighbor column lost, or drawn over a router or an AS, now
// that only an origin inside the AS yields such a node
// (TestTraceLayoutFollowsTheWalk proved this order on an exit toward an
// external address, which draws no address node any more).
func TestTraceLayoutPlacesTheOriginNextHopBeforeTheASes(t *testing.T) {
	routers := traceTestSummary(t,
		traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.3"), traceTestRouter("198.51.100.4"),
	)
	g := buildTraceGraph(traceTestRoutes(t,
		traceTestRow("198.51.100.4", "198.51.100.1", 65101, 64500),
		traceTestRow("198.51.100.1", "198.51.100.1", 65101, 64500),
		traceTestRow("198.51.100.3", "203.0.113.9"),
	), routers)
	if want := netip.MustParseAddr("203.0.113.9"); len(g.Neighbors) != 1 || g.Neighbors[0] != want {
		t.Fatalf("neighbors = %v, want [%s]: the input no longer yields the node this test places", g.Neighbors, want)
	}
	layout := computeTraceLayout(g, traceNames{})

	// A router's label is 12 characters, the next hop's 11 and an AS's 7, at
	// 7 pixels each plus 16 of padding on both sides: 116, 109 and 81. Each
	// column starts one 120-pixel gap after the one before it, and a second
	// node of a column sits 60 pixels under the first.
	addresses := map[string]Position{
		"198.51.100.4": {X: 20, Y: 120, Width: 116, Height: 40},
		"198.51.100.3": {X: 20, Y: 20, Width: 116, Height: 40},
		"198.51.100.1": {X: 256, Y: 20, Width: 116, Height: 40},
		"203.0.113.9":  {X: 492, Y: 20, Width: 109, Height: 40},
	}
	for address, want := range addresses {
		got, placed := layout.Addresses[netip.MustParseAddr(address)]
		if !placed {
			t.Errorf("%s has no position", address)
			continue
		}
		if got != want {
			t.Errorf("%s is at %+v, want %+v", address, got, want)
		}
	}
	if len(layout.Addresses) != len(addresses) {
		t.Errorf("%d addresses are placed, want %d", len(layout.Addresses), len(addresses))
	}
	ases := map[uint32]Position{
		65101: {X: 721, Y: 20, Width: 81, Height: 40},
		64500: {X: 922, Y: 20, Width: 81, Height: 40},
	}
	for number, want := range ases {
		if got := layout.ASes[number]; got != want {
			t.Errorf("AS%d is at %+v, want %+v", number, got, want)
		}
	}

	// The order, read from the layout itself: the exit router, then the next
	// hop of the origin, then the first AS.
	exit := layout.Addresses[netip.MustParseAddr("198.51.100.1")]
	neighbor := layout.Addresses[netip.MustParseAddr("203.0.113.9")]
	first := layout.ASes[65101]
	if exit.X+exit.Width >= neighbor.X {
		t.Errorf("the exit router %+v is not left of the next hop %+v", exit, neighbor)
	}
	if neighbor.X+neighbor.Width >= first.X {
		t.Errorf("the next hop %+v is not left of the first AS %+v", neighbor, first)
	}
}
