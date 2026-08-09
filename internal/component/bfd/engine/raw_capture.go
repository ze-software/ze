// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- BFD raw capture ring for pcap export
// Overview: engine.go -- Loop struct holds the atomic capture pointer

package engine

import (
	"sync"
	"time"
)

const (
	rawCaptureSlotSize = 512
	rawCaptureSlots    = 256
)

type rawSlot struct {
	timestamp time.Time
	direction uint8
	length    int
	data      [rawCaptureSlotSize]byte
}

// RawCaptureEntry is the exported view of one raw captured BFD packet.
type RawCaptureEntry struct {
	Timestamp string `json:"timestamp"`
	Direction string `json:"direction"`
	Data      []byte `json:"data"`
}

// RawCaptureRing stores raw BFD packet bytes when activated.
type RawCaptureRing struct {
	mu    sync.Mutex
	slots []rawSlot
	head  int
	count int
}

// NewRawCaptureRing allocates a BFD raw capture ring.
func NewRawCaptureRing() *RawCaptureRing {
	return &RawCaptureRing{slots: make([]rawSlot, rawCaptureSlots)}
}

// Append copies raw bytes into the ring.
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

	n := r.count
	if limit > 0 && limit < n {
		n = limit
	}
	out := make([]RawCaptureEntry, n)
	for i := range n {
		idx := (r.head - 1 - i + len(r.slots)) % len(r.slots)
		s := &r.slots[idx]
		dir := "in"
		if s.direction == 1 {
			dir = "out"
		}
		out[i] = RawCaptureEntry{
			Timestamp: s.timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Direction: dir,
			Data:      append([]byte(nil), s.data[:s.length]...),
		}
	}
	return out
}

// EnableRawCapture installs a capture ring on the Loop.
func (l *Loop) EnableRawCapture() {
	if l.rawCapture.Load() == nil {
		l.rawCapture.Store(NewRawCaptureRing())
	}
}

// DisableRawCapture removes the capture ring from the Loop.
func (l *Loop) DisableRawCapture() {
	l.rawCapture.Store(nil)
}

// RawCaptureSnapshot returns captured packets. Returns nil when capture is disabled.
func (l *Loop) RawCaptureSnapshot(limit int) []RawCaptureEntry {
	ring := l.rawCapture.Load()
	if ring == nil {
		return nil
	}
	return ring.Snapshot(limit)
}

func (l *Loop) captureRx(raw []byte) {
	if ring := l.rawCapture.Load(); ring != nil {
		ring.Append(0, raw)
	}
}

func (l *Loop) captureTx(raw []byte) {
	if ring := l.rawCapture.Load(); ring != nil {
		ring.Append(1, raw)
	}
}
