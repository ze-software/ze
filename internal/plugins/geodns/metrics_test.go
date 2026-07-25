package geodns

import (
	"testing"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/metrics"
)

// VALIDATES: query-type metric labels are bounded — known types map to their
// name, everything else collapses to OTHER.
// PREVENTS: an attacker flooding arbitrary qtypes from exploding Prometheus
// label cardinality.
func TestQtypeLabelBounded(t *testing.T) {
	t.Parallel()
	cases := map[uint16]string{
		dns.TypeA:    "A",
		dns.TypeAAAA: "AAAA",
		dns.TypeSRV:  "SRV",
		dns.TypeSOA:  "SOA",
		dns.TypeNS:   "NS",
		dns.TypeANY:  "ANY",
		dns.TypeMX:   "OTHER",
		dns.TypeTXT:  "OTHER",
	}
	for qt, want := range cases {
		if got := qtypeLabel(qt); got != want {
			t.Errorf("qtypeLabel(%d) = %q, want %q", qt, got, want)
		}
	}
}

// VALIDATES: buildMetrics wires every metric against a registry and the
// resulting counters/gauges/histogram accept operations without panicking.
// PREVENTS: a label-arity mismatch crashing the daemon on the first query.
func TestBuildMetricsIncrements(t *testing.T) {
	t.Parallel()
	m := buildMetrics(metrics.NopRegistry{})
	m.requestTotal.With("geodns.example.", "A").Inc()
	m.responseTotal.With("geodns.example.", "A", "NOERROR").Inc()
	m.latency.Observe(1.5)
	m.listenerUp.With("udp", "127.0.0.1").Set(1)
	m.reloadTotal.With("success").Inc()
}
