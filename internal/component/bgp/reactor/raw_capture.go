// Design: docs/architecture/diagnostics/packet-capture.md -- opt-in raw byte capture for pcap export

package reactor

import (
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
)

const (
	bgpRawSlotSize  = 4096
	bgpRawSlotCount = 256
)

type bgpRawSlot struct {
	timestamp time.Time
	direction uint8
	length    int
	data      [bgpRawSlotSize]byte
}

// BGPRawCaptureEntry is the exported view of one raw packet.
type BGPRawCaptureEntry struct {
	Timestamp time.Time
	Direction uint8
	Data      []byte
}

// BGPRawCaptureRing stores raw BGP message bytes when activated.
// Append copies into fixed-size array slots. Messages exceeding
// bgpRawSlotSize are truncated (extended messages up to 65535 bytes
// lose the tail, but headers and most attributes are preserved).
// Safe for concurrent use.
type BGPRawCaptureRing struct {
	mu    sync.Mutex
	clock clock.Clock
	slots []bgpRawSlot
	head  int
	count int
}

// newBGPRawCaptureRing allocates a raw capture ring.
func newBGPRawCaptureRing(c clock.Clock) *BGPRawCaptureRing {
	return &BGPRawCaptureRing{clock: c, slots: make([]bgpRawSlot, bgpRawSlotCount)}
}

// Append copies raw bytes into the ring. Truncates to slot size.
func (r *BGPRawCaptureRing) Append(direction uint8, raw []byte) {
	n := min(len(raw), bgpRawSlotSize)
	now := r.clock.Now()
	r.mu.Lock()
	s := &r.slots[r.head]
	s.timestamp = now
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
func (r *BGPRawCaptureRing) Snapshot(limit int) []BGPRawCaptureEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return []BGPRawCaptureEntry{}
	}
	out := make([]BGPRawCaptureEntry, 0, r.count)
	for i := range r.count {
		idx := (r.head - 1 - i + len(r.slots)) % len(r.slots)
		s := &r.slots[idx]
		buf := make([]byte, s.length)
		copy(buf, s.data[:s.length])
		out = append(out, BGPRawCaptureEntry{
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
