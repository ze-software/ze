// Design: docs/architecture/irr-owner.md -- the owner's refresh metrics
// Related: owner.go -- refreshSet and run, the two writers
//
// The owner is the only IRR fetcher, so the IRR refresh metrics are its own:
// bgp-filter-irr, which fetched before it, registers none any more.

package irr

import (
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
)

// The outcomes ze_irr_refresh_outcomes_total counts, one per fetch.
const (
	outcomeStored      = "stored"       // the answer was stored and announced
	outcomeEmpty       = "empty"        // the server answered no prefix; the stored list stays, marked stale
	outcomeError       = "error"        // the server did not answer; the stored copy stays
	outcomeStoreFailed = "store-failed" // the answer was not stored; the stored copy stays
	outcomePanic       = "panic"        // the fetch panicked and was recovered; the stored copy stays
)

type ownerMetrics struct {
	refreshOutcomes metrics.CounterVec
	dataAge         metrics.Gauge
}

var ownerMetricsPtr atomic.Pointer[ownerMetrics]

// setMetricsRegistry registers the owner's metrics. The registry calls it
// once, through Registration.ConfigureMetrics.
func setMetricsRegistry(reg metrics.Registry) {
	ownerMetricsPtr.Store(&ownerMetrics{
		refreshOutcomes: reg.CounterVec("ze_irr_refresh_outcomes_total",
			"IRR fetch outcomes of the irr plugin: stored, empty, error, store-failed, panic.", []string{"result"}),
		dataAge: reg.Gauge("ze_irr_data_age_seconds",
			"Age in seconds of the oldest IRR prefix data stored for the configured server, as of the irr plugin's last pass."),
	})
}

// countOutcome counts one fetch outcome.
func countOutcome(result string) {
	if m := ownerMetricsPtr.Load(); m != nil {
		m.refreshOutcomes.With(result).Inc()
	}
}

// publishMetrics sets the data age from the datasets of the configured server.
// It runs on the worker only, after every pass; with nothing stored the age
// is 0.
func (o *owner) publishMetrics() {
	m := ownerMetricsPtr.Load()
	if m == nil {
		return
	}
	var oldest time.Time
	for _, info := range o.datasets {
		if info.server != o.serverKey || info.refreshedAt.IsZero() {
			continue
		}
		if oldest.IsZero() || info.refreshedAt.Before(oldest) {
			oldest = info.refreshedAt
		}
	}
	if oldest.IsZero() {
		m.dataAge.Set(0)
		return
	}
	m.dataAge.Set(o.clock().Sub(oldest).Seconds())
}
