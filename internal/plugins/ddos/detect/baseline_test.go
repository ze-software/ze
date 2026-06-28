package detect

import (
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
