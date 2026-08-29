// Design: docs/features/interfaces.md -- Router Advertisement sender for a LAN unit
// Related: register.go -- the factory that binds the counters declared here
//
// Package ifacera sends IPv6 Router Advertisements on a configured interface
// unit, so hosts on the link autoconfigure addresses, learn a default router,
// and learn resolvers. The interface component parses the configuration and
// decides which senders run; this package owns the socket, the timers, and the
// Router Solicitation answers.
//
// The counters live here, with no build tag, so they run on every host. Only
// the socket work is Linux. The timing arithmetic is internal/core/ndp, shared
// with the PPP subscriber sender.
package ifacera

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// raMetrics holds the counters this plugin publishes.
type raMetrics struct {
	sent      metrics.CounterVec
	solicited metrics.CounterVec
}

var metricsPtr atomic.Pointer[raMetrics]

// labelInterface is the Prometheus label that carries the interface a Router
// Advertisement was sent on.
const labelInterface = "interface"

// SetMetricsRegistry binds the counters to a registry. Called through the
// plugin registration's metrics hook; until then every counter is a no-op.
func SetMetricsRegistry(reg metrics.Registry) {
	metricsPtr.Store(&raMetrics{
		sent: reg.CounterVec("ze_iface_ra_sent_total",
			"Router Advertisements sent, periodic and solicited together, by interface.",
			[]string{labelInterface}),
		solicited: reg.CounterVec("ze_iface_ra_solicited_total",
			"Router Advertisements sent in answer to a Router Solicitation, by interface.",
			[]string{labelInterface}),
	})
}

// incSent counts one advertisement put on the wire.
func incSent(ifaceName string) {
	if m := metricsPtr.Load(); m != nil {
		m.sent.With(ifaceName).Inc()
	}
}

// incSolicited counts one advertisement that answered a Router Solicitation.
// The same advertisement is counted by incSent as well, so sent stays the total.
func incSolicited(ifaceName string) {
	if m := metricsPtr.Load(); m != nil {
		m.solicited.With(ifaceName).Inc()
	}
}
