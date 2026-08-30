// Design: docs/architecture/core-design.md — reactor-level Prometheus metrics
// RFC: rfc/short/rfc1997.md — well-known community egress suppression counter
// Overview: reactor.go — Reactor struct and lifecycle
// Related: forward_pool.go — overflow depth, pool ratio, source stats polled by metrics loop

package reactor

import (
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/metrics"
)

// Prometheus label NAMES. These name the dimension a metric is sliced by, and
// they are not the label values: metricLabelPeer is the string "peer" that
// heads the column, while the value in that column is a peer address. A label
// name is part of the metric's published contract, so an operator's query
// breaks when one changes.
const (
	metricLabelPeer    = "peer"
	metricLabelType    = "type"
	metricLabelCode    = "code"
	metricLabelSubcode = "subcode"
	metricLabelFamily  = "family"
)

// metricsUpdateIntervalDefault is how often periodic metrics are refreshed.
// Overridable via ze.metrics.interval env var for testing.
const metricsUpdateIntervalDefault = 10 * time.Second

// metricsUpdateInterval returns the configured metrics refresh interval.
func metricsUpdateInterval() time.Duration {
	return env.GetDuration("ze.metrics.interval", metricsUpdateIntervalDefault)
}

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

	// connectRetryCounter is RFC 4271 §8.1.1 mandatory session attribute 2,
	// "the number of times a BGP peer has tried to establish a peer session".
	// A GAUGE, not a counter: the RFC's own §8.2.2 clauses reset it to zero on
	// an operator start or stop, and a Prometheus counter that goes down is a
	// counter reset to every consumer of rate().
	connectRetryCounter metrics.GaugeVec // labels: peer

	// An OPEN that arrived on a connection already in Established or OpenConfirm
	// and was refused with a Cease (session_handlers.go, handleOpen's state
	// gate). Non-zero names a peer that tried to re-negotiate mid-session.
	openInEstablished metrics.CounterVec // labels: peer

	// Forward pool events
	fwdCongestionEvents  metrics.CounterVec // Channel full onset events (peer)
	fwdCongestionResume  metrics.CounterVec // Channel resumed from congestion (peer)
	fwdBufferDeniedTotal metrics.Counter    // AC-2: total buffer denials
	fwdTeardownTotal     metrics.Counter    // AC-4: forced congestion teardowns

	// Egress modification failures. A non-zero value means a configured
	// modification could not be applied to a route, so the route was suppressed
	// for that destination rather than forwarded unmodified. The reason label
	// set is closed (see modifyFailure.String, forward_modify_failure.go).
	updateModifyFailed metrics.CounterVec // labels: reason

	// RFC 1997 well-known community egress suppressions. A non-zero value means
	// a route received carrying NO_EXPORT, NO_ADVERTISE or NO_EXPORT_SUBCONFED
	// was withheld from a destination the community forbids. The suppression is
	// not configurable and is never logged per route, so this counter is the
	// only place an operator sees it. The label set is closed
	// (wireu.WellKnown.BlockingName).
	wellKnownSuppressed metrics.CounterVec // labels: community

	// Config + operational
	configReloads      metrics.Counter    // Successful config reloads
	configReloadErrors metrics.CounterVec // labels: error_type
	peersAddedTotal    metrics.Counter    // Peers added via config
	peersRemovedTotal  metrics.Counter    // Peers removed via config

	// captureDroppedEvents counts protocol-event-capture records shed because
	// the writer queue was full. Non-zero means the capture file has a gap;
	// the gap is also written into the stream (capture_replay.go).
	captureDroppedEvents metrics.CounterVec // labels: peer

	// Wire layer
	wireBytesRecv   metrics.CounterVec // labels: peer
	wireBytesSent   metrics.CounterVec // labels: peer
	wireReadErrors  metrics.CounterVec // labels: peer
	wireWriteErrors metrics.CounterVec // labels: peer

	// A received UPDATE whose attribute count exceeded the inline span capacity
	// (attribute.SpanInline), so its index spilled to the heap. The inline size is
	// a judgement about the attribute-count distribution rather than a measurement,
	// and this counter is what makes that judgement answerable from a running
	// daemon.
	attrSpanSpill metrics.CounterVec // labels: peer

	// Startup timing (histograms)
	pluginStartupSeconds      metrics.Histogram    // WaitForPluginStartupComplete duration
	apiReadySeconds           metrics.Histogram    // WaitForAPIReady duration
	peerDialSeconds           metrics.HistogramVec // TCP dial duration (labels: peer, result)
	peerConnectAttemptSeconds metrics.HistogramVec // Full runOnce duration (labels: peer)
	peerConnectAttempts       metrics.CounterVec   // Connection attempts (labels: peer)
	peerBackoffSeconds        metrics.HistogramVec // Backoff wait duration (labels: peer)

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

		peerState:     reg.GaugeVec("ze_peer_state", "Peer FSM state (0=stopped, 1=connecting, 2=active, 3=established).", []string{metricLabelPeer}),
		peerMsgRecv:   reg.CounterVec("ze_peer_messages_received_total", "BGP messages received from peer.", []string{metricLabelPeer, metricLabelType}),
		peerMsgSent:   reg.CounterVec("ze_peer_messages_sent_total", "BGP messages sent to peer.", []string{metricLabelPeer, metricLabelType}),
		overflowItems: reg.GaugeVec("ze_bgp_overflow_items", "Items in per-destination overflow buffer.", []string{metricLabelPeer}),
		overflowRatio: reg.GaugeVec("ze_bgp_overflow_ratio", "Per-source overflow ratio: overflowed/(forwarded+overflowed).", []string{"source"}),

		// Session lifecycle
		sessionsEstablished: reg.CounterVec("ze_peer_sessions_established_total", "Times session reached Established.", []string{metricLabelPeer}),
		sessionFlaps:        reg.CounterVec("ze_peer_session_flaps_total", "Sessions dropped from Established.", []string{metricLabelPeer}),
		stateTransitions:    reg.CounterVec("ze_peer_state_transitions_total", "Peer state transitions.", []string{metricLabelPeer, "from", "to"}),
		notifSent:           reg.CounterVec("ze_peer_notifications_sent_total", "NOTIFICATION messages sent.", []string{metricLabelPeer, metricLabelCode, metricLabelSubcode}),
		notifRecv:           reg.CounterVec("ze_peer_notifications_received_total", "NOTIFICATION messages received.", []string{metricLabelPeer, metricLabelCode, metricLabelSubcode}),
		sessionDuration:     reg.GaugeVec("ze_peer_session_duration_seconds", "Seconds since session established.", []string{metricLabelPeer}),
		connectRetryCounter: reg.GaugeVec("ze_bgp_connect_retry_counter", "RFC 4271 ConnectRetryCounter: times this peer has tried to establish a session since the last operator start or stop.", []string{metricLabelPeer}),
		openInEstablished: reg.CounterVec("ze_bgp_open_in_established_total",
			"OPEN messages refused because the connection was already in Established or OpenConfirm.", []string{metricLabelPeer}),

		// Forward pool events
		fwdCongestionEvents:  reg.CounterVec("ze_forward_congestion_events_total", "Channel full events (onset).", []string{metricLabelPeer}),
		fwdCongestionResume:  reg.CounterVec("ze_forward_congestion_resumed_total", "Channel resumed from congestion.", []string{metricLabelPeer}),
		fwdBufferDeniedTotal: reg.Counter("ze_forward_buffer_denied_total", "Buffer denials due to congestion backpressure (AC-2)."),
		fwdTeardownTotal:     reg.Counter("ze_forward_congestion_teardown_total", "Forced session teardowns due to pool exhaustion (AC-4)."),

		wellKnownSuppressed: reg.CounterVec(
			"ze_bgp_wellknown_community_suppressed_total",
			"Routes withheld from a destination peer by an RFC 1997 well-known community.",
			[]string{"community"},
		),

		updateModifyFailed: reg.CounterVec(
			"ze_bgp_update_modify_failed_total",
			"Egress modifications that could not be applied, so the route was suppressed for that destination.",
			[]string{"reason"},
		),

		// Config + operational
		configReloads:      reg.Counter("ze_config_reloads_total", "Successful config reloads."),
		configReloadErrors: reg.CounterVec("ze_config_reload_errors_total", "Failed config reloads.", []string{"error_type"}),
		peersAddedTotal:    reg.Counter("ze_peers_added_total", "Peers added via config."),
		peersRemovedTotal:  reg.Counter("ze_peers_removed_total", "Peers removed via config."),
		captureDroppedEvents: reg.CounterVec("ze_bgp_capture_dropped_events_total",
			"Protocol event capture records shed because the writer queue was full.", []string{metricLabelPeer}),

		// Wire layer
		wireBytesRecv:   reg.CounterVec("ze_wire_bytes_received_total", "Bytes read from TCP.", []string{metricLabelPeer}),
		wireBytesSent:   reg.CounterVec("ze_wire_bytes_sent_total", "Bytes written to TCP.", []string{metricLabelPeer}),
		wireReadErrors:  reg.CounterVec("ze_wire_read_errors_total", "Socket read failures.", []string{metricLabelPeer}),
		wireWriteErrors: reg.CounterVec("ze_wire_write_errors_total", "Socket write failures.", []string{metricLabelPeer}),
		attrSpanSpill: reg.CounterVec("ze_bgp_update_span_spill_total",
			"Received UPDATEs whose attribute count exceeded the inline span capacity.", []string{metricLabelPeer}),

		// Startup and connection timing
		pluginStartupSeconds: reg.Histogram("ze_plugin_startup_seconds", "WaitForPluginStartupComplete duration.",
			[]float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15}),
		apiReadySeconds: reg.Histogram("ze_api_ready_seconds", "WaitForAPIReady duration.",
			[]float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15}),
		peerDialSeconds: reg.HistogramVec("ze_peer_dial_seconds", "TCP dial duration.",
			[]float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5}, []string{metricLabelPeer, "result"}),
		peerConnectAttemptSeconds: reg.HistogramVec("ze_peer_connect_attempt_seconds", "Full connection attempt (runOnce) duration.",
			[]float64{0.1, 0.5, 1, 5, 10, 30, 60, 300}, []string{metricLabelPeer}),
		peerConnectAttempts: reg.CounterVec("ze_peer_connect_attempts_total", "Connection attempts.", []string{metricLabelPeer}),
		peerBackoffSeconds: reg.HistogramVec("ze_peer_backoff_seconds", "Backoff wait duration before retry.",
			[]float64{1, 2, 5, 10, 30, 60, 120}, []string{metricLabelPeer}),

		// RFC 4486: Prefix limit metrics
		prefixCount:           reg.GaugeVec("ze_bgp_prefix_count", "Current prefix count per family.", []string{metricLabelPeer, metricLabelFamily}),
		prefixMaximum:         reg.GaugeVec("ze_bgp_prefix_maximum", "Configured hard maximum per family.", []string{metricLabelPeer, metricLabelFamily}),
		prefixWarning:         reg.GaugeVec("ze_bgp_prefix_warning", "Configured warning threshold per family.", []string{metricLabelPeer, metricLabelFamily}),
		prefixWarningExceeded: reg.GaugeVec("ze_bgp_prefix_warning_exceeded", "1 if count >= warning for this family.", []string{metricLabelPeer, metricLabelFamily}),
		prefixRatio:           reg.GaugeVec("ze_bgp_prefix_ratio", "Current prefix count / maximum (0.0 to 1.0+).", []string{metricLabelPeer, metricLabelFamily}),
		prefixExceededTotal:   reg.CounterVec("ze_bgp_prefix_maximum_exceeded_total", "Times this family exceeded maximum.", []string{metricLabelPeer, metricLabelFamily}),
		prefixTeardownTotal:   reg.CounterVec("ze_bgp_prefix_teardown_total", "Times session torn down for prefix limit.", []string{metricLabelPeer}),
		prefixStale:           reg.GaugeVec("ze_bgp_prefix_stale", "1 if prefix data is older than 6 months.", []string{metricLabelPeer}),
	}
}

// metricsUpdateLoop periodically refreshes gauges that are read from snapshots
// rather than incremented on events. Runs until the reactor context is canceled.
func (r *Reactor) metricsUpdateLoop() {
	ticker := r.clock.NewTicker(metricsUpdateInterval())
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C():
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

	// Per-peer session duration, and the RFC 4271 §8.1.1 ConnectRetryCounter.
	// The counter is refreshed here rather than at each FSM mutation because
	// its producers live in the fsm package, which has no metrics registry;
	// a snapshot gauge reads the same value with no coupling.
	r.mu.RLock()
	for _, peer := range r.peers {
		label := peer.peerAddrLabel()
		if est := peer.EstablishedAt(); !est.IsZero() {
			m.sessionDuration.With(label).Set(now.Sub(est).Seconds())
		}
		m.connectRetryCounter.With(label).Set(float64(peer.ConnectRetryCounter()))
	}
	r.mu.RUnlock()

	// Forward pool workers + overflow metrics
	if r.fwdPool != nil {
		m.forwardWorkersActive.Set(float64(r.fwdPool.WorkerCount()))

		// AC-18: pool utilization ratio
		m.poolUsedRatio.Set(r.fwdPool.poolUsedRatio())

		// AC-17: per-destination overflow depth
		for peer, depth := range r.fwdPool.overflowDepths() {
			m.overflowItems.With(peer).Set(float64(depth))
		}

		// AC-16: per-source overflow ratio
		for peer, ratio := range r.fwdPool.sourceOverflowRatios() {
			m.overflowRatio.With(peer).Set(ratio)
		}
	}
}
