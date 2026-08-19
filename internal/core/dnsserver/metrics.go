// Design: docs/architecture/dns/server-harness.md -- the harness owns the single
// wire write, so it owns the counter for a write that fails
// Related: handler.go -- send() calls recordWriteFailure

package dnsserver

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// The transport label values of writeFailures. Two constants and no third, so
// the label set stays bounded whatever a client sends: a datagram write and a
// stream write fail for different reasons, and an operator reads them apart.
const (
	transportDatagram = "datagram"
	transportStream   = "stream"
)

// harnessMetrics holds the metrics the harness itself owns.
//
// The listener gauge is NOT here, and Options.OnListenerChange keeps it with
// the consumer, because its label values are the consumer's listen addresses.
// A failed reply write is the opposite case: it happens inside send, where no
// consumer can see it, so the harness counts it.
type harnessMetrics struct {
	writeFailures metrics.CounterVec // labels: transport
}

// harnessMetricsPtr is swapped from a lazy Nop default to the host registry's
// metrics when a consumer calls SetMetricsRegistry, so the query path never
// nil-checks. Safe for concurrent use.
var harnessMetricsPtr atomic.Pointer[harnessMetrics]

// buildMetrics registers the harness metric set against reg.
func buildMetrics(reg metrics.Registry) *harnessMetrics {
	return &harnessMetrics{
		writeFailures: reg.CounterVec(
			"ze_dns_reply_write_failure_total",
			"DNS replies the transport refused to write, by transport.",
			[]string{"transport"},
		),
	}
}

// SetMetricsRegistry publishes the harness metrics against reg. Every consumer
// of this package calls it from its plugin registration's ConfigureMetrics, so
// a process serving DNS from more than one plugin registers the same metric
// twice. metrics.Registry is idempotent by name, so the two consumers share one
// counter rather than colliding. Safe for concurrent use.
func SetMetricsRegistry(reg metrics.Registry) { harnessMetricsPtr.Store(buildMetrics(reg)) }

// hmetrics returns the current metric set, lazily defaulting to a no-op
// registry so a reply written before ConfigureMetrics has run still counts
// against something.
func hmetrics() *harnessMetrics {
	if m := harnessMetricsPtr.Load(); m != nil {
		return m
	}
	harnessMetricsPtr.CompareAndSwap(nil, buildMetrics(metrics.NopRegistry{}))
	return harnessMetricsPtr.Load()
}

// recordWriteFailure counts one reply the transport refused to write.
func recordWriteFailure(transport string) {
	hmetrics().writeFailures.With(transport).Inc()
}
