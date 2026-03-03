package reactor

import (
	"testing"
	"time"
)

// TestAPISyncNoProcesses verifies no wait when no processes.
//
// VALIDATES: WaitForAPIReady returns immediately with zero processes.
// PREVENTS: Unnecessary delay when no API processes configured.
func TestAPISyncNoProcesses(t *testing.T) {
	r := New(&Config{})
	// Don't call SetAPIProcessCount - defaults to 0

	start := time.Now()
	r.WaitForAPIReady()
	elapsed := time.Since(start)

	if elapsed > 10*time.Millisecond {
		t.Errorf("took too long: %v (expected immediate)", elapsed)
	}
}

// TestAPISyncSingleProcess verifies waiting for one process.
//
// VALIDATES: WaitForAPIReady waits for single ready signal.
// PREVENTS: Proceeding before process is ready.
func TestAPISyncSingleProcess(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(1)
	r.apiTimeout = 200 * time.Millisecond

	go func() {
		time.Sleep(30 * time.Millisecond)
		r.SignalAPIReady()
	}()

	start := time.Now()
	r.WaitForAPIReady()
	elapsed := time.Since(start)

	if elapsed < 20*time.Millisecond || elapsed > 100*time.Millisecond {
		t.Errorf("unexpected timing: %v (expected ~30ms)", elapsed)
	}
}

// TestAPISyncMultipleProcesses verifies waiting for all processes.
//
// VALIDATES: WaitForAPIReady waits for all N ready signals.
// PREVENTS: Proceeding before all processes are ready.
func TestAPISyncMultipleProcesses(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(3)
	r.apiTimeout = 500 * time.Millisecond

	go func() {
		time.Sleep(10 * time.Millisecond)
		r.SignalAPIReady() // 1
		time.Sleep(10 * time.Millisecond)
		r.SignalAPIReady() // 2
		time.Sleep(10 * time.Millisecond)
		r.SignalAPIReady() // 3
	}()

	start := time.Now()
	r.WaitForAPIReady()
	elapsed := time.Since(start)

	if elapsed < 25*time.Millisecond || elapsed > 100*time.Millisecond {
		t.Errorf("unexpected timing: %v (expected ~30ms)", elapsed)
	}
}

// TestAPISyncTimeout verifies timeout when process doesn't respond.
//
// VALIDATES: WaitForAPIReady times out and proceeds.
// PREVENTS: Hanging forever when process is stuck.
func TestAPISyncTimeout(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(2)
	r.apiTimeout = 100 * time.Millisecond

	// Only send 1 ready, expect 2
	go func() {
		time.Sleep(10 * time.Millisecond)
		r.SignalAPIReady()
	}()

	start := time.Now()
	r.WaitForAPIReady()
	elapsed := time.Since(start)

	if elapsed < 90*time.Millisecond || elapsed > 150*time.Millisecond {
		t.Errorf("unexpected timing: %v (expected ~100ms)", elapsed)
	}
}

// TestAPISyncImmediateReady verifies quick return when already ready.
//
// VALIDATES: WaitForAPIReady returns immediately if already ready.
// PREVENTS: Unnecessary delay on repeated calls.
func TestAPISyncImmediateReady(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(1)

	// Signal before waiting
	r.SignalAPIReady()

	start := time.Now()
	r.WaitForAPIReady()
	elapsed := time.Since(start)

	if elapsed > 10*time.Millisecond {
		t.Errorf("took too long: %v (expected immediate)", elapsed)
	}
}

// TestAPISyncMultipleCalls verifies idempotency.
//
// VALIDATES: Multiple WaitForAPIReady calls don't block.
// PREVENTS: Deadlock on repeated calls.
func TestAPISyncMultipleCalls(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(1)
	r.SignalAPIReady()

	// First call
	r.WaitForAPIReady()

	// Second call should return immediately
	start := time.Now()
	r.WaitForAPIReady()
	elapsed := time.Since(start)

	if elapsed > 10*time.Millisecond {
		t.Errorf("second call took too long: %v", elapsed)
	}
}

// TestAPISyncConcurrent verifies thread safety.
//
// VALIDATES: Concurrent SignalAPIReady calls are safe.
// PREVENTS: Race conditions in API sync signaling.
func TestAPISyncConcurrent(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(10)
	r.apiTimeout = 500 * time.Millisecond

	// Spawn 10 goroutines signaling concurrently
	for range 10 {
		go func() {
			time.Sleep(10 * time.Millisecond)
			r.SignalAPIReady()
		}()
	}

	r.WaitForAPIReady()

	// Verify all signals were received (readyCount should be 10)
	if r.readyCount.Load() != 10 {
		t.Errorf("expected 10 ready signals, got %d", r.readyCount.Load())
	}
}
