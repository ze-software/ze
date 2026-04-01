// Design: docs/architecture/web-interface.md -- Next-hop forwarding topology graph
// Related: graph.go -- AS path topology graph (external view)
// Related: layout_nexthop.go -- Layout and SVG rendering for next-hop graph
// Related: handler_graph.go -- Graph HTTP handler

package lg

import (
	"fmt"
	"sort"
	"strings"
)

// NextHopGraph represents an internal forwarding topology as routers and forwarding links.
type NextHopGraph struct {
	Nodes []NextHopNode
	Edges []NextHopEdge
}

// NextHopNode represents a router in the forwarding topology.
type NextHopNode struct {
	Address string // Router IP address.
	Name    string // Router name from decorator.
	Egress  bool   // True when this router learned the route externally (peer == next-hop).
	Layer   int    // Hop depth (0 = egress, increasing toward source).
}

// NextHopEdge represents a forwarding link from one router to another.
type NextHopEdge struct {
	From string // Source router (peer-address).
	To   string // Destination router (next-hop).
}

// buildNextHopGraph constructs a forwarding topology from routes.
// Each route needs "peer-address" and "next-hop" fields.
// A router is egress when peer-address == next-hop (learned externally, next-hop-self).
func buildNextHopGraph(routes []any) *NextHopGraph {
	type edgeKey struct{ from, to string }

	nodeSet := make(map[string]bool)
	egressSet := make(map[string]bool)
	edgeSet := make(map[edgeKey]bool)

	for _, r := range routes {
		route, ok := r.(map[string]any)
		if !ok {
			continue
		}

		peer := extractString(route, "peer-address")
		nhop := extractString(route, "next-hop")
		if peer == "" || nhop == "" {
			continue
		}

		nodeSet[peer] = true
		nodeSet[nhop] = true

		if peer == nhop {
			egressSet[peer] = true
		} else {
			edgeSet[edgeKey{peer, nhop}] = true
		}
	}

	// Compute layers via BFS from egress nodes.
	layerMap := make(map[string]int)
	for addr := range egressSet {
		layerMap[addr] = 0
	}

	reverse := make(map[string][]string)
	for key := range edgeSet {
		reverse[key.to] = append(reverse[key.to], key.from)
	}

	queue := make([]string, 0, len(egressSet))
	for addr := range egressSet {
		queue = append(queue, addr)
	}
	sort.Strings(queue) // deterministic BFS order

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		curLayer := layerMap[cur]
		sources := reverse[cur]
		sort.Strings(sources)
		for _, from := range sources {
			if _, assigned := layerMap[from]; !assigned {
				layerMap[from] = curLayer + 1
				queue = append(queue, from)
			}
		}
	}

	// Unconnected nodes default to layer 1.
	for addr := range nodeSet {
		if _, assigned := layerMap[addr]; !assigned {
			layerMap[addr] = 1
		}
	}

	// Build sorted node list (higher layer first, then by address).
	nodes := make([]NextHopNode, 0, len(nodeSet))
	for addr := range nodeSet {
		nodes = append(nodes, NextHopNode{
			Address: addr,
			Egress:  egressSet[addr],
			Layer:   layerMap[addr],
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Layer != nodes[j].Layer {
			return nodes[i].Layer > nodes[j].Layer
		}
		return nodes[i].Address < nodes[j].Address
	})

	// Build sorted edge list.
	edges := make([]NextHopEdge, 0, len(edgeSet))
	for key := range edgeSet {
		edges = append(edges, NextHopEdge{From: key.from, To: key.to})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	return &NextHopGraph{Nodes: nodes, Edges: edges}
}

// extractString extracts a string value from a route map.
func extractString(route map[string]any, key string) string {
	v, ok := route[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// renderNextHopGraphText returns a deterministic text representation for testing.
func renderNextHopGraphText(g *NextHopGraph) string {
	var sb strings.Builder
	sb.WriteString("mode nexthop\n")
	for _, n := range g.Nodes {
		fmt.Fprintf(&sb, "node %s layer=%d", n.Address, n.Layer)
		if n.Egress {
			sb.WriteString(" egress")
		}
		if n.Name != "" {
			fmt.Fprintf(&sb, " name=%s", n.Name)
		}
		sb.WriteByte('\n')
	}
	for _, e := range g.Edges {
		fmt.Fprintf(&sb, "edge %s -> %s\n", e.From, e.To)
	}
	return sb.String()
}
