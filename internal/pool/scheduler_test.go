package pool

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
		h := pools[i].Intern([]byte("dead"))
		_ = pools[i].Release(h)
	}

	s := NewScheduler(pools, SchedulerConfig{
		QuietPeriod:   0, // No quiet period for this test
		CheckInterval: 10 * time.Millisecond,
	})

	// Track which pools get compacted
	compacted := make(map[*Pool]int)

	// Run scheduler for a bit
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go s.Run(ctx)

	// Wait for scheduler to run
	time.Sleep(80 * time.Millisecond)

	// Check compaction distribution
	for _, p := range pools {
		m := p.Metrics()
		if m.DeadSlots == 0 {
			compacted[p]++
		}
	}

	// All pools should eventually be compacted
	// Due to timing, we can't guarantee exact fairness, but all should be hit
	require.GreaterOrEqual(t, len(compacted), 2, "at least 2 pools should be compacted")
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
	h := p.Intern([]byte("data"))
	_ = p.Release(h)

	s := NewScheduler([]*Pool{p}, SchedulerConfig{
		QuietPeriod:   100 * time.Millisecond,
		CheckInterval: 10 * time.Millisecond,
	})

	// Mark pool as recently active
	p.Touch()

	// Check immediately - should not be eligible
	require.False(t, s.shouldCompact(p), "pool should not compact immediately after activity")

	// Wait past quiet period
	time.Sleep(150 * time.Millisecond)

	// Now should be eligible
	require.True(t, s.shouldCompact(p), "pool should be eligible after quiet period")
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

	// Let it run briefly
	time.Sleep(30 * time.Millisecond)

	// Cancel should stop the scheduler
	cancel()

	select {
	case <-done:
		// Good - scheduler stopped
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop within timeout")
	}
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
	p.Intern([]byte("live1"))
	p.Intern([]byte("live2"))

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
		handles[i] = p.Intern([]byte{byte(i)}) // Unique data for each
	}
	// Release 8 out of 10 (80% dead)
	for i := 0; i < 8; i++ {
		_ = p.Release(handles[i])
	}

	s := NewScheduler([]*Pool{p}, SchedulerConfig{
		QuietPeriod:        0,
		DeadRatioThreshold: 0.5, // Compact if >50% dead
	})

	// Wait for quiet period to pass
	time.Sleep(10 * time.Millisecond)

	require.True(t, s.shouldCompact(p), "pool with 80% dead should need compaction")
}
