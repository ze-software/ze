// Design: docs/architecture/core-design.md — read throttle for forward pool congestion
// Overview: forward_pool.go — per-peer forward worker pool
// Related: reactor_metrics.go — metrics loop polls overflow depth, pool ratio, source stats

package reactor

import (
	"context"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
)

// ReadThrottle computes a sleep duration to insert between TCP reads from
// source peers whose traffic is causing forward pool congestion. The sleep
// is proportional to BOTH the pool fill level AND the source peer's overflow
// ratio, so peers causing more pressure are throttled harder.
//
// The maximum sleep never exceeds keepalive_interval / 6 (AC-9) to prevent
// hold timer expiry. With a typical 90s hold time (30s keepalive), the max
// sleep is 5 seconds.
//
// Safe for concurrent use from multiple session read goroutines.
type ReadThrottle struct {
	// poolFillRatio returns pool utilization (0.0 = empty, 1.0 = full).
	// Maps to fwdPool.PoolUsedRatio().
	poolFillRatio func() float64

	// sourceRatio returns the overflow ratio (0.0-1.0) for the given source
	// peer: overflowed/(forwarded+overflowed). Maps to per-source stats.
	sourceRatio func(sourceAddr string) float64

	// enabled controls whether throttling is active. When false,
	// ComputeSleep always returns 0. Controlled by ze.fwd.throttle.enabled.
	enabled bool

	// clock for creating timers in ThrottleSleep. Injected for testability.
	clock clock.Clock
}

// ComputeSleep returns the sleep duration to insert after reading a message
// from the given source peer. Returns 0 if no throttling is needed.
//
// The sleep is computed from a lookup table indexed by pool fill level and
// source overflow ratio:
//
//	| Pool fill | Source overflow ratio | Sleep range     |
//	|-----------|---------------------|-----------------|
//	| 0-25%     | Any                 | 0 (normal)      |
//	| 25-50%    | Low (<10%)          | 0               |
//	| 25-50%    | High (>50%)         | 1-5ms           |
//	| 50-75%    | Low (<10%)          | 0-1ms           |
//	| 50-75%    | High (>50%)         | 10-50ms         |
//	| 75-100%   | Any                 | 100-500ms       |
//
// The result is clamped to keepaliveInterval/6 (AC-9).
func (rt *ReadThrottle) ComputeSleep(sourceAddr string, keepaliveInterval time.Duration) time.Duration {
	if !rt.enabled {
		return 0
	}

	// Hold time 0 means no timers (RFC 4271 Section 4.4). We cannot safely
	// throttle reads because there is no keepalive budget to stay within.
	if keepaliveInterval <= 0 {
		return 0
	}

	fillRatio := rt.poolFillRatio()
	if fillRatio <= 0 {
		return 0
	}
	if fillRatio > 1.0 {
		fillRatio = 1.0
	}

	srcRatio := rt.sourceRatio(sourceAddr)

	var sleepMs float64

	switch {
	case fillRatio <= 0.25:
		// No throttle below 25% fill
		return 0

	case fillRatio <= 0.50:
		// Moderate fill: only throttle high-ratio sources
		if srcRatio < 0.10 {
			return 0
		}
		// Scale 1-5ms based on source ratio and fill within this band
		bandProgress := (fillRatio - 0.25) / 0.25 // 0.0 to 1.0 within band
		sleepMs = 1.0 + 4.0*bandProgress*srcRatio

	case fillRatio <= 0.75:
		// High fill: throttle everyone, proportional to source ratio
		if srcRatio < 0.10 {
			// Low-ratio sources get 0-1ms
			bandProgress := (fillRatio - 0.50) / 0.25
			sleepMs = bandProgress * 1.0
		} else {
			// High-ratio sources get 10-50ms
			bandProgress := (fillRatio - 0.50) / 0.25
			sleepMs = 10.0 + 40.0*bandProgress*srcRatio
		}

	default: // fillRatio > 0.75 — critical fill
		// Critical fill (75-100%): everyone gets throttled hard
		bandProgress := (fillRatio - 0.75) / 0.25
		if bandProgress > 1.0 {
			bandProgress = 1.0
		}
		// Base 100ms + up to 400ms based on fill and ratio
		sleepMs = 100.0 + 400.0*bandProgress*srcRatio
		// Even low-ratio sources get meaningful throttle at critical fill
		if sleepMs < 100.0*bandProgress {
			sleepMs = 100.0 * bandProgress
		}
	}

	d := time.Duration(sleepMs * float64(time.Millisecond))

	// AC-9: clamp to keepalive_interval / 6
	maxSleep := keepaliveInterval / 6
	if d > maxSleep {
		d = maxSleep
	}

	return d
}

// ThrottleSleep sleeps for the computed throttle duration, interruptible by
// context cancellation. If ComputeSleep returns 0, this is a no-op.
func (rt *ReadThrottle) ThrottleSleep(ctx context.Context, sourceAddr string, keepaliveInterval time.Duration) {
	d := rt.ComputeSleep(sourceAddr, keepaliveInterval)
	if d <= 0 {
		return
	}

	timer := rt.clock.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C():
		// Full sleep completed
	case <-ctx.Done():
		// Interrupted by shutdown
	}
}
