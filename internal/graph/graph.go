// Design: docs/architecture/web-interface.md -- shared AS path topology graph data model
// Related: layout.go -- Layered graph layout algorithm (parameterized)
// Related: text.go -- Unicode box-drawing text renderer

package graph

import "sort"

// Graph represents an AS path topology as nodes (ASes) and edges (peering links).
type Graph struct {
	Nodes []Node
	Edges []Edge
}

// Node represents an autonomous system in the topology graph.
type Node struct {
	ASN   uint32
	Name  string // Organization name (populated by decorator, empty if unavailable).
	Layer int    // Hop depth (0 = origin, increasing toward source).
}

// Edge represents a peering link between two autonomous systems.
type Edge struct {
	FromASN uint32
	ToASN   uint32
}

// MaxNodes caps the number of nodes in a topology graph to prevent
// terminal flooding or excessive SVG rendering.
const MaxNodes = 100

// DeduplicateASPath removes consecutive duplicate ASNs (AS prepending collapse).
func DeduplicateASPath(path []uint32) []uint32 {
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

// BuildGraphFromPaths constructs a topology graph from a set of AS paths.
// Each path is a slice of uint32 ASNs from source to origin (left to right).
// Consecutive duplicate ASNs (AS prepending) are collapsed automatically.
func BuildGraphFromPaths(paths [][]uint32) *Graph {
	type edgeKey struct{ from, to uint32 }
	nodeSet := make(map[uint32]bool)
	edgeSet := make(map[edgeKey]bool)
	layerMap := make(map[uint32]int) // ASN -> max layer

	for _, path := range paths {
		deduped := DeduplicateASPath(path)
		if len(deduped) == 0 {
			continue
		}

		// Layer 0 = origin (last AS in path), increasing toward source.
		for i, asn := range deduped {
			nodeSet[asn] = true
			layer := len(deduped) - 1 - i
			if layer > layerMap[asn] {
				layerMap[asn] = layer
			}
		}

		// Add edges between consecutive ASNs.
		for i := range len(deduped) - 1 {
			edgeSet[edgeKey{deduped[i], deduped[i+1]}] = true
		}
	}

	// Build sorted node list (higher layer first, then by ASN).
	nodes := make([]Node, 0, len(nodeSet))
	for asn := range nodeSet {
		nodes = append(nodes, Node{
			ASN:   asn,
			Layer: layerMap[asn],
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Layer != nodes[j].Layer {
			return nodes[i].Layer > nodes[j].Layer
		}
		return nodes[i].ASN < nodes[j].ASN
	})

	// Build sorted edge list.
	edges := make([]Edge, 0, len(edgeSet))
	for key := range edgeSet {
		edges = append(edges, Edge{FromASN: key.from, ToASN: key.to})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromASN != edges[j].FromASN {
			return edges[i].FromASN < edges[j].FromASN
		}
		return edges[i].ToASN < edges[j].ToASN
	})

	return &Graph{Nodes: nodes, Edges: edges}
}

// FormatNodeLabel returns the display label for a graph node.
func FormatNodeLabel(n Node) string {
	if n.Name != "" {
		return "AS" + uitoa(n.ASN) + " " + n.Name
	}
	return "AS" + uitoa(n.ASN)
}

// uitoa converts a uint32 to its decimal string representation.
func uitoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
