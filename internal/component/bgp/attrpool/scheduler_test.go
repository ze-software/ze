package attrpool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSchedulerRoundRobin verifies fair pool selection for compaction.
//
// VALIDATES: Fairness in pool compaction scheduling.
//
// PREVENTS: Pool starvation where one busy pool always gets compacted
// while others grow unbounded.
func TestSchedulerRoundRobin(t *testing.T) {
	pools := make([]*Pool, 3)
	for i := range pools {
		pools[i] = New(1024)
		// Create dead entries to make pools need compaction
		h := mustIntern(t, pools[i], []byte("dead"))
		_ = pools[i].Release(h)
	}

	s := NewScheduler(pools, SchedulerConfig{
		QuietPeriod:   0, // No quiet period for this test
		CheckInterval: 10 * time.Millisecond,
	})

	go s.Run(t.Context())

	// Wait until at least 2 pools have been compacted (dead slots cleared).
	require.Eventually(t, func() bool {
		compacted := 0
		for _, p := range pools {
			m := p.Metrics()
			if m.DeadSlots == 0 {
				compacted++
			}
		}
		return compacted >= 2
	}, 2*time.Second, time.Millisecond, "at least 2 pools should be compacted")
}

// TestSchedulerRespectsQuietPeriod verifies compaction waits for inactivity.
//
// VALIDATES: Compaction doesn't interfere with active operations.
//
// PREVENTS: Compaction running during high activity, causing lock
// contention and increased latency.
func TestSchedulerRespectsQuietPeriod(t *testing.T) {
	p := New(1024)

	// Create dead entry
	h := mustIntern(t, p, []byte("data"))
	_ = p.Release(h)

	s := NewScheduler([]*Pool{p}, SchedulerConfig{
		QuietPeriod:   100 * time.Millisecond,
		CheckInterval: 10 * time.Millisecond,
	})

	// Mark pool as recently active
	p.Touch()

	// Check immediately - should not be eligible
	require.False(t, s.shouldCompact(p), "pool should not compact immediately after activity")

	// Poll until quiet period elapses and pool becomes eligible.
	require.Eventually(t, func() bool {
		return s.shouldCompact(p)
	}, 2*time.Second, 10*time.Millisecond, "pool should be eligible after quiet period")
}

// TestSchedulerStop verifies graceful shutdown.
//
// VALIDATES: Clean scheduler shutdown.
//
// PREVENTS: Goroutine leaks or hanging on shutdown.
func TestSchedulerStop(t *testing.T) {
	p := New(1024)
	s := NewScheduler([]*Pool{p}, SchedulerConfig{
		CheckInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	// Verify the scheduler is still running (has not exited on its own).
	require.Never(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 30*time.Millisecond, 10*time.Millisecond, "scheduler should keep running until canceled")

	// Cancel should stop the scheduler.
	cancel()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "scheduler should stop after cancel")
}

// TestSchedulerNoPools verifies scheduler handles empty pool list.
//
// VALIDATES: Edge case - no pools to manage.
//
// PREVENTS: Panic or infinite loop with no pools.
func TestSchedulerNoPools(t *testing.T) {
	s := NewScheduler(nil, SchedulerConfig{
		CheckInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Should not panic
	require.NotPanics(t, func() {
		s.Run(ctx)
	})
}

// TestSchedulerSkipsCleanPools verifies pools without dead entries are skipped.
//
// VALIDATES: Efficient scheduling - only compact when needed.
//
// PREVENTS: Unnecessary compaction of clean pools.
func TestSchedulerSkipsCleanPools(t *testing.T) {
	p := New(1024)

	// Only live entries, no dead
	mustIntern(t, p, []byte("live1"))
	mustIntern(t, p, []byte("live2"))

	s := NewScheduler([]*Pool{p}, SchedulerConfig{
		QuietPeriod:   0,
		CheckInterval: 10 * time.Millisecond,
	})

	// Pool with no dead entries should not need compaction
	require.False(t, s.shouldCompact(p), "clean pool should not need compaction")
}

// TestSchedulerCompactsHighDeadRatio verifies pools with many dead entries are compacted.
//
// VALIDATES: Compaction triggered by dead entry ratio.
//
// PREVENTS: Memory waste from accumulated dead entries.
func TestSchedulerCompactsHighDeadRatio(t *testing.T) {
	p := New(1024)

	// Create many unique entries, then release most
	handles := make([]Handle, 10)
	for i := range handles {
		handles[i] = mustIntern(t, p, []byte{byte(i)}) // Unique data for each
	}
	// Release 8 out of 10 (80% dead)
	for i := range 8 {
		_ = p.Release(handles[i])
	}

	s := NewScheduler([]*Pool{p}, SchedulerConfig{
		QuietPeriod:        0,
		DeadRatioThreshold: 0.5, // Compact if >50% dead
	})

	// QuietPeriod is 0, so pool should be eligible immediately.
	require.Eventually(t, func() bool {
		return s.shouldCompact(p)
	}, 2*time.Second, time.Millisecond, "pool with 80% dead should need compaction")
}
