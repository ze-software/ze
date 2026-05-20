package iface

import (
	"math"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type stubBackend struct {
	Backend
	interfaces []InterfaceInfo
	err        error
}

func (s *stubBackend) ListInterfaces() ([]InterfaceInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.interfaces, nil
}

func withStubBackend(t *testing.T, ifs []InterfaceInfo) (*stubBackend, func()) {
	t.Helper()
	stub := &stubBackend{interfaces: ifs}
	backendsMu.Lock()
	prev := activeBackend
	activeBackend = stub
	backendsMu.Unlock()
	return stub, func() {
		backendsMu.Lock()
		activeBackend = prev
		backendsMu.Unlock()
	}
}

func TestRateTracker_ComputesDelta(t *testing.T) {
	stub, cleanup := withStubBackend(t, []InterfaceInfo{
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 1000, TxBytes: 2000, RxPackets: 10, TxPackets: 20}},
	})
	defer cleanup()

	tracker := newRateTracker()
	tracker.collect()

	backendsMu.Lock()
	stub.interfaces = []InterfaceInfo{
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 2000, TxBytes: 4000, RxPackets: 20, TxPackets: 40}},
	}
	backendsMu.Unlock()

	tracker.prevAt = time.Now().Add(-time.Second)
	tracker.collect()

	rate, ok := tracker.get("eth0")
	if !ok {
		t.Fatal("expected rate for eth0")
	}
	if rate.RxBps < 900 || rate.RxBps > 1100 {
		t.Errorf("RxBps = %f, want ~1000", rate.RxBps)
	}
	if rate.TxBps < 1900 || rate.TxBps > 2100 {
		t.Errorf("TxBps = %f, want ~2000", rate.TxBps)
	}
	if rate.RxPps < 9 || rate.RxPps > 11 {
		t.Errorf("RxPps = %f, want ~10", rate.RxPps)
	}
	if rate.TxPps < 19 || rate.TxPps > 21 {
		t.Errorf("TxPps = %f, want ~20", rate.TxPps)
	}
}

func TestRateTracker_WrapReturnsZero(t *testing.T) {
	stub, cleanup := withStubBackend(t, []InterfaceInfo{
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 5000, TxBytes: 5000, RxPackets: 50, TxPackets: 50}},
	})
	defer cleanup()

	tracker := newRateTracker()
	tracker.collect()

	backendsMu.Lock()
	stub.interfaces = []InterfaceInfo{
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 100, TxBytes: 100, RxPackets: 1, TxPackets: 1}},
	}
	backendsMu.Unlock()

	tracker.prevAt = time.Now().Add(-time.Second)
	tracker.collect()

	rate, ok := tracker.get("eth0")
	if !ok {
		t.Fatal("expected rate for eth0")
	}
	if rate.RxBps != 0 {
		t.Errorf("RxBps = %f, want 0 (counter wrap)", rate.RxBps)
	}
	if rate.TxBps != 0 {
		t.Errorf("TxBps = %f, want 0 (counter wrap)", rate.TxBps)
	}
	if rate.RxPps != 0 {
		t.Errorf("RxPps = %f, want 0 (counter wrap)", rate.RxPps)
	}
	if rate.TxPps != 0 {
		t.Errorf("TxPps = %f, want 0 (counter wrap)", rate.TxPps)
	}
}

func TestRateTracker_NewInterfaceZeroRate(t *testing.T) {
	_, cleanup := withStubBackend(t, []InterfaceInfo{
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 1000, TxBytes: 2000}},
	})
	defer cleanup()

	tracker := newRateTracker()
	tracker.collect()

	rate, ok := tracker.get("eth0")
	if !ok {
		t.Fatal("expected rate for eth0")
	}
	if rate.RxBps != 0 || rate.TxBps != 0 || rate.RxPps != 0 || rate.TxPps != 0 {
		t.Errorf("first-seen interface should have zero rates, got rx=%f tx=%f", rate.RxBps, rate.TxBps)
	}
}

func TestRateTracker_StaleCleanup(t *testing.T) {
	stub, cleanup := withStubBackend(t, []InterfaceInfo{
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 100}},
		{Name: "eth1", Stats: &InterfaceStats{RxBytes: 200}},
	})
	defer cleanup()

	tracker := newRateTracker()
	tracker.collect()

	backendsMu.Lock()
	stub.interfaces = []InterfaceInfo{
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 200}},
	}
	backendsMu.Unlock()

	tracker.prevAt = time.Now().Add(-time.Second)
	tracker.collect()

	if _, ok := tracker.get("eth1"); ok {
		t.Error("eth1 should have been cleaned up after disappearing")
	}
	if _, ok := tracker.get("eth0"); !ok {
		t.Error("eth0 should still be present")
	}
}

func TestRateTracker_NilMetrics(t *testing.T) {
	_, cleanup := withStubBackend(t, []InterfaceInfo{
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 100}},
	})
	defer cleanup()

	old := ifaceMetricsPtr.Load()
	ifaceMetricsPtr.Store(nil)
	defer ifaceMetricsPtr.Store(old)

	tracker := newRateTracker()
	tracker.collect()

	rate, ok := tracker.get("eth0")
	if !ok {
		t.Fatal("expected rate for eth0 even without metrics registry")
	}
	if rate.Name != "eth0" {
		t.Errorf("name = %q, want eth0", rate.Name)
	}
}

type fakeGauge struct{ value float64 }

func (g *fakeGauge) Set(v float64) { g.value = v }
func (g *fakeGauge) Inc()          {}
func (g *fakeGauge) Dec()          {}
func (g *fakeGauge) Add(float64)   {}

type fakeGaugeVec struct {
	gauges map[string]*fakeGauge
}

func newFakeGaugeVec() *fakeGaugeVec {
	return &fakeGaugeVec{gauges: make(map[string]*fakeGauge)}
}

func (v *fakeGaugeVec) With(labels ...string) metrics.Gauge {
	key := labels[0]
	if _, ok := v.gauges[key]; !ok {
		v.gauges[key] = &fakeGauge{}
	}
	return v.gauges[key]
}

func (v *fakeGaugeVec) Delete(labels ...string) bool {
	delete(v.gauges, labels[0])
	return true
}

func TestIfaceMetrics_BindRegistry(t *testing.T) {
	var registered int
	reg := &fakeRegistry{onGaugeVec: func() { registered++ }}
	bindMetricsRegistry(reg)
	defer ifaceMetricsPtr.Store(nil)

	if registered != 12 {
		t.Errorf("registered %d gauges, want 12", registered)
	}

	m := ifaceMetricsPtr.Load()
	if m == nil {
		t.Fatal("metrics pointer is nil after bind")
	}
}

type fakeRegistry struct {
	onGaugeVec func()
}

func (r *fakeRegistry) Counter(string, string) metrics.Counter                 { return nil }
func (r *fakeRegistry) Gauge(string, string) metrics.Gauge                     { return nil }
func (r *fakeRegistry) CounterVec(string, string, []string) metrics.CounterVec { return nil }
func (r *fakeRegistry) GaugeVec(string, string, []string) metrics.GaugeVec {
	if r.onGaugeVec != nil {
		r.onGaugeVec()
	}
	return newFakeGaugeVec()
}
func (r *fakeRegistry) Histogram(string, string, []float64) metrics.Histogram { return nil }
func (r *fakeRegistry) HistogramVec(string, string, []float64, []string) metrics.HistogramVec {
	return nil
}

func TestListRates_ReturnsData(t *testing.T) {
	_, cleanup := withStubBackend(t, []InterfaceInfo{
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 100}},
	})
	defer cleanup()

	tracker := newRateTracker()
	globalTracker.Store(tracker)
	defer globalTracker.Store(nil)

	tracker.collect()

	rates := ListRates()
	if rates == nil {
		t.Fatal("ListRates returned nil")
	}
	if _, ok := rates["eth0"]; !ok {
		t.Error("expected eth0 in rates")
	}
}

func TestGetRate_SingleInterface(t *testing.T) {
	_, cleanup := withStubBackend(t, []InterfaceInfo{
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 100}},
		{Name: "eth1", Stats: &InterfaceStats{RxBytes: 200}},
	})
	defer cleanup()

	tracker := newRateTracker()
	globalTracker.Store(tracker)
	defer globalTracker.Store(nil)

	tracker.collect()

	rate, ok := GetRate("eth0")
	if !ok {
		t.Fatal("expected rate for eth0")
	}
	if rate.Name != "eth0" {
		t.Errorf("name = %q, want eth0", rate.Name)
	}
}

func TestGetRate_NotFound(t *testing.T) {
	_, cleanup := withStubBackend(t, []InterfaceInfo{
		{Name: "eth0", Stats: &InterfaceStats{RxBytes: 100}},
	})
	defer cleanup()

	tracker := newRateTracker()
	globalTracker.Store(tracker)
	defer globalTracker.Store(nil)

	tracker.collect()

	_, ok := GetRate("nonexistent")
	if ok {
		t.Error("expected false for nonexistent interface")
	}
}

func TestRateDelta(t *testing.T) {
	tests := []struct {
		name    string
		cur     uint64
		prev    uint64
		elapsed float64
		want    float64
	}{
		{"normal", 2000, 1000, 1.0, 1000.0},
		{"wrap", 100, 5000, 1.0, 0.0},
		{"same", 1000, 1000, 1.0, 0.0},
		{"half second", 2000, 1000, 0.5, 2000.0},
		{"max uint64", math.MaxUint64, math.MaxUint64 - 1000, 1.0, 1000.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rateDelta(tt.cur, tt.prev, tt.elapsed)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("rateDelta(%d, %d, %f) = %f, want %f", tt.cur, tt.prev, tt.elapsed, got, tt.want)
			}
		})
	}
}

func TestRateTracker_StartStop(t *testing.T) {
	_, cleanup := withStubBackend(t, nil)
	defer cleanup()

	tracker := newRateTracker()
	tracker.Start()

	var stopped atomic.Bool
	go func() {
		tracker.Stop()
		stopped.Store(true)
	}()

	time.Sleep(50 * time.Millisecond)
	if !stopped.Load() {
		t.Error("tracker goroutine did not exit after Stop()")
	}
}
