// Design: docs/architecture/plugin/rib-storage-design.md -- receive validation gate
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md -- Section 5.7 retained ineligible paths
package adj_rib_in

import (
	"encoding/hex"
	"net/netip"
	"sync/atomic"
	"unique"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/ze"
)

var validationBusPtr atomic.Pointer[ze.EventBus]

// validationRouteKey is the store key of the route a selection query names:
// the prefix for a family keyed by CIDR, and the family's route identity,
// decoded from route.NLRI the way routeKeyFromNLRI decodes a received NLRI,
// for every other. ok is false when the query names no route of its family
// (no NLRI, or one the family's key function refuses), so the caller answers
// for a route it could not find rather than for one it found.
func validationRouteKey(route ribevents.ValidationRoute) (key compactRouteKey, ok bool) {
	if isSimplePrefixFamily(route.Family) {
		return compactRouteKey{Fam: route.Family, Prefix: route.Prefix, PathID: route.PathID}, true
	}
	if nlrisplit.KeysByCIDR(route.Family) {
		return compactRouteKey{Fam: route.Family, Prefix: route.Prefix, PathID: route.PathID}, true
	}
	if route.NLRI == "" {
		return compactRouteKey{}, false
	}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	identity, err := nlrisplit.GetPrefixKey(route.Family)([]byte(route.NLRI), scratch[:], false)
	if err != nil {
		return compactRouteKey{}, false
	}
	return compactRouteKey{Fam: route.Family, PathID: route.PathID, NLRI: unique.Make(string(identity))}, true
}

// routeEligible is the selection query, not a second copy of validation state.
// A route the store does not hold, or a query that names no route, is not
// eligible: the answer is never assumed for a family the store keys otherwise.
//
// FlowSpec is the one family this store has no verdict for: it never stores a
// FlowSpec rule (a rule carries no next hop, so installStructuredNLRIs skips
// it), and ribevents.RouteEligible asks the FlowSpec provider, bgp-rib, first.
// That provider applies the source-policy denial to FlowSpec authorization
// (rib-storage-design.md, "Source-policy lifecycle owners").
func (r *AdjRIBInManager) routeEligible(key ribevents.ValidationRoute, msgID uint64) bool {
	// refuse-by-default: guards ribevents.RouteEligible -- the FlowSpec provider already refused every rule it did not authorize, and this store holds none
	if ribevents.IsFlowSpec(key.Family) {
		return true
	}
	rk, named := validationRouteKey(key)
	if !named {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, pending := r.pending[pendingKey(key.Peer, rk)]; pending {
		return false
	}
	routes := r.ribIn[key.Peer]
	if routes == nil {
		return false
	}
	route, ok := routes.Get(rk)
	if !ok {
		return false
	}
	// RFC 8212 Section 3: an applied import denial excludes the route.
	if r.sourcePolicyDeniedLocked(key.Peer, route.SourcePolicyEpoch) {
		return false
	}
	if newer := r.earlyDecisions[pendingKey(key.Peer, rk)]; newer != nil && newer.msgID > route.MsgID {
		return false
	}
	if r.validationEnabled && route.Ineligible {
		return false
	}
	if msgID != 0 {
		return route.MsgID == msgID
	}
	return true
}

// routePresent keeps regenerated path identities owned while validation only
// suppresses advertisement. Cache eviction may query it; no cache operation or
// external callback may run under this receive-store lock.
func (r *AdjRIBInManager) routePresent(key ribevents.ValidationRoute) bool {
	rk, named := validationRouteKey(key)
	if !named {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, pending := r.pending[pendingKey(key.Peer, rk)]; pending {
		return true
	}
	if routes := r.ribIn[key.Peer]; routes != nil {
		_, present := routes.Get(rk)
		return present
	}
	return false
}

// noteValidationChange MUST run under mu. The caller MUST finish with
// unlockValidation, never mu.Unlock, so downstream selection sees the change.
//
// Nothing is noted until the eligibility gate is registered. Without it,
// ribevents.RouteEligible answers true for every path, so the live forward
// already carried each route and no verdict changed. A ValidationChange then
// makes the route server and reflector replay the path to every target, which
// sends each UPDATE a second time and re-announces a route the next UPDATE
// withdraws.
//
// A route keyed by its NLRI identity is named by its native NLRI, the form
// validationRouteKey decodes, so a subscriber that asks the selection query
// with the notified key finds the route. The caller MUST note such a route
// while it is still stored: a removal notes before it deletes.
func (r *AdjRIBInManager) noteValidationChange(peer netip.Addr, key compactRouteKey) {
	if r.validationBus == nil || !r.validationGate {
		return
	}
	change := ribevents.ValidationRoute{Peer: peer, Family: key.Fam, Prefix: key.Prefix, PathID: key.PathID}
	if key.keyedByNLRI() {
		nlri, named := r.validationNLRILocked(peer, key)
		if !named {
			return
		}
		change.NLRI = nlri
	}
	r.validationChanges = append(r.validationChanges, change)
}

// validationNLRILocked is the native NLRI, without ADD-PATH, of the route key
// names: the bytes the stored or pending route carries. named is false when
// this store holds no such route. Its eligibility was then false and stays
// false, since routeEligible refuses a key it cannot find, so nothing changed
// to announce. It is also false, and logged, when the stored bytes do not
// decode back to key: a ValidationRoute no subscriber could resolve is never
// emitted. MUST hold mu.
func (r *AdjRIBInManager) validationNLRILocked(peer netip.Addr, key compactRouteKey) (nlri string, named bool) {
	var route *RawRoute
	if pending := r.pending[pendingKey(peer, key)]; pending != nil {
		route = pending.route
	} else if routes := r.ribIn[peer]; routes != nil {
		route, _ = routes.Get(key)
	}
	if route == nil {
		return "", false
	}
	wire, err := hex.DecodeString(route.NLRIHex)
	if err != nil {
		logger().Error("validation change not announced: the stored route's NLRI is not hex, so no subscriber could find the route; this is a bgp-adj-rib-in defect, report it with this line",
			"peer", peer, "family", key.Fam, "route", key.routeName(), "error", err)
		return "", false
	}
	change := ribevents.ValidationRoute{Peer: peer, Family: key.Fam, PathID: key.PathID, NLRI: string(wire)}
	if resolved, ok := validationRouteKey(change); !ok || resolved != key {
		logger().Error("validation change not announced: the stored route's NLRI does not name its own store key, so no subscriber could find the route; this is a bgp-adj-rib-in defect, report it with this line",
			"peer", peer, "family", key.Fam, "route", key.routeName(), "nlri", route.NLRIHex)
		return "", false
	}
	return change.NLRI, true
}

// unlockValidation MUST follow every mutation that calls noteValidationChange.
// Notifications carry keys only: subscribers read the current state even when
// another mutation overtakes this emission. No store lock crosses the event bus.
func (r *AdjRIBInManager) unlockValidation() {
	changes := r.validationChanges
	r.validationChanges = nil
	r.mu.Unlock()
	if len(changes) == 0 {
		return
	}
	if _, err := ribevents.ValidationChange.Emit(r.validationBus, changes); err != nil {
		logger().Error("validation selection notification failed", "error", err)
	}
}

// removeInstalled also invalidates a replaced path before its replacement has
// a validation result. The old path must not remain selectable or replayable.
//
// The change is noted before the delete, while the route still supplies the
// NLRI that names it (noteValidationChange).
func (r *AdjRIBInManager) removeInstalled(peer netip.Addr, key compactRouteKey) {
	r.noteValidationChange(peer, key)
	if routes := r.ribIn[peer]; routes != nil {
		routes.Delete(key)
	}
}

func (r *AdjRIBInManager) removePeerInstalled(peer netip.Addr) {
	if routes := r.ribIn[peer]; routes != nil {
		routes.Range(func(key compactRouteKey, _ uint64, _ *RawRoute) bool {
			r.noteValidationChange(peer, key)
			return true
		})
	}
	delete(r.ribIn, peer)
	delete(r.validationPeers, peer)
	r.dropSourcePolicyLocked(peer)
}
