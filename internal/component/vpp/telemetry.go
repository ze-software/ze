// Design: docs/research/vpp-deployment-reference.md -- VPP stats segment telemetry
// Overview: vpp.go -- VPP lifecycle manager (starts/stops telemetry poller)
// Related: config.go -- StatsSettings with poll interval and socket path

package vpp

import (
	"context"
	"sync/atomic"
	"time"

	"go.fd.io/govpp/api"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// metricsRegPtr stores the metrics registry, set via SetVPPMetricsRegistry.
var metricsRegPtr atomic.Pointer[metrics.Registry]

// SetVPPMetricsRegistry sets the package-level metrics registry for VPP telemetry.
// Called via ConfigureMetrics callback before RunEngine.
func SetVPPMetricsRegistry(reg metrics.Registry) {
	if reg != nil {
		metricsRegPtr.Store(&reg)
	}
}

func getMetricsRegistry() metrics.Registry {
	p := metricsRegPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

// statsProvider abstracts VPP stats access for testing.
// In production, *core.StatsConnection satisfies this interface.
type statsProvider interface {
	GetInterfaceStats(*api.InterfaceStats) error
	GetNodeStats(*api.NodeStats) error
	GetSystemStats(*api.SystemStats) error
}

// statsDisconnector is satisfied by *core.StatsConnection for cleanup.
type statsDisconnector interface {
	Disconnect()
}

// vppMetrics holds registered Prometheus metrics for VPP telemetry.
type vppMetrics struct {
	// Interface metrics (per-interface label).
	ifaceRxPackets metrics.CounterVec
	ifaceTxPackets metrics.CounterVec
	ifaceRxBytes   metrics.CounterVec
	ifaceTxBytes   metrics.CounterVec
	ifaceDrops     metrics.CounterVec
	ifaceRxErrors  metrics.CounterVec
	ifaceTxErrors  metrics.CounterVec

	// Node metrics (per-node label).
	nodeClocks  metrics.GaugeVec
	nodeVectors metrics.GaugeVec
	nodeCalls   metrics.GaugeVec

	// System metrics (no labels).
	sysVectorRate metrics.Gauge
	sysInputRate  metrics.Gauge

	// Connection state.
	statsUp metrics.Gauge
}

// newVPPMetrics registers all VPP telemetry metrics with the given registry.
func newVPPMetrics(reg metrics.Registry) *vppMetrics {
	return &vppMetrics{
		ifaceRxPackets: reg.CounterVec("ze_vpp_interface_rx_packets", "VPP interface received packets.", []string{"interface"}),
		ifaceTxPackets: reg.CounterVec("ze_vpp_interface_tx_packets", "VPP interface transmitted packets.", []string{"interface"}),
		ifaceRxBytes:   reg.CounterVec("ze_vpp_interface_rx_bytes", "VPP interface received bytes.", []string{"interface"}),
		ifaceTxBytes:   reg.CounterVec("ze_vpp_interface_tx_bytes", "VPP interface transmitted bytes.", []string{"interface"}),
		ifaceDrops:     reg.CounterVec("ze_vpp_interface_drops", "VPP interface dropped packets.", []string{"interface"}),
		ifaceRxErrors:  reg.CounterVec("ze_vpp_interface_rx_errors", "VPP interface receive errors.", []string{"interface"}),
		ifaceTxErrors:  reg.CounterVec("ze_vpp_interface_tx_errors", "VPP interface transmit errors.", []string{"interface"}),

		nodeClocks:  reg.GaugeVec("ze_vpp_node_clocks", "VPP graph node clock cycles.", []string{"node"}),
		nodeVectors: reg.GaugeVec("ze_vpp_node_vectors", "VPP graph node vectors processed.", []string{"node"}),
		nodeCalls:   reg.GaugeVec("ze_vpp_node_calls", "VPP graph node calls.", []string{"node"}),

		sysVectorRate: reg.Gauge("ze_vpp_system_vector_rate", "VPP system vector rate."),
		sysInputRate:  reg.Gauge("ze_vpp_system_input_rate", "VPP system input rate."),

		statsUp: reg.Gauge("ze_vpp_stats_up", "VPP stats connection state (1=connected, 0=disconnected)."),
	}
}

// statsPoller periodically reads VPP stats and updates Prometheus metrics.
type statsPoller struct {
	provider statsProvider
	metrics  *vppMetrics
	interval time.Duration

	// Reusable stats structs to avoid allocation per poll.
	ifaceStats api.InterfaceStats
	nodeStats  api.NodeStats
	sysStats   api.SystemStats

	// Previous counter values for delta computation.
	// VPP counters are cumulative; Prometheus counters need Add(delta).
	prevIface map[string]ifaceSnapshot
}

// ifaceSnapshot stores the previous poll's counter values for an interface.
type ifaceSnapshot struct {
	rxPackets uint64
	txPackets uint64
	rxBytes   uint64
	txBytes   uint64
	drops     uint64
	rxErrors  uint64
	txErrors  uint64
}

// newStatsPoller creates a stats poller with the given provider, metrics, and interval.
func newStatsPoller(provider statsProvider, m *vppMetrics, interval time.Duration) *statsPoller {
	return &statsPoller{
		provider:  provider,
		metrics:   m,
		interval:  interval,
		prevIface: make(map[string]ifaceSnapshot),
	}
}

// run polls VPP stats at the configured interval until ctx is canceled.
// Long-lived goroutine, not per-event.
func (p *statsPoller) run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		p.poll()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// poll reads all stat categories and updates metrics. On error, sets stats_up=0.
func (p *statsPoller) poll() {
	lg := logger()

	if err := p.provider.GetInterfaceStats(&p.ifaceStats); err != nil {
		lg.Warn("vpp: stats poll interface failed", "error", err)
		p.metrics.statsUp.Set(0)
		return
	}
	if err := p.provider.GetNodeStats(&p.nodeStats); err != nil {
		lg.Warn("vpp: stats poll node failed", "error", err)
		p.metrics.statsUp.Set(0)
		return
	}
	if err := p.provider.GetSystemStats(&p.sysStats); err != nil {
		lg.Warn("vpp: stats poll system failed", "error", err)
		p.metrics.statsUp.Set(0)
		return
	}

	p.metrics.statsUp.Set(1)
	p.updateInterfaceMetrics()
	p.updateNodeMetrics()
	p.updateSystemMetrics()
}

// updateInterfaceMetrics converts VPP interface counters to Prometheus counter deltas.
func (p *statsPoller) updateInterfaceMetrics() {
	seen := make(map[string]struct{}, len(p.ifaceStats.Interfaces))

	for i := range p.ifaceStats.Interfaces {
		iface := &p.ifaceStats.Interfaces[i]
		name := iface.InterfaceName
		if name == "" {
			continue
		}
		seen[name] = struct{}{}

		prev := p.prevIface[name]

		// Add delta to counters. On first poll or counter reset, add current value.
		p.addCounterDelta(p.metrics.ifaceRxPackets, name, prev.rxPackets, iface.Rx.Packets)
		p.addCounterDelta(p.metrics.ifaceTxPackets, name, prev.txPackets, iface.Tx.Packets)
		p.addCounterDelta(p.metrics.ifaceRxBytes, name, prev.rxBytes, iface.Rx.Bytes)
		p.addCounterDelta(p.metrics.ifaceTxBytes, name, prev.txBytes, iface.Tx.Bytes)
		p.addCounterDelta(p.metrics.ifaceDrops, name, prev.drops, iface.Drops)
		p.addCounterDelta(p.metrics.ifaceRxErrors, name, prev.rxErrors, iface.RxErrors)
		p.addCounterDelta(p.metrics.ifaceTxErrors, name, prev.txErrors, iface.TxErrors)

		p.prevIface[name] = ifaceSnapshot{
			rxPackets: iface.Rx.Packets,
			txPackets: iface.Tx.Packets,
			rxBytes:   iface.Rx.Bytes,
			txBytes:   iface.Tx.Bytes,
			drops:     iface.Drops,
			rxErrors:  iface.RxErrors,
			txErrors:  iface.TxErrors,
		}
	}

	// Remove stale entries for interfaces that no longer exist.
	for name := range p.prevIface {
		if _, ok := seen[name]; !ok {
			delete(p.prevIface, name)
		}
	}
}

// addCounterDelta adds the positive delta between prev and curr to the counter.
// If curr < prev (VPP restart / counter reset), adds curr as the full value.
func (p *statsPoller) addCounterDelta(cv metrics.CounterVec, label string, prev, curr uint64) {
	var delta uint64
	if curr >= prev {
		delta = curr - prev
	} else {
		// Counter reset (VPP restart). Add the current value.
		delta = curr
	}
	if delta > 0 {
		cv.With(label).Add(float64(delta))
	}
}

// updateNodeMetrics sets per-node gauge values from VPP node stats.
// A production VPP has ~600 graph nodes, producing ~1800 gauge time series.
func (p *statsPoller) updateNodeMetrics() {
	for i := range p.nodeStats.Nodes {
		node := &p.nodeStats.Nodes[i]
		name := node.NodeName
		if name == "" {
			continue
		}
		p.metrics.nodeClocks.With(name).Set(float64(node.Clocks))
		p.metrics.nodeVectors.With(name).Set(float64(node.Vectors))
		p.metrics.nodeCalls.With(name).Set(float64(node.Calls))
	}
}

// updateSystemMetrics sets system-wide gauge values from VPP system stats.
func (p *statsPoller) updateSystemMetrics() {
	p.metrics.sysVectorRate.Set(float64(p.sysStats.VectorRate))
	p.metrics.sysInputRate.Set(float64(p.sysStats.InputRate))
}
