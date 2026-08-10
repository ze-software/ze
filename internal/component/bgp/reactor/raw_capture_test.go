package reactor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/sim"
)

var rawTestStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestBGPRawCaptureRingAppend(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	r.Append(0, []byte{1, 2, 3})
	r.Append(1, []byte{4, 5, 6, 7})

	snap := r.Snapshot(0)
	if len(snap) != 2 {
		t.Fatalf("count = %d, want 2", len(snap))
	}
	if len(snap[0].Data) != 4 {
		t.Errorf("newest len = %d, want 4", len(snap[0].Data))
	}
	if snap[0].Direction != 1 {
		t.Errorf("newest direction = %d, want 1", snap[0].Direction)
	}
}

func TestBGPRawCaptureRingOverflow(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	for i := range bgpRawSlotCount + 10 {
		r.Append(0, []byte{byte(i)})
	}
	snap := r.Snapshot(0)
	if len(snap) != bgpRawSlotCount {
		t.Fatalf("count = %d, want %d", len(snap), bgpRawSlotCount)
	}
}

func TestBGPRawCaptureRingLimit(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	for range 10 {
		r.Append(0, []byte{1})
	}
	snap := r.Snapshot(3)
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
}

func TestBGPRawCaptureRingEmpty(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	snap := r.Snapshot(0)
	if snap == nil {
		t.Fatal("snapshot should be non-nil empty slice")
	}
	if len(snap) != 0 {
		t.Fatalf("count = %d, want 0", len(snap))
	}
}

func TestBGPRawCaptureRingTruncation(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	big := make([]byte, bgpRawSlotSize+100)
	r.Append(0, big)
	snap := r.Snapshot(0)
	if len(snap[0].Data) != bgpRawSlotSize {
		t.Errorf("data len = %d, want %d", len(snap[0].Data), bgpRawSlotSize)
	}
}
