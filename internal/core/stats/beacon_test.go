// VALIDATES: AC-8 coarse beaconing -- a regular (clock-like) arrival series scores
// high, a random series scores low, and series below the count/period floor return
// no signal (0), per the 1s-tick Nyquist limit.
// PREVENTS: claiming a beacon from sub-2s intervals the feed cannot resolve, and
// divide-by-zero on a zero mean.

package stats

import "testing"

func TestIntervalRegularity(t *testing.T) {
	// too few intervals -> no signal
	if r := IntervalRegularity([]float64{5, 5}); r != 0 {
		t.Errorf("regularity(2 intervals) = %v, want 0 (need >= 3)", r)
	}
	// perfectly regular 5s beacon -> ~1.0
	reg := IntervalRegularity([]float64{5, 5, 5, 5, 5})
	if reg < 0.99 {
		t.Errorf("regularity(perfect 5s) = %v, want ~1.0", reg)
	}
	// irregular -> well below regular
	irr := IntervalRegularity([]float64{5, 40, 3, 60, 2})
	if irr >= reg {
		t.Errorf("regularity(irregular)=%v should be < regular=%v", irr, reg)
	}
	if irr < 0 || irr > 1 {
		t.Errorf("regularity out of [0,1]: %v", irr)
	}
	// below the 2s floor -> no signal even if perfectly regular
	if r := IntervalRegularity([]float64{1, 1, 1, 1}); r != 0 {
		t.Errorf("regularity(1s, below floor) = %v, want 0", r)
	}
}
