// Design: docs/architecture/web-interface.md -- AS path topology graph data model
// Related: graph_nexthop.go -- Next-hop forwarding topology graph (internal view)
// Related: layout.go -- Layout algorithm and SVG rendering
// Related: handler_graph.go -- Graph HTTP handler

package lg

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/graph"
)

// Graph is a type alias for the shared graph data model.
type Graph = graph.Graph

// GraphNode is a type alias for the shared graph node type.
type GraphNode = graph.Node

// GraphEdge is a type alias for the shared graph edge type.
type GraphEdge = graph.Edge

// buildGraph constructs a topology graph from a set of routes.
// Each route must have an "as-path" field (array of numbers).
// AS prepending (repeated ASN) is collapsed to a single node.
// This is an LG-specific wrapper that parses JSON route maps.
func buildGraph(routes []any) *Graph {
	var paths [][]uint32
	for _, r := range routes {
		route, ok := r.(map[string]any)
		if !ok {
			continue
		}
		asPath := extractASPath(route)
		if len(asPath) == 0 {
			continue
		}
		paths = append(paths, asPath)
	}
	return graph.BuildGraphFromPaths(paths)
}

// extractASPath extracts the AS path from a route map as a slice of uint32.
func extractASPath(route map[string]any) []uint32 {
	raw, ok := route["as-path"]
	if !ok {
		return nil
	}

	arr, ok := raw.([]any)
	if !ok {
		return nil
	}

	const maxASN = 4294967295
	var path []uint32
	for _, v := range arr {
		var asn int64
		valid := true
		switch n := v.(type) {
		case float64:
			if n < 0 || n > maxASN {
				valid = false
			} else {
				asn = int64(n)
			}
		case int:
			asn = int64(n)
		case int64:
			asn = n
		default: // unknown type (e.g., string) -- skip
			valid = false
		}
		if valid && asn >= 0 && asn <= maxASN {
			path = append(path, uint32(asn))
		}
	}

	return path
}

// renderGraphText returns a deterministic text representation for testing.
func renderGraphText(g *Graph) string {
	var sb textbuf.Buffer
	sb.Str("mode aspath\n")
	var tb textbuf.Buffer
	for _, n := range g.Nodes {
		tb.Reset().Str("AS").Int(int64(n.ASN))
		if n.Name != "" {
			tb.Byte(' ').Str(n.Name)
		}
		label := tb.String()
		fmt.Fprintf(&sb, "node %s layer=%d\n", label, n.Layer) //nolint:errcheck // report output
	}
	for _, e := range g.Edges {
		fmt.Fprintf(&sb, "edge AS%d -> AS%d\n", e.FromASN, e.ToASN) //nolint:errcheck // report output
	}
	return sb.String()
}
