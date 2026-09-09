package pppoe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

// scrapeDiscoveryReadErrors renders reg through the same promhttp handler an
// operator's exporter serves, and returns the
// ze_pppoe_discovery_read_errors_total line, or the empty string when the
// series does not exist yet.
func scrapeDiscoveryReadErrors(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, request)

	const prefix = "ze_pppoe_discovery_read_errors_total "
	for line := range strings.SplitSeq(recorder.Body.String(), "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

// scrapeRefusals renders reg through the same promhttp handler an operator's
// exporter serves, and returns the ze_pppoe_discovery_refusals_total line for
// reason, or the empty string when the series does not exist. Reading the
// rendered text rather than the collector proves an operator can see the
// count, which is what AC-10 asks for.
func scrapeRefusals(t *testing.T, reg *metrics.PrometheusRegistry, reason string) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, request)

	prefix := `ze_pppoe_discovery_refusals_total{reason="` + reason + `"} `
	for line := range strings.SplitSeq(recorder.Body.String(), "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

// TestDiscoveryCounterBindsWhenTheRegistryArrivesLast -- AC-10's wiring. The
// counter has to reach an operator on a PPPoE-only daemon, and that daemon
// creates its Prometheus registry AFTER engine.Start has already run
// Subsystem.Start (startStandaloneTelemetry, cmd/ze/hub/main.go). So the order
// this test drives is the deployed one: register the hook with no registry in
// existence, count a refusal that must reach nobody and must not panic, then
// let the registry arrive and require the counter to exist and to count.
//
// VALIDATES: AC-10 reaches the operator's /metrics endpoint rather than only
// the unit test that hands the counter in directly.
// PREVENTS: registerDiscoveryMetrics going back to reading
// registry.GetMetricsRegistry, which answers nil at Subsystem.Start on this
// configuration and leaves ze_pppoe_discovery_refusals_total absent for the
// process lifetime with no log line saying so.
func TestDiscoveryCounterBindsWhenTheRegistryArrivesLast(t *testing.T) {
	previousMetrics := pppoeMetricsPtr.Swap(nil)
	t.Cleanup(func() { pppoeMetricsPtr.Store(previousMetrics) })

	previousRegistry := registry.GetMetricsRegistry()
	t.Cleanup(func() { registry.SetMetricsRegistry(previousRegistry) })
	registry.SetMetricsRegistry(nil)

	registerDiscoveryMetrics()

	if pppoeMetricsPtr.Load() != nil {
		t.Fatal("counters bound with no registry in existence")
	}
	countRefusal(reasonServiceNameMissing)

	reg := metrics.NewPrometheusRegistry()
	registry.SetMetricsRegistry(reg)

	if pppoeMetricsPtr.Load() == nil {
		t.Fatal("the discovery counters stayed unbound after the telemetry exporter created the registry")
	}

	countRefusal(reasonServiceNameMissing)

	want := `ze_pppoe_discovery_refusals_total{reason="service-name-missing"} 1`
	if got := scrapeRefusals(t, reg, reasonServiceNameMissing); got != want {
		t.Errorf("scraped %q, want %q", got, want)
	}
}
