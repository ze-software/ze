package plugin

import (
	"context"
	"testing"
	"time"
)

// TestWaitForStageProgressSurvivesSlowTier
// VALIDATES: the startup barrier bounds the wait by ABSENCE OF PROGRESS, not by
// a flat wall-clock budget for the whole stage. A tier whose plugins each keep
// completing the stage -- but whose total takes longer than the stall window --
// must NOT be torn down.
// PREVENTS: regression to `deadline := StageStartTime().Add(timeout)`, a flat
// per-stage budget shared by every plugin in the tier. A real tier is ~20+
// plugins (bgp + bgp-bmp + bgp-filter-* + bgp-rpki + ...); on a loaded host they
// all slow down together, blow the shared budget, and every plugin in the tier
// is stopped ("startup barrier aborted") even though all were making progress.
func TestWaitForStageProgressSurvivesSlowTier(t *testing.T) {
	const (
		plugins = 5
		// Each plugin's gap is well inside the stall window (3.3x headroom), so
		// scheduling jitter cannot turn a healthy tier into a failure. The TOTAL
		// (5 * 300ms = 1.5s) deliberately exceeds the 1s window: that excess is
		// exactly what the old flat budget failed on and what this test pins.
		stall = 1 * time.Second
		step  = 300 * time.Millisecond
	)

	coord := NewStartupCoordinator(plugins)

	go func() {
		for i := range plugins {
			time.Sleep(step)
			coord.StageComplete(i, StageRegistration)
		}
	}()

	start := time.Now()
	err := coord.WaitForStageProgress(context.Background(), StageConfig, stall)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("WaitForStageProgress returned %v; a tier making steady progress must not be failed", err)
	}
	if elapsed < plugins*step {
		t.Fatalf("returned after %v, before the last plugin completed (%v); the barrier did not actually wait", elapsed, plugins*step)
	}
	if got := coord.CurrentStage(); got != StageConfig {
		t.Fatalf("CurrentStage = %v, want %v", got, StageConfig)
	}
}

// TestWaitForStageProgressStillFailsWedgedTier
// VALIDATES: the wait stays BOUNDED. A plugin that never completes its stage
// still trips the barrier once the tier has gone the stall window with no
// progress at all.
// PREVENTS: turning a load flake into a hang by removing the bound outright --
// an unbounded startup wait is worse than the timeout it replaced.
func TestWaitForStageProgressStillFailsWedgedTier(t *testing.T) {
	const stall = 150 * time.Millisecond

	coord := NewStartupCoordinator(2)
	// Plugin 0 completes; plugin 1 is wedged and never does.
	coord.StageComplete(0, StageRegistration)

	start := time.Now()
	err := coord.WaitForStageProgress(context.Background(), StageConfig, stall)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("WaitForStageProgress returned nil for a wedged tier; the barrier must stay bounded")
	}
	if elapsed > 10*stall {
		t.Fatalf("took %v to fail a wedged tier (stall window %v); the bound is not being applied", elapsed, stall)
	}
}

// TestWaitForStageProgressRepeatedCompleteIsNotProgress
// VALIDATES: only a plugin's FIRST completion of the current stage counts as
// progress.
// PREVENTS: a misbehaving or looping plugin holding the barrier open forever by
// re-sending StageComplete, which would silently remove the bound that
// TestWaitForStageProgressStillFailsWedgedTier pins.
func TestWaitForStageProgressRepeatedCompleteIsNotProgress(t *testing.T) {
	const stall = 200 * time.Millisecond

	coord := NewStartupCoordinator(2)

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			// Plugin 0 spams its completion; plugin 1 never completes.
			coord.StageComplete(0, StageRegistration)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	err := coord.WaitForStageProgress(context.Background(), StageConfig, stall)
	if err == nil {
		t.Fatal("repeated StageComplete from one plugin kept the barrier open; only the first completion is progress")
	}
}

// TestWaitForStageProgressHonorsContext
// VALIDATES: daemon shutdown (the parent context) still unblocks the barrier.
// PREVENTS: a stalled startup ignoring cancellation and blocking shutdown.
func TestWaitForStageProgressHonorsContext(t *testing.T) {
	coord := NewStartupCoordinator(2)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	// Stall window far longer than the test: only cancellation can end this.
	err := coord.WaitForStageProgress(ctx, StageConfig, time.Hour)
	if err == nil {
		t.Fatal("WaitForStageProgress ignored context cancellation")
	}
	if !errorsIsContextCanceled(err) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func errorsIsContextCanceled(err error) bool {
	return err == context.Canceled //nolint:errorlint // exact sentinel returned by ctx.Err()
}
