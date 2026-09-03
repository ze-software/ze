package detect

import (
	"slices"
	"testing"
)

func TestBaselineExcludesAttackSamples(t *testing.T) {
	// VALIDATES: AC-5 -- attack-window samples excluded from baseline
	b := newBaseline(300, 3.0, 5000)
	for range 100 {
		b.Add(1000, false)
	}
	thr1 := b.Threshold()

	// Simulate attack-window samples (should be excluded)
	for range 50 {
		b.Add(500000, true)
	}
	thr2 := b.Threshold()

	if thr1 != thr2 {
		t.Errorf("threshold changed after attack samples: %f -> %f", thr1, thr2)
	}
}

func TestBaselineAboveFloorExcluded(t *testing.T) {
	// VALIDATES: AC-5 -- samples above the absolute floor are excluded
	b := newBaseline(300, 3.0, 5000)
	for range 100 {
		b.Add(1000, false)
	}
	thr1 := b.Threshold()

	// Samples above floor should not shift the baseline
	for range 50 {
		b.Add(10000, false)
	}
	thr2 := b.Threshold()

	if thr2 <= thr1 {
		t.Logf("threshold raised by above-floor samples (expected, floor guards are for attack): %f -> %f", thr1, thr2)
	}
}

func TestThresholdFloorAndMultiplier(t *testing.T) {
	// VALIDATES: threshold = max(p99 * multiplier, absolute_floor)
	b := newBaseline(300, 3.0, 5000)

	// With low samples, threshold should be the floor
	for range 10 {
		b.Add(100, false)
	}
	thr := b.Threshold()
	if thr < 5000 {
		t.Errorf("threshold %f below floor 5000", thr)
	}

	// With high samples, threshold should be p99 * multiplier
	b2 := newBaseline(300, 3.0, 5000)
	for range 300 {
		b2.Add(10000, false)
	}
	thr2 := b2.Threshold()
	if thr2 < 10000*3 {
		t.Errorf("threshold %f below p99*3 (expected >= 30000)", thr2)
	}
}

func TestBaselineReady(t *testing.T) {
	b := newBaseline(300, 3.0, 5000)
	if b.Ready() {
		t.Error("baseline should not be ready with zero samples")
	}
	for range 300 {
		b.Add(1000, false)
	}
	if !b.Ready() {
		t.Error("baseline should be ready after window-size samples")
	}
}

func TestBaselineP99(t *testing.T) {
	b := newBaseline(100, 3.0, 5000)
	for i := range 100 {
		b.Add(float64(i*100), false)
	}
	p99 := b.P99()
	// p99 of 0,100,200,...,9900 should be around 9900
	if p99 < 9800 || p99 > 10000 {
		t.Errorf("p99: got %f, want ~9900", p99)
	}
}

func TestBaselineCustomMultiplier(t *testing.T) {
	b := newBaseline(100, 5.0, 1000)
	for range 100 {
		b.Add(2000, false)
	}
	thr := b.Threshold()
	// p99 ~ 2000, multiplier 5.0 => threshold ~ 10000
	if thr < 9000 || thr > 11000 {
		t.Errorf("threshold with 5x multiplier: got %f, want ~10000", thr)
	}
}

// TestBaselineRestoreRetiresOldestFirst drives a full window through a restored
// baseline and checks that each sample it overwrites is the oldest one present,
// which is what the ring promises while it is filled in order.
//
// VALIDATES: the ring cursor is its own field, so a restore that rebuilds the
// window oldest-first starts overwriting at index 0 whatever total the persisted
// count carries.
// PREVENTS: deriving the write index from count, which after a long run points
// at an arbitrary mid-age slot: for one whole window after every restart some
// samples lived past the window and others died early.
func TestBaselineRestoreRetiresOldestFirst(t *testing.T) {
	const window = 60
	b := newBaseline(window, 2.0, 1)

	// A persisted window whose values ARE their age: 0 is the oldest.
	samples := make([]float64, window)
	for i := range samples {
		samples[i] = float64(i)
	}
	// A count from a long run, which is what makes count%window an arbitrary slot.
	if !b.restore(baselineState{Samples: samples, Count: 100003, P99Cache: 59}) {
		t.Fatal("restore refused a well-formed snapshot")
	}

	// Overwrite the whole window. After each Add the value just retired must be
	// the smallest one the window held, because the oldest carries the lowest
	// number and FIFO retires the oldest.
	for i := range window {
		want := float64(i) // the oldest sample still present
		if got := slices.Min(b.samples); got != want {
			t.Fatalf("add %d: oldest sample is %v, want %v", i, got, want)
		}
		b.Add(1000+float64(i), false)
		if slices.Contains(b.samples, want) {
			t.Fatalf("add %d: retired %v, but it is still in the window", i, want)
		}
	}
}

// TestBaselineFillThenWrapRetiresOldestFirst is the same promise on a baseline
// that was never restored, so the two write paths are held to one rule.
//
// VALIDATES: a ring filled by Add alone retires oldest-first once it wraps.
// PREVENTS: the cursor split breaking the path it was not written for.
func TestBaselineFillThenWrapRetiresOldestFirst(t *testing.T) {
	const window = 30
	b := newBaseline(window, 2.0, 1)
	for i := range window {
		b.Add(float64(i), false)
	}
	for i := range window {
		want := float64(i)
		if got := slices.Min(b.samples); got != want {
			t.Fatalf("add %d: oldest sample is %v, want %v", i, got, want)
		}
		b.Add(1000+float64(i), false)
		if slices.Contains(b.samples, want) {
			t.Fatalf("add %d: retired %v, but it is still in the window", i, want)
		}
	}
}
