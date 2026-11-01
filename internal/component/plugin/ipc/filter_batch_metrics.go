// Design: docs/plugin-development/metrics.md -- ze_plugin_filter_batch_routes
// Related: filter_batch.go -- filterBatcher.exchange, which observes each batch

package ipc

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// filterBatchMetrics holds the slow path's filter-update metrics.
type filterBatchMetrics struct {
	routes metrics.Histogram
}

// filterBatchMetricsPtr stays nil until the plugin server wires a registry,
// so observeFilterBatch guards its use.
var filterBatchMetricsPtr atomic.Pointer[filterBatchMetrics]

// filterBatchBuckets are powers of two up to rpc.FilterUpdateBatchMax, the
// most routes one exchange carries.
var filterBatchBuckets = []float64{1, 2, 4, 8, 16, 32, rpc.FilterUpdateBatchMax}

// SetFilterMetrics creates the filter-update batch histogram from reg. The
// engine's plugin server calls it when it receives its metrics registry. A
// nil registry is a no-op.
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
