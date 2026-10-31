package redistribute

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"

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
func (r *recordingRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	return recordingCounterVec{registry: r, name: name}
}

// recordingCounterVec records each label set as its own counter, keyed
// name{label,label} in the registry.
type recordingCounterVec struct {
	registry *recordingRegistry
	name     string
}

func (v recordingCounterVec) With(labels ...string) metrics.Counter {
	return v.registry.Counter(v.name+"{"+strings.Join(labels, ",")+"}", "")
}
func (recordingCounterVec) Delete(...string) bool                               { return false }
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
	configredist.ResetConsumersForTest()
	metricsPtr.Store(nil)
	t.Cleanup(func() {
		redistevents.ResetForTest()
		configredist.SetGlobal(nil)
		configredist.ResetConsumersForTest()
		metricsPtr.Store(nil)
	})
}

// VALIDATES: AC-14 -- counters increment at the documented cadences for a known
// event sequence (3 accepted adds, 1 accepted remove, 1 chain-rejected entry,
// 1 consumer-protocol-skipped batch, 1 entry no rule imports).
// PREVENTS: Counter drift from reality.
func TestMetricsCadence(t *testing.T) {
	resetMetricsState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	bgpID := redistevents.RegisterProtocol("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Final: "accept", Chain: familyChain{family.IPv4Unicast}},
	}))

	consumer := &stubConsumer{name: "bgp"}
	require.NoError(t, configredist.RegisterConsumer(consumer))

	rec := newRecordingRegistry()
	setMetricsRegistry(rec)

	skip := skipIDs(bgpID)

	// 3 accepted adds.
	for _, p := range []string{"10.0.0.1/32", "10.0.0.2/32", "10.0.0.3/32"} {
		handleBatch(context.Background(), skip, addBatch(id, afiIPv4, p, ""))
	}
	// 1 accepted remove.
	handleBatch(context.Background(), skip, removeBatch(id, afiIPv4, "10.0.0.1/32"))
	// 1 batch rejected by rule (ipv6/unicast not allowed).
	handleBatch(context.Background(), skip, addBatch(id, afiIPv6, "2001:db8::1/128", ""))
	// 1 batch with BGP protocol -- should be skipped at handler entry.
	handleBatch(context.Background(), skip, addBatch(bgpID, afiIPv4, "10.0.0.99/32", ""))
	// 1 batch from a source no rule imports: rejected, and no chain's verdict.
	otherID := redistevents.RegisterProtocol("otherredist")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "otherredist", Protocol: "otherredist"}))
	handleBatch(context.Background(), skip, addBatch(otherID, afiIPv4, "10.0.0.98/32", ""))

	assert.Equal(t, int64(7), rec.value("ze_bgp_redistribute_events_received"), "6 offered batches + 1 skipped = 7 received")
	assert.Equal(t, int64(3), rec.value("ze_bgp_redistribute_announcements"))
	assert.Equal(t, int64(1), rec.value("ze_bgp_redistribute_withdrawals"))
	assert.Equal(t, int64(1), rec.value("ze_bgp_redistribute_filtered_protocol_total"))
	assert.Equal(t, int64(1), rec.value("ze_redistribute_chain_rejected_total{bgp,fakeredist}"), "the chain rejected the 1 ipv6/unicast entry")
	assert.Equal(t, int64(0), rec.value("ze_redistribute_chain_rejected_total{bgp,otherredist}"), "no rule imports otherredist, so no chain rejected it")

	assert.Len(t, consumer.snapshotInjected(), 3)
	assert.Len(t, consumer.snapshotWithdrawn(), 1)
}
