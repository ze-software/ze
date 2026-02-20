// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"time"
)

// IterationStats tracks results across multiple test runs.
type IterationStats struct {
	Nick      string
	Passed    int
	Failed    int
	TimedOut  int
	Durations []time.Duration
}

// Add records a single test iteration result.
func (s *IterationStats) Add(state State, duration time.Duration) {
	switch state { //nolint:exhaustive // only terminal states matter
	case StateSuccess:
		s.Passed++
	case StateFail:
		s.Failed++
	case StateTimeout:
		s.TimedOut++
	}
	s.Durations = append(s.Durations, duration)
}

// Total returns the total number of iterations.
func (s *IterationStats) Total() int {
	return s.Passed + s.Failed + s.TimedOut
}

// Min returns the minimum duration.
func (s *IterationStats) Min() time.Duration {
	if len(s.Durations) == 0 {
		return 0
	}
	min := s.Durations[0]
	for _, d := range s.Durations[1:] {
		if d < min {
			min = d
		}
	}
	return min
}

// Max returns the maximum duration.
func (s *IterationStats) Max() time.Duration {
	if len(s.Durations) == 0 {
		return 0
	}
	max := s.Durations[0]
	for _, d := range s.Durations[1:] {
		if d > max {
			max = d
		}
	}
	return max
}

// Avg returns the average duration.
func (s *IterationStats) Avg() time.Duration {
	if len(s.Durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range s.Durations {
		total += d
	}
	return total / time.Duration(len(s.Durations))
}

// PassRate returns the percentage of passed iterations.
func (s *IterationStats) PassRate() float64 {
	total := s.Total()
	if total == 0 {
		return 0
	}
	return float64(s.Passed) / float64(total) * 100
}

// StressStats maps test nicks to their iteration stats.
type StressStats map[string]*IterationStats

// NewStressStats creates a stats map for all tests.
func NewStressStats(tests *Tests) StressStats {
	stats := make(StressStats)
	for _, r := range tests.Registered() {
		stats[r.Nick] = &IterationStats{Nick: r.Nick}
	}
	return stats
}

// StressResult holds results from a stress test run.
type StressResult struct {
	Stats              StressStats
	IterationDurations []time.Duration
	TotalDuration      time.Duration
	AllPassed          bool
}

// IterationMin returns the minimum iteration duration.
func (r *StressResult) IterationMin() time.Duration {
	if len(r.IterationDurations) == 0 {
		return 0
	}
	min := r.IterationDurations[0]
	for _, d := range r.IterationDurations[1:] {
		if d < min {
			min = d
		}
	}
	return min
}

// IterationMax returns the maximum iteration duration.
func (r *StressResult) IterationMax() time.Duration {
	if len(r.IterationDurations) == 0 {
		return 0
	}
	max := r.IterationDurations[0]
	for _, d := range r.IterationDurations[1:] {
		if d > max {
			max = d
		}
	}
	return max
}

// IterationAvg returns the average iteration duration.
func (r *StressResult) IterationAvg() time.Duration {
	if len(r.IterationDurations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range r.IterationDurations {
		total += d
	}
	return total / time.Duration(len(r.IterationDurations))
}
