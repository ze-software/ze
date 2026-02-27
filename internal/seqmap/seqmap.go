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
// Sequence numbers must be assigned in non-decreasing order by the caller.
// Not safe for concurrent use; callers must synchronize externally.
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
// Sequence numbers must be non-decreasing across calls.
func (m *Map[K, V]) Put(key K, seq uint64, value V) {
	if old, ok := m.items[key]; ok {
		old.live = false
		m.dead++
	}
	e := &entry[K, V]{key: key, seq: seq, val: value, live: true}
	m.items[key] = e
	m.log = append(m.log, e)
	m.maybeCompact()
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
	m.items = make(map[K]*entry[K, V])
	m.log = nil
	m.dead = 0
}

// Since calls fn for each live entry with sequence >= fromSeq, in ascending
// sequence order. If fn returns false, iteration stops early.
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
