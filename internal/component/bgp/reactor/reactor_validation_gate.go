// Design: docs/architecture/plugin/rib-storage-design.md -- validation eligibility between receive stores
// Related: forward_validation.go -- the forward path that reads ribevents.RouteEligible

package reactor

import (
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// subscribeValidationGate applies every bgp-rib/validation-gate announcement
// to the daemon's receive gate (ribevents.NoteValidationGate). The gate's
// owner may run forked, where its eligibility lookup cannot be registered in
// this process; the announcement is then what makes the forward path refuse
// routes instead of forwarding every one as eligible.
//
// It runs from StartWithContext beside subscribeDaemonSignals, before the
// plugins that depend on BGP (bgp-adj-rib-in, bgp-rpki) run their startup,
// because a subscription delivers future events only and bgp-rpki enables
// validation while plugins start. The unsubscribe joins eventBusUnsubs
// (releaseEventSubscriptions).
func (r *Reactor) subscribeValidationGate() {
	if r.eventBus == nil {
		return
	}
	unsubscribe := ribevents.ValidationGate.Subscribe(r.eventBus, ribevents.NoteValidationGate)
	r.eventBusUnsubs = append(r.eventBusUnsubs, unsubscribe)
}
