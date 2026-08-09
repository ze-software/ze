// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- Export Prometheus metrics

package flowexport

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

type exportMetrics struct {
	datagramsTotal  metrics.CounterVec
	bytesTotal      metrics.CounterVec
	errorsTotal     metrics.CounterVec
	samplesTotal    metrics.CounterVec // per sampled interface (spec 2)
	flowsTotal      metrics.CounterVec // per collector, per-flow records (spec 2)
	flowsActive     metrics.Gauge      // tracked conntrack flows (spec 2)
	recentRingDrops metrics.Gauge      // recent-flow ring overwrites before read (characterization tap)
}

var metricsPtr atomic.Pointer[exportMetrics]

// BindMetrics registers Prometheus counters for flow export.
func BindMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	labels := []string{"collector", "protocol"}
	m := &exportMetrics{
		datagramsTotal:  reg.CounterVec("ze_flowexport_datagrams_total", "Flow export datagrams sent", labels),
		bytesTotal:      reg.CounterVec("ze_flowexport_bytes_total", "Flow export bytes sent", labels),
		errorsTotal:     reg.CounterVec("ze_flowexport_errors_total", "Flow export send errors", labels),
		samplesTotal:    reg.CounterVec("ze_flowexport_samples_total", "Packet samples received and exported", []string{"interface"}),
		flowsTotal:      reg.CounterVec("ze_flowexport_flows_total", "Per-flow records exported", []string{"collector"}),
		flowsActive:     reg.Gauge("ze_flowexport_flows_active", "Conntrack flows currently tracked for export"),
		recentRingDrops: reg.Gauge("ze_flowexport_recent_ring_drops", "Recent-flow ring entries overwritten before being read (cumulative)"),
	}
	metricsPtr.Store(m)
}

func setRecentRingDrops(n float64) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.recentRingDrops.Set(n)
}

func incDatagrams(collector, protocol string) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.datagramsTotal.With(collector, protocol).Inc()
}

func addBytes(collector, protocol string, n float64) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.bytesTotal.With(collector, protocol).Add(n)
}

func incErrors(collector, protocol string) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.errorsTotal.With(collector, protocol).Inc()
}

func incSamples(iface string) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.samplesTotal.With(iface).Inc()
}

func addFlows(collector string, n float64) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.flowsTotal.With(collector).Add(n)
}

func setFlowsActive(n float64) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.flowsActive.Set(n)
}
