package attrpool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// shouldCompact reports whether the pool is idle enough and any of its shards is
// fragmented enough to compact, mirroring the decision tick() makes per unit. It
// is a test-only seam for asserting pool-level eligibility; production code in
// tick() inlines the per-shard dead-ratio check and the per-pool quiet gate.
func (s *Scheduler) shouldCompact(p *Pool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config.QuietPeriod > 0 && !p.IsIdle(s.config.QuietPeriod) {
		return false
	}
	for i := range p.shards {
		if p.shards[i].deadRatioExceeds(s.config.DeadRatioThreshold) {
			return true
		}
	}
	return false
}

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

// TestSchedulerQuietPeriodGatesEligibleShard verifies the quiet period is judged
// for the whole pool: a shard that is fragmented enough to compact is still held
// off while the pool is active. This pins the pool-level quiet gate (compaction
// copies bytes and must stay out of the way during a pool's activity burst),
// distinct from the per-shard dead-ratio decision.
//
// PREVENTS: regressing to a per-shard idle gate that lets one shard compact while
// the rest of the pool is mid-burst.
func TestSchedulerQuietPeriodGatesEligibleShard(t *testing.T) {
	p := New(1 << 16)

	// Make one shard fragmented enough to be eligible by dead ratio.
	dead := make([]Handle, 0, 10)
	for _, k := range genKeysInShard(0, 10) {
		dead = append(dead, mustIntern(t, p, k))
	}
	for _, h := range dead {
		require.NoError(t, p.Release(h))
	}

	s := NewScheduler([]*Pool{p}, SchedulerConfig{
		QuietPeriod:        time.Hour, // effectively never idle once touched
		DeadRatioThreshold: 0.25,
	})

	p.Touch() // mark the pool active
	require.False(t, s.shouldCompact(p),
		"an eligible shard must not compact while its pool is active")
}

// TestSchedulerCompactsFragmentedShardDespiteLowAggregate verifies the per-shard
// scheduler compacts a single heavily-fragmented shard even when the pool-wide
// dead ratio is below the threshold. An aggregate (pool-level) scheduler would
// dilute that shard's fragmentation across the other shards and never compact it.
//
// VALIDATES: compaction is decided per shard, fixing the dilution starvation.
//
// PREVENTS: a hot shard accumulating dead entries indefinitely because clean
// shards keep the pool-wide ratio low.
func TestSchedulerCompactsFragmentedShardDespiteLowAggregate(t *testing.T) {
	p := New(1 << 16) // default 16-shard pool
	const target = 0

	// Shard `target`: intern 20 distinct entries, then release them all, so the
	// shard ends with 20 dead slots (releasing one at a time would just reuse the
	// freed slot and leave a single dead entry).
	dead := make([]Handle, 0, 20)
	for _, k := range genKeysInShard(target, 20) {
		h := mustIntern(t, p, k)
		require.Equal(t, uint32(target), h.shardID())
		dead = append(dead, h)
	}
	for _, h := range dead {
		require.NoError(t, p.Release(h))
	}

	// Other shards: many live entries so the pool-wide dead ratio stays low.
	live := 0
	for i := 0; live < 200; i++ {
		k := fmt.Appendf(nil, "live-%d", i)
		if defaultShardOf(k) == target {
			continue // keep shard `target` purely dead
		}
		mustIntern(t, p, k)
		live++
	}

	// Precondition: aggregate dead ratio is below the 25% threshold.
	m := p.Metrics()
	require.Equal(t, int32(20), m.DeadSlots)
	require.Less(t, float64(m.DeadSlots)/float64(m.TotalSlots), 0.25,
		"precondition: pool-wide dead ratio must be below threshold")

	s := NewScheduler([]*Pool{p}, SchedulerConfig{
		QuietPeriod:        0,
		CheckInterval:      time.Millisecond,
		DeadRatioThreshold: 0.25,
		MigrateBatchSize:   8,
	})
	go s.Run(t.Context())

	require.Eventually(t, func() bool {
		return p.Metrics().DeadSlots == 0
	}, 2*time.Second, time.Millisecond,
		"fragmented shard must be compacted despite the low pool-wide ratio")

	require.Equal(t, int32(200), p.Metrics().LiveSlots, "live entries untouched")
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
