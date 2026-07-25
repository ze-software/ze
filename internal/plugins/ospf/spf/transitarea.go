// Design: plan/learned/1043-ospf-ext-7-virtual-links.md -- the shared transit-area SPF side of
// OSPF virtual links.
// RFC 2328 Section 16.1: a virtual neighbor's reachability, cost, and next hop come from
// the transit area's intra-area shortest-path tree; the virtual link is down when the
// neighbor is unreachable there. Section 16.3: an ABR attached to a transit area
// (TransitCapability TRUE) re-examines that area's Summary-LSAs to IMPROVE already-
// reachable backbone routes and to resolve the real transit next hop for any route whose
// next hop is a virtual link; it never makes a new destination reachable.
// RFC 5340 Section 3.5: a fully adjacent virtual-link endpoint is backbone-attached.
// RFC: rfc/short/rfc2328.md (sec 15, 16.1, 16.3), rfc/short/rfc5340.md (sec 3.5, 4.2)

package spf

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VirtualLinkRequest identifies a configured virtual link for transit-area resolution: the
// transit area whose intra-area SPF result is read, and the far endpoint's Router ID.
type VirtualLinkRequest struct {
	TransitArea types.AreaID
	Neighbor    types.RouterID
}

// VirtualNeighborResult is the transit-area SPF resolution of one virtual link (RFC 2328
// Section 16.1 / RFC 5340 Section 4.2): whether the neighbor is a reachable router vertex
// in the transit area, the intra-area path cost (the virtual link's output cost), and the
// transit next hops toward it (the real next hop for backbone routes via the virtual link).
type VirtualNeighborResult struct {
	TransitArea types.AreaID
	Neighbor    types.RouterID
	Reachable   bool
	Cost        uint64
	NextHops    []NextHop
}

// SetVirtualLinks configures the virtual links resolved each SPF run against their transit
// area's result. An empty list disables virtual-link processing (byte-for-byte the prior
// behavior).
func (c *Computer) SetVirtualLinks(reqs []VirtualLinkRequest) {
	c.mu.Lock()
	c.virtualLinks = append([]VirtualLinkRequest(nil), reqs...)
	c.mu.Unlock()
}

// SetOnVirtualLinks registers a callback invoked after an SPF run whose resolved virtual
// neighbor set changed (reachability, cost, or next hops). It drives the engine's synthetic
// virtual interface up/down and its cost; a nil callback disables it.
func (c *Computer) SetOnVirtualLinks(fn func([]VirtualNeighborResult)) {
	c.mu.Lock()
	c.onVirtual = fn
	c.mu.Unlock()
}

// resolveVirtualNeighbor reads a transit area's intra-area SPF Result for one virtual
// neighbor (RFC 2328 Section 16.1). The neighbor is usable only if it is a reachable
// router vertex (its Router-LSA is present in the graph, i.e. non-MaxAge) with a finite
// cost and at least one next hop; otherwise Reachable is false and the link is down.
func resolveVirtualNeighbor(res *Result, neighbor types.RouterID) VirtualNeighborResult {
	out := VirtualNeighborResult{Neighbor: neighbor}
	if res == nil {
		return out
	}
	out.TransitArea = res.Area
	if res.Graph == nil || res.Graph.Routers[neighbor] == nil {
		return out
	}
	nr := res.Nodes[routerVertex(neighbor)]
	if nr == nil || nr.Metric >= LSInfinity || len(nr.NextHops) == 0 {
		return out
	}
	out.Reachable = true
	out.Cost = nr.Metric
	out.NextHops = append([]NextHop(nil), nr.NextHops...)
	return out
}

// TransitCapability reports RFC 2328 Section 16.3 TransitCapability for an area: TRUE iff
// some Router-LSA in the area sets the V-bit (the router is a fully adjacent virtual-link
// endpoint using this area as transit).
func TransitCapability(res *Result) bool {
	if res == nil || res.Graph == nil {
		return false
	}
	for _, rv := range res.Graph.Routers {
		if rv != nil && rv.Flags&packet.RouterFlagV != 0 {
			return true
		}
	}
	return false
}

// resolveVirtualNeighbors resolves every configured virtual link against its transit area's
// SPF result. Requests are preserved even when their transit result is missing (an
// unreachable/absent transit area yields a not-Reachable entry).
func resolveVirtualNeighbors(reqs []VirtualLinkRequest, results map[types.AreaID]*Result) []VirtualNeighborResult {
	if len(reqs) == 0 {
		return nil
	}
	out := make([]VirtualNeighborResult, 0, len(reqs))
	for _, req := range reqs {
		vr := resolveVirtualNeighbor(results[req.TransitArea], req.Neighbor)
		vr.TransitArea = req.TransitArea
		vr.Neighbor = req.Neighbor
		out = append(out, vr)
	}
	return out
}

// updateVirtualLocked replaces the cached resolution with the new one and returns the
// callback plus whether anything changed. It runs under c.mu.
func (c *Computer) updateVirtualLocked(results []VirtualNeighborResult) (func([]VirtualNeighborResult), bool) {
	next := make(map[VirtualLinkRequest]VirtualNeighborResult, len(results))
	changed := len(results) != len(c.lastVirtual)
	for _, vr := range results {
		key := VirtualLinkRequest{TransitArea: vr.TransitArea, Neighbor: vr.Neighbor}
		next[key] = vr
		if prev, ok := c.lastVirtual[key]; !ok || !virtualResultEqual(prev, vr) {
			changed = true
		}
	}
	c.lastVirtual = next
	return c.onVirtual, changed
}

func virtualResultEqual(a, b VirtualNeighborResult) bool {
	if a.Reachable != b.Reachable || a.Cost != b.Cost || len(a.NextHops) != len(b.NextHops) {
		return false
	}
	for i := range a.NextHops {
		if a.NextHops[i] != b.NextHops[i] {
			return false
		}
	}
	return true
}

// rootVirtualBackboneAttached reports RFC 5340 Section 3.5 backbone attachment: the local
// router is the endpoint of a fully adjacent virtual link, signaled by a RouterLinkTypeVirtual
// record in its own backbone Router-LSA (origination emits that record only when the virtual
// link is Full).
func rootVirtualBackboneAttached(results map[types.AreaID]*Result, root types.RouterID) bool {
	bb := results[types.BackboneArea]
	if bb == nil || bb.Graph == nil {
		return false
	}
	rv := bb.Graph.Routers[root]
	if rv == nil {
		return false
	}
	for _, l := range rv.Links {
		if l.Type == packet.RouterLinkTypeVirtual {
			return true
		}
	}
	return false
}

// transitAreaPass applies RFC 2328 Section 16.3 to the candidate route set. It first
// rewrites any route whose next hop is a virtual link to the real transit next hop (or
// discards it when the neighbor is unreachable in the transit area), then, for each transit
// area with TransitCapability, re-examines that area's Summary-LSAs to lower the metric of
// destinations that are ALREADY reachable. It never adds a destination that was not already
// reachable.
func (c *Computer) transitAreaPass(results map[types.AreaID]*Result, candidates []RouteEntry, virtual []VirtualNeighborResult, maxPaths int) []RouteEntry {
	if subst := virtualNextHopSubst(results[types.BackboneArea], virtual); len(subst) > 0 {
		candidates = rewriteVirtualNextHops(candidates, subst, maxPaths)
	}
	reachable := reachablePrefixes(candidates)
	reader := c.strategy.SummaryReader(c.src)
	for area, res := range results {
		if area == types.BackboneArea || !TransitCapability(res) {
			continue
		}
		c.mTransit.With(area.String()).Inc()
		if reader == nil {
			continue
		}
		for _, s := range reader(area) {
			if s.IsASBR || !s.Prefix.IsValid() || s.AdvertisingRouter == c.root {
				continue
			}
			pfx := s.Prefix.Masked()
			cur, ok := reachable[pfx]
			if !ok {
				continue // improve-only: never make a new destination reachable
			}
			abrNode := res.Nodes[routerVertex(s.AdvertisingRouter)]
			if abrNode == nil {
				continue
			}
			alt := clampMetric(abrNode.Metric, s.Metric)
			if alt >= LSInfinity || alt >= cur {
				continue
			}
			hops := decorateNextHops(abrNode.NextHops, c.resolver, maxPaths)
			if len(hops) == 0 {
				continue
			}
			candidates = append(candidates, RouteEntry{AreaID: area, Prefix: pfx, Metric: alt, Type: RouteInterArea, Origin: s.AdvertisingRouter, NextHops: hops})
		}
	}
	return candidates
}

// virtualNextHopSubst maps each address the backbone SPF assigned to a virtual neighbor to
// the real transit next hops toward that neighbor. An unreachable neighbor maps to an empty
// slice, so a route whose only next hop is that virtual link is discarded, not installed
// with an unroutable next hop (RFC 2328 Section 16.3).
func virtualNextHopSubst(backbone *Result, virtual []VirtualNeighborResult) map[netip.Addr][]NextHop {
	if backbone == nil {
		return nil
	}
	subst := make(map[netip.Addr][]NextHop)
	for _, vr := range virtual {
		bn := backbone.Nodes[routerVertex(vr.Neighbor)]
		if bn == nil {
			continue
		}
		var real []NextHop
		if vr.Reachable {
			real = append([]NextHop(nil), vr.NextHops...)
		}
		for _, nh := range bn.NextHops {
			if nh.Addr.IsValid() {
				subst[nh.Addr] = real
			}
		}
	}
	return subst
}

// rewriteVirtualNextHops replaces any next hop that matches a virtual-link address with the
// resolved transit next hops. A route left with no next hop (an unresolved virtual link) is
// dropped.
// NOTE (#6): a virtual next hop is matched by ADDRESS (the address the backbone SPF assigned
// to the virtual neighbor), which is a heuristic: a distinct route that legitimately shares
// that exact next-hop address would also be rewritten. In practice the virtual neighbor's
// backbone next-hop address is its own transit/global address, not shared with a real
// adjacency, so the match is unambiguous for OSPF's addressing.
func rewriteVirtualNextHops(candidates []RouteEntry, subst map[netip.Addr][]NextHop, maxPaths int) []RouteEntry {
	out := make([]RouteEntry, 0, len(candidates))
	for _, r := range candidates {
		var hops []NextHop
		replaced := false
		for _, nh := range r.NextHops {
			real, isVirtual := subst[nh.Addr]
			if !isVirtual {
				hops = append(hops, nh)
				continue
			}
			replaced = true
			hops = append(hops, real...)
		}
		if replaced {
			hops = capNextHops(hops, maxPaths)
		}
		if len(hops) == 0 {
			continue
		}
		r.NextHops = hops
		out = append(out, r)
	}
	return out
}

// reachablePrefixes returns the best (lowest) metric per already-reachable candidate prefix,
// so the Section 16.3 pass can improve only destinations that are already reachable.
func reachablePrefixes(candidates []RouteEntry) map[netip.Prefix]uint64 {
	best := make(map[netip.Prefix]uint64)
	for _, r := range candidates {
		if !r.Prefix.IsValid() || len(r.NextHops) == 0 || r.Metric >= LSInfinity {
			continue
		}
		p := r.Prefix.Masked()
		if cur, ok := best[p]; !ok || r.Metric < cur {
			best[p] = r.Metric
		}
	}
	return best
}
