// Design: docs/architecture/traffic/traffic-analysis-layers.md -- exponentially weighted moving average

package stats

// EWMA is an exponentially weighted moving average with smoothing factor alpha in
// (0,1]: a higher alpha weights recent samples more. Not safe for concurrent use.
type EWMA struct {
	alpha float64
	value float64
	ready bool
}

// NewEWMA returns an EWMA with the given smoothing factor. alpha outside (0,1]
// defaults to 0.5.
func NewEWMA(alpha float64) *EWMA {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.5
	}
	return &EWMA{alpha: alpha}
}

// Add folds x into the average. The first sample seeds the value exactly.
func (e *EWMA) Add(x float64) {
	if !e.ready {
		e.value = x
		e.ready = true
		return
	}
	e.value = e.alpha*x + (1-e.alpha)*e.value
}

// Value returns the current average (0 before the first Add).
func (e *EWMA) Value() float64 { return e.value }

// Ready reports whether at least one sample has been added.
func (e *EWMA) Ready() bool { return e.ready }
