package redistributeegress

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	configredist "codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingCounter struct {
	value atomic.Int64
}

func (r *recordingCounter) Inc()          { r.value.Add(1) }
func (r *recordingCounter) Add(v float64) { r.value.Add(int64(v)) }
func (r *recordingCounter) Get() int64    { return r.value.Load() }

type recordingRegistry struct {
	mu       sync.Mutex
	counters map[string]*recordingCounter
}

func newRecordingRegistry() *recordingRegistry {
	return &recordingRegistry{counters: map[string]*recordingCounter{}}
}

func (r *recordingRegistry) Counter(name, _ string) metrics.Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &recordingCounter{}
	r.counters[name] = c
	return c
}

func (r *recordingRegistry) Gauge(string, string) metrics.Gauge { return nopGauge{} }
func (r *recordingRegistry) CounterVec(string, string, []string) metrics.CounterVec {
	return nopCounterVec{}
}
func (r *recordingRegistry) GaugeVec(string, string, []string) metrics.GaugeVec { return nopGaugeVec{} }
func (r *recordingRegistry) Histogram(string, string, []float64) metrics.Histogram {
	return nopHistogram{}
}
func (r *recordingRegistry) HistogramVec(string, string, []float64, []string) metrics.HistogramVec {
	return nopHistogramVec{}
}

func (r *recordingRegistry) value(name string) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c.Get()
	}
	return 0
}

type nopGauge struct{}

func (nopGauge) Set(float64) {}
func (nopGauge) Inc()        {}
func (nopGauge) Dec()        {}
func (nopGauge) Add(float64) {}

type nopCounterVec struct{}

func (nopCounterVec) With(...string) metrics.Counter { return nopCounter{} }
func (nopCounterVec) Delete(...string) bool          { return false }

type nopGaugeVec struct{}

func (nopGaugeVec) With(...string) metrics.Gauge { return nopGauge{} }
func (nopGaugeVec) Delete(...string) bool        { return false }

type nopCounter struct{}

func (nopCounter) Inc()        {}
func (nopCounter) Add(float64) {}

type nopHistogram struct{}

func (nopHistogram) Observe(float64) {}

type nopHistogramVec struct{}

func (nopHistogramVec) With(...string) metrics.Histogram { return nopHistogram{} }
func (nopHistogramVec) Delete(...string) bool            { return false }

func resetMetricsState(t *testing.T) {
	t.Helper()
	redistevents.ResetForTest()
	configredist.SetGlobal(nil)
	metricsPtr.Store(nil)
	t.Cleanup(func() {
		redistevents.ResetForTest()
		configredist.SetGlobal(nil)
		metricsPtr.Store(nil)
	})
}

// VALIDATES: AC-14 -- counters increment at the documented cadences for a known
// event sequence (3 accepted adds, 1 accepted remove, 1 rule-rejected entry,
// 1 BGP-protocol-skipped batch).
// PREVENTS: Counter drift from reality.
func TestMetricsCadence(t *testing.T) {
	resetMetricsState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	bgpID := redistevents.RegisterProtocol("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	rec := newRecordingRegistry()
	setMetricsRegistry(rec)

	disp := &fakeDispatcher{}

	// 3 accepted adds.
	for _, p := range []string{"10.0.0.1/32", "10.0.0.2/32", "10.0.0.3/32"} {
		handleBatch(context.Background(), disp, bgpID, addBatch(id, afiIPv4, p, ""))
	}
	// 1 accepted remove.
	handleBatch(context.Background(), disp, bgpID, removeBatch(id, afiIPv4, "10.0.0.1/32"))
	// 1 batch rejected by rule (ipv6/unicast not allowed).
	handleBatch(context.Background(), disp, bgpID, addBatch(id, afiIPv6, "2001:db8::1/128", ""))
	// 1 batch with BGP protocol -- should be skipped at handler entry.
	handleBatch(context.Background(), disp, bgpID, addBatch(bgpID, afiIPv4, "10.0.0.99/32", ""))

	assert.Equal(t, int64(6), rec.value("ze_bgp_redistribute_events_received"), "5 accepted batches + 1 skipped = 6 received")
	assert.Equal(t, int64(3), rec.value("ze_bgp_redistribute_announcements"))
	assert.Equal(t, int64(1), rec.value("ze_bgp_redistribute_withdrawals"))
	assert.Equal(t, int64(1), rec.value("ze_bgp_redistribute_filtered_protocol_total"))
	assert.Equal(t, int64(1), rec.value("ze_bgp_redistribute_filtered_rule_total"), "ipv6/unicast batch had 1 entry")

	assert.Len(t, disp.snapshot(), 4)
}
