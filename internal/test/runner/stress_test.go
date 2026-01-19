package runner

import (
	"testing"
	"time"
)

// TestIterationStats_Add verifies stats accumulation.
//
// VALIDATES: Stats correctly track pass/fail/timeout counts and durations.
//
// PREVENTS: Lost data when adding multiple iterations.
func TestIterationStats_Add(t *testing.T) {
	stats := &IterationStats{Nick: "0"}

	stats.Add(StateSuccess, 100*time.Millisecond)
	stats.Add(StateSuccess, 200*time.Millisecond)
	stats.Add(StateFail, 150*time.Millisecond)

	if stats.Passed != 2 {
		t.Errorf("Passed = %d, want 2", stats.Passed)
	}
	if stats.Failed != 1 {
		t.Errorf("Failed = %d, want 1", stats.Failed)
	}
	if len(stats.Durations) != 3 {
		t.Errorf("Durations len = %d, want 3", len(stats.Durations))
	}
}

// TestIterationStats_Timeout verifies timeout tracking.
//
// VALIDATES: Timeouts are tracked separately from failures.
//
// PREVENTS: Timeouts being miscounted as failures.
func TestIterationStats_Timeout(t *testing.T) {
	stats := &IterationStats{Nick: "0"}

	stats.Add(StateTimeout, 30*time.Second)
	stats.Add(StateFail, 100*time.Millisecond)

	if stats.TimedOut != 1 {
		t.Errorf("TimedOut = %d, want 1", stats.TimedOut)
	}
	if stats.Failed != 1 {
		t.Errorf("Failed = %d, want 1", stats.Failed)
	}
}

// TestIterationStats_MinMaxAvg verifies timing statistics.
//
// VALIDATES: Min/Max/Avg correctly computed from durations.
//
// PREVENTS: Wrong timing statistics in stress test reports.
func TestIterationStats_MinMaxAvg(t *testing.T) {
	stats := &IterationStats{Nick: "0"}

	stats.Add(StateSuccess, 100*time.Millisecond)
	stats.Add(StateSuccess, 300*time.Millisecond)
	stats.Add(StateSuccess, 200*time.Millisecond)

	if stats.Min() != 100*time.Millisecond {
		t.Errorf("Min() = %v, want 100ms", stats.Min())
	}
	if stats.Max() != 300*time.Millisecond {
		t.Errorf("Max() = %v, want 300ms", stats.Max())
	}
	if stats.Avg() != 200*time.Millisecond {
		t.Errorf("Avg() = %v, want 200ms", stats.Avg())
	}
}

// TestIterationStats_PassRate verifies pass rate calculation.
//
// VALIDATES: Pass rate = passed / total * 100.
//
// PREVENTS: Wrong pass rate in stress test summary.
func TestIterationStats_PassRate(t *testing.T) {
	stats := &IterationStats{Nick: "0"}

	stats.Add(StateSuccess, 100*time.Millisecond)
	stats.Add(StateSuccess, 100*time.Millisecond)
	stats.Add(StateFail, 100*time.Millisecond)
	stats.Add(StateTimeout, 100*time.Millisecond)

	// 2 passed out of 4 total = 50%
	rate := stats.PassRate()
	if rate != 50.0 {
		t.Errorf("PassRate() = %v, want 50.0", rate)
	}
}

// TestIterationStats_Empty verifies handling of no data.
//
// VALIDATES: Empty stats return zero values without panic.
//
// PREVENTS: Division by zero or nil pointer panics.
func TestIterationStats_Empty(t *testing.T) {
	stats := &IterationStats{Nick: "0"}

	if stats.Min() != 0 {
		t.Errorf("Min() = %v, want 0", stats.Min())
	}
	if stats.Max() != 0 {
		t.Errorf("Max() = %v, want 0", stats.Max())
	}
	if stats.Avg() != 0 {
		t.Errorf("Avg() = %v, want 0", stats.Avg())
	}
	if stats.PassRate() != 0 {
		t.Errorf("PassRate() = %v, want 0", stats.PassRate())
	}
}

// TestIterationStats_Total verifies total count.
//
// VALIDATES: Total() returns sum of passed + failed + timeout.
//
// PREVENTS: Wrong total in statistics display.
func TestIterationStats_Total(t *testing.T) {
	stats := &IterationStats{Nick: "0"}

	stats.Add(StateSuccess, 100*time.Millisecond)
	stats.Add(StateFail, 100*time.Millisecond)
	stats.Add(StateTimeout, 100*time.Millisecond)

	if stats.Total() != 3 {
		t.Errorf("Total() = %d, want 3", stats.Total())
	}
}

// TestNewStressStats verifies stress stats collection creation.
//
// VALIDATES: NewStressStats creates a map keyed by test nick.
//
// PREVENTS: Missing stats for tests.
func TestNewStressStats(t *testing.T) {
	ResetNickCounter()
	tests := NewTests()
	tests.Add("test1")
	tests.Add("test2")

	stats := NewStressStats(tests)

	if len(stats) != 2 {
		t.Errorf("len(stats) = %d, want 2", len(stats))
	}

	// Nicks are "0" and "1" (generated)
	if _, ok := stats["0"]; !ok {
		t.Error("missing stats for nick '0'")
	}
	if _, ok := stats["1"]; !ok {
		t.Error("missing stats for nick '1'")
	}
}
