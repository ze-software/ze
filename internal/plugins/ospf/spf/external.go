// Design: docs/architecture/ospf/ospf-10-as-external-asbr.md -- RFC 2328 sec 16.4 AS-External routes.
// RFC: rfc/short/rfc2328.md -- sec 16.4 (E1/E2 cost, forwarding address), trap #7 (E1 > E2)

package spf

import (
	"net/netip"
	"sort"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// ExternalInput is one RFC 2328 sec 16.4 AS-External route computation pass over the
// received Type 5 LSAs, run AFTER the intra/inter-area route table is resolved (the
// ASBR and forwarding-address lookups resolve against it).
type ExternalInput struct {
	Source           Source
	Root             types.RouterID
	BorderRouters    []BorderRouterEntry // per-area ASBR reachability from ComputeInterArea
	Routes           []RouteEntry        // intra/inter-area candidates, before cross-area selection
	Resolver         InterfaceResolver
	MaxPaths         int
	NSSAAreas        []types.AreaID                     // attached NSSA areas whose Type 7 LSAs also yield externals (RFC 3101)
	NSSAPolicies     map[types.AreaID]AreaSummaryPolicy // per-NSSA summary-import policy
	NSSABorderRouter bool                               // the calculating router is an ABR attached to an NSSA

	// Native-run AS-scope inputs remain known even without a resolved ASBR path.
	knownOrigins map[types.RouterID]struct{}
}

type asbrReach struct {
	area     types.AreaID
	metric   uint64
	nextHops []NextHop
}

type asbrKey struct {
	area   types.AreaID
	router types.RouterID
}

type externalKey struct {
	prefix netip.Prefix
	fa     netip.Addr
}

type externalCand struct {
	prefix   netip.Prefix
	metric   uint64
	rtype    RouteType
	fwdDist  uint64 // distance to the forwarding target (E2 tie-break)
	origin   types.RouterID
	nextHops []NextHop
	fa       netip.Addr
	pref     uint8        // RFC 3101 sec 2.5 source preference (Type7 P=1 < Type5 < Type7 P=0)
	area     types.AreaID // origin area (backbone for Type 5, the NSSA for Type 7)
}

// ComputeExternal computes RFC 2328 sec 16.4 external routes from Type 5
// AS-External-LSAs. E1 cost = distance-to-forwarding + advertised metric; E2 cost =
// advertised metric only (tie-broken by the forwarding distance). E1 is always
// preferred over E2 regardless of cost (trap #7); externals rank below internal
// routes (resolved later by selectBestRoutes).
// ExternalRecord is one decoded external LSA, address-family-neutral: the destination
// prefix, advertised metric, E1/E2 metric type, optional forwarding address, RFC 3101
// source preference, and the advertising ASBR. The address-family-specific decode
// (OSPFv2 Type 5/7 vs OSPFv3 AS-External 0x4005) yields these; the shared computation
// handles ASBR/forwarding reachability, E1/E2 cost and best-route selection.
type ExternalRecord struct {
	Prefix         netip.Prefix
	Metric         uint64
	Type2          bool
	ForwardingAddr netip.Addr // invalid/zero => forward via the advertising ASBR
	Pref           uint8
	Origin         types.RouterID
}

// ExternalReader decodes the external LSA at h (in area) into an ExternalRecord, returning
// false when the LSA is not an external this family handles or is unusable. It is the only
// address-family-specific part of the external computation.
type ExternalReader func(area types.AreaID, h packet.LSAHeader) (ExternalRecord, bool)

func ComputeExternal(in ExternalInput) []RouteEntry {
	return ComputeExternalWith(in, v4ExternalReader(in.Source))
}

// ComputeExternalWith is ComputeExternal parameterized by an address-family external
// reader, so the OSPFv3 strategy can decode AS-External-LSAs (RFC 5340 App A.4.7) while
// sharing the ASBR reachability, forwarding-address resolution, E1/E2 cost (RFC 2328 sec
// 16.4) and RFC 3101 sec 2.5 source-preference selection.
func ComputeExternalWith(in ExternalInput, read ExternalReader) []RouteEntry {
	if in.Source == nil || read == nil {
		return nil
	}
	maxPaths := in.MaxPaths
	if maxPaths <= 0 {
		maxPaths = DefaultMaxPaths
	}
	reach := make(map[asbrKey]asbrReach)
	for _, b := range in.BorderRouters {
		if b.Kind != BorderRouterASBR || b.RouterID == (types.RouterID{}) || len(b.NextHops) == 0 || b.Metric >= LSInfinity {
			continue
		}
		area := b.AreaID
		if in.type5Capable(area) {
			area = types.BackboneArea
		} else if !in.nssaArea(area) {
			continue
		}
		key := asbrKey{area: area, router: b.RouterID}
		cur, exists := reach[key]
		if exists {
			if b.Metric > cur.metric {
				continue
			}
			if b.Metric == cur.metric {
				if compare4(b.AreaID, cur.area) <= 0 {
					continue
				}
			}
		}
		reach[key] = asbrReach{area: b.AreaID, metric: b.Metric, nextHops: b.NextHops}
	}

	byForwarding := make(map[externalKey]externalCand)
	// AS-External-LSAs from the AS-wide store (OSPFv2 Type 5 / OSPFv3 0x4005).
	for _, h := range in.Source.Summary(types.BackboneArea) {
		if !h.Age.IsMaxAge() && h.AdvertisingRouter != (types.RouterID{}) &&
			(h.Type.ASWide() || h.Type == types.LSTypeOpaqueAS) && in.knownOrigins != nil {
			in.knownOrigins[h.AdvertisingRouter] = struct{}{}
		}
		if !h.Type.ASExternal() || h.Age.IsMaxAge() || h.AdvertisingRouter == in.Root {
			continue
		}
		if rec, ok := read(types.BackboneArea, h); ok {
			if cand, ok := in.externalCandidateFrom(types.BackboneArea, rec, reach, maxPaths); ok {
				keepBestExternal(byForwarding, cand, maxPaths)
			}
		}
	}
	// RFC 3101 Section 2.5 runs the same external calculation for every attached
	// NSSA, with ASBR and forwarding-address lookups confined to that NSSA.
	for _, area := range in.NSSAAreas {
		for _, h := range in.Source.Summary(area) {
			if !h.Type.NSSA() || h.Age.IsMaxAge() || h.AdvertisingRouter == in.Root {
				continue
			}
			if rec, ok := read(area, h); ok {
				if in.NSSABorderRouter && rec.Prefix.Bits() == 0 {
					// RFC requirement: RFC3101-2.4-4 -- an NSSA border
					// router MUST reject a P-clear Type-7 default.
					if rec.Pref == prefType7P0 {
						continue
					}
					// RFC requirement: RFC3101-2.5-1 -- an NSSA border
					// router MUST reject Type-7 defaults when it suppresses
					// summary-route import.
					if in.NSSAPolicies[area].NoSummary {
						continue
					}
				}
				if cand, ok := in.externalCandidateFrom(area, rec, reach, maxPaths); ok {
					keepBestExternal(byForwarding, cand, maxPaths)
				}
			}
		}
	}
	// Apply equivalence tie-breaks within each forwarding-address group before
	// merging equal-cost exits. Replacing one group's Type 5 with its Type 7
	// must not discard an unrelated, equal-cost forwarding address.
	best := make(map[externalKey]externalCand)
	for _, candidate := range byForwarding {
		candidate.fa = netip.Addr{}
		keepBestExternal(best, candidate, maxPaths)
	}

	out := make([]RouteEntry, 0, len(best))
	for _, c := range best {
		out = append(out, RouteEntry{AreaID: c.area, Prefix: c.prefix, Metric: c.metric, Type: c.rtype, Origin: c.origin, NextHops: c.nextHops})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Prefix.Compare(out[j].Prefix) < 0 })
	return out
}

// v4ExternalReader decodes OSPFv2 Type 5 AS-External and Type 7 NSSA LSAs into the
// address-family-neutral ExternalRecord (prefix from Link State ID + mask, RFC 3101 sec
// 2.5 preference from the type and the Type-7 P-bit).
func v4ExternalReader(src Source) ExternalReader {
	return func(area types.AreaID, h packet.LSAHeader) (ExternalRecord, bool) {
		if h.Type != types.LSTypeASExternal && h.Type != types.LSTypeNSSA {
			return ExternalRecord{}, false
		}
		lsa, ok := src.LookupLSA(area, h.Key())
		if !ok {
			return ExternalRecord{}, false
		}
		body, err := externalBody(lsa)
		if err != nil || uint64(body.Metric) >= LSInfinity {
			return ExternalRecord{}, false
		}
		pfx, ok := summaryPrefix(h.LinkStateID, body.NetworkMask)
		if !ok {
			return ExternalRecord{}, false
		}
		pref := prefType5
		if h.Type == types.LSTypeNSSA {
			if lsa.Header.Options.Has(types.OptionNP) {
				pref = prefType7P1
			} else {
				pref = prefType7P0
			}
		}
		var fa netip.Addr
		if body.ForwardingAddr != ([4]byte{}) {
			fa = netip.AddrFrom4(body.ForwardingAddr)
		}
		return ExternalRecord{Prefix: pfx, Metric: uint64(body.Metric), Type2: body.ExternalType2, ForwardingAddr: fa, Pref: pref, Origin: h.AdvertisingRouter}, true
	}
}

// externalCandidateFrom builds the RFC 2328 sec 16.4 external candidate from a decoded
// ExternalRecord, resolving the forwarding next-hops and computing the E1/E2 cost. Returns
// false when the external is unusable (LSInfinity metric, unreachable forwarding target).
func (in ExternalInput) externalCandidateFrom(area types.AreaID, rec ExternalRecord, reach map[asbrKey]asbrReach, maxPaths int) (externalCand, bool) {
	if rec.Metric >= LSInfinity || !rec.Prefix.IsValid() {
		return externalCand{}, false
	}
	// RFC 3101 Section 2.5(3) requires a reachable ASBR even when a non-zero
	// forwarding address supplies the next hop.
	asbr, exists := reach[asbrKey{area: area, router: rec.Origin}]
	if !exists {
		return externalCand{}, false
	}
	baseCost, nextHops, ok := in.resolveForwarding(area, rec.ForwardingAddr, asbr, maxPaths)
	if !ok {
		return externalCand{}, false // ASBR or forwarding address unreachable
	}
	cand := externalCand{prefix: rec.Prefix, origin: rec.Origin, nextHops: nextHops, fwdDist: baseCost, pref: rec.Pref, area: area, fa: rec.ForwardingAddr}
	if rec.Type2 {
		cand.metric = rec.Metric
		cand.rtype = RouteExternalType2
	} else {
		cand.metric = clampMetric(baseCost, rec.Metric)
		cand.rtype = RouteExternalType1
	}
	if cand.metric >= LSInfinity {
		return externalCand{}, false
	}
	return cand, true
}

// resolveForwarding applies RFC 3101 Section 2.5(3). A Type-7 forwarding
// address needs an intra-area route in its own NSSA. A Type-5 forwarding
// address needs an internal route through a Type-5 capable area.
func (in ExternalInput) resolveForwarding(area types.AreaID, fa netip.Addr, asbr asbrReach, maxPaths int) (uint64, []NextHop, bool) {
	if !fa.IsValid() || fa.IsUnspecified() {
		return asbr.metric, decorateNextHops(asbr.nextHops, in.Resolver, maxPaths), true
	}
	var best *RouteEntry
	var nextHops []NextHop
	for idx := range in.Routes {
		route := &in.Routes[idx]
		if route.Type != RouteIntraArea {
			if route.Type != RouteInterArea {
				continue
			}
		}
		if area == types.BackboneArea {
			if !in.type5Capable(route.AreaID) {
				continue
			}
		} else {
			if route.AreaID != area {
				continue
			}
			if route.Type != RouteIntraArea {
				continue
			}
		}
		if !route.Prefix.IsValid() {
			continue
		}
		if route.Prefix.Bits() == 0 {
			continue
		}
		if !route.Prefix.Contains(fa) {
			continue
		}
		if route.Metric >= LSInfinity {
			continue
		}
		if len(route.NextHops) == 0 {
			continue
		}
		if best != nil {
			if route.Prefix.Bits() < best.Prefix.Bits() {
				continue
			}
			if route.Prefix.Bits() == best.Prefix.Bits() {
				if routeSamePreference(*route, *best) {
					nextHops, _ = mergeNextHops(nextHops, route.NextHops, maxPaths)
					continue
				}
				if !routeBetter(*route, *best) {
					continue
				}
			}
		}
		best = route
		nextHops = route.NextHops
	}
	if best == nil {
		return 0, nil, false
	}
	return best.Metric, decorateNextHops(nextHops, in.Resolver, maxPaths), true
}

func (in ExternalInput) nssaArea(area types.AreaID) bool {
	if in.NSSAPolicies[area].Type == types.AreaTypeNSSA {
		return true
	}
	for _, nssa := range in.NSSAAreas {
		if nssa == area {
			return true
		}
	}
	return false
}

func (in ExternalInput) type5Capable(area types.AreaID) bool {
	if in.nssaArea(area) {
		return false
	}
	return in.NSSAPolicies[area].Type != types.AreaTypeStub
}

func keepBestExternal(best map[externalKey]externalCand, c externalCand, maxPaths int) {
	c.nextHops = capNextHops(c.nextHops, maxPaths)
	sortNextHops(c.nextHops)
	if len(c.nextHops) == 0 {
		return
	}
	key := externalKey{prefix: c.prefix, fa: c.fa}
	cur, ok := best[key]
	switch {
	case !ok || betterExternal(c, cur):
		best[key] = c
	case sameExternalPref(c, cur):
		cur.nextHops, _ = mergeNextHops(cur.nextHops, c.nextHops, maxPaths)
		if compare4(c.origin, cur.origin) < 0 {
			cur.origin = c.origin
		}
		best[key] = cur
	}
}

// betterExternal follows RFC 3101 Section 2.5(6): metric type and cost are
// compared before source preference. The Type-7 P=1 / Type-5 / higher-RID
// tie-break applies only to functionally equivalent LSAs with a shared,
// non-zero forwarding address.
func betterExternal(a, b externalCand) bool {
	if routeTypeRank(a.rtype) != routeTypeRank(b.rtype) {
		return routeTypeRank(a.rtype) < routeTypeRank(b.rtype)
	}
	if a.metric != b.metric {
		return a.metric < b.metric
	}
	// Forwarding distance is an E2-ONLY tie-break (RFC 2328 sec 16.4 step (d)): an E1 route's
	// metric already folds in the intra-AS path cost, so equal-cost E1 paths are equal-
	// preference regardless of fwdDist and must merge (ECMP), not be ordered by it.
	if a.rtype == RouteExternalType2 && a.fwdDist != b.fwdDist {
		return a.fwdDist < b.fwdDist
	}
	if !a.fa.IsValid() || a.fa.IsUnspecified() || a.fa != b.fa {
		return false
	}
	if a.pref != b.pref {
		return a.pref < b.pref
	}
	return compare4(a.origin, b.origin) > 0
}

func sameExternalPref(a, b externalCand) bool {
	return !betterExternal(a, b) && !betterExternal(b, a)
}

func externalBody(lsa packet.LSA) (packet.ExternalLSA, error) {
	if lsa.External != nil {
		return *lsa.External, nil
	}
	return lsa.DecodeExternal()
}
