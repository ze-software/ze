// Design: docs/architecture/anomaly/anomaly-2-shape.md -- responder metrics

package shape

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

type shapeMetrics struct {
	armed      metrics.Gauge
	reverted   metrics.Counter
	armRefused metrics.Counter
	killswitch metrics.Counter
}

var metricsPtr atomic.Pointer[shapeMetrics]

func bindMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	metricsPtr.Store(&shapeMetrics{
		armed:      reg.Gauge("ze_anomaly_shape_armed", "Currently armed live anomaly response actions"),
		reverted:   reg.Counter("ze_anomaly_shape_reverted_total", "Armed anomaly actions withdrawn (auto-revert or clear)"),
		armRefused: reg.Counter("ze_anomaly_shape_arm_refused_total", "Arm attempts refused by the blast-radius cap"),
		killswitch: reg.Counter("ze_anomaly_shape_killswitch_total", "Kill-switch engagements"),
	})
}

func loadMetrics() *shapeMetrics { return metricsPtr.Load() }
