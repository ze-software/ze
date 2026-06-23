// VALIDATES: spec-ospf-13 AC-12 -- every metric the OSPF engine registers is `ze_ospf_*`
// namespaced (never a bare `ospf_*`), and the canonical series owned by the producing
// specs (5/6/10/11/12) are present.
// PREVENTS: a metric escaping the ze_ospf_ namespace, or a canonical series going
// unregistered.
package ospf

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// recordingRegistry captures every registered metric name and otherwise behaves as a nop.
type recordingRegistry struct {
	metrics.NopRegistry
	names []string
}

func (r *recordingRegistry) Counter(name, help string) metrics.Counter {
	r.names = append(r.names, name)
	return r.NopRegistry.Counter(name, help)
}

func (r *recordingRegistry) Gauge(name, help string) metrics.Gauge {
	r.names = append(r.names, name)
	return r.NopRegistry.Gauge(name, help)
}

func (r *recordingRegistry) CounterVec(name, help string, labels []string) metrics.CounterVec {
	r.names = append(r.names, name)
	return r.NopRegistry.CounterVec(name, help, labels)
}

func (r *recordingRegistry) GaugeVec(name, help string, labels []string) metrics.GaugeVec {
	r.names = append(r.names, name)
	return r.NopRegistry.GaugeVec(name, help, labels)
}

func (r *recordingRegistry) HistogramVec(name, help string, buckets []float64, labels []string) metrics.HistogramVec {
	r.names = append(r.names, name)
	return r.NopRegistry.HistogramVec(name, help, buckets, labels)
}

func TestOSPFMetricsNamespaced(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	rec := &recordingRegistry{}
	eng.setMetrics(rec)

	assert.NotEmpty(t, rec.names, "setMetrics registers OSPF series")
	for _, n := range rec.names {
		assert.Truef(t, strings.HasPrefix(n, "ze_ospf_"), "metric %q must be ze_ospf_ namespaced, never bare ospf_ (AC-12)", n)
	}

	// Canonical series owned by ospf-5/6/10/11/12, registered through the engine registry.
	for _, want := range []string{
		"ze_ospf_interface_up",            // ospf-5
		"ze_ospf_neighbors",               // ospf-6
		"ze_ospf_adjacencies_full",        // ospf-6
		"ze_ospf_asbr",                    // ospf-10
		"ze_ospf_external_lsas",           // ospf-10
		"ze_ospf_nssa_translations_total", // ospf-11
		"ze_ospf_auth_failures_total",     // ospf-12
	} {
		assert.Containsf(t, rec.names, want, "canonical series %q must be registered", want)
	}
}
