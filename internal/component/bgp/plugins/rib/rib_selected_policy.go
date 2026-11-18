// Design: docs/architecture/plugin/rib-storage-design.md -- learned policy reoffer.
// Related: rib_selected_dispatch.go -- bounded current-selection worker.
package rib

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// reofferLearned coalesces policy changes on one existing destination. The
// worker consumes the bit after its current RPC settles, then rereads only the
// current elected received routes. CurrentSelection stays a read-only query.
func (d *selectedDispatch) reofferLearned(peer netip.Addr, session uint64) error {
	if d == nil {
		return errors.New("selected dispatcher is unavailable for policy reconciliation")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fatal != nil {
		return d.fatal
	}
	if d.stopped {
		return errors.New("selected dispatcher stopped during policy reconciliation")
	}
	destination := d.destinations[selectedDestinationID{peer: peer, session: session}]
	if destination == nil {
		// A late old-Session event cannot create or rebind a destination.
		return nil
	}
	destination.reofferLearned = true
	d.signalLocked()
	return nil
}

// consumeExportPolicy returns before ordinary UP/source bookkeeping. Only the
// exact owner projection can enqueue work; observers acquire no replay right.
func (r *RIBManager) consumeExportPolicy(peer netip.Addr, up, eligible bool, session uint64, reason string) bool {
	if reason != rpc.ReasonExportPolicy {
		return false
	}
	if up && eligible && session != 0 {
		if err := r.selectedDispatch.reofferLearned(peer, session); err != nil {
			r.selectedDispatch.fail(err)
		}
	}
	return true
}
