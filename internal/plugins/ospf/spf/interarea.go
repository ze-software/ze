// Design: plan/learned/963-ospf-9-inter-area-abr.md -- inter-area SPF and ABR snapshots.
// RFC 2328 Sections 3.3, 16.2, and 16.3: ABR detection, inter-area cost
// composition, and the ABR backbone-only summary acceptance rule.

package spf

import (
	"net/netip"
	"sort"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// AreaRange is the OSPF area range used by ABRs when summarizing Type 3 LSAs.
type AreaRange struct {
	Prefix    netip.Prefix
	Advertise bool
	Cost      uint32
	HasCost   bool
}

// AreaConfig is the SPF-owned view of an OSPF area.
type AreaConfig struct {
	AreaID      types.AreaID
	Options     types.Options
	Ranges      []AreaRange
	AreaType    string // normal | stub | nssa
	NoSummary   bool   // totally-stubby / totally-NSSA
	DefaultCost uint32 // stub/NSSA injected default metric
}

// BorderRouterKind identifies the border-router snapshot role.
type BorderRouterKind string

const (
	BorderRouterABR  BorderRouterKind = "abr"
	BorderRouterASBR BorderRouterKind = "asbr"
)

// BorderRouterEntry is one reachable ABR/ASBR before JSON snapshot rendering.
type BorderRouterEntry struct {
	RouterID types.RouterID
	AreaID   types.AreaID
	Kind     BorderRouterKind
	Metric   uint64
	NextHops []NextHop
}

// BorderRouterSnapshotEntry is one `show ospf border-routers` row.
type BorderRouterSnapshotEntry struct {
	RouterID string             `json:"router_id"`
	Area     string             `json:"area"`
	Kind     string             `json:"kind"`
	Metric   uint64             `json:"metric"`
	NextHops []RouteSnapshotHop `json:"next_hops"`
}

// RFC 2328 Section 3.3: an area border router is attached to multiple areas and
// one of them is the backbone. Virtual-link backbone repair is out of scope.
func IsABR(areas []types.AreaID) bool {
	seen := make(map[types.AreaID]struct{}, len(areas))
	for _, area := range areas {
		seen[area] = struct{}{}
	}
	if len(seen) < 2 {
		return false
	}
	_, ok := seen[types.BackboneArea]
	return ok
}

// InterAreaInput is one inter-area computation pass over the already-computed
// intra-area SPF results.
type InterAreaInput struct {
	Source   Source
	Root     types.RouterID
	Areas    []types.AreaID
	Results  map[types.AreaID]*Result
	Ranges   map[types.AreaID][]AreaRange
	Resolver InterfaceResolver
	MaxPaths int
}

// InterAreaSummary is one decoded summary record an ABR advertises into an area: an
// inter-area network prefix or (when IsASBR) an inter-area ASBR reachability. The
// address-family-specific decode (OSPFv2 Type 3/4 Summary-LSAs vs OSPFv3
// Inter-Area-Prefix-LSA 0x2003 / Inter-Area-Router-LSA 0x2004) yields these; the shared
// computation handles ABR reachability, metric composition, ranges and selection.
type InterAreaSummary struct {
	AdvertisingRouter types.RouterID
	Metric            uint64
	IsASBR            bool
	Prefix            netip.Prefix   // the inter-area network (when !IsASBR)
	ASBR              types.RouterID // the summarized ASBR (when IsASBR)
}

// SummaryReader yields the inter-area summaries an area carries, decoded for the address
// family. It is the only address-family-specific part of the inter-area computation.
type SummaryReader func(area types.AreaID) []InterAreaSummary

// ComputeInterArea computes RFC 2328 Section 16.2 inter-area routes from OSPFv2 Type 3/4
// Summary-LSAs. RFC 2328 Section 16.3: when the calculating router is an ABR, only
// backbone summaries are accepted for route computation.
func ComputeInterArea(in InterAreaInput) ([]RouteEntry, []BorderRouterEntry) {
	return ComputeInterAreaWith(in, v4SummaryReader(in.Source))
}

// ComputeInterAreaWith is ComputeInterArea parameterized by an address-family summary
// reader, so the OSPFv3 strategy can supply Inter-Area-Prefix / Inter-Area-Router decode
// (RFC 5340 App A.4.10/A.4.11) while sharing the ABR reachability, metric composition
// (RFC 2328 sec 16.2), area-range suppression and border-router selection.
func ComputeInterAreaWith(in InterAreaInput, read SummaryReader) ([]RouteEntry, []BorderRouterEntry) {
	maxPaths := in.MaxPaths
	if maxPaths <= 0 {
		maxPaths = DefaultMaxPaths
	}
	areas := canonicalAreas(in.Areas)
	abr := IsABR(areas)
	var candidates []RouteEntry
	var border []BorderRouterEntry
	for _, area := range areas {
		res := in.Results[area]
		if res == nil || res.Graph == nil {
			continue
		}
		border = append(border, intraAreaBorderRouters(res, in.Root, in.Resolver, maxPaths)...)
		if abr && area != types.BackboneArea {
			continue
		}
		if read == nil {
			continue
		}
		for _, s := range read(area) {
			if s.AdvertisingRouter == in.Root {
				continue
			}
			abrNode := res.Nodes[routerVertex(s.AdvertisingRouter)]
			if abrNode == nil {
				continue
			}
			nextHops := decorateNextHops(abrNode.NextHops, in.Resolver, maxPaths)
			if len(nextHops) == 0 {
				continue
			}
			metric := clampMetric(abrNode.Metric, s.Metric)
			if metric >= LSInfinity {
				continue
			}
			if s.IsASBR {
				border = append(border, BorderRouterEntry{RouterID: s.ASBR, AreaID: area, Kind: BorderRouterASBR, Metric: metric, NextHops: nextHops})
				continue
			}
			if !s.Prefix.IsValid() || isConfiguredAreaRange(s.Prefix, in.Ranges[area]) {
				continue
			}
			candidates = append(candidates, RouteEntry{AreaID: area, Prefix: s.Prefix, Metric: metric, Type: RouteInterArea, Origin: s.AdvertisingRouter, NextHops: nextHops})
		}
	}
	return candidates, selectBorderRouters(border, maxPaths)
}

// v4SummaryReader decodes OSPFv2 Type 3 (network) and Type 4 (ASBR) Summary-LSAs into the
// address-family-neutral InterAreaSummary records.
func v4SummaryReader(src Source) SummaryReader {
	return func(area types.AreaID) []InterAreaSummary {
		if src == nil {
			return nil
		}
		var out []InterAreaSummary
		for _, h := range src.Summary(area) {
			if h.Age.IsMaxAge() {
				continue
			}
			if h.Type != types.LSTypeSummaryNetwork && h.Type != types.LSTypeSummaryASBR {
				continue
			}
			lsa, ok := src.LookupLSA(area, h.Key())
			if !ok {
				continue
			}
			body, err := summaryBody(lsa)
			if err != nil || body.TOS != 0 || uint64(body.Metric) >= LSInfinity {
				continue
			}
			if h.Type == types.LSTypeSummaryASBR {
				out = append(out, InterAreaSummary{AdvertisingRouter: h.AdvertisingRouter, Metric: uint64(body.Metric), IsASBR: true, ASBR: types.RouterID(h.LinkStateID)})
				continue
			}
			pfx, ok := summaryPrefix(h.LinkStateID, body.NetworkMask)
			if !ok {
				continue
			}
			out = append(out, InterAreaSummary{AdvertisingRouter: h.AdvertisingRouter, Metric: uint64(body.Metric), Prefix: pfx})
		}
		return out
	}
}

func summaryBody(lsa packet.LSA) (packet.SummaryLSA, error) {
	if lsa.Summary != nil {
		return *lsa.Summary, nil
	}
	return lsa.DecodeSummary()
}

func summaryPrefix(id types.LinkStateID, mask [4]byte) (netip.Prefix, bool) {
	bits, ok := maskPrefixLen(mask)
	if !ok {
		return netip.Prefix{}, false
	}
	addr := netip.AddrFrom4([4]byte(id))
	return netip.PrefixFrom(addr, bits).Masked(), true
}

// isConfiguredAreaRange reports whether pfx falls WITHIN any configured area range, so the
// ABR suppresses the component summary (RFC 2328 sec 12.4.3): a range aggregates every more-
// specific network it covers, not only an exact match. A 10.0.0.0/16 range therefore
// suppresses an accepted 10.0.5.0/24 inter-area summary.
func isConfiguredAreaRange(pfx netip.Prefix, ranges []AreaRange) bool {
	for _, r := range ranges {
		if rangeCovers(r.Prefix, pfx) {
			return true
		}
	}
	return false
}

func intraAreaBorderRouters(res *Result, root types.RouterID, resolver InterfaceResolver, maxPaths int) []BorderRouterEntry {
	var out []BorderRouterEntry
	for id, nr := range res.Nodes {
		if id.Kind != VertexRouter || id.Router == root || nr == nil {
			continue
		}
		rv := res.Graph.Routers[id.Router]
		if rv == nil {
			continue
		}
		nextHops := decorateNextHops(nr.NextHops, resolver, maxPaths)
		if len(nextHops) == 0 {
			continue
		}
		if rv.Flags&packet.RouterFlagB != 0 {
			out = append(out, BorderRouterEntry{RouterID: id.Router, AreaID: res.Area, Kind: BorderRouterABR, Metric: nr.Metric, NextHops: nextHops})
		}
		if rv.Flags&packet.RouterFlagE != 0 {
			out = append(out, BorderRouterEntry{RouterID: id.Router, AreaID: res.Area, Kind: BorderRouterASBR, Metric: nr.Metric, NextHops: nextHops})
		}
	}
	return out
}

type borderKey struct {
	kind BorderRouterKind
	rid  types.RouterID
}

func selectBorderRouters(in []BorderRouterEntry, maxPaths int) []BorderRouterEntry {
	if maxPaths <= 0 {
		maxPaths = DefaultMaxPaths
	}
	best := make(map[borderKey]BorderRouterEntry)
	for _, b := range in {
		if b.RouterID == (types.RouterID{}) || len(b.NextHops) == 0 || b.Metric >= LSInfinity {
			continue
		}
		b.NextHops = capNextHops(b.NextHops, maxPaths)
		sortNextHops(b.NextHops)
		k := borderKey{kind: b.Kind, rid: b.RouterID}
		cur, ok := best[k]
		switch {
		case !ok || b.Metric < cur.Metric:
			best[k] = b
		case b.Metric == cur.Metric:
			cur.NextHops, _ = mergeNextHops(cur.NextHops, b.NextHops, maxPaths)
			if compare4(b.AreaID, cur.AreaID) < 0 {
				cur.AreaID = b.AreaID
			}
			best[k] = cur
		}
	}
	out := make([]BorderRouterEntry, 0, len(best))
	for _, b := range best {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return compare4(out[i].RouterID, out[j].RouterID) < 0
	})
	return out
}

// BorderRouterSnapshot renders ABR/ASBR rows as stable values for the later CLI.
func BorderRouterSnapshot(rows []BorderRouterEntry) []BorderRouterSnapshotEntry {
	out := make([]BorderRouterSnapshotEntry, 0, len(rows))
	for _, row := range rows {
		hops := make([]RouteSnapshotHop, 0, len(row.NextHops))
		for _, nh := range row.NextHops {
			hops = append(hops, RouteSnapshotHop{NextHop: nh.Addr.String(), Interface: nh.Interface})
		}
		out = append(out, BorderRouterSnapshotEntry{RouterID: row.RouterID.String(), Area: row.AreaID.String(), Kind: string(row.Kind), Metric: row.Metric, NextHops: hops})
	}
	return out
}

func canonicalAreas(areas []types.AreaID) []types.AreaID {
	seen := make(map[types.AreaID]struct{}, len(areas))
	for _, area := range areas {
		seen[area] = struct{}{}
	}
	out := make([]types.AreaID, 0, len(seen))
	for area := range seen {
		out = append(out, area)
	}
	sort.Slice(out, func(i, j int) bool { return compare4(out[i], out[j]) < 0 })
	return out
}
