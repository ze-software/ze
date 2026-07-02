// VALIDATES: AC-5 core/stats math primitives -- Mean, population StdDev, and
// linear-interpolation Quantile behave correctly at the empty, single-sample,
// zero-variance, and clamped-q edges.
// PREVENTS: divide-by-zero on empty input, a sample-vs-population stddev mixup,
// and Quantile mutating the caller's slice.

package stats

import (
	"math"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestMeanStdDev validates the arithmetic mean and population standard deviation,
// including the empty and single-sample edge cases.
func TestMeanStdDev(t *testing.T) {
	if m := Mean(nil); m != 0 {
		t.Errorf("Mean(nil) = %v, want 0", m)
	}
	if m := Mean([]float64{2, 4, 6}); !almost(m, 4) {
		t.Errorf("Mean = %v, want 4", m)
	}
	if s := StdDev([]float64{5}); s != 0 {
		t.Errorf("StdDev(single) = %v, want 0", s)
	}
	// population stddev of {2,4,6}: mean 4, var = (4+0+4)/3 = 2.6667, sd ~= 1.63299
	if s := StdDev([]float64{2, 4, 6}); !almost(s, math.Sqrt(8.0/3.0)) {
		t.Errorf("StdDev = %v, want %v", s, math.Sqrt(8.0/3.0))
	}
	// zero variance
	if s := StdDev([]float64{7, 7, 7, 7}); s != 0 {
		t.Errorf("StdDev(constant) = %v, want 0", s)
	}
}

// TestQuantile validates linear-interpolation quantiles and boundary clamping.
func TestQuantile(t *testing.T) {
	if q := Quantile(nil, 0.5); q != 0 {
		t.Errorf("Quantile(nil) = %v, want 0", q)
	}
	xs := []float64{1, 2, 3, 4, 5}
	if q := Quantile(xs, 0); !almost(q, 1) {
		t.Errorf("Quantile q=0 = %v, want 1 (min)", q)
	}
	if q := Quantile(xs, 1); !almost(q, 5) {
		t.Errorf("Quantile q=1 = %v, want 5 (max)", q)
	}
	if q := Quantile(xs, 0.5); !almost(q, 3) {
		t.Errorf("Quantile q=0.5 = %v, want 3 (median)", q)
	}
	// clamp out-of-range q
	if q := Quantile(xs, 1.5); !almost(q, 5) {
		t.Errorf("Quantile q=1.5 clamped = %v, want 5", q)
	}
	if q := Quantile(xs, -0.5); !almost(q, 1) {
		t.Errorf("Quantile q=-0.5 clamped = %v, want 1", q)
	}
	// input not mutated (was unsorted)
	un := []float64{5, 1, 3}
	_ = Quantile(un, 0.5)
	if un[0] != 5 || un[1] != 1 || un[2] != 3 {
		t.Errorf("Quantile mutated input: %v", un)
	}
}
