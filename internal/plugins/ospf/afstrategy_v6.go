// Design: docs/architecture/ospf/ospf-af-unify.md -- Phase 5: the OSPFv3 AF prefix strategy.
// RFC: rfc/short/rfc5340.md (App A.4 LSA formats, sec 16 SPF)
//
// v6Strategy is the engine-side implementation of spf.AFPrefixStrategy for the IPv6
// family. BuildGraph decodes the address-free OSPFv3 Router/Network LSAs (RFC 5340
// App A.4.3/A.4.4) from the LSDB and translates their adjacency into the shared SPF
// graph, so the AF-agnostic Dijkstra runs unchanged. BuildRoutes attaches Intra-Area-Prefix
// prefixes (App A.4.10), ComputeInterArea reads Inter-Area-Prefix / Inter-Area-Router LSAs
// (App A.4.10/A.4.11) for inter-area routes, and ComputeExternal reads AS-External-LSAs
// (App A.4.7) for external routes; NextHopSource resolves the OSPFv3 next-hop (the
// neighbor's IPv6 link-local). OriginateSummaries originates the ABR Inter-Area-Prefix /
// Inter-Area-Router LSAs (App A.4.5/A.4.6) into the other areas (see origination_v6_summary.go).

package ospf

import (
	"net/netip"

	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v6Strategy implements spf.AFPrefixStrategy for OSPFv3. It holds the engine so it
// reads the live LSDB (prefix-bearing LSAs) and neighbor table (next-hop link-local)
// -- the engine recreates its neighbor table on (re)configure, so a captured pointer
// would go stale.
type v6Strategy struct {
	eng *engine
}

// v6NetKey identifies an OSPFv3 transit network by its DR's Router ID + Interface ID --
// the (DR-RID, DR-iface-ID) pair RFC 5340 App A.4.4 uses to name a Network-LSA. It joins a
// Router-LSA transit link (NeighborRouterID + NeighborInterfaceID) to the Network vertex.
type v6NetKey struct {
	dr      types.RouterID
	ifaceID uint32
}

func v6LSIDToUint32(l types.LinkStateID) uint32 {
	return uint32(l[0])<<24 | uint32(l[1])<<16 | uint32(l[2])<<8 | uint32(l[3])
}

// BuildGraph decodes the area's OSPFv3 Router-LSAs and Network-LSAs into the shared SPF
// graph. OSPFv3 Router/Network LSAs are address-free: a Router-LSA link carries the
// neighbor's Router ID + Interface ID (not an IP), so the neighbor identity maps onto the
// graph's RouterLink.LinkID and the next-hop is resolved separately via NextHopSource. It
// runs in two passes so transit links can join to the Network vertices: pass 1 decodes the
// Network-LSAs and assigns each a synthetic graph-local ID (the shared Dijkstra treats the
// LinkStateID as opaque; the real (DR-RID, DR-iface-ID) identity is kept on the vertex), pass
// 2 decodes the Router-LSAs and translates their links, joining a transit link to its Network
// vertex via that pair.
func (v6Strategy) BuildGraph(src ospfspf.Source, area types.AreaID) *ospfspf.Graph {
	g := ospfspf.NewGraph(area)
	if src == nil {
		return g
	}
	headers := src.Summary(area)
	// Pass 1: Network-LSAs -> Network vertices keyed by a synthetic graph-local ID, plus a
	// (DR-RID, DR-iface-ID) -> synthetic map for the transit-link join in pass 2.
	netID := make(map[v6NetKey]types.LinkStateID)
	var counter uint32
	for _, h := range headers {
		if h.Age.IsMaxAge() || ospfv3types.LSType(h.Type) != ospfv3types.LSTypeNetwork {
			continue
		}
		lsa, ok := src.LookupLSA(area, h.Key())
		if !ok {
			continue
		}
		decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
		if err != nil {
			continue
		}
		body, err := decoded.DecodeNetwork()
		if err != nil {
			continue
		}
		attached := make([]types.RouterID, len(body.AttachedRouters))
		for i, r := range body.AttachedRouters {
			attached[i] = types.RouterID(r)
		}
		counter++
		syn := v6SummaryLSID(counter)
		netID[v6NetKey{dr: h.AdvertisingRouter, ifaceID: v6LSIDToUint32(h.LinkStateID)}] = syn
		g.Networks[syn] = &ospfspf.NetworkVertex{
			ID:              syn,
			AdvertisingDR:   h.AdvertisingRouter,
			DRInterfaceID:   h.LinkStateID,
			AttachedRouters: attached,
		}
	}
	// Pass 2: Router-LSAs -> Router vertices, translating p2p + transit links.
	for _, h := range headers {
		if h.Age.IsMaxAge() || ospfv3types.LSType(h.Type) != ospfv3types.LSTypeRouter {
			continue
		}
		lsa, ok := src.LookupLSA(area, h.Key())
		if !ok {
			continue
		}
		decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
		if err != nil {
			continue
		}
		body, err := decoded.DecodeRouter()
		if err != nil {
			continue
		}
		g.Routers[h.AdvertisingRouter] = &ospfspf.RouterVertex{
			ID:    h.AdvertisingRouter,
			Flags: body.Flags,
			Links: v6RouterLinks(body.Links, netID),
		}
	}
	return g
}

// v6RouterLinks translates OSPFv3 Router-LSA links to the shared SPF RouterLink. A
// point-to-point link keys the neighbor by its Router ID; a transit link is joined to its
// Network vertex's synthetic graph ID via (NeighborRouterID, NeighborInterfaceID) (RFC 5340
// App A.4.3) -- a transit link whose Network-LSA is absent is dropped (the network is not yet
// reachable). The two-way check and Dijkstra then run unchanged on the shared graph.
func v6RouterLinks(in []ospfv3packet.RouterLink, netID map[v6NetKey]types.LinkStateID) []packet.RouterLink {
	out := make([]packet.RouterLink, 0, len(in))
	for _, l := range in {
		switch l.Type {
		case ospfv3packet.RouterLinkTypeP2P:
			out = append(out, packet.RouterLink{
				Type:   packet.RouterLinkTypeP2P,
				LinkID: types.LinkStateID(l.NeighborRouterID),
				Metric: types.Metric(l.Metric),
			})
		case ospfv3packet.RouterLinkTypeVirtual:
			// RFC 5340 App A.4.3: a virtual link keys the neighbor by its Router ID, exactly
			// like a p2p link. The shared Dijkstra already treats RouterLinkTypeVirtual as
			// p2p (spf.transitEdges), so the backbone graph reaches the virtual neighbor as a
			// router vertex. The virtual next hop (transit-area next hop) is resolved in the
			// RFC 2328 sec 16.3 transit-area pass, not here.
			out = append(out, packet.RouterLink{
				Type:   packet.RouterLinkTypeVirtual,
				LinkID: types.LinkStateID(l.NeighborRouterID),
				Metric: types.Metric(l.Metric),
			})
		case ospfv3packet.RouterLinkTypeTransit:
			syn, ok := netID[v6NetKey{dr: types.RouterID(l.NeighborRouterID), ifaceID: uint32(l.NeighborInterfaceID)}]
			if !ok {
				continue
			}
			out = append(out, packet.RouterLink{
				Type:   packet.RouterLinkTypeTransit,
				LinkID: syn,
				Metric: types.Metric(l.Metric),
			})
		}
	}
	return out
}

// v6NetworkVertexRef finds the Network vertex for an Intra-Area-Prefix-LSA's Network
// reference (RFC 5340 App A.4.7): the reference names the network by (DR-RID, DR-iface-ID),
// which BuildGraph stored on the vertex while keying the graph by a synthetic ID.
func v6NetworkVertexRef(res *ospfspf.Result, drRID types.RouterID, drIfaceID types.LinkStateID) (ospfspf.VertexID, bool) {
	if res == nil || res.Graph == nil {
		return ospfspf.VertexID{}, false
	}
	for id, nv := range res.Graph.Networks {
		if nv != nil && nv.AdvertisingDR == drRID && nv.DRInterfaceID == drIfaceID {
			return ospfspf.VertexID{Kind: ospfspf.VertexNetwork, Network: id}, true
		}
	}
	return ospfspf.VertexID{}, false
}

// BuildRoutes attaches OSPFv3 intra-area prefixes (RFC 5340 App A.4.10). Each
// Intra-Area-Prefix-LSA references a reached Router-LSA (its prefixes attach to that
// router vertex) or Network-LSA (the transit network's prefixes); the route inherits
// the referenced vertex's SPF next-hops and cost (vertex cost + per-prefix metric).
// The root's own prefixes have no SPF next-hop and are skipped (the connected source
// owns them), mirroring OSPFv2.
func (s v6Strategy) BuildRoutes(res *ospfspf.Result, _ int, _ ospfspf.InterfaceResolver) []ospfspf.RouteEntry {
	if s.eng == nil {
		return nil
	}
	return v6BuildRoutes(s.eng.lsdb, res, s.prefixAF())
}

// v6BuildRoutes is BuildRoutes over an explicit Source (the engine's LSDB), split out
// so the prefix attachment is unit-testable without a full engine. af selects the prefix
// address width (RFC 5838: 4 bytes for IPv4 families, 16 for IPv6).
func v6BuildRoutes(src ospfspf.Source, res *ospfspf.Result, af addressFamily) []ospfspf.RouteEntry {
	if res == nil || src == nil {
		return nil
	}
	var out []ospfspf.RouteEntry
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
		if nr == nil || len(nr.NextHops) == 0 {
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
			out = append(out, ospfspf.RouteEntry{
				AreaID:   res.Area,
				Prefix:   pfx,
				Metric:   metric,
				Type:     ospfspf.RouteIntraArea,
				Origin:   types.RouterID(body.ReferencedAdvRouter),
				NextHops: nr.NextHops,
			})
		}
	}
	return out
}

// v6PrefixToNetip converts an OSPFv3 prefix (RFC 5340 App A.4.1: prefix length +
// word-padded address) to a netip.Prefix at the address family's width: 4 bytes for the
// RFC 5838 IPv4 families (a 0..32-bit prefix in one 32-bit word, §2.7), 16 bytes for IPv6.
// A prefix length wider than the AF's address rejects the LSA.
func v6PrefixToNetip(p ospfv3packet.Prefix, af addressFamily) (netip.Prefix, bool) {
	bits := int(p.Length)
	if af.isIPv4() {
		if bits > 32 {
			return netip.Prefix{}, false
		}
		var a [4]byte
		copy(a[:], p.Address)
		return netip.PrefixFrom(netip.AddrFrom4(a), bits).Masked(), true
	}
	if bits > 128 {
		return netip.Prefix{}, false
	}
	var a [16]byte
	copy(a[:], p.Address)
	return netip.PrefixFrom(netip.AddrFrom16(a), bits).Masked(), true
}

// v6ForwardingAddr renders an OSPFv3 AS-External/NSSA forwarding address at the AF's address
// width (RFC 5838 §2.7): an IPv4-unicast/multicast AF carries a 4-byte IPv4 forwarding
// address in the leading octets of the 128-bit field; every other AF carries a full IPv6
// address. Benign when the FA is zero, but load-bearing for a peer that sets an IPv4 FA.
func v6ForwardingAddr(fa [16]byte, af addressFamily) netip.Addr {
	if af.isIPv4() {
		var a [4]byte
		copy(a[:], fa[:4])
		return netip.AddrFrom4(a)
	}
	return netip.AddrFrom16(fa)
}

// prefixAF returns the address family whose width v6PrefixToNetip must use. A strategy
// without an engine (direct unit tests) defaults to IPv6-unicast (16-byte).
func (s v6Strategy) prefixAF() addressFamily {
	if s.eng == nil {
		return afIPv6Unicast
	}
	return s.eng.af
}

// ComputeInterArea computes OSPFv3 inter-area routes by reading the area's
// Inter-Area-Prefix-LSAs (0x2003, RFC 5340 App A.4.10) and Inter-Area-Router-LSAs (0x2004,
// App A.4.11) through the v6 summary reader; the shared SPF computation handles ABR
// reachability, metric composition, area-range suppression and border-router selection.
func (s v6Strategy) ComputeInterArea(in ospfspf.InterAreaInput) ([]ospfspf.RouteEntry, []ospfspf.BorderRouterEntry) {
	return ospfspf.ComputeInterAreaWith(in, v6SummaryReader(in.Source, s.prefixAF()))
}

// v6SummaryReader decodes the OSPFv3 inter-area summaries an ABR advertises into an area:
// Inter-Area-Prefix-LSAs carry a network prefix + metric; Inter-Area-Router-LSAs carry a
// summarized ASBR + metric. Both are area-scoped and address-free (RFC 5340 App A.4.10/11).
func v6SummaryReader(src ospfspf.Source, af addressFamily) ospfspf.SummaryReader {
	return func(area types.AreaID) []ospfspf.InterAreaSummary {
		if src == nil {
			return nil
		}
		var out []ospfspf.InterAreaSummary
		for _, h := range src.Summary(area) {
			if h.Age.IsMaxAge() {
				continue
			}
			lsa, ok := src.LookupLSA(area, h.Key())
			if !ok {
				continue
			}
			switch ospfv3types.LSType(h.Type) {
			case ospfv3types.LSTypeInterAreaPrefix:
				decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
				if err != nil {
					continue
				}
				body, err := decoded.DecodeInterAreaPrefix()
				if err != nil || uint64(body.Metric) >= ospfspf.LSInfinity {
					continue
				}
				pfx, ok := v6PrefixToNetip(body.Prefix, af)
				if !ok {
					continue
				}
				out = append(out, ospfspf.InterAreaSummary{AdvertisingRouter: h.AdvertisingRouter, Metric: uint64(body.Metric), Prefix: pfx})
			case ospfv3types.LSTypeInterAreaRouter:
				decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
				if err != nil {
					continue
				}
				body, err := decoded.DecodeInterAreaRouter()
				if err != nil || uint64(body.Metric) >= ospfspf.LSInfinity {
					continue
				}
				out = append(out, ospfspf.InterAreaSummary{AdvertisingRouter: h.AdvertisingRouter, Metric: uint64(body.Metric), IsASBR: true, ASBR: types.RouterID(body.DestinationRouter)})
			default:
				continue
			}
		}
		return out
	}
}

// ComputeExternal computes OSPFv3 AS-external routes by reading AS-External-LSAs (0x4005,
// RFC 5340 App A.4.7) and, for the attached NSSA areas, NSSA-LSAs (0x2007, App A.4.8) through
// the v6 external reader; the shared SPF computation handles ASBR / forwarding-address
// reachability, the E1/E2 cost (RFC 2328 sec 16.4) and the RFC 3101 source-preference selection.
func (s v6Strategy) ComputeExternal(in ospfspf.ExternalInput) []ospfspf.RouteEntry {
	return ospfspf.ComputeExternalWith(in, v6ExternalReader(in.Source, s.prefixAF()))
}

// v6ExternalReader decodes an OSPFv3 AS-External-LSA (0x4005) or NSSA-LSA (0x2007) into an
// address-family-neutral ExternalRecord: the IPv6 prefix, advertised metric, E1/E2 type, and
// optional 128-bit forwarding address (RFC 5340 App A.4.7/A.4.8 -- the two bodies are
// identical). For an NSSA-LSA the RFC 3101 sec 2.5 source preference is set from the prefix's
// P-bit (OptPrefixP); an AS-External is always the Type-5 preference.
func v6ExternalReader(src ospfspf.Source, af addressFamily) ospfspf.ExternalReader {
	return func(area types.AreaID, h packet.LSAHeader) (ospfspf.ExternalRecord, bool) {
		isNSSA := h.Type.NSSA()
		if !h.Type.ASExternal() && !isNSSA {
			return ospfspf.ExternalRecord{}, false
		}
		lsa, ok := src.LookupLSA(area, h.Key())
		if !ok {
			return ospfspf.ExternalRecord{}, false
		}
		decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
		if err != nil {
			return ospfspf.ExternalRecord{}, false
		}
		body, err := decoded.DecodeExternal()
		if err != nil || uint64(body.Metric) >= ospfspf.LSInfinity {
			return ospfspf.ExternalRecord{}, false
		}
		pfx, ok := v6PrefixToNetip(body.Prefix, af)
		if !ok {
			return ospfspf.ExternalRecord{}, false
		}
		var fa netip.Addr
		if body.HasForwardingAddr {
			fa = v6ForwardingAddr(body.ForwardingAddr, af)
		}
		pref := ospfspf.ExternalPrefType5
		if isNSSA {
			if ospfv3packet.NSSAPropagate(body) {
				pref = ospfspf.ExternalPrefType7P1
			} else {
				pref = ospfspf.ExternalPrefType7P0
			}
		}
		return ospfspf.ExternalRecord{
			Prefix:         pfx,
			Metric:         uint64(body.Metric),
			Type2:          body.ExternalType2,
			ForwardingAddr: fa,
			Pref:           pref,
			Origin:         h.AdvertisingRouter,
		}, true
	}
}

// OriginateSummaries originates this ABR's OSPFv3 inter-area summaries (Inter-Area-Prefix
// and Inter-Area-Router LSAs) into its attached areas, mirroring the OSPFv2 ABR summary
// origination (RFC 2328 sec 12.4.3) over the address-free OSPFv3 LSA formats (RFC 5340 App
// A.4.5/A.4.6). The engine owns the LSDB, so origination runs through the same OriginateSelf
// / FlushStaleSelfLSAs seams as the v6 self Router / Intra-Area-Prefix LSAs (the v4 Sink is
// not used); see v6OriginateSummaries.
func (s v6Strategy) OriginateSummaries(in ospfspf.SummaryInput) ospfspf.SummaryOriginResult {
	if s.eng == nil {
		return ospfspf.SummaryOriginResult{}
	}
	return s.eng.v6OriginateSummaries(in)
}

// NextHopSource resolves the OSPFv3 next-hop: the neighbor's IPv6 link-local from
// the adjacency table (RFC 5340 sec 3.8.1 -- v3 next-hops come from the adjacency,
// not the LSA, which is address-free).
func (s v6Strategy) NextHopSource() ospfspf.NextHopSource {
	if s.eng == nil {
		return v6NextHop{}
	}
	// Read the engine's current neighbor table: NextHopSource is invoked per SPF run,
	// after the table is established, so the read is fresh (not a stale capture).
	return v6NextHop{neighbors: s.eng.neighbors}
}

// v6NextHop resolves the next-hop to a directly-reached OSPFv3 neighbor by looking up
// its link-local in the adjacency table by Router ID.
type v6NextHop struct {
	neighbors *ospfneighbor.Table
}

func (n v6NextHop) P2PNextHop(_ *ospfspf.Graph, neighbor, _ types.RouterID) (netip.Addr, bool) {
	if n.neighbors == nil {
		return netip.Addr{}, false
	}
	return n.neighbors.AddressOf(neighbor)
}

func (n v6NextHop) TransitNextHop(_ *ospfspf.Graph, router types.RouterID, _ types.LinkStateID) (netip.Addr, bool) {
	if n.neighbors == nil {
		return netip.Addr{}, false
	}
	return n.neighbors.AddressOf(router)
}

// SummaryReader supplies the OSPFv3 inter-area summary decode for the RFC 2328 sec 16.3
// transit-area pass (the same reader ComputeInterArea uses, including the RFC 5838 address
// family so the summary prefixes decode for this engine's AF).
func (s v6Strategy) SummaryReader(src ospfspf.Source) ospfspf.SummaryReader {
	return v6SummaryReader(src, s.prefixAF())
}

var (
	_ ospfspf.AFPrefixStrategy = v6Strategy{}
	_ ospfspf.NextHopSource    = v6NextHop{}
)
