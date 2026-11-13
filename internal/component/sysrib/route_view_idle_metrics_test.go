// Design: docs/plugin-development/metrics.md -- meaningful selected-input counts.

package sysrib

import (
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

func TestRouteViewEmptySelectedPollDoesNotCountInput(t *testing.T) {
	previous := sysribMetricsPtr.Load()
	t.Cleanup(func() { sysribMetricsPtr.Store(previous) })
	registry := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(registry)
	state := newSysRIB()
	if batches, err := state.applySelectedBatch(nil); err != nil || len(batches) != 0 {
		t.Fatalf("empty selected idle poll produced forwarding: batches=%+v error=%v", batches, err)
	}
	assertRouteViewMetric(t, registry, "ze_systemrib_events_received_total 0")
}
