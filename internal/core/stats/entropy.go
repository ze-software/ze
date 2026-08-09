// Design: docs/architecture/traffic/traffic-analysis-layers.md -- Shannon entropy of a distribution

package stats

import "math"

// Entropy returns the Shannon entropy in bits of the distribution implied by
// counts (each element a nonnegative weight). It is 0 for an empty distribution
// or one with a single nonzero bucket, and log2(n) for n equal buckets.
// Non-positive weights are ignored.
func Entropy(counts []float64) float64 {
	var total float64
	for _, c := range counts {
		if c > 0 {
			total += c
		}
	}
	if total <= 0 {
		return 0
	}
	var h float64
	for _, c := range counts {
		if c <= 0 {
			continue
		}
		p := c / total
		h -= p * math.Log2(p)
	}
	return h
}
