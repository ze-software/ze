package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// TestPrometheusRegistry_Counter verifies counter creation and increment.
//
// VALIDATES: PrometheusRegistry.Counter creates a working counter.
// PREVENTS: Counter not registered or not incrementing.
func TestPrometheusRegistry_Counter(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	c := reg.Counter("test_total", "A test counter.")
	c.Inc()
	c.Add(4)

	body := scrapeMetrics(t, reg)
	assert.Contains(t, body, "test_total 5")
}

// TestPrometheusRegistry_Gauge verifies gauge creation and operations.
//
// VALIDATES: PrometheusRegistry.Gauge creates a working gauge.
// PREVENTS: Gauge not registered or operations not working.
func TestPrometheusRegistry_Gauge(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	g := reg.Gauge("test_gauge", "A test gauge.")
	g.Set(10)
	g.Inc()
	g.Dec()
	g.Add(5)

	body := scrapeMetrics(t, reg)
	assert.Contains(t, body, "test_gauge 15")
}

// TestPrometheusRegistry_CounterVec verifies labeled counter creation.
//
// VALIDATES: PrometheusRegistry.CounterVec creates working labeled counters.
// PREVENTS: Label values not applied or counter not incrementing.
func TestPrometheusRegistry_CounterVec(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	cv := reg.CounterVec("requests_total", "Total requests.", []string{"method"})
	cv.With("GET").Add(3)
	cv.With("POST").Inc()

	body := scrapeMetrics(t, reg)
	assert.Contains(t, body, `requests_total{method="GET"} 3`)
	assert.Contains(t, body, `requests_total{method="POST"} 1`)
}

// TestPrometheusRegistry_GaugeVec verifies labeled gauge creation.
//
// VALIDATES: PrometheusRegistry.GaugeVec creates working labeled gauges.
// PREVENTS: Label values not applied or gauge operations not working.
func TestPrometheusRegistry_GaugeVec(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	gv := reg.GaugeVec("connections", "Active connections.", []string{"peer"})
	gv.With("10.0.0.1").Set(5)
	gv.With("10.0.0.2").Set(3)

	body := scrapeMetrics(t, reg)
	assert.Contains(t, body, `connections{peer="10.0.0.1"} 5`)
	assert.Contains(t, body, `connections{peer="10.0.0.2"} 3`)
}

// TestPrometheusRegistry_ImplementsRegistry verifies PrometheusRegistry satisfies the Registry interface.
//
// VALIDATES: PrometheusRegistry is a valid Registry implementation.
// PREVENTS: Interface mismatch at compile time.
func TestPrometheusRegistry_ImplementsRegistry(t *testing.T) {
	var _ metrics.Registry = &metrics.PrometheusRegistry{}
}

// TestPrometheusRegistry_Concurrent verifies concurrent metric operations have no race.
//
// VALIDATES: PrometheusRegistry is safe for concurrent use.
// PREVENTS: Data race on concurrent Inc/Set from multiple goroutines.
func TestPrometheusRegistry_Concurrent(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	c := reg.Counter("concurrent_total", "Concurrent test counter.")
	g := reg.Gauge("concurrent_gauge", "Concurrent test gauge.")
	cv := reg.CounterVec("concurrent_vec_total", "Concurrent labeled counter.", []string{"worker"})

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range 100 {
				c.Inc()
				g.Set(float64(id))
				g.Add(1)
				cv.With("worker").Inc()
			}
		}(i)
	}
	wg.Wait()

	body := scrapeMetrics(t, reg)
	assert.Contains(t, body, "concurrent_total 1000")
	assert.Contains(t, body, `concurrent_vec_total{worker="worker"} 1000`)
}

// TestPrometheusRegistry_Idempotent verifies that calling Counter/Gauge/Vec
// with the same name returns the same metric instance (no panic on re-register).
//
// VALIDATES: Registry methods are idempotent (get-or-create by name).
// PREVENTS: Panic on duplicate metric registration; stale references.
func TestPrometheusRegistry_Idempotent(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()

	// Counter: same name returns same instance, increments accumulate.
	c1 := reg.Counter("idem_total", "First call.")
	c1.Add(3)
	c2 := reg.Counter("idem_total", "Second call.")
	c2.Add(7)
	body := scrapeMetrics(t, reg)
	assert.Contains(t, body, "idem_total 10")

	// Gauge: same instance, last Set wins.
	g1 := reg.Gauge("idem_gauge", "First.")
	g1.Set(5)
	g2 := reg.Gauge("idem_gauge", "Second.")
	g2.Set(42)
	body = scrapeMetrics(t, reg)
	assert.Contains(t, body, "idem_gauge 42")

	// CounterVec: same instance, labels shared.
	cv1 := reg.CounterVec("idem_cv_total", "First.", []string{"k"})
	cv1.With("a").Inc()
	cv2 := reg.CounterVec("idem_cv_total", "Second.", []string{"k"})
	cv2.With("a").Add(4)
	body = scrapeMetrics(t, reg)
	assert.Contains(t, body, `idem_cv_total{k="a"} 5`)

	// GaugeVec: same instance, labels shared.
	gv1 := reg.GaugeVec("idem_gv", "First.", []string{"k"})
	gv1.With("x").Set(1)
	gv2 := reg.GaugeVec("idem_gv", "Second.", []string{"k"})
	gv2.With("x").Set(99)
	body = scrapeMetrics(t, reg)
	assert.Contains(t, body, `idem_gv{k="x"} 99`)
}

// scrapeMetrics returns the /metrics body from the registry's handler.
func scrapeMetrics(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", http.NoBody))
	require.Equal(t, 200, rec.Code)
	b, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	return strings.TrimSpace(string(b))
}
