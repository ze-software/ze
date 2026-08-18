// VALIDATES: AC-5/AC-6 confirm/clear lifecycle + emit, AC-7 the recent-incident
// ring, AC-8 disabled = zero work, AC-9 bounded per-entity baseline with idle
// eviction. Drives onTick directly with crafted trafficfeature snapshots.
// PREVENTS: an incident firing before ConfirmDuration, never clearing, unbounded
// baseline growth under source churn, and any work while disabled.

package detect

import (
	"math"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/trafficfeature"
	"github.com/ze-software/ze/internal/core/anomalyevent"
	"github.com/ze-software/ze/internal/core/metrics"
)

func snapOf(entries ...trafficfeature.FeatureEntry) *trafficfeature.Snapshot {
	return &trafficfeature.Snapshot{Sources: entries}
}

func destSnapOf(entries ...trafficfeature.FeatureEntry) *trafficfeature.Snapshot {
	return &trafficfeature.Snapshot{Dests: entries}
}

func portSnapOf(entries ...trafficfeature.PortFeatureEntry) *trafficfeature.Snapshot {
	return &trafficfeature.Snapshot{Ports: entries}
}

// normalPort is a port entity behaving as it always has: one source, no reply
// amplification, no spread.
func normalPort(port uint16) trafficfeature.PortFeatureEntry {
	return trafficfeature.PortFeatureEntry{
		PortKey: trafficfeature.PortKey{Port: port, Proto: 17},
		FanOut:  1, OutInRatio: 1, SrcEntropy: 0.1,
	}
}

// spikePort is the same port under a distributed sweep: many sources, evenly
// spread, answering far more than it is asked.
func spikePort(port uint16) trafficfeature.PortFeatureEntry {
	return trafficfeature.PortFeatureEntry{
		PortKey: trafficfeature.PortKey{Port: port, Proto: 17},
		FanOut:  300, OutInRatio: 80, SrcEntropy: 0.1,
	}
}

func normalEntry(addr string) trafficfeature.FeatureEntry {
	return trafficfeature.FeatureEntry{
		Addr: netip.MustParseAddr(addr), FanOut: 1, OutInRatio: 1, PortEntropy: 0.1,
	}
}

func spikeEntry(addr string) trafficfeature.FeatureEntry {
	// high fan-out + high out/in ratio: two continuous features deviate hard.
	return trafficfeature.FeatureEntry{
		Addr: netip.MustParseAddr(addr), FanOut: 200, OutInRatio: 500, PortEntropy: 0.1,
	}
}

func testConfig() *Config {
	c := DefaultConfig()
	c.Enabled = true
	c.DeviationThreshold = 3
	c.MinFeaturesToCorrelate = 2
	c.ConfirmDuration = 2
	c.ClearConsecutive = 2
	c.MinCohortSize = 1000 // effectively disable cohort rarity; self-deviation drives
	return c
}

func TestConfirmClearLifecycle(t *testing.T) {
	d := newDetector(testConfig(), nil)
	src := "198.51.100.42"

	// Seed the baseline with several normal ticks (no incident).
	for range 5 {
		d.onTick(snapOf(normalEntry(src)))
	}
	if got := len(d.recentIncidents()); got != 0 {
		t.Fatalf("incidents after normal traffic = %d, want 0", got)
	}

	// One spike tick: fires but not yet confirmed (ConfirmDuration=2).
	d.onTick(snapOf(spikeEntry(src)))
	if got := len(d.recentIncidents()); got != 0 {
		t.Fatalf("incident after 1 spike = %d, want 0 (needs %d)", got, d.cfg.ConfirmDuration)
	}
	// Second consecutive spike: confirms -> one incident.
	d.onTick(snapOf(spikeEntry(src)))
	if got := len(d.recentIncidents()); got != 1 {
		t.Fatalf("incident after ConfirmDuration spikes = %d, want 1", got)
	}
	inc := d.recentIncidents()[0]
	if len(inc.FiredFeatures) < 2 {
		t.Errorf("incident fired %d features, want >= 2 (correlation)", len(inc.FiredFeatures))
	}
	if !inc.Entity.IsValid() {
		t.Error("incident entity must be a valid source prefix")
	}

	// Return to normal: clears after ClearConsecutive below-threshold ticks.
	for range d.cfg.ClearConsecutive {
		d.onTick(snapOf(normalEntry(src)))
	}
	if d.activeCount() != 0 {
		t.Errorf("active incidents after clear window = %d, want 0", d.activeCount())
	}
}

func TestDisabledNoop(t *testing.T) {
	cfg := testConfig()
	cfg.Enabled = false
	d := newDetector(cfg, nil)
	for range 5 {
		d.onTick(snapOf(spikeEntry("203.0.113.7")))
	}
	if len(d.states) != 0 || len(d.recentIncidents()) != 0 {
		t.Errorf("disabled detector did work: states=%d incidents=%d", len(d.states), len(d.recentIncidents()))
	}
}

func TestBaselineCapsAndEviction(t *testing.T) {
	d := newDetector(testConfig(), nil)
	src := netip.MustParseAddr("192.0.2.50")
	d.onTick(snapOf(trafficfeature.FeatureEntry{Addr: src, FanOut: 1, OutInRatio: 1}))
	if _, ok := d.states[src]; !ok {
		t.Fatal("entity state not created")
	}
	// Idle it out: many ticks with no traffic for this source.
	for range evictIdleTicks + 2 {
		d.onTick(snapOf())
	}
	if _, ok := d.states[src]; ok {
		t.Errorf("idle entity not evicted after %d ticks", evictIdleTicks)
	}
}

// TestBuildCohortsExcludesInfiniteRatio proves an exfiltrator (out-only, +Inf
// ratio) is left OUT of its cohort's ratio baseline so it cannot inflate the
// cohort statistics and mask its peers' milder ratio elevations (review ISSUE 1).
// It is still counted for the other features.
func TestBuildCohortsExcludesInfiniteRatio(t *testing.T) {
	d := newDetector(testConfig(), nil)
	snap := snapOf(
		trafficfeature.FeatureEntry{Addr: netip.MustParseAddr("198.51.100.1"), FanOut: 1, OutInRatio: 2},
		trafficfeature.FeatureEntry{Addr: netip.MustParseAddr("198.51.100.2"), FanOut: 1, OutInRatio: 2},
		trafficfeature.FeatureEntry{Addr: netip.MustParseAddr("198.51.100.3"), FanOut: 1, OutInRatio: 2},
		trafficfeature.FeatureEntry{Addr: netip.MustParseAddr("198.51.100.9"), FanOut: 1, OutInRatio: math.Inf(1)},
	)
	cohorts := d.cohortsOf(snap.Sources)
	ca := cohorts[d.cohortPrefix(netip.MustParseAddr("198.51.100.1"))]
	if ca == nil {
		t.Fatal("no cohort built")
	}
	if ca.ratio.count != 3 {
		t.Errorf("cohort ratio.count = %d, want 3 (the +Inf exfil host excluded)", ca.ratio.count)
	}
	if ca.fanout.count != 4 {
		t.Errorf("cohort fanout.count = %d, want 4 (all hosts counted)", ca.fanout.count)
	}
}

// TestDetectDestCohortRarity proves the DESTINATION axis is scored against its own
// prefix cohort: an outlier receiver among ordinary receivers in the same /24 is
// flagged on cohort rarity alone, before its own baseline has warmed up, and the
// incident says kind=dest with the dest prefix as its cohort. It also proves the
// +Inf ratio sentinel is left out of the dest cohort, as it is for sources.
//
// VALIDATES: child-5 AC-4 -- dest scored with the prefix cohort, kind-tagged emit.
// PREVENTS: the dest axis reusing the SOURCE cohort (which would measure a server
// against the clients talking to it), and a pure receiver's +Inf defining the cohort.
func TestDetectDestCohortRarity(t *testing.T) {
	cfg := testConfig()
	cfg.MinCohortSize = 2 // cohort rarity active, unlike the source tests
	d := newDetector(cfg, nil)

	normals := []trafficfeature.FeatureEntry{}
	for i := 1; i <= 6; i++ {
		fe := normalEntry(netip.AddrFrom4([4]byte{10, 1, 0, byte(i)}).String())
		normals = append(normals, fe)
	}
	outlier := netip.MustParseAddr("10.1.0.9")
	spike := trafficfeature.FeatureEntry{Addr: outlier, FanOut: 200, OutInRatio: 500, PortEntropy: 0.1}
	entries := append(append([]trafficfeature.FeatureEntry{}, normals...), spike)

	// The cohort excludes the +Inf receiver from the ratio statistics only.
	withInf := append(append([]trafficfeature.FeatureEntry{}, entries...),
		trafficfeature.FeatureEntry{Addr: netip.MustParseAddr("10.1.0.20"), FanOut: 1, OutInRatio: math.Inf(1)})
	ca := d.cohortsOf(withInf)[d.cohortPrefix(outlier)]
	if ca == nil {
		t.Fatal("no dest cohort built")
	}
	if ca.ratio.count != 7 {
		t.Errorf("dest cohort ratio.count = %d, want 7 (the +Inf receiver excluded)", ca.ratio.count)
	}
	if ca.fanout.count != 8 {
		t.Errorf("dest cohort fanout.count = %d, want 8 (all receivers counted)", ca.fanout.count)
	}

	// ConfirmDuration ticks of the same picture confirm exactly one incident, on the
	// outlier, before any baseline has warmed up: cohort rarity is what fired it.
	for range cfg.ConfirmDuration {
		d.onTick(destSnapOf(entries...))
	}
	incs := d.recentIncidents()
	if len(incs) != 1 {
		t.Fatalf("dest incidents = %d, want exactly 1 (the outlier)", len(incs))
	}
	inc := incs[0]
	if inc.EntityKind != anomalyevent.EntityKindDest {
		t.Errorf("incident kind = %q, want dest", inc.EntityKind)
	}
	if inc.Entity != netip.MustParsePrefix("10.1.0.9/32") {
		t.Errorf("incident entity = %v, want 10.1.0.9/32", inc.Entity)
	}
	if inc.Cohort != "10.1.0.0/24" {
		t.Errorf("incident cohort = %q, want 10.1.0.0/24", inc.Cohort)
	}
	if inc.Port != 0 || inc.Proto != 0 {
		t.Errorf("dest incident carries port %d/%d, want zero", inc.Proto, inc.Port)
	}
	// The source axis saw nothing: the axes keep separate baselines.
	if len(d.states) != 0 {
		t.Errorf("source axis grew %d baselines from a dest-only snapshot", len(d.states))
	}
	if len(d.destStates) != len(entries) {
		t.Errorf("dest baselines = %d, want %d", len(d.destStates), len(entries))
	}
}

// TestDetectPortCohortFree proves the PORT axis is scored with NO cohort: a port is a
// number, so its snapshot peers are not its peers. The same outlier that a cohort
// would flag on sight is NOT flagged while its own baseline is unwarmed, and IS
// flagged once that baseline exists. Self-deviation is the only thing scoring it.
//
// VALIDATES: child-5 AC-5 and A-4/A-5 -- cohort-free port scoring, obtained by
// passing no cohort rather than by editing the pinned rule (AC-10).
// PREVENTS: the port axis borrowing the address cohort machinery, which would flag
// every port that differs from the other ports in the same tick.
func TestDetectPortCohortFree(t *testing.T) {
	cfg := testConfig()
	cfg.MinCohortSize = 2 // a cohort WOULD be active at this size, if there were one
	cfg.ConfirmDuration = 1
	peers := []trafficfeature.PortFeatureEntry{}
	for p := uint16(31000); p < 31006; p++ {
		peers = append(peers, normalPort(p))
	}
	withSpike := append(append([]trafficfeature.PortFeatureEntry{}, peers...), spikePort(31337))
	allNormal := append(append([]trafficfeature.PortFeatureEntry{}, peers...), normalPort(31337))

	// One wild outlier among six ordinary ports, on a detector that has never seen
	// any of them. A cohort would flag it on this very tick; cohort-free it cannot,
	// because the only thing that can score a port is its own history.
	cold := newDetector(cfg, nil)
	cold.onTick(portSnapOf(withSpike...))
	if got := len(cold.recentIncidents()); got != 0 {
		t.Fatalf("port incidents on the first tick = %d, want 0 (no cohort, no baseline)", got)
	}

	// Self-deviation is what does flag it: the same picture, on a detector whose
	// baseline for that port was built from ordinary ticks.
	warm := newDetector(cfg, nil)
	for range warmupTicks + 2 {
		warm.onTick(portSnapOf(allNormal...))
	}
	if got := len(warm.recentIncidents()); got != 0 {
		t.Fatalf("port incidents during warmup = %d, want 0", got)
	}
	warm.onTick(portSnapOf(withSpike...))

	incs := warm.recentIncidents()
	if len(incs) != 1 {
		t.Fatalf("port incidents after the baseline warmed = %d, want 1", len(incs))
	}
	inc := incs[0]
	if inc.EntityKind != anomalyevent.EntityKindPort {
		t.Errorf("incident kind = %q, want port", inc.EntityKind)
	}
	if inc.Port != 31337 || inc.Proto != 17 {
		t.Errorf("incident identity = %d/%d, want 17/31337", inc.Proto, inc.Port)
	}
	if inc.Entity.IsValid() {
		t.Errorf("port incident entity = %v, want the zero prefix", inc.Entity)
	}
	if inc.Cohort != "" {
		t.Errorf("port incident cohort = %q, want empty (a port has no cohort)", inc.Cohort)
	}
	// rare-port must not be among the fired features: the port number never changed,
	// so it is not evidence of a deviation.
	for _, f := range inc.FiredFeatures {
		if f.Name == "rare-port" {
			t.Error("rare-port fired on the port axis; it is a property of the key, not a deviation")
		}
	}
}

// TestRatioSentinelPerAxis proves the two axes read "nothing came back" differently,
// which is the whole reason the ratio rule is per-axis. A SENDER with no inbound
// traffic is exfiltrating and scores at the ceiling. A RECEIVER with no outbound
// traffic is an ordinary sink -- a collector, a mirror, a log target -- so the feature
// is dropped for the tick instead of firing, and it never enters the baseline.
//
// VALIDATES: child-5 R-6 -- the dest axis does not over-fire on every quiet receiver.
// PREVENTS: two failures at once. Every pure receiver on the network firing one
// feature at maximum forever; and the +Inf value, which cohortsOf deliberately
// leaves OUT of the cohort, being scored against that cohort anyway, where its own
// missing contribution makes the leave-one-out mean meaningless.
func TestRatioSentinelPerAxis(t *testing.T) {
	cfg := testConfig()
	cfg.MinCohortSize = 2 // cohort rarity active: the garbage leave-one-out is reachable
	d := newDetector(cfg, nil)

	sink := netip.MustParseAddr("203.0.113.90")
	pureReceiver := trafficfeature.FeatureEntry{Addr: sink, FanOut: 1, OutInRatio: math.Inf(1), PortEntropy: 0.1}
	peers := []trafficfeature.FeatureEntry{}
	for i := 1; i <= 5; i++ {
		peers = append(peers, normalEntry(netip.AddrFrom4([4]byte{203, 0, 113, byte(i)}).String()))
	}
	entries := append(append([]trafficfeature.FeatureEntry{}, peers...), pureReceiver)

	st := stateFor(d.destStates, sink, maxTrackedDestEntities, d.alpha)
	fired, updates := d.scoreEntity(destSignals(pureReceiver), st, d.cohortsOf(entries)[d.cohortPrefix(sink)])
	for _, f := range fired {
		if f.Name == "out-in-ratio" {
			t.Errorf("a pure receiver fired out-in-ratio at z=%v; nothing came back, so there is nothing to measure", f.Z)
		}
	}
	for _, u := range updates {
		if u.val > 1e5 {
			t.Errorf("the +Inf sentinel was queued into a dest baseline as %v", u.val)
		}
	}

	// The control: the same value on the SOURCE axis IS a finding, so the assertion
	// above is about the axis and not about a ratio that never fires.
	srcSt := stateFor(d.states, sink, maxTrackedEntities, d.alpha)
	srcFired, _ := d.scoreEntity(sourceSignals(pureReceiver), srcSt, nil)
	var sawRatio bool
	for _, f := range srcFired {
		if f.Name == "out-in-ratio" {
			sawRatio = true
			if f.Z != zMax {
				t.Errorf("source +Inf ratio fired at z=%v, want the %v ceiling", f.Z, zMax)
			}
		}
	}
	if !sawRatio {
		t.Error("a pure sender must fire out-in-ratio: sending with nothing coming back is exfiltration")
	}
}

// TestDestPortConfirmClearLifecycle proves the confirm/clear state machine runs
// per-axis for the two new axes, with the same debounce as sources.
//
// VALIDATES: child-5 AC-4/AC-5 -- confirm after ConfirmDuration, clear after
// ClearConsecutive, on the dest and port axes.
// PREVENTS: a dest or port incident firing on one tick, or never clearing.
func TestDestPortConfirmClearLifecycle(t *testing.T) {
	cfg := testConfig()
	cfg.MinCohortSize = 1000 // self-deviation only, on every axis
	d := newDetector(cfg, nil)
	dst := "203.0.113.44"
	const port = 31337

	// Seed both axes with normal ticks.
	for range 5 {
		d.onTick(&trafficfeature.Snapshot{
			Dests: []trafficfeature.FeatureEntry{normalEntry(dst)},
			Ports: []trafficfeature.PortFeatureEntry{normalPort(port)},
		})
	}
	if got := len(d.recentIncidents()); got != 0 {
		t.Fatalf("incidents after normal traffic = %d, want 0", got)
	}

	spiking := &trafficfeature.Snapshot{
		Dests: []trafficfeature.FeatureEntry{spikeEntry(dst)},
		Ports: []trafficfeature.PortFeatureEntry{spikePort(port)},
	}
	d.onTick(spiking)
	if got := len(d.recentIncidents()); got != 0 {
		t.Fatalf("incidents after 1 spike = %d, want 0 (needs %d)", got, cfg.ConfirmDuration)
	}
	d.onTick(spiking)
	incs := d.recentIncidents()
	if len(incs) != 2 {
		t.Fatalf("incidents after ConfirmDuration spikes = %d, want 2 (one dest, one port)", len(incs))
	}
	kinds := map[anomalyevent.EntityKind]bool{}
	for i := range incs {
		kinds[incs[i].EntityKind] = true
	}
	if !kinds[anomalyevent.EntityKindDest] || !kinds[anomalyevent.EntityKindPort] {
		t.Errorf("confirmed kinds = %v, want one dest and one port", kinds)
	}
	if d.activeCount() != 2 {
		t.Errorf("active incidents = %d, want 2", d.activeCount())
	}

	// Back to normal: both clear.
	for range cfg.ClearConsecutive {
		d.onTick(&trafficfeature.Snapshot{
			Dests: []trafficfeature.FeatureEntry{normalEntry(dst)},
			Ports: []trafficfeature.PortFeatureEntry{normalPort(port)},
		})
	}
	if d.activeCount() != 0 {
		t.Errorf("active incidents after the clear window = %d, want 0", d.activeCount())
	}
}

// TestFreezeLearnDestPort proves freeze-learn is a property of the pipeline, not of
// the source axis: a sustained dest or port anomaly does not drift its own baseline
// up, so the incident stays open instead of self-clearing.
//
// VALIDATES: child-5 AC-6 and A-6 -- freeze-learn and warmup reused verbatim per axis.
// PREVENTS: a new axis getting the scoring but not the freeze, which would make every
// sustained anomaly disappear after a baseline window.
func TestFreezeLearnDestPort(t *testing.T) {
	cfg := testConfig()
	cfg.BaselineWindow = 10 // fast baseline -> would poison in a few ticks unfrozen
	cfg.MinCohortSize = 1000
	d := newDetector(cfg, nil)
	dst := netip.MustParseAddr("203.0.113.77")
	const port = 31338
	key := trafficfeature.PortKey{Port: port, Proto: 17}

	normal := &trafficfeature.Snapshot{
		Dests: []trafficfeature.FeatureEntry{{Addr: dst, FanOut: 1, OutInRatio: 1, PortEntropy: 0.1}},
		Ports: []trafficfeature.PortFeatureEntry{normalPort(port)},
	}
	for range 6 {
		d.onTick(normal)
	}
	if base := d.destStates[dst].fanout.mean.Value(); base > 2 {
		t.Fatalf("dest baseline not established low: fan-in mean = %v", base)
	}
	if base := d.portStates[key].fanout.mean.Value(); base > 2 {
		t.Fatalf("port baseline not established low: fan-out mean = %v", base)
	}

	spiking := &trafficfeature.Snapshot{
		Dests: []trafficfeature.FeatureEntry{{Addr: dst, FanOut: 200, OutInRatio: 500, PortEntropy: 0.1}},
		Ports: []trafficfeature.PortFeatureEntry{spikePort(port)},
	}
	for range 20 {
		d.onTick(spiking)
	}

	if got := d.destStates[dst].fanout.mean.Value(); got > 10 {
		t.Errorf("dest baseline poisoned: fan-in mean = %v, want ~1 (frozen)", got)
	}
	if !d.destStates[dst].active {
		t.Error("sustained dest anomaly self-cleared instead of staying active")
	}
	if got := d.portStates[key].fanout.mean.Value(); got > 10 {
		t.Errorf("port baseline poisoned: fan-out mean = %v, want ~1 (frozen)", got)
	}
	if !d.portStates[key].active {
		t.Error("sustained port anomaly self-cleared instead of staying active")
	}
}

// fakeGauge records the last value set, per label set.
type fakeGauge struct{ value float64 }

func (g *fakeGauge) Set(v float64) { g.value = v }
func (g *fakeGauge) Inc()          { g.value++ }
func (g *fakeGauge) Dec()          { g.value-- }
func (g *fakeGauge) Add(v float64) { g.value += v }

type fakeCounter struct{ n float64 }

func (c *fakeCounter) Inc()          { c.n++ }
func (c *fakeCounter) Add(v float64) { c.n += v }

// fakeGaugeVec hands out one fakeGauge per label value.
type fakeGaugeVec struct{ gauges map[string]*fakeGauge }

func (v *fakeGaugeVec) With(labelValues ...string) metrics.Gauge {
	key := labelValues[0]
	if g, ok := v.gauges[key]; ok {
		return g
	}
	g := &fakeGauge{}
	v.gauges[key] = g
	return g
}

func (v *fakeGaugeVec) Delete(...string) bool { return false }

// fakeRegistry serves the three metrics bindMetrics asks for and nothing else.
type fakeRegistry struct{ vec *fakeGaugeVec }

func (r *fakeRegistry) Counter(_, _ string) metrics.Counter { return &fakeCounter{} }
func (r *fakeRegistry) Gauge(_, _ string) metrics.Gauge     { return &fakeGauge{} }
func (r *fakeRegistry) GaugeVec(_, _ string, _ []string) metrics.GaugeVec {
	return r.vec
}
func (r *fakeRegistry) CounterVec(_, _ string, _ []string) metrics.CounterVec { return nil }
func (r *fakeRegistry) Histogram(_, _ string, _ []float64) metrics.Histogram  { return nil }
func (r *fakeRegistry) HistogramVec(_, _ string, _ []float64, _ []string) metrics.HistogramVec {
	return nil
}

// TestTrackedGaugeByDimension proves ze_anomaly_tracked_entities reports each axis
// separately, which is the early signal for the per-axis memory ceilings.
//
// VALIDATES: child-5 AC-3 -- the dimension label carries each map's own count.
// PREVENTS: one summed gauge, which cannot say which axis is filling up.
func TestTrackedGaugeByDimension(t *testing.T) {
	vec := &fakeGaugeVec{gauges: make(map[string]*fakeGauge)}
	prev := metricsPtr.Load()
	t.Cleanup(func() { metricsPtr.Store(prev) })
	bindMetrics(&fakeRegistry{vec: vec})

	d := newDetector(testConfig(), nil)
	d.onTick(&trafficfeature.Snapshot{
		Sources: []trafficfeature.FeatureEntry{normalEntry("198.51.100.1"), normalEntry("198.51.100.2")},
		Dests:   []trafficfeature.FeatureEntry{normalEntry("203.0.113.1")},
		Ports:   []trafficfeature.PortFeatureEntry{normalPort(31337), normalPort(443), normalPort(53)},
	})

	for dim, want := range map[string]float64{"source": 2, "dest": 1, "port": 3} {
		g, ok := vec.gauges[dim]
		if !ok {
			t.Errorf("no tracked gauge for dimension %q", dim)
			continue
		}
		if g.value != want {
			t.Errorf("tracked{dimension=%q} = %v, want %v", dim, g.value, want)
		}
	}
}

// TestFreezeLearnDuringSustainedAnomaly proves a sustained anomaly does NOT drift
// the entity's own baseline up until it looks normal (freeze-learn). A short
// baseline window makes the baseline fast to poison, so without the freeze the
// incident would self-clear; with it, the baseline holds and the incident persists.
func TestFreezeLearnDuringSustainedAnomaly(t *testing.T) {
	cfg := testConfig()
	cfg.BaselineWindow = 10 // fast baseline -> would poison in a few ticks without the freeze
	d := newDetector(cfg, nil)
	src := netip.MustParseAddr("198.51.100.77")

	// Warm and establish a low baseline with normal traffic.
	for range 6 {
		d.onTick(snapOf(trafficfeature.FeatureEntry{Addr: src, FanOut: 1, OutInRatio: 1, PortEntropy: 0.1}))
	}
	if base := d.states[src].fanout.mean.Value(); base > 2 {
		t.Fatalf("baseline not established low: fan-out mean = %v", base)
	}

	// Sustain the anomaly well beyond the baseline window.
	spike := trafficfeature.FeatureEntry{Addr: src, FanOut: 200, OutInRatio: 500, PortEntropy: 0.1}
	for range 20 {
		d.onTick(snapOf(spike))
	}

	st := d.states[src]
	if got := st.fanout.mean.Value(); got > 10 {
		t.Errorf("baseline poisoned by the sustained anomaly: fan-out mean = %v, want ~1 (frozen)", got)
	}
	if !st.active {
		t.Error("sustained anomaly self-cleared (baseline poisoning) instead of staying active")
	}
}
