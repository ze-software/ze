// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- per-client restart and IPv6CP visibility.
// Related: register.go -- deferred telemetry registration; connected_session.go -- entry.
// Related: ipv6cp_client.go -- applyIPv6CP, the IPv6CP transitions the gauge follows.

package pppoeclient

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

type clientMetrics struct {
	renegotiations metrics.CounterVec
	// ipv6cpOpened is 1 while the client's IPv6CP is in the Opened state, so
	// IPv6 passes on the session, and 0 otherwise: before it opens, after the
	// peer rejected or closed it, during an LCP restart and once the session
	// ended.
	ipv6cpOpened metrics.GaugeVec
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
		ipv6cpOpened: reg.GaugeVec("ze_pppoe_client_ipv6cp_opened",
			"1 while the client's IPv6CP is Opened and IPv6 passes, 0 otherwise, by configured client name.", []string{"name"}),
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

// recordIPv6CPOpened sets the IPv6CP gauge of one client. It is called at
// every transition into and out of Opened (applyIPv6CP, restart) and at the
// end of the session (keepaliveLoop). Safe for concurrent clients and
// registry replacement; no registry means telemetry is disabled.
func recordIPv6CPOpened(name string, opened bool) {
	m := clientMetricsPtr.Load()
	if m == nil {
		return
	}
	value := 0.0
	if opened {
		value = 1
	}
	m.ipv6cpOpened.With(name).Set(value)
}
