package system

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/metrics"
)

// metricValues is a metrics.Registry that keeps each vec's last value by label.
type metricValues struct {
	gauges   map[string]*valueGauge
	counters map[string]*valueGauge
}

type valueGauge struct{ value float64 }

func (g *valueGauge) Set(v float64) { g.value = v }
func (g *valueGauge) Inc()          { g.value++ }
func (g *valueGauge) Dec()          { g.value-- }
func (g *valueGauge) Add(v float64) { g.value += v }

type valueVec struct{ values map[string]*valueGauge }

func (v *valueVec) get(label string) *valueGauge {
	g, held := v.values[label]
	if !held {
		g = &valueGauge{}
		v.values[label] = g
	}
	return g
}
func (v *valueVec) Delete(labels ...string) bool { delete(v.values, labels[0]); return true }

type gaugeVec struct{ *valueVec }

func (v gaugeVec) With(labels ...string) metrics.Gauge { return v.get(labels[0]) }

type counterVec struct{ *valueVec }

func (v counterVec) With(labels ...string) metrics.Counter { return v.get(labels[0]) }

func (m *metricValues) Counter(string, string) metrics.Counter { return &valueGauge{} }
func (m *metricValues) Gauge(string, string) metrics.Gauge     { return &valueGauge{} }
func (m *metricValues) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	m.counters = map[string]*valueGauge{}
	return counterVec{&valueVec{values: m.counters}}
}
func (m *metricValues) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	m.gauges = map[string]*valueGauge{}
	return gaugeVec{&valueVec{values: m.gauges}}
}
func (m *metricValues) Histogram(string, string, []float64) metrics.Histogram { return nil }
func (m *metricValues) HistogramVec(string, string, []float64, []string) metrics.HistogramVec {
	return nil
}

// metricsTestOrigin is the lease origin the metrics test learns under.
var metricsTestOrigin = RegisterResolverOrigin("metrics-test", 951)

// TestResolverMetricsCountRecordsAndChanges proves the registry's two
// dns-source metrics: ze_dns_source_records{origin} follows the records each
// origin holds, back to zero when its lease is withdrawn, and
// ze_dns_source_selection_changes_total{consumer} counts a changed
// publication after the first and never an unchanged one. Method: a
// registry over a recording metrics registry, one lease set and withdrawn,
// and three publications.
func TestResolverMetricsCountRecordsAndChanges(t *testing.T) {
	r := &Resolvers{}
	values := &metricValues{}
	r.SetMetrics(values)

	source := metricsTestOrigin.Source("wan0", "0")
	if err := r.SetLease(source, []string{"192.0.2.53"}, UntilWithdrawn); err != nil {
		t.Fatalf("set lease: %v", err)
	}
	if got := values.gauges["metrics-test"]; got == nil || got.value != 1 {
		t.Fatalf("records gauge after the lease = %+v, want 1", got)
	}
	if err := r.SetLease(source, nil, UntilWithdrawn); err != nil {
		t.Fatalf("withdraw lease: %v", err)
	}
	if got := values.gauges["metrics-test"]; got == nil || got.value != 0 {
		t.Fatalf("records gauge after the withdrawal = %+v, want 0", got)
	}

	first := &dnssource.Publication{Consumer: "metrics-consumer", Selections: []dnssource.ConsumerSelection{{Path: "p"}}}
	changed := &dnssource.Publication{Consumer: "metrics-consumer", Selections: []dnssource.ConsumerSelection{
		{Path: "p", Servers: []netip.Addr{netip.MustParseAddr("192.0.2.1")}},
	}}
	for _, publication := range []*dnssource.Publication{first, first, changed} {
		if err := r.SetPublication(publication); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	if got := values.counters["metrics-consumer"]; got == nil || got.value != 1 {
		t.Fatalf("selection changes = %+v, want 1 (the first publication and a repeat are no change)", got)
	}
}
