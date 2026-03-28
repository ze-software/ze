package reactor

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestAPISyncNoProcesses verifies no wait when no processes.
//
// VALIDATES: WaitForAPIReady returns immediately with zero processes.
// PREVENTS: Unnecessary delay when no API processes configured.
func TestAPISyncNoProcesses(t *testing.T) {
	r := New(&Config{})
	// Don't call SetAPIProcessCount - defaults to 0

	done := make(chan struct{})
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return immediately with zero processes")
}

// TestAPISyncSingleProcess verifies waiting for one process.
//
// VALIDATES: WaitForAPIReady waits for single ready signal.
// PREVENTS: Proceeding before process is ready.
func TestAPISyncSingleProcess(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(1)
	r.apiTimeout = 2 * time.Second

	done := make(chan struct{})

	// WaitForAPIReady should NOT return before we signal.
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	// Verify it does NOT complete before the signal.
	require.Never(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, time.Millisecond, "WaitForAPIReady should block until signal")

	// Now signal ready.
	r.SignalAPIReady()

	// Verify it completes after the signal.
	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return after signal")
}

// TestAPISyncMultipleProcesses verifies waiting for all processes.
//
// VALIDATES: WaitForAPIReady waits for all N ready signals.
// PREVENTS: Proceeding before all processes are ready.
func TestAPISyncMultipleProcesses(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(3)
	r.apiTimeout = 2 * time.Second

	done := make(chan struct{})
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	// After 1 signal, should still be blocked.
	r.SignalAPIReady()
	require.Never(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, time.Millisecond, "WaitForAPIReady should block after 1 of 3 signals")

	// After 2 signals, should still be blocked.
	r.SignalAPIReady()
	require.Never(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, time.Millisecond, "WaitForAPIReady should block after 2 of 3 signals")

	// After 3rd signal, should unblock.
	r.SignalAPIReady()
	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return after all 3 signals")
}

// TestAPISyncTimeout verifies timeout when process doesn't respond.
//
// VALIDATES: WaitForAPIReady times out and proceeds.
// PREVENTS: Hanging forever when process is stuck.
func TestAPISyncTimeout(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(2)
	r.apiTimeout = 100 * time.Millisecond

	// Only send 1 ready, expect 2 -- must timeout.
	r.SignalAPIReady()

	done := make(chan struct{})
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	// Should NOT return immediately (still waiting for 2nd signal).
	require.Never(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, time.Millisecond, "WaitForAPIReady should not return before timeout")

	// Should eventually return after the 100ms timeout.
	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return after timeout")
}

// TestAPISyncImmediateReady verifies quick return when already ready.
//
// VALIDATES: WaitForAPIReady returns immediately if already ready.
// PREVENTS: Unnecessary delay on repeated calls.
func TestAPISyncImmediateReady(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(1)

	// Signal before waiting.
	r.SignalAPIReady()

	done := make(chan struct{})
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return immediately when already ready")
}

// TestAPISyncMultipleCalls verifies idempotency.
//
// VALIDATES: Multiple WaitForAPIReady calls don't block.
// PREVENTS: Deadlock on repeated calls.
func TestAPISyncMultipleCalls(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(1)
	r.SignalAPIReady()

	// First call.
	r.WaitForAPIReady()

	// Second call should return immediately.
	done := make(chan struct{})
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "second WaitForAPIReady call should return immediately")
}

// TestAPISyncConcurrent verifies thread safety.
//
// VALIDATES: Concurrent SignalAPIReady calls are safe.
// PREVENTS: Race conditions in API sync signaling.
func TestAPISyncConcurrent(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(10)
	r.apiTimeout = 2 * time.Second

	// Use a WaitGroup to start all goroutines simultaneously.
	var ready sync.WaitGroup
	ready.Add(10)

	var started sync.WaitGroup
	started.Add(10)

	// Spawn 10 goroutines signaling concurrently.
	for range 10 {
		go func() {
			started.Done()
			ready.Wait() // All goroutines start signaling at the same time.
			r.SignalAPIReady()
		}()
	}

	// Wait for all goroutines to be ready, then release them.
	started.Wait()
	ready.Add(-10) // Release all.

	done := make(chan struct{})
	var readyCount int32
	go func() {
		r.WaitForAPIReady()
		readyCount = r.readyCount.Load()
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return after all concurrent signals")

	// Verify all signals were received.
	require.Equal(t, int32(10), readyCount, "all 10 ready signals should be received")
}
