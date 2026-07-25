// Design: plan/learned/1031-ospf-ext-3-router-information.md -- OSPFv3 Router Information LSA origination.
// RFC: rfc/short/rfc7770.md -- sec 2.2 (OSPFv3 RI LSA = function code 12, U-bit set, per-scope
// wire type), sec 2.4 (Informational Capabilities TLV first), sec 2.7 (per-scope flooding).
// RFC: rfc/short/rfc5340.md -- App A.4.2.1 (U/S2/S1 LS Type), the native self-LSA seam.
//
// The OSPFv3 RI LSA is a NATIVE LSA (not opaque): function code 12 with the U-bit set so a
// non-supporting router still floods it (RFC 5340 sec 4.4.1). It is originated exactly like
// the OSPFv3 Router-LSA -- build the shared RI body, compute the key, call OriginateSelf --
// but keyed on the scope-specific LSType (area 0xA00C, AS 0xC00C). v6OriginateSelf calls
// v6OriginateRI on the same pass and adds its keys to the stale-flush keep set, so disabling
// RI (or removing a scope) MaxAge-flushes the RI LSA through the existing FlushStaleSelfLSAs
// path (AC-9). The body is identical to the OSPFv2 RI body (AC-11): both call buildRIInstances.

package ospf

import (
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v6RILSType returns the RFC 7770 sec 2.2 OSPFv3 Router Information wire LS Type for a
// flooding scope (U-bit set): 0x800C link-local, 0xA00C area, 0xC00C AS. OSPFv3 RI is
// originated only at area and AS scope; link scope falls back to the link wire type for the
// type layer (it is not originated here -- area+AS cover the deployment).
func v6RILSType(scope OpaqueScope) ospfv3types.LSType {
	switch scope {
	case OpaqueScopeLink:
		return ospfv3types.LSTypeRouterInformationLink
	case OpaqueScopeAS:
		return ospfv3types.LSTypeRouterInformationAS
	default:
		return ospfv3types.LSTypeRouterInformationArea
	}
}

// v6RIKey is the LSDB key for an OSPFv3 RI LSA at scope lsType and Instance ID inst: RFC 7770
// sec 2.2 assigns the whole 32-bit Link State ID as the Instance ID.
func v6RIKey(router types.RouterID, lsType ospfv3types.LSType, inst uint32) types.LSAKey {
	return types.LSAKey{Type: types.LSType(lsType), LinkStateID: v6SummaryLSID(inst), AdvertisingRouter: router}
}

// v6OriginateRI (re)originates this router's OSPFv3 RI LSA(s) for every configured flooding
// scope, adding each key to keep so v6OriginateSelf's stale flush retains them (and purges
// them when RI is disabled or a scope is removed, AC-9). Area scope originates into every
// active area; AS scope originates one AS-wide LSA plus, per RFC 7770 sec 2.7 SHOULD, an
// area-scoped RI LSA into each attached NSSA. It returns the number of LSAs (re)originated.
func (e *engine) v6OriginateRI(router types.RouterID, activeAreas []types.AreaID, keep map[ospflsdb.SelfLSARef]struct{}) int {
	if e.lsdb == nil {
		return 0
	}
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	ri := cfg.RouterInformation
	if !ri.Enabled {
		e.refreshRIMetrics()
		return 0
	}
	count := 0
	if ri.HasScope(OpaqueScopeArea) {
		for _, area := range activeAreas {
			count += e.v6OriginateRIScope(router, OpaqueScopeArea, area, keep)
		}
	}
	if ri.HasScope(OpaqueScopeAS) {
		count += e.v6OriginateRIScope(router, OpaqueScopeAS, types.BackboneArea, keep)
		for _, area := range v6NSSAActiveAreas(cfg, activeAreas) {
			count += e.v6OriginateRIScope(router, OpaqueScopeArea, area, keep)
		}
	}
	e.refreshRIMetrics()
	return count
}

// v6OriginateRIScope originates the RI LSA instances for one scope into one area (the
// backbone for AS scope, whose store routing sends the AS-scope LSType to the AS-wide store).
func (e *engine) v6OriginateRIScope(router types.RouterID, scope OpaqueScope, area types.AreaID, keep map[ospflsdb.SelfLSARef]struct{}) int {
	lsType := v6RILSType(scope)
	bodies := e.buildRIInstances(scope, router)
	count := 0
	for i := range bodies {
		body := bodies[i]
		inst := uint32(i)
		lsid := v6SummaryLSID(inst)
		key := v6RIKey(router, lsType, inst)
		_, ok := e.lsdb.OriginateSelf(area, key, body, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
			return v6SelfLSA(ospfv3packet.LSA{
				Header: v6OriginHeader(lsType, ospfv3types.LinkStateID(lsid), router, seq, purge),
				Body:   body,
			})
		})
		if ok {
			count++
			e.ri.originations.With(e.afLabel(), scope.String()).Inc()
		}
		keep[ospflsdb.SelfLSARef{Area: area, Key: key}] = struct{}{}
	}
	return count
}

// v6NSSAActiveAreas returns the attached NSSA areas among the active areas (RFC 7770 sec 2.7
// SHOULD: an AS-scoped RI router also advertises area-scoped RI into attached NSSAs).
func v6NSSAActiveAreas(cfg ospfConfig, active []types.AreaID) []types.AreaID {
	activeSet := make(map[types.AreaID]struct{}, len(active))
	for _, a := range active {
		activeSet[a] = struct{}{}
	}
	out := make([]types.AreaID, 0, len(cfg.Areas))
	for _, a := range cfg.Areas {
		if a.AreaType != areaTypeNSSA {
			continue
		}
		if _, ok := activeSet[a.AreaID]; ok {
			out = append(out, a.AreaID)
		}
	}
	return out
}
