// Design: docs/guide/monitoring.md -- selected-output resource bounds.
// Related: rib.go -- the existing ten-second metrics worker owns gauge updates.
package rib

const (
	selectedBatchCompleted = "completed"
	selectedBatchFailed    = "failed"
)

func (r *RIBManager) updateSelectedMetrics(m *ribMetrics) {
	var current, destinations, pending, inFlight, failed int
	if d := r.selectedDispatch; d != nil {
		d.mu.Lock()
		current, destinations = len(d.current), len(d.destinations)
		for _, destination := range d.destinations {
			pending += len(destination.pending)
			inFlight += len(destination.inFlight)
			if destination.failure != nil {
				failed++
			}
		}
		d.mu.Unlock()
	}
	m.selectedCurrent.Set(float64(current))
	m.selectedDestinations.Set(float64(destinations))
	m.selectedPending.Set(float64(pending))
	m.selectedInFlight.Set(float64(inFlight))
	m.selectedFailures.Set(float64(failed))
}

func recordSelectedBatch(err error) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	outcome := selectedBatchCompleted
	if err != nil {
		outcome = selectedBatchFailed
	}
	m.selectedBatches.With(outcome).Inc()
}
