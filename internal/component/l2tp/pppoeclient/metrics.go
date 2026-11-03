// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- per-client restart visibility.
// Related: register.go -- deferred telemetry registration; connected_session.go -- entry.

package pppoeclient

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

type clientMetrics struct {
	renegotiations metrics.CounterVec
}

var clientMetricsPtr atomic.Pointer[clientMetrics]

// bindClientMetrics runs when the daemon supplies its telemetry registry.
func bindClientMetrics(reg metrics.Registry) {
	if reg == nil {
		clientMetricsPtr.Store(nil)
		return
	}
	clientMetricsPtr.Store(&clientMetrics{
		renegotiations: reg.CounterVec("ze_pppoe_client_renegotiations_total",
			"Network-phase LCP renegotiations started, by configured client name.", []string{"name"}),
	})
}

// countRenegotiation is safe for concurrent clients and registry replacement.
// No registry means telemetry is disabled; negotiation still runs.
func countRenegotiation(name string) {
	m := clientMetricsPtr.Load()
	if m == nil {
		return
	}
	m.renegotiations.With(name).Inc()
}
