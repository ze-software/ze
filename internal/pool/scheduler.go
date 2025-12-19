package pool

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
}

// Scheduler manages compaction across multiple pools.
// Only one pool compacts at a time. Uses round-robin for fairness.
type Scheduler struct {
	pools  []*Pool
	config SchedulerConfig

	mu        sync.Mutex
	lastIndex int // round-robin cursor
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

	return &Scheduler{
		pools:     pools,
		config:    config,
		lastIndex: -1,
	}
}

// Run starts the scheduler loop. Blocks until context is cancelled.
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

	// Find next pool needing compaction (round-robin)
	n := len(s.pools)
	for i := 0; i < n; i++ {
		idx := (s.lastIndex + 1 + i) % n
		p := s.pools[idx]

		if s.shouldCompactLocked(p) {
			s.lastIndex = idx
			p.Compact()
			return // Only compact one pool per tick
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
