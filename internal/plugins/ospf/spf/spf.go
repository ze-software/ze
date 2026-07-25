// Design: plan/learned/962-ospf-8-spf-rib.md -- RFC 2328 intra-area SPF.
// RFC 2328 Section 16.1: stage 1 runs Dijkstra over router and transit-network
// vertices with a two-way check; stage 2 attaches stub networks to reached
// router vertices.

package spf

import (
	"net/netip"
	"slices"
	"sort"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// LSInfinity is the RFC 2328 unreachable OSPF cost ceiling. A path cost at or
// above this value is excluded from the tree and route table.
const LSInfinity uint64 = 0x00ff_ffff

// DefaultMaxPaths is the umbrella committed ECMP cap.
const DefaultMaxPaths = 8

// NextHop is one resolved equal-cost OSPF next-hop. Router is the Router ID at
// the far end of the FIRST hop toward the destination (the SPF next-hop router):
// for a vertex reached directly from root it is the neighbor; for a deeper vertex
// it is inherited from the first hop. It is a deterministic function of Addr (the
// router owning that interface address) and drives the RFC 8665 §5 / RFC 8666 §6
// SR-MPLS label source (the next-hop router's SRGB, not the SID originator's).
type NextHop struct {
	Addr      netip.Addr
	Interface string
	Router    types.RouterID
}

// InterfaceResolver optionally maps a next-hop address to an outgoing interface
// name for snapshots. The SPF next-hop address itself comes from LSAs.
type InterfaceResolver interface {
	ResolveInterface(netip.Addr) (string, bool)
}

// NodeResult is the stage-1 SPF result for one reached transit vertex.
type NodeResult struct {
	ID       VertexID
	Metric   uint64
	NextHops []NextHop
}

// Result is the complete stage-1 SPF tree for one area.
type Result struct {
	Area    types.AreaID
	Root    types.RouterID
	Nodes   map[VertexID]*NodeResult
	Graph   *Graph
	MaxPath uint64
}

type tent struct {
	dist     uint64
	nextHops []NextHop
	settled  bool
}

type heapItem struct {
	id   VertexID
	dist uint64
}

type spfHeap []heapItem

func (h spfHeap) less(i, j int) bool {
	if h[i].dist != h[j].dist {
		return h[i].dist < h[j].dist
	}
	if h[i].id.Kind != h[j].id.Kind {
		return h[i].id.Kind == VertexNetwork
	}
	return compareVertexID(h[i].id, h[j].id) < 0
}

func (h *spfHeap) push(it heapItem) {
	*h = append(*h, it)
	s := *h
	i := len(s) - 1
	for i > 0 {
		parent := (i - 1) / 2
		if !s.less(i, parent) {
			break
		}
		s[i], s[parent] = s[parent], s[i]
		i = parent
	}
}

func (h *spfHeap) pop() heapItem {
	s := *h
	n := len(s)
	it := s[0]
	s[0] = s[n-1]
	s = s[:n-1]
	*h = s
	i := 0
	for {
		left := 2*i + 1
		if left >= len(s) {
			break
		}
		j := left
		if right := left + 1; right < len(s) && s.less(right, left) {
			j = right
		}
		if !s.less(j, i) {
			break
		}
		s[i], s[j] = s[j], s[i]
		i = j
	}
	return it
}

// NextHopSource resolves the SPF next-hop address for a vertex reached directly
// from the root. OSPFv2 reads the neighbor's interface address from the
// reciprocal Router-LSA link data (carried in the graph); OSPFv3 Router-LSAs have
// no per-link address, so the v6 source resolves the neighbor's IPv6 link-local
// from the adjacency table. The address is AF-neutral (netip.Addr); only its
// source differs, which is why this is the one AF seam inside the Dijkstra.
type NextHopSource interface {
	// P2PNextHop returns the next-hop to a point-to-point neighbor reached from root.
	P2PNextHop(g *Graph, neighbor, root types.RouterID) (netip.Addr, bool)
	// TransitNextHop returns the next-hop to a router reached via a root-attached
	// transit network.
	TransitNextHop(g *Graph, router types.RouterID, network types.LinkStateID) (netip.Addr, bool)
}

// v4NextHop is the OSPFv2 NextHopSource: the next-hop is the neighbor's IPv4
// interface address carried in the reciprocal Router-LSA link data.
type v4NextHop struct{}

func (v4NextHop) P2PNextHop(g *Graph, neighbor, root types.RouterID) (netip.Addr, bool) {
	return p2pNeighborAddress(g, neighbor, root)
}

func (v4NextHop) TransitNextHop(g *Graph, router types.RouterID, network types.LinkStateID) (netip.Addr, bool) {
	return transitRouterAddress(g, router, network)
}

// Compute runs RFC 2328 Section 16.1 stage 1 for one area with the OSPFv2 next-hop
// source. The root must have a Router-LSA in the graph or only an empty result is
// returned.
func Compute(g *Graph, root types.RouterID, maxPaths int) *Result {
	return computeWithNextHop(g, root, maxPaths, v4NextHop{})
}

// computeWithNextHop is Compute parameterized by the address-family next-hop
// source, so the v6 family can resolve next-hops from its adjacency table.
func computeWithNextHop(g *Graph, root types.RouterID, maxPaths int, nh NextHopSource) *Result {
	if maxPaths <= 0 {
		maxPaths = DefaultMaxPaths
	}
	res := &Result{Root: root, Nodes: make(map[VertexID]*NodeResult), Graph: g, MaxPath: LSInfinity}
	if g == nil {
		return res
	}
	res.Area = g.Area
	rootID := routerVertex(root)
	if g.Routers[root] == nil {
		return res
	}

	tents := map[VertexID]*tent{rootID: {dist: 0}}
	h := &spfHeap{{id: rootID, dist: 0}}

	// RFC 2328 Section 16.1: stage 1 considers only router and transit-network
	// vertices. Stub networks are leaves and are attached later in route.go.
	for len(*h) > 0 {
		item := h.pop()
		cur := tents[item.id]
		if cur == nil || cur.settled || item.dist > cur.dist {
			continue
		}
		cur.settled = true
		for _, e := range transitEdges(g, item.id, cur, rootID, nh) {
			relax(h, tents, cur, e, maxPaths)
		}
	}

	for id, t := range tents {
		if !t.settled {
			continue
		}
		hops := make([]NextHop, len(t.nextHops))
		copy(hops, t.nextHops)
		res.Nodes[id] = &NodeResult{ID: id, Metric: t.dist, NextHops: hops}
	}
	return res
}

type edge struct {
	to       VertexID
	metric   uint64
	nextHops []NextHop
}

func transitEdges(g *Graph, from VertexID, cur *tent, rootID VertexID, nh NextHopSource) []edge {
	switch from.Kind {
	case VertexRouter:
		r := g.Routers[from.Router]
		if r == nil {
			return nil
		}
		out := make([]edge, 0, len(r.Links))
		for _, l := range r.Links {
			switch l.Type {
			case packet.RouterLinkTypeP2P, packet.RouterLinkTypeVirtual:
				toRouter := routerIDFromLinkStateID(l.LinkID)
				to := routerVertex(toRouter)
				if !twoWayRouterLink(g, from.Router, toRouter) {
					continue
				}
				hops := nextHopsForP2P(g, cur, from, rootID, toRouter, nh)
				out = append(out, edge{to: to, metric: uint64(l.Metric), nextHops: hops})
			case packet.RouterLinkTypeTransit:
				to := networkVertex(l.LinkID)
				if !twoWayRouterNetworkLink(g, from.Router, l.LinkID) {
					continue
				}
				hops := inheritedHops(cur)
				out = append(out, edge{to: to, metric: uint64(l.Metric), nextHops: hops})
			}
		}
		return out
	case VertexNetwork:
		n := g.Networks[from.Network]
		if n == nil {
			return nil
		}
		out := make([]edge, 0, len(n.AttachedRouters))
		for _, rid := range n.AttachedRouters {
			if !twoWayRouterNetworkLink(g, rid, from.Network) {
				continue
			}
			hops := nextHopsForNetwork(g, cur, from, rid, nh)
			out = append(out, edge{to: routerVertex(rid), metric: 0, nextHops: hops})
		}
		return out
	default:
		return nil
	}
}

func relax(h *spfHeap, tents map[VertexID]*tent, cur *tent, e edge, maxPaths int) {
	nd := clampMetric(cur.dist, e.metric)
	if nd >= LSInfinity {
		return
	}
	hops := capNextHops(e.nextHops, maxPaths)
	nt, ok := tents[e.to]
	if !ok {
		tents[e.to] = &tent{dist: nd, nextHops: hops}
		h.push(heapItem{id: e.to, dist: nd})
		return
	}
	if nd < nt.dist {
		nt.dist = nd
		nt.nextHops = hops
		nt.settled = false
		h.push(heapItem{id: e.to, dist: nd})
		return
	}
	if nd == nt.dist {
		merged, changed := mergeNextHops(nt.nextHops, hops, maxPaths)
		if changed {
			nt.nextHops = merged
			nt.settled = false
			h.push(heapItem{id: e.to, dist: nd})
		}
	}
}

func clampMetric(base, delta uint64) uint64 {
	sum := base + delta
	if sum < base || sum >= LSInfinity {
		return LSInfinity
	}
	return sum
}

// RFC 2328 Section 16.1 step 2b: a transit link is usable only when the target
// vertex has an LSA that links back to the current vertex.
func twoWayRouterLink(g *Graph, from, to types.RouterID) bool {
	tr := g.Routers[to]
	if tr == nil {
		return false
	}
	want := linkStateIDFromRouterID(from)
	for _, l := range tr.Links {
		if (l.Type == packet.RouterLinkTypeP2P || l.Type == packet.RouterLinkTypeVirtual) && l.LinkID == want {
			return true
		}
	}
	return false
}

func twoWayRouterNetworkLink(g *Graph, router types.RouterID, network types.LinkStateID) bool {
	n := g.Networks[network]
	if n == nil || !slices.Contains(n.AttachedRouters, router) {
		return false
	}
	r := g.Routers[router]
	if r == nil {
		return false
	}
	for _, l := range r.Links {
		if l.Type == packet.RouterLinkTypeTransit && l.LinkID == network {
			return true
		}
	}
	return false
}

// RFC 2328 Section 16.1.1: when the parent is the root, the P2P next-hop is the
// neighbor's interface address from the neighbor's reciprocal Router-LSA link.
// Deeper vertices inherit the parent's next-hop set.
func nextHopsForP2P(g *Graph, cur *tent, from, rootID VertexID, to types.RouterID, nh NextHopSource) []NextHop {
	if from == rootID {
		// Directly-reached neighbor: the next-hop router IS `to`. Record it so the SR
		// installer can source the label from this neighbor's SRGB (RFC 8665 §5).
		if addr, ok := nh.P2PNextHop(g, to, rootID.Router); ok {
			return []NextHop{{Addr: addr, Router: to}}
		}
		return nil
	}
	return inheritedHops(cur)
}

// RFC 2328 Section 16.1.1: for a router reached through a root-attached transit
// network, the next hop is that router's interface address on the network,
// carried in the router's transit Router-LSA link data. Deeper vertices inherit.
func nextHopsForNetwork(g *Graph, cur *tent, from VertexID, to types.RouterID, nh NextHopSource) []NextHop {
	if from.Kind == VertexNetwork && len(cur.nextHops) == 0 {
		// Router reached through a root-attached transit network: the next-hop router
		// IS `to` (the attached router). Record it for the SR label source (RFC 8665 §5).
		if addr, ok := nh.TransitNextHop(g, to, from.Network); ok {
			return []NextHop{{Addr: addr, Router: to}}
		}
		return nil
	}
	return inheritedHops(cur)
}

func inheritedHops(cur *tent) []NextHop {
	if cur == nil || len(cur.nextHops) == 0 {
		return nil
	}
	out := make([]NextHop, len(cur.nextHops))
	copy(out, cur.nextHops)
	return out
}

func p2pNeighborAddress(g *Graph, neighbor, root types.RouterID) (netip.Addr, bool) {
	r := g.Routers[neighbor]
	if r == nil {
		return netip.Addr{}, false
	}
	want := linkStateIDFromRouterID(root)
	for _, l := range r.Links {
		if (l.Type == packet.RouterLinkTypeP2P || l.Type == packet.RouterLinkTypeVirtual) && l.LinkID == want {
			return addrFrom4(l.LinkData), true
		}
	}
	return netip.Addr{}, false
}

func transitRouterAddress(g *Graph, router types.RouterID, network types.LinkStateID) (netip.Addr, bool) {
	r := g.Routers[router]
	if r == nil {
		return netip.Addr{}, false
	}
	for _, l := range r.Links {
		if l.Type == packet.RouterLinkTypeTransit && l.LinkID == network {
			return addrFrom4(l.LinkData), true
		}
	}
	return netip.Addr{}, false
}

func mergeNextHops(a, b []NextHop, maxPaths int) ([]NextHop, bool) {
	out := make([]NextHop, len(a), len(a)+len(b))
	copy(out, a)
	changed := false
	for _, nh := range b {
		if !nh.Addr.IsValid() {
			continue
		}
		if containsNextHop(out, nh) {
			continue
		}
		if maxPaths > 0 && len(out) >= maxPaths {
			continue
		}
		out = append(out, nh)
		changed = true
	}
	return out, changed
}

func capNextHops(in []NextHop, maxPaths int) []NextHop {
	if len(in) == 0 {
		return nil
	}
	out := make([]NextHop, 0, len(in))
	for _, nh := range in {
		if !nh.Addr.IsValid() || containsNextHop(out, nh) {
			continue
		}
		if maxPaths > 0 && len(out) >= maxPaths {
			break
		}
		out = append(out, nh)
	}
	return out
}

func containsNextHop(hops []NextHop, nh NextHop) bool {
	return slices.ContainsFunc(hops, func(x NextHop) bool { return x.Addr == nh.Addr && x.Interface == nh.Interface })
}

func sortNextHops(hops []NextHop) {
	sort.Slice(hops, func(i, j int) bool {
		if c := hops[i].Addr.Compare(hops[j].Addr); c != 0 {
			return c < 0
		}
		return hops[i].Interface < hops[j].Interface
	})
}

func addrFrom4(a [4]byte) netip.Addr { return netip.AddrFrom4(a) }

func linkStateIDFromRouterID(id types.RouterID) types.LinkStateID { return types.LinkStateID(id) }

func routerIDFromLinkStateID(id types.LinkStateID) types.RouterID { return types.RouterID(id) }

func compareVertexID(a, b VertexID) int {
	if a.Kind != b.Kind {
		if a.Kind < b.Kind {
			return -1
		}
		return 1
	}
	if a.Kind == VertexRouter {
		return compare4(a.Router, b.Router)
	}
	return compare4(a.Network, b.Network)
}

func compare4[A ~[4]byte](a, b A) int {
	for i := range 4 {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
