// Design: plan/learned/972-ospf-af-unify.md -- Phase 4: the AF prefix-strategy seam.
//
// RFC 5340 keeps OSPF's algorithms identical across address families and changes
// only the encodings and the prefix model. OSPFv2 carries prefixes inside the
// Router-LSA (stub links) and Network-LSA (mask) and decodes router/network
// adjacency from those same v2 LSA bodies; OSPFv3 (RFC 5340 App A.4) makes the
// Router/Network LSAs address-free and carries prefixes in Intra-Area-Prefix and
// Link LSAs. So the graph-adjacency decode and the prefix attachment are the two
// genuinely AF-specific parts of SPF; the Dijkstra over the decoded graph is
// AF-agnostic. AFPrefixStrategy is the seam over those two parts: the Computer
// reads the LSDB and attaches prefixes through it, so an OSPFv3 strategy
// (Intra-Area-Prefix / Link LSAs, supplied by the engine) plugs in without
// touching the shared Computer or duplicating the FSM/flooding/Dijkstra.
//
// RFC: rfc/short/rfc2328.md (sec 16.1), rfc/short/rfc5340.md (App A.4)

package spf

import "github.com/ze-software/ze/internal/plugins/ospf/types"

// AFPrefixStrategy is the address-family-specific half of SPF: graph adjacency
// decode (BuildGraph) and prefix attachment (intra-area, inter-area, external).
// The shortest-path computation (Compute) over the decoded graph is AF-agnostic
// and is deliberately NOT part of this seam. The OSPFv2 strategy reproduces the
// current behavior exactly; the OSPFv3 strategy is supplied by the engine.
type AFPrefixStrategy interface {
	// BuildGraph decodes one area's Router/Network LSAs into the SPF graph
	// (router/network vertices + edges). OSPFv2 reads adjacency from the v2
	// Router/Network LSA bodies; OSPFv3 reads it from the address-free v6 bodies.
	BuildGraph(src Source, area types.AreaID) *Graph
	// BuildRoutes attaches intra-area prefixes to the reached vertices (RFC 2328
	// sec 16.1 stage 2). OSPFv2 reads stub links + the transit-network mask;
	// OSPFv3 reads Intra-Area-Prefix / Link LSAs.
	BuildRoutes(res *Result, maxPaths int, resolver InterfaceResolver) []RouteEntry
	// ComputeInterArea computes inter-area routes (RFC 2328 sec 16.2). OSPFv2
	// reads Type 3 Summary-LSAs; OSPFv3 reads Inter-Area-Prefix-LSAs.
	ComputeInterArea(in InterAreaInput) ([]RouteEntry, []BorderRouterEntry)
	// ComputeExternal computes AS-external / NSSA routes (RFC 2328 sec 16.4).
	ComputeExternal(in ExternalInput) []RouteEntry
	// OriginateSummaries originates the inter-area prefix-carrying LSAs an ABR
	// floods (RFC 2328 sec 12.4.3). OSPFv2 originates Type 3 Summary-LSAs
	// (network + mask); OSPFv3 originates Inter-Area-Prefix-LSAs (an IPv6 prefix).
	OriginateSummaries(in SummaryInput) SummaryOriginResult
	// NextHopSource resolves the SPF next-hop per address family: OSPFv2 from the
	// Router-LSA link data, OSPFv3 from the neighbor adjacency table (link-local).
	NextHopSource() NextHopSource
	// SummaryReader decodes an area's inter-area summaries (the same reader
	// ComputeInterArea uses). The RFC 2328 sec 16.3 transit-area pass reuses it to
	// re-examine a transit area's summaries; OSPFv2 reads Type 3/4 Summary-LSAs,
	// OSPFv3 reads Inter-Area-Prefix / Inter-Area-Router LSAs.
	SummaryReader(src Source) SummaryReader
}

// v4Strategy is the OSPFv2 AFPrefixStrategy. Each method delegates to the package
// function that already implements the v2 behavior, so routing the Computer
// through the seam is byte-for-byte identical to the prior direct calls (the v2
// SPF suite is the proof).
type v4Strategy struct{}

func (v4Strategy) BuildGraph(src Source, area types.AreaID) *Graph { return BuildGraph(src, area) }

func (v4Strategy) BuildRoutes(res *Result, maxPaths int, resolver InterfaceResolver) []RouteEntry {
	return BuildRoutes(res, maxPaths, resolver)
}

func (v4Strategy) ComputeInterArea(in InterAreaInput) ([]RouteEntry, []BorderRouterEntry) {
	return ComputeInterArea(in)
}

func (v4Strategy) ComputeExternal(in ExternalInput) []RouteEntry { return ComputeExternal(in) }

func (v4Strategy) OriginateSummaries(in SummaryInput) SummaryOriginResult {
	return OriginateSummaries(in)
}

func (v4Strategy) NextHopSource() NextHopSource { return v4NextHop{} }

func (v4Strategy) SummaryReader(src Source) SummaryReader { return v4SummaryReader(src) }

// The OSPFv2 strategy satisfies the seam.
var _ AFPrefixStrategy = v4Strategy{}
