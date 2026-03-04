// Design: docs/architecture/core-design.md — forward routing for route server
// Overview: server.go — route server plugin orchestration
// Related: server_withdrawal.go — withdrawal tracking

package bgp_rs

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// forwardBatch accumulates forward items for batch RPC.
// Per-worker state: no concurrent access for a given workerKey.
type forwardBatch struct {
	ids       []uint64
	selector  string   // comma-joined target peers
	targetBuf []string // reusable buffer for selectForwardTargets
}

// forwardCmd is a single fire-and-forget forward RPC to be sent by the background sender.
type forwardCmd struct {
	peer string
	cmd  string
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
			for family := range families {
				if peer.SupportsFamily(family) {
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
	}

	batch.ids = append(batch.ids, msgID)
	batch.selector = sel

	// Flush on batch full.
	if len(batch.ids) >= maxBatchSize {
		rs.flushBatch(batch)
		batch.ids = batch.ids[:0]
		batch.selector = ""
	}
}

// flushBatch sends a single batched cache-forward RPC for all accumulated IDs.
// Uses asyncForward (fire-and-forget) so the worker goroutine doesn't block
// waiting for the engine's RPC response.
func (rs *RouteServer) flushBatch(batch *forwardBatch) {
	if len(batch.ids) == 0 {
		return
	}

	// Single ID — use existing format (no comma).
	if len(batch.ids) == 1 {
		rs.asyncForward("*", fmt.Sprintf("bgp cache %d forward %s", batch.ids[0], batch.selector))
		return
	}

	// Multiple IDs — comma-separated batch format.
	idStrs := make([]string, len(batch.ids))
	for i, id := range batch.ids {
		idStrs[i] = strconv.FormatUint(id, 10)
	}
	rs.asyncForward("*", fmt.Sprintf("bgp cache %s forward %s", strings.Join(idStrs, ","), batch.selector))
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
}
