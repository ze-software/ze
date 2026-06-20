package engine

import (
	"math"
	"testing"
	"time"
)

// VALIDATES: RFC 5880 Section 6.8.7 -- for bfd.DetectMult >= 2 the
// per-packet reduction is drawn from [0, 25%) of the base interval.
// Every draw MUST be non-negative and strictly less than 25%.
// PREVENTS: regression where jitter exceeds 25% and causes the receiver
// to time out, or goes negative and pushes the deadline backwards.
func TestApplyJitterDetectMultDefault(t *testing.T) {
	loop := NewLoop(nil, nil)
	const base = 300_000 * time.Microsecond
	const upper = time.Duration(float64(base) * JitterMaxFraction)

	const draws = 10_000
	for i := range draws {
		got := loop.applyJitter(base, 3)
		if got < 0 {
			t.Fatalf("draw %d: negative reduction %v", i, got)
		}
		if got >= upper {
			t.Fatalf("draw %d: reduction %v >= upper bound %v (base %v)", i, got, upper, base)
		}
	}
}

// VALIDATES: RFC 5880 Section 6.8.7 -- for bfd.DetectMult == 1 the
// reduction is drawn from [10%, 25%) so the transmitted interval sits in
// [75%, 90%] of the base.
// PREVENTS: regression where the DetectMult==1 branch allows reductions
// below 10%, letting the receiver detect before the next packet arrives.
func TestApplyJitterDetectMultOne(t *testing.T) {
	loop := NewLoop(nil, nil)
	const base = 300_000 * time.Microsecond
	lower := time.Duration(float64(base) * JitterMinFractionDetectMultOne)
	upper := time.Duration(float64(base) * JitterMaxFraction)

	const draws = 10_000
	for i := range draws {
		got := loop.applyJitter(base, 1)
		if got < lower {
			t.Fatalf("draw %d: reduction %v < lower bound %v (base %v)", i, got, lower, base)
		}
		if got >= upper {
			t.Fatalf("draw %d: reduction %v >= upper bound %v (base %v)", i, got, upper, base)
		}
	}
}

// VALIDATES: zero or negative base returns zero reduction. Edge case for
// sessions whose TransmitInterval has not yet been set.
// PREVENTS: divide-by-zero or time.Duration sign errors.
func TestApplyJitterZeroBase(t *testing.T) {
	loop := NewLoop(nil, nil)
	if got := loop.applyJitter(0, 3); got != 0 {
		t.Fatalf("applyJitter(0, 3) = %v, want 0", got)
	}
	if got := loop.applyJitter(-1*time.Second, 3); got != 0 {
		t.Fatalf("applyJitter(-1s, 3) = %v, want 0", got)
	}
}

// VALIDATES: DetectMult >= 2 draws are approximately uniformly distributed
// across the [0, 25%) band. Asserts the sample mean is within 2% of the
// theoretical 12.5% center.
// PREVENTS: regression where the RNG is biased (e.g., constant returning
// zero) or the scaling formula clips.
func TestApplyJitterUniformityDetectMultDefault(t *testing.T) {
	loop := NewLoop(nil, nil)
	const base = 1_000_000 * time.Microsecond
	const draws = 20_000
	var sum float64
	for range draws {
		got := loop.applyJitter(base, 3)
		sum += float64(got) / float64(base)
	}
	mean := sum / draws
	const theoretical = (0 + JitterMaxFraction) / 2
	if math.Abs(mean-theoretical) > 0.02 {
		t.Fatalf("mean reduction fraction %v outside tolerance (want ~%v ± 0.02)", mean, theoretical)
	}
}

// VALIDATES: DetectMult == 1 draws are approximately uniformly distributed
// across the [10%, 25%) band. Sample mean should be within 2% of 17.5%.
// PREVENTS: regression where the offset is dropped and the range collapses
// back to [0, 25%).
func TestApplyJitterUniformityDetectMultOne(t *testing.T) {
	loop := NewLoop(nil, nil)
	const base = 1_000_000 * time.Microsecond
	const draws = 20_000
	var sum float64
	for range draws {
		got := loop.applyJitter(base, 1)
		sum += float64(got) / float64(base)
	}
	mean := sum / draws
	const theoretical = (JitterMinFractionDetectMultOne + JitterMaxFraction) / 2
	if math.Abs(mean-theoretical) > 0.02 {
		t.Fatalf("mean reduction fraction %v outside tolerance (want ~%v ± 0.02)", mean, theoretical)
	}
}
