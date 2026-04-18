// Design: docs/architecture/core-design.md — forward routing for route server
// Overview: server.go — route server plugin orchestration
// Related: server_withdrawal.go — withdrawal tracking

package rs

import (
	"context"
	"sort"
	"strings"
)

// forwardBatch accumulates forward items for batch RPC.
// Per-worker state: no concurrent access for a given workerKey.
//
// Invariants:
//   - targetBuf is a scratch buffer reused by selectForwardTargets each call;
//     its contents are valid only until the next selectForwardTargets call.
//   - targets is the immutable destination snapshot that applies to every id
//     in the current batch. Populated on the first accumulate of a new batch
//     and cleared (together with ids + selector) on every flush. The snapshot
//     is independent of targetBuf's backing array, so a later targetBuf
//     refresh cannot corrupt in-flight batch state.
type forwardBatch struct {
	ids       []uint64
	selector  string   // comma-joined target peers (batch.targets joined for equality check)
	targetBuf []string // scratch buffer for selectForwardTargets (not batch state)
	targets   []string // immutable destination snapshot for this batch (rs-fastpath-3)
}

// selectForwardTargets returns peers that should receive an UPDATE with the given families.
// A peer is included if it is up, is not the source, and supports at least one family
// in the UPDATE (or has nil Families, meaning unknown/all-accepted).
func (rs *RouteServer) selectForwardTargets(buf []string, sourcePeer string, families map[string]bool) []string {
	buf = buf[:0]
	for addr, peer := range rs.peers {
		if addr == sourcePeer || !peer.Up {
			continue
		}
		if peer.Families != nil {
			hasAny := false
			for fam := range families {
				if peer.SupportsFamily(fam) {
					hasAny = true
					break
				}
			}
			if !hasAny {
				continue
			}
		}
		buf = append(buf, addr)
	}
	sort.Strings(buf)
	return buf
}

// batchForwardUpdate accumulates a forward item into the per-worker batch.
// Selects targets, then appends to the current batch. Flushes the old batch
// if the target selector changes (different peer set). Flushes when the batch
// reaches maxBatchSize items. Partial batches are flushed by the onDrained
// callback when the worker channel empties.
func (rs *RouteServer) batchForwardUpdate(key workerKey, sourcePeer string, msgID uint64, families map[string]bool) {
	val, _ := rs.batches.LoadOrStore(key, &forwardBatch{})
	batch, ok := val.(*forwardBatch)
	if !ok {
		rs.releaseCache(msgID)
		return
	}

	rs.mu.RLock()
	batch.targetBuf = rs.selectForwardTargets(batch.targetBuf, sourcePeer, families)
	rs.mu.RUnlock()
	targets := batch.targetBuf

	if len(targets) == 0 {
		rs.releaseCache(msgID)
		return
	}

	sel := strings.Join(targets, ",")

	// Selector changed — flush old batch, start fresh.
	if batch.selector != "" && batch.selector != sel {
		rs.flushBatch(batch)
		batch.ids = batch.ids[:0]
		batch.selector = ""
		batch.targets = batch.targets[:0]
	}

	batch.ids = append(batch.ids, msgID)
	batch.selector = sel
	// Snapshot the destination list for the current selector. Reuse the
	// underlying array across flushes via batch.targets (reset on flush).
	if len(batch.targets) == 0 {
		batch.targets = append(batch.targets[:0], targets...)
	}

	// Flush on batch full.
	if len(batch.ids) >= maxBatchSize {
		rs.flushBatch(batch)
		batch.ids = batch.ids[:0]
		batch.selector = ""
		batch.targets = batch.targets[:0]
	}
}

// flushBatch sends the accumulated IDs via the reactor-owned ForwardCached
// primitive (rs-fastpath-3). Bypasses the text-command tokenise path; the
// engine dispatches directly to the reactor adapter.
func (rs *RouteServer) flushBatch(batch *forwardBatch) {
	if len(batch.ids) == 0 {
		return
	}

	if rs.forwardCachedHook != nil {
		rs.forwardCachedHook(batch.ids, batch.targets)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), forwardCachedTimeout)
	defer cancel()
	err := rs.plugin.ForwardCached(ctx, batch.ids, batch.targets)
	if err != nil { //nolint:gocritic // ifElseChain: switch blocked by block-silent-ignore hook
		if rs.stopping.Load() {
			logger().Debug("forward-cached failed (shutting down)", "ids", len(batch.ids), "error", err)
		} else if isConnectionError(err) {
			logger().Warn("forward-cached failed (peer disconnected)", "ids", len(batch.ids), "error", err)
		} else {
			logger().Error("forward-cached failed", "ids", len(batch.ids), "error", err)
		}
	}
}

// flushWorkerBatch flushes the batch for a given worker key.
// Called by the onDrained callback when the worker's channel empties.
func (rs *RouteServer) flushWorkerBatch(key workerKey) {
	val, loaded := rs.batches.Load(key)
	if !loaded {
		return
	}
	batch, ok := val.(*forwardBatch)
	if !ok {
		return
	}
	rs.flushBatch(batch)
	batch.ids = batch.ids[:0]
	batch.selector = ""
	batch.targets = batch.targets[:0]
}
