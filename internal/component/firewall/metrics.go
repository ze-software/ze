// Design: docs/architecture/core-design.md -- Firewall reconcile concurrency
// Related: registry.go -- ApplyAll, the one reconcile these observe

package firewall

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
)

// Reconcile outcomes, the values of the applyDuration "result" label.
//
// The label exists because latency alone cannot tell a healthy-but-slow apply
// from one that gave up: a backend deadline of 10s and a 10s successful
// reconcile land in the same bucket, and only the label separates them.
const (
	applyResultOK      = "ok"
	applyResultTimeout = "timeout"
	applyResultError   = "error"
	// applyResultPanic is recorded when Backend.Apply does not return at all.
	// A panicking backend must not read as a healthy reconcile.
	applyResultPanic = "panic"
)

// applyMetrics observes the ONE reconcile path every firewall owner shares.
//
// ApplyAll serializes the whole snapshot-plus-apply behind reconcileMu, so a
// slow Apply does not delay one owner, it delays all of them. The latency is
// therefore a property of the process rather than of a caller. The timeout
// counter reports the case the backend deadline exists for: the dataplane
// never answered, so the ruleset the registry holds is not the ruleset the
// dataplane enforces.
//
// applyTimeouts and applyDuration's timeout label count the same population by
// construction: observeApply derives both from one result value, so no edit can
// increment one without the other.
type applyMetrics struct {
	applyDuration metrics.HistogramVec
	applyTimeouts metrics.Counter
}

// applyDurationBuckets spans both regimes this call has. A healthy apply takes
// under a millisecond to a few milliseconds; a wedged one runs to the backend
// deadline, so the tail buckets separate "slow" from "gave up".
//
// The last finite bucket is DERIVED from MaxBackendDeadline (backend.go), the
// ceiling both backends clamp their knob to. A hand-typed ceiling here would be
// a third literal that nothing keeps in step: raise a backend clamp alone and
// every max-deadline timeout lands in +Inf, indistinguishable from any other
// overrun and unreachable by a quantile, with nothing red to say so.
var applyDurationBuckets = []float64{
	0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30,
	MaxBackendDeadline.Seconds(),
}

var applyMetricsPtr atomic.Pointer[applyMetrics]

// bindMetrics is called through Registration.ConfigureMetrics.
//
// It does NOT reliably run before the engine: InjectPluginMetrics defers the
// hook when no metrics registry exists yet at plugin spawn, and runs it later
// when one is set. Until it runs, observeApply finds a nil pointer and only
// logs. In the shipped daemon the deferral window closes before any reconcile,
// because startStandaloneTelemetry (cmd/ze/hub) calls SetMetricsRegistry ahead
// of the plugin phase, so the boot reconcile is observed.
func bindMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	applyMetricsPtr.Store(&applyMetrics{
		applyDuration: reg.HistogramVec("ze_firewall_apply_duration_seconds",
			"Time spent in Backend.Apply, the serialized firewall reconcile every owner shares, by outcome.",
			applyDurationBuckets, []string{"result"}),
		applyTimeouts: reg.Counter("ze_firewall_apply_timeout_total",
			"Firewall reconciles that failed because the dataplane did not answer within the backend deadline."),
	})
}

// applyResultOf maps an Apply error to its label value.
func applyResultOf(err error) string {
	switch {
	case err == nil:
		return applyResultOK
	case errors.Is(err, ErrKernelTimeout):
		return applyResultTimeout
	default:
		return applyResultError
	}
}

// observeApply records the reconcile latency under its outcome and reports a
// wedged dataplane.
//
// The log sits outside the metrics nil check on purpose: a timeout means the
// dataplane is not enforcing what the registry holds, and that must be visible
// whether or not telemetry is enabled. It lives here rather than in each owner
// because ApplyAll is the only caller of Backend.Apply, so an owner that
// swallows the error cannot make a wedged dataplane silent.
func observeApply(d time.Duration, result string, err error) {
	if m := applyMetricsPtr.Load(); m != nil {
		m.applyDuration.With(result).Observe(d.Seconds())
		if result == applyResultTimeout {
			m.applyTimeouts.Inc()
		}
	}
	if result == applyResultTimeout {
		loggerPtr.Load().Error("firewall: dataplane did not answer within the backend deadline, its ruleset is now behind the registry",
			"error", err, "duration", d)
	}
}
