// Design: docs/plugin-development/metrics.md — metrics of a forked plugin
// Overview: metrics.go — metric collection interfaces
// Related: prometheus.go — PrometheusRegistry records the shape of each metric it creates

package metrics

import (
	"slices"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// Kind names the Registry method that created a metric. The zero value is
// Unspecified and is never the kind of a created metric.
type Kind uint8

const (
	KindUnspecified Kind = iota
	KindCounter
	KindGauge
	KindCounterVec
	KindGaugeVec
	KindHistogram
	KindHistogramVec
)

// String names the kind as the Registry method that creates it.
func (k Kind) String() string {
	switch k {
	case KindCounter:
		return "Counter"
	case KindGauge:
		return "Gauge"
	case KindCounterVec:
		return "CounterVec"
	case KindGaugeVec:
		return "GaugeVec"
	case KindHistogram:
		return "Histogram"
	case KindHistogramVec:
		return "HistogramVec"
	case KindUnspecified:
		return "Unspecified"
	}
	return "Kind(" + strconv.Itoa(int(k)) + ")"
}

// Shape is what a metric was created with: the Registry method, the label
// names of a vector and the buckets of a histogram (resolved, never empty).
//
// Two creations of one name share the metric only when their shapes are
// equal. Any other pairing panics inside the Prometheus client: a second
// collector under the same name, or With called with a different number of
// values than the vector has labels. Code that creates metrics whose names
// and shapes arrive from another process, as the forwarded metrics of a
// forked plugin do, reads the existing shape first and refuses a mismatch.
type Shape struct {
	Kind       Kind
	LabelNames []string
	Buckets    []float64
}

// Equal reports whether two shapes would share one metric.
func (s Shape) Equal(other Shape) bool {
	if s.Kind != other.Kind {
		return false
	}
	if !slices.Equal(s.LabelNames, other.LabelNames) {
		return false
	}
	return slices.Equal(s.Buckets, other.Buckets)
}

// ShapeReader is a Registry that reports the shape a metric name was created
// with. The answer is false when no metric of that name was created through
// the Registry methods. Safe for concurrent use.
type ShapeReader interface {
	Shape(name string) (Shape, bool)
}

// ResolveBuckets returns the buckets a histogram created with buckets uses:
// a copy of buckets, or the Prometheus default set when buckets is empty.
// A forked plugin accumulates observations per bucket, so it needs the same
// bounds the daemon's histogram will count with.
func ResolveBuckets(buckets []float64) []float64 {
	if len(buckets) == 0 {
		return slices.Clone(prometheus.DefBuckets)
	}
	return slices.Clone(buckets)
}

// Shape reports the shape name was created with through this registry.
// Metrics registered as raw collectors (the Go runtime and process
// collectors, all outside the ze_ prefix) are not reported.
func (r *PrometheusRegistry) Shape(name string) (Shape, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	shape, ok := r.shapes[name]
	if !ok {
		return Shape{}, false
	}
	return Shape{
		Kind:       shape.Kind,
		LabelNames: slices.Clone(shape.LabelNames),
		Buckets:    slices.Clone(shape.Buckets),
	}, true
}

// Shape reports that no metric exists: a NopRegistry creates nothing, so no
// creation through it can collide with another.
func (NopRegistry) Shape(string) (Shape, bool) { return Shape{}, false }

var (
	_ ShapeReader = (*PrometheusRegistry)(nil)
	_ ShapeReader = NopRegistry{}
)
