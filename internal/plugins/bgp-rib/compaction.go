// Design: docs/architecture/pool-architecture.md — compaction scheduler wiring
// Overview: rib.go — RIB plugin lifecycle

package bgp_rib

import (
	"context"

	"codeberg.org/thomas-mangin/ze/internal/attrpool"
)

// runCompaction runs the compaction scheduler for the given pools.
// Blocks until ctx is canceled. Called as a goroutine from OnStarted.
func runCompaction(ctx context.Context, pools []*attrpool.Pool) {
	sched := attrpool.NewScheduler(pools, attrpool.SchedulerConfig{})
	sched.Run(ctx)
}
