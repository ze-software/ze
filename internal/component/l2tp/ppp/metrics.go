// Design: docs/research/l2tpv2-ze-integration.md -- IPv6CP interface-identifier negotiation
// Related: session_run.go -- afterLCPOpenIPv6Service: refuses to start the IPv6 service for an unnegotiated identifier
// Related: ncp.go -- onNCPOpened: withholds the session-up address for an unnegotiated identifier;
//   buildNakOrReject: falls back to a Configure-Reject when a Nak suggestion cannot be drawn
// Related: manager.go -- Driver.Start registers this component's metrics hook

package ppp

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

// IPv6CP identifier-refusal reasons. The set is closed and kept small.
// A session or a tunnel ID is NOT a label: a subscriber daemon carries
// thousands of PPP sessions, and a label with that cardinality turns one
// series into thousands. Every site that refuses IPv6CP for want of a
// usable interface identifier names exactly one of these three, and a
// paired log line carries the same string, so the log and the counter
// never drift into two vocabularies for one event.
const (
	// reasonServiceUnnegotiated: afterLCPOpenIPv6Service (session_run.go)
	// refuses to start the IPv6 service because IPv6CP opened without a
	// negotiated peer interface identifier.
	reasonServiceUnnegotiated = "service-unnegotiated"
	// reasonEventUnnegotiated: onNCPOpened (ncp.go) withholds the
	// session-up address for the same reason.
	reasonEventUnnegotiated = "event-unnegotiated"
	// reasonSuggestionDrawFailed: buildNakOrReject (ncp.go) could not
	// draw a valid Nak suggestion and answered with a Configure-Reject
	// instead.
	reasonSuggestionDrawFailed = "suggestion-draw-failed"
)

// metricsHookName is the key registry.InjectPluginMetrics stores this
// component's hook under. It is a namespace, not a plugin declaration: the
// registry keys the deferral by name only, and no plugin under
// internal/plugins/ carries this one.
const metricsHookName = "l2tp-ppp"

// pppMetrics holds the Prometheus metrics the PPP driver publishes. One
// instance serves every Driver in the process, so a refusal on any
// session accumulates on one series per reason rather than splitting the
// count by session or by tunnel.
type pppMetrics struct {
	ipv6cpIdentifierRefusalsTotal metrics.CounterVec // labels: reason
}

// pppMetricsPtr holds the active metric set. It is nil until a registry
// exists, which is why countIdentifierRefusal reads it on every call
// rather than caching it on the session: the refusal sites run from a
// session goroutine, while the store happens on whichever goroutine calls
// Driver.Start.
var pppMetricsPtr atomic.Pointer[pppMetrics]

// initPPPMetrics creates the PPP Prometheus metrics from reg.
func initPPPMetrics(reg metrics.Registry) *pppMetrics {
	return &pppMetrics{
		ipv6cpIdentifierRefusalsTotal: reg.CounterVec("ze_ppp_ipv6cp_identifier_refusals_total",
			"IPv6CP negotiations refused for want of a usable interface identifier, by reason.",
			[]string{"reason"}),
	}
}

// registerPPPMetrics asks for this component's counters, now if a metrics
// registry already exists and as soon as one does otherwise.
//
// Reading registry.GetMetricsRegistry here instead would bind nothing on
// an L2TP- or PPPoE-only daemon with no bgp block, and leave
// ze_ppp_ipv6cp_identifier_refusals_total absent for the process
// lifetime with no line saying so: runYANGConfig (cmd/ze/hub/main.go)
// runs engine.Start, which runs Driver.Start, and only afterwards runs
// startStandaloneTelemetry, which creates the registry. A daemon that
// also carries a bgp block gets its registry earlier, so the order is a
// property of the operator's configuration rather than of this code.
// Deferring makes whichever event happens second do the binding -- the
// same gap internal/component/l2tp/pppoe/metrics.go documents and closes
// for its own counter.
func registerPPPMetrics() {
	registry.InjectPluginMetrics(metricsHookName, bindPPPMetrics)
}

// bindPPPMetrics publishes the PPP counters into reg. It is the hook
// registerPPPMetrics hands to the registry, never called directly by the
// driver.
func bindPPPMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	pppMetricsPtr.Store(initPPPMetrics(reg))
}

// countIdentifierRefusal increments the IPv6CP identifier-refusal counter
// for reason. It is a no-op while no registry is bound, which is a daemon
// started without a telemetry block and every test that does not ask for
// the counter.
func countIdentifierRefusal(reason string) {
	m := pppMetricsPtr.Load()
	if m == nil {
		return
	}
	m.ipv6cpIdentifierRefusalsTotal.With(reason).Inc()
}
