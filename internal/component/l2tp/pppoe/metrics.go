// Design: docs/architecture/l2tp/bng-5-pppoe.md -- discovery refusal visibility
// Related: server.go -- handlePADI, handlePADR: the refusal sites that call countRefusal
// Related: subsystem.go -- Start registers bindPPPoEMetrics with the plugin registry

package pppoe

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

// Discovery refusal reasons. The set is closed and kept small: each site that
// refuses a discovery packet for an RFC or configuration condition names
// exactly one of these, and a paired log line (where one exists) carries the
// same string, so the log and the counter never drift into two vocabularies
// for one event.
const (
	reasonRateLimited         = "rate-limited"
	reasonServiceNameMismatch = "service-name-mismatch"
	reasonServiceNameMissing  = "service-name-missing"
	reasonCookieInvalid       = "cookie-invalid"
	reasonSessionIDExhausted  = "session-id-exhausted"
	reasonPerMACCapReached    = "per-mac-cap-reached"
)

// metricsHookName is the key registry.InjectPluginMetrics stores this
// component's hook under. It is a namespace, not a plugin declaration: the
// registry keys the deferral by name only, and no plugin under
// internal/plugins/ carries this one.
const metricsHookName = "l2tp-pppoe"

// pppoeMetrics holds the Prometheus metrics the PPPoE component publishes.
// One instance serves every InterfaceServer in the process, so a refusal on
// any access interface accumulates on one series per reason rather than
// splitting the count by interface.
type pppoeMetrics struct {
	discoveryRefusalsTotal metrics.CounterVec // labels: reason
}

// pppoeMetricsPtr holds the active metric set. It is nil until a registry
// exists, which is why countRefusal reads it on every call rather than
// caching it on the server: the refusal sites run from the discovery reader
// goroutine while the store happens on the startup goroutine.
var pppoeMetricsPtr atomic.Pointer[pppoeMetrics]

// initPPPoEMetrics creates the PPPoE Prometheus metrics from reg.
func initPPPoEMetrics(reg metrics.Registry) *pppoeMetrics {
	return &pppoeMetrics{
		discoveryRefusalsTotal: reg.CounterVec("ze_pppoe_discovery_refusals_total",
			"PPPoE discovery packets (PADI or PADR) refused, by reason.", []string{"reason"}),
	}
}

// registerDiscoveryMetrics asks for this component's counters, now if a
// metrics registry already exists and as soon as one does otherwise.
//
// Reading registry.GetMetricsRegistry here instead would bind nothing on a
// PPPoE-only daemon and leave ze_pppoe_discovery_refusals_total absent for
// the process lifetime, with no line saying so. runYANGConfig
// (cmd/ze/hub/main.go) runs engine.Start, which runs Subsystem.Start, and
// only afterwards runs startStandaloneTelemetry, which creates the registry.
// A daemon that also carries a bgp block gets its registry earlier, from the
// reactor's config loader during plugin start, so the order is a property of
// the operator's configuration rather than of this code. Deferring makes
// whichever event happens second do the binding.
func registerDiscoveryMetrics() {
	registry.InjectPluginMetrics(metricsHookName, bindPPPoEMetrics)
}

// bindPPPoEMetrics publishes the discovery counters into reg. It is the hook
// registerDiscoveryMetrics hands to the registry, never called directly by
// the subsystem.
func bindPPPoEMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	pppoeMetricsPtr.Store(initPPPoEMetrics(reg))
}

// countRefusal increments the discovery-refusal counter for reason. It is a
// no-op while no registry is bound, which is a daemon started without a
// telemetry block and every test that does not ask for the counter.
func countRefusal(reason string) {
	m := pppoeMetricsPtr.Load()
	if m == nil {
		return
	}
	m.discoveryRefusalsTotal.With(reason).Inc()
}
