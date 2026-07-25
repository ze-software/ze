// Design: plan/learned/1051-ospf-ext-6-ti-lfa.md -- TI-LFA repair list builder. Where no
// directly-connected loop-free alternate exists (RFC 5286), TI-LFA builds an
// explicit Segment Routing repair list along the post-convergence path: a
// Prefix-SID toward a P-node (a node S can reach avoiding the protected resource)
// then, when the P-space and Q-space do not overlap, an Adj-SID across the
// protected resource into the Q-space, then the destination Prefix-SID. The SR
// labels come from ext-5's resolved Prefix-SID / Adj-SID maps (RFC 8665
// Section 5 / Section 6.1); ext-6 only reads them.
// RFC: rfc/short/rfc5286.md (Section 3.2 P/Q space); rfc/short/rfc8665.md (Section 5/6.1)

package spf

import (
	"sort"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// buildTILFA computes a TI-LFA SR repair backup protecting the primary next-hop
// (whose far end is neighbor E) toward destination vertex v. It protects against
// E's node failure (which also covers the S->E link), using node-protecting
// P-space and Q-space (RFC 5286 Section 3.2). It returns false when v is not a
// router destination, no P/Q repair exists, or the required SR labels are
// unresolvable (the prefix is then left unprotected, counted as no-sr).
func buildTILFA(g *Graph, res *Result, v VertexID, primary NextHop, in fastRerouteInput, ensureSPT func(types.RouterID) *Result) (Backup, bool) {
	if v.Kind != VertexRouter {
		return Backup{}, false // TI-LFA repair lists steer toward a router destination
	}
	dest := v.Router
	e := primary.Router
	if e == (types.RouterID{}) || e == in.root {
		return Backup{}, false
	}
	sptE := ensureSPT(e)
	if sptE == nil {
		return Backup{}, false
	}
	dSE := vertexDist(res, routerVertex(e)) // D_opt(S,E)
	dEV := vertexDist(sptE, v)              // D_opt(E,D)
	if dSE >= LSInfinity {
		return Backup{}, false
	}

	// Post-convergence reachability gate (RFC 5286 / TI-LFA, spec A-5): clone the
	// graph, remove the protected node, and re-run SPF. If the destination is
	// unreachable without E, fall back to removing only the S->E link (link
	// protection). If it is still unreachable, E is a genuine single point of
	// failure and no repair exists.
	pruned := g.Clone()
	pruned.excludeRouter(e)
	if vertexDist(computeWithNextHop(pruned, in.root, in.maxPaths, in.nh), v) >= LSInfinity {
		pruned = g.Clone()
		pruned.excludeLink(in.root, e)
		if vertexDist(computeWithNextHop(pruned, in.root, in.maxPaths, in.nh), v) >= LSInfinity {
			return Backup{}, false
		}
	}

	// P-space: routers whose optimal path from S avoids E (RFC 5286 Section 3.2).
	// Q-space: routers whose optimal path to D avoids E.
	type node struct {
		id   types.RouterID
		dist uint64 // D_opt(root,node) for P ordering, or D_opt(node,D) for Q ordering
	}
	var pSpace, qSpace []node
	inQ := make(map[types.RouterID]bool)
	for _, rid := range sortedRouterIDs(g) {
		if rid == in.root || rid == e {
			continue
		}
		sptR := ensureSPT(rid)
		if sptR == nil {
			continue
		}
		dSR := vertexDist(res, routerVertex(rid))
		dER := vertexDist(sptE, routerVertex(rid))
		// P-space: D_opt(S,R) < D_opt(S,E) + D_opt(E,R).
		if dSR < LSInfinity && dSR < clampSum(dSE, dER) {
			pSpace = append(pSpace, node{id: rid, dist: dSR})
		}
		dRV := vertexDist(sptR, v)
		dRE := vertexDist(sptR, routerVertex(e))
		// Q-space: D_opt(R,D) < D_opt(R,E) + D_opt(E,D).
		if dRV < LSInfinity && dRV < clampSum(dRE, dEV) {
			qSpace = append(qSpace, node{id: rid, dist: dRV})
			inQ[rid] = true
		}
	}
	sort.Slice(pSpace, func(i, j int) bool { return pSpace[i].dist < pSpace[j].dist })
	sort.Slice(qSpace, func(i, j int) bool { return qSpace[i].dist < qSpace[j].dist })

	sr := in.sr

	// Case A: a single PQ node (in both P-space and Q-space). Steer to it with its
	// Prefix-SID; it then forwards natively to D avoiding E.
	for _, p := range pSpace {
		if !inQ[p.id] {
			continue
		}
		label, ok := sr.PrefixSIDLabel(p.id)
		if !ok || label > maxMPLSLabel {
			continue
		}
		labels := []uint32{label}
		if p.id != dest {
			if dl, dok := sr.PrefixSIDLabel(dest); dok && dl <= maxMPLSLabel {
				labels = append(labels, dl)
			}
		}
		if b, ok := tilfaBackup(res, p.id, labels, in); ok {
			return b, true
		}
	}

	// Case B: P-space and Q-space are disjoint. Steer to a P-node, then an Adj-SID
	// across the protected resource into an adjacent Q-node, then the destination
	// Prefix-SID (RFC 8665 Section 6.1 Q-segment).
	for _, p := range pSpace {
		pLabel, ok := sr.PrefixSIDLabel(p.id)
		if !ok || pLabel > maxMPLSLabel {
			continue
		}
		for _, q := range qSpace {
			if !adjacent(g, p.id, q.id) {
				continue
			}
			adj, aok := sr.AdjSIDLabel(p.id, q.id)
			if !aok || adj > maxMPLSLabel {
				continue
			}
			labels := []uint32{pLabel, adj}
			if q.id != dest {
				if dl, dok := sr.PrefixSIDLabel(dest); dok && dl <= maxMPLSLabel {
					labels = append(labels, dl)
				}
			}
			if len(labels) > maxRepairDepth {
				continue
			}
			if b, ok := tilfaBackup(res, p.id, labels, in); ok {
				return b, true
			}
		}
	}
	return Backup{}, false
}

// tilfaBackup builds the backup next-hop toward the P-node `to` (S's optimal
// next-hop, which is in P-space and therefore avoids the protected resource) and
// attaches the resolved repair label stack.
func tilfaBackup(res *Result, to types.RouterID, labels []uint32, in fastRerouteInput) (Backup, bool) {
	nr := res.Nodes[routerVertex(to)]
	if nr == nil || len(nr.NextHops) == 0 || len(labels) == 0 || len(labels) > maxRepairDepth {
		return Backup{}, false
	}
	hop := nr.NextHops[0]
	iface := hop.Interface
	if iface == "" && in.resolver != nil {
		if name, ok := in.resolver.ResolveInterface(hop.Addr); ok {
			iface = name
		}
	}
	return Backup{
		NextHop:      hop.Addr,
		Interface:    iface,
		RepairLabels: labels,
		LinkProtect:  true, // the post-convergence path avoids the protected link
		NodeProtect:  true, // and the protected node E
		Downstream:   false,
		Kind:         BackupTILFA,
	}, true
}

// adjacent reports whether router a has a point-to-point / virtual link to b.
func adjacent(g *Graph, a, b types.RouterID) bool {
	r := g.Routers[a]
	if r == nil {
		return false
	}
	want := linkStateIDFromRouterID(b)
	for _, l := range r.Links {
		if (l.Type == rlP2P || l.Type == rlVirtual) && l.LinkID == want {
			return true
		}
	}
	return false
}

func sortedRouterIDs(g *Graph) []types.RouterID {
	out := make([]types.RouterID, 0, len(g.Routers))
	for id := range g.Routers {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return compare4(out[i], out[j]) < 0 })
	return out
}
