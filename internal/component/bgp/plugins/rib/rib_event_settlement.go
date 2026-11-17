// Design: docs/architecture/plugin/rib-storage-design.md -- deferred event settlement.
package rib

import "context"

// settleEvents joins effects deferred by already-applied events. The SDK runs
// this callback independently of event application and current-selection RPCs,
// so selected output and its nested receipts can finish while it waits. No
// peer, election, publication or storage lock is held across either barrier.
func (r *RIBManager) settleEvents(ctx context.Context) error {
	if err := r.selectedDispatch.barrier(ctx); err != nil {
		return err
	}
	// A selected write can release its final original-source reference. Join
	// those zero-reference leases after output, using the existing lease owner.
	return r.sourceLeases.settle(ctx)
}
