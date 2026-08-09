// Design: docs/architecture/ospf/ospf-9-inter-area-abr.md -- Type 3/4 Summary-LSA origination.
// RFC: rfc/short/rfc2328.md (sec 12.4.3 Summary-LSA origination + area ranges)
//
// ABRs originate Summary-LSAs for inter-area destinations, aggregate configured area ranges,
// and withdraw stale summaries by premature aging.

package spf

import (
	"net/netip"
	"sort"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// SummarySink is the narrow LSDB write API needed for ABR Summary-LSA
// origination. The OSPF LSDB implements it; tests can provide a hand-built sink.
type SummarySink interface {
	OriginateSummary(types.AreaID, types.RouterID, types.Options, types.LSType, types.LinkStateID, [4]byte, uint32) (packet.LSAHeader, bool)
	FlushStaleSummaryLSAs(types.AreaID, types.RouterID, map[types.LSAKey]struct{}) int
}

// SummaryInput is one ABR Summary-LSA origination pass. Areas is the active
// attachment set used for ABR decisions and new summaries; FlushAreas extends
// stale-summary cleanup to areas that were previously active or still configured.
type SummaryInput struct {
	Sink          SummarySink
	Root          types.RouterID
	Areas         []types.AreaID
	FlushAreas    []types.AreaID
	Options       map[types.AreaID]types.Options
	Ranges        map[types.AreaID][]AreaRange
	Results       map[types.AreaID]*Result
	InterRoutes   []RouteEntry
	BorderRouters []BorderRouterEntry
	// Policies carries the per-destination-area stub/NSSA origination policy (RFC 2328
	// sec 3.6 / RFC 3101). A missing entry is treated as a normal area.
	Policies map[types.AreaID]AreaSummaryPolicy
}

type summaryNetwork struct {
	Prefix netip.Prefix
	Metric uint64
}

type summaryASBR struct {
	Router types.RouterID
	Metric uint64
}

type summaryDesired struct {
	Type   types.LSType
	LSID   types.LinkStateID
	Mask   [4]byte
	Metric uint32
}

// SummaryOriginResult reports the changes and current desired summary count from
// one origination pass.
type SummaryOriginResult struct {
	Changed int
	Counts  map[types.AreaID]int
}

// OriginateSummaries originates Type 3/4 LSAs for an ABR and flushes stale self
// summaries. RFC 2328 Section 3.3 requires a real backbone attachment; a
// non-ABR flushes any summaries it previously originated.
func OriginateSummaries(in SummaryInput) SummaryOriginResult {
	if in.Sink == nil || in.Root == (types.RouterID{}) {
		return SummaryOriginResult{}
	}
	areas := canonicalAreas(in.Areas)
	flushAreas := canonicalAreas(append(append([]types.AreaID(nil), areas...), in.FlushAreas...))
	if !IsABR(areas) {
		res := SummaryOriginResult{Counts: make(map[types.AreaID]int, len(flushAreas))}
		for _, area := range flushAreas {
			res.Changed += in.Sink.FlushStaleSummaryLSAs(area, in.Root, nil)
			res.Counts[area] = 0
		}
		return res
	}

	networks := make(map[types.AreaID][]summaryNetwork, len(areas))
	asbrs := make(map[types.AreaID][]summaryASBR, len(areas))
	for _, area := range areas {
		res := in.Results[area]
		networks[area] = applyAreaRanges(collectSummaryNetworks(res), in.Ranges[area])
		asbrs[area] = collectSummaryASBRs(res, in.Root)
	}

	desiredByArea := make(map[types.AreaID]map[types.LSAKey]struct{}, len(flushAreas))
	res := SummaryOriginResult{Counts: make(map[types.AreaID]int, len(flushAreas))}
	for _, dst := range areas {
		desired := summaryDesiredForArea(dst, areas, networks, asbrs, in.InterRoutes, in.BorderRouters)
		desired = applyAreaTypePolicy(desired, in.Policies[dst])
		desired = assignSummaryLSIDs(dedupeSummaryDesired(desired))
		keep := make(map[types.LSAKey]struct{}, len(desired))
		for _, item := range desired {
			key := types.LSAKey{Type: item.Type, LinkStateID: item.LSID, AdvertisingRouter: in.Root}
			keep[key] = struct{}{}
			if _, ok := in.Sink.OriginateSummary(dst, in.Root, in.Options[dst], item.Type, item.LSID, item.Mask, item.Metric); ok {
				res.Changed++
			}
		}
		desiredByArea[dst] = keep
		res.Counts[dst] = len(keep)
	}
	for _, area := range flushAreas {
		res.Changed += in.Sink.FlushStaleSummaryLSAs(area, in.Root, desiredByArea[area])
	}
	return res
}

func summaryDesiredForArea(dst types.AreaID, areas []types.AreaID, networks map[types.AreaID][]summaryNetwork, asbrs map[types.AreaID][]summaryASBR, interRoutes []RouteEntry, borderRouters []BorderRouterEntry) []summaryDesired {
	var out []summaryDesired
	for _, src := range areas {
		if src == dst {
			continue
		}
		for _, n := range networks[src] {
			if n.Metric >= LSInfinity {
				continue
			}
			out = append(out, summaryDesired{Type: types.LSTypeSummaryNetwork, LSID: types.LinkStateID(n.Prefix.Addr().As4()), Mask: maskFromBits(n.Prefix.Bits()), Metric: uint32(n.Metric)})
		}
		for _, a := range asbrs[src] {
			if a.Metric >= LSInfinity || a.Router == (types.RouterID{}) {
				continue
			}
			out = append(out, summaryDesired{Type: types.LSTypeSummaryASBR, LSID: types.LinkStateID(a.Router), Metric: uint32(a.Metric)})
		}
	}
	if dst != types.BackboneArea {
		for _, r := range interRoutes {
			if r.Type != RouteInterArea || r.AreaID != types.BackboneArea || r.Metric >= LSInfinity {
				continue
			}
			out = append(out, summaryDesired{Type: types.LSTypeSummaryNetwork, LSID: types.LinkStateID(r.Prefix.Addr().As4()), Mask: maskFromBits(r.Prefix.Bits()), Metric: uint32(r.Metric)})
		}
		for _, b := range borderRouters {
			if b.Kind != BorderRouterASBR || b.AreaID != types.BackboneArea || b.Metric >= LSInfinity || b.RouterID == (types.RouterID{}) {
				continue
			}
			out = append(out, summaryDesired{Type: types.LSTypeSummaryASBR, LSID: types.LinkStateID(b.RouterID), Metric: uint32(b.Metric)})
		}
	}
	return out
}

type summaryDesiredKey struct {
	typ  types.LSType
	lsid types.LinkStateID
	mask [4]byte
}

func dedupeSummaryDesired(in []summaryDesired) []summaryDesired {
	if len(in) < 2 {
		return in
	}
	best := make(map[summaryDesiredKey]summaryDesired, len(in))
	for _, item := range in {
		key := summaryDesiredKey{typ: item.Type, lsid: item.LSID, mask: item.Mask}
		cur, ok := best[key]
		if !ok || item.Metric < cur.Metric {
			best[key] = item
		}
	}
	out := make([]summaryDesired, 0, len(best))
	for _, item := range best {
		out = append(out, item)
	}
	return out
}

// collectSummaryNetworks gathers an area's intra-area network prefixes for ABR
// Type 3 origination: stub links from every reached router vertex (including the
// root's own connected stub networks) plus transit (broadcast LAN) networks
// reached in the SPF tree (RFC 2328 Section 16.1 step (4)). Unlike route install,
// the ABR summarizes its own directly-connected networks, so transit vertices are
// included regardless of next-hop presence.
func collectSummaryNetworks(res *Result) []summaryNetwork {
	if res == nil || res.Graph == nil {
		return nil
	}
	best := make(map[netip.Prefix]summaryNetwork)
	add := func(pfx netip.Prefix, metric uint64) {
		if metric >= LSInfinity {
			return
		}
		if cur, exists := best[pfx]; !exists || metric < cur.Metric {
			best[pfx] = summaryNetwork{Prefix: pfx, Metric: metric}
		}
	}
	for id, nr := range res.Nodes {
		if nr == nil {
			continue
		}
		switch id.Kind {
		case VertexRouter:
			r := res.Graph.Routers[id.Router]
			if r == nil {
				continue
			}
			for _, link := range r.Links {
				if link.Type != packet.RouterLinkTypeStub {
					continue
				}
				pfx, ok := stubPrefix(link.LinkID, link.LinkData)
				if !ok {
					continue
				}
				add(pfx, clampMetric(nr.Metric, uint64(link.Metric)))
			}
		case VertexNetwork:
			nv := res.Graph.Networks[id.Network]
			if nv == nil {
				continue
			}
			pfx, ok := stubPrefix(nv.ID, nv.NetworkMask)
			if !ok {
				continue
			}
			add(pfx, nr.Metric)
		}
	}
	out := make([]summaryNetwork, 0, len(best))
	for _, n := range best {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Prefix.Compare(out[j].Prefix) < 0 })
	return out
}

func collectSummaryASBRs(res *Result, root types.RouterID) []summaryASBR {
	if res == nil || res.Graph == nil {
		return nil
	}
	var out []summaryASBR
	for id, nr := range res.Nodes {
		if id.Kind != VertexRouter || id.Router == root || nr == nil {
			continue
		}
		r := res.Graph.Routers[id.Router]
		if r == nil || r.Flags&packet.RouterFlagE == 0 || nr.Metric >= LSInfinity {
			continue
		}
		out = append(out, summaryASBR{Router: id.Router, Metric: nr.Metric})
	}
	sort.Slice(out, func(i, j int) bool { return compare4(out[i].Router, out[j].Router) < 0 })
	return out
}

// applyAreaRanges is the OSPFv2 adapter over the address-family-neutral ApplyAreaRanges.
func applyAreaRanges(in []summaryNetwork, ranges []AreaRange) []summaryNetwork {
	if len(in) == 0 || len(ranges) == 0 {
		return in
	}
	rin := make([]RangeInput, len(in))
	for i, n := range in {
		rin[i] = RangeInput(n)
	}
	out := ApplyAreaRanges(rin, ranges)
	res := make([]summaryNetwork, len(out))
	for i, r := range out {
		res[i] = summaryNetwork(r)
	}
	return res
}

// RangeInput is one (prefix, metric) intra-area network for ApplyAreaRanges.
type RangeInput struct {
	Prefix netip.Prefix
	Metric uint64
}

// ApplyAreaRanges aggregates an ABR's intra-area networks into the configured area ranges
// (RFC 2328 sec 12.4.3). It is address-family-neutral -- the OSPFv2 Summary-LSA and OSPFv3
// Inter-Area-Prefix-LSA origination share it: a network covered by an advertised range is
// replaced by the range (cost = the range's configured cost, else the maximum covered
// metric); an unadvertised matching range suppresses its covered networks; uncovered
// networks pass through unchanged. Ranges apply longest-prefix first so a more specific range
// wins. A range and the networks it covers must be the same address family (rangeCovers).
func ApplyAreaRanges(in []RangeInput, ranges []AreaRange) []RangeInput {
	if len(in) == 0 || len(ranges) == 0 {
		return in
	}
	ranges = append([]AreaRange(nil), ranges...)
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].Prefix.Bits() != ranges[j].Prefix.Bits() {
			return ranges[i].Prefix.Bits() > ranges[j].Prefix.Bits()
		}
		return ranges[i].Prefix.Addr().Compare(ranges[j].Prefix.Addr()) < 0
	})
	covered := make([]bool, len(in))
	var out []RangeInput
	for _, r := range ranges {
		if !r.Prefix.IsValid() {
			continue
		}
		var matched bool
		var metric uint64
		for i, n := range in {
			if covered[i] || !rangeCovers(r.Prefix, n.Prefix) {
				continue
			}
			covered[i] = true
			matched = true
			if n.Metric > metric {
				metric = n.Metric
			}
		}
		if !matched || !r.Advertise {
			continue
		}
		if r.HasCost {
			metric = uint64(r.Cost)
		}
		if metric < LSInfinity {
			out = append(out, RangeInput{Prefix: r.Prefix.Masked(), Metric: metric})
		}
	}
	for i, n := range in {
		if !covered[i] {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Prefix.Compare(out[j].Prefix) < 0 })
	return out
}

// rangeCovers reports whether range r aggregates prefix p: p is at least as specific and r
// contains it. Cross-family pairs never match (netip Contains returns false), so the same
// check is safe for both IPv4 and IPv6.
func rangeCovers(r, p netip.Prefix) bool {
	if !r.IsValid() || !p.IsValid() {
		return false
	}
	return p.Bits() >= r.Bits() && r.Contains(p.Addr())
}

func assignSummaryLSIDs(in []summaryDesired) []summaryDesired {
	sort.Slice(in, func(i, j int) bool {
		if in[i].Type != in[j].Type {
			return in[i].Type < in[j].Type
		}
		if compare4(in[i].LSID, in[j].LSID) != 0 {
			return compare4(in[i].LSID, in[j].LSID) < 0
		}
		return compare4(in[i].Mask, in[j].Mask) < 0
	})
	used := make(map[types.LSAKey]struct{}, len(in))
	for i := range in {
		key := types.LSAKey{Type: in[i].Type, LinkStateID: in[i].LSID}
		if _, ok := used[key]; ok {
			in[i].LSID = nextFreeLSID(in[i].LSID, in[i].Type, used)
			key.LinkStateID = in[i].LSID
		}
		used[key] = struct{}{}
	}
	return in
}

func nextFreeLSID(base types.LinkStateID, typ types.LSType, used map[types.LSAKey]struct{}) types.LinkStateID {
	v := uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
	for {
		v++
		cand := types.LinkStateID{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
		if _, ok := used[types.LSAKey{Type: typ, LinkStateID: cand}]; !ok {
			return cand
		}
	}
}

func maskFromBits(bits int) [4]byte {
	var out [4]byte
	for i := 0; i < bits && i < 32; i++ {
		out[i/8] |= 1 << (7 - uint(i%8))
	}
	return out
}
