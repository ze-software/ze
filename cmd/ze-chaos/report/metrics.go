// Design: docs/architecture/chaos-web-dashboard.md — chaos reporting and metrics

package report

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// Metrics tracks Prometheus counters, gauges, and histograms for chaos events.
// Uses a per-instance registry (not the global default) so tests don't interfere
// and the HTTP handler serves only chaos-specific metrics.
// Implements the Consumer interface.
type Metrics struct {
	registry *prometheus.Registry

	routesAnnounced  prometheus.Counter
	routesReceived   prometheus.Counter
	routesWithdrawn  prometheus.Counter
	chaosEvents      prometheus.Counter
	reconnections    prometheus.Counter
	peersEstablished prometheus.Gauge
}

// NewMetrics creates a Metrics instance with all counters and gauges registered.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	routesAnnounced := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ze_chaos_routes_announced_total",
		Help: "Total routes announced by simulated peers.",
	})
	routesReceived := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ze_chaos_routes_received_total",
		Help: "Total routes received back from the route server.",
	})
	routesWithdrawn := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ze_chaos_routes_withdrawn_total",
		Help: "Total routes withdrawn by chaos actions.",
	})
	chaosEvents := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ze_chaos_chaos_events_total",
		Help: "Total chaos events executed.",
	})
	reconnections := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ze_chaos_reconnections_total",
		Help: "Total peer reconnections after chaos disconnects.",
	})
	peersEstablished := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ze_chaos_peers_established",
		Help: "Number of peers currently in Established state.",
	})

	reg.MustRegister(routesAnnounced, routesReceived, routesWithdrawn,
		chaosEvents, reconnections, peersEstablished)

	return &Metrics{
		registry:         reg,
		routesAnnounced:  routesAnnounced,
		routesReceived:   routesReceived,
		routesWithdrawn:  routesWithdrawn,
		chaosEvents:      chaosEvents,
		reconnections:    reconnections,
		peersEstablished: peersEstablished,
	}
}

// ProcessEvent updates Prometheus metrics based on the event type.
func (m *Metrics) ProcessEvent(ev peer.Event) {
	switch ev.Type {
	case peer.EventEstablished:
		m.peersEstablished.Inc()
	case peer.EventDisconnected:
		m.peersEstablished.Dec()
	case peer.EventRouteSent:
		m.routesAnnounced.Inc()
	case peer.EventRouteReceived:
		m.routesReceived.Inc()
	case peer.EventChaosExecuted:
		m.chaosEvents.Inc()
	case peer.EventReconnecting:
		m.reconnections.Inc()
	case peer.EventWithdrawalSent:
		m.routesWithdrawn.Add(float64(ev.Count))
	case peer.EventRouteWithdrawn, peer.EventEORSent, peer.EventError, peer.EventRouteAction:
		// No specific metric for these event types.
	}
}

// Handler returns an HTTP handler that serves the /metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Close is a no-op — the caller manages the HTTP server lifecycle.
func (m *Metrics) Close() error {
	return nil
}
