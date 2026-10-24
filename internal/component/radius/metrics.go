// Design: docs/guide/radius.md -- the unsigned EAP answer discard and its counter
// Related: client.go -- dispatchResponse: the discard site that calls countUnsignedEAPDiscard
// Related: register.go -- init registers bindRadiusMetrics with the plugin registry

package radius

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

// metricsHookName is the key registry.InjectPluginMetrics stores this
// component's hook under. It is a namespace, not a plugin declaration: no
// plugin under internal/plugins/ carries this name.
const metricsHookName = "radius-client"

// radiusMetrics holds the Prometheus metrics the RADIUS client publishes. One
// instance serves every Client in the process, the admin login backend and the
// subscriber plugin alike, so a discard by any of them lands on one series.
type radiusMetrics struct {
	unsignedEAPDiscardsTotal metrics.Counter
}

// radiusMetricsPtr holds the active metric set. It is nil until a registry
// exists, which is why countUnsignedEAPDiscard reads it on every call: the
// discard runs on a client's read goroutine while the store happens on the
// goroutine that creates the registry.
var radiusMetricsPtr atomic.Pointer[radiusMetrics]

// registerRadiusMetrics asks for this component's counters, now if a metrics
// registry already exists and as soon as one does otherwise. The registry is
// created by the hub's telemetry start, which can run after this package's
// init, so reading registry.GetMetricsRegistry here would bind nothing on a
// daemon whose registry arrives later, with no line saying so.
func registerRadiusMetrics() {
	registry.InjectPluginMetrics(metricsHookName, bindRadiusMetrics)
}

// bindRadiusMetrics publishes the RADIUS client counters into reg. It is the
// hook registerRadiusMetrics hands to the registry.
func bindRadiusMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	radiusMetricsPtr.Store(&radiusMetrics{
		unsignedEAPDiscardsTotal: reg.Counter("ze_radius_unsigned_eap_discards_total",
			"RADIUS answers carrying an EAP-Message and no Message-Authenticator, discarded under RFC 3579 Section 3.1."),
	})
}

// countUnsignedEAPDiscard increments the unsigned EAP discard counter. The
// series is the only count of these discards: the warning dispatchResponse
// logs names each one, and this counter is what an operator graphs. It is a
// no-op while no registry is bound, which is a daemon started without a
// telemetry block, where the log line alone reports the discard.
func countUnsignedEAPDiscard() {
	m := radiusMetricsPtr.Load()
	if m == nil {
		return
	}
	m.unsignedEAPDiscardsTotal.Inc()
}
