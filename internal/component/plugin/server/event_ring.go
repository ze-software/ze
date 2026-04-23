// Design: plan/spec-diag-2-event-history.md -- global event ring buffer

package server

import (
	"sync"
	"time"
)

const defaultEventRingCapacity = 1024

// EventRecord is one entry in the global event ring.
type EventRecord struct {
	Timestamp time.Time
	Namespace string
	EventType string
}

// EventRing is a fixed-size circular buffer of EventRecord values.
// Safe for concurrent use. Append is O(1) with no allocation.
type EventRing struct {
	mu      sync.Mutex
	records []EventRecord
	head    int
	count   int
}

// NewEventRing creates a ring with the given capacity.
func NewEventRing(capacity int) *EventRing {
	if capacity <= 0 {
		capacity = defaultEventRingCapacity
	}
	return &EventRing{records: make([]EventRecord, capacity)}
}

// Append adds a record to the ring, overwriting the oldest if full.
func (r *EventRing) Append(namespace, eventType string) {
	now := time.Now()
	r.mu.Lock()
	r.records[r.head] = EventRecord{
		Timestamp: now,
		Namespace: namespace,
		EventType: eventType,
	}
	r.head = (r.head + 1) % len(r.records)
	if r.count < len(r.records) {
		r.count++
	}
	r.mu.Unlock()
}

// Snapshot returns up to limit records, newest first. If limit <= 0,
// returns all records. If namespace is non-empty, only matching records
// are returned.
func (r *EventRing) Snapshot(limit int, namespace string) []EventRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return []EventRecord{}
	}

	out := make([]EventRecord, 0, r.count)
	for i := range r.count {
		idx := (r.head - 1 - i + len(r.records)) % len(r.records)
		rec := r.records[idx]
		if namespace != "" && rec.Namespace != namespace {
			continue
		}
		out = append(out, rec)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// Count returns the number of records currently in the ring.
func (r *EventRing) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

// NamespaceCounts returns a map of namespace -> event count.
func (r *EventRing) NamespaceCounts() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()

	counts := make(map[string]int)
	start := (r.head - r.count + len(r.records)) % len(r.records)
	for i := range r.count {
		idx := (start + i) % len(r.records)
		counts[r.records[idx].Namespace]++
	}
	return counts
}
