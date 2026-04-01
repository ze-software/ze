// Design: docs/architecture/web-interface.md -- AS path topology graph data model
// Related: graph_nexthop.go -- Next-hop forwarding topology graph (internal view)
// Related: layout.go -- Layout algorithm and SVG rendering
// Related: handler_graph.go -- Graph HTTP handler

package lg

import (
	"fmt"
	"sort"
	"strings"
)

// Graph represents an AS path topology as nodes (ASes) and edges (peering links).
type Graph struct {
	Nodes []GraphNode
	Edges []GraphEdge
}

// GraphNode represents an autonomous system in the topology graph.
type GraphNode struct {
	ASN   uint32
	Name  string // Organization name from decorator.
	Layer int    // Hop depth (0 = origin, increasing toward source).
}

// GraphEdge represents a peering link between two autonomous systems.
type GraphEdge struct {
	FromASN uint32
	ToASN   uint32
}

// buildGraph constructs a topology graph from a set of routes.
// Each route must have an "as-path" field (array of numbers).
// AS prepending (repeated ASN) is collapsed to a single node.
func buildGraph(routes []any) *Graph {
	type edgeKey struct{ from, to uint32 }
	nodeSet := make(map[uint32]bool)
	edgeSet := make(map[edgeKey]bool)
	layerMap := make(map[uint32]int) // ASN -> max layer

	for _, r := range routes {
		route, ok := r.(map[string]any)
		if !ok {
			continue
		}

		asPath := extractASPath(route)
		if len(asPath) == 0 {
			continue
		}

		// Deduplicate consecutive ASes (AS prepending).
		deduped := deduplicateASPath(asPath)

		// Add nodes and compute layers.
		// Layer 0 = origin (last AS in path), increasing toward source.
		for i, asn := range deduped {
			nodeSet[asn] = true
			layer := len(deduped) - 1 - i
			if layer > layerMap[asn] {
				layerMap[asn] = layer
			}
		}

		// Add edges.
		for i := range len(deduped) - 1 {
			edgeSet[edgeKey{deduped[i], deduped[i+1]}] = true
		}
	}

	// Build sorted node list.
	nodes := make([]GraphNode, 0, len(nodeSet))
	for asn := range nodeSet {
		nodes = append(nodes, GraphNode{
			ASN:   asn,
			Layer: layerMap[asn],
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Layer != nodes[j].Layer {
			return nodes[i].Layer > nodes[j].Layer // Higher layer (source) first.
		}
		return nodes[i].ASN < nodes[j].ASN
	})

	// Build edge list.
	edges := make([]GraphEdge, 0, len(edgeSet))
	for key := range edgeSet {
		edges = append(edges, GraphEdge{FromASN: key.from, ToASN: key.to})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromASN != edges[j].FromASN {
			return edges[i].FromASN < edges[j].FromASN
		}
		return edges[i].ToASN < edges[j].ToASN
	})

	return &Graph{Nodes: nodes, Edges: edges}
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

// deduplicateASPath removes consecutive duplicate ASNs (AS prepending).
func deduplicateASPath(path []uint32) []uint32 {
	if len(path) == 0 {
		return nil
	}

	result := []uint32{path[0]}
	for i := 1; i < len(path); i++ {
		if path[i] != path[i-1] {
			result = append(result, path[i])
		}
	}

	return result
}

// renderGraphText returns a deterministic text representation for testing.
func renderGraphText(g *Graph) string {
	var sb strings.Builder
	sb.WriteString("mode aspath\n")
	for _, n := range g.Nodes {
		label := fmt.Sprintf("AS%d", n.ASN)
		if n.Name != "" {
			label += " " + n.Name
		}
		fmt.Fprintf(&sb, "node %s layer=%d\n", label, n.Layer)
	}
	for _, e := range g.Edges {
		fmt.Fprintf(&sb, "edge AS%d -> AS%d\n", e.FromASN, e.ToASN)
	}
	return sb.String()
}
