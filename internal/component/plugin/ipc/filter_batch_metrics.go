// Design: docs/plugin-development/metrics.md -- ze_plugin_filter_batch_routes
// Related: filter_batch.go -- filterBatcher.exchange, which observes each batch

package ipc

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// filterBatchMetrics holds queue admissions, completed typed judgments and
// encoded batches.
type filterBatchMetrics struct {
	enqueuedRoutes metrics.Counter
	slotRoutes     metrics.Counter
	routes         metrics.Histogram
}

// filterBatchMetricsPtr stays nil until the plugin server wires a registry,
// so observeFilterBatch guards its use.
var filterBatchMetricsPtr atomic.Pointer[filterBatchMetrics]

// filterBatchBuckets are powers of two up to rpc.FilterUpdateBatchMax, the
// most routes one exchange carries.
var filterBatchBuckets = []float64{1, 2, 4, 8, 16, 32, rpc.FilterUpdateBatchMax}

// SetFilterMetrics creates the queue/slot counters and batch histogram from
// reg. The engine's plugin server calls it when it receives its metrics
// registry. A nil registry is a no-op.
func SetFilterMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	filterBatchMetricsPtr.Store(&filterBatchMetrics{
		routes: reg.Histogram(
			"ze_plugin_filter_batch_routes",
			"Routes carried by one filter-update exchange with a forked plugin.",
			filterBatchBuckets,
		),
		slotRoutes: reg.Counter(
			"ze_plugin_filter_slot_routes_total",
			"Routes successfully answered through a plugin's typed filter slot.",
		),
		enqueuedRoutes: reg.Counter(
			"ze_plugin_filter_route_enqueues_total",
			"Routes successfully admitted to a plugin's encoded filter-update queue.",
		),
	})
}

// observeFilterBatch records one exchange of routes routes.
func observeFilterBatch(routes int) {
	m := filterBatchMetricsPtr.Load()
	if m == nil {
		return
	}
	m.routes.Observe(float64(routes))
}

// observeFilterSlot records a route successfully answered by the typed slot.
// Closed slots and evaluator errors do not count as completed judgments.
func observeFilterSlot() {
	if m := filterBatchMetricsPtr.Load(); m != nil {
		m.slotRoutes.Inc()
	}
}

// observeFilterEnqueue records a successful queue send, before admit returns.
// Closed connections and canceled waits that enqueue nothing do not count.
func observeFilterEnqueue() {
	if m := filterBatchMetricsPtr.Load(); m != nil {
		m.enqueuedRoutes.Inc()
	}
}
