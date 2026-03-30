// Design: docs/architecture/web-interface.md -- AS path topology graph handler
// Overview: server.go -- LG server and route registration
// Related: graph.go -- Graph data model and construction
// Related: layout.go -- Layout algorithm and SVG rendering

package lg

import (
	"fmt"
	"net/http"
)

// maxGraphNodes caps the number of nodes in the topology graph.
const maxGraphNodes = 100

// handleGraph renders an AS path topology graph as SVG for the given prefix.

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

	result := s.query(fmt.Sprintf("rib show prefix %s", prefix))
	zeData := parseJSON(result)
	routes := extractRoutes(zeData)

	// Build graph from AS paths.
	graph := buildGraph(routes)

	if len(graph.Nodes) == 0 {
		writeSVG(w, `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="50"><text x="10" y="30" font-family="monospace" font-size="14" fill="#666">No routes found</text></svg>`)
		return
	}

	if len(graph.Nodes) > maxGraphNodes {
		writeSVG(w, fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="400" height="50"><text x="10" y="30" font-family="monospace" font-size="14" fill="#666">Too many ASes (%d) for graph</text></svg>`, len(graph.Nodes)))
		return
	}

	// Assign layout positions.
	layout := computeLayout(graph)

	// Render SVG.
	svg := renderGraphSVG(graph, layout)

	writeSVG(w, svg)
}
