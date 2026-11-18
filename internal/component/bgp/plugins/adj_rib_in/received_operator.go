// Design: docs/architecture/plugin/rib-storage-design.md -- explicit address-selected validation.
// Related: rib_commands.go -- existing accept-routes and reject-routes commands.
package adj_rib_in

import (
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
)

// applyOperatorDecisionLocked preserves the explicit operator address aggregate.
// Every matching current owner is selected deliberately, then receives its own
// captured physical facts. An empty address creates only an observational early
// decision, never authority for an owner whose route has not arrived.
// Caller MUST hold mu and finish with unlockValidationCommand.
func (r *AdjRIBInManager) applyOperatorDecisionLocked(address netip.Addr, native compactRouteKey, decision *rpc.ValidationDecision) bool {
	sources := make(map[receivedPeer]struct{})
	for source, routes := range r.ribIn {
		if source.address != address {
			continue
		}
		if _, present := routes.Get(native); present {
			sources[source] = struct{}{}
		}
	}
	for key := range r.pending {
		if key.Peer.address != address {
			continue
		}
		if key.Route == native {
			sources[key.Peer] = struct{}{}
		}
	}
	if len(sources) == 0 {
		return r.applyDecision(receivedPeer{address: address}, native, decision)
	}
	early := false
	for source := range sources {
		route := r.validationStoredRouteLocked(source, native)
		exact := *decision
		exact.PeerOwner = source.owner
		exact.ConnectionID = route.ConnectionID
		if r.applyDecision(source, native, &exact) {
			early = true
		}
	}
	return early
}
