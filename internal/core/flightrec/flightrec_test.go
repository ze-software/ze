// Design: docs/architecture/diagnostics/production-diagnostics.md -- unit tests for Go flight recorder wrapper
// Related: flightrec.go -- flight recorder lifecycle and snapshot ring

package flightrec

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

// TestFlightRecStartCaptureRingAndCooldown exercises the full lifecycle of the
// flight recorder: disabled refusal, start/idempotent start, manual capture,
// ring eviction past 4 snapshots, SnapshotByID retrieval, automatic capture
// cooldown throttling, and stop.
//
// VALIDATES: AC-3, AC-4, R-1, R-3 -- flightrec starts/stops cleanly, retains at
// most 4 newest-first snapshots with non-empty trace bytes, throttles repeated
// CaptureAuto calls inside the cooldown window, and refuses CaptureNow when
// stopped.
// PREVENTS: silent capture failures, unbounded snapshot retention, capture
// storms under repeated panics, or missing ErrNotEnabled when stopped.
func TestFlightRecStartCaptureRingAndCooldown(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	if st := Status(); st.Enabled {
		t.Fatalf("expected recorder disabled initially, got %+v", st)
	}
	if _, err := CaptureNow("manual", "before-start"); !errors.Is(err, ErrNotEnabled) {
		t.Fatalf("CaptureNow while stopped: err = %v, want ErrNotEnabled", err)
	}
	if CaptureAuto("taskguard-panic", "before-start") {
		t.Fatal("CaptureAuto while stopped returned true, want false")
	}

	if err := Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if err := Start(); err != nil {
		t.Fatalf("second Start() must be idempotent, got: %v", err)
	}

	st := Status()
	if !st.Enabled {
		t.Fatalf("Status().Enabled = false after Start()")
	}
	if st.MaxSnapshots != 4 {
		t.Fatalf("Status().MaxSnapshots = %d, want 4", st.MaxSnapshots)
	}
	if st.MinAge <= 0 || st.MaxBytes == 0 {
		t.Fatalf("expected non-zero MinAge and MaxBytes, got %+v", st)
	}

	var captured []SnapshotMeta
	for i := range 5 {
		meta, err := CaptureNow("operator-request", string(rune('a'+i)))
		if err != nil {
			t.Fatalf("CaptureNow #%d failed: %v", i, err)
		}
		if meta.ID == "" || meta.Bytes <= 0 || meta.CapturedAt.IsZero() {
			t.Fatalf("CaptureNow #%d returned incomplete metadata: %+v", i, meta)
		}
		captured = append(captured, meta)
	}

	snaps := Snapshots()
	if len(snaps) != 4 {
		t.Fatalf("Snapshots() len = %d, want 4", len(snaps))
	}
	if snaps[0].ID != captured[4].ID {
		t.Errorf("newest snapshot ID = %q, want %q", snaps[0].ID, captured[4].ID)
	}
	if _, ok := SnapshotByID(captured[0].ID); ok {
		t.Errorf("oldest evicted snapshot %q should no longer be found", captured[0].ID)
	}
	latest, ok := SnapshotByID(captured[4].ID)
	if !ok {
		t.Fatalf("SnapshotByID(%q) returned ok=false", captured[4].ID)
	}
	if len(latest.Data) == 0 || len(latest.Data) != latest.Meta.Bytes {
		t.Fatalf("latest snapshot data len=%d, meta.Bytes=%d", len(latest.Data), latest.Meta.Bytes)
	}

	if !CaptureAuto("taskguard-panic", "bgp.read") {
		t.Fatal("first CaptureAuto should succeed outside cooldown")
	}
	if CaptureAuto("taskguard-panic", "bgp.read") {
		t.Fatal("second immediate CaptureAuto should be throttled by cooldown")
	}

	Stop()
	if Status().Enabled {
		t.Fatal("Status().Enabled = true after Stop()")
	}
	if _, err := CaptureNow("operator-request", "after-stop"); !errors.Is(err, ErrNotEnabled) {
		t.Fatalf("CaptureNow after Stop(): err = %v, want ErrNotEnabled", err)
	}
}

// TestFlightRecSnapshotByteCap verifies that limitedBuffer enforces the
// per-snapshot byte ceiling without overflowing or returning write errors, and
// that SnapshotByID returns an isolated copy of stored trace bytes.
//
// VALIDATES: AC-3, R-3 -- limitedBuffer caps stored bytes at maxSnapshotBytes
// and reports dropped bytes cleanly; mutating a returned Snapshot.Data slice
// does not corrupt the retained ring snapshot; CaptureAuto resumes after the
// cooldown expires.
// PREVENTS: an oversized trace window exhausting daemon memory or a caller
// mutating retained trace bytes in place.
func TestFlightRecSnapshotByteCap(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	var buf limitedBuffer
	buf.maxBytes = maxSnapshotBytes

	chunk := bytes.Repeat([]byte{0xab}, 1<<20)
	for i := range 6 {
		n, err := buf.Write(chunk)
		if err != nil {
			t.Fatalf("Write #%d failed: %v", i, err)
		}
		if n != len(chunk) {
			t.Fatalf("Write #%d returned n=%d, want %d", i, n, len(chunk))
		}
	}
	if buf.buf.Len() != maxSnapshotBytes {
		t.Fatalf("buffered len = %d, want %d", buf.buf.Len(), maxSnapshotBytes)
	}
	if buf.dropped != 2<<20 {
		t.Fatalf("dropped = %d, want %d", buf.dropped, 2<<20)
	}

	var small limitedBuffer
	small.maxBytes = 5
	n, err := small.Write([]byte("abcdefg"))
	if err != nil {
		t.Fatalf("partial Write failed: %v", err)
	}
	if n != 7 {
		t.Fatalf("partial Write returned n=%d, want 7", n)
	}
	if small.buf.String() != "abcde" {
		t.Fatalf("partial buffered = %q, want %q", small.buf.String(), "abcde")
	}
	if small.dropped != 2 {
		t.Fatalf("partial dropped = %d, want 2", small.dropped)
	}

	if err := Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	global.mu.Lock()
	global.cooldown = time.Millisecond
	global.mu.Unlock()

	if !CaptureAuto("test-trigger", "first") {
		t.Fatal("first CaptureAuto failed")
	}
	time.Sleep(5 * time.Millisecond)
	if !CaptureAuto("test-trigger", "after-cooldown") {
		t.Fatal("CaptureAuto after cooldown expired should succeed")
	}

	snaps := Snapshots()
	if len(snaps) != 2 {
		t.Fatalf("Snapshots() len = %d, want 2", len(snaps))
	}
	got, ok := SnapshotByID(snaps[0].ID)
	if !ok {
		t.Fatalf("SnapshotByID(%q) returned ok=false", snaps[0].ID)
	}
	origFirst := got.Data[0]
	got.Data[0] ^= 0xff
	again, ok := SnapshotByID(snaps[0].ID)
	if !ok {
		t.Fatalf("second SnapshotByID(%q) returned ok=false", snaps[0].ID)
	}
	if again.Data[0] != origFirst {
		t.Fatalf("SnapshotByID returned shared slice: got 0x%x, want 0x%x", again.Data[0], origFirst)
	}
}

// TestBlockProfileRateTracking verifies SetBlockProfileRate records the active
// rate and normalizes negative values to 0.
//
// VALIDATES: AC-1 -- SetBlockProfileRate and BlockProfileRate expose the
// configured block profile sampling rate for show system profile type block.
// PREVENTS: block profile responses omitting or misreporting sampling-rate.
func TestBlockProfileRateTracking(t *testing.T) {
	prev := BlockProfileRate()
	t.Cleanup(func() { SetBlockProfileRate(prev) })

	SetBlockProfileRate(100_000)
	if got := BlockProfileRate(); got != 100_000 {
		t.Fatalf("BlockProfileRate() = %d, want 100000", got)
	}
	SetBlockProfileRate(-5)
	if got := BlockProfileRate(); got != 0 {
		t.Fatalf("BlockProfileRate() after negative set = %d, want 0", got)
	}
}
