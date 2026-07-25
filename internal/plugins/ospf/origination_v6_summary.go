// Design: plan/learned/972-ospf-af-unify.md -- OSPFv3 (IPv6) ABR inter-area summary origination.
// RFC: rfc/short/rfc5340.md (App A.4.5/A.4.6, sec 3.5), rfc/short/rfc2328.md (sec 12.4.3)
//
// v6OriginateSummaries is the IPv6 side of RFC 2328 sec 12.4.3 / RFC 5340 sec 3.5: an area
// border router condenses each attached area's intra-area reachability into the OTHER areas
// as Inter-Area-Prefix-LSAs (networks, RFC 5340 App A.4.5) and Inter-Area-Router-LSAs
// (ASBRs, App A.4.6). It mirrors the OSPFv2 spf.OriginateSummaries algorithm -- collect each
// area's networks + ASBRs, advertise every other area's set into each area, plus the
// backbone's inter-area routes into the non-backbone areas -- but originates address-free
// OSPFv3 LSAs through the same lsdb.OriginateSelf / FlushStaleSelfLSAs seams as the v6 self
// Router / Intra-Area-Prefix LSAs, so sequencing, rate-limiting, install and flooding are
// shared and OSPFv2 is untouched.
//
// Link State ID: unlike OSPFv2 (where the LSID is the summarized network address), an
// OSPFv3 Inter-Area-Prefix/Router-LSA's LSID is an arbitrary unique index (RFC 5340 sec
// 4.4.3.4 -- the 128-bit prefix does not fit a 32-bit ID). It is assigned from the sorted
// desired set so a stable topology yields stable IDs and OriginateSelf floods nothing; a
// changed set reshuffles only the affected trailing indices.
//
// Configured area ranges (aggregation) ARE applied: v6ApplyRanges runs each source area's
// networks through the shared address-family-neutral spf.ApplyAreaRanges, so a configured
// prefix collapses its covered networks into one Inter-Area-Prefix-LSA (RFC 2328 sec 12.4.3).
//
// RFC: rfc/short/rfc5340.md (App A.4.5/A.4.6, sec 3.5), rfc/short/rfc2328.md (sec 12.4.3)

package ospf

import (
	"net/netip"
	"sort"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v6SummarySelfTypes are the OSPFv3 self-LSA types v6OriginateSummaries owns; the
// stale-flush sweeps only these, leaving the self Router / Intra-Area-Prefix LSAs (managed
// by v6OriginateSelf) untouched -- the two type sets are disjoint.
var v6SummarySelfTypes = map[types.LSType]struct{}{
	types.LSType(ospfv3types.LSTypeInterAreaPrefix): {},
	types.LSType(ospfv3types.LSTypeInterAreaRouter): {},
}

// v6SummaryNet is an intra-area IPv6 network and the ABR's cost to reach it.
type v6SummaryNet struct {
	Prefix netip.Prefix
	Metric uint32
}

// v6SummaryRouter is an ASBR reachable in an area and the ABR's cost to reach it.
type v6SummaryRouter struct {
	Router types.RouterID
	Metric uint32
}

// v6OriginateSummaries regenerates this ABR's OSPFv3 inter-area summaries from one SPF pass
// and withdraws any no longer desired. A router that is not an ABR (active in fewer than two
// areas, or not attached to the backbone) withdraws every inter-area summary it previously
// originated (RFC 2328 sec 3.3). It returns the number of LSAs (re)originated/flushed and
// the per-area desired count.
func (e *engine) v6OriginateSummaries(in ospfspf.SummaryInput) ospfspf.SummaryOriginResult {
	if e.lsdb == nil || in.Root == (types.RouterID{}) {
		return ospfspf.SummaryOriginResult{}
	}
	if !ospfspf.IsABR(in.Areas) {
		n := e.lsdb.FlushStaleSelfLSAs(in.Root, v6SummarySelfTypes, nil)
		return ospfspf.SummaryOriginResult{Changed: n, Counts: map[types.AreaID]int{}}
	}

	networks := make(map[types.AreaID][]v6SummaryNet, len(in.Areas))
	asbrs := make(map[types.AreaID][]v6SummaryRouter, len(in.Areas))
	for _, area := range in.Areas {
		res := in.Results[area]
		networks[area] = v6ApplyRanges(v6SummaryNetworks(e.lsdb, res, e.af), in.Ranges[area])
		asbrs[area] = v6SummaryASBRs(res, in.Root)
	}

	keep := make(map[ospflsdb.SelfLSARef]struct{})
	out := ospfspf.SummaryOriginResult{Counts: make(map[types.AreaID]int, len(in.Areas))}
	for _, dst := range in.Areas {
		nets, routers := v6DesiredSummaries(dst, in.Areas, networks, asbrs, in.InterRoutes, in.BorderRouters)
		nets, routers = v6ApplyAreaTypePolicy(nets, routers, in.Policies[dst])
		opts := neutralToV6Options(in.Options[dst])
		out.Changed += e.v6OriginateAreaSummaries(dst, in.Root, opts, nets, routers, keep)
		out.Counts[dst] = len(nets) + len(routers)
	}
	out.Changed += e.lsdb.FlushStaleSelfLSAs(in.Root, v6SummarySelfTypes, keep)
	return out
}

// v6SummaryNetworks collects an area's intra-area IPv6 prefixes for Inter-Area-Prefix
// origination: every Intra-Area-Prefix-LSA (RFC 5340 App A.4.7) referencing a vertex reached
// in the area's SPF tree, INCLUDING the root's own prefixes (the ABR summarizes its own
// directly-connected networks into other areas). The cost is the SPF cost to the referenced
// vertex plus the per-prefix metric; duplicate prefixes keep the lowest cost. This differs
// from v6BuildRoutes only in admitting the root vertex (whose next-hop set is empty).
func v6SummaryNetworks(src ospfspf.Source, res *ospfspf.Result, af addressFamily) []v6SummaryNet {
	if src == nil || res == nil {
		return nil
	}
	best := make(map[netip.Prefix]uint32)
	for _, h := range src.Summary(res.Area) {
		if h.Age.IsMaxAge() || ospfv3types.LSType(h.Type) != ospfv3types.LSTypeIntraAreaPrefix {
			continue
		}
		lsa, ok := src.LookupLSA(res.Area, h.Key())
		if !ok {
			continue
		}
		decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
		if err != nil {
			continue
		}
		body, err := decoded.DecodeIntraAreaPrefix()
		if err != nil {
			continue
		}
		var nr *ospfspf.NodeResult
		switch body.ReferencedLSType {
		case ospfv3types.LSTypeRouter:
			nr = res.Nodes[ospfspf.VertexID{Kind: ospfspf.VertexRouter, Router: types.RouterID(body.ReferencedAdvRouter)}]
		case ospfv3types.LSTypeNetwork:
			if vid, ok := v6NetworkVertexRef(res, types.RouterID(body.ReferencedAdvRouter), types.LinkStateID(body.ReferencedLinkStateID)); ok {
				nr = res.Nodes[vid]
			}
		default:
			continue
		}
		if nr == nil {
			continue
		}
		for _, p := range body.Prefixes {
			pfx, ok := v6PrefixToNetip(p, af)
			if !ok {
				continue
			}
			metric := nr.Metric + uint64(p.Field16)
			if metric >= ospfspf.LSInfinity {
				continue
			}
			if cur, exists := best[pfx]; !exists || metric < uint64(cur) {
				best[pfx] = uint32(metric)
			}
		}
	}
	out := make([]v6SummaryNet, 0, len(best))
	for pfx, m := range best {
		out = append(out, v6SummaryNet{Prefix: pfx, Metric: m})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Prefix.Compare(out[j].Prefix) < 0 })
	return out
}

// v6ApplyRanges aggregates an area's intra-area networks into its configured area ranges via
// the shared address-family-neutral spf.ApplyAreaRanges (RFC 2328 sec 12.4.3), so an OSPFv3
// ABR can summarize many /64s into one configured prefix instead of advertising each
// Inter-Area-Prefix-LSA individually. No ranges configured for the area -> unchanged.
func v6ApplyRanges(nets []v6SummaryNet, ranges []ospfspf.AreaRange) []v6SummaryNet {
	if len(ranges) == 0 || len(nets) == 0 {
		return nets
	}
	in := make([]ospfspf.RangeInput, len(nets))
	for i, n := range nets {
		in[i] = ospfspf.RangeInput{Prefix: n.Prefix, Metric: uint64(n.Metric)}
	}
	out := ospfspf.ApplyAreaRanges(in, ranges)
	res := make([]v6SummaryNet, 0, len(out))
	for _, r := range out {
		if r.Metric >= ospfspf.LSInfinity {
			continue
		}
		res = append(res, v6SummaryNet{Prefix: r.Prefix, Metric: uint32(r.Metric)})
	}
	return res
}

// v6SummaryASBRs collects the ASBRs reached in an area (other than the calculating router)
// for Inter-Area-Router origination. An ASBR is a router vertex whose Router-LSA has the
// E-bit set (RFC 5340 App A.4.3 -- the bit position matches OSPFv2, so packet.RouterFlagE
// applies to the shared graph's flags). Mirrors spf.collectSummaryASBRs for the v6 graph.
func v6SummaryASBRs(res *ospfspf.Result, root types.RouterID) []v6SummaryRouter {
	if res == nil || res.Graph == nil {
		return nil
	}
	var out []v6SummaryRouter
	for id, nr := range res.Nodes {
		if id.Kind != ospfspf.VertexRouter || id.Router == root || nr == nil {
			continue
		}
		r := res.Graph.Routers[id.Router]
		if r == nil || r.Flags&packet.RouterFlagE == 0 || nr.Metric >= ospfspf.LSInfinity {
			continue
		}
		out = append(out, v6SummaryRouter{Router: id.Router, Metric: uint32(nr.Metric)})
	}
	sort.Slice(out, func(i, j int) bool { return v6CompareRID(out[i].Router, out[j].Router) < 0 })
	return out
}

// v6DesiredSummaries builds the inter-area prefixes and ASBRs this ABR advertises INTO area
// dst: every other attached area's networks + ASBRs (RFC 2328 sec 12.4.3), and -- for the
// non-backbone areas -- the inter-area routes and ASBRs learned through the backbone (so a
// transit ABR re-advertises Area 0's summaries downward). Each network keeps its lowest
// cost; each ASBR its lowest cost. Self-summaries are never re-imported because
// spf.ComputeInterAreaWith skips LSAs advertised by the root.
func v6DesiredSummaries(dst types.AreaID, areas []types.AreaID, networks map[types.AreaID][]v6SummaryNet, asbrs map[types.AreaID][]v6SummaryRouter, interRoutes []ospfspf.RouteEntry, borderRouters []ospfspf.BorderRouterEntry) ([]v6SummaryNet, []v6SummaryRouter) {
	netBest := make(map[netip.Prefix]uint32)
	rtrBest := make(map[types.RouterID]uint32)
	addNet := func(pfx netip.Prefix, m uint32) {
		if uint64(m) >= ospfspf.LSInfinity {
			return
		}
		if cur, ok := netBest[pfx]; !ok || m < cur {
			netBest[pfx] = m
		}
	}
	addRtr := func(rid types.RouterID, m uint32) {
		if rid == (types.RouterID{}) || uint64(m) >= ospfspf.LSInfinity {
			return
		}
		if cur, ok := rtrBest[rid]; !ok || m < cur {
			rtrBest[rid] = m
		}
	}
	for _, src := range areas {
		if src == dst {
			continue
		}
		for _, n := range networks[src] {
			addNet(n.Prefix, n.Metric)
		}
		for _, a := range asbrs[src] {
			addRtr(a.Router, a.Metric)
		}
	}
	if dst != types.BackboneArea {
		for _, r := range interRoutes {
			if r.Type != ospfspf.RouteInterArea || r.AreaID != types.BackboneArea || r.Metric >= ospfspf.LSInfinity || !r.Prefix.Addr().Is6() {
				continue
			}
			addNet(r.Prefix, uint32(r.Metric))
		}
		for _, b := range borderRouters {
			if b.Kind != ospfspf.BorderRouterASBR || b.AreaID != types.BackboneArea || b.Metric >= ospfspf.LSInfinity {
				continue
			}
			addRtr(b.RouterID, uint32(b.Metric))
		}
	}
	nets := make([]v6SummaryNet, 0, len(netBest))
	for pfx, m := range netBest {
		nets = append(nets, v6SummaryNet{Prefix: pfx, Metric: m})
	}
	sort.Slice(nets, func(i, j int) bool { return nets[i].Prefix.Compare(nets[j].Prefix) < 0 })
	routers := make([]v6SummaryRouter, 0, len(rtrBest))
	for rid, m := range rtrBest {
		routers = append(routers, v6SummaryRouter{Router: rid, Metric: m})
	}
	sort.Slice(routers, func(i, j int) bool { return v6CompareRID(routers[i].Router, routers[j].Router) < 0 })
	return nets, routers
}

// v6OriginateAreaSummaries originates this ABR's Inter-Area-Prefix-LSAs and
// Inter-Area-Router-LSAs into one area, assigning each a sequential Link State ID from the
// sorted desired set, and records every (area,key) it originated in keep for the caller's
// stale-flush. It returns the number of LSAs (re)originated (an unchanged set re-originates
// nothing).
func (e *engine) v6OriginateAreaSummaries(area types.AreaID, router types.RouterID, opts ospfv3types.Options, nets []v6SummaryNet, routers []v6SummaryRouter, keep map[ospflsdb.SelfLSARef]struct{}) int {
	changed := 0
	var lsid uint32
	for _, n := range nets {
		prefix, ok := netipToV6Prefix(n.Prefix, 0) // Field16 reserved (0) in Inter-Area-Prefix
		if !ok {
			continue
		}
		lsid++
		key, originated := e.v6OriginateInterAreaPrefix(area, router, lsid, ospfv3packet.InterAreaPrefixLSA{Metric: n.Metric, Prefix: prefix})
		if originated {
			changed++
		}
		keep[ospflsdb.SelfLSARef{Area: area, Key: key}] = struct{}{}
	}
	lsid = 0
	for _, r := range routers {
		lsid++
		key, originated := e.v6OriginateInterAreaRouter(area, router, lsid, ospfv3packet.InterAreaRouterLSA{Options: opts, Metric: r.Metric, DestinationRouter: ospfv3types.RouterID(r.Router)})
		if originated {
			changed++
		}
		keep[ospflsdb.SelfLSARef{Area: area, Key: key}] = struct{}{}
	}
	return changed
}

// v6OriginateInterAreaPrefix originates one Inter-Area-Prefix-LSA with the given sequential
// Link State ID and returns its LSDB key (for the stale-flush keep set) and whether it was
// (re)originated.
func (e *engine) v6OriginateInterAreaPrefix(area types.AreaID, router types.RouterID, lsid uint32, body ospfv3packet.InterAreaPrefixLSA) (types.LSAKey, bool) {
	id := v6SummaryLSID(lsid)
	key := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeInterAreaPrefix), LinkStateID: id, AdvertisingRouter: router}
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	b := body
	_, ok := e.lsdb.OriginateSelf(area, key, bodyBytes, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header:       v6OriginHeader(ospfv3types.LSTypeInterAreaPrefix, ospfv3types.LinkStateID(id), router, seq, purge),
			InterAreaPfx: &b,
		})
	})
	return key, ok
}

// v6OriginateInterAreaRouter originates one Inter-Area-Router-LSA with the given sequential
// Link State ID and returns its LSDB key and whether it was (re)originated.
func (e *engine) v6OriginateInterAreaRouter(area types.AreaID, router types.RouterID, lsid uint32, body ospfv3packet.InterAreaRouterLSA) (types.LSAKey, bool) {
	id := v6SummaryLSID(lsid)
	key := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeInterAreaRouter), LinkStateID: id, AdvertisingRouter: router}
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	b := body
	_, ok := e.lsdb.OriginateSelf(area, key, bodyBytes, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header:       v6OriginHeader(ospfv3types.LSTypeInterAreaRouter, ospfv3types.LinkStateID(id), router, seq, purge),
			InterAreaRtr: &b,
		})
	})
	return key, ok
}

// v6SummaryLSID encodes a sequential index as an OSPFv3 Link State ID (RFC 5340 sec
// 4.4.3.4: an arbitrary unique value the originating router assigns).
func v6SummaryLSID(v uint32) types.LinkStateID {
	return types.LinkStateID{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

// v6CompareRID orders two Router IDs by their big-endian bytes.
func v6CompareRID(a, b types.RouterID) int {
	for i := range a {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}
