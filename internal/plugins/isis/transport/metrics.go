// Design: plan/learned/929-isis-3-l2-transport.md -- transport Prometheus metrics
// Related: transport.go -- send/receive/open paths increment these
//
// This spec OWNS and registers the transport rows of the umbrella Metrics
// (canonical) table -- and ONLY those rows. Other owners register their own
// series; isis-13 only scrapes them. Names are exactly ze_isis_*; labels are
// `interface` and (for drops) `reason`, per the umbrella table.

package transport

import "github.com/ze-software/ze/internal/core/metrics"

// transportMetrics holds the four transport-owned series.
type transportMetrics struct {
	framesSent     metrics.CounterVec // ze_isis_frames_sent_total{interface}
	framesReceived metrics.CounterVec // ze_isis_frames_received_total{interface}
	framesDropped  metrics.CounterVec // ze_isis_frames_dropped_total{interface,reason}
	socketsOpen    metrics.Gauge      // ze_isis_sockets_open
}

// newTransportMetrics registers the transport series on reg. Exact names and
// labels come from the umbrella Metrics-table rows owned by isis-3.
func newTransportMetrics(reg metrics.Registry) *transportMetrics {
	return &transportMetrics{
		framesSent: reg.CounterVec(
			"ze_isis_frames_sent_total",
			"Total IS-IS frames transmitted, by interface.",
			[]string{"interface"},
		),
		framesReceived: reg.CounterVec(
			"ze_isis_frames_received_total",
			"Total IS-IS frames received, by interface.",
			[]string{"interface"},
		),
		framesDropped: reg.CounterVec(
			"ze_isis_frames_dropped_total",
			"Total IS-IS frames dropped, by interface and reason.",
			[]string{"interface", "reason"},
		),
		socketsOpen: reg.Gauge(
			"ze_isis_sockets_open",
			"Current number of open IS-IS raw L2 sockets.",
		),
	}
}

// nopTransportMetrics returns metrics backed by the no-op registry, used before
// SetMetrics wires a real Prometheus registry (and in unit tests).
func nopTransportMetrics() *transportMetrics {
	return newTransportMetrics(metrics.NopRegistry{})
}
