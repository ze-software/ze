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
)

func snapOf(entries ...trafficfeature.FeatureEntry) *trafficfeature.Snapshot {
	return &trafficfeature.Snapshot{Sources: entries}
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
	cohorts := d.buildCohorts(snap)
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
