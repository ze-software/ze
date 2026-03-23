// Design: (none -- new tool, predates documentation)
// Related: benchmark.go -- benchmark orchestration using metrics computation

package perf

import (
	"math"
	"sort"
	"time"
)

// IterationResult holds the measurements from a single benchmark iteration.
type IterationResult struct {
	ConvergenceMs     int
	FirstRouteMs      int
	ThroughputAvg     int
	ThroughputPeak    int
	LatencyP50Ms      int
	LatencyP90Ms      int
	LatencyP99Ms      int
	LatencyMaxMs      int
	RoutesReceived    int
	SessionSenderMs   int
	SessionReceiverMs int
}

// AggregatedResult holds median and stddev values computed across iterations.
type AggregatedResult struct {
	ConvergenceMs       int
	ConvergenceStddevMs int
	FirstRouteMs        int
	ThroughputAvg       int
	ThroughputAvgStddev int
	ThroughputPeak      int
	LatencyP50Ms        int
	LatencyP90Ms        int
	LatencyP99Ms        int
	LatencyP99StddevMs  int
	LatencyMaxMs        int
	RoutesReceived      int
	SessionSenderMs     int
	SessionReceiverMs   int
}

// Median returns the median of a slice of ints. Returns 0 for empty input.
// For even-length slices, returns the truncated average of the two middle values.
// The input slice is not modified.
func Median(vals []int) int {
	if len(vals) == 0 {
		return 0
	}

	sorted := make([]int, len(vals))
	copy(sorted, vals)
	sort.Ints(sorted)

	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}

	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// Stddev returns the population standard deviation of a slice of ints, truncated to int.
// Returns 0 for slices with fewer than 2 elements.
func Stddev(vals []int) int {
	if len(vals) <= 1 {
		return 0
	}

	sum := 0
	for _, v := range vals {
		sum += v
	}

	mean := float64(sum) / float64(len(vals))

	variance := 0.0
	for _, v := range vals {
		diff := float64(v) - mean
		variance += diff * diff
	}

	variance /= float64(len(vals))

	return int(math.Sqrt(variance))
}

// Percentile returns the nearest-rank percentile of a sorted slice of ints.
// p must be in [0,1]. Returns 0 for empty input. Input must be sorted.
func Percentile(sorted []int, p float64) int {
	if len(sorted) == 0 {
		return 0
	}

	if p <= 0 {
		return sorted[0]
	}

	if p >= 1 {
		return sorted[len(sorted)-1]
	}

	rank := max(int(math.Ceil(p*float64(len(sorted)))), 1)

	return sorted[rank-1]
}

// CalculateLatencies converts durations to millisecond percentiles.
// Returns p50, p90, p99, max. Returns all zeros for empty input.
// For a single duration, returns that value for all percentiles.
func CalculateLatencies(durations []time.Duration) (p50, p90, p99, max int) {
	if len(durations) == 0 {
		return 0, 0, 0, 0
	}

	ms := make([]int, len(durations))
	for i, d := range durations {
		ms[i] = int(d / time.Millisecond)
	}

	sort.Ints(ms)

	p50 = Percentile(ms, 0.50)
	p90 = Percentile(ms, 0.90)
	p99 = Percentile(ms, 0.99)
	max = ms[len(ms)-1]

	return p50, p90, p99, max
}

// CalculateThroughput computes average and peak routes/sec.
// convergence is the total time from first send to last receive.
// timestamps are per-route arrival times (used for peak calculation).
// avg = total routes / convergence seconds. peak = max count in any 1-second window.
// When all routes arrive in a single burst (zero timestamp spread), peak equals route count.
func CalculateThroughput(timestamps []time.Time, convergence time.Duration) (avg, peak int) {
	if len(timestamps) == 0 {
		return 0, 0
	}

	// Average: routes / convergence time.
	if convergence > 0 {
		avg = int(float64(len(timestamps)) / convergence.Seconds())
	}

	// Peak: sliding 1-second window over sorted arrival timestamps.
	sorted := make([]time.Time, len(timestamps))
	copy(sorted, timestamps)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Before(sorted[j])
	})

	left := 0

	for right := range sorted {
		for sorted[right].Sub(sorted[left]) >= time.Second {
			left++
		}

		count := right - left + 1
		if count > peak {
			peak = count
		}
	}

	return avg, peak
}

// RemoveOutliers removes iterations where ConvergenceMs is beyond 2 stddev
// from the median. Never removes below 3 remaining. Returns a new slice.
func RemoveOutliers(results []IterationResult) []IterationResult {
	if len(results) <= 3 {
		out := make([]IterationResult, len(results))
		copy(out, results)
		return out
	}

	vals := make([]int, len(results))
	for i, r := range results {
		vals[i] = r.ConvergenceMs
	}

	med := Median(vals)
	sd := Stddev(vals)

	threshold := 2 * sd

	// First pass: determine which are outliers.
	type indexed struct {
		idx     int
		outlier bool
		dist    int
	}

	entries := make([]indexed, len(results))
	outlierCount := 0

	for i, r := range results {
		dist := r.ConvergenceMs - med
		if dist < 0 {
			dist = -dist
		}

		isOutlier := dist > threshold
		entries[i] = indexed{idx: i, outlier: isOutlier, dist: dist}

		if isOutlier {
			outlierCount++
		}
	}

	// Respect minimum 3 remaining: if removing all outliers leaves < 3, keep the closest ones.
	maxRemovable := max(len(results)-3, 0)

	if outlierCount > maxRemovable {
		// Sort outliers by distance descending, only remove the worst ones.
		type outlierEntry struct {
			idx  int
			dist int
		}

		var outliers []outlierEntry

		for _, e := range entries {
			if e.outlier {
				outliers = append(outliers, outlierEntry{idx: e.idx, dist: e.dist})
			}
		}

		sort.Slice(outliers, func(i, j int) bool {
			return outliers[i].dist > outliers[j].dist
		})

		// Reset all outlier flags, then mark only the worst maxRemovable.
		for i := range entries {
			entries[i].outlier = false
		}

		for i := 0; i < maxRemovable && i < len(outliers); i++ {
			entries[outliers[i].idx].outlier = true
		}
	}

	var kept []IterationResult

	for i, e := range entries {
		if !e.outlier {
			kept = append(kept, results[i])
		}
	}

	return kept
}

// Aggregate computes median and stddev for each metric across iterations.
func Aggregate(results []IterationResult) AggregatedResult {
	n := len(results)
	if n == 0 {
		return AggregatedResult{}
	}

	convergence := make([]int, n)
	firstRoute := make([]int, n)
	tpAvg := make([]int, n)
	tpPeak := make([]int, n)
	latP50 := make([]int, n)
	latP90 := make([]int, n)
	latP99 := make([]int, n)
	latMax := make([]int, n)
	routes := make([]int, n)
	sessSender := make([]int, n)
	sessReceiver := make([]int, n)

	for i, r := range results {
		convergence[i] = r.ConvergenceMs
		firstRoute[i] = r.FirstRouteMs
		tpAvg[i] = r.ThroughputAvg
		tpPeak[i] = r.ThroughputPeak
		latP50[i] = r.LatencyP50Ms
		latP90[i] = r.LatencyP90Ms
		latP99[i] = r.LatencyP99Ms
		latMax[i] = r.LatencyMaxMs
		routes[i] = r.RoutesReceived
		sessSender[i] = r.SessionSenderMs
		sessReceiver[i] = r.SessionReceiverMs
	}

	return AggregatedResult{
		ConvergenceMs:       Median(convergence),
		ConvergenceStddevMs: Stddev(convergence),
		FirstRouteMs:        Median(firstRoute),
		ThroughputAvg:       Median(tpAvg),
		ThroughputAvgStddev: Stddev(tpAvg),
		ThroughputPeak:      Median(tpPeak),
		LatencyP50Ms:        Median(latP50),
		LatencyP90Ms:        Median(latP90),
		LatencyP99Ms:        Median(latP99),
		LatencyP99StddevMs:  Stddev(latP99),
		LatencyMaxMs:        Median(latMax),
		RoutesReceived:      Median(routes),
		SessionSenderMs:     Median(sessSender),
		SessionReceiverMs:   Median(sessReceiver),
	}
}
