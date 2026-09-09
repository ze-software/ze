// Design: docs/architecture/web-interface.md -- AS path topology graph data model
// Related: graph_nexthop.go -- Next-hop forwarding topology graph (internal view)
// Related: layout.go -- Layout algorithm and SVG rendering
// Related: handler_graph.go -- Graph HTTP handler

package lg

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/bgp/asn"

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
// Each route must have an "as-path" field, whose members are JSON numbers
// under asplain and quoted strings under a dotted notation.
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

	// asn.FromJSON reads every shape an AS number arrives in, including the
	// STRING a dotted notation writes (bgp/as-notation). This walk used to skip
	// a string. A graph built under asdot would then have dropped every AS
	// number of 65536 or more, and drawn a path that does not exist.
	var path []uint32
	for _, v := range arr {
		if number, ok := asn.FromJSON(v); ok {
			path = append(path, number)
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
		// A node label is read by a human, so it carries the configured
		// notation. The lookup key resolveASN builds is decimal (server.go).
		tb.Reset().Str("AS").Str(asn.Text(n.ASN, asn.Configured()))
		if n.Name != "" {
			tb.Byte(' ').Str(n.Name)
		}
		label := tb.String()
		fmt.Fprintf(&sb, "node %s layer=%d\n", label, n.Layer) //nolint:errcheck // report output
	}
	for _, e := range g.Edges {
		fmt.Fprintf(&sb, "edge AS%s -> AS%s\n", //nolint:errcheck // report output
			asn.Text(e.FromASN, asn.Configured()), asn.Text(e.ToASN, asn.Configured()))
	}
	return sb.String()
}
