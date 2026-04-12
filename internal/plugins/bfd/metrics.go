// Design: docs/architecture/bfd.md -- Prometheus metric surface
// Related: bfd.go -- plugin lifecycle that wires the metrics hook onto loops
// Related: engine/engine.go -- MetricsHook interface implemented here
//
// Prometheus metrics for the BFD plugin. Counters are incremented
// directly from the engine express-loop via a MetricsHook; the
// sessions gauge is populated from Snapshot at scrape time.
//
// Metric naming follows the ze_<subsystem>_<name>_<unit> convention
// used by other plugins (see sysrib.go for the reference shape).
package bfd

import (
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/engine"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// bfdMetrics is the set of Prometheus metrics the BFD plugin publishes.
// The fields are populated once in bindMetricsRegistry; subsequent reads
// load through bfdMetricsPtr atomically.
type bfdMetrics struct {
	sessions         metrics.GaugeVec   // labels: state, mode, vrf
	transitions      metrics.CounterVec // labels: from, to, diag, mode
	detectionExpired metrics.CounterVec // labels: mode
	txPackets        metrics.CounterVec // labels: mode
	rxPackets        metrics.CounterVec // labels: mode
	authFailures     metrics.CounterVec // labels: mode
	echoTxPackets    metrics.CounterVec // labels: mode (single-hop only)
	echoRxPackets    metrics.CounterVec // labels: mode
}

// bfdMetricsPtr holds the active metrics set. Nil while
// bindMetricsRegistry has not been called yet (early CLI invocations,
// unit tests running without a Prometheus registry).
var bfdMetricsPtr atomic.Pointer[bfdMetrics]

// bindMetricsRegistry creates the BFD Prometheus metrics from reg.
// Called via registry.Registration.ConfigureMetrics before
// RunBFDPlugin. Safe to call multiple times (last writer wins); Stage
// 4 registers once per daemon lifetime.
//
// The name deliberately avoids the project-wide SetMetricsRegistry
// convention because that helper is internal to each plugin and the
// pre-write guard disallows duplicating a top-level symbol across
// plugins.
func bindMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		return
	}
	m := &bfdMetrics{
		sessions:    reg.GaugeVec("ze_bfd_sessions", "Live BFD session count by state.", []string{"state", "mode", "vrf"}),
		transitions: reg.CounterVec("ze_bfd_transitions_total", "BFD session state transitions.", []string{"from", "to", "diag", "mode"}),
		detectionExpired: reg.CounterVec("ze_bfd_detection_expired_total",
			"Detection-timer expirations (RFC 5880 Section 6.8.4).", []string{"mode"}),
		txPackets:     reg.CounterVec("ze_bfd_tx_packets_total", "BFD Control packets transmitted.", []string{"mode"}),
		rxPackets:     reg.CounterVec("ze_bfd_rx_packets_total", "BFD Control packets received (after TTL gate).", []string{"mode"}),
		authFailures:  reg.CounterVec("ze_bfd_auth_failures_total", "BFD authentication verify failures.", []string{"mode"}),
		echoTxPackets: reg.CounterVec("ze_bfd_echo_tx_packets_total", "BFD Echo packets transmitted (RFC 5880 Section 6.4).", []string{"mode"}),
		echoRxPackets: reg.CounterVec("ze_bfd_echo_rx_packets_total", "BFD Echo packets received on UDP port 3785.", []string{"mode"}),
	}
	bfdMetricsPtr.Store(m)
}

// metricsHook is the engine.MetricsHook implementation that forwards
// events into the live Prometheus counters. A nil bfdMetricsPtr is a
// no-op so the engine can run without telemetry wired up.
type metricsHook struct{}

// OnStateChange increments the transitions counter and, on a
// Control-Detect-Expired diagnostic, also the detection-expired
// counter. RFC 5880 Section 6.8.4 is the sole path that sets that
// diagnostic in the current engine, so the two events are one-to-one.
func (metricsHook) OnStateChange(from, to packet.State, diag packet.Diag, mode, _ string) {
	m := bfdMetricsPtr.Load()
	if m == nil {
		return
	}
	m.transitions.With(from.String(), to.String(), diag.String(), mode).Inc()
	if diag == packet.DiagControlDetectExpired && to == packet.StateDown {
		m.detectionExpired.With(mode).Inc()
	}
}

// OnTxPacket increments the TX packet counter for mode.
func (metricsHook) OnTxPacket(mode string) {
	m := bfdMetricsPtr.Load()
	if m == nil {
		return
	}
	m.txPackets.With(mode).Inc()
}

// OnRxPacket increments the RX packet counter for mode.
func (metricsHook) OnRxPacket(mode string) {
	m := bfdMetricsPtr.Load()
	if m == nil {
		return
	}
	m.rxPackets.With(mode).Inc()
}

// OnAuthFailure increments the auth-failures counter for mode. Called
// from engine.handleInbound when Machine.Verify rejects a packet.
func (metricsHook) OnAuthFailure(mode string) {
	m := bfdMetricsPtr.Load()
	if m == nil {
		return
	}
	m.authFailures.With(mode).Inc()
}

// OnEchoTx increments the echo TX counter. Single-hop only.
func (metricsHook) OnEchoTx(mode string) {
	m := bfdMetricsPtr.Load()
	if m == nil {
		return
	}
	m.echoTxPackets.With(mode).Inc()
}

// OnEchoRx increments the echo RX counter. Single-hop only.
func (metricsHook) OnEchoRx(mode string) {
	m := bfdMetricsPtr.Load()
	if m == nil {
		return
	}
	m.echoRxPackets.With(mode).Inc()
}

// attachMetricsHook installs the metricsHook on a newly-created Loop.
// Called from loopFor immediately after engine.NewLoop so every TX/RX
// and state change flows through the same hook regardless of which
// (vrf, mode) tuple created the loop.
func attachMetricsHook(loop *engine.Loop) {
	if loop == nil {
		return
	}
	loop.SetMetricsHook(metricsHook{})
}

// refreshSessionsGauge rebuilds the ze_bfd_sessions gauge from a
// snapshot. Each call sets every (state, mode, vrf) cell to the count
// observed at that moment; missing cells are not cleared because
// GaugeVec has no "zero all" primitive -- labels that vanish from the
// config stay at their last recorded value until the daemon exits.
// That matches the Prometheus gauge semantics operators expect.
func refreshSessionsGauge(snapshot []api.SessionState) {
	m := bfdMetricsPtr.Load()
	if m == nil {
		return
	}
	type bucket struct {
		state, mode, vrf string
	}
	counts := make(map[bucket]float64, len(snapshot))
	for i := range snapshot {
		s := &snapshot[i]
		counts[bucket{state: s.State, mode: s.Mode, vrf: s.VRF}]++
	}
	for b, n := range counts {
		m.sessions.With(b.state, b.mode, b.vrf).Set(n)
	}
}
