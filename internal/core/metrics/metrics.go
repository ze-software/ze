// Design: docs/architecture/core-design.md — metric collection interfaces
// Detail: prometheus.go — Prometheus backend implementing Registry
// Detail: nop.go — no-op backend for tests and disabled state
// Detail: server.go — HTTP server for metrics endpoint
//
// Package metrics provides metric collection interfaces and backends.
// The interfaces (Counter, Gauge, Histogram, CounterVec, GaugeVec,
// HistogramVec, Registry) are deliberately compatible with prometheus
// client_golang types so that the Prometheus backend requires zero
// wrapping for scalar metrics.
package metrics

// Counter is a monotonically increasing metric.
type Counter interface {
	Inc()
	Add(float64)
}

// Gauge is a metric that can go up and down.
type Gauge interface {
	Set(float64)
	Inc()
	Dec()
	Add(float64)
}

// CounterVec is a Counter partitioned by label values.
// With() must receive exactly the number of values matching the label names
// used at creation time; mismatches cause a Prometheus panic.
type CounterVec interface {
	With(labelValues ...string) Counter
	Delete(labelValues ...string) bool
}

// GaugeVec is a Gauge partitioned by label values.
// With() must receive exactly the number of values matching the label names
// used at creation time; mismatches cause a Prometheus panic.
type GaugeVec interface {
	With(labelValues ...string) Gauge
	Delete(labelValues ...string) bool
}

// Histogram records observations in configurable buckets for distribution analysis.
type Histogram interface {
	Observe(float64)
}

// HistogramVec is a Histogram partitioned by label values.
// With() must receive exactly the number of values matching the label names
// used at creation time; mismatches cause a Prometheus panic.
type HistogramVec interface {
	With(labelValues ...string) Histogram
	Delete(labelValues ...string) bool
}

// Registry creates and registers metrics.
type Registry interface {
	Counter(name, help string) Counter
	Gauge(name, help string) Gauge
	CounterVec(name, help string, labelNames []string) CounterVec
	GaugeVec(name, help string, labelNames []string) GaugeVec
	Histogram(name, help string, buckets []float64) Histogram
	HistogramVec(name, help string, buckets []float64, labelNames []string) HistogramVec
}
