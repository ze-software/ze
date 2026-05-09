package l2tp

import (
	"testing"
)

func TestRawCaptureRingAppend(t *testing.T) {
	r := NewRawCaptureRing()
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

func TestRawCaptureRingOverflow(t *testing.T) {
	r := NewRawCaptureRing()
	for i := range rawCaptureSlots + 10 {
		r.Append(0, []byte{byte(i)})
	}
	snap := r.Snapshot(0)
	if len(snap) != rawCaptureSlots {
		t.Fatalf("count = %d, want %d", len(snap), rawCaptureSlots)
	}
}

func TestRawCaptureRingLimit(t *testing.T) {
	r := NewRawCaptureRing()
	for range 10 {
		r.Append(0, []byte{1})
	}
	snap := r.Snapshot(3)
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
}

func TestRawCaptureRingEmpty(t *testing.T) {
	r := NewRawCaptureRing()
	snap := r.Snapshot(0)
	if snap == nil {
		t.Fatal("snapshot should be non-nil empty slice")
	}
	if len(snap) != 0 {
		t.Fatalf("count = %d, want 0", len(snap))
	}
}

func TestRawCaptureRingTruncation(t *testing.T) {
	r := NewRawCaptureRing()
	big := make([]byte, rawCaptureSlotSize+100)
	for i := range big {
		big[i] = byte(i)
	}
	r.Append(0, big)
	snap := r.Snapshot(0)
	if len(snap[0].Data) != rawCaptureSlotSize {
		t.Errorf("data len = %d, want %d", len(snap[0].Data), rawCaptureSlotSize)
	}
}

func TestRawCaptureRingCopiesData(t *testing.T) {
	r := NewRawCaptureRing()
	orig := []byte{10, 20, 30}
	r.Append(0, orig)
	orig[0] = 99
	snap := r.Snapshot(0)
	if snap[0].Data[0] != 10 {
		t.Errorf("data[0] = %d, want 10 (should be independent copy)", snap[0].Data[0])
	}
}
