// Design: plan/learned/639-rib-unified.md -- Phase 4 (per-shard locrib metrics)
// Related: manager.go -- counters incremented from RIB.insert / RIB.Remove

package locrib

import (
	"strconv"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
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
			[]string{"family", "shard"},
		),
		removes: reg.CounterVec(
			"ze_locrib_shard_removes_total",
			"Removes handled by each Loc-RIB shard, partitioned by family.",
			[]string{"family", "shard"},
		),
		lookups: reg.CounterVec(
			"ze_locrib_shard_lookups_total",
			"Lookups served by each Loc-RIB shard, partitioned by family.",
			[]string{"family", "shard"},
		),
		depth: reg.GaugeVec(
			"ze_locrib_shard_depth",
			"Number of prefixes currently held by each Loc-RIB shard.",
			[]string{"family", "shard"},
		),
	}
	locribMetricsPtr.Store(m)
}

// shardLabel formats the shard index as a metric label value.
func shardLabel(idx int) string { return strconv.Itoa(idx) }

func recordInsert(famStr string, shardIdx int) {
	if m := locribMetricsPtr.Load(); m != nil {
		m.inserts.With(famStr, shardLabel(shardIdx)).Inc()
	}
}

func recordRemove(famStr string, shardIdx int) {
	if m := locribMetricsPtr.Load(); m != nil {
		m.removes.With(famStr, shardLabel(shardIdx)).Inc()
	}
}

func recordLookup(famStr string, shardIdx int) {
	if m := locribMetricsPtr.Load(); m != nil {
		m.lookups.With(famStr, shardLabel(shardIdx)).Inc()
	}
}

func updateDepth(famStr string, shardIdx, depth int) {
	if m := locribMetricsPtr.Load(); m != nil {
		m.depth.With(famStr, shardLabel(shardIdx)).Set(float64(depth))
	}
}
