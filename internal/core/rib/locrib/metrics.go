// Design: docs/architecture/rib/unified-locrib.md -- per-shard Loc-RIB metrics
// Related: manager.go -- counters incremented from RIB.insert / RIB.Remove

package locrib

import (
	"strconv"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// locribMetrics groups the per-shard counters and gauges. One instance per
// process, set via SetMetricsRegistry. Atomic pointer load on the hot path
// so the no-metrics case is one branch.
type locribMetrics struct {
	inserts metrics.CounterVec // labels: family, shard
	removes metrics.CounterVec // labels: family, shard
	lookups metrics.CounterVec // labels: family, shard
	depth   metrics.GaugeVec   // labels: family, shard
}

var locribMetricsPtr atomic.Pointer[locribMetrics]

// shardLabels are the two labels every Loc-RIB shard metric carries: the
// address family the shard holds and the shard's own index. A new slice is
// returned on each call because the registry keeps the slice it is given.
func shardLabels() []string {
	return []string{"family", "shard"}
}

// SetMetricsRegistry wires Prometheus (or any metrics.Registry) into the
// locrib package. Calling with nil unregisters; idempotent. Safe to call
// from any goroutine.
func SetMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		locribMetricsPtr.Store(nil)
		return
	}
	m := &locribMetrics{
		inserts: reg.CounterVec(
			"ze_locrib_shard_inserts_total",
			"Inserts handled by each Loc-RIB shard, partitioned by family.",
			shardLabels(),
		),
		removes: reg.CounterVec(
			"ze_locrib_shard_removes_total",
			"Removes handled by each Loc-RIB shard, partitioned by family.",
			shardLabels(),
		),
		lookups: reg.CounterVec(
			"ze_locrib_shard_lookups_total",
			"Lookups served by each Loc-RIB shard, partitioned by family.",
			shardLabels(),
		),
		depth: reg.GaugeVec(
			"ze_locrib_shard_depth",
			"Number of prefixes currently held by each Loc-RIB shard.",
			shardLabels(),
		),
	}
	locribMetricsPtr.Store(m)
}

// shardLabel formats the shard index as a metric label value.
func shardLabel(idx int) string { return strconv.Itoa(idx) }

func recordInsert(family string, shardIdx int) {
	if m := locribMetricsPtr.Load(); m != nil {
		m.inserts.With(family, shardLabel(shardIdx)).Inc()
	}
}

func recordRemove(family string, shardIdx int) {
	if m := locribMetricsPtr.Load(); m != nil {
		m.removes.With(family, shardLabel(shardIdx)).Inc()
	}
}

func recordLookup(family string, shardIdx int) {
	if m := locribMetricsPtr.Load(); m != nil {
		m.lookups.With(family, shardLabel(shardIdx)).Inc()
	}
}

func updateDepth(family string, shardIdx, depth int) {
	if m := locribMetricsPtr.Load(); m != nil {
		m.depth.With(family, shardLabel(shardIdx)).Set(float64(depth))
	}
}
