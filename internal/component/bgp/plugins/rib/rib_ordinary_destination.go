// Design: docs/architecture/plugin/rib-storage-design.md -- authenticated ordinary destinations.
// Related: rib_selected_dispatch.go -- one joined worker per captured destination Session.
package rib

import (
	"context"
	"net/netip"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// updateSelectedDestination consumes only the engine's per-recipient state
// facts. Callers MUST invoke it on every state delivery, before taking peer or
// election locks. An ordinary UP MUST bypass legacy SENT-history replay; the
// selected worker owns its current snapshot and final-receipt readiness.
func (r *RIBManager) updateSelectedDestination(peer netip.Addr, up, eligible bool, owner, session uint64) bool {
	d := r.selectedDispatch
	if d == nil {
		return false
	}
	if up {
		if !eligible {
			return false
		}
		if err := d.destinationOwnerUp(peer, owner, session); err != nil {
			// A claimed ordinary destination without a valid captured Session
			// cannot safely fall back to the legacy forwarding authority.
			d.fail(err)
		}
		return true
	}
	if session == 0 {
		return false // A legacy observer has no ordinary connection to retire.
	}

	// DOWN MUST join only the worker named by the ended Session receipt. A
	// delayed predecessor event cannot cancel a replacement at this address.
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	err := d.destinationDown(ctx, peer, session)
	cancel()
	if err != nil {
		d.mu.Lock()
		if destination := d.destinations[selectedDestinationID{peer: peer, session: session}]; destination != nil {
			destination.failure = err
			d.signalChangeLocked()
			d.signalReportLocked()
		}
		d.mu.Unlock()
	}
	return false
}
