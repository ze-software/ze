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

	"github.com/ze-software/ze/internal/core/metrics"
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
		"ze_ospf_opaque_lsas",             // ospf-ext-1
		"ze_ospf_opaque_originations_total",
		"ze_ospf_opaque_received_total",
		"ze_ospf_opaque_consumer_errors_total",
		"ze_ospf_opaque_capable_neighbors",
		"ze_ospf_ext_prefix_lsas", // ospf-ext-4
		"ze_ospf_ext_link_lsas",
		"ze_ospf_ext_originations_total",
		"ze_ospf_ext_malformed_total",
		"ze_ospf_ext_subtlv_errors_total",
		"ze_ospf_ldp_sync_state", // ospf-ext-11 (LDP-IGP sync)
	} {
		assert.Containsf(t, rec.names, want, "canonical series %q must be registered", want)
	}
}

// TestOSPFBFDMetrics checks the three BFD series (spec-ospf-ext-10) register under the unified
// ze_ospf_ namespace, never a ze_ospfv3_bfd_* fork (learned 970).
func TestOSPFBFDMetrics(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	rec := &recordingRegistry{}
	eng.setBFDMetrics(rec)

	for _, want := range []string{
		"ze_ospf_bfd_sessions",
		"ze_ospf_bfd_session_down_total",
		"ze_ospf_bfd_register_failures_total",
	} {
		assert.Containsf(t, rec.names, want, "BFD series %q must be registered", want)
	}
	for _, n := range rec.names {
		assert.Truef(t, strings.HasPrefix(n, "ze_ospf_"), "metric %q must be ze_ospf_ namespaced", n)
		assert.Falsef(t, strings.HasPrefix(n, "ze_ospfv3_"), "metric %q must not fork a ze_ospfv3_ namespace (learned 970)", n)
	}
}
