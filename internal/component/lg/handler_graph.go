// Design: docs/architecture/web-interface.md -- Topology graph handler
// Overview: server.go -- LG server and route registration
// Related: graph.go -- AS path graph data model and construction
// Related: graph_nexthop.go -- Next-hop graph data model and construction
// Related: layout.go -- AS path graph layout and SVG rendering
// Related: layout_nexthop.go -- Next-hop graph layout and SVG rendering

package lg

import (
	"fmt"
	"net/http"
)

// maxGraphNodes caps the number of nodes in the topology graph.
const maxGraphNodes = 100

// Graph mode and format constants.
const (
	graphModeASPath  = "aspath"
	graphModeNextHop = "nexthop"
	graphFormatSVG   = "svg"
	graphFormatText  = "text"
)

// handleGraph renders a topology graph for the given prefix.
// Query parameters:
//   - prefix (required): CIDR prefix to look up.
//   - mode: "aspath" (default) for AS path topology, "nexthop" for internal forwarding.
//   - format: "svg" (default) for SVG image, "text" for deterministic text output.
func (s *LGServer) handleGraph(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		http.Error(w, "prefix query parameter required", http.StatusBadRequest)
		return
	}

	if !isValidPrefix(prefix) {
		http.Error(w, "invalid prefix", http.StatusBadRequest)
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = graphModeASPath
	}
	if mode != graphModeASPath && mode != graphModeNextHop {
		http.Error(w, "mode must be aspath or nexthop", http.StatusBadRequest)
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

	result := s.query(fmt.Sprintf("rib show prefix %s", prefix))
	zeData := parseJSON(result)
	routes := extractRoutes(zeData)

	if mode == graphModeNextHop {
		s.handleNextHopGraph(w, routes, format)
		return
	}
	s.handleASPathGraph(w, routes, format)
}

// handleASPathGraph renders the AS path topology graph.
func (s *LGServer) handleASPathGraph(w http.ResponseWriter, routes []any, format string) {
	graph := buildGraph(routes)

	if len(graph.Nodes) == 0 {
		writeEmpty(w, format, "No routes found")
		return
	}
	if len(graph.Nodes) > maxGraphNodes {
		writeEmpty(w, format, fmt.Sprintf("Too many ASes (%d) for graph", len(graph.Nodes)))
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

	if len(graph.Nodes) == 0 {
		writeEmpty(w, format, "No routes found")
		return
	}
	if len(graph.Nodes) > maxGraphNodes {
		writeEmpty(w, format, fmt.Sprintf("Too many routers (%d) for graph", len(graph.Nodes)))
		return
	}

	if format == graphFormatText {
		writeText(w, renderNextHopGraphText(graph))
		return
	}

	layout := computeNextHopLayout(graph)
	writeSVG(w, renderNextHopGraphSVG(graph, layout))
}

// decorateGraphNodes resolves ASN names for all graph nodes via the decorator.
func (s *LGServer) decorateGraphNodes(g *Graph) {
	for i := range g.Nodes {
		g.Nodes[i].Name = s.resolveASN(fmt.Sprintf("%d", g.Nodes[i].ASN))
	}
}

// writeEmpty writes an empty-state message in the requested format.
func writeEmpty(w http.ResponseWriter, format, msg string) {
	if format == graphFormatText {
		writeText(w, msg+"\n")
		return
	}
	writeSVG(w, fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="400" height="50">`+
			`<text x="10" y="30" font-family="monospace" font-size="14" fill="currentColor">%s</text></svg>`, msg))
}

// writeText writes a plain text response.
func writeText(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := fmt.Fprint(w, text); err != nil {
		lgLogger.Debug("write text response failed", "error", err)
	}
}
