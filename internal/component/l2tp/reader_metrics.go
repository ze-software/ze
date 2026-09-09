// Design: docs/research/l2tpv2-ze-integration.md -- receiver goroutines and a failing socket
// Related: listener.go -- readLoop: the site that calls countReadError
// Related: subsystem.go -- Start registers bindListenerMetrics with the plugin registry

package l2tp

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

// listenerMetricsHookName is the key registry.InjectPluginMetrics stores this
// hook under. It is a namespace, not a plugin declaration: the registry keys
// the deferral by name only, and no plugin under internal/plugins/ carries
// this one. It is a separate hook from l2tpMetrics's own binding
// (metrics.go) rather than a field added to that struct, because
// bindL2TPMetrics still reads registry.GetMetricsRegistry directly and
// readLoop's counter must not inherit that gap (see the note on
// registerListenerMetrics below).
const listenerMetricsHookName = "l2tp-listener"

// listenerMetrics holds the Prometheus counter readLoop publishes for a read
// error it swallows and retries. One instance serves every UDPListener in
// the process, so a failure on any bound listener accumulates on one
// series; a subscriber daemon binds at most one listener per configured
// listen address, and the failure this counter names is a property of the
// socket, not of any one peer.
type listenerMetrics struct {
	readErrorsTotal metrics.Counter
}

// listenerMetricsPtr holds the active metric set. It is nil until a
// registry exists, which is why countReadError reads it on every call
// rather than caching it on the listener: readLoop runs from its own
// goroutine, started well before Start's caller can know whether a metrics
// registry exists yet.
var listenerMetricsPtr atomic.Pointer[listenerMetrics]

// initListenerMetrics creates the l2tp listener's Prometheus counter from reg.
func initListenerMetrics(reg metrics.Registry) *listenerMetrics {
	return &listenerMetrics{
		readErrorsTotal: reg.Counter("ze_l2tp_listener_read_errors_total",
			"UDP read errors readLoop swallowed and retried, after the socket-closed case is ruled out."),
	}
}

// registerListenerMetrics asks for this counter, now if a metrics registry
// already exists and as soon as one does otherwise.
//
// Reading registry.GetMetricsRegistry here instead would bind nothing on an
// L2TP-only daemon with no bgp block, and leave
// ze_l2tp_listener_read_errors_total absent for the process lifetime with
// no line saying so: the same gap internal/component/l2tp/pppoe/metrics.go
// and internal/component/l2tp/ppp/metrics.go document and close for their
// own counters. This package's own bindL2TPMetrics (metrics.go) still reads
// registry.GetMetricsRegistry directly at Subsystem.Start and carries that
// gap for the session gauges and CQM series it binds; it is unrelated to
// readLoop's counter, untouched by this change, and recorded in
// plan/journal/registry-read-outruns-its-lazy-creation.md rather than fixed
// here, because folding readLoop's counter into that same call site would
// also have to defer poller.start() and s.statsPoller's assignment, which
// changes the concurrency of code this work does not otherwise touch.
func registerListenerMetrics() {
	registry.InjectPluginMetrics(listenerMetricsHookName, bindListenerMetrics)
}

// bindListenerMetrics publishes the counter into reg. It is the hook
// registerListenerMetrics hands to the registry, never called directly by
// the listener.
func bindListenerMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	listenerMetricsPtr.Store(initListenerMetrics(reg))
}

// countReadError increments readLoop's swallowed-read-error counter. It is
// a no-op while no registry is bound, which is a daemon started without a
// telemetry block and every test that does not ask for the counter.
func countReadError() {
	m := listenerMetricsPtr.Load()
	if m == nil {
		return
	}
	m.readErrorsTotal.Inc()
}
