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

// schedUnit is the compaction unit: a single shard of a pool. Sharding made each
// shard an independent compaction target (own lock, buffer and dead-ratio), so
// the scheduler schedules shards, not whole pools. A 1-shard pool contributes
// exactly one unit, reproducing the pre-sharding pool-at-a-time behavior.
type schedUnit struct {
	pool  *Pool
	shard *shard
}

// Scheduler manages compaction across multiple pools' shards.
// Only one shard compacts at a time. Uses round-robin over shards for fairness.
// Uses incremental MigrateBatch for non-blocking compaction.
type Scheduler struct {
	pools  []*Pool
	units  []schedUnit
	config SchedulerConfig

	mu        sync.Mutex
	lastIndex int        // round-robin cursor over units
	active    *schedUnit // shard currently being compacted
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

	// Flatten every pool's shards into independent compaction units.
	var units []schedUnit
	for _, p := range pools {
		for i := range p.shards {
			units = append(units, schedUnit{pool: p, shard: &p.shards[i]})
		}
	}

	return &Scheduler{
		pools:     pools,
		units:     units,
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

	// Continue active compaction.
	if s.active != nil {
		// Pause migration while the owning pool is being written. The quiet gate
		// stays at pool granularity: compaction copies bytes (memory bandwidth,
		// CPU), so it should stay out of the way during a pool's activity burst,
		// not just when the one shard it is migrating happens to be quiet. Which
		// shard to compact is still decided per shard (see deadRatioExceeds); only
		// the "interfere with live traffic" gate is pool-wide.
		if s.config.QuietPeriod > 0 && !s.active.pool.IsIdle(s.config.QuietPeriod) {
			return
		}

		sh := s.active.shard
		if sh.migrateBatch(s.config.MigrateBatchSize) {
			sh.checkOldBufferRelease()
			if !sh.isCompacting() {
				s.active = nil
			}
		}
		return
	}

	// Find next shard needing compaction (round-robin over all units). The dead
	// ratio is judged per shard (fixing pool-wide dilution); the quiet period is
	// judged per owning pool (matching the pause gate above). Units are contiguous
	// per pool (see NewScheduler), so a single-entry cache collapses the repeated
	// per-pool IsIdle checks to one per pool without allocating.
	n := len(s.units)
	var cachedPool *Pool
	var cachedIdle bool
	for i := range n {
		idx := (s.lastIndex + 1 + i) % n
		u := &s.units[idx]

		if s.config.QuietPeriod > 0 {
			if u.pool != cachedPool {
				cachedPool = u.pool
				cachedIdle = u.pool.IsIdle(s.config.QuietPeriod)
			}
			if !cachedIdle {
				continue
			}
		}
		if u.shard.deadRatioExceeds(s.config.DeadRatioThreshold) {
			s.lastIndex = idx
			u.shard.startCompaction()
			s.active = u
			return // Start compaction, will continue in subsequent ticks
		}
	}
}
