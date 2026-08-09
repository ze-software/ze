// Design: docs/architecture/anomaly/anomaly-1-detect.md -- Scoring & Correlation Rule (pinned)
//
// Pure scoring primitives for the pinned rule. Stateful per-entity baselines live
// in detector.go; these functions take the already-derived mean/stddev and the
// per-feature z-scores and combine them deterministically.

package detect

import (
	"math"
	"sort"
)

const (
	// zMax bounds any single feature's per-tick deviation so one wild feature
	// cannot dominate an incident.
	zMax = 10.0
	// scoreMax caps the combined incident score (3 * zMax) so a benign
	// multi-feature move cannot saturate into a critical incident.
	scoreMax = 30.0
)

// zScore returns the positive deviation of value above a baseline mean in units
// of stddev, clamped to [0, zMax]. stddev is floored to avoid divide-by-tiny-
// variance; below-baseline values score 0 (only excess is anomalous).
func zScore(value, mean, stddev, floor float64) float64 {
	sd := stddev
	if sd < floor {
		sd = floor
	}
	z := (value - mean) / sd
	if z <= 0 {
		return 0
	}
	if z > zMax {
		return zMax
	}
	return z
}

// cohortStats accumulates a cohort's distribution for one feature as running
// sum / sum-of-squares / count, so leave-one-out rarity is O(1) per entity and a
// single extreme (or deliberately excluded) member does not define the baseline.
type cohortStats struct {
	sum, sumSq float64
	count      int
}

func (c *cohortStats) add(v float64) {
	c.sum += v
	c.sumSq += v * v
	c.count++
}

// rarity scores how far value sits above the cohort distribution with value's own
// contribution removed (leave-one-out), so an outlier is never compared against a
// baseline it dominates. It returns 0 when fewer than minSize OTHER members remain
// (the tiny-cohort fallback). A member intentionally left out of the cohort (an
// infinite ratio) still scores via self-deviation, so its garbage leave-one-out is
// harmless.
func (c cohortStats) rarity(value float64, minSize int, floor float64) float64 {
	n := c.count - 1
	if n < minSize {
		return 0
	}
	mean := (c.sum - value) / float64(n)
	variance := (c.sumSq-value*value)/float64(n) - mean*mean
	if variance < 0 {
		variance = 0
	}
	return zScore(value, mean, math.Sqrt(variance), floor)
}

// combineScore fuses the fired features' z-scores into one incident score:
// Zstrong (the largest, full weight) plus a discounted sum of the rest (weight in
// [0,1]), capped at scoreMax. The discount prevents correlated co-moving features
// from double-counting (R-5); it is NOT a naive sum. The caller applies the
// min-features gate before calling this.
func combineScore(fired []float64, weight float64) float64 {
	if len(fired) == 0 {
		return 0
	}
	sorted := append([]float64(nil), fired...)
	sort.Sort(sort.Reverse(sort.Float64Slice(sorted)))
	score := sorted[0]
	for _, z := range sorted[1:] {
		score += weight * z
	}
	if score > scoreMax {
		return scoreMax
	}
	return score
}
