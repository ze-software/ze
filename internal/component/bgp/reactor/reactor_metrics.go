// Design: docs/architecture/core-design.md — reactor-level Prometheus metrics
// Overview: reactor.go — Reactor struct and lifecycle

package reactor

import (
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// metricsUpdateInterval is how often periodic metrics are refreshed.
const metricsUpdateInterval = 10 * time.Second

// reactorMetrics holds Prometheus metrics for the reactor.
// Created once at startup when a metrics registry is set.
type reactorMetrics struct {
	// Reactor-level (unlabeled)
	peersConfigured      metrics.Gauge
	uptimeSeconds        metrics.Gauge
	cacheEntries         metrics.Gauge
	forwardWorkersActive metrics.Gauge

	// Per-peer (labeled by peer address)
	peerState   metrics.GaugeVec
	peerMsgRecv metrics.CounterVec
	peerMsgSent metrics.CounterVec
}

// initReactorMetrics creates reactor-level metrics from the registry.
// Called during StartWithContext when metrics are enabled from config.
func initReactorMetrics(reg metrics.Registry, version, routerID, localAS string) *reactorMetrics {
	// ze_info gauge with version/router_id/local_as labels
	info := reg.GaugeVec("ze_info", "Ze instance information.", []string{"version", "router_id", "local_as"})
	info.With(version, routerID, localAS).Set(1)

	return &reactorMetrics{
		peersConfigured:      reg.Gauge("ze_peers_configured", "Number of configured BGP peers."),
		uptimeSeconds:        reg.Gauge("ze_uptime_seconds", "Seconds since reactor started."),
		cacheEntries:         reg.Gauge("ze_cache_entries", "UPDATE cache entry count."),
		forwardWorkersActive: reg.Gauge("ze_forward_workers_active", "Active forward pool workers."),

		peerState:   reg.GaugeVec("ze_peer_state", "Peer FSM state (0=stopped, 1=connecting, 2=active, 3=established).", []string{"peer"}),
		peerMsgRecv: reg.CounterVec("ze_peer_messages_received_total", "BGP messages received from peer.", []string{"peer"}),
		peerMsgSent: reg.CounterVec("ze_peer_messages_sent_total", "BGP messages sent to peer.", []string{"peer"}),
	}
}

// metricsUpdateLoop periodically refreshes gauges that are read from snapshots
// rather than incremented on events. Runs until the reactor context is canceled.
func (r *Reactor) metricsUpdateLoop() {
	ticker := time.NewTicker(metricsUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.updatePeriodicMetrics()
		}
	}
}

// updatePeriodicMetrics refreshes snapshot-based gauges.
func (r *Reactor) updatePeriodicMetrics() {
	m := r.rmetrics
	if m == nil {
		return
	}

	// Uptime
	m.uptimeSeconds.Set(r.clock.Now().Sub(r.startTime).Seconds())

	// Cache entries
	m.cacheEntries.Set(float64(r.recentUpdates.Len()))

	// Forward pool workers
	if r.fwdPool != nil {
		m.forwardWorkersActive.Set(float64(r.fwdPool.WorkerCount()))
	}
}
