// Design: docs/architecture/core-design.md — forward pool barrier for deterministic flush
// Overview: forward_pool.go — per-peer forward worker pool

package reactor

import (
	"context"
	"net/netip"
)

// Barrier blocks until all workers have processed their queued items.
// Dispatches a sentinel fwdItem to each worker's channel. The sentinel carries
// only a done callback (no data). Workers process items in FIFO order, so
// reaching the sentinel guarantees all prior items have been written to the wire.
//
// Returns nil on success, or the context error if canceled/timed out.
// Returns nil immediately if the pool is stopped or has no workers.
func (fp *fwdPool) Barrier(ctx context.Context) error {
	return fp.barrier(ctx, nil)
}

// BarrierPeer blocks until the worker for a specific peer address has drained.
// Returns nil immediately if no worker exists for that peer.
func (fp *fwdPool) BarrierPeer(ctx context.Context, peerAddr netip.AddrPort) error {
	target := fwdKey{peerAddr: peerAddr}
	return fp.barrier(ctx, func(k fwdKey) bool { return k == target })
}

// barrier is the internal implementation for Barrier and BarrierPeer.
// If filter is nil, all workers are targeted. Otherwise, only workers
// whose key passes the filter are targeted.
func (fp *fwdPool) barrier(ctx context.Context, filter func(fwdKey) bool) error {
	fp.mu.Lock()
	if fp.stopped {
		fp.mu.Unlock()
		return nil
	}

	// Collect targeted workers and their keys.
	type target struct {
		key    fwdKey
		worker *fwdWorker
		done   chan struct{}
	}

	var targets []target
	for k, w := range fp.workers {
		if filter != nil && !filter(k) {
			continue
		}
		targets = append(targets, target{key: k, worker: w, done: make(chan struct{})})
	}
	fp.mu.Unlock()

	if len(targets) == 0 {
		return nil
	}

	// Dispatch a sentinel to each targeted worker.
	// The sentinel has no data (nil peer, no rawBodies/updates).
	// Its done callback signals completion.
	//
	// FIFO invariant: if the worker already has items in its overflow
	// buffer, the sentinel MUST go through overflow too. Using TryDispatch
	// would queue the sentinel in the channel ahead of overflow items the
	// worker hasn't drained yet, so the sentinel's done() would fire before
	// those items were processed and Barrier would return early. This was
	// the failure mode of TestFwdPool_Barrier_WithOverflow under
	// -race -count=20: batch1 = [item1], chan = [item2], overflow =
	// [item3, item4], Barrier.TryDispatch put sentinel at chan[1] ->
	// handler processes [item2, sentinel], sentinel.done() fires, item3
	// and item4 still in overflow.
	//
	// Only use TryDispatch when the worker's overflow is empty, which
	// guarantees the sentinel is queued behind every prior dispatch.
	for i := range targets {
		done := targets[i].done
		sentinel := fwdItem{
			done: func() { close(done) },
		}

		w := targets[i].worker
		w.overflowMu.Lock()
		overflowEmpty := len(w.overflow) == 0
		w.overflowMu.Unlock()

		if overflowEmpty {
			if fp.TryDispatch(targets[i].key, sentinel) {
				continue
			}
			// Channel was full; fall through to overflow path.
		}
		// Overflow had items (or channel was full): queue sentinel in
		// overflow so it is processed strictly after every dispatch that
		// preceded Barrier. If DispatchOverflow returns false (pool
		// stopped), it calls sentinel.done() itself.
		fp.DispatchOverflow(targets[i].key, sentinel)
	}

	// Wait for all sentinels to be processed.
	for _, tgt := range targets {
		select {
		case <-tgt.done:
			// Sentinel processed — all prior items for this worker are on the wire.
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}
