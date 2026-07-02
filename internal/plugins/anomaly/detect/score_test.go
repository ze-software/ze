// VALIDATES: AC-2/AC-3/AC-4 the pinned Scoring & Correlation Rule -- per-feature
// z-score (positive deviation only, stddev floored, clamped to Zmax), cohort
// rarity with the tiny-cohort fallback, and the capped/discounted combine
// (Zstrong + weight*sum(others), cap ScoreMax) that avoids double-counting.
// PREVENTS: divide-by-tiny-variance, a below-baseline value scoring positive, a
// meaningless rarity from a 1-member cohort, and naive-sum score inflation (R-5).

package detect

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSelfDeviationScore(t *testing.T) {
	// (10-4)/2 = 3 sigma
	if z := zScore(10, 4, 2, 0.1); !approx(z, 3) {
		t.Errorf("zScore = %v, want 3", z)
	}
	// below baseline -> 0 (we only flag positive deviations)
	if z := zScore(1, 4, 2, 0.1); z != 0 {
		t.Errorf("below-baseline z = %v, want 0", z)
	}
	// zero variance clamped by the floor, no div-by-zero
	if z := zScore(5, 4, 0, 0.5); !approx(z, 2) {
		t.Errorf("floored z = %v, want (5-4)/0.5 = 2", z)
	}
	// clamped to Zmax
	if z := zScore(1000, 0, 1, 0.1); z != zMax {
		t.Errorf("z = %v, want clamp to zMax=%v", z, zMax)
	}
}

func mkCohort(vals ...float64) cohortStats {
	var c cohortStats
	for _, v := range vals {
		c.add(v)
	}
	return c
}

func TestCohortRarity(t *testing.T) {
	// The scored member (100) is left out of its own baseline: vs the remaining
	// {2,2,2,2} it is a strong outlier and is NOT dampened by self-inclusion.
	c := mkCohort(2, 2, 2, 2, 100)
	r := c.rarity(100, 4, 0.5)
	if r < 5 {
		t.Errorf("leave-one-out outlier rarity = %v, want high (self excluded)", r)
	}
	// Had self been included ({2,2,2,2,100}: mean 21.6, stddev ~39), the same value
	// would score < 3 -- prove the leave-one-out is materially different.
	inclMean, inclSD := 21.6, 39.0
	if selfIncluded := (100 - inclMean) / inclSD; r <= selfIncluded {
		t.Errorf("leave-one-out (%v) should exceed self-included (~%.2f)", r, selfIncluded)
	}
	// A member consistent with its cohort scores ~0.
	if got := c.rarity(2, 4, 0.5); got != 0 {
		t.Errorf("in-distribution rarity = %v, want 0", got)
	}
	// Tiny cohort (fewer than minSize OTHER members) yields no signal.
	if got := mkCohort(2, 10).rarity(10, 4, 0.5); got != 0 {
		t.Errorf("tiny-cohort rarity = %v, want 0 (self-deviation only)", got)
	}
}

func TestCorrelateCappedCombine(t *testing.T) {
	// Zstrong + weight*sum(others): 8 + 0.5*(6+4) = 13
	if s := combineScore([]float64{4, 8, 6}, 0.5); !approx(s, 13) {
		t.Errorf("combine = %v, want 13 (dominant 8 + 0.5*(6+4))", s)
	}
	// naive sum would be 40; capped/discounted stays at ScoreMax
	if s := combineScore([]float64{10, 10, 10, 10}, 1.0); s != scoreMax {
		t.Errorf("combine = %v, want cap %v (not naive-sum 40)", s, scoreMax)
	}
	// a single fired feature is just its own z (gate is applied by the caller)
	if s := combineScore([]float64{5}, 0.5); !approx(s, 5) {
		t.Errorf("single combine = %v, want 5", s)
	}
	if s := combineScore(nil, 0.5); s != 0 {
		t.Errorf("empty combine = %v, want 0", s)
	}
}
