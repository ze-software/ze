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
	// count is the TOTAL of samples ever admitted, and next is the ring index
	// the following sample overwrites once the window is full. They were one
	// field, and restore is where the two disagree: it rebuilds the ring with
	// the OLDEST sample at index 0 while count carries a long run's total, so
	// count%window pointed at an arbitrary mid-age slot and samples retired out
	// of order for a whole window after every restart.
	count    int
	next     int
	p99Cache float64
	// attackRun counts the consecutive samples marked attacking. It drives the
	// slow-adapt admission in admit, and it is transient: a restart starts the
	// adapt clock again rather than resuming a run.
	attackRun int
}

func newBaseline(window int, multiplier, floor float64) *baseline {
	return &baseline{
		window:     window,
		multiplier: multiplier,
		floor:      floor,
		samples:    make([]float64, 0, window),
	}
}

// slowAdaptSamples is how many CONSECUTIVE attacking samples the baseline
// refuses before it admits one. It is the only thing that separates an attack
// from a permanent rise in offered load. One sample cannot tell the two apart,
// and how long the level lasts can.
//
// Refusing every attacking sample is what stops an attack from raising the
// threshold that detects it. With no way back it also latches: a new customer, a
// migrated service, or any lasting traffic shift then holds the detector active
// for good against traffic that is not an attack.
//
// The count is in samples, so wall-clock time scales with check-interval. At the
// default 1 second, one sample is admitted per hour of unbroken above-threshold
// traffic. The p99 of a 300-sample window is its 4th largest sample, so a
// sustained new level moves the threshold after 4 to 13 admissions, which is 4 to
// 13 hours. The spread is the every-10th-sample recalc cadence below. An attack
// shorter than that leaves the threshold where it was.
const slowAdaptSamples = 3600

// Add offers one sample to the rolling window. attacking marks a sample the
// detector does not trust as normal traffic: it is above the threshold, or the
// state machine is in an attack state. Such a sample is refused, apart from the
// one in slowAdaptSamples that admit lets through.
func (b *baseline) Add(pps float64, attacking bool) {
	if !b.admit(attacking) {
		return
	}
	if len(b.samples) < b.window {
		b.samples = append(b.samples, pps)
	} else {
		b.samples[b.next] = pps
		b.next = (b.next + 1) % b.window
	}
	b.count++

	if b.count%10 == 0 {
		b.recalc()
	}
}

// admit reports whether this sample enters the window. A sample that is not
// marked attacking always enters, and it ends the current run, so a condition
// that flaps never accumulates towards the slow adapt. An attacking sample
// enters when its run reaches slowAdaptSamples, and the run then starts again.
func (b *baseline) admit(attacking bool) bool {
	if !attacking {
		b.attackRun = 0
		return true
	}
	b.attackRun++
	if b.attackRun < slowAdaptSamples {
		return false
	}
	b.attackRun = 0
	return true
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
	// The ring was just rebuilt oldest-first, so the next overwrite is index 0
	// whatever total count carries. Deriving the cursor from count here is what
	// retired samples out of order for one window after each restart.
	b.next = 0
	b.recalc()
	return true
}
