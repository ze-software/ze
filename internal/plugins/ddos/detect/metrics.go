// Design: plan/learned/1015-cp-survival-5-detect-5-characterization.md -- Stage-2 metrics.
// Related: characterize.go -- increments incCharacterize/incFallback on the emit paths.
// Characterization observability: per-family outcome counts and a fallback count
// (characterization attempted but no flow source / no usable data). Registered
// via Registration.ConfigureMetrics; no-op until a registry is bound.

package detect

import (
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type detectMetrics struct {
	characterize metrics.CounterVec // outcomes labeled by attack family
	fallback     metrics.Counter    // characterize enabled but no flow source / no data
	bpsTrigger   metrics.Counter    // detections attributed to the bandwidth (BPS) trigger (PPS alone would not have fired)
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
