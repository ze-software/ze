// Design: plan/learned/964-ospf-10-as-external-asbr.md -- RFC 2328 sec 16.4 AS-External routes.
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
	BorderRouters    []BorderRouterEntry // ASBR reachability (router-id -> cost + next-hops), from ComputeInterArea
	Routes           []RouteEntry        // resolved intra + inter route table, for forwarding-address resolution
	Resolver         InterfaceResolver
	MaxPaths         int
	NSSAAreas        []types.AreaID                     // attached NSSA areas whose Type 7 LSAs also yield externals (RFC 3101)
	NSSAPolicies     map[types.AreaID]AreaSummaryPolicy // per-NSSA summary-import policy
	NSSABorderRouter bool                               // the calculating router is an ABR attached to an NSSA
}

type asbrReach struct {
	metric   uint64
	nextHops []NextHop
}

type externalCand struct {
	prefix   netip.Prefix
	metric   uint64
	rtype    RouteType
	fwdDist  uint64 // distance to the forwarding target (E2 tie-break)
	origin   types.RouterID
	nextHops []NextHop
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
	reach := make(map[types.RouterID]asbrReach)
	for _, b := range in.BorderRouters {
		if b.Kind != BorderRouterASBR || b.RouterID == (types.RouterID{}) || len(b.NextHops) == 0 || b.Metric >= LSInfinity {
			continue
		}
		if cur, ok := reach[b.RouterID]; !ok || b.Metric < cur.metric {
			reach[b.RouterID] = asbrReach{metric: b.Metric, nextHops: b.NextHops}
		}
	}

	best := make(map[netip.Prefix]externalCand)
	// AS-External-LSAs from the AS-wide store (OSPFv2 Type 5 / OSPFv3 0x4005).
	for _, h := range in.Source.Summary(types.BackboneArea) {
		if !h.Type.ASExternal() || h.Age.IsMaxAge() || h.AdvertisingRouter == in.Root {
			continue
		}
		if rec, ok := read(types.BackboneArea, h); ok {
			if cand, ok := in.externalCandidateFrom(types.BackboneArea, rec, reach, maxPaths); ok {
				keepBestExternal(best, cand, maxPaths)
			}
		}
	}
	// Type 7 NSSA-LSAs in each attached NSSA (RFC 3101): they also yield externals and,
	// for a prefix known both ways, the sec 2.5 source preference (carried in cand.pref)
	// resolves the winner ahead of the sec 16.4 cost.
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
					keepBestExternal(best, cand, maxPaths)
				}
			}
		}
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
func (in ExternalInput) externalCandidateFrom(area types.AreaID, rec ExternalRecord, reach map[types.RouterID]asbrReach, maxPaths int) (externalCand, bool) {
	if rec.Metric >= LSInfinity || !rec.Prefix.IsValid() {
		return externalCand{}, false
	}
	baseCost, nextHops, ok := in.resolveForwarding(rec.ForwardingAddr, rec.Origin, reach, maxPaths)
	if !ok {
		return externalCand{}, false // ASBR or forwarding address unreachable
	}
	cand := externalCand{prefix: rec.Prefix, origin: rec.Origin, nextHops: nextHops, fwdDist: baseCost, pref: rec.Pref, area: area}
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

// resolveForwarding returns the base cost and next-hops to the forwarding target. An
// invalid/zero Forwarding Address forwards via the advertising ASBR; a set one must itself
// be reachable via an intra/inter-area route (else the external is skipped).
func (in ExternalInput) resolveForwarding(fa netip.Addr, asbr types.RouterID, reach map[types.RouterID]asbrReach, maxPaths int) (uint64, []NextHop, bool) {
	if !fa.IsValid() || fa.IsUnspecified() {
		p, ok := reach[asbr]
		if !ok || len(p.nextHops) == 0 {
			return 0, nil, false
		}
		return p.metric, decorateNextHops(p.nextHops, in.Resolver, maxPaths), true
	}
	route, ok := routeToAddr(in.Routes, fa)
	if !ok || len(route.NextHops) == 0 {
		return 0, nil, false
	}
	return route.Metric, decorateNextHops(route.NextHops, in.Resolver, maxPaths), true
}

func keepBestExternal(best map[netip.Prefix]externalCand, c externalCand, maxPaths int) {
	c.nextHops = capNextHops(c.nextHops, maxPaths)
	sortNextHops(c.nextHops)
	if len(c.nextHops) == 0 {
		return
	}
	cur, ok := best[c.prefix]
	switch {
	case !ok || betterExternal(c, cur):
		best[c.prefix] = c
	case sameExternalPref(c, cur):
		cur.nextHops, _ = mergeNextHops(cur.nextHops, c.nextHops, maxPaths)
		if compare4(c.origin, cur.origin) < 0 {
			cur.origin = c.origin
		}
		best[c.prefix] = cur
	}
}

// betterExternal orders external candidates. The NSSA source preference (Type-7 P=1 > Type-5
// > Type-7 P=0, RFC 3101 sec 2.5) is the PRIMARY key by deliberate design (see
// TestOSPFNSSAPreference): a router prefers the NSSA-local Type-7 P=1 route even over a
// lower-cost Type-5, keeping NSSA traffic on the local exit. Below that it follows RFC 2328 sec
// 16.4: a Type-1 (E1) path beats a Type-2 (E2), then lower cost, then (E2 only) lower forwarding
// distance. (The review flagged the source-pref-primary order as a possible RFC 2328 sec 16.4(b)
// inversion; it is intentional and spec-defensible under RFC 3101 NSSA locality, so it stands.)
func betterExternal(a, b externalCand) bool {
	if a.pref != b.pref {
		return a.pref < b.pref
	}
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
	return false
}

func sameExternalPref(a, b externalCand) bool {
	if a.pref != b.pref || routeTypeRank(a.rtype) != routeTypeRank(b.rtype) || a.metric != b.metric {
		return false
	}
	// fwdDist distinguishes preference only for E2 (RFC 2328 sec 16.4 step (d)); equal-cost E1
	// paths are ECMP-equal whatever their forwarding distance.
	if a.rtype == RouteExternalType2 && a.fwdDist != b.fwdDist {
		return false
	}
	return true
}

func externalBody(lsa packet.LSA) (packet.ExternalLSA, error) {
	if lsa.External != nil {
		return *lsa.External, nil
	}
	return lsa.DecodeExternal()
}

// routeToAddr returns the longest-prefix-match route covering addr, used only to resolve
// an AS-external LSA's non-zero forwarding address. RFC 2328 sec 16.4: the forwarding
// address must resolve to a specific intra/inter-area route; the default route does not
// count (in.Routes already excludes external routes, so the remaining exclusion is /0).
// Matching a default would make an otherwise-unreachable forwarding address appear
// reachable whenever any default exists, defeating the reachability check (the LSA is
// skipped when this returns false).
func routeToAddr(routes []RouteEntry, addr netip.Addr) (RouteEntry, bool) {
	best := RouteEntry{}
	bestBits := -1
	for _, r := range routes {
		if r.Prefix.IsValid() && r.Prefix.Bits() > 0 && r.Prefix.Contains(addr) && r.Prefix.Bits() > bestBits {
			best = r
			bestBits = r.Prefix.Bits()
		}
	}
	return best, bestBits >= 0
}
