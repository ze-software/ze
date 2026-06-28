// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- rolling baseline with poisoning guards

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
