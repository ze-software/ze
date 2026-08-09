// Design: docs/architecture/diagnostics/packet-capture.md -- opt-in raw byte capture for pcap export

package l2tp

import (
	"sync"
	"time"
)

const (
	rawCaptureSlotSize = 1500
	rawCaptureSlots    = 256
)

type rawSlot struct {
	timestamp time.Time
	direction uint8
	length    int
	data      [rawCaptureSlotSize]byte
}

// RawCaptureEntry is the exported view of one raw packet.
type RawCaptureEntry struct {
	Timestamp time.Time
	Direction uint8
	Data      []byte
}

// RawCaptureRing stores raw packet bytes when activated.
// Append copies into fixed-size array slots (zero heap alloc).
// Safe for concurrent use.
type RawCaptureRing struct {
	mu    sync.Mutex
	slots []rawSlot
	head  int
	count int
}

// NewRawCaptureRing allocates a raw capture ring.
func NewRawCaptureRing() *RawCaptureRing {
	return &RawCaptureRing{slots: make([]rawSlot, rawCaptureSlots)}
}

// Append copies raw bytes into the ring. Truncates to slot size.
func (r *RawCaptureRing) Append(direction uint8, raw []byte) {
	n := min(len(raw), rawCaptureSlotSize)
	r.mu.Lock()
	s := &r.slots[r.head]
	s.timestamp = time.Now()
	s.direction = direction
	s.length = n
	copy(s.data[:n], raw[:n])
	r.head = (r.head + 1) % len(r.slots)
	if r.count < len(r.slots) {
		r.count++
	}
	r.mu.Unlock()
}

// Snapshot returns raw entries newest-first. limit <= 0 returns all.
func (r *RawCaptureRing) Snapshot(limit int) []RawCaptureEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return []RawCaptureEntry{}
	}
	out := make([]RawCaptureEntry, 0, r.count)
	for i := range r.count {
		idx := (r.head - 1 - i + len(r.slots)) % len(r.slots)
		s := &r.slots[idx]
		buf := make([]byte, s.length)
		copy(buf, s.data[:s.length])
		out = append(out, RawCaptureEntry{
			Timestamp: s.timestamp,
			Direction: s.direction,
			Data:      buf,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
