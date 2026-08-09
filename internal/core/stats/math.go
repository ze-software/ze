// Design: docs/architecture/traffic/traffic-analysis-layers.md -- neutral statistical primitives (mean, stddev, quantile)

package stats

import (
	"math"
	"sort"
)

// Mean returns the arithmetic mean of xs, or 0 for an empty slice.
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// StdDev returns the population standard deviation of xs, or 0 for fewer than two
// samples.
func StdDev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := Mean(xs)
	var ss float64
	for _, x := range xs {
		d := x - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(xs)))
}

// Quantile returns the q-quantile (q in [0,1]) of xs using linear interpolation
// between closest ranks. It returns 0 for empty input, clamps q to [0,1], and
// does not modify xs.
func Quantile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	s := make([]float64, len(xs))
	copy(s, xs)
	sort.Float64s(s)
	if len(s) == 1 {
		return s[0]
	}
	pos := q * float64(len(s)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return s[lo]
	}
	frac := pos - float64(lo)
	return s[lo]*(1-frac) + s[hi]*frac
}
