// Design: docs/plugin-development/metrics.md -- BMP receiver route-inject and sender message-drop counters
// Related: register.go -- ConfigureMetrics wiring
// Related: bmp.go -- receiver route injection errors
// Related: bmp_events.go -- sender collector delivery failures

package bmp

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

const (
	metricBMPRouteInjectErrors = "ze_bmp_route_inject_errors_total"
	metricBMPMessageDrops      = "ze_bmp_message_drops_total"

	metricLabelPeer      = "peer"
	metricLabelCollector = "collector"
	metricLabelMessage   = "message"
	metricLabelReason    = "reason"
)

type bmpMetrics struct {
	routeInjectErrors metrics.CounterVec
	messageDrops      metrics.CounterVec
}

// metricsPtr stores BMP Prometheus counters, set by SetMetricsRegistry.
var metricsPtr atomic.Pointer[bmpMetrics]

// SetMetricsRegistry registers the BMP Prometheus metrics on reg.
// Passing nil disables metric updates (used in tests).
func SetMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		metricsPtr.Store(nil)
		return
	}
	metricsPtr.Store(&bmpMetrics{
		routeInjectErrors: reg.CounterVec(
			metricBMPRouteInjectErrors,
			"BMP receiver route-monitoring UPDATEs that failed injection into the BGP RIB.",
			[]string{metricLabelPeer},
		),
		messageDrops: reg.CounterVec(
			metricBMPMessageDrops,
			"BMP sender messages dropped because delivery to a collector failed.",
			[]string{metricLabelCollector, metricLabelMessage, metricLabelReason},
		),
	})
}

func incRouteInjectErrorMetric(peer string) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.routeInjectErrors.With(peer).Inc()
}

func incMessageDropMetric(collector, message, reason string) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.messageDrops.With(collector, message, reason).Inc()
}
