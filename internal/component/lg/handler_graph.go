// Design: docs/architecture/web-interface.md -- Topology graph handler
// Overview: server.go -- LG server and route registration
// Related: graph.go -- AS path graph data model and construction
// Related: graph_nexthop.go -- Next-hop graph data model and construction
// Related: layout.go -- AS path graph layout and SVG rendering
// Related: layout_nexthop.go -- Next-hop graph layout and SVG rendering
// Related: graph_trace.go -- Trace graph data model, walk and text form
// Related: layout_trace.go -- Trace graph layout and SVG rendering

package lg

import (
	"fmt"
	"net/http"
	"net/netip"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// maxGraphNodes caps the number of nodes in the topology graph.
const maxGraphNodes = 100

// Graph mode and format constants.
const (
	graphModeASPath  = "aspath"
	graphModeNextHop = "nexthop"
	graphModeTrace   = "trace"
	graphFormatSVG   = "svg"
	graphFormatText  = "text"
)

// handleGraph renders a topology graph for the given prefix.
// Query parameters:
//   - prefix (required): a prefix with its length, or a bare address, which
//     draws every route whose prefix contains it.
//   - mode: "aspath" (default) for AS path topology, "nexthop" for internal
//     forwarding, "trace" for the trace from each router of the AS to the
//     origin AS.
//   - format: "svg" (default) for SVG image, "text" for deterministic text output.
func (s *LGServer) handleGraph(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		http.Error(w, "prefix query parameter required", http.StatusBadRequest)
		return
	}

	// The graph asks what the search table asked for the same field, so it
	// draws the routes the table lists (parsePrefixLookup).
	lookup, err := parsePrefixLookup(prefix)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = graphModeASPath
	}
	switch mode {
	case graphModeASPath, graphModeNextHop, graphModeTrace:
	default:
		// The mode is text from the URL: any other word is refused.
		http.Error(w, graphModeRefusal(mode).Error(), http.StatusBadRequest)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = graphFormatSVG
	}
	if format != graphFormatSVG && format != graphFormatText {
		http.Error(w, "format must be svg or text", http.StatusBadRequest)
		return
	}

	zeData, failure := s.ask(lookup.receivedCommand())
	if failure != nil {
		// A refusal is not an empty graph: "No routes found" over a failed
		// dispatch would tell the operator the prefix is absent.
		http.Error(w, engineRefusal(failure).Error(), failure.status)
		return
	}
	routes := extractRoutes(zeData)

	switch mode {
	case graphModeNextHop:
		s.handleNextHopGraph(w, routes, format)
	case graphModeTrace:
		s.handleTraceGraph(w, routes, format)
	default:
		// handleGraph refused every word but the three above.
		s.handleASPathGraph(w, routes, format)
	}
}

// handleASPathGraph renders the AS path topology graph.
func (s *LGServer) handleASPathGraph(w http.ResponseWriter, routes []any, format string) {
	graph := buildGraph(routes)

	if len(graph.Nodes) == 0 {
		writeEmpty(w, format, "No routes found")
		return
	}
	if len(graph.Nodes) > maxGraphNodes {
		writeEmpty(w, format, textbuf.StrIntStr("Too many ASes (", int64(len(graph.Nodes)), ") for graph"))
		return
	}

	s.decorateGraphNodes(graph)

	if format == graphFormatText {
		writeText(w, renderGraphText(graph))
		return
	}

	layout := computeLayout(graph)
	writeSVG(w, renderGraphSVG(graph, layout))
}

// handleNextHopGraph renders the next-hop forwarding topology graph.
func (s *LGServer) handleNextHopGraph(w http.ResponseWriter, routes []any, format string) {
	graph := buildNextHopGraph(routes)
	s.nameNextHopNodes(graph)

	if len(graph.Nodes) == 0 {
		writeEmpty(w, format, "No routes found")
		return
	}
	if len(graph.Nodes) > maxGraphNodes {
		writeEmpty(w, format, textbuf.StrIntStr("Too many routers (", int64(len(graph.Nodes)), ") for graph"))
		return
	}

	if format == graphFormatText {
		writeText(w, renderNextHopGraphText(graph))
		return
	}

	layout := computeNextHopLayout(graph)
	writeSVG(w, renderNextHopGraphSVG(graph, layout))
}

// handleTraceGraph renders the trace: from each router of the AS,
// through the routers its BGP next hops name, to the exit, then the AS path.
//
// routes is the answer of the one RIB query handleGraph made. The routers of
// the AS come from the peer summary the peers page already reads, so the mode
// adds no RIB query and no command.
func (s *LGServer) handleTraceGraph(w http.ResponseWriter, routes []any, format string) {
	summary, failure := s.ask(cmdBGPOverview)
	if failure != nil {
		// As for the RIB query: a refusal is not a graph with no router.
		http.Error(w, engineRefusal(failure).Error(), failure.status)
		return
	}

	routers, err := traceRoutersFrom(summary)
	if err != nil {
		// A summary that cannot tell a router of the AS from a neighbour would
		// draw a wrong exit, so the trace is refused and never drawn partly.
		// The refusal carries the whole reason to the reader, and names the
		// command that shows the summary, so this line is a record of the
		// refused request and no problem site of its own: it logs at Info.
		s.logger.Info("trace refused: the peer summary does not name the routers of the AS",
			"command", cmdBGPOverview, "reason", err.Error())
		http.Error(w, traceSummaryRefusal(err).Error(), http.StatusServiceUnavailable)
		return
	}

	graph := buildTraceGraph(routes, routers)
	if message, isUndrawn := graph.undrawn(); isUndrawn {
		writeEmpty(w, format, message)
		return
	}

	names := s.traceNames()
	if format == graphFormatText {
		writeText(w, renderTraceGraphText(graph, names))
		return
	}

	writeSVG(w, renderTraceGraphSVG(graph, computeTraceLayout(graph, names), names))
}

// graphModeRefusal is the operator error a `mode` that names no graph answers.
// It repeats the word, lists the three modes, and shows a valid one.
func graphModeRefusal(mode string) *opererr.Error {
	return opererr.New(
		"mode "+mode+" is not a graph mode",
		"the graph is drawn as "+graphModeASPath+" (the AS paths), "+graphModeNextHop+
			" (the routers and their egress) or "+graphModeTrace+
			" (from each router of the AS to the origin AS)",
		"mode="+graphModeASPath,
	)
}

// traceSummaryFix is the fix the refusal below names: the command whose
// answer the trace could not read, where each peer's type and address show.
const traceSummaryFix = "ze " + cmdBGPOverview

// traceSummaryRefusal is the operator error the trace mode answers when the
// peer summary does not say which peers are routers of the AS. err MUST be
// the error traceRoutersFrom returned. Its first line holds the whole reason,
// so the log holds no more than the reader was told.
func traceSummaryRefusal(err error) *opererr.Error {
	return opererr.New(
		cmdBGPOverview+": "+err.Error(),
		"the trace tells a router of the AS from an external neighbour by each peer's "+
			"peer-type and address, so without them it would state a wrong exit",
		traceSummaryFix,
	)
}

// traceNames answers the names the trace graph labels its nodes with: the
// operator's named range for a router or next-hop address, and the AS name
// decorator the AS path graph uses for an AS.
func (s *LGServer) traceNames() traceNames {
	return traceNames{
		address: func(address netip.Addr) string { return s.nextHopName(address.String()) },
		as:      func(number uint32) string { return s.resolveASN(textbuf.StringInt(int64(number))) },
	}
}

// decorateGraphNodes resolves ASN names for all graph nodes via the decorator.
func (s *LGServer) decorateGraphNodes(g *Graph) {
	for i := range g.Nodes {
		g.Nodes[i].Name = s.resolveASN(textbuf.StringInt(int64(g.Nodes[i].ASN)))
	}
}

// writeEmpty writes an empty-state message in the requested format.
func writeEmpty(w http.ResponseWriter, format, msg string) {
	if format == graphFormatText {
		var tb textbuf.Buffer

		writeText(w, tb.Str(msg).Byte('\n').String())

		return
	}

	svg, err := renderToString(graphEmpty(msg))
	if err != nil {
		lgLogger.Warn("empty graph render failed", "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)

		return
	}

	writeSVG(w, svg)
}

// writeText writes a plain text response.
func writeText(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := fmt.Fprint(w, text); err != nil {
		lgLogger.Debug("write text response failed", "error", err)
	}
}
