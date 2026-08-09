// Design: docs/architecture/ospf/ospf-ext-11-ldp-igp-sync.md -- RFC 6138 cut-edge query.
// RFC: rfc/short/rfc6138.md -- Section 4 (cut-edge MUST-advertise) + Appendix A
// (cut-edge derived from the last SPF; a pending SPF MUST run first).

package spf

import (
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// flushPendingSPF runs a scheduled-but-pending SPF immediately, satisfying RFC 6138
// Appendix A: "If an SPF run was scheduled but is pending execution, that SPF MUST be
// executed immediately before any procedure checks whether an interface is a
// 'cut-edge'." If the pending timer has already fired (its Run is in flight or done),
// nothing is re-run: the last graph is already fresh. Internal to IsCutEdge.
func (c *Computer) flushPendingSPF() {
	c.mu.Lock()
	run := false
	if c.pending && !c.stopped {
		// Cancel the armed timer and take over its run so the WaitGroup stays balanced
		// (TriggerArea added one; the timer callback would have called Done).
		if c.timer != nil && c.timer.Stop() {
			c.runWG.Done()
			c.pending = false
			run = true
		}
	}
	c.mu.Unlock()
	if run {
		c.Run()
	}
}

// IsCutEdge reports whether the broadcast segment whose pseudonode is identified by
// network (the Network-LSA Link State ID, i.e. the DR interface address in OSPFv2) is
// a "cut-edge" for this router: RFC 6138 Section 4 / Appendix A -- there is no
// alternate path to the directly connected network once this router's own link to it
// is removed. A cut-edge MUST be advertised immediately (RFC 6138 §4 MUST NOT-delay),
// so the LDP-sync withhold is skipped for it.
//
// The query first flushes a pending SPF (Appendix A MUST), then traverses the last
// SPF graph with this router's edge to the pseudonode removed: if the pseudonode is
// still reachable via another router the segment has an alternate path (not a
// cut-edge, safe to withhold). If it becomes unreachable, withholding would partition
// the network, so it is a cut-edge and must be advertised.
//
// Default-safe: if the area graph or root is unavailable the interface is treated as a
// cut-edge (advertise, never withhold), so a missing/stale SPF result can never
// partition the network (R-3).
func (c *Computer) IsCutEdge(area types.AreaID, network types.LinkStateID) bool {
	c.flushPendingSPF()
	c.mu.Lock()
	g := c.lastGraphs[area]
	root := c.root
	c.mu.Unlock()
	if g == nil || root == (types.RouterID{}) {
		return true
	}
	return !networkReachableWithoutRootEdge(g, root, network)
}

// networkReachableWithoutRootEdge reports whether the transit-network vertex target is
// reachable from root when root's own direct link to target is removed (a plain
// breadth-first reachability walk over the retained SPF graph -- no metrics, no
// Dijkstra, per RFC 6138 Appendix A "should not increase the algorithmic complexity of
// SPF").
func networkReachableWithoutRootEdge(g *Graph, root types.RouterID, target types.LinkStateID) bool {
	if g == nil {
		return false
	}
	visited := make(map[VertexID]struct{}, len(g.Routers)+len(g.Networks))
	targetV := networkVertex(target)
	start := routerVertex(root)
	queue := []VertexID{start}
	visited[start] = struct{}{}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, nb := range graphNeighbors(g, v, root, target) {
			if nb == targetV {
				return true
			}
			if _, seen := visited[nb]; seen {
				continue
			}
			visited[nb] = struct{}{}
			queue = append(queue, nb)
		}
	}
	_, ok := visited[targetV]
	return ok
}

// graphNeighbors returns the adjacent vertices of v. The single directed edge from the
// root router to the target pseudonode is omitted (that is the link RFC 6138 would
// withhold); every other edge -- including other routers' links to the same pseudonode
// -- is kept, so an alternate path through a peer is still discoverable.
func graphNeighbors(g *Graph, v VertexID, root types.RouterID, target types.LinkStateID) []VertexID {
	var out []VertexID
	switch v.Kind {
	case VertexRouter:
		rv := g.Routers[v.Router]
		if rv == nil {
			return nil
		}
		for _, link := range rv.Links {
			switch link.Type {
			case packet.RouterLinkTypeTransit:
				netID := link.LinkID
				if v.Router == root && netID == target {
					continue // the edge we are testing removal of
				}
				out = append(out, networkVertex(netID))
			case packet.RouterLinkTypeP2P:
				out = append(out, routerVertex(types.RouterID(link.LinkID)))
			}
		}
	case VertexNetwork:
		nv := g.Networks[v.Network]
		if nv == nil {
			return nil
		}
		for _, r := range nv.AttachedRouters {
			out = append(out, routerVertex(r))
		}
	}
	return out
}
