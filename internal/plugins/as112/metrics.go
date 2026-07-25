// Design: plan/learned/1033-as112-2-dns-server.md -- as112 Prometheus metrics
// Related: internal/core/metrics -- Registry/CounterVec/Histogram/GaugeVec

package as112

import (
	"sync/atomic"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/metrics"
)

// latencyBuckets are exponential ms buckets (0.1ms .. ~205ms), matching
// geodns's histogram resolution for DNS request handling.
var latencyBuckets = []float64{0.1, 0.2, 0.4, 0.8, 1.6, 3.2, 6.4, 12.8, 25.6, 51.2, 102.4, 204.8}

// as112Metrics holds the plugin's counters/gauges. Label sets are bounded
// (the fixed 22-zone set, the fixed qtype/rcode enums, the two protocols) so
// cardinality stays flat; never label by client IP or queried hostname.
type as112Metrics struct {
	requestTotal  metrics.CounterVec // labels: zone, qtype
	responseTotal metrics.CounterVec // labels: zone, qtype, rcode
	latency       metrics.Histogram
	listenerUp    metrics.GaugeVec   // labels: protocol, address
	reloadTotal   metrics.CounterVec // labels: result
	deniedTotal   metrics.CounterVec // labels: reason
}

var as112MetricsPtr atomic.Pointer[as112Metrics]

// buildMetrics registers the metric set against reg.
func buildMetrics(reg metrics.Registry) *as112Metrics {
	return &as112Metrics{
		requestTotal:  reg.CounterVec("ze_as112_dns_request_total", "DNS requests received, by zone and query type.", []string{"zone", "qtype"}),
		responseTotal: reg.CounterVec("ze_as112_dns_response_total", "DNS responses sent, by zone, query type and response code.", []string{"zone", "qtype", "rcode"}),
		latency:       reg.Histogram("ze_as112_dns_request_latency_milliseconds", "Time to handle a DNS request, in milliseconds.", latencyBuckets),
		listenerUp:    reg.GaugeVec("ze_as112_listener_up", "1 while a listener socket is bound, by protocol and listen address.", []string{"protocol", "address"}),
		reloadTotal:   reg.CounterVec("ze_as112_config_reload_total", "Config generations applied by the engine, by result.", []string{"result"}),
		deniedTotal:   reg.CounterVec("ze_as112_dns_denied_total", "Queries dropped because the client source was not in allow-from, by reason.", []string{"reason"}),
	}
}

// setMetricsRegistry publishes metrics backed by the host registry. Called
// via the plugin Registration's ConfigureMetrics before RunEngine.
func setMetricsRegistry(reg metrics.Registry) { as112MetricsPtr.Store(buildMetrics(reg)) }

// ametrics returns the current metric set, lazily defaulting to a no-op
// registry so increments are safe before ConfigureMetrics has run.
func ametrics() *as112Metrics {
	if m := as112MetricsPtr.Load(); m != nil {
		return m
	}
	as112MetricsPtr.CompareAndSwap(nil, buildMetrics(metrics.NopRegistry{}))
	return as112MetricsPtr.Load()
}

// qtypeLabel maps a query type to a bounded label, collapsing anything
// outside the served set to OTHER so cardinality cannot be driven by
// arbitrary qtypes.
func qtypeLabel(qtype uint16) string {
	switch qtype {
	case dns.TypeSOA, dns.TypeNS, dns.TypeTXT, dns.TypeANY:
		return dns.TypeToString[qtype]
	default:
		return "OTHER"
	}
}
