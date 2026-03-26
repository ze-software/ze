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
func (fp *fwdPool) BarrierPeer(ctx context.Context, peerAddr netip.Addr) error {
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
	// We use TryDispatch first (non-blocking), falling back to DispatchOverflow.
	// Both paths ensure the sentinel is queued behind existing items.
	for i := range targets {
		done := targets[i].done
		sentinel := fwdItem{
			done: func() { close(done) },
		}

		if !fp.TryDispatch(targets[i].key, sentinel) {
			// Channel full or pool stopped — use overflow.
			// If DispatchOverflow returns false (pool stopped), it calls
			// sentinel.done() which closes the done channel. The wait
			// loop below will see it immediately.
			fp.DispatchOverflow(targets[i].key, sentinel)
		}
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
