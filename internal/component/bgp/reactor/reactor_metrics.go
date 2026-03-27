// Design: docs/architecture/core-design.md — reactor-level Prometheus metrics
// Overview: reactor.go — Reactor struct and lifecycle
// Related: forward_pool.go — overflow depth, pool ratio, source stats polled by metrics loop

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
	poolUsedRatio        metrics.Gauge // AC-18: overflow pool utilization (0.0 to 1.0)

	// Per-peer (labeled by peer address)
	peerState     metrics.GaugeVec
	peerMsgRecv   metrics.CounterVec // labels: peer, type
	peerMsgSent   metrics.CounterVec // labels: peer, type
	overflowItems metrics.GaugeVec   // AC-17: per-destination overflow depth
	overflowRatio metrics.GaugeVec   // AC-16: per-source overflowed/(forwarded+overflowed)

	// Session lifecycle (labeled by peer address)
	sessionsEstablished metrics.CounterVec // Times session reached Established
	sessionFlaps        metrics.CounterVec // Sessions dropped from Established
	stateTransitions    metrics.CounterVec // labels: peer, from, to
	notifSent           metrics.CounterVec // labels: peer, code, subcode
	notifRecv           metrics.CounterVec // labels: peer, code, subcode
	sessionDuration     metrics.GaugeVec   // Seconds since session established

	// Forward pool events
	fwdCongestionEvents metrics.CounterVec // Channel full onset events (peer)
	fwdCongestionResume metrics.CounterVec // Channel resumed from congestion (peer)

	// Config + operational
	configReloads      metrics.Counter    // Successful config reloads
	configReloadErrors metrics.CounterVec // labels: error_type
	peersAddedTotal    metrics.Counter    // Peers added via config
	peersRemovedTotal  metrics.Counter    // Peers removed via config

	// Wire layer
	wireBytesRecv   metrics.CounterVec // labels: peer
	wireBytesSent   metrics.CounterVec // labels: peer
	wireReadErrors  metrics.CounterVec // labels: peer
	wireWriteErrors metrics.CounterVec // labels: peer

	// Prefix limits (labeled by peer + family)
	prefixCount           metrics.GaugeVec   // Current prefix count per family
	prefixMaximum         metrics.GaugeVec   // Configured hard maximum per family
	prefixWarning         metrics.GaugeVec   // Configured warning threshold per family
	prefixWarningExceeded metrics.GaugeVec   // 1 if count >= warning for this family
	prefixRatio           metrics.GaugeVec   // current_count / maximum (0.0 to 1.0+)
	prefixExceededTotal   metrics.CounterVec // Times this family exceeded maximum
	prefixTeardownTotal   metrics.CounterVec // Times session torn down for prefix limit (per peer)
	prefixStale           metrics.GaugeVec   // 1 if prefix updated timestamp is older than 6 months (per peer)
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
		poolUsedRatio:        reg.Gauge("ze_bgp_pool_used_ratio", "Overflow pool utilization (0.0 = empty, 1.0 = full)."),

		peerState:     reg.GaugeVec("ze_peer_state", "Peer FSM state (0=stopped, 1=connecting, 2=active, 3=established).", []string{"peer"}),
		peerMsgRecv:   reg.CounterVec("ze_peer_messages_received_total", "BGP messages received from peer.", []string{"peer", "type"}),
		peerMsgSent:   reg.CounterVec("ze_peer_messages_sent_total", "BGP messages sent to peer.", []string{"peer", "type"}),
		overflowItems: reg.GaugeVec("ze_bgp_overflow_items", "Items in per-destination overflow buffer.", []string{"peer"}),
		overflowRatio: reg.GaugeVec("ze_bgp_overflow_ratio", "Per-source overflow ratio: overflowed/(forwarded+overflowed).", []string{"source"}),

		// Session lifecycle
		sessionsEstablished: reg.CounterVec("ze_peer_sessions_established_total", "Times session reached Established.", []string{"peer"}),
		sessionFlaps:        reg.CounterVec("ze_peer_session_flaps_total", "Sessions dropped from Established.", []string{"peer"}),
		stateTransitions:    reg.CounterVec("ze_peer_state_transitions_total", "Peer state transitions.", []string{"peer", "from", "to"}),
		notifSent:           reg.CounterVec("ze_peer_notifications_sent_total", "NOTIFICATION messages sent.", []string{"peer", "code", "subcode"}),
		notifRecv:           reg.CounterVec("ze_peer_notifications_received_total", "NOTIFICATION messages received.", []string{"peer", "code", "subcode"}),
		sessionDuration:     reg.GaugeVec("ze_peer_session_duration_seconds", "Seconds since session established.", []string{"peer"}),

		// Forward pool events
		fwdCongestionEvents: reg.CounterVec("ze_forward_congestion_events_total", "Channel full events (onset).", []string{"peer"}),
		fwdCongestionResume: reg.CounterVec("ze_forward_congestion_resumed_total", "Channel resumed from congestion.", []string{"peer"}),

		// Config + operational
		configReloads:      reg.Counter("ze_config_reloads_total", "Successful config reloads."),
		configReloadErrors: reg.CounterVec("ze_config_reload_errors_total", "Failed config reloads.", []string{"error_type"}),
		peersAddedTotal:    reg.Counter("ze_peers_added_total", "Peers added via config."),
		peersRemovedTotal:  reg.Counter("ze_peers_removed_total", "Peers removed via config."),

		// Wire layer
		wireBytesRecv:   reg.CounterVec("ze_wire_bytes_received_total", "Bytes read from TCP.", []string{"peer"}),
		wireBytesSent:   reg.CounterVec("ze_wire_bytes_sent_total", "Bytes written to TCP.", []string{"peer"}),
		wireReadErrors:  reg.CounterVec("ze_wire_read_errors_total", "Socket read failures.", []string{"peer"}),
		wireWriteErrors: reg.CounterVec("ze_wire_write_errors_total", "Socket write failures.", []string{"peer"}),

		// RFC 4486: Prefix limit metrics
		prefixCount:           reg.GaugeVec("ze_bgp_prefix_count", "Current prefix count per family.", []string{"peer", "family"}),
		prefixMaximum:         reg.GaugeVec("ze_bgp_prefix_maximum", "Configured hard maximum per family.", []string{"peer", "family"}),
		prefixWarning:         reg.GaugeVec("ze_bgp_prefix_warning", "Configured warning threshold per family.", []string{"peer", "family"}),
		prefixWarningExceeded: reg.GaugeVec("ze_bgp_prefix_warning_exceeded", "1 if count >= warning for this family.", []string{"peer", "family"}),
		prefixRatio:           reg.GaugeVec("ze_bgp_prefix_ratio", "Current prefix count / maximum (0.0 to 1.0+).", []string{"peer", "family"}),
		prefixExceededTotal:   reg.CounterVec("ze_bgp_prefix_maximum_exceeded_total", "Times this family exceeded maximum.", []string{"peer", "family"}),
		prefixTeardownTotal:   reg.CounterVec("ze_bgp_prefix_teardown_total", "Times session torn down for prefix limit.", []string{"peer"}),
		prefixStale:           reg.GaugeVec("ze_bgp_prefix_stale", "1 if prefix data is older than 6 months.", []string{"peer"}),
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

	now := r.clock.Now()

	// Uptime
	m.uptimeSeconds.Set(now.Sub(r.startTime).Seconds())

	// Cache entries
	m.cacheEntries.Set(float64(r.recentUpdates.Len()))

	// Per-peer session duration
	r.mu.RLock()
	for _, peer := range r.peers {
		if est := peer.EstablishedAt(); !est.IsZero() {
			m.sessionDuration.With(peer.peerAddrLabel()).Set(now.Sub(est).Seconds())
		}
	}
	r.mu.RUnlock()

	// Forward pool workers + overflow metrics
	if r.fwdPool != nil {
		m.forwardWorkersActive.Set(float64(r.fwdPool.WorkerCount()))

		// AC-18: pool utilization ratio
		m.poolUsedRatio.Set(r.fwdPool.PoolUsedRatio())

		// AC-17: per-destination overflow depth
		for peer, depth := range r.fwdPool.OverflowDepths() {
			m.overflowItems.With(peer).Set(float64(depth))
		}

		// AC-16: per-source overflow ratio
		for peer, ratio := range r.fwdPool.SourceOverflowRatios() {
			m.overflowRatio.With(peer).Set(ratio)
		}
	}
}
