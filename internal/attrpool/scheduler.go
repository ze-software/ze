// Design: docs/architecture/pool-architecture.md — attribute and NLRI pools

package attrpool

import (
	"context"
	"sync"
	"time"
)

// SchedulerConfig configures the compaction scheduler.
type SchedulerConfig struct {
	// QuietPeriod is how long a pool must be idle before compaction.
	// Default: 100ms
	QuietPeriod time.Duration

	// CheckInterval is how often to check for pools needing compaction.
	// Default: 50ms
	CheckInterval time.Duration

	// DeadRatioThreshold is the minimum dead/total ratio to trigger compaction.
	// Default: 0.25 (25%)
	DeadRatioThreshold float64

	// MigrateBatchSize is number of slots to migrate per tick.
	// Default: 100
	MigrateBatchSize int
}

// Scheduler manages compaction across multiple pools.
// Only one pool compacts at a time. Uses round-robin for fairness.
// Uses incremental MigrateBatch for non-blocking compaction.
type Scheduler struct {
	pools  []*Pool
	config SchedulerConfig

	mu         sync.Mutex
	lastIndex  int   // round-robin cursor
	activePool *Pool // pool currently being compacted
}

// NewScheduler creates a scheduler for the given pools.
func NewScheduler(pools []*Pool, config SchedulerConfig) *Scheduler {
	// Apply defaults
	if config.CheckInterval == 0 {
		config.CheckInterval = 50 * time.Millisecond
	}
	if config.DeadRatioThreshold == 0 {
		config.DeadRatioThreshold = 0.25
	}
	if config.MigrateBatchSize == 0 {
		config.MigrateBatchSize = 100
	}

	return &Scheduler{
		pools:     pools,
		config:    config,
		lastIndex: -1,
	}
}

// Run starts the scheduler loop. Blocks until context is canceled.
func (s *Scheduler) Run(ctx context.Context) {
	if len(s.pools) == 0 {
		<-ctx.Done()
		return
	}

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

// tick performs one scheduling cycle.
func (s *Scheduler) tick() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Continue active compaction
	if s.activePool != nil {
		// Check if any pool has activity - pause if so
		for _, p := range s.pools {
			if s.config.QuietPeriod > 0 && !p.IsIdle(s.config.QuietPeriod) {
				return // Pause compaction during activity
			}
		}

		// Continue migration
		done := s.activePool.MigrateBatch(s.config.MigrateBatchSize)
		if done {
			// Check if old buffer can be freed
			s.activePool.CheckOldBufferRelease()
			if s.activePool.State() == PoolNormal {
				s.activePool = nil
			}
		}
		return
	}

	// Find next pool needing compaction (round-robin)
	n := len(s.pools)
	for i := range n {
		idx := (s.lastIndex + 1 + i) % n
		p := s.pools[idx]

		if s.shouldCompactLocked(p) {
			s.lastIndex = idx
			p.StartCompaction()
			s.activePool = p
			return // Start compaction, will continue in subsequent ticks
		}
	}
}

// shouldCompact returns true if the pool needs and is ready for compaction.
// Thread-safe.
func (s *Scheduler) shouldCompact(p *Pool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shouldCompactLocked(p)
}

// shouldCompactLocked checks if pool should be compacted.
// Caller must hold s.mu.
func (s *Scheduler) shouldCompactLocked(p *Pool) bool {
	// Check quiet period
	if s.config.QuietPeriod > 0 && !p.IsIdle(s.config.QuietPeriod) {
		return false
	}

	// Check if pool has dead entries worth compacting
	m := p.Metrics()
	if m.DeadSlots == 0 {
		return false
	}

	// Check dead ratio threshold
	if m.TotalSlots > 0 {
		deadRatio := float64(m.DeadSlots) / float64(m.TotalSlots)
		return deadRatio >= s.config.DeadRatioThreshold
	}

	return m.DeadSlots > 0
}
