// Design: plan/learned/957-ospf-3-ip-transport.md -- metric series registration tests

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

type captureCounterVec struct{ labels *[][]string }

func (v captureCounterVec) With(labelValues ...string) metrics.Counter {
	cp := append([]string(nil), labelValues...)
	*v.labels = append(*v.labels, cp)
	return noopCounter{}
}
func (captureCounterVec) Delete(...string) bool { return false }

type dropCaptureRegistry struct {
	recordingRegistry
	drops [][]string
}

func (r *dropCaptureRegistry) CounterVec(name, help string, labels []string) metrics.CounterVec {
	if name == "ze_ospf_packets_dropped_total" {
		r.records = append(r.records, metricRecord{name: name, labels: append([]string(nil), labels...)})
		return captureCounterVec{labels: &r.drops}
	}
	return r.recordingRegistry.CounterVec(name, help, labels)
}

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

func TestOSPFTransportMetricsSeries(t *testing.T) {
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

func TestOSPFTransportRecordDrop(t *testing.T) {
	reg := &dropCaptureRegistry{}
	tr := New(nil)
	tr.SetMetrics(reg)
	tr.RecordDrop("eth0", "hello-interval")
	want := [][]string{{"eth0", "hello-interval"}}
	if !reflect.DeepEqual(reg.drops, want) {
		t.Fatalf("drop labels = %#v, want %#v", reg.drops, want)
	}
}
