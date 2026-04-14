// Design: docs/architecture/wire/l2tp.md — L2TP reliable delivery engine
// RFC: rfc/short/rfc2661.md — RFC 2661 Section 5.8 out-of-order queueing
// Related: reliable.go — engine core that owns the reorder queue per tunnel

package l2tp

// reorderEntry is one payload delivered from the reorder queue, with its
// original Ns. The engine uses both the payload (for delivery) and the
// Ns (to advance nextRecvSeq).
type reorderEntry struct {
	ns      uint16
	payload []byte
}

// reorderQueue buffers control messages that arrived out of order --
// specifically, those with Ns > nextRecvSeq but within the advertised
// receive window. RFC 2661 Section 5.8: "Messages arriving out of order
// may be queued for in-order delivery when the missing messages are
// received, or they may be discarded". Ze takes the queued path.
//
// The queue uses a map keyed by Ns, bounded by capacity on each store.
// Accepted offsets from nextRecvSeq are (0, capacity); offset 0 is an
// in-order arrival handled by the engine directly, and offsets >=
// capacity are beyond the advertised window (peer violated our RWS or
// reordering is worse than one window -- either way, discarded).
//
// NOT safe for concurrent use. Callers (the reliable engine) run on the
// reactor goroutine and have exclusive access.
type reorderQueue struct {
	capacity uint16
	entries  map[uint16][]byte
}

// newReorderQueue constructs a reorder queue with the given capacity.
// capacity is taken from the engine's advertised recv_window, which is
// configurable; the unparam lint sees only the test values (all 4) but
// the engine will call with DefaultRecvWindow (16) or a configured value.
//
//nolint:unparam // capacity comes from config in reliable.go; tests exercise the boundary at 4
func newReorderQueue(capacity uint16) *reorderQueue {
	return &reorderQueue{
		capacity: capacity,
		entries:  make(map[uint16][]byte, capacity),
	}
}

// store buffers payload for later in-order delivery. Returns false if:
//   - Ns equals nextRecvSeq (offset 0 is in-order, not for this queue)
//   - Ns is at or beyond nextRecvSeq + capacity (outside advertised window)
//   - Ns is already buffered (duplicate of a reorder-queued message)
//
// Payload is copied defensively -- the caller may reuse its buffer.
func (q *reorderQueue) store(ns, nextRecvSeq uint16, payload []byte) bool {
	offset := ns - nextRecvSeq
	if offset == 0 || offset >= q.capacity {
		return false
	}
	if _, exists := q.entries[ns]; exists {
		return false
	}
	buf := make([]byte, len(payload))
	copy(buf, payload)
	q.entries[ns] = buf
	return true
}

// popInOrder returns all buffered entries whose Ns forms an unbroken
// sequence starting at nextRecvSeq. Stops at the first gap. Returned
// entries are removed from the queue. Callers advance their
// nextRecvSeq to (nextRecvSeq + len(out)) after consumption.
func (q *reorderQueue) popInOrder(nextRecvSeq uint16) []reorderEntry {
	var out []reorderEntry
	ns := nextRecvSeq
	for {
		payload, exists := q.entries[ns]
		if !exists {
			break
		}
		out = append(out, reorderEntry{ns: ns, payload: payload})
		delete(q.entries, ns)
		ns++
	}
	return out
}

// len reports the number of entries currently buffered. Used by the
// engine for observability and bounded-memory invariants.
func (q *reorderQueue) len() int { //nolint:unused // reserved for stats wiring in phase 7
	return len(q.entries)
}
