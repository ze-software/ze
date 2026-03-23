package perf

import (
	"testing"
)

// VALIDATES: "Threshold-based regression flagging".
// PREVENTS: Regressions going undetected when convergence doubles.
func TestRegressionDetection(t *testing.T) {
	previous := Result{
		DUTName:             "ze",
		DUTVersion:          "0.1.0",
		Family:              "ipv4/unicast",
		ConvergenceMs:       1000,
		ConvergenceStddevMs: 50,
		ThroughputAvg:       50000,
		ThroughputAvgStddev: 1000,
		LatencyP99Ms:        10,
		LatencyP99StddevMs:  2,
	}

	current := Result{
		DUTName:             "ze",
		DUTVersion:          "0.2.0",
		Family:              "ipv4/unicast",
		ConvergenceMs:       2000, // doubled
		ConvergenceStddevMs: 60,
		ThroughputAvg:       50000,
		ThroughputAvgStddev: 1000,
		LatencyP99Ms:        10,
		LatencyP99StddevMs:  2,
	}

	regs := CheckRegression(current, previous, DefaultThresholds())

	if len(regs) == 0 {
		t.Fatal("expected regression to be detected when convergence doubled")
	}

	found := false
	for _, r := range regs {
		if r.Metric == "convergence-ms" {
			found = true

			if r.CurrentValue != 2000 {
				t.Errorf("CurrentValue = %d, want 2000", r.CurrentValue)
			}

			if r.PreviousValue != 1000 {
				t.Errorf("PreviousValue = %d, want 1000", r.PreviousValue)
			}

			if r.Message == "" {
				t.Error("regression Message is empty")
			}
		}
	}

	if !found {
		t.Error("no regression for convergence-ms metric")
	}
}

// VALIDATES: "Delta within combined stddev does not trigger".
// PREVENTS: False positives when variation is within noise.
func TestRegressionStddevAware(t *testing.T) {
	previous := Result{
		DUTName:             "ze",
		DUTVersion:          "0.1.0",
		Family:              "ipv4/unicast",
		ConvergenceMs:       1000,
		ConvergenceStddevMs: 300, // large stddev
		ThroughputAvg:       50000,
		ThroughputAvgStddev: 5000,
		LatencyP99Ms:        10,
		LatencyP99StddevMs:  5,
	}

	// 20% worse convergence, but delta (200) < sqrt(300^2 + 350^2) = sqrt(90000+122500) ~ 461
	current := Result{
		DUTName:             "ze",
		DUTVersion:          "0.2.0",
		Family:              "ipv4/unicast",
		ConvergenceMs:       1200, // 20% worse
		ConvergenceStddevMs: 350,  // large stddev
		ThroughputAvg:       50000,
		ThroughputAvgStddev: 5000,
		LatencyP99Ms:        10,
		LatencyP99StddevMs:  5,
	}

	regs := CheckRegression(current, previous, DefaultThresholds())

	for _, r := range regs {
		if r.Metric == "convergence-ms" {
			t.Errorf("unexpected convergence regression: delta within combined stddev; got %+v", r)
		}
	}
}

// VALIDATES: "Small variations do not trigger".
// PREVENTS: False positives on minor fluctuations.
func TestRegressionNoFalsePositive(t *testing.T) {
	previous := Result{
		DUTName:             "ze",
		DUTVersion:          "0.1.0",
		Family:              "ipv4/unicast",
		ConvergenceMs:       1000,
		ConvergenceStddevMs: 50,
		ThroughputAvg:       50000,
		ThroughputAvgStddev: 1000,
		LatencyP99Ms:        10,
		LatencyP99StddevMs:  2,
	}

	// 5% worse -- well under 20% threshold
	current := Result{
		DUTName:             "ze",
		DUTVersion:          "0.2.0",
		Family:              "ipv4/unicast",
		ConvergenceMs:       1050,
		ConvergenceStddevMs: 55,
		ThroughputAvg:       47500, // 5% worse
		ThroughputAvgStddev: 1100,
		LatencyP99Ms:        10,
		LatencyP99StddevMs:  2,
	}

	regs := CheckRegression(current, previous, DefaultThresholds())

	if len(regs) != 0 {
		t.Errorf("expected no regressions for 5%% variation, got %d: %+v", len(regs), regs)
	}
}

// VALIDATES: "Routes lost triggers regression".
// PREVENTS: Data loss going undetected.
func TestRegressionRoutesLost(t *testing.T) {
	previous := Result{
		DUTName:    "ze",
		Family:     "ipv4/unicast",
		RoutesLost: 0,
	}

	current := Result{
		DUTName:    "ze",
		Family:     "ipv4/unicast",
		RoutesLost: 5,
	}

	regs := CheckRegression(current, previous, DefaultThresholds())

	found := false
	for _, r := range regs {
		if r.Metric == "routes-lost" {
			found = true

			if r.CurrentValue != 5 {
				t.Errorf("CurrentValue = %d, want 5", r.CurrentValue)
			}
		}
	}

	if !found {
		t.Error("no regression for routes-lost when current > 0")
	}
}

// VALIDATES: "CheckHistory compares last N runs".
// PREVENTS: History comparison failing silently.
func TestCheckHistory(t *testing.T) {
	results := []Result{
		{
			DUTName:             "ze",
			DUTVersion:          "0.1.0",
			Family:              "ipv4/unicast",
			Timestamp:           "2026-03-18T10:00:00Z",
			ConvergenceMs:       1000,
			ConvergenceStddevMs: 50,
			ThroughputAvg:       50000,
			ThroughputAvgStddev: 1000,
			LatencyP99Ms:        10,
			LatencyP99StddevMs:  2,
		},
		{
			DUTName:             "ze",
			DUTVersion:          "0.2.0",
			Family:              "ipv4/unicast",
			Timestamp:           "2026-03-19T10:00:00Z",
			ConvergenceMs:       1050,
			ConvergenceStddevMs: 55,
			ThroughputAvg:       49000,
			ThroughputAvgStddev: 1100,
			LatencyP99Ms:        11,
			LatencyP99StddevMs:  2,
		},
		{
			DUTName:             "ze",
			DUTVersion:          "0.3.0",
			Family:              "ipv4/unicast",
			Timestamp:           "2026-03-20T10:00:00Z",
			ConvergenceMs:       3000, // big regression
			ConvergenceStddevMs: 60,
			ThroughputAvg:       48000,
			ThroughputAvgStddev: 1200,
			LatencyP99Ms:        25,
			LatencyP99StddevMs:  3,
		},
	}

	// last=0 means compare last 2
	regs := CheckHistory(results, DefaultThresholds(), 0)

	if len(regs) == 0 {
		t.Fatal("expected regression when last run tripled convergence")
	}
}

// VALIDATES: "CheckHistory with insufficient results returns empty".
// PREVENTS: Panic on short history.
func TestCheckHistoryShort(t *testing.T) {
	results := []Result{
		{DUTName: "ze", Family: "ipv4/unicast", ConvergenceMs: 1000},
	}

	regs := CheckHistory(results, DefaultThresholds(), 0)

	if len(regs) != 0 {
		t.Errorf("expected no regressions with single result, got %d", len(regs))
	}
}
