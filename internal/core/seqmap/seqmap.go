// Design: (none — internal utility, no architecture doc) — generic sequence-indexed map
//
// Package seqmap provides a key-value map with efficient range queries
// by monotonic sequence number. Not safe for concurrent use.
package seqmap

import "sort"

// compactMinLog is the minimum log size before auto-compaction is considered.
const compactMinLog = 256

type entry[K comparable, V any] struct {
	key  K
	seq  uint64
	val  V
	live bool
}

// Map is a key-value map supporting efficient range queries by sequence number.
// Sequence numbers SHOULD be assigned in non-decreasing order by the caller:
// that is the case Put makes cheap. An out-of-order sequence is placed at its
// sorted position instead of appended, so the log is sorted at all times and
// Since is correct for every caller whatever the order.
// Not safe for concurrent use; callers must synchronize externally.
//
// Keeping the log sorted rather than tolerating an unsorted one is deliberate.
// RecentUpdateCache (bgp/reactor) breaks the order for real: it takes the id
// from nextMsgID() and calls Add only after the ingress filter chain, on
// per-peer read goroutines. Its cumulative ack walks a Since range on the
// per-UPDATE path under an exclusive lock, so paying a short insert on the rare
// out-of-order arrival is far cheaper than dropping that walk to a full scan.
type Map[K comparable, V any] struct {
	items map[K]*entry[K, V]
	log   []*entry[K, V]
	dead  int
}

// New creates an empty Map.
func New[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		items: make(map[K]*entry[K, V]),
	}
}

// Put inserts or replaces a key with the given sequence number and value.
// If the key already exists, the old entry is logically deleted.
//
// A sequence at or above the last one appends, which is the whole cost for a
// caller that assigns them in non-decreasing order. A lower one is placed at
// its sorted position so the log never stops being sorted; see Map.
func (m *Map[K, V]) Put(key K, seq uint64, value V) {
	if old, ok := m.items[key]; ok {
		old.live = false
		m.dead++
	}
	e := &entry[K, V]{key: key, seq: seq, val: value, live: true}
	m.items[key] = e
	m.insertLog(e)
	m.maybeCompact()
}

// insertLog appends e, or slides it into place when its sequence is below the
// tail. Ties go after the entries they equal, which nothing depends on: it is
// the arrival order, and Since skips the dead entry a replaced key leaves.
func (m *Map[K, V]) insertLog(e *entry[K, V]) {
	n := len(m.log)
	if n == 0 || e.seq >= m.log[n-1].seq {
		m.log = append(m.log, e)
		return
	}
	i := sort.Search(n, func(i int) bool { return m.log[i].seq > e.seq })
	m.log = append(m.log, nil)
	copy(m.log[i+1:], m.log[i:])
	m.log[i] = e
}

// Delete removes a key. Returns true if the key existed.
func (m *Map[K, V]) Delete(key K) bool {
	e, ok := m.items[key]
	if !ok {
		return false
	}
	e.live = false
	m.dead++
	delete(m.items, key)
	m.maybeCompact()
	return true
}

// Get retrieves a value by key. Returns the value and true if found.
func (m *Map[K, V]) Get(key K) (V, bool) {
	e, ok := m.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	return e.val, true
}

// Len returns the number of live entries.
func (m *Map[K, V]) Len() int {
	return len(m.items)
}

// Clear removes all entries and resets internal state.
func (m *Map[K, V]) Clear() {
	clear(m.items)
	m.log = nil
	m.dead = 0
}

// Since calls fn for each live entry with sequence >= fromSeq, in ascending
// sequence order. If fn returns false, iteration stops early.
//
// The bound holds for every caller and every insert order, because Put keeps
// the log sorted. That matters rather than being tidy: RecentUpdateCache
// (bgp/reactor) acks each entry fn hands it, so an extra entry there is acked
// by a plugin that never handled it and a missing one strands that plugin's
// obligation and pins a pooled read buffer.
func (m *Map[K, V]) Since(fromSeq uint64, fn func(key K, seq uint64, value V) bool) {
	if len(m.log) == 0 {
		return
	}
	i := sort.Search(len(m.log), func(i int) bool {
		return m.log[i].seq >= fromSeq
	})
	for ; i < len(m.log); i++ {
		e := m.log[i]
		if !e.live {
			continue
		}
		if !fn(e.key, e.seq, e.val) {
			return
		}
	}
}

// Range calls fn for each live entry (unordered). If fn returns false,
// iteration stops early.
func (m *Map[K, V]) Range(fn func(key K, seq uint64, value V) bool) {
	for _, e := range m.items {
		if !fn(e.key, e.seq, e.val) {
			return
		}
	}
}

// maybeCompact triggers compaction when the dead-entry ratio is high enough.
func (m *Map[K, V]) maybeCompact() {
	if m.dead > len(m.log)/2 && len(m.log) > compactMinLog {
		m.compact()
	}
}

// compact rebuilds the log from live entries, sorted by sequence number.
func (m *Map[K, V]) compact() {
	newLog := make([]*entry[K, V], 0, len(m.items))
	for _, e := range m.items {
		newLog = append(newLog, e)
	}
	sort.Slice(newLog, func(i, j int) bool {
		return newLog[i].seq < newLog[j].seq
	})
	m.log = newLog
	m.dead = 0
}
