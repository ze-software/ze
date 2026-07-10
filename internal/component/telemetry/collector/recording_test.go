// VALIDATES: shared test harness — a recording metrics.Registry that captures
// gauge/counter values by metric name + label tuple, so per-collector fixture
// tests can assert the exact values a Collect() emits.
// PREVENTS: collectors silently emitting wrong values (only NopRegistry existed,
// which records nothing, so collector Collect() output was previously untestable).

//go:build linux

package collector

import (
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// recordingRegistry is a metrics.Registry that records the last Set/Add value
// for every gauge and counter, keyed by metric name and joined label values.
type recordingRegistry struct {
	gauges   map[string]*recVec
	counters map[string]*recVec
}

func newRecordingRegistry() *recordingRegistry {
	return &recordingRegistry{
		gauges:   map[string]*recVec{},
		counters: map[string]*recVec{},
	}
}

// gauge returns the recorded value for a gauge metric at the given label tuple.
func (r *recordingRegistry) gauge(t *testing.T, name string, labels ...string) float64 {
	t.Helper()
	v, ok := r.gaugeOK(name, labels...)
	if !ok {
		t.Fatalf("gauge %q labels=%v not recorded (recorded: %v)", name, labels, r.gaugeKeys(name))
	}
	return v
}

func (r *recordingRegistry) gaugeOK(name string, labels ...string) (float64, bool) {
	vec, ok := r.gauges[name]
	if !ok {
		return 0, false
	}
	v, ok := vec.values[strings.Join(labels, "\x1f")]
	return v, ok
}

func (r *recordingRegistry) gaugeKeys(name string) []string {
	vec, ok := r.gauges[name]
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(vec.values))
	for k := range vec.values {
		keys = append(keys, k)
	}
	return keys
}

//nolint:unused // shared harness: consumed by counter-based collector tests as the W-4 sweep expands
func (r *recordingRegistry) counter(t *testing.T, name string, labels ...string) float64 {
	t.Helper()
	vec, ok := r.counters[name]
	if !ok {
		t.Fatalf("counter %q not recorded", name)
	}
	v, ok := vec.values[strings.Join(labels, "\x1f")]
	if !ok {
		t.Fatalf("counter %q labels=%v not recorded", name, labels)
	}
	return v
}

type recVec struct{ values map[string]float64 }

func (v *recVec) With(labelValues ...string) metrics.Gauge {
	return &recPoint{vec: v, key: strings.Join(labelValues, "\x1f")}
}

func (v *recVec) withCounter(labelValues ...string) metrics.Counter {
	return &recPoint{vec: v, key: strings.Join(labelValues, "\x1f")}
}

func (v *recVec) Delete(...string) bool { return false }

type recCounterVec struct{ vec *recVec }

func (c recCounterVec) With(labelValues ...string) metrics.Counter {
	return c.vec.withCounter(labelValues...)
}
func (c recCounterVec) Delete(...string) bool { return false }

type recPoint struct {
	vec *recVec
	key string
}

func (p *recPoint) Set(f float64) { p.vec.values[p.key] = f }
func (p *recPoint) Inc()          { p.vec.values[p.key]++ }
func (p *recPoint) Dec()          { p.vec.values[p.key]-- }
func (p *recPoint) Add(f float64) { p.vec.values[p.key] += f }

// metrics.Registry implementation.

func (r *recordingRegistry) Gauge(name, _ string) metrics.Gauge {
	vec := &recVec{values: map[string]float64{}}
	r.gauges[name] = vec
	return &recPoint{vec: vec, key: ""}
}

func (r *recordingRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	vec := &recVec{values: map[string]float64{}}
	r.gauges[name] = vec
	return vec
}

func (r *recordingRegistry) Counter(name, _ string) metrics.Counter {
	vec := &recVec{values: map[string]float64{}}
	r.counters[name] = vec
	return &recPoint{vec: vec, key: ""}
}

func (r *recordingRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	vec := &recVec{values: map[string]float64{}}
	r.counters[name] = vec
	return recCounterVec{vec: vec}
}

func (r *recordingRegistry) Histogram(string, string, []float64) metrics.Histogram {
	return metrics.NopRegistry{}.Histogram("", "", nil)
}

func (r *recordingRegistry) HistogramVec(string, string, []float64, []string) metrics.HistogramVec {
	return metrics.NopRegistry{}.HistogramVec("", "", nil, nil)
}
