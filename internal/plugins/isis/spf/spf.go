// Design: docs/architecture/isis/isis-9-spf-rib.md -- per-level Dijkstra over the LSDB graph.
// ISO/IEC 10589 clause 7.2 (the Decision Process): a shortest-path-first
// computation rooted at the local node over the directed graph of nodes and
// adjacencies, yielding the distance and first-hop(s) to every reachable node.
//
// RFC: rfc/short/rfc5305.md -- the IS-reachability edge metric is 24-bit; the
//   IP/IPv6 prefix metric is 32-bit. Path cost is accumulated in a 64-bit
//   accumulator and clamped at MAX_PATH_METRIC (0xFE000000) so a sum of wide
//   metrics never wraps (sec 3, sec 4).
// RFC: rfc/short/rfc3787.md -- a node with the overload bit is reachable as a
//   destination but is NOT used as a transit node: SPF does not relax edges OUT
//   of an overloaded node (its own prefixes still attach).
//
// ECMP: equal-cost predecessors are all retained, so a node reachable by two
// equal-cost paths carries two first-hops; the route builder (route.go) turns
// them into one locrib.Path per next-hop (spec AC-3).

package spf

import (
	"slices"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// MaxPathMetric is RFC 5305 / RFC 5308 MAX_PATH_METRIC. A prefix advertised with
// a metric >= this value is unreachable and excluded from SPF; an accumulated
// path cost is clamped here so a sum of wide metrics never wraps (spec
// "Total path metric" boundary, RFC 5305 sec 3 / RFC 5308 sec 2).
const MaxPathMetric uint64 = 0xFE000000

// clampMetric adds delta to base and clamps the 64-bit sum at MaxPathMetric so
// the accumulator never wraps. A result at or above MaxPathMetric is treated as
// unreachable by the caller (RFC 5305 sec 3).
func clampMetric(base, delta uint64) uint64 {
	sum := base + delta
	if sum < base || sum > MaxPathMetric { // overflow or past the ceiling
		return MaxPathMetric
	}
	return sum
}

// NodeResult is the SPF outcome for one reachable node: its total distance from
// the root and the set of equal-cost first-hops toward it (ECMP). The first-hop
// is a directly-adjacent neighbor System ID (a real router, never a
// pseudo-node); the route builder resolves it to a next-hop address + interface
// via the local adjacency table.
type NodeResult struct {
	// ID is the reached node's Source ID.
	ID types.SourceID
	// Metric is the total path cost from the root (clamped at MaxPathMetric).
	Metric uint64
	// FirstHops are the System IDs of the directly-adjacent neighbors on the
	// equal-cost shortest paths to ID. The root itself has an empty set (it is
	// directly connected, no next-hop). One entry for a single path; multiple for
	// ECMP. A reachable node always has at least one first-hop unless it is the
	// root.
	FirstHops []types.SystemID
}

// Result is the full SPF output for one level: the reachable nodes keyed by
// Source ID, with the root's own entry at distance 0.
type Result struct {
	// Root is the System ID SPF was rooted at.
	Root types.SystemID
	// Level is the level this result covers.
	Level Level
	// Nodes maps each reachable node's Source ID to its result.
	Nodes map[types.SourceID]*NodeResult
}

// spfHeap entry: a node plus its tentative distance. The heap orders by distance
// (smallest first) so Dijkstra always extracts the closest tentative node.
type heapItem struct {
	id   types.SourceID
	dist uint64
}

type spfHeap []heapItem

func (h *spfHeap) push(it heapItem) {
	*h = append(*h, it)
	s := *h
	i := len(s) - 1
	for i > 0 {
		parent := (i - 1) / 2
		if s[i].dist >= s[parent].dist {
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
		if right := left + 1; right < len(s) && s[right].dist < s[left].dist {
			j = right
		}
		if s[j].dist >= s[i].dist {
			break
		}
		s[i], s[j] = s[j], s[i]
		i = j
	}
	return it
}

// tent is the per-node tentative state during the Dijkstra relaxation.
type tent struct {
	dist      uint64
	firstHops []types.SystemID
	settled   bool
}

// Compute runs Dijkstra over g rooted at the router with System ID root, for the
// given level. It returns the distance and ECMP first-hops to every reachable
// node. An overloaded node (RFC 3787) is reached as a destination but its
// outgoing edges are not relaxed, so it never serves as transit. A node absent
// from the graph (a dangling edge target) is skipped. The accumulator is 64-bit
// and clamped at MaxPathMetric so wide-metric sums never wrap (RFC 5305 sec 3).
func Compute(g *Graph, root types.SystemID, level Level) *Result {
	res := &Result{
		Root:  root,
		Level: level,
		Nodes: make(map[types.SourceID]*NodeResult),
	}
	if g == nil {
		return res
	}
	rootID := types.NewSourceID(root, 0)
	if _, ok := g.Nodes[rootID]; !ok {
		// The root has no self-LSP in this graph: nothing is reachable.
		return res
	}

	tents := make(map[types.SourceID]*tent)
	tents[rootID] = &tent{dist: 0}

	h := &spfHeap{{id: rootID, dist: 0}}

	for len(*h) > 0 {
		item := h.pop()
		cur := tents[item.id]
		if cur == nil || cur.settled {
			continue // a stale heap entry for an already-settled node
		}
		if item.dist > cur.dist {
			continue // superseded by a shorter relaxation already queued
		}
		cur.settled = true

		node := g.Nodes[item.id]
		if node == nil {
			continue // dangling edge target: reachable distance known, no edges
		}
		// RFC 3787 sec 4: an overloaded node is a valid destination but MUST NOT
		// be used as transit. Do not relax its outgoing edges. The root is never
		// treated as overloaded for its OWN edges (it must reach its neighbors);
		// in practice a node never sets overload against itself in a way that
		// should strand its directly-connected links, and the root's prefixes
		// always attach regardless.
		if node.Overload && item.id != rootID {
			continue
		}

		for _, e := range node.Edges {
			relax(h, tents, cur, item.id, rootID, e)
		}
	}

	for id, t := range tents {
		if !t.settled {
			continue
		}
		res.Nodes[id] = &NodeResult{
			ID:        id,
			Metric:    t.dist,
			FirstHops: t.firstHops,
		}
	}
	return res
}

// relax considers the edge from the node identified by fromID (with settled
// tentative state cur) to e.To, updating the neighbor's tentative distance and
// ECMP first-hop set. The first-hop is seeded at the edge OUT of the root (the
// neighbor's own System ID) and, for the LAN case, at the edge out of a
// root-attached pseudo-node (the member's own System ID); deeper edges inherit
// the predecessor's first-hops. Pseudo-nodes are never a first-hop (a first-hop
// must be a directly-adjacent real router), so an edge from the root to a
// pseudo-node contributes no first-hop and the member routers behind the
// pseudo-node are seeded with their own System ID when the pseudo-node's edges
// are relaxed (firstHopsFor).
func relax(h *spfHeap, tents map[types.SourceID]*tent, cur *tent, fromID, rootID types.SourceID, e Edge) {
	nd := clampMetric(cur.dist, uint64(e.Metric))
	if nd >= MaxPathMetric {
		return // unreachable: the edge pushes the path past MAX_PATH_METRIC
	}

	hops := firstHopsFor(cur, fromID, rootID, e.To)

	nt, ok := tents[e.To]
	if !ok {
		tents[e.To] = &tent{dist: nd, firstHops: hops}
		h.push(heapItem{id: e.To, dist: nd})
		return
	}
	if nt.settled {
		return // already finalized at an equal or shorter distance
	}
	switch {
	case nd < nt.dist:
		nt.dist = nd
		nt.firstHops = hops
		h.push(heapItem{id: e.To, dist: nd})
	case nd == nt.dist:
		// Equal-cost path: merge the first-hop sets (ECMP), de-duplicated.
		nt.firstHops = mergeHops(nt.firstHops, hops)
	}
}

// firstHopsFor returns the first-hop set a neighbor inherits when reached via the
// edge from fromID. An edge leaving the root seeds the first-hop with the target
// router's own System ID (it is directly adjacent); a target that is a
// pseudo-node contributes no first-hop (a pseudo-node is not a router) and the
// real routers behind it inherit theirs as the relaxation continues. Any deeper
// edge inherits the predecessor's first-hops unchanged.
//
// ISO/IEC 10589 clause 7.2.7: a vertex's first hop (the "Adj(N)" / parent
// adjacency) is set when its parent on the SPT is the root OR a root-attached
// pseudo-node. The LAN case is the common topology: the root advertises only
// root->PN, and the pseudo-node advertises PN->member at metric 0, so a member
// router's PARENT is the pseudo-node, not the root. The first hop toward that
// member is the member ITSELF (it is directly reachable across the shared LAN),
// not the pseudo-node (which is virtual and never a next-hop). Without this the
// member inherits the pseudo-node's empty first-hop set and BuildRoutes drops
// every LAN-learned prefix (route.go len(FirstHops)==0 guard).
func firstHopsFor(cur *tent, fromID, rootID, to types.SourceID) []types.SystemID {
	if fromID == rootID {
		// Edge directly out of the root.
		if to.IsPseudonode() {
			// The LAN pseudo-node is one hop from the root but is not itself a
			// next-hop; the routers it connects to become the first-hops when the
			// pseudo-node's own edges are relaxed (the branch below).
			return nil
		}
		return []types.SystemID{to.SystemID()}
	}
	// Edge out of a ROOT-ATTACHED pseudo-node: the predecessor is a pseudo-node
	// (fromID.IsPseudonode()) that was reached directly from the root, which is
	// exactly the case where it carries an EMPTY first-hop set (a pseudo-node
	// reached deeper would have inherited a non-empty set). The member router on
	// the far side of the LAN is the first hop toward itself, so seed its own
	// System ID. A pseudo-node target is still never a first-hop.
	if len(cur.firstHops) == 0 && fromID.IsPseudonode() && !to.IsPseudonode() {
		return []types.SystemID{to.SystemID()}
	}
	// Deeper in the tree: inherit the predecessor's first-hops.
	return cur.firstHops
}

// mergeHops returns the union of two first-hop slices with duplicates removed,
// preserving a stable order (existing entries first, then new ones). Small N
// (the ECMP width), so a linear membership check is fine.
func mergeHops(a, b []types.SystemID) []types.SystemID {
	out := make([]types.SystemID, len(a), len(a)+len(b))
	copy(out, a)
	for _, x := range b {
		if !slices.Contains(out, x) {
			out = append(out, x)
		}
	}
	return out
}
