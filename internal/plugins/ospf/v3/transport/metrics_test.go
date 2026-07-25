// VALIDATES: spec-ospfv3-3-ipv6-transport -- the OSPFv3 transport requests the
// SHARED ze_ospf_ transport series (not a distinct ze_ospfv3_ namespace), with
// the OSPFv2-matching names and label sets so the get-or-create registry shares
// one series across v2 and v3. PREVENTS a renamed/forked OSPFv3 metric namespace.

package transport

import (
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

type metricRecord struct {
	name   string
	labels []string
}

type recordingRegistry struct{ records []metricRecord }

type noopCounter struct{}

func (noopCounter) Inc()        {}
func (noopCounter) Add(float64) {}

type noopGauge struct{}

func (noopGauge) Set(float64) {}
func (noopGauge) Inc()        {}
func (noopGauge) Dec()        {}
func (noopGauge) Add(float64) {}

type noopCounterVec struct{}

func (noopCounterVec) With(...string) metrics.Counter { return noopCounter{} }
func (noopCounterVec) Delete(...string) bool          { return false }

type noopGaugeVec struct{}

func (noopGaugeVec) With(...string) metrics.Gauge { return noopGauge{} }
func (noopGaugeVec) Delete(...string) bool        { return false }

type noopHistogram struct{}

func (noopHistogram) Observe(float64) {}

type noopHistogramVec struct{}

func (noopHistogramVec) With(...string) metrics.Histogram { return noopHistogram{} }
func (noopHistogramVec) Delete(...string) bool            { return false }

func (r *recordingRegistry) Counter(name, _ string) metrics.Counter {
	r.records = append(r.records, metricRecord{name: name})
	return noopCounter{}
}
func (r *recordingRegistry) Gauge(name, _ string) metrics.Gauge {
	r.records = append(r.records, metricRecord{name: name})
	return noopGauge{}
}
func (r *recordingRegistry) CounterVec(name, _ string, labels []string) metrics.CounterVec {
	r.records = append(r.records, metricRecord{name: name, labels: append([]string(nil), labels...)})
	return noopCounterVec{}
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

func TestOSPFv3TransportMetricsSeries(t *testing.T) {
	reg := &recordingRegistry{}
	_ = newTransportMetrics(reg)
	want := []metricRecord{
		{name: "ze_ospf_packets_sent_total", labels: []string{"interface", "type"}},
		{name: "ze_ospf_packets_received_total", labels: []string{"interface", "type"}},
		{name: "ze_ospf_packets_dropped_total", labels: []string{"interface", "reason"}},
		{name: "ze_ospf_sockets_open"},
	}
	if !reflect.DeepEqual(reg.records, want) {
		t.Fatalf("metric series = %#v, want %#v", reg.records, want)
	}
}
