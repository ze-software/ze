// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- transport metric series + per-instance snapshot tests

package transport

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

type metricRecord struct {
	name   string
	labels []string
}

// countingCounter records how many times Inc/Add were called, so a test can
// assert a per-instance increment actually reached the Prometheus series.
type countingCounter struct{ n *int64 }

func (c countingCounter) Inc()          { atomic.AddInt64(c.n, 1) }
func (c countingCounter) Add(v float64) { atomic.AddInt64(c.n, int64(v)) }

type countingGauge struct{ v *int64 }

func (g countingGauge) Set(v float64) { atomic.StoreInt64(g.v, int64(v)) }
func (countingGauge) Inc()            {}
func (countingGauge) Dec()            {}
func (countingGauge) Add(float64)     {}

// countingCounterVec sums increments across all label sets for one series.
type countingCounterVec struct {
	mu sync.Mutex
	n  map[string]*int64 // key = joined label values
}

func (v *countingCounterVec) With(labelValues ...string) metrics.Counter {
	key := ""
	for _, lv := range labelValues {
		key += lv + "|"
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.n == nil {
		v.n = make(map[string]*int64)
	}
	c, ok := v.n[key]
	if !ok {
		var z int64
		c = &z
		v.n[key] = c
	}
	return countingCounter{n: c}
}
func (*countingCounterVec) Delete(...string) bool { return false }

func (v *countingCounterVec) total() int64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	var t int64
	for _, c := range v.n {
		t += atomic.LoadInt64(c)
	}
	return t
}

type noopGaugeVec struct{}

func (noopGaugeVec) With(...string) metrics.Gauge { return countingGauge{v: new(int64)} }
func (noopGaugeVec) Delete(...string) bool        { return false }

type noopHistogram struct{}

func (noopHistogram) Observe(float64) {}

type noopHistogramVec struct{}

func (noopHistogramVec) With(...string) metrics.Histogram { return noopHistogram{} }
func (noopHistogramVec) Delete(...string) bool            { return false }

// recordingRegistry records every series created (name + labels) and hands out
// counting counters so increments can be asserted.
type recordingRegistry struct {
	records []metricRecord
	vecs    map[string]*countingCounterVec
	gauge   *int64
}

func newRecordingRegistry() *recordingRegistry {
	return &recordingRegistry{vecs: make(map[string]*countingCounterVec), gauge: new(int64)}
}

func (r *recordingRegistry) Counter(name, _ string) metrics.Counter {
	r.records = append(r.records, metricRecord{name: name})
	return countingCounter{n: new(int64)}
}
func (r *recordingRegistry) Gauge(name, _ string) metrics.Gauge {
	r.records = append(r.records, metricRecord{name: name})
	return countingGauge{v: r.gauge}
}
func (r *recordingRegistry) CounterVec(name, _ string, labels []string) metrics.CounterVec {
	r.records = append(r.records, metricRecord{name: name, labels: append([]string(nil), labels...)})
	v := &countingCounterVec{}
	r.vecs[name] = v
	return v
}
func (r *recordingRegistry) GaugeVec(name, _ string, labels []string) metrics.GaugeVec {
	r.records = append(r.records, metricRecord{name: name, labels: append([]string(nil), labels...)})
	return noopGaugeVec{}
}
func (r *recordingRegistry) Histogram(name, _ string, _ []float64) metrics.Histogram {
	r.records = append(r.records, metricRecord{name: name})
	return noopHistogram{}
}
func (r *recordingRegistry) HistogramVec(name, _ string, _ []float64, labels []string) metrics.HistogramVec {
	r.records = append(r.records, metricRecord{name: name, labels: append([]string(nil), labels...)})
	return noopHistogramVec{}
}

func TestTransportMetricsRegistered(t *testing.T) {
	// VALIDATES: AC-12 -- the five ze_vrrp_* series register with exact names and
	// labels; NopRegistry backs the counters before SetMetrics.
	reg := newRecordingRegistry()
	_ = newTransportMetrics(reg)
	want := []metricRecord{
		{name: "ze_vrrp_adverts_sent_total", labels: []string{"interface", "vrid", "family"}},
		{name: "ze_vrrp_adverts_received_total", labels: []string{"interface", "vrid", "family"}},
		{name: "ze_vrrp_packet_errors_total", labels: []string{"interface", "vrid", "family", "reason"}},
		{name: "ze_vrrp_announcements_sent_total", labels: []string{"interface", "vrid", "family", "kind"}},
		{name: "ze_vrrp_sockets_open"},
	}
	if !reflect.DeepEqual(reg.records, want) {
		t.Fatalf("metric series = %#v, want %#v", reg.records, want)
	}
	// NopRegistry default must not panic.
	nm := nopTransportMetrics()
	nm.advertsSent.With("eth0", "10", "ipv4").Inc()
	nm.socketsOpen.Set(1)
}

func TestCounterSnapshotAndReset(t *testing.T) {
	// VALIDATES: AC-12 + Finding 7 -- CounterSnapshot reads back per-instance
	// increments (and reaches the Prometheus series); ResetCounters zeroes only
	// that instance's snapshot, leaving Prometheus counters untouched.
	reg := newRecordingRegistry()
	var mp atomic.Pointer[transportMetrics]
	mp.Store(newTransportMetrics(reg))

	c := newInstanceCounters(&mp, "eth0", 10, 4)
	c.advertSent()
	c.advertSent()
	c.advertReceived()
	c.announcement(kindGARP)
	c.announcement(kindGARP)
	c.announcement(kindGARP)
	c.packetError(reasonRxOverflow)
	c.packetError("checksum")

	snap := c.snapshot()
	if snap.AdvertsSent != 2 || snap.AdvertsReceived != 1 || snap.AnnouncementsGARP != 3 {
		t.Fatalf("snapshot counts wrong: %+v", snap)
	}
	if snap.PacketErrors[reasonRxOverflow] != 1 || snap.PacketErrors["checksum"] != 1 {
		t.Fatalf("snapshot packet errors wrong: %+v", snap.PacketErrors)
	}
	// The increments must have reached the Prometheus series.
	if got := reg.vecs["ze_vrrp_adverts_sent_total"].total(); got != 2 {
		t.Fatalf("prometheus adverts_sent = %d, want 2", got)
	}
	if got := reg.vecs["ze_vrrp_announcements_sent_total"].total(); got != 3 {
		t.Fatalf("prometheus announcements_sent = %d, want 3", got)
	}

	c.reset()
	snap = c.snapshot()
	if snap.AdvertsSent != 0 || snap.AdvertsReceived != 0 || snap.AnnouncementsGARP != 0 || len(snap.PacketErrors) != 0 {
		t.Fatalf("reset did not zero the snapshot: %+v", snap)
	}
	// Prometheus counters are monotonic and MUST NOT be reset by ResetCounters.
	if got := reg.vecs["ze_vrrp_adverts_sent_total"].total(); got != 2 {
		t.Fatalf("prometheus adverts_sent after reset = %d, want 2 (monotonic)", got)
	}
}
