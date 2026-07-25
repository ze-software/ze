// Design: plan/learned/1048-anomaly-1-detect.md -- behavioral anomaly detector (report-only)
//
// Consumes trafficfeature.Snapshot each tick, maintains a per-(entity,feature)
// EWMA baseline, scores self-deviation + cohort rarity via the pinned rule, runs
// a confirm/clear state machine, and EMITS anomalyevent incidents plus a bounded
// recent-incident ring. It takes NO action -- no firewall, no dispatch (D5).

package detect

import (
	"log/slog"
	"math"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/trafficfeature"
	"github.com/ze-software/ze/internal/core/anomalyevent"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/stats"
	"github.com/ze-software/ze/pkg/ze"
)

const (
	// evictIdleTicks drops a per-entity baseline after this many ticks with no
	// traffic for that entity, bounding memory under source churn.
	evictIdleTicks = 10
	// maxTrackedEntities caps distinct per-entity baselines (memory ceiling).
	maxTrackedEntities = 10000
	// incidentRingSize bounds the recent-incident report ring.
	incidentRingSize = 128
	// warmupTicks is how many samples a per-entity baseline must accumulate before
	// self-deviation is scored, so a never-seen entity is not flagged against an
	// empty baseline (its early values seed the baseline first).
	warmupTicks = 3
)

// baselineUpdate defers folding a value into a feature baseline so the caller can
// apply it only when the entity is NOT anomalous this tick (freeze-learn).
type baselineUpdate struct {
	base featBaseline
	val  float64
}

// per-feature stddev floors keep the z-score meaningful across the differing
// scales of each feature (avoids divide-by-tiny-variance).
func floorFor(name string) float64 {
	switch name {
	case "fan-out":
		return 1.0
	case "out-in-ratio":
		return 0.5
	case "port-entropy":
		return 0.2
	default: // beaconing
		return 0.1
	}
}

// featBaseline is a per-(entity,feature) EWMA of the value and of its square, from
// which a running mean and standard deviation are derived.
type featBaseline struct {
	mean *stats.EWMA
	sq   *stats.EWMA
}

func newFeatBaseline(alpha float64) featBaseline {
	return featBaseline{mean: stats.NewEWMA(alpha), sq: stats.NewEWMA(alpha)}
}

func (b featBaseline) update(x float64) {
	b.mean.Add(x)
	b.sq.Add(x * x)
}

func (b featBaseline) stddev() float64 {
	m := b.mean.Value()
	v := b.sq.Value() - m*m
	if v < 0 {
		v = 0
	}
	return math.Sqrt(v)
}

type entityState struct {
	fanout, ratio, entropy, beacon featBaseline
	samples                        int // ticks scored (warmup gate for self-deviation)
	idle                           int
	above                          int
	below                          int
	active                         bool
}

type cohortAgg struct {
	fanout, ratio, entropy, beacon cohortStats
}

type detector struct {
	cfg   *Config
	bus   ze.EventBus
	alpha float64

	mu     sync.Mutex
	states map[netip.Addr]*entityState
	inc    []anomalyevent.AnomalyDetected // recent-incident ring (newest last)
}

func newDetector(cfg *Config, bus ze.EventBus) *detector {
	return &detector{
		cfg:    cfg,
		bus:    bus,
		alpha:  2.0 / (float64(cfg.BaselineWindow) + 1.0),
		states: make(map[netip.Addr]*entityState),
	}
}

// onTick folds one trafficfeature snapshot into the baselines, scores every
// entity, and drives the confirm/clear state machine. No-op when disabled.
func (d *detector) onTick(snap *trafficfeature.Snapshot) {
	if !d.cfg.Enabled || snap == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	cohorts := d.buildCohorts(snap)
	seen := make(map[netip.Addr]bool, len(snap.Sources))

	for i := range snap.Sources {
		fe := snap.Sources[i]
		seen[fe.Addr] = true
		st := d.stateFor(fe.Addr)
		if st == nil {
			continue // at cardinality cap
		}
		fired, updates := d.scoreEntity(fe, st, cohorts[d.cohortPrefix(fe.Addr)])
		score := combineScore(zValues(fired), d.cfg.CorroborationWeight)
		above := len(fired) >= d.cfg.MinFeaturesToCorrelate

		// Freeze-learn: fold this tick into the baselines only when the entity is
		// NOT anomalous, or while still warming up. A sustained anomaly therefore
		// cannot drift the entity's own baseline up until it looks normal again.
		if !above || st.samples < warmupTicks {
			for _, u := range updates {
				u.base.update(u.val)
			}
		}
		st.samples++

		if above {
			st.above++
			st.below = 0
		} else {
			st.below++
			st.above = 0
		}

		switch {
		case !st.active && st.above >= d.cfg.ConfirmDuration:
			st.active = true
			d.activate(fe.Addr, score, fired)
		case st.active && above:
			d.emitOngoing(fe.Addr, score)
		}
		if st.active && st.below >= d.cfg.ClearConsecutive {
			st.active = false
			d.emitCleared(fe.Addr)
		}
	}

	for addr, st := range d.states {
		if seen[addr] {
			st.idle = 0
			continue
		}
		st.idle++
		if st.idle > evictIdleTicks {
			delete(d.states, addr)
		}
	}
	d.publishGauges()
}

func (d *detector) buildCohorts(snap *trafficfeature.Snapshot) map[netip.Prefix]*cohortAgg {
	cohorts := make(map[netip.Prefix]*cohortAgg)
	for i := range snap.Sources {
		fe := snap.Sources[i]
		pfx := d.cohortPrefix(fe.Addr)
		ca := cohorts[pfx]
		if ca == nil {
			ca = &cohortAgg{}
			cohorts[pfx] = ca
		}
		ca.fanout.add(float64(fe.FanOut))
		// Exclude an infinite ratio (pure sender / exfil) from the cohort baseline:
		// it is flagged via self-deviation and would otherwise dominate the cohort
		// statistics and mask its peers' milder ratio elevations.
		if !math.IsInf(fe.OutInRatio, 1) {
			ca.ratio.add(fe.OutInRatio)
		}
		ca.entropy.add(fe.PortEntropy)
		ca.beacon.add(fe.Beaconing)
	}
	return cohorts
}

// scoreEntity computes the fired feature signals for one entity: continuous
// features take max(self-deviation, cohort-rarity); binary features fire at the
// threshold. It does NOT mutate the baselines -- it returns the per-feature updates
// so the caller can fold them only when the entity is not anomalous this tick
// (freeze-learn). Self-deviation is suppressed until the baseline has warmed up.
func (d *detector) scoreEntity(fe trafficfeature.FeatureEntry, st *entityState, ca *cohortAgg) ([]anomalyevent.FeatureSignal, []baselineUpdate) {
	var fired []anomalyevent.FeatureSignal
	var updates []baselineUpdate
	thr := d.cfg.DeviationThreshold
	warmed := st.samples >= warmupTicks

	cont := func(name string, val float64, base featBaseline, cohort cohortStats, forceMax bool) {
		var self float64
		if forceMax {
			// An infinite ratio is scored at max and never folded into the baseline.
			self = zMax
		} else {
			if warmed {
				self = zScore(val, base.mean.Value(), base.stddev(), floorFor(name))
			}
			updates = append(updates, baselineUpdate{base: base, val: val})
		}
		z := math.Max(self, cohort.rarity(val, d.cfg.MinCohortSize, floorFor(name)))
		if z >= thr {
			fired = append(fired, anomalyevent.FeatureSignal{Name: name, Z: z})
		}
	}

	var fanout, ratio, entropy, beacon cohortStats
	if ca != nil {
		fanout, ratio, entropy, beacon = ca.fanout, ca.ratio, ca.entropy, ca.beacon
	}
	cont("fan-out", float64(fe.FanOut), st.fanout, fanout, false)
	cont("out-in-ratio", finiteRatio(fe.OutInRatio), st.ratio, ratio, math.IsInf(fe.OutInRatio, 1))
	cont("port-entropy", fe.PortEntropy, st.entropy, entropy, false)
	cont("beaconing", fe.Beaconing, st.beacon, beacon, false)

	// Binary features contribute exactly one unit of evidence (threshold) so they
	// satisfy the correlation gate but never dominate a continuous signal.
	if fe.NewPeer {
		fired = append(fired, anomalyevent.FeatureSignal{Name: "new-peer", Z: thr})
	}
	if fe.RarePort {
		fired = append(fired, anomalyevent.FeatureSignal{Name: "rare-port", Z: thr})
	}
	return fired, updates
}

func (d *detector) cohortPrefix(addr netip.Addr) netip.Prefix {
	bits := d.cfg.CohortPrefixLenV4
	if addr.Is6() {
		bits = d.cfg.CohortPrefixLenV6
	}
	p, err := addr.Prefix(bits)
	if err != nil {
		return netip.PrefixFrom(addr, addr.BitLen())
	}
	return p
}

// stateFor returns the entity's baseline state, creating it (bounded by
// maxTrackedEntities) on first sight. Returns nil at the cap.
func (d *detector) stateFor(addr netip.Addr) *entityState {
	if st, ok := d.states[addr]; ok {
		return st
	}
	if len(d.states) >= maxTrackedEntities {
		return nil
	}
	st := &entityState{
		fanout:  newFeatBaseline(d.alpha),
		ratio:   newFeatBaseline(d.alpha),
		entropy: newFeatBaseline(d.alpha),
		beacon:  newFeatBaseline(d.alpha),
	}
	d.states[addr] = st
	return st
}

func (d *detector) activate(addr netip.Addr, score float64, fired []anomalyevent.FeatureSignal) {
	ev := anomalyevent.AnomalyDetected{
		Entity:        netip.PrefixFrom(addr, addr.BitLen()),
		Cohort:        d.cohortPrefix(addr).String(),
		FiredFeatures: fired,
		Score:         score,
		Severity:      anomalyevent.GradeSeverity(score, d.cfg.DeviationThreshold),
		At:            time.Now(),
		Observable:    true,
	}
	d.inc = append(d.inc, ev)
	if len(d.inc) > incidentRingSize {
		d.inc = d.inc[len(d.inc)-incidentRingSize:]
	}
	if m := metricsPtr.Load(); m != nil {
		m.incidents.Inc()
	}
	if d.bus != nil {
		if _, err := anomalyevent.Detected.Emit(d.bus, &ev); err != nil {
			slog.Default().Warn("anomaly-detect: emit detected failed", "error", err)
		}
	}
}

func (d *detector) emitOngoing(addr netip.Addr, score float64) {
	if d.bus == nil {
		return
	}
	if _, err := anomalyevent.Ongoing.Emit(d.bus, &anomalyevent.AnomalyOngoing{
		Entity: netip.PrefixFrom(addr, addr.BitLen()), Score: score, Observable: true,
	}); err != nil {
		slog.Default().Warn("anomaly-detect: emit ongoing failed", "error", err)
	}
}

func (d *detector) emitCleared(addr netip.Addr) {
	if d.bus == nil {
		return
	}
	if _, err := anomalyevent.Cleared.Emit(d.bus, &anomalyevent.AnomalyCleared{
		Entity: netip.PrefixFrom(addr, addr.BitLen()), Observable: true,
	}); err != nil {
		slog.Default().Warn("anomaly-detect: emit cleared failed", "error", err)
	}
}

func (d *detector) publishGauges() {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	var active int
	for _, st := range d.states {
		if st.active {
			active++
		}
	}
	m.active.Set(float64(active))
	m.tracked.Set(float64(len(d.states)))
}

// recentIncidents returns a copy of the recent-incident ring (newest last).
func (d *detector) recentIncidents() []anomalyevent.AnomalyDetected {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]anomalyevent.AnomalyDetected, len(d.inc))
	copy(out, d.inc)
	return out
}

func (d *detector) activeCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	var n int
	for _, st := range d.states {
		if st.active {
			n++
		}
	}
	return n
}

func zValues(sigs []anomalyevent.FeatureSignal) []float64 {
	out := make([]float64, len(sigs))
	for i, s := range sigs {
		out[i] = s.Z
	}
	return out
}

// finiteRatio maps the +Inf exfil sentinel to a large finite value so it never
// propagates NaN/Inf through the scoring arithmetic. A +Inf entity is excluded
// from its cohort's ratio baseline (buildCohorts) and scores via self-deviation
// (forceMax), so this value only ever appears in that entity's own harmless
// leave-one-out rarity.
func finiteRatio(r float64) float64 {
	if math.IsInf(r, 1) {
		return 1e6
	}
	return r
}

// --- process-global detector for the show handler (plugins run in-process) ---

var globalDetector atomic.Pointer[detector]

func setGlobalDetector(d *detector) { globalDetector.Store(d) }

func loadGlobalDetector() *detector { return globalDetector.Load() }

// --- metrics ---

type detectorMetrics struct {
	incidents metrics.Counter
	active    metrics.Gauge
	tracked   metrics.Gauge
}

var metricsPtr atomic.Pointer[detectorMetrics]

func bindMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	metricsPtr.Store(&detectorMetrics{
		incidents: reg.Counter("ze_anomaly_incidents_total", "Behavioral anomaly incidents confirmed"),
		active:    reg.Gauge("ze_anomaly_active", "Currently active anomaly incidents"),
		tracked:   reg.Gauge("ze_anomaly_tracked_entities", "Entities with a live behavioral baseline"),
	})
}
