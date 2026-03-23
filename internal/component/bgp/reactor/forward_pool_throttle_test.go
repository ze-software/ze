package reactor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
)

// TestReadThrottle_NoThrottleWhenNoOverflow verifies no sleep is returned
// when the overflow pool is empty (no congestion).
//
// VALIDATES: AC-2 — "Overflow pool crosses fill threshold" — when pool is empty, no throttle.
// PREVENTS: False positive throttling when system is healthy.
func TestReadThrottle_NoThrottleWhenNoOverflow(t *testing.T) {
	rt := &ReadThrottle{
		poolFillRatio: func() float64 { return 0.0 },
		sourceRatio:   func(_ string) float64 { return 0 },
		enabled:       true,
	}

	d := rt.ComputeSleep("10.0.0.1", 30*time.Second)
	assert.Equal(t, time.Duration(0), d, "no sleep when overflow is empty")
}

// TestReadThrottle_ThrottlesHighRatioSource verifies that a source peer with
// high overflow ratio gets throttled when pool fill exceeds threshold.
//
// VALIDATES: AC-2 — "Source peer read throttle activates, targeted by overflow ratio"
// PREVENTS: High-overflow source peers reading at full speed during congestion.
func TestReadThrottle_ThrottlesHighRatioSource(t *testing.T) {
	rt := &ReadThrottle{
		poolFillRatio: func() float64 { return 0.75 },
		sourceRatio:   func(_ string) float64 { return 0.8 },
		enabled:       true,
	}

	d := rt.ComputeSleep("10.0.0.1", 30*time.Second)
	assert.Greater(t, d, time.Duration(0), "high-ratio source should be throttled at 75% fill")
	assert.LessOrEqual(t, d, 500*time.Millisecond, "throttle should not exceed 500ms")
}

// TestReadThrottle_LowRatioSourceNotThrottledAtLowFill verifies a source peer
// with low overflow ratio is not throttled when pool fill is moderate.
//
// VALIDATES: AC-2 — targeting by overflow ratio means low-ratio sources are spared at moderate fill.
// PREVENTS: Innocent peers being throttled when they are not causing the pressure.
func TestReadThrottle_LowRatioSourceNotThrottledAtLowFill(t *testing.T) {
	rt := &ReadThrottle{
		poolFillRatio: func() float64 { return 0.30 },
		sourceRatio:   func(_ string) float64 { return 0.05 },
		enabled:       true,
	}

	d := rt.ComputeSleep("10.0.0.1", 30*time.Second)
	assert.Equal(t, time.Duration(0), d, "low-ratio source should not be throttled at 30% fill")
}

// TestReadThrottle_ThrottleEasesAsPoolDrains verifies that throttle duration
// decreases as the pool drains.
//
// VALIDATES: AC-3 — "Throttle eases proportionally, lowest-ratio sources first"
// PREVENTS: Throttle staying locked at maximum even after congestion eases.
func TestReadThrottle_ThrottleEasesAsPoolDrains(t *testing.T) {
	fillLevel := 0.80
	rt := &ReadThrottle{
		poolFillRatio: func() float64 { return fillLevel },
		sourceRatio:   func(_ string) float64 { return 0.6 },
		enabled:       true,
	}

	d80 := rt.ComputeSleep("10.0.0.1", 30*time.Second)

	fillLevel = 0.40
	d40 := rt.ComputeSleep("10.0.0.1", 30*time.Second)

	fillLevel = 0.10
	d10 := rt.ComputeSleep("10.0.0.1", 30*time.Second)

	assert.Greater(t, d80, d40, "throttle should be higher at 80%% than at 40%%")
	assert.Greater(t, d40, d10, "throttle should be higher at 40%% than at 10%%")
}

// TestReadThrottle_LowestRatioSourceFirstToRecover verifies that as pool drains,
// lower-ratio sources recover (stop being throttled) before higher-ratio ones.
//
// VALIDATES: AC-3 — "lowest-ratio sources first"
// PREVENTS: All sources recovering at the same time regardless of their contribution.
func TestReadThrottle_LowestRatioSourceFirstToRecover(t *testing.T) {
	rt := &ReadThrottle{
		poolFillRatio: func() float64 { return 0.35 },
		sourceRatio: func(addr string) float64 {
			if addr == "low" {
				return 0.05
			}
			return 0.8
		},
		enabled: true,
	}

	dLow := rt.ComputeSleep("low", 30*time.Second)
	dHigh := rt.ComputeSleep("high", 30*time.Second)

	assert.Equal(t, time.Duration(0), dLow, "low-ratio source should not be throttled at moderate fill")
	assert.Greater(t, dHigh, time.Duration(0), "high-ratio source should still be throttled at moderate fill")
}

// TestReadThrottle_ClampsToKeepaliveInterval verifies the max sleep never
// exceeds keepalive_interval / 6.
//
// VALIDATES: AC-9 — "Never exceeds source peer keepalive_interval / 6"
// PREVENTS: Hold timer expiry caused by read throttle sleeping too long.
func TestReadThrottle_ClampsToKeepaliveInterval(t *testing.T) {
	rt := &ReadThrottle{
		poolFillRatio: func() float64 { return 1.0 },
		sourceRatio:   func(_ string) float64 { return 1.0 },
		enabled:       true,
	}

	keepaliveInterval := 30 * time.Second
	maxAllowed := keepaliveInterval / 6 // 5s

	d := rt.ComputeSleep("10.0.0.1", keepaliveInterval)
	assert.LessOrEqual(t, d, maxAllowed,
		"throttle must not exceed keepalive_interval/6 (%v), got %v", maxAllowed, d)
}

// TestReadThrottle_ClampsSmallKeepalive verifies clamping works with short
// keepalive intervals (e.g. 3s hold time -> 1s keepalive -> 166ms max).
//
// VALIDATES: AC-9 — boundary case with minimal hold time.
// PREVENTS: Throttle exceeding safe limit for aggressive hold timer configs.
func TestReadThrottle_ClampsSmallKeepalive(t *testing.T) {
	rt := &ReadThrottle{
		poolFillRatio: func() float64 { return 1.0 },
		sourceRatio:   func(_ string) float64 { return 1.0 },
		enabled:       true,
	}

	keepaliveInterval := 1 * time.Second
	maxAllowed := keepaliveInterval / 6 // ~166ms

	d := rt.ComputeSleep("10.0.0.1", keepaliveInterval)
	assert.LessOrEqual(t, d, maxAllowed,
		"throttle must not exceed keepalive_interval/6 (%v), got %v", maxAllowed, d)
	assert.Greater(t, d, time.Duration(0), "should still throttle")
}

// TestReadThrottle_Disabled verifies no throttle when disabled via env var.
//
// VALIDATES: AC-2 — env var ze.fwd.throttle.enabled=false disables throttling.
// PREVENTS: Throttling when operator explicitly disabled it.
func TestReadThrottle_Disabled(t *testing.T) {
	rt := &ReadThrottle{
		poolFillRatio: func() float64 { return 1.0 },
		sourceRatio:   func(_ string) float64 { return 1.0 },
		enabled:       false,
	}

	d := rt.ComputeSleep("10.0.0.1", 30*time.Second)
	assert.Equal(t, time.Duration(0), d, "throttle should return 0 when disabled")
}

// TestReadThrottle_ThrottleSleepTable verifies the throttle table from the spec:
// fill level x source ratio -> expected sleep range.
//
// VALIDATES: AC-2 — complete throttle table from spec.
// PREVENTS: Throttle curve deviating from design intent.
func TestReadThrottle_ThrottleSleepTable(t *testing.T) {
	tests := []struct {
		name      string
		fillPct   float64 // fill ratio (0.0-1.0)
		srcRatio  float64
		expectMin time.Duration
		expectMax time.Duration
	}{
		{"0-25% any ratio", 0.20, 0.9, 0, 0},
		{"25-50% low ratio", 0.35, 0.05, 0, 0},
		{"25-50% high ratio", 0.35, 0.6, 1 * time.Millisecond, 5 * time.Millisecond},
		{"50-75% low ratio", 0.60, 0.05, 0, 1 * time.Millisecond},
		{"50-75% high ratio", 0.60, 0.6, 10 * time.Millisecond, 50 * time.Millisecond},
		{"75-100% any ratio", 0.90, 0.05, 1 * time.Millisecond, 500 * time.Millisecond},
		{"75-100% high ratio", 0.90, 0.9, 100 * time.Millisecond, 500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &ReadThrottle{
				poolFillRatio: func() float64 { return tt.fillPct },
				sourceRatio:   func(_ string) float64 { return tt.srcRatio },
				enabled:       true,
			}

			d := rt.ComputeSleep("10.0.0.1", 30*time.Second)
			if tt.expectMin == 0 && tt.expectMax == 0 {
				assert.Equal(t, time.Duration(0), d, "expected no throttle")
			} else {
				assert.GreaterOrEqual(t, d, tt.expectMin, "sleep too short")
				assert.LessOrEqual(t, d, tt.expectMax, "sleep too long")
			}
		})
	}
}

// TestReadThrottle_InterruptibleSleep verifies ThrottleSleep is canceled when
// context is canceled (for shutdown).
//
// VALIDATES: AC-2 — throttle sleep must be interruptible for clean shutdown.
// PREVENTS: Shutdown blocked by sleeping read goroutine.
func TestReadThrottle_InterruptibleSleep(t *testing.T) {
	rt := &ReadThrottle{
		poolFillRatio: func() float64 { return 0.9 },
		sourceRatio:   func(_ string) float64 { return 0.8 },
		enabled:       true,
		clock:         clock.RealClock{},
	}

	ctx, cancel := context.WithCancel(context.Background())

	var sleptDuration atomic.Int64
	done := make(chan struct{})
	go func() {
		start := time.Now()
		rt.ThrottleSleep(ctx, "10.0.0.1", 30*time.Second)
		sleptDuration.Store(int64(time.Since(start)))
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
		d := time.Duration(sleptDuration.Load())
		assert.Less(t, d, 200*time.Millisecond, "sleep should have been interrupted quickly")
	case <-time.After(2 * time.Second):
		t.Fatal("ThrottleSleep not interrupted by context cancel")
	}
}

// TestReadThrottle_ZeroKeepaliveNoThrottle verifies that with keepalive=0
// (hold time 0 = no timers), no throttle is applied since there's no safe maximum.
//
// VALIDATES: AC-9 — edge case where hold time is 0 (RFC 4271 §4.4 "no KEEPALIVE messages").
// PREVENTS: Division by zero or undefined behavior with hold time 0.
func TestReadThrottle_ZeroKeepaliveNoThrottle(t *testing.T) {
	rt := &ReadThrottle{
		poolFillRatio: func() float64 { return 1.0 },
		sourceRatio:   func(_ string) float64 { return 1.0 },
		enabled:       true,
	}

	d := rt.ComputeSleep("10.0.0.1", 0)
	assert.Equal(t, time.Duration(0), d, "no throttle when keepalive is 0 (timers disabled)")
}

// TestReadThrottle_NegativeKeepaliveNoThrottle verifies negative keepalive is handled.
//
// VALIDATES: AC-9 boundary: invalid-below (negative keepalive).
// PREVENTS: Unexpected behavior with malformed keepalive values.
func TestReadThrottle_NegativeKeepaliveNoThrottle(t *testing.T) {
	rt := &ReadThrottle{
		poolFillRatio: func() float64 { return 1.0 },
		sourceRatio:   func(_ string) float64 { return 1.0 },
		enabled:       true,
	}

	d := rt.ComputeSleep("10.0.0.1", -1*time.Second)
	assert.Equal(t, time.Duration(0), d, "no throttle when keepalive is negative")
}

// TestReadThrottle_BoundaryValues verifies exact fill ratio boundaries (0.25, 0.50, 0.75).
//
// VALIDATES: AC-2 boundary testing per TDD rules.
// PREVENTS: Off-by-one at band transitions.
func TestReadThrottle_BoundaryValues(t *testing.T) {
	highSrc := func(_ string) float64 { return 0.8 }

	tests := []struct {
		name      string
		fill      float64
		expectMin time.Duration
		expectMax time.Duration
	}{
		{"at 0.25 (last no-throttle)", 0.25, 0, 0},
		{"at 0.50 with high src (band 2 max)", 0.50, 1 * time.Millisecond, 5 * time.Millisecond},
		{"at 0.75 with high src (band 3 max)", 0.75, 10 * time.Millisecond, 50 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &ReadThrottle{
				poolFillRatio: func() float64 { return tt.fill },
				sourceRatio:   highSrc,
				enabled:       true,
			}
			d := rt.ComputeSleep("10.0.0.1", 30*time.Second)
			if tt.expectMin == 0 && tt.expectMax == 0 {
				assert.Equal(t, time.Duration(0), d)
			} else {
				assert.GreaterOrEqual(t, d, tt.expectMin)
				assert.LessOrEqual(t, d, tt.expectMax)
			}
		})
	}
}

// TestReadThrottle_SourceRatioClamped verifies srcRatio values outside [0,1]
// are clamped and don't produce invalid sleep durations.
//
// VALIDATES: ComputeSleep robustness with out-of-range source ratios.
// PREVENTS: Negative return values or sleep exceeding documented range.
func TestReadThrottle_SourceRatioClamped(t *testing.T) {
	tests := []struct {
		name     string
		srcRatio float64
	}{
		{"negative ratio", -0.5},
		{"ratio above 1", 2.0},
		{"ratio way above 1", 10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &ReadThrottle{
				poolFillRatio: func() float64 { return 0.90 },
				sourceRatio:   func(_ string) float64 { return tt.srcRatio },
				enabled:       true,
			}
			d := rt.ComputeSleep("10.0.0.1", 30*time.Second)
			assert.GreaterOrEqual(t, d, time.Duration(0), "sleep must never be negative")
			assert.LessOrEqual(t, d, 500*time.Millisecond, "sleep must not exceed documented max (500ms)")
		})
	}
}
