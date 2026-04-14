// Design: docs/research/vpp-deployment-reference.md -- VPP FIB route programming
// Overview: fibvpp.go -- fibVPP type with installed map
//
// FIB VPP metrics: route count gauge and install/removal counters.
// Follows the same pattern as fibkernel (SetMetricsRegistry + atomic pointer).

package fibvpp

import (
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// fibVPPMetrics holds Prometheus metrics for the fib-vpp plugin.
type fibVPPMetrics struct {
	routesInstalled metrics.Gauge   // current installed route count
	routeInstalls   metrics.Counter // routes successfully added
	routeUpdates    metrics.Counter // routes successfully replaced
	routeRemovals   metrics.Counter // routes successfully withdrawn
}

// fibVPPMetricsPtr stores fib-vpp metrics, set by SetMetricsRegistry.
var fibVPPMetricsPtr atomic.Pointer[fibVPPMetrics]

// SetMetricsRegistry creates fib-vpp metrics from the given registry.
// Called via ConfigureMetrics callback before RunEngine.
func SetMetricsRegistry(reg metrics.Registry) {
	m := &fibVPPMetrics{
		routesInstalled: reg.Gauge("ze_fibvpp_routes_installed", "Current number of VPP FIB routes installed by ze."),
		routeInstalls:   reg.Counter("ze_fibvpp_route_installs_total", "Routes successfully added to VPP FIB."),
		routeUpdates:    reg.Counter("ze_fibvpp_route_updates_total", "Routes successfully replaced in VPP FIB."),
		routeRemovals:   reg.Counter("ze_fibvpp_route_removals_total", "Routes successfully removed from VPP FIB."),
	}
	fibVPPMetricsPtr.Store(m)
}
