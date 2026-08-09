// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- rolling baseline with poisoning guards

package detect

import (
	"math"
	"sort"
)

type baseline struct {
	window     int
	multiplier float64
	floor      float64
	samples    []float64
	count      int
	p99Cache   float64
}

func newBaseline(window int, multiplier, floor float64) *baseline {
	return &baseline{
		window:     window,
		multiplier: multiplier,
		floor:      floor,
		samples:    make([]float64, 0, window),
	}
}

func (b *baseline) Add(pps float64, attacking bool) {
	if attacking {
		return
	}
	if len(b.samples) < b.window {
		b.samples = append(b.samples, pps)
	} else {
		b.samples[b.count%b.window] = pps
	}
	b.count++

	if b.count%10 == 0 {
		b.recalc()
	}
}

func (b *baseline) recalc() {
	if len(b.samples) == 0 {
		b.p99Cache = 0
		return
	}
	sorted := make([]float64, len(b.samples))
	copy(sorted, b.samples)
	sort.Float64s(sorted)
	idx := min(max(int(math.Ceil(float64(len(sorted))*0.99))-1, 0), len(sorted)-1)
	b.p99Cache = sorted[idx]
}

func (b *baseline) P99() float64 {
	return b.p99Cache
}

func (b *baseline) Threshold() float64 {
	return math.Max(b.p99Cache*b.multiplier, b.floor)
}

func (b *baseline) Ready() bool {
	return len(b.samples) >= b.window
}

// minRestoreSamples is the fewest samples a persisted baseline must carry (capped
// at the window) before a restore is trusted. Below this the detector warms fresh
// rather than detecting against a barely-warmed baseline.
const minRestoreSamples = 50

// baselineState is the serialisable snapshot of a rolling baseline: the raw
// samples plus the derived running index and cached p99. window/multiplier/floor
// are NOT persisted -- they come from live config, so a config change is honored
// on restore.
type baselineState struct {
	Samples  []float64 `json:"samples"`
	Count    int       `json:"count"`
	P99Cache float64   `json:"p99"`
}

// snapshot copies the baseline's mutable state for persistence.
func (b *baseline) snapshot() baselineState {
	s := make([]float64, len(b.samples))
	copy(s, b.samples)
	return baselineState{Samples: s, Count: b.count, P99Cache: b.p99Cache}
}

// restore loads a persisted snapshot, returning true only when it is trustworthy:
// enough samples (>= min(minRestoreSamples, window)) and no NaN/Inf/negative values.
// Samples beyond the current window are dropped (keeping the most recent), so a
// shrunk baseline-window is honored. A rejected restore leaves the baseline empty
// to warm fresh.
func (b *baseline) restore(st baselineState) bool {
	if len(st.Samples) < min(minRestoreSamples, b.window) {
		return false
	}
	for _, v := range st.Samples {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return false
		}
	}
	if math.IsNaN(st.P99Cache) || math.IsInf(st.P99Cache, 0) || st.P99Cache < 0 {
		return false
	}
	samples := st.Samples
	if len(samples) > b.window {
		samples = samples[len(samples)-b.window:]
	}
	b.samples = append(b.samples[:0], samples...)
	b.count = max(st.Count, len(b.samples))
	b.recalc()
	return true
}
