// Design: docs/architecture/plugin/rib-storage-design.md -- source mutation receipts.
// Related: source_policy.go, rib_commands.go -- settle output before releasing source rights.
package rib

import (
	"context"
	"errors"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// settleSourceOutput MUST follow election/publication and MUST precede source
// lease release. Captured workers cover an old winner that had no wire receipt;
// the post-mutation source barrier also covers newly admitted output. No RIB or
// election lock may be held. A refused join MUST retain the release obligation.
func (r *RIBManager) settleSourceOutput(ctx context.Context, active []*selectedDestination, receipts []uint64, release func()) error {
	err := r.selectedDispatch.barrierDestinations(ctx, active)
	post, captureErr := r.selectedDispatch.sourceDestinations(receipts)
	releasePost := r.sourceLeases.holdSourceOutput(receipts, post)
	defer releasePost()
	err = errors.Join(err, captureErr)
	if err == nil {
		err = r.selectedDispatch.barrierDestinations(ctx, post)
	}
	releasePost()
	release()
	if err != nil {
		return err
	}
	return r.sourceLeases.settleReceipts(ctx, receipts)
}

// settlePurgedSource bounds the command/timer path by the existing cancellable
// operation budget and the selecting process lifetime. Its caller MUST capture
// workers before removing input, then publish every replacement before calling.
func (r *RIBManager) settlePurgedSource(active []*selectedDestination, receipts []uint64, release func()) error {
	parent := context.Background()
	if r.selectedDispatch != nil {
		parent = r.selectedDispatch.ctx
	}
	ctx, cancel := context.WithTimeout(parent, rpc.CancellableRequestTimeout)
	defer cancel()
	return r.settleSourceOutput(ctx, active, receipts, release)
}
