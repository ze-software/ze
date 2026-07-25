package geodns

import (
	"net"
	"strconv"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

// recordingRegistry is a metrics.Registry that sums operations per metric name,
// so a test can assert that a metric was actually updated. Values aggregate
// across label sets (enough to prove wiring).
type recordingRegistry struct {
	mu  sync.Mutex
	val map[string]float64
}

func newRecordingRegistry() *recordingRegistry { return &recordingRegistry{val: map[string]float64{}} }

func (r *recordingRegistry) add(name string, v float64) {
	r.mu.Lock()
	r.val[name] += v
	r.mu.Unlock()
}

func (r *recordingRegistry) set(name string, v float64) {
	r.mu.Lock()
	r.val[name] = v
	r.mu.Unlock()
}

func (r *recordingRegistry) get(name string) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.val[name]
}

func (r *recordingRegistry) Counter(name, _ string) metrics.Counter { return &recMetric{r, name} }
func (r *recordingRegistry) Gauge(name, _ string) metrics.Gauge     { return &recMetric{r, name} }
func (r *recordingRegistry) Histogram(name, _ string, _ []float64) metrics.Histogram {
	return &recMetric{r, name}
}

func (r *recordingRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	return &recCounterVec{r, name}
}
func (r *recordingRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	return &recGaugeVec{r, name}
}
func (r *recordingRegistry) HistogramVec(name, _ string, _ []float64, _ []string) metrics.HistogramVec {
	return &recHistogramVec{r, name}
}

type recMetric struct {
	r    *recordingRegistry
	name string
}

func (m *recMetric) Inc()            { m.r.add(m.name, 1) }
func (m *recMetric) Add(v float64)   { m.r.add(m.name, v) }
func (m *recMetric) Dec()            { m.r.add(m.name, -1) }
func (m *recMetric) Set(v float64)   { m.r.set(m.name, v) }
func (m *recMetric) Observe(float64) { m.r.add(m.name, 1) }

type recCounterVec struct {
	r    *recordingRegistry
	name string
}

func (v *recCounterVec) With(...string) metrics.Counter { return &recMetric{v.r, v.name} }
func (v *recCounterVec) Delete(...string) bool          { return false }

type recGaugeVec struct {
	r    *recordingRegistry
	name string
}

func (v *recGaugeVec) With(...string) metrics.Gauge { return &recMetric{v.r, v.name} }
func (v *recGaugeVec) Delete(...string) bool        { return false }

type recHistogramVec struct {
	r    *recordingRegistry
	name string
}

func (v *recHistogramVec) With(...string) metrics.Histogram { return &recMetric{v.r, v.name} }
func (v *recHistogramVec) Delete(...string) bool            { return false }

// VALIDATES: a bound listener sets ze_geodns_listener_up, and a real DNS query
// increments ze_geodns_dns_request_total -- i.e. the metrics are wired into the
// host registry and updated on the live query path.
// PREVENTS: a metric that is defined but never incremented (silent observability
// gap), which the Nop-registry unit test cannot catch.
func TestQueryIncrementsMetrics(t *testing.T) {
	rec := newRecordingRegistry()
	setMetricsRegistry(rec)
	t.Cleanup(func() { setMetricsRegistry(metrics.NopRegistry{}) })

	port := freePort(t)
	cfg := resolveTestConfig(t, port)
	storeApplied(cfg, 1)
	mgr := newServerManager(testLogger())
	if err := mgr.apply(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	t.Cleanup(mgr.stopAll)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))

	if got := rec.get("ze_geodns_listener_up"); got < 1 {
		t.Errorf("ze_geodns_listener_up = %v, want >= 1 after bind", got)
	}

	// request_total is incremented before the reply is written, so it is set by
	// the time the client Exchange returns (no defer race).
	queryA(t, "udp", addr, "proxy.test.example.", "82.219.4.10")
	if got := rec.get("ze_geodns_dns_request_total"); got < 1 {
		t.Errorf("ze_geodns_dns_request_total = %v, want >= 1 after a query", got)
	}
}
