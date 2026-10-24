// Related: metrics.go -- registerRadiusMetrics, bindRadiusMetrics, countUnsignedEAPDiscard
// Related: client_test.go -- the unsigned EAP answer exchange these helpers count

package radius

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

const unsignedEAPDiscardsSeries = "ze_radius_unsigned_eap_discards_total"

// bindTestRadiusMetrics binds the RADIUS counters to a fresh Prometheus
// registry for the duration of t, and restores whatever was bound before.
func bindTestRadiusMetrics(t *testing.T) *metrics.PrometheusRegistry {
	t.Helper()
	previous := radiusMetricsPtr.Load()
	t.Cleanup(func() { radiusMetricsPtr.Store(previous) })
	reg := metrics.NewPrometheusRegistry()
	bindRadiusMetrics(reg)
	return reg
}

// scrapeUnsignedEAPDiscards renders reg through the promhttp handler an
// operator's exporter serves and returns the series value, or -1 when the
// series is absent. Reading the rendered text proves an operator can see it.
func scrapeUnsignedEAPDiscards(t *testing.T, reg *metrics.PrometheusRegistry) float64 {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, request)
	for line := range strings.SplitSeq(recorder.Body.String(), "\n") {
		value, found := strings.CutPrefix(line, unsignedEAPDiscardsSeries+" ")
		if !found {
			continue
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			t.Fatalf("series %s carries %q: %v", unsignedEAPDiscardsSeries, value, err)
		}
		return parsed
	}
	return -1
}

// TestRadiusUnsignedEAPDiscardCounted proves the discard counter reaches the
// operator's /metrics endpoint in the deployed order: the hook is registered
// before any metrics registry exists (init runs before the hub's telemetry
// start), a discard then reaches nobody and must not panic, and once the
// registry arrives the series exists and counts.
//
// VALIDATES: AC-9, an unsigned EAP answer discard is a Prometheus series.
// PREVENTS: binding by reading registry.GetMetricsRegistry at init, which
// answers nil there and leaves the series absent for the process lifetime.
func TestRadiusUnsignedEAPDiscardCounted(t *testing.T) {
	previousMetrics := radiusMetricsPtr.Swap(nil)
	t.Cleanup(func() { radiusMetricsPtr.Store(previousMetrics) })
	previousRegistry := registry.GetMetricsRegistry()
	t.Cleanup(func() { registry.SetMetricsRegistry(previousRegistry) })
	registry.SetMetricsRegistry(nil)

	// init already handed the hook over with no registry in existence; this
	// call is the same request again and must stay deferred.
	registerRadiusMetrics()
	if radiusMetricsPtr.Load() != nil {
		t.Fatal("counters bound with no registry in existence")
	}
	countUnsignedEAPDiscard()

	reg := metrics.NewPrometheusRegistry()
	registry.SetMetricsRegistry(reg)
	if got := scrapeUnsignedEAPDiscards(t, reg); got != 0 {
		t.Fatalf("%s after the registry arrived = %v, want 0", unsignedEAPDiscardsSeries, got)
	}
	countUnsignedEAPDiscard()
	if got := scrapeUnsignedEAPDiscards(t, reg); got != 1 {
		t.Fatalf("%s after one discard = %v, want 1", unsignedEAPDiscardsSeries, got)
	}
}
