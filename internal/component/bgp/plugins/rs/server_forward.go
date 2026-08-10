// Design: docs/architecture/core-design.md — forward routing for route server
// Overview: server.go — route server plugin orchestration
// Related: server_withdrawal.go — withdrawal tracking

package rs

import (
	"context"
	"net/netip"
	"sort"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// batchForwardUpdateSkipped forwards a cached UPDATE to only the peers that
// the reactor fast path skipped (those with ExportFilters). Called when
// ReactorForwarded is true and FastPathSkipped is non-empty.
func (rs *routeServer) batchForwardUpdateSkipped(key workerKey, sourcePeer string, msgID uint64, families map[family.Family]bool, skipped []netip.AddrPort) {
	val, _ := rs.batches.LoadOrStore(key, &forwardBatch{})
	batch, ok := val.(*forwardBatch)
	if !ok {
		rs.releaseCache(msgID)
		return
	}

	// Build target list from skipped peers only (excluding source).
	rs.mu.RLock()
	batch.targetBuf = batch.targetBuf[:0]
	for _, addrPort := range skipped {
		addr := addrPort.Addr().String()
		if addr == sourcePeer {
			continue
		}
		peer := rs.peers[addr]
		if peer == nil || !peer.Up {
			continue
		}
		// Same peer-up cut as selectForwardTargets: this rail carries the peers
		// the reactor fast path skipped, not a different forwarding policy.
		if msgID != 0 && msgID <= peer.ForwardFrom {
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
		batch.targetBuf = append(batch.targetBuf, addr)
	}
	rs.mu.RUnlock()
	targets := batch.targetBuf

	if len(targets) == 0 {
		rs.releaseCache(msgID)
		return
	}

	sort.Strings(targets)
	sel := textbuf.Join(targets, ",")

	if batch.selector != "" && batch.selector != sel {
		rs.flushBatch(batch)
		batch.ids = batch.ids[:0]
		batch.selector = ""
		batch.targets = batch.targets[:0]
	}

	batch.ids = append(batch.ids, msgID)
	batch.selector = sel
	if len(batch.targets) == 0 {
		batch.targets = append(batch.targets[:0], targets...)
	}

	if len(batch.ids) >= maxBatchSize {
		rs.flushBatch(batch)
		batch.ids = batch.ids[:0]
		batch.selector = ""
		batch.targets = batch.targets[:0]
	}
}

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
// A peer is included if it is up, is not the source, supports at least one family
// in the UPDATE (or has nil Families, meaning unknown/all-accepted), and the UPDATE
// is NEWER than the peer's peer-up cut.
//
// msgID is the reactor MessageID of the UPDATE being forwarded. A peer whose
// ForwardFrom is at or above it was not yet a live forward target when this
// UPDATE was taken delivery of, so the UPDATE belongs to that peer's Adj-RIB-In
// replay instead. Excluding it here is what makes the two rails disjoint: the
// same route reaching a peer twice, in scheduling order, was the duplicate; a
// replayed announcement overtaking a live withdrawal of the same prefix would
// have resurrected a withdrawn route.
func (rs *routeServer) selectForwardTargets(buf []string, sourcePeer string, msgID uint64, families map[family.Family]bool) []string {
	buf = buf[:0]
	for addr, peer := range rs.peers {
		if addr == sourcePeer || !peer.Up {
			continue
		}
		if msgID != 0 && msgID <= peer.ForwardFrom {
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

// explainNoTarget names, per peer, why selectForwardTargets returned nothing.
//
// Caller must hold rs.mu. Built only on the discard path, so the cost is paid
// only when an UPDATE is about to be dropped for every destination.
func (rs *routeServer) explainNoTarget(sourcePeer string, msgID uint64, families map[family.Family]bool) string {
	var b textbuf.Buffer
	first := true
	for addr, peer := range rs.peers {
		if !first {
			b.Str(", ")
		}
		first = false
		b.Str(addr).Byte('=')
		switch {
		case addr == sourcePeer:
			b.Str("source")
		// The two !Up cases are split because the log line exists to tell them
		// apart (see the caller: "the two causes need opposite fixes"), and
		// PeerState.StateSeen is what distinguishes them. Printing one label for
		// both sent a reader chasing a peer-up/replay ordering bug when the peer
		// had simply closed the session and there was no destination left --
		// which is what a check-mode test peer does the moment its rule matches.
		case !peer.Up && peer.StateSeen:
			b.Str("down")
		case !peer.Up:
			b.Str("not-yet-up")
		case msgID != 0 && msgID <= peer.ForwardFrom:
			b.Str("below-cut(").Uint(peer.ForwardFrom).Byte(')')
		default:
			b.Str("family-mismatch(")
			for fam := range families {
				b.Str(fam.String()).Byte(' ')
			}
			b.Byte(')')
		}
	}
	if first {
		return "no peers registered"
	}
	return b.String()
}

// batchForwardUpdate accumulates a forward item into the per-worker batch.
// Selects targets, then appends to the current batch. Flushes the old batch
// if the target selector changes (different peer set). Flushes when the batch
// reaches maxBatchSize items. Partial batches are flushed by the onDrained
// callback when the worker channel empties.
func (rs *routeServer) batchForwardUpdate(key workerKey, sourcePeer string, msgID uint64, families map[family.Family]bool) {
	val, _ := rs.batches.LoadOrStore(key, &forwardBatch{})
	batch, ok := val.(*forwardBatch)
	if !ok {
		rs.releaseCache(msgID)
		return
	}

	rs.mu.RLock()
	batch.targetBuf = rs.selectForwardTargets(batch.targetBuf, sourcePeer, msgID, families)
	known := len(rs.peers)
	var why string
	if len(batch.targetBuf) == 0 {
		why = rs.explainNoTarget(sourcePeer, msgID, families)
	}
	rs.mu.RUnlock()
	targets := batch.targetBuf

	if len(targets) == 0 {
		// Say it: this arm discards the UPDATE for every destination, and the
		// only rail that can recover it is the announce-only Adj-RIB-In replay,
		// so a withdrawal discarded here is lost outright
		// (ai/rules/evidence.md).
		//
		// "peers-known" alone was not enough to act on: it says how many peers
		// exist, not why each was skipped, and the two causes need opposite
		// fixes. Not-yet-Up means the peer-up event has not reached rs, so the
		// replay owns the UPDATE. Below-cut means rs had already advanced past
		// this MessageID when it registered, which is the cut doing its job.
		logger().Warn("forward matched no target",
			"source-peer", sourcePeer, "msg-id", msgID, "peers-known", known, "why", why)
		rs.releaseCache(msgID)
		return
	}

	sel := textbuf.Join(targets, ",")

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
func (rs *routeServer) flushBatch(batch *forwardBatch) {
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
func (rs *routeServer) flushWorkerBatch(key workerKey) {
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
