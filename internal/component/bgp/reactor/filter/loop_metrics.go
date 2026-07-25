// Design: docs/architecture/core-design.md — loop-filter Prometheus metrics

package filter

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// loopMetrics holds the loop-filter Prometheus metrics.
type loopMetrics struct {
	asPathLoopDetected metrics.CounterVec // labels: peer
}

// loopMetricsPtr stores the loop-filter metrics, set by SetMetricsRegistry.
// It stays nil until the reactor wires a registry (no-telemetry builds leave it
// nil), so LoopIngress guards every use.
var loopMetricsPtr atomic.Pointer[loopMetrics]

// SetMetricsRegistry creates the loop-filter metrics from the given registry.
// Called from the reactor's metrics-enable block. A nil registry is a no-op, so
// the increment in LoopIngress stays disabled.
func SetMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		return
	}
	loopMetricsPtr.Store(&loopMetrics{
		asPathLoopDetected: reg.CounterVec(
			"ze_bgp_as_path_loop_detected_total",
			"Routes rejected on ingress for an AS_PATH loop (local ASN in AS_PATH, RFC 4271 Section 9).",
			[]string{"peer"},
		),
	})
}

// recordASPathLoop increments the per-peer AS_PATH-loop counter, if wired.
func recordASPathLoop(peer string) {
	if m := loopMetricsPtr.Load(); m != nil {
		m.asPathLoopDetected.With(peer).Inc()
	}
}
