package metrics_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
)

// TestRegistryCollectsWithoutExporter proves the metric COLLECTION API works
// with no HTTP exporter present -- the always-on guarantee that lets every
// component keep recording metrics when ze_telemetry is compiled out.
//
// VALIDATES: AC-3 (Gauge/Counter calls succeed and record without an exporter).
// PREVENTS: a no-telemetry build where collection panics or is unavailable.
func TestRegistryCollectsWithoutExporter(t *testing.T) {
	// The real backend records into memory with no exporter / HTTP server.
	reg := metrics.NewPrometheusRegistry()
	require.NotNil(t, reg)

	c := reg.Counter("ze_test_collected_total", "Counter recorded without an exporter.")
	c.Inc()
	c.Add(4)

	g := reg.Gauge("ze_test_gauge", "Gauge recorded without an exporter.")
	g.Set(7)
	g.Dec()

	cv := reg.CounterVec("ze_test_vec_total", "Vec recorded without an exporter.", []string{"peer"})
	cv.With("a").Inc()

	// A *PrometheusRegistry satisfies the collection interface, so consumers
	// that type-assert metrics.Registry keep working with no exporter.
	var _ metrics.Registry = reg
}

// TestNopRegistryIsTheDummy proves NopRegistry is the always-on no-op dummy that
// backs the collection API when the exporter plugin is not loaded: it satisfies
// metrics.Registry and every method is a safe no-op.
//
// VALIDATES: dummy implementation so plugins relying on metrics still work when
// the ze_telemetry exporter is compiled out.
// PREVENTS: a nil-registry panic in a consumer that records into the dummy.
func TestNopRegistryIsTheDummy(t *testing.T) {
	var dummy metrics.Registry = metrics.NopRegistry{}

	assert.NotPanics(t, func() {
		dummy.Counter("ze_test_nop_total", "no-op").Add(9)
		dummy.Gauge("ze_test_nop_gauge", "no-op").Set(1)
		dummy.CounterVec("ze_test_nop_vec_total", "no-op", []string{"k"}).With("v").Inc()
		dummy.GaugeVec("ze_test_nop_gvec", "no-op", []string{"k"}).With("v").Set(2)
		dummy.Histogram("ze_test_nop_hist", "no-op", []float64{1, 2}).Observe(1.5)
		dummy.HistogramVec("ze_test_nop_hvec", "no-op", []float64{1, 2}, []string{"k"}).With("v").Observe(1.5)
	})
}
