// Design: docs/architecture/plugin/rib-storage-design.md -- receive validation gate
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md -- Section 5.7 retained ineligible paths
package adj_rib_in

import (
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/ze"
)

var validationBusPtr atomic.Pointer[ze.EventBus]

// routeEligible is the selection query, not a second copy of validation state.
func (r *AdjRIBInManager) routeEligible(key ribevents.ValidationRoute, msgID uint64) bool {
	if !isSimplePrefixFamily(key.Family) {
		return true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	rk := compactRouteKey{Fam: key.Family, Prefix: key.Prefix, PathID: key.PathID}
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	rk := compactRouteKey{Fam: key.Family, Prefix: key.Prefix, PathID: key.PathID}
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
func (r *AdjRIBInManager) noteValidationChange(peer netip.Addr, key compactRouteKey) {
	if r.validationBus == nil {
		return
	}
	r.validationChanges = append(r.validationChanges, ribevents.ValidationRoute{
		Peer: peer, Family: key.Fam, Prefix: key.Prefix, PathID: key.PathID,
	})
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
func (r *AdjRIBInManager) removeInstalled(peer netip.Addr, key compactRouteKey) {
	if routes := r.ribIn[peer]; routes != nil {
		routes.Delete(key)
	}
	r.noteValidationChange(peer, key)
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
}
