// Related: graph_trace_test.go -- the fixtures in the engine's two shapes, and the walk itself
// Related: handler_graph.go -- the mode these tests reach through GET /lg/graph
// Related: layout_trace.go -- the drawing these tests pin

package lg

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// traceModeEngine is the engine a trace mode test talks to: the text of its
// two answers, and the command it fails, if any.
type traceModeEngine struct {
	summary string
	rows    []string
	failing string
}

const traceModeRIBCommand = "show bgp rib received exact " + traceTestPrefix

// traceModeFourRouters is the network of TestTraceGraphTextIsOrdered: .4 is
// the client of .3, which points at the exit .1, and .2 leaves toward an
// external address.
func traceModeFourRouters() traceModeEngine {
	return traceModeEngine{
		summary: traceTestSummaryText(
			traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.2"),
			traceTestRouter("198.51.100.3"), traceTestRouter("198.51.100.4"),
			traceTestPeer{address: "203.0.113.9", remoteAS: 65102},
		),
		rows: []string{
			traceTestRow("198.51.100.4", "198.51.100.3", 65101, 64500),
			traceTestRow("198.51.100.3", "198.51.100.1", 65101, 64500),
			traceTestRow("198.51.100.1", "198.51.100.1", 65101, 64500),
			traceTestRow("198.51.100.2", "203.0.113.9", 65102, 64500),
		},
	}
}

// traceModeRequest asks GET /lg/graph of a server whose engine is the given
// one, with one named range and an AS name decorator, and answers the response
// and every command the server sent.
func traceModeRequest(t *testing.T, engine traceModeEngine, query string) (*httptest.ResponseRecorder, []string) {
	t.Helper()

	var mu sync.Mutex
	var commands []string
	dispatch := func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
		mu.Lock()
		commands = append(commands, cmd)
		mu.Unlock()
		if cmd == engine.failing {
			return nil, errTestEngineDown
		}
		switch cmd {
		case cmdBGPOverview:
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(engine.summary)), nil
		case traceModeRIBCommand:
			answer := `{"routes":[` + strings.Join(engine.rows, ",") + `]}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(answer)), nil
		}
		return nil, errUnknownCommand
	}

	srv, err := NewLGServer(LGConfig{
		ListenAddrs:   []string{"127.0.0.1:0"},
		Dispatch:      dispatch,
		NextHopRanges: map[netip.Prefix]string{netip.MustParsePrefix("203.0.113.0/24"): "ixp-lan-a"},
		DecorateASN: func(number string) string {
			if number == "65101" {
				return "Transit <A>"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatalf("new lg server: %v", err)
	}

	target := "/lg/graph?prefix=" + traceTestPrefix + query
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	rec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rec, req)

	mu.Lock()
	defer mu.Unlock()
	return rec, append([]string(nil), commands...)
}

// VALIDATES: AC-21, AC-23 at the handler -- `mode=trace&format=text` answers
// the trace graph's text form from ONE RIB query and the peer summary, with a
// node named from the named ranges (address) and from the AS name decorator.
// PREVENTS: a second RIB query, a new command, or a text form with no names.
func TestTraceGraphModeAnswersText(t *testing.T) {
	rec, commands := traceModeRequest(t, traceModeFourRouters(), "&mode=trace&format=text")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", contentType)
	}

	want := []string{traceModeRIBCommand, cmdBGPOverview}
	if strings.Join(commands, "|") != strings.Join(want, "|") {
		t.Fatalf("commands = %q, want %q", commands, want)
	}

	body := rec.Body.String()
	if !strings.HasPrefix(body, "mode trace\n") {
		t.Fatalf("the answer is not the trace text form:\n%s", body)
	}
	for _, line := range []string{
		"node router 198.51.100.1 exit\n",
		"node neighbor 203.0.113.9 name=ixp-lan-a\n",
		"node AS65101 name=Transit <A>\n",
		"edge 198.51.100.1 -> AS65101\n",
		"trace ipv4/unicast 198.18.0.0/24 198.51.100.4 routers=198.51.100.4,198.51.100.3,198.51.100.1" +
			" end=exit-own-address next-hop=198.51.100.1 as-path=65101,64500 says: ",
	} {
		if !strings.Contains(body, line) {
			t.Errorf("the text form does not hold %q:\n%s", line, body)
		}
	}
}

// VALIDATES: AC-21, AC-29 at the handler -- `mode=trace` answers an SVG that
// marks the exit as the next-hop graph marks egress, writes the line of every
// walk and the note of traceNote, and escapes an AS name.
// PREVENTS: a drawing that ends a walk silently, and a name written as markup.
func TestTraceGraphModeAnswersSVG(t *testing.T) {
	rec, _ := traceModeRequest(t, traceModeFourRouters(), "&mode=trace")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", contentType)
	}

	body := rec.Body.String()
	if !strings.HasPrefix(body, "<svg") || !strings.HasSuffix(body, "</svg>") {
		t.Fatalf("the answer is not one svg element:\n%s", body)
	}
	if got := strings.Count(body, `<g class="node egress">`); got != 2 {
		t.Errorf("%d nodes are marked egress, want the two exits", got)
	}
	for _, part := range []string{
		">198.18.0.0/24 from 198.51.100.4: leaves the AS at 198.51.100.1, which announces its own address as next hop<",
		">198.18.0.0/24 from 198.51.100.2: leaves the AS at 198.51.100.2 toward 203.0.113.9, the address of no router of the AS<",
		">" + template.HTMLEscapeString(traceNote) + "<",
		"<title>203.0.113.9 - ixp-lan-a</title>",
		"<title>AS65101 Transit &lt;A&gt;</title>",
	} {
		if !strings.Contains(body, part) {
			t.Errorf("the drawing does not hold %q:\n%s", part, body)
		}
	}
	if strings.Contains(body, "<A>") {
		t.Errorf("an AS name reached the drawing unescaped:\n%s", body)
	}
}

// VALIDATES: R-12 at the handler -- past 100 nodes the trace mode answers the
// cap's message in place of the graph, in both formats, and a prefix no router
// of the AS announced answers its own message.
// PREVENTS: an unreadable drawing, and an empty answer read as a graph.
func TestTraceGraphModeStatesWhatItDoesNotDraw(t *testing.T) {
	longPath := make([]uint32, 0, maxGraphNodes)
	for i := range maxGraphNodes {
		longPath = append(longPath, uint32(64600+i))
	}
	tooMany := traceModeEngine{
		summary: traceTestSummaryText(traceTestRouter("198.51.100.1")),
		rows:    []string{traceTestRow("198.51.100.1", "198.51.100.1", longPath...)},
	}
	const capMessage = "Too many nodes (101) for graph"

	rec, _ := traceModeRequest(t, tooMany, "&mode=trace&format=text")
	if rec.Code != http.StatusOK || rec.Body.String() != capMessage+"\n" {
		t.Errorf("text past the cap: status %d, body %q, want 200 and %q", rec.Code, rec.Body.String(), capMessage)
	}
	rec, _ = traceModeRequest(t, tooMany, "&mode=trace")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), ">"+capMessage+"<") {
		t.Errorf("svg past the cap: status %d, body %q, want 200 and %q", rec.Code, rec.Body.String(), capMessage)
	}

	// The only row comes from an external neighbor, so no walk starts.
	noRouter := traceModeEngine{
		summary: traceTestSummaryText(
			traceTestRouter("198.51.100.1"), traceTestPeer{address: "203.0.113.9", remoteAS: 65102},
		),
		rows: []string{traceTestRow("203.0.113.9", "203.0.113.9", 65102)},
	}
	rec, _ = traceModeRequest(t, noRouter, "&mode=trace&format=text")
	if rec.Code != http.StatusOK || rec.Body.String() != "No trace: no route the query answered came from a router of the AS "+
		"(a peer with an internal session). Peers: 2, routers of the AS: 1, routes answered: 1.\n" {
		t.Errorf("no router holds a route: status %d, body %q", rec.Code, rec.Body.String())
	}
}

// VALIDATES: the trace mode answers an engine failure as the two other modes
// do (engineRefusal, with the failure's status), refuses a summary that cannot
// say which peers are routers of the AS, and the mode word is checked.
// PREVENTS: "No route found" over a failed dispatch, and a trace drawn from a
// summary that would take a router for a neighbor.
func TestTraceGraphModeRefusals(t *testing.T) {
	t.Run("the rib query fails", func(t *testing.T) {
		engine := traceModeFourRouters()
		engine.failing = traceModeRIBCommand
		rec, commands := traceModeRequest(t, engine, "&mode=trace&format=text")
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502; body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), traceModeRIBCommand+": "+failureEngineError) {
			t.Errorf("the refusal does not name the command and the failure: %s", rec.Body.String())
		}
		if len(commands) != 1 {
			t.Errorf("commands = %q, want the rib query alone", commands)
		}
	})

	t.Run("the peer summary fails", func(t *testing.T) {
		engine := traceModeFourRouters()
		engine.failing = cmdBGPOverview
		rec, _ := traceModeRequest(t, engine, "&mode=trace&format=text")
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502; body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), cmdBGPOverview+": "+failureEngineError) {
			t.Errorf("the refusal does not name the command and the failure: %s", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "mode trace") {
			t.Errorf("a graph was answered over a failed summary: %s", rec.Body.String())
		}
	})

	t.Run("a peer of the summary names no peer-type", func(t *testing.T) {
		engine := traceModeFourRouters()
		engine.summary = `{"router-id":"10.0.0.1","local-as":0,"peers":[{"address":"198.51.100.1","remote-as":65000}]}`
		rec, _ := traceModeRequest(t, engine, "&mode=trace&format=text")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503; body %s", rec.Code, rec.Body.String())
		}
		for _, part := range []string{"does not say which peers are routers of the AS", "peer-type", traceSummaryFix} {
			if !strings.Contains(rec.Body.String(), part) {
				t.Errorf("the refusal does not hold %q: %s", part, rec.Body.String())
			}
		}
	})

	t.Run("an unknown mode", func(t *testing.T) {
		rec, commands := traceModeRequest(t, traceModeFourRouters(), "&mode=joint")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
		}
		for _, part := range []string{"mode joint is not a graph mode", "aspath", "nexthop", "trace", "mode=aspath"} {
			if !strings.Contains(rec.Body.String(), part) {
				t.Errorf("the refusal does not hold %q: %s", part, rec.Body.String())
			}
		}
		if len(commands) != 0 {
			t.Errorf("an unknown mode reached the engine: %q", commands)
		}
	})
}

// VALIDATES: the layout places the routers left to right in walk order (the
// client, the reflector, the exit), a second starting router under the first,
// then the external next hop, then the AS chain with the origin AS last.
// PREVENTS: a drawing whose columns do not read as the direction of the walk.
func TestTraceLayoutFollowsTheWalk(t *testing.T) {
	engine := traceModeFourRouters()
	routers, err := traceRoutersFrom(parseJSON(engine.summary))
	if err != nil {
		t.Fatalf("traceRoutersFrom: %v", err)
	}
	g := buildTraceGraph(traceTestRoutes(t, engine.rows...), routers)
	layout := computeTraceLayout(g, traceNames{})

	at := func(address string) Position {
		position, placed := layout.Addresses[netip.MustParseAddr(address)]
		if !placed {
			t.Fatalf("%s has no position", address)
		}
		return position
	}
	client, reflector, exit := at("198.51.100.4"), at("198.51.100.3"), at("198.51.100.1")
	second, neighbor := at("198.51.100.2"), at("203.0.113.9")
	first, other, origin := layout.ASes[65101], layout.ASes[65102], layout.ASes[64500]

	columns := []int{client.X, reflector.X, exit.X, neighbor.X, first.X, origin.X}
	for i := 1; i < len(columns); i++ {
		if columns[i] <= columns[i-1] {
			t.Fatalf("column %d starts at %d, not right of %d: %v", i, columns[i], columns[i-1], columns)
		}
	}
	if second.X != client.X || second.Y == client.Y {
		t.Errorf("the second starting router is at %+v, want the column of %+v and another row", second, client)
	}
	if other.X != first.X || other.Y == first.Y {
		t.Errorf("the second first AS is at %+v, want the column of %+v and another row", other, first)
	}
	if len(layout.Ends) != len(g.Walks) {
		t.Errorf("%d lines for %d walks", len(layout.Ends), len(g.Walks))
	}
}

// VALIDATES: the whole drawing of the smallest trace graph, byte for byte:
// one exit router, one AS, the edge between them, the line of the walk and the
// note, at the positions the layout computed.
// PREVENTS: a layout regression nobody sees (R-13), in the one drawing no
// saved file compares.
func TestTraceGraphSVGOfOneExit(t *testing.T) {
	routers := traceTestSummary(t, traceTestRouter("198.51.100.1"))
	g := buildTraceGraph(traceTestRoutes(t, traceTestRow("198.51.100.1", "198.51.100.1", 65101)), routers)
	layout := computeTraceLayout(g, traceNames{})

	// The router's label is 12 characters and the AS's 7, at 7 pixels each
	// plus 16 of padding on both sides: 116 and 81. The second column starts
	// one 120-pixel gap after the first.
	if got, want := layout.Addresses[netip.MustParseAddr("198.51.100.1")], (Position{X: 20, Y: 20, Width: 116, Height: 40}); got != want {
		t.Errorf("router at %+v, want %+v", got, want)
	}
	if got, want := layout.ASes[65101], (Position{X: 256, Y: 20, Width: 81, Height: 40}); got != want {
		t.Errorf("AS at %+v, want %+v", got, want)
	}
	// The note is the longest line, so it decides the width.
	width := strconv.Itoa(40 + len(traceNote)*7)
	if got := strconv.Itoa(layout.Width); got != width {
		t.Errorf("width = %s, want %s", got, width)
	}
	if layout.Height != 136 {
		t.Errorf("height = %d, want 136", layout.Height)
	}

	const font = ` font-family="monospace" font-size="12" fill="currentColor">`
	want := `<svg xmlns="http://www.w3.org/2000/svg" width="` + width + `" height="136" viewBox="0 0 ` + width + ` 136">` + "\n" +
		routerGraphStyle + "\n" +
		`<g class="edge"><line x1="136" y1="40" x2="256" y2="40"/><polygon points="256,40 248,36 248,44"/></g>` + "\n" +
		`<g class="node egress"><rect x="20" y="20" width="116" height="40"/>` +
		`<text x="36" y="44" class="label">198.51.100.1</text><title>198.51.100.1</title></g>` + "\n" +
		`<g class="node"><rect x="256" y="20" width="81" height="40"/>` +
		`<text x="272" y="44" class="label">AS65101</text><title>AS65101</title></g>` + "\n" +
		`<g class="trace-ends">` + "\n" +
		`<text x="20" y="98"` + font +
		`198.18.0.0/24 from 198.51.100.1: leaves the AS at 198.51.100.1, which announces its own address as next hop</text>` + "\n" +
		`</g>` + "\n" + `<g class="trace-note">` + "\n" +
		`<text x="20" y="116"` + font + template.HTMLEscapeString(traceNote) + `</text>` + "\n" +
		`</g>` + "\n" + `</svg>`

	if got := renderTraceGraphSVG(g, layout, traceNames{}); got != want {
		t.Errorf("svg =\n%s\nwant\n%s", got, want)
	}
}

// VALIDATES: every end of a walk that draws no exit still has its line under
// the drawing: a loop, a named router with no route, and an origin inside.
// PREVENTS: a walk that ends silently on the page.
func TestTraceGraphSVGStatesEveryEnd(t *testing.T) {
	routers := traceTestSummary(t,
		traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.2"),
		traceTestRouter("198.51.100.3"), traceTestRouter("198.51.100.4"),
		traceTestRouter("198.51.100.5"),
	)
	g := buildTraceGraph(traceTestRoutes(t,
		traceTestRow("198.51.100.1", "198.51.100.2", 65101),
		traceTestRow("198.51.100.2", "198.51.100.1", 65101),
		traceTestRow("198.51.100.3", "198.51.100.4", 65101),
		traceTestRow("198.51.100.5", "198.51.100.5"),
	), routers)
	svg := renderTraceGraphSVG(g, computeTraceLayout(g, traceNames{}), traceNames{})

	for _, trace := range g.Walks {
		line := ">" + traceTestPrefix + " from " + trace.Routers[0].String() + ": " +
			template.HTMLEscapeString(trace.statement()) + "<"
		if !strings.Contains(svg, line) {
			t.Errorf("the drawing does not state the %s end of the walk from %s:\n%s", trace.End, trace.Routers[0], svg)
		}
	}
	ends := make(map[traceEnd]bool)
	for _, trace := range g.Walks {
		ends[trace.End] = true
	}
	for _, end := range []traceEnd{traceEndLoop, traceEndNoRoute, traceEndOriginInside} {
		if !ends[end] {
			t.Errorf("the fixture produced no %s end, so the test does not cover it", end)
		}
	}
}

// VALIDATES: the trace mode is chosen where the two other modes are: a third
// button of the same class, which fetches `mode=trace` into the graph
// container and carries no inline handler.
// PREVENTS: a mode the handler serves and no page reaches.
func TestGraphModeControlOffersTheTrace(t *testing.T) {
	panel, err := renderToString(routeResults(searchView{
		Prefix: "203.0.113.0/24",
		Routes: lgRouteRows(), Count: 2,
	}))
	if err != nil {
		t.Fatalf("render routeResults: %v", err)
	}

	if got := strings.Count(panel, `class="`+graphModeClass+`"`); got != 3 {
		t.Errorf("%d graph mode buttons, want 3", got)
	}
	for _, part := range []string{
		`data-mode="` + graphModeTrace + `"`,
		`mode=` + graphModeTrace + `"`,
		`hx-target="#graph-container"`,
	} {
		if !strings.Contains(panel, part) {
			t.Errorf("the result panel does not hold %q:\n%s", part, panel)
		}
	}
}

// VALIDATES: AC-29 -- the help page carries the note of traceNote, in
// the panel its Trace Graph card opens (assets/theme.js reads data-help and
// shows the template `help-<key>`).
// PREVENTS: a second wording of the note, and a card that opens nothing.
func TestHelpCarriesTheTraceNote(t *testing.T) {
	page, err := renderToString(helpPage())
	if err != nil {
		t.Fatalf("render helpPage: %v", err)
	}
	for _, part := range []string{
		`data-help="trace"`,
		`<template id="help-trace">`,
		template.HTMLEscapeString(traceNote),
	} {
		if !strings.Contains(page, part) {
			t.Errorf("the help page does not hold %q", part)
		}
	}
}
