// Design: plan/spec-l2tp-10-metrics.md -- RADIUS Prometheus metrics
// Related: handler.go -- auth request/response call sites
// Related: acct.go -- accounting start/stop/interim call sites

package l2tpauthradius

import (
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type radiusMetrics struct {
	up          metrics.GaugeVec   // labels: server_id, server_addr
	authSent    metrics.CounterVec // labels: server_id, server_addr
	acctSent    metrics.CounterVec // labels: server_id, server_addr
	interimSent metrics.CounterVec // labels: server_id, server_addr
}

var radiusMetricsPtr atomic.Pointer[radiusMetrics]

var radiusServerLabels = []string{"server_id", "server_addr"}

func bindRADIUSMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	m := &radiusMetrics{
		up:          reg.GaugeVec("ze_radius_up", "RADIUS server reachability (1=up, 0=down).", radiusServerLabels),
		authSent:    reg.CounterVec("ze_radius_auth_sent_total", "RADIUS Access-Request packets sent.", radiusServerLabels),
		acctSent:    reg.CounterVec("ze_radius_acct_sent_total", "RADIUS Accounting-Request packets sent.", radiusServerLabels),
		interimSent: reg.CounterVec("ze_radius_interim_sent_total", "RADIUS Interim-Update packets sent.", radiusServerLabels),
	}
	radiusMetricsPtr.Store(m)
}

func incAuthSent(serverID, serverAddr string) {
	m := radiusMetricsPtr.Load()
	if m == nil {
		return
	}
	m.authSent.With(serverID, serverAddr).Inc()
}

func incAcctSent(serverID, serverAddr string) {
	m := radiusMetricsPtr.Load()
	if m == nil {
		return
	}
	m.acctSent.With(serverID, serverAddr).Inc()
}

func incInterimSent(serverID, serverAddr string) {
	m := radiusMetricsPtr.Load()
	if m == nil {
		return
	}
	m.interimSent.With(serverID, serverAddr).Inc()
}

func setRadiusUp(serverID, serverAddr string, up bool) {
	m := radiusMetricsPtr.Load()
	if m == nil {
		return
	}
	v := float64(0)
	if up {
		v = 1
	}
	m.up.With(serverID, serverAddr).Set(v)
}
