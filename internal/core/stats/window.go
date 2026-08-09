// Design: docs/architecture/traffic/traffic-analysis-layers.md -- shared traffic-analysis stats primitives
//
// Package stats holds the domain-NEUTRAL statistical primitives shared by the
// traffic-analysis layers (trafficstat, trafficfeature) and the detection plugins
// (ddos, anomaly). It is a core leaf: pure math and small stateful helpers with no
// component imports, so any tier may depend on it.
package stats

// Window is a per-key rolling byte accumulator: it converts bytes seen during the
// current (not-yet-closed) tick window into a per-second rate, tracks consecutive
// idle ticks so callers can evict quiet keys, and optionally retains a bounded
// history of recent rates. It is the shared primitive underneath trafficstat's
// per-entity aggregation and trafficfeature's per-source state.
//
// Not safe for concurrent use; callers serialize access with their own lock.
type Window struct {
	windowBytes float64 // bytes seen since the last Tick
	lastCumul   float64 // last cumulative counter value (cumulative sources only)
	hasCumul    bool
	bps         float64   // rate computed at the last Tick
	idle        int       // consecutive Ticks with zero windowBytes
	hist        []float64 // recent per-tick rates (newest last), bounded to histCap
	histCap     int
}

// NewWindow returns a Window that retains up to histCap recent rate samples. A
// histCap of 0 disables history (for keys that only need a rate, e.g. ports).
func NewWindow(histCap int) *Window {
	return &Window{histCap: histCap}
}

// AddDelta adds a per-publish byte delta to the current window. Non-positive
// deltas are ignored.
func (w *Window) AddDelta(v float64) {
	if v > 0 {
		w.windowBytes += v
	}
}

// AddCumulative folds a cumulative counter sample into the current window: the
// contribution is (v - lastValue), clamped at 0 across a counter reset. The first
// sample only primes the baseline.
func (w *Window) AddCumulative(v float64) {
	if w.hasCumul && v >= w.lastCumul {
		w.windowBytes += v - w.lastCumul
	}
	w.lastCumul = v
	w.hasCumul = true
}

// Tick closes the current window over dt seconds: it computes the per-second
// rate, resets the window to zero, advances the idle counter (reset on any
// traffic), records the rate in history, and returns the rate. A non-positive dt
// is treated as one second.
func (w *Window) Tick(dt float64) float64 {
	if dt <= 0 {
		dt = 1
	}
	w.bps = w.windowBytes / dt
	if w.windowBytes <= 0 {
		w.idle++
	} else {
		w.idle = 0
	}
	w.windowBytes = 0
	w.pushHistory(w.bps)
	return w.bps
}

// Bps returns the rate computed at the last Tick.
func (w *Window) Bps() float64 { return w.bps }

// Idle returns the number of consecutive Ticks with no traffic.
func (w *Window) Idle() int { return w.idle }

func (w *Window) pushHistory(bps float64) {
	if w.histCap <= 0 {
		return
	}
	if len(w.hist) < w.histCap {
		w.hist = append(w.hist, bps)
		return
	}
	copy(w.hist, w.hist[1:])
	w.hist[len(w.hist)-1] = bps
}

// History returns a defensive copy of the retained rate history (newest last), or
// nil when empty.
func (w *Window) History() []float64 {
	if len(w.hist) == 0 {
		return nil
	}
	out := make([]float64, len(w.hist))
	copy(out, w.hist)
	return out
}
