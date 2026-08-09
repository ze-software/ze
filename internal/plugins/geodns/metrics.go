// Design: docs/architecture/dns/geodns.md -- geodns Prometheus metrics

package geodns

import (
	"sync/atomic"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/metrics"
)

// latencyBuckets are exponential ms buckets (0.1ms .. ~205ms), matching the
// reference daemon's histogram resolution for DNS request handling.
var latencyBuckets = []float64{0.1, 0.2, 0.4, 0.8, 1.6, 3.2, 6.4, 12.8, 25.6, 51.2, 102.4, 204.8}

// geodnsMetrics holds the plugin's counters/gauges. Label sets are deliberately
// bounded (configured zones, the fixed qtype/rcode enums, the two protocols, the
// operator-set listen addresses) so cardinality stays flat; never label by
// client IP or queried hostname.
type geodnsMetrics struct {
	requestTotal  metrics.CounterVec // labels: zone, qtype
	responseTotal metrics.CounterVec // labels: zone, qtype, rcode
	latency       metrics.Histogram
	listenerUp    metrics.GaugeVec   // labels: protocol, address (server-config)
	reloadTotal   metrics.CounterVec // labels: result
}

// geodnsMetricsPtr is swapped from a lazy Nop default to the host registry's
// metrics when ConfigureMetrics fires (before the engine runs), so the query
// path never nil-checks.
var geodnsMetricsPtr atomic.Pointer[geodnsMetrics]

// buildMetrics registers the metric set against reg.
func buildMetrics(reg metrics.Registry) *geodnsMetrics {
	return &geodnsMetrics{
		requestTotal:  reg.CounterVec("ze_geodns_dns_request_total", "DNS requests received, by zone and query type.", []string{"zone", "qtype"}),
		responseTotal: reg.CounterVec("ze_geodns_dns_response_total", "DNS responses sent, by zone, query type and response code.", []string{"zone", "qtype", "rcode"}),
		latency:       reg.Histogram("ze_geodns_dns_request_latency_milliseconds", "Time to handle a DNS request, in milliseconds.", latencyBuckets),
		listenerUp:    reg.GaugeVec("ze_geodns_listener_up", "1 while a listener socket is bound, by protocol and listen address.", []string{"protocol", "address"}),
		reloadTotal:   reg.CounterVec("ze_geodns_config_reload_total", "Config generations applied by the engine, by result.", []string{"result"}),
	}
}

// setMetricsRegistry publishes metrics backed by the host registry. Called via
// the plugin Registration's ConfigureMetrics before RunEngine.
func setMetricsRegistry(reg metrics.Registry) { geodnsMetricsPtr.Store(buildMetrics(reg)) }

// gmetrics returns the current metric set, lazily defaulting to a no-op registry
// so increments are safe before ConfigureMetrics has run.
func gmetrics() *geodnsMetrics {
	if m := geodnsMetricsPtr.Load(); m != nil {
		return m
	}
	geodnsMetricsPtr.CompareAndSwap(nil, buildMetrics(metrics.NopRegistry{}))
	return geodnsMetricsPtr.Load()
}

// qtypeLabel maps a query type to a bounded label, collapsing anything outside
// the served set to OTHER so cardinality cannot be driven by arbitrary qtypes.
func qtypeLabel(qtype uint16) string {
	switch qtype {
	case dns.TypeA, dns.TypeAAAA, dns.TypeSRV, dns.TypeSOA, dns.TypeNS, dns.TypeANY:
		return dns.TypeToString[qtype]
	default:
		return "OTHER"
	}
}
