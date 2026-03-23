package perf

import (
	"testing"
	"time"
)

// VALIDATES: Median computes the correct middle value for various input sizes.
// PREVENTS: Off-by-one errors in median calculation for odd/even slices.
func TestMedianCalculation(t *testing.T) {
	tests := []struct {
		name string
		vals []int
		want int
	}{
		{name: "empty", vals: nil, want: 0},
		{name: "single", vals: []int{42}, want: 42},
		{name: "odd count", vals: []int{3, 1, 2}, want: 2},
		{name: "even count truncated", vals: []int{1, 2, 3, 4}, want: 2}, // (2+3)/2 = 2 (truncated)
		{name: "already sorted", vals: []int{10, 20, 30, 40, 50}, want: 30},
		{name: "reverse sorted", vals: []int{50, 40, 30, 20, 10}, want: 30},
		{name: "duplicates", vals: []int{5, 5, 5, 5}, want: 5},
		{name: "two elements", vals: []int{10, 20}, want: 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Median(tt.vals)
			if got != tt.want {
				t.Errorf("Median(%v) = %d, want %d", tt.vals, got, tt.want)
			}
		})
	}

	t.Run("does not modify input", func(t *testing.T) {
		input := []int{3, 1, 2}
		Median(input)
		if input[0] != 3 || input[1] != 1 || input[2] != 2 {
			t.Errorf("Median modified input slice: %v", input)
		}
	})
}

// VALIDATES: Stddev computes population standard deviation correctly.
// PREVENTS: Wrong stddev formula (sample vs population) or truncation errors.
func TestStddevCalculation(t *testing.T) {
	tests := []struct {
		name string
		vals []int
		want int
	}{
		{name: "empty", vals: nil, want: 0},
		{name: "single", vals: []int{42}, want: 0},
		{name: "identical values", vals: []int{5, 5, 5, 5}, want: 0},
		{name: "known distribution", vals: []int{2, 4, 4, 4, 5, 5, 7, 9}, want: 2}, // stddev = 2.0
		{name: "simple pair", vals: []int{0, 10}, want: 5},                         // stddev = 5.0
		{name: "three values", vals: []int{1, 2, 3}, want: 0},                      // stddev ~= 0.816, truncated to 0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Stddev(tt.vals)
			if got != tt.want {
				t.Errorf("Stddev(%v) = %d, want %d", tt.vals, got, tt.want)
			}
		})
	}
}

// VALIDATES: Percentile returns correct nearest-rank percentile from sorted input.
// PREVENTS: Off-by-one in percentile index calculation.
func TestPercentileCalculation(t *testing.T) {
	tests := []struct {
		name   string
		sorted []int
		p      float64
		want   int
	}{
		{name: "empty", sorted: nil, p: 0.5, want: 0},
		{name: "single p50", sorted: []int{42}, p: 0.5, want: 42},
		{name: "single p99", sorted: []int{42}, p: 0.99, want: 42},
		{name: "p0", sorted: []int{1, 2, 3, 4, 5}, p: 0.0, want: 1},
		{name: "p100", sorted: []int{1, 2, 3, 4, 5}, p: 1.0, want: 5},
		{name: "p50 odd", sorted: []int{1, 2, 3, 4, 5}, p: 0.5, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Percentile(tt.sorted, tt.p)
			if got != tt.want {
				t.Errorf("Percentile(%v, %.2f) = %d, want %d", tt.sorted, tt.p, got, tt.want)
			}
		})
	}
}

// VALIDATES: CalculateLatencies returns correct percentiles from duration slices.
// PREVENTS: Wrong conversion from time.Duration to milliseconds.
func TestLatencyCalculation(t *testing.T) {
	t.Run("100 durations 1ms to 100ms", func(t *testing.T) {
		durations := make([]time.Duration, 100)
		for i := range durations {
			durations[i] = time.Duration(i+1) * time.Millisecond
		}

		p50, p90, p99, max := CalculateLatencies(durations)

		if p50 != 50 {
			t.Errorf("p50 = %d, want 50", p50)
		}
		if p90 != 90 {
			t.Errorf("p90 = %d, want 90", p90)
		}
		if p99 != 99 {
			t.Errorf("p99 = %d, want 99", p99)
		}
		if max != 100 {
			t.Errorf("max = %d, want 100", max)
		}
	})
}

// VALIDATES: CalculateLatencies handles edge cases without panic.
// PREVENTS: Panic on empty or single-element duration slices.
func TestLatencyCalculationEdgeCases(t *testing.T) {
	t.Run("zero durations", func(t *testing.T) {
		p50, p90, p99, max := CalculateLatencies(nil)
		if p50 != 0 || p90 != 0 || p99 != 0 || max != 0 {
			t.Errorf("expected all zeros, got p50=%d p90=%d p99=%d max=%d", p50, p90, p99, max)
		}
	})

	t.Run("single duration", func(t *testing.T) {
		p50, p90, p99, max := CalculateLatencies([]time.Duration{42 * time.Millisecond})
		if p50 != 42 {
			t.Errorf("p50 = %d, want 42", p50)
		}
		if p90 != 42 {
			t.Errorf("p90 = %d, want 42", p90)
		}
		if p99 != 42 {
			t.Errorf("p99 = %d, want 42", p99)
		}
		if max != 42 {
			t.Errorf("max = %d, want 42", max)
		}
	})
}

// VALIDATES: CalculateThroughput computes avg and peak routes/sec from timestamps.
// PREVENTS: Division by zero on empty timestamps or wrong peak calculation.
func TestThroughputCalculation(t *testing.T) {
	t.Run("100 timestamps over 1 second", func(t *testing.T) {
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		timestamps := make([]time.Time, 100)
		for i := range timestamps {
			// Spread 100 timestamps evenly over 1 second.
			timestamps[i] = base.Add(time.Duration(i) * 10 * time.Millisecond)
		}

		avg, peak := CalculateThroughput(timestamps)

		// 100 routes over ~990ms. avg should be ~100 routes/sec.
		if avg < 90 || avg > 110 {
			t.Errorf("avg = %d, want ~100", avg)
		}
		// All 100 routes within the same 1-second window.
		if peak != 100 {
			t.Errorf("peak = %d, want 100", peak)
		}
	})

	t.Run("empty timestamps", func(t *testing.T) {
		avg, peak := CalculateThroughput(nil)
		if avg != 0 || peak != 0 {
			t.Errorf("expected 0,0 for empty, got %d,%d", avg, peak)
		}
	})

	t.Run("single timestamp", func(t *testing.T) {
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		avg, peak := CalculateThroughput([]time.Time{base})
		if avg != 0 || peak != 0 {
			t.Errorf("expected 0,0 for single timestamp, got %d,%d", avg, peak)
		}
	})

	t.Run("two second span", func(t *testing.T) {
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		timestamps := make([]time.Time, 200)
		// 100 routes in first second, 100 routes in second second.
		for i := range 100 {
			timestamps[i] = base.Add(time.Duration(i) * 5 * time.Millisecond)
		}
		for i := range 100 {
			timestamps[100+i] = base.Add(time.Second + time.Duration(i)*5*time.Millisecond)
		}

		avg, peak := CalculateThroughput(timestamps)

		// 200 routes over ~1.5 seconds.
		if avg < 80 || avg > 200 {
			t.Errorf("avg = %d, want ~100-133", avg)
		}
		if peak < 90 || peak > 110 {
			t.Errorf("peak = %d, want ~100", peak)
		}
	})
}

// VALIDATES: Aggregate computes median and stddev across multiple iterations.
// PREVENTS: Wrong field mapping between IterationResult and AggregatedResult.
func TestAggregateIterations(t *testing.T) {
	t.Run("five iterations with known values", func(t *testing.T) {
		results := []IterationResult{
			{ConvergenceMs: 100, FirstRouteMs: 10, ThroughputAvg: 1000, ThroughputPeak: 1500, LatencyP50Ms: 5, LatencyP90Ms: 12, LatencyP99Ms: 25, LatencyMaxMs: 30, RoutesReceived: 990, SessionSenderMs: 10, SessionReceiverMs: 20},
			{ConvergenceMs: 110, FirstRouteMs: 12, ThroughputAvg: 1100, ThroughputPeak: 1600, LatencyP50Ms: 6, LatencyP90Ms: 13, LatencyP99Ms: 26, LatencyMaxMs: 31, RoutesReceived: 995, SessionSenderMs: 11, SessionReceiverMs: 21},
			{ConvergenceMs: 120, FirstRouteMs: 11, ThroughputAvg: 1050, ThroughputPeak: 1550, LatencyP50Ms: 5, LatencyP90Ms: 12, LatencyP99Ms: 24, LatencyMaxMs: 29, RoutesReceived: 992, SessionSenderMs: 10, SessionReceiverMs: 20},
			{ConvergenceMs: 105, FirstRouteMs: 13, ThroughputAvg: 1080, ThroughputPeak: 1580, LatencyP50Ms: 7, LatencyP90Ms: 14, LatencyP99Ms: 27, LatencyMaxMs: 32, RoutesReceived: 998, SessionSenderMs: 12, SessionReceiverMs: 22},
			{ConvergenceMs: 115, FirstRouteMs: 14, ThroughputAvg: 1020, ThroughputPeak: 1520, LatencyP50Ms: 6, LatencyP90Ms: 13, LatencyP99Ms: 26, LatencyMaxMs: 31, RoutesReceived: 993, SessionSenderMs: 11, SessionReceiverMs: 21},
		}

		agg := Aggregate(results)

		// Convergence values sorted: [100, 105, 110, 115, 120] -> median = 110
		if agg.ConvergenceMs != 110 {
			t.Errorf("ConvergenceMs = %d, want 110", agg.ConvergenceMs)
		}

		// FirstRoute values sorted: [10, 11, 12, 13, 14] -> median = 12
		if agg.FirstRouteMs != 12 {
			t.Errorf("FirstRouteMs = %d, want 12", agg.FirstRouteMs)
		}

		// ThroughputAvg sorted: [1000, 1020, 1050, 1080, 1100] -> median = 1050
		if agg.ThroughputAvg != 1050 {
			t.Errorf("ThroughputAvg = %d, want 1050", agg.ThroughputAvg)
		}

		// ThroughputPeak sorted: [1500, 1520, 1550, 1580, 1600] -> median = 1550
		if agg.ThroughputPeak != 1550 {
			t.Errorf("ThroughputPeak = %d, want 1550", agg.ThroughputPeak)
		}

		// RoutesReceived sorted: [990, 992, 993, 995, 998] -> median = 993
		if agg.RoutesReceived != 993 {
			t.Errorf("RoutesReceived = %d, want 993", agg.RoutesReceived)
		}

		// SessionSenderMs sorted: [10, 10, 11, 11, 12] -> median = 11
		if agg.SessionSenderMs != 11 {
			t.Errorf("SessionSenderMs = %d, want 11", agg.SessionSenderMs)
		}

		// SessionReceiverMs sorted: [20, 20, 21, 21, 22] -> median = 21
		if agg.SessionReceiverMs != 21 {
			t.Errorf("SessionReceiverMs = %d, want 21", agg.SessionReceiverMs)
		}

		// Stddev should be non-negative.
		if agg.ConvergenceStddevMs < 0 {
			t.Errorf("ConvergenceStddevMs = %d, want >= 0", agg.ConvergenceStddevMs)
		}

		// Convergence stddev: population stddev of [100,105,110,115,120].
		// Mean = 110, variance = (100+25+0+25+100)/5 = 50, stddev = sqrt(50) ~= 7.07, truncated = 7
		if agg.ConvergenceStddevMs != 7 {
			t.Errorf("ConvergenceStddevMs = %d, want 7", agg.ConvergenceStddevMs)
		}
	})
}

// VALIDATES: RemoveOutliers removes iterations beyond 2 stddev from median.
// PREVENTS: Outlier iterations skewing aggregate results.
func TestOutlierRemoval(t *testing.T) {
	t.Run("removes outlier", func(t *testing.T) {
		results := []IterationResult{
			{ConvergenceMs: 100},
			{ConvergenceMs: 105},
			{ConvergenceMs: 110},
			{ConvergenceMs: 108},
			{ConvergenceMs: 330}, // ~3x median, outlier
		}

		kept := RemoveOutliers(results)
		if len(kept) != 4 {
			t.Errorf("expected 4 kept, got %d", len(kept))
		}

		for _, r := range kept {
			if r.ConvergenceMs == 330 {
				t.Error("outlier (330) should have been removed")
			}
		}
	})
}

// VALIDATES: RemoveOutliers keeps all when all values are within 2 stddev.
// PREVENTS: Over-aggressive outlier removal.
func TestOutlierRemovalKeepsAll(t *testing.T) {
	t.Run("all within range", func(t *testing.T) {
		results := []IterationResult{
			{ConvergenceMs: 100},
			{ConvergenceMs: 102},
			{ConvergenceMs: 104},
			{ConvergenceMs: 106},
			{ConvergenceMs: 108},
		}

		kept := RemoveOutliers(results)
		if len(kept) != 5 {
			t.Errorf("expected 5 kept, got %d", len(kept))
		}
	})
}

// VALIDATES: RemoveOutliers never removes below 3 remaining.
// PREVENTS: Too few iterations remaining for meaningful statistics.
func TestOutlierRemovalMinRuns(t *testing.T) {
	t.Run("keeps minimum 3", func(t *testing.T) {
		results := []IterationResult{
			{ConvergenceMs: 100},
			{ConvergenceMs: 300}, // outlier
			{ConvergenceMs: 105},
			{ConvergenceMs: 500}, // outlier
		}

		kept := RemoveOutliers(results)
		if len(kept) < 3 {
			t.Errorf("expected at least 3 kept, got %d", len(kept))
		}
	})
}
