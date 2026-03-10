// Design: docs/architecture/core-design.md — Prometheus metrics backend
// Overview: metrics.go — metric collection interfaces
// Related: server.go — HTTP server exposing Prometheus metrics

package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusRegistry implements Registry backed by a per-instance
// prometheus.Registry. Uses a private registry (not the global default)
// so tests don't interfere and the HTTP handler serves only ze metrics.
//
// All methods are idempotent: calling Counter/Gauge/CounterVec/GaugeVec
// with the same name returns the existing metric. This allows multiple
// components and plugins to register metrics dynamically by name.
type PrometheusRegistry struct {
	registry    *prometheus.Registry
	counters    map[string]Counter
	gauges      map[string]Gauge
	counterVecs map[string]CounterVec
	gaugeVecs   map[string]GaugeVec
	mu          sync.RWMutex
}

// NewPrometheusRegistry creates a PrometheusRegistry with a fresh
// per-instance prometheus.Registry.
func NewPrometheusRegistry() *PrometheusRegistry {
	return &PrometheusRegistry{
		registry:    prometheus.NewRegistry(),
		counters:    make(map[string]Counter),
		gauges:      make(map[string]Gauge),
		counterVecs: make(map[string]CounterVec),
		gaugeVecs:   make(map[string]GaugeVec),
	}
}

// Counter returns a prometheus Counter, creating and registering it on first call.
// Subsequent calls with the same name return the existing counter.
// prometheus.Counter already satisfies the metrics.Counter interface
// (Inc() and Add(float64)), so no wrapping is needed.
func (r *PrometheusRegistry) Counter(name, help string) Counter {
	r.mu.RLock()
	if c, ok := r.counters[name]; ok {
		r.mu.RUnlock()
		return c
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if c, ok := r.counters[name]; ok {
		return c
	}

	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: name,
		Help: help,
	})
	r.registry.MustRegister(c)
	r.counters[name] = c

	return c
}

// Gauge returns a prometheus Gauge, creating and registering it on first call.
// Subsequent calls with the same name return the existing gauge.
// prometheus.Gauge already satisfies the metrics.Gauge interface
// (Set(float64), Inc(), Dec(), Add(float64)), so no wrapping is needed.
func (r *PrometheusRegistry) Gauge(name, help string) Gauge {
	r.mu.RLock()
	if g, ok := r.gauges[name]; ok {
		r.mu.RUnlock()
		return g
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if g, ok := r.gauges[name]; ok {
		return g
	}

	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	})
	r.registry.MustRegister(g)
	r.gauges[name] = g

	return g
}

// CounterVec returns a prometheus CounterVec, creating and registering it on first call.
// Subsequent calls with the same name return the existing vector.
// Wrapped to return our Counter interface from With().
func (r *PrometheusRegistry) CounterVec(name, help string, labelNames []string) CounterVec {
	r.mu.RLock()
	if cv, ok := r.counterVecs[name]; ok {
		r.mu.RUnlock()
		return cv
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if cv, ok := r.counterVecs[name]; ok {
		return cv
	}

	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: help,
	}, labelNames)
	r.registry.MustRegister(cv)
	wrapped := &promCounterVec{cv: cv}
	r.counterVecs[name] = wrapped

	return wrapped
}

// GaugeVec returns a prometheus GaugeVec, creating and registering it on first call.
// Subsequent calls with the same name return the existing vector.
// Wrapped to return our Gauge interface from With().
func (r *PrometheusRegistry) GaugeVec(name, help string, labelNames []string) GaugeVec {
	r.mu.RLock()
	if gv, ok := r.gaugeVecs[name]; ok {
		r.mu.RUnlock()
		return gv
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if gv, ok := r.gaugeVecs[name]; ok {
		return gv
	}

	gv := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, labelNames)
	r.registry.MustRegister(gv)
	wrapped := &promGaugeVec{gv: gv}
	r.gaugeVecs[name] = wrapped

	return wrapped
}

// Handler returns an HTTP handler that serves the /metrics endpoint.
func (r *PrometheusRegistry) Handler() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{})
}

// promCounterVec wraps prometheus.CounterVec to return metrics.Counter.
type promCounterVec struct {
	cv *prometheus.CounterVec
}

func (v *promCounterVec) With(labelValues ...string) Counter {
	return v.cv.WithLabelValues(labelValues...)
}

func (v *promCounterVec) Delete(labelValues ...string) bool {
	return v.cv.DeleteLabelValues(labelValues...)
}

// promGaugeVec wraps prometheus.GaugeVec to return metrics.Gauge.
type promGaugeVec struct {
	gv *prometheus.GaugeVec
}

func (v *promGaugeVec) With(labelValues ...string) Gauge {
	return v.gv.WithLabelValues(labelValues...)
}

func (v *promGaugeVec) Delete(labelValues ...string) bool {
	return v.gv.DeleteLabelValues(labelValues...)
}
