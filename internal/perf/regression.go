// Design: (none -- new tool, predates documentation)
package perf

import (
	"fmt"
	"math"
)

// Thresholds defines regression detection thresholds as percentages.
type Thresholds struct {
	ConvergencePct int
	ThroughputPct  int
	P99Pct         int
}

// DefaultThresholds returns the default regression detection thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{
		ConvergencePct: 20,
		ThroughputPct:  20,
		P99Pct:         30,
	}
}

// Regression describes a single detected performance regression.
type Regression struct {
	Metric        string
	CurrentValue  int
	PreviousValue int
	ThresholdPct  int
	DeltaPct      float64
	Message       string
}

// CheckRegression compares current against previous and returns any regressions found.
// An empty slice means no regression was detected.
//
//nolint:gocritic // hugeParam: Result is passed by value for API simplicity; not a hot path.
func CheckRegression(current, previous Result, thresholds Thresholds) []Regression {
	var regs []Regression

	// Convergence: regressed if current > previous * (1 + threshold/100) AND delta > combined stddev.
	if r := checkHigherIsBad("convergence-ms",
		current.ConvergenceMs, previous.ConvergenceMs,
		current.ConvergenceStddevMs, previous.ConvergenceStddevMs,
		thresholds.ConvergencePct,
	); r != nil {
		regs = append(regs, *r)
	}

	// Throughput: regressed if current < previous * (1 - threshold/100) AND delta > combined stddev.
	if r := checkLowerIsBad("throughput-avg",
		current.ThroughputAvg, previous.ThroughputAvg,
		current.ThroughputAvgStddev, previous.ThroughputAvgStddev,
		thresholds.ThroughputPct,
	); r != nil {
		regs = append(regs, *r)
	}

	// P99 latency: regressed if current > previous * (1 + threshold/100) AND delta > combined stddev.
	if r := checkHigherIsBad("latency-p99-ms",
		current.LatencyP99Ms, previous.LatencyP99Ms,
		current.LatencyP99StddevMs, previous.LatencyP99StddevMs,
		thresholds.P99Pct,
	); r != nil {
		regs = append(regs, *r)
	}

	// Routes lost: regressed if current > 0.
	if current.RoutesLost > 0 {
		regs = append(regs, Regression{
			Metric:        "routes-lost",
			CurrentValue:  current.RoutesLost,
			PreviousValue: previous.RoutesLost,
			ThresholdPct:  0,
			DeltaPct:      0,
			Message:       fmt.Sprintf("routes-lost: %d routes lost (previous: %d)", current.RoutesLost, previous.RoutesLost),
		})
	}

	return regs
}

// CheckHistory checks the last N runs for regressions. If last is 0, compares
// the last 2 entries. Returns regressions found between the most recent and
// the entry before it (within the window).
func CheckHistory(results []Result, thresholds Thresholds, last int) []Regression {
	if len(results) < 2 {
		return nil
	}

	window := results
	if last > 0 && last < len(results) {
		window = results[len(results)-last:]
	}

	if len(window) < 2 {
		return nil
	}

	current := window[len(window)-1]
	previous := window[len(window)-2]

	return CheckRegression(current, previous, thresholds)
}

// checkHigherIsBad detects regression where a higher value is worse (convergence, latency).
func checkHigherIsBad(metric string, current, previous, currentStddev, previousStddev, thresholdPct int) *Regression {
	if previous == 0 {
		return nil
	}

	threshold := float64(previous) * float64(thresholdPct) / 100.0
	delta := float64(current - previous)

	if delta <= threshold {
		return nil
	}

	// Stddev-aware: delta must exceed combined stddev.
	combinedStddev := math.Sqrt(float64(currentStddev)*float64(currentStddev) + float64(previousStddev)*float64(previousStddev))

	if combinedStddev > 0 && math.Abs(delta) <= combinedStddev {
		return nil
	}

	deltaPct := delta / float64(previous) * 100.0

	return &Regression{
		Metric:        metric,
		CurrentValue:  current,
		PreviousValue: previous,
		ThresholdPct:  thresholdPct,
		DeltaPct:      deltaPct,
		Message:       fmt.Sprintf("%s: %d -> %d (%.1f%% increase, threshold %d%%)", metric, previous, current, deltaPct, thresholdPct),
	}
}

// checkLowerIsBad detects regression where a lower value is worse (throughput).
func checkLowerIsBad(metric string, current, previous, currentStddev, previousStddev, thresholdPct int) *Regression {
	if previous == 0 {
		return nil
	}

	threshold := float64(previous) * float64(thresholdPct) / 100.0
	delta := float64(previous - current) // positive means decrease (bad)

	if delta <= threshold {
		return nil
	}

	// Stddev-aware: delta must exceed combined stddev.
	combinedStddev := math.Sqrt(float64(currentStddev)*float64(currentStddev) + float64(previousStddev)*float64(previousStddev))

	if combinedStddev > 0 && math.Abs(delta) <= combinedStddev {
		return nil
	}

	deltaPct := delta / float64(previous) * 100.0

	return &Regression{
		Metric:        metric,
		CurrentValue:  current,
		PreviousValue: previous,
		ThresholdPct:  thresholdPct,
		DeltaPct:      -deltaPct, // negative = decrease
		Message:       fmt.Sprintf("%s: %d -> %d (%.1f%% decrease, threshold %d%%)", metric, previous, current, deltaPct, thresholdPct),
	}
}
