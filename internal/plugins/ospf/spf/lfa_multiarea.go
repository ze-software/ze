// Design: docs/architecture/ospf/ospf-ext-6-ti-lfa.md -- RFC 5286 Section 6.3 OSPF multi-area
// LFA suppression. OSPF traffic can leave and re-enter an area, so the local-area
// SPF may not reflect the real inter-area / external path. Where the Section 6.3
// leakage conditions hold, an LFA computed from the local-area SPF can micro-loop,
// so it is suppressed (the prefix is left unprotected) rather than installed wrong.
// RFC: rfc/short/rfc5286.md (Section 6.3 a-d)

package spf

import "github.com/ze-software/ze/internal/plugins/ospf/types"

// suppressLFA reports whether an LFA MUST be suppressed for a route of the given
// type in the given area (RFC 5286 Section 6.3). Intra-area routes are always
// safe: the single-area SPF is authoritative for them. Inter-area and external
// routes are suppressed in the listed leakage topologies:
//
//   - Section 6.3(a): the backbone with virtual links configured (a transit area
//     without a full virtual-link mesh can leak a path); conservatively suppress
//     backbone inter-area/external LFAs whenever any virtual link is configured.
//   - Section 6.3(b): an area containing more than one alternate ABR -- an
//     inter-area route may really exit via a different ABR than the local SPF
//     assumes, so the alternate can micro-loop.
//   - Section 6.3(c/d): AS-external / ASBR routes reachable via more than one ASBR
//     (or more than one ABR) in non-backbone areas.
func suppressLFA(t RouteType, area types.AreaID, abrCount, asbrCount int, virtualLinks bool) bool {
	switch t {
	case RouteIntraArea:
		return false
	case RouteInterArea:
		if area == types.BackboneArea && virtualLinks {
			return true
		}
		return abrCount > 1
	case RouteExternalType1, RouteExternalType2:
		if area == types.BackboneArea && virtualLinks {
			return true
		}
		return asbrCount > 1 || abrCount > 1
	default:
		return true
	}
}
