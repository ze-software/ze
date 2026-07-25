// Design: plan/learned/1015-cp-survival-5-detect-5-characterization.md -- Stage-2 metrics.
// Related: characterize.go -- increments incCharacterize/incFallback on the emit paths.
// Characterization observability: per-family outcome counts and a fallback count
// (characterization attempted but no flow source / no usable data). Registered
// via Registration.ConfigureMetrics; no-op until a registry is bound.

package detect

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/metrics"
)

type detectMetrics struct {
	characterize metrics.CounterVec // outcomes labeled by attack family
	fallback     metrics.Counter    // characterize enabled but no flow source / no data
	bpsTrigger   metrics.Counter    // detections attributed to the bandwidth (BPS) trigger (PPS alone would not have fired)

	policySuppressed metrics.CounterVec // attacks the traffic policy exempted, by scope (detection|mitigation)
	direction        metrics.CounterVec // detected attacks by victim direction (local|remote)
}

var metricsPtr atomic.Pointer[detectMetrics]

func setMetricsRegistry(reg metrics.Registry) {
	metricsPtr.Store(&detectMetrics{
		characterize: reg.CounterVec("ze_ddos_detect_characterize_total",
			"DDoS characterization outcomes by attack family.", []string{"family"}),
		fallback: reg.Counter("ze_ddos_detect_characterize_fallback_total",
			"Characterizations that produced no AttackCharacterized (no flow source or no usable flows)."),
		bpsTrigger: reg.Counter("ze_ddos_detect_bps_trigger_total",
			"Detections attributed to the bandwidth (BPS) trigger, where the packet-rate threshold alone would not have fired."),
		policySuppressed: reg.CounterVec("ze_ddos_policy_suppressed_total",
			"Attacks exempted by the ddos/detect traffic policy, labeled by scope (detection|mitigation).", []string{"scope"}),
		direction: reg.CounterVec("ze_ddos_direction_total",
			"Detected attacks by victim direction (local|remote).", []string{"direction"}),
	})
}

func incBpsTrigger() {
	if m := metricsPtr.Load(); m != nil {
		m.bpsTrigger.Inc()
	}
}

func incCharacterize(family ddosevent.AttackFamily) {
	if m := metricsPtr.Load(); m != nil {
		m.characterize.With(string(family)).Inc()
	}
}

func incFallback() {
	if m := metricsPtr.Load(); m != nil {
		m.fallback.Inc()
	}
}

func incPolicySuppressed(scope string) {
	if m := metricsPtr.Load(); m != nil {
		m.policySuppressed.With(scope).Inc()
	}
}

func incDirection(dir ddosevent.Direction) {
	if m := metricsPtr.Load(); m != nil {
		m.direction.With(string(dir)).Inc()
	}
}
