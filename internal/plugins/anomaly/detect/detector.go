// Design: docs/architecture/anomaly/anomaly-1-detect.md -- behavioral anomaly detector (report-only)
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
	// maxTrackedDestEntities and maxTrackedPortEntities cap the other two axes.
	// Each axis has its OWN ceiling, mirroring the facts layer: a source flood must
	// not be able to evict the baseline of the destination it is aimed at, and a
	// port sweep must not evict either address axis.
	maxTrackedDestEntities = 10000
	maxTrackedPortEntities = 4096
	// incidentRingSize bounds the recent-incident report ring.
	incidentRingSize = 128
	// warmupTicks is how many samples a per-entity baseline must accumulate before
	// self-deviation is scored, so a never-seen entity is not flagged against an
	// empty baseline (its early values seed the baseline first).
	warmupTicks = 3

	// featOutInRatio names one feature in three places: its stddev floor, and the
	// two contribution rows that report it. The name is what the show command and
	// the tests match on, so the three must not drift apart.
	featOutInRatio = "out-in-ratio"
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
	case featOutInRatio:
		return 0.5
	case "port-entropy", "src-entropy":
		// Both are an entropy in bits, on the same scale.
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

	mu sync.Mutex
	// One baseline map per entity axis. They are independent by construction: the
	// same address can be an anomalous sender and an ordinary receiver at once, and
	// each axis has its own cardinality ceiling so neither evicts the other.
	states     map[netip.Addr]*entityState
	destStates map[netip.Addr]*entityState
	portStates map[trafficfeature.PortKey]*entityState
	inc        []anomalyevent.AnomalyDetected // recent-incident ring (newest last)
}

func newDetector(cfg *Config, bus ze.EventBus) *detector {
	return &detector{
		cfg:        cfg,
		bus:        bus,
		alpha:      2.0 / (float64(cfg.BaselineWindow) + 1.0),
		states:     make(map[netip.Addr]*entityState),
		destStates: make(map[netip.Addr]*entityState),
		portStates: make(map[trafficfeature.PortKey]*entityState),
	}
}

// onTick folds one trafficfeature snapshot into the baselines, scores every entity
// on every axis, and drives the confirm/clear state machine. No-op when disabled.
func (d *detector) onTick(snap *trafficfeature.Snapshot) {
	if !d.cfg.Enabled || snap == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	d.scoreSources(snap.Sources)
	d.scoreDests(snap.Dests)
	d.scorePorts(snap.Ports)
	d.publishGauges()
}

// scoreSources runs the pipeline over the SOURCE axis: an entity is measured
// against its own history and against the other senders in its prefix. Caller holds
// d.mu.
func (d *detector) scoreSources(entries []trafficfeature.FeatureEntry) {
	cohorts := d.cohortsOf(entries)
	seen := make(map[netip.Addr]bool, len(entries))
	for i := range entries {
		fe := entries[i]
		seen[fe.Addr] = true
		st := stateFor(d.states, fe.Addr, maxTrackedEntities, d.alpha)
		if st == nil {
			continue // at cardinality cap
		}
		pfx := d.cohortPrefix(fe.Addr)
		v := d.stepEntity(st, sourceSignals(fe), cohorts[pfx])
		d.report(v, identity{kind: anomalyevent.EntityKindSource, entity: hostPrefix(fe.Addr)}, pfx.String())
	}
	evictIdle(d.states, seen)
}

// scoreDests runs the pipeline over the DESTINATION axis. The prefix cohort carries
// over unchanged: a destination's peers are the other destinations in its prefix, so
// a busy server is measured against the other servers beside it rather than against
// the senders. Caller holds d.mu.
func (d *detector) scoreDests(entries []trafficfeature.FeatureEntry) {
	cohorts := d.cohortsOf(entries)
	seen := make(map[netip.Addr]bool, len(entries))
	for i := range entries {
		fe := entries[i]
		seen[fe.Addr] = true
		st := stateFor(d.destStates, fe.Addr, maxTrackedDestEntities, d.alpha)
		if st == nil {
			continue
		}
		pfx := d.cohortPrefix(fe.Addr)
		v := d.stepEntity(st, destSignals(fe), cohorts[pfx])
		d.report(v, identity{kind: anomalyevent.EntityKindDest, entity: hostPrefix(fe.Addr)}, pfx.String())
	}
	evictIdle(d.destStates, seen)
}

// scorePorts runs the pipeline over the destination-PORT axis, COHORT-FREE: a port
// is a number, so it has no prefix and no natural peer group (tcp/443 is not a peer
// of tcp/444). Passing no cohort makes rarity return 0 for every feature, so a port
// is scored purely against its own history -- and the pinned rule in score.go needs
// no change to say so. Caller holds d.mu.
func (d *detector) scorePorts(entries []trafficfeature.PortFeatureEntry) {
	seen := make(map[trafficfeature.PortKey]bool, len(entries))
	for i := range entries {
		pe := entries[i]
		seen[pe.PortKey] = true
		st := stateFor(d.portStates, pe.PortKey, maxTrackedPortEntities, d.alpha)
		if st == nil {
			continue
		}
		v := d.stepEntity(st, portSignals(pe), nil)
		d.report(v, identity{kind: anomalyevent.EntityKindPort, port: pe.Port, proto: pe.Proto}, "")
	}
	evictIdle(d.portStates, seen)
}

// verdict is what one tick decided about one entity.
type verdict struct {
	confirmed bool // the confirm window just completed: open an incident
	ongoing   bool // already active and still above threshold
	cleared   bool // the clear window just completed: close the incident
	score     float64
	fired     []anomalyevent.FeatureSignal
}

// stepEntity folds one tick of one entity's signals into its baseline state and
// returns what that tick decided. It holds the freeze-learn and warmup discipline
// for EVERY axis, in one place: the baselines take this tick only when the entity is
// not anomalous, or while it is still warming up, so a sustained anomaly can never
// drift the baseline up to meet it. Caller holds d.mu.
func (d *detector) stepEntity(st *entityState, sig entitySignals, ca *cohortAgg) verdict {
	fired, updates := d.scoreEntity(sig, st, ca)
	above := len(fired) >= d.cfg.MinFeaturesToCorrelate

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

	v := verdict{
		score: combineScore(zValues(fired), d.cfg.CorroborationWeight),
		fired: fired,
	}
	switch {
	case !st.active && st.above >= d.cfg.ConfirmDuration:
		st.active = true
		v.confirmed = true
	case st.active && above:
		v.ongoing = true
	}
	if st.active && st.below >= d.cfg.ClearConsecutive {
		st.active = false
		v.cleared = true
	}
	return v
}

// report turns one tick's verdict into the incident events it owes. Caller holds
// d.mu.
func (d *detector) report(v verdict, id identity, cohort string) {
	switch {
	case v.confirmed:
		d.activate(id, cohort, v.score, v.fired)
	case v.ongoing:
		d.emitOngoing(id, v.score)
	}
	if v.cleared {
		d.emitCleared(id)
	}
}

// stateFor returns the entity's baseline state in one axis's map, creating it
// (bounded by limit) on first sight. Returns nil at the cap, and the caller then
// skips the entity rather than growing the map.
func stateFor[K comparable](states map[K]*entityState, key K, limit int, alpha float64) *entityState {
	if st, ok := states[key]; ok {
		return st
	}
	if len(states) >= limit {
		return nil
	}
	st := &entityState{
		fanout:  newFeatBaseline(alpha),
		ratio:   newFeatBaseline(alpha),
		entropy: newFeatBaseline(alpha),
		beacon:  newFeatBaseline(alpha),
	}
	states[key] = st
	return st
}

// evictIdle drops the baselines of entities absent for more than evictIdleTicks
// consecutive ticks, bounding one axis's memory under key churn.
func evictIdle[K comparable](states map[K]*entityState, seen map[K]bool) {
	for k, st := range states {
		if seen[k] {
			st.idle = 0
			continue
		}
		st.idle++
		if st.idle > evictIdleTicks {
			delete(states, k)
		}
	}
}

// cohortsOf groups one address axis's entries into prefix cohorts, so an entity is
// measured against its neighbors as well as against its own history. One grouping
// serves both address axes; only the axis whose entries are passed differs.
func (d *detector) cohortsOf(entries []trafficfeature.FeatureEntry) map[netip.Prefix]*cohortAgg {
	cohorts := make(map[netip.Prefix]*cohortAgg)
	for i := range entries {
		fe := entries[i]
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

// entitySignals is the axis-neutral view of one entity's scored features, so one
// scoring pipeline serves all three axes. The facts layer emits a FeatureEntry for
// an address and a PortFeatureEntry for a port; each converts to this.
//
// Four continuous features and two binary ones, exactly as before. Where a feature's
// MEANING changes with the axis, the name travels with the value, because that name
// is what an operator reads back in an incident's fired features.
type entitySignals struct {
	fanOut     float64
	ratio      float64
	spread     float64 // an entropy, in bits
	spreadName string
	beacon     float64
	fresh      bool // first seen recently
	freshName  string
	rarePort   bool
	// ratioMaxOnEmpty is this axis's policy for the one case an asymmetry ratio
	// cannot express: nothing at all came back, so the facts layer reported +Inf.
	//
	// True on the SOURCE axis, where sending with nothing coming back is
	// exfiltration and a finding on its own: the ratio scores at the ceiling and
	// never enters the baseline. False everywhere else, where that tick's ratio is
	// simply not measurable, so the feature is dropped for the tick -- a receiver
	// that does not answer is ordinary for a sink, and flagging it would flag every
	// quiet collector on the network.
	ratioMaxOnEmpty bool
}

// sourceSignals views a sending entity: how many destinations it reached, how
// lopsided its traffic is outbound, how spread its destination ports are.
func sourceSignals(fe trafficfeature.FeatureEntry) entitySignals {
	return entitySignals{
		fanOut:          float64(fe.FanOut),
		ratio:           fe.OutInRatio,
		spread:          fe.PortEntropy,
		spreadName:      "port-entropy",
		beacon:          fe.Beaconing,
		fresh:           fe.NewPeer,
		freshName:       "new-peer",
		rarePort:        fe.RarePort,
		ratioMaxOnEmpty: true,
	}
}

// destSignals views a receiving entity: how many sources reached it (fan-in), how
// lopsided its traffic is inbound, how spread the ports it was addressed on are.
// The feature names are the source axis's, read in the receiving direction.
func destSignals(fe trafficfeature.FeatureEntry) entitySignals {
	return entitySignals{
		fanOut:     float64(fe.FanOut),
		ratio:      fe.OutInRatio,
		spread:     fe.PortEntropy,
		spreadName: "port-entropy",
		beacon:     fe.Beaconing,
		fresh:      fe.NewPeer,
		freshName:  "new-peer",
		// Read in the receiving direction, RarePort says the port this entity was
		// ADDRESSED on is outside the well-known allowlist. For any client that is
		// a destination only because a server answered it, that port is ephemeral,
		// so the feature is true for the entity's whole life and reports nothing
		// that CHANGED. Firing it handed every ordinary client half the correlation
		// gate, and new-peer supplied the other half for its first newPeerTicks, so
		// a receiver that behaved exactly like its neighbors drew an incident.
		// Dropped for the same reason portSignals drops it, and the dest axis keeps
		// fan-in, ratio, port-entropy and beaconing to find a real sweep.
		rarePort: false,
	}
}

// portSignals views a service-port entity: how many sources sent to it, how much it
// answers relative to what it is asked (amplification), how evenly those sources
// share the traffic.
func portSignals(pe trafficfeature.PortFeatureEntry) entitySignals {
	return entitySignals{
		fanOut:     float64(pe.FanOut),
		ratio:      pe.OutInRatio,
		spread:     pe.SrcEntropy,
		spreadName: "src-entropy",
		beacon:     pe.Beaconing,
		fresh:      pe.NewPort,
		freshName:  "new-port",
		// PortFeatureEntry.RarePort is a property of the KEY, true for the entity's
		// whole life, so it is evidence that nothing CHANGED. Firing it would hand
		// every high-numbered service half of the correlation gate for free. It
		// stays on the fact for a reader and off the score.
		rarePort: false,
	}
}

// scoreEntity computes the fired feature signals for one entity: continuous
// features take max(self-deviation, cohort-rarity); binary features fire at the
// threshold. It does NOT mutate the baselines -- it returns the per-feature updates
// so the caller can fold them only when the entity is not anomalous this tick
// (freeze-learn). Self-deviation is suppressed until the baseline has warmed up.
//
// A nil cohort scores the entity on self-deviation alone: every rarity reads 0
// because there is no cohort to be rare in. That is how the port axis is scored,
// with no change to the pinned rule.
func (d *detector) scoreEntity(sig entitySignals, st *entityState, ca *cohortAgg) ([]anomalyevent.FeatureSignal, []baselineUpdate) {
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
	cont("fan-out", sig.fanOut, st.fanout, fanout, false)
	// The ratio has one unmeasurable case: +Inf, nothing came back at all.
	// ratioMaxOnEmpty carries the axis's policy for it. Where the axis does not read
	// it as a finding, the feature is dropped for this tick -- no signal, and nothing
	// folded into the baseline, so the next two-way tick measures it honestly. The
	// forceMax branch also keeps the value away from the cohort it was deliberately
	// left out of (cohortsOf), and needs nothing else to: zMax exceeds any rarity.
	if !math.IsInf(sig.ratio, 1) {
		cont(featOutInRatio, sig.ratio, st.ratio, ratio, false)
	} else if sig.ratioMaxOnEmpty {
		cont(featOutInRatio, finiteRatio(sig.ratio), st.ratio, ratio, true)
	}
	cont(sig.spreadName, sig.spread, st.entropy, entropy, false)
	cont("beaconing", sig.beacon, st.beacon, beacon, false)

	// Binary features contribute exactly one unit of evidence (threshold) so they
	// satisfy the correlation gate but never dominate a continuous signal.
	if sig.fresh {
		fired = append(fired, anomalyevent.FeatureSignal{Name: sig.freshName, Z: thr})
	}
	if sig.rarePort {
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

// identity is one incident's subject: which axis it sits on, and the value that
// names it there. An address axis fills entity; the port axis fills port and proto
// and leaves entity zero, because a port is not an address.
type identity struct {
	kind   anomalyevent.EntityKind
	entity netip.Prefix
	port   uint16
	proto  uint8
}

// hostPrefix names one address as the single-host prefix the event contract carries.
func hostPrefix(addr netip.Addr) netip.Prefix {
	return netip.PrefixFrom(addr, addr.BitLen())
}

func (d *detector) activate(id identity, cohort string, score float64, fired []anomalyevent.FeatureSignal) {
	ev := anomalyevent.AnomalyDetected{
		EntityKind:    id.kind,
		Entity:        id.entity,
		Port:          id.port,
		Proto:         id.proto,
		Cohort:        cohort,
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

func (d *detector) emitOngoing(id identity, score float64) {
	if d.bus == nil {
		return
	}
	if _, err := anomalyevent.Ongoing.Emit(d.bus, &anomalyevent.AnomalyOngoing{
		EntityKind: id.kind, Entity: id.entity, Port: id.port, Proto: id.proto,
		Score: score, Observable: true,
	}); err != nil {
		slog.Default().Warn("anomaly-detect: emit ongoing failed", "error", err)
	}
}

func (d *detector) emitCleared(id identity) {
	if d.bus == nil {
		return
	}
	if _, err := anomalyevent.Cleared.Emit(d.bus, &anomalyevent.AnomalyCleared{
		EntityKind: id.kind, Entity: id.entity, Port: id.port, Proto: id.proto,
		Observable: true,
	}); err != nil {
		slog.Default().Warn("anomaly-detect: emit cleared failed", "error", err)
	}
}

func (d *detector) publishGauges() {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.active.Set(float64(activeIn(d.states) + activeIn(d.destStates) + activeIn(d.portStates)))
	// One gauge per axis, so an operator sees WHICH map is growing rather than a
	// single number three axes contribute to.
	m.tracked.With(anomalyevent.EntityKindSource.String()).Set(float64(len(d.states)))
	m.tracked.With(anomalyevent.EntityKindDest.String()).Set(float64(len(d.destStates)))
	m.tracked.With(anomalyevent.EntityKindPort.String()).Set(float64(len(d.portStates)))
}

// activeIn counts one axis's entities with an open incident.
func activeIn[K comparable](states map[K]*entityState) int {
	var n int
	for _, st := range states {
		if st.active {
			n++
		}
	}
	return n
}

// recentIncidents returns a copy of the recent-incident ring (newest last).
func (d *detector) recentIncidents() []anomalyevent.AnomalyDetected {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]anomalyevent.AnomalyDetected, len(d.inc))
	copy(out, d.inc)
	return out
}

// activeCount reports open incidents across every axis.
func (d *detector) activeCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return activeIn(d.states) + activeIn(d.destStates) + activeIn(d.portStates)
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
// from its cohort's ratio baseline (cohortsOf) and scores via self-deviation
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
	// tracked is labeled by entity axis (source, dest, port). Each axis has its own
	// ceiling, so one summed number could not say which map is filling up.
	tracked metrics.GaugeVec
}

var metricsPtr atomic.Pointer[detectorMetrics]

func bindMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	metricsPtr.Store(&detectorMetrics{
		incidents: reg.Counter("ze_anomaly_incidents_total", "Behavioral anomaly incidents confirmed"),
		active:    reg.Gauge("ze_anomaly_active", "Currently active anomaly incidents"),
		tracked: reg.GaugeVec("ze_anomaly_tracked_entities",
			"Entities with a live behavioral baseline, by entity axis", []string{"dimension"}),
	})
}
