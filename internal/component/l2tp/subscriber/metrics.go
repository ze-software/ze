// Design: plan/learned/760-subscriber-session-model.md -- session telemetry

package subscriber

import (
	"sync"

	"github.com/ze-software/ze/internal/core/metrics"
)

type subscriberMetrics struct {
	sessionsActive metrics.GaugeVec
	sessionsTotal  metrics.CounterVec
	authResults    metrics.CounterVec
}

var (
	metricsMu    sync.Mutex
	boundMetrics *subscriberMetrics
)

func BindMetrics(reg metrics.Registry) {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	if boundMetrics != nil {
		return
	}
	accessTypeLabel := []string{"access_type"}
	boundMetrics = &subscriberMetrics{
		sessionsActive: reg.GaugeVec(
			"ze_subscriber_sessions",
			"Number of active subscriber sessions.",
			accessTypeLabel,
		),
		sessionsTotal: reg.CounterVec(
			"ze_subscriber_sessions_total",
			"Total subscriber sessions created.",
			accessTypeLabel,
		),
		authResults: reg.CounterVec(
			"ze_subscriber_auth_results",
			"Subscriber authentication outcomes.",
			[]string{"access_type", "result"},
		),
	}
}

func RecordSessionUp(accessType AccessType) {
	metricsMu.Lock()
	m := boundMetrics
	metricsMu.Unlock()
	if m == nil {
		return
	}
	at := string(accessType)
	m.sessionsActive.With(at).Inc()
	m.sessionsTotal.With(at).Inc()
}

func RecordSessionDown(accessType AccessType) {
	metricsMu.Lock()
	m := boundMetrics
	metricsMu.Unlock()
	if m == nil {
		return
	}
	m.sessionsActive.With(string(accessType)).Dec()
}

func RecordAuthResult(accessType AccessType, accept bool) {
	metricsMu.Lock()
	m := boundMetrics
	metricsMu.Unlock()
	if m == nil {
		return
	}
	result := "reject"
	if accept {
		result = "accept"
	}
	m.authResults.With(string(accessType), result).Inc()
}
