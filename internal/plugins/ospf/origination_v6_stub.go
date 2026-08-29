// Design: docs/architecture/ospf/ospfv3-6-interop-coverage.md -- OSPFv3 stub-area policy for the ABR
// inter-area summary set (the v6 counterpart of spf.applyAreaTypePolicy).
// RFC: rfc/short/rfc5340.md (sec 3.5 inter-area), rfc/short/rfc2328.md (sec 3.6 stub areas)
//
// The shared SPF computer feeds the per-destination-area stub/NSSA policy to BOTH families
// via SummaryInput.Policies. The OSPFv2 summary path applies it in spf.applyAreaTypePolicy;
// this is the OSPFv3 equivalent, applied to the (address-bearing) v6 inter-area desired set
// before v6OriginateAreaSummaries turns it into Inter-Area-Prefix/Router-LSAs.

package ospf

import (
	"net/netip"
	"sort"

	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
)

// v6ApplyAreaTypePolicy mirrors spf.applyAreaTypePolicy for the OSPFv3 inter-area set
// (RFC 5340 sec 3.5 / RFC 2328 sec 3.6): a stub/NSSA destination area never receives an
// Inter-Area-Router-LSA (Type 0x2004, the v6 Type-4 ASBR-summary equivalent); a
// totally-stubby/NSSA area (no-summary) suppresses every Inter-Area-Prefix except the
// injected default; a stub area always gets exactly one Inter-Area-Prefix default (::/0) at
// default-cost so its internal routers have a way out without carrying external state. A
// regular NSSA takes its default as an NSSA-LSA instead (applyNSSADefaults), so ::/0 is
// injected for a stub area and for a no-summary NSSA only. A normal area is returned
// unchanged.
//
// RFC 3101 Section 2.7: "When OSPF's summary routes are not imported, the default LSA
// originated by an NSSA border router into the NSSA should be a Type-3 summary-LSA."
// The OSPFv3 equivalent of that Type-3 is the Inter-Area-Prefix-LSA (RFC 5340 Section 4.4).
func v6ApplyAreaTypePolicy(nets []v6SummaryNet, routers []v6SummaryRouter, p ospfspf.AreaSummaryPolicy) ([]v6SummaryNet, []v6SummaryRouter) {
	if p.Type != ospfspf.AreaTypeStub && p.Type != ospfspf.AreaTypeNSSA {
		return nets, routers
	}
	routers = nil // stub/NSSA: never originate an Inter-Area-Router-LSA into the area
	if p.NoSummary {
		nets = nil // totally-stubby/NSSA: suppress every inter-area prefix but the default
	}
	// RFC requirement: RFC3101-2.7-2 -- a no-summary NSSA takes its
	// border-router default as a summary-LSA, not as a Type-7.
	if p.Type == ospfspf.AreaTypeStub || p.NoSummary {
		nets = append(nets, v6SummaryNet{Prefix: netip.PrefixFrom(netip.IPv6Unspecified(), 0), Metric: p.DefaultCost})
		sort.Slice(nets, func(i, j int) bool { return nets[i].Prefix.Compare(nets[j].Prefix) < 0 })
	}
	return nets, routers
}
