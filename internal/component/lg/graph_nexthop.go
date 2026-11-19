// Design: docs/architecture/web-interface.md -- Next-hop forwarding topology graph
// Related: graph.go -- AS path topology graph (external view)
// Related: layout_nexthop.go -- Layout and SVG rendering for next-hop graph
// Related: handler_graph.go -- Graph HTTP handler

package lg

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sort"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// fieldRemoteAS is the key of the peer summary a peer's AS is read from
// (bgp/plugins/cmd/peer/summary.go, handleBgpSummary).
const fieldRemoteAS = "remote-as"

var errNextHopSummary = errors.New("lg next-hop graph: the peer summary does not say which peers are outside the AS")

// nextHopOutside is the set of peers outside the AS: the peers whose session
// with Ze is external, each known by its session address and its AS. The graph
// draws such a peer as its AS and never as its address. Only
// nextHopOutsideFrom builds one from a summary. The zero value holds no peer,
// so every address stays an address.
type nextHopOutside struct {
	asByAddress map[netip.Addr]uint32
}

// nextHopOutsideFrom reads the peers outside the AS out of the `show bgp`
// answer, as traceRoutersFrom reads the routers of the AS: by each row's
// `peer-type`, which the engine derives from the session's own local AS.
//
// A summary that cannot say so is refused: with a peer whose type, address or
// present `remote-as` cannot be read, the graph would draw an outside peer as
// an address, as if it were a router of the AS. An external peer with no
// `remote-as` at all is a dynamic peer whose OPEN has not arrived: its AS is
// not yet known, it has sent no route, and it is left out of the set.
func nextHopOutsideFrom(summary map[string]any) (nextHopOutside, error) {
	asByAddress := make(map[netip.Addr]uint32)
	for _, entry := range summaryPeers(summary) {
		peer, isRow := entry.(map[string]any)
		if !isRow {
			return nextHopOutside{}, fmt.Errorf("%w: a peer entry is not a row", errNextHopSummary)
		}

		// The set is open: the value arrives from the engine's answer.
		switch peerType := getStr(peer, fieldPeerType); peerType {
		case peerTypeInternal:
			// A router of the AS is drawn by its address, as a row names it.
			continue
		case peerTypeExternal:
		default:
			return nextHopOutside{}, fmt.Errorf("%w: peer %q carries %s %q, which is neither %s nor %s",
				errNextHopSummary, peerAddress(peer), fieldPeerType, peerType,
				peerTypeInternal, peerTypeExternal)
		}

		address, err := netip.ParseAddr(peerAddress(peer))
		if err != nil {
			return nextHopOutside{}, fmt.Errorf("%w: peer address %q is not an IP address",
				errNextHopSummary, peerAddress(peer))
		}
		// AS not yet known: a dynamic peer is in the summary from its TCP
		// accept, typed external, and the engine leaves `remote-as` out until
		// its OPEN names the AS (setRemoteAS, bgp/plugins/cmd/peer/peer.go).
		// Such a peer has sent no route, so it is left out of the outside set:
		// an address a row names that is no known outside peer stays an
		// address, and nothing is derived for it. A `remote-as` that is
		// present but unreadable, `null` included, is still refused below.
		if _, hasAS := peer[fieldRemoteAS]; !hasAS {
			continue
		}
		number, isReadable := asn.FromJSON(getVal(peer, fieldRemoteAS))
		if !isReadable {
			return nextHopOutside{}, fmt.Errorf("%w: external peer %q carries no readable %s",
				errNextHopSummary, peerAddress(peer), fieldRemoteAS)
		}
		asByAddress[address.Unmap()] = number
	}
	return nextHopOutside{asByAddress: asByAddress}, nil
}

// asOf answers the AS of the outside peer whose session address is the given
// text, and whether there is one. A text that is no address, and the address
// of no external peer, are both no outside peer: the node stays what the row
// wrote.
func (o nextHopOutside) asOf(address string) (number uint32, isOutside bool) {
	parsed, err := netip.ParseAddr(address)
	if err != nil {
		return 0, false
	}
	number, isOutside = o.asByAddress[parsed.Unmap()]
	return number, isOutside
}

// nextHopGraph represents an internal forwarding topology as routers and forwarding links.
type nextHopGraph struct {
	Nodes []NextHopNode
	Edges []NextHopEdge
}

// NextHopNode represents a node of the forwarding topology: a router by its
// address, or an AS outside the local one.
type NextHopNode struct {
	// Address identifies the node, and an edge names it: the IP address a row
	// wrote, or `AS<n>` in the configured notation when Outside.
	Address string
	// Name is the operator's name for the range Address is in
	// (nexthop_ranges.go), or the AS name when Outside. "" for none.
	Name string
	// Egress is true when this router learned the route externally (peer == next-hop).
	Egress bool
	// Outside is true when the node is an AS outside the local one: every
	// external peer of that AS is this one node. It is never Egress, which is
	// a statement about a router of the AS.
	Outside bool
	AS      uint32 // The AS number. Read only when Outside.
	Layer   int    // Hop depth (0 = egress or outside, increasing toward source).
}

// NextHopEdge represents a forwarding link from one node to another.
type NextHopEdge struct {
	From string // Source node (peer-address, or the AS of an outside peer).
	To   string // Destination node (next-hop, or the AS of an outside peer).
}

// buildNextHopGraph constructs a forwarding topology from routes.
// Each route needs "peer-address" and "next-hop" fields.
// A router is egress when peer-address == next-hop (learned externally, next-hop-self).
//
// An address that is the session address of a peer outside the AS is drawn as
// that AS: two external peers of one AS are one node and their edges merge.
// Every other address is a node of its own, as the row wrote it: a router of
// the AS, and an address that is no peer of Ze, about which nothing is derived.
func buildNextHopGraph(routes []any, outside nextHopOutside) *nextHopGraph {
	type edgeKey struct{ from, to string }

	nodeSet := make(map[string]bool)
	egressSet := make(map[string]bool)
	edgeSet := make(map[edgeKey]bool)
	outsideAS := make(map[string]uint32)

	// nodeOf answers the node an address is drawn as.
	nodeOf := func(address string) string {
		number, isOutside := outside.asOf(address)
		if !isOutside {
			return address
		}
		node := "AS" + asn.Text(number, asn.Configured())
		outsideAS[node] = number
		return node
	}

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

		peer, nhop = nodeOf(peer), nodeOf(nhop)
		nodeSet[peer] = true
		nodeSet[nhop] = true

		if peer == nhop {
			egressSet[peer] = true
		} else {
			edgeSet[edgeKey{peer, nhop}] = true
		}
	}

	// Compute layers via BFS from the egress nodes and the outside nodes: an
	// AS outside is where the forwarding ends, whether or not its peer sent Ze
	// a row of its own.
	for node := range outsideAS {
		egressSet[node] = true
	}
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
	slices.Sort(queue) // deterministic BFS order

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		curLayer := layerMap[cur]
		sources := reverse[cur]
		slices.Sort(sources)
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
		number, isOutside := outsideAS[addr]
		nodes = append(nodes, NextHopNode{
			Address: addr,
			Egress:  egressSet[addr] && !isOutside,
			Outside: isOutside,
			AS:      number,
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

	return &nextHopGraph{Nodes: nodes, Edges: edges}
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

// nextHopOutsideMark is the word the text form writes on the node of an AS
// outside the local one, where a router that is an exit carries `egress`.
const nextHopOutsideMark = "outside"

// renderNextHopGraphText returns a deterministic text representation for testing.
//
// A node line reads `node <address> layer=N [egress] [name=...]` for a router
// or an address that is no peer, and `node AS<n> layer=N outside` for an AS
// outside the local one.
func renderNextHopGraphText(g *nextHopGraph) string {
	var sb textbuf.Buffer
	sb.WriteString("mode nexthop\n")
	for _, n := range g.Nodes {
		fmt.Fprintf(&sb, "node %s layer=%d", n.Address, n.Layer) //nolint:errcheck // report output
		if n.Outside {
			sb.Str(" ").Str(nextHopOutsideMark).Byte('\n')
			continue
		}
		if n.Egress {
			sb.WriteString(" egress")
		}
		if n.Name != "" {
			fmt.Fprintf(&sb, " name=%s", n.Name) //nolint:errcheck // report output
		}
		sb.WriteByte('\n')
	}
	for _, e := range g.Edges {
		fmt.Fprintf(&sb, "edge %s -> %s\n", e.From, e.To) //nolint:errcheck // report output
	}
	return sb.String()
}
