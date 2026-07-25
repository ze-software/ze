// VALIDATES: AC-9 rate derivation per Feature -- flow deltas are summed per
// window and converted to a per-second rate, then RESET the next window;
// cumulative counters are diffed. AC-4 separate top source/dest IP rankings.
// PREVENTS: the accumulate-forever regression where flow "rates" grew without
// bound, and the empty-TopDestIPs / src-dst-conflation defect.

package trafficstat

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/observation"
)

func bpsOf(list []TalkerEntry, addr netip.Addr) float64 {
	for i := range list {
		if list[i].Addr == addr {
			return list[i].Bps
		}
	}
	return 0
}

// TestAggregatorFlowDeltaRateAndReset proves flow byte-deltas become a per-second
// rate for the window and reset to zero the next idle window (the regression).
func TestAggregatorFlowDeltaRateAndReset(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	a.snapshot(t0, nil) // prime lastSnap

	src := netip.MustParseAddr("198.51.100.1")
	dst := netip.MustParseAddr("203.0.113.9")
	flow := func(v float64) observation.Observation {
		return observation.Observation{
			Kind:    observation.KindFlow,
			Feature: observation.FeatureFlowBytes,
			Flow:    observation.FlowKey{Src: src, Dst: dst, DstPort: 443, Proto: 6},
			Value:   v,
			At:      t0,
		}
	}
	a.ingest(flow(1000))
	a.ingest(flow(1000))

	snap := a.snapshot(t0.Add(time.Second), nil) // dt = 1s
	if got := bpsOf(snap.TopSourceIPs, src); got != 2000 {
		t.Fatalf("src bps = %v, want 2000", got)
	}
	if got := bpsOf(snap.TopDestIPs, dst); got != 2000 {
		t.Fatalf("dst bps = %v, want 2000 (TopDestIPs must be populated separately)", got)
	}
	if len(snap.TopPorts) != 1 || snap.TopPorts[0].Port != 443 || snap.TopPorts[0].Bps != 2000 {
		t.Fatalf("top ports = %+v, want a single 443 entry at 2000 bps", snap.TopPorts)
	}

	// Next window, no traffic: the rate must drop to 0, not keep the old sum.
	snap2 := a.snapshot(t0.Add(2*time.Second), nil)
	if got := bpsOf(snap2.TopSourceIPs, src); got != 0 {
		t.Fatalf("src bps after idle window = %v, want 0 (no accumulation)", got)
	}
}

// TestAggregatorCumulativeRate proves cumulative counters are diffed into the
// window and converted to a rate.
func TestAggregatorCumulativeRate(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	a.snapshot(t0, nil)

	src := netip.MustParseAddr("198.51.100.7")
	cumul := func(v float64) observation.Observation {
		return observation.Observation{
			Kind:    observation.KindSourceIP,
			Feature: observation.FeatureRxBytes,
			Flow:    observation.FlowKey{Src: src},
			Value:   v,
			At:      t0,
		}
	}
	a.ingest(cumul(1000)) // primes lastCumul, no window contribution
	a.ingest(cumul(3000)) // +2000 this window

	snap := a.snapshot(t0.Add(time.Second), nil)
	if got := bpsOf(snap.TopSourceIPs, src); got != 2000 {
		t.Fatalf("cumulative src bps = %v, want 2000", got)
	}
}

func histOf(list []TalkerEntry, addr netip.Addr) []float64 {
	for i := range list {
		if list[i].Addr == addr {
			return list[i].History
		}
	}
	return nil
}

// TestAggregatorPerSourceHistory proves per-source rolling history is exposed on
// TalkerEntry (AC-1) and bounded to sourceHistoryLen samples (AC-10), so the
// behavioral detector can baseline a source against its own recent rate.
func TestAggregatorPerSourceHistory(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	a.snapshot(t0, nil)

	src := netip.MustParseAddr("198.51.100.42")
	cumul := 0.0
	feed := func(sec int) *Snapshot {
		a.ingest(observation.Observation{
			Kind:    observation.KindSourceIP,
			Feature: observation.FeatureRxBytes,
			Flow:    observation.FlowKey{Src: src},
			Value:   cumul, // cumulative counter
			At:      t0,
		})
		return a.snapshot(t0.Add(time.Duration(sec)*time.Second), nil)
	}

	feed(0) // primes lastCumul
	var snap *Snapshot
	for i := 1; i <= 3; i++ {
		cumul += 1000
		snap = feed(i)
	}
	if h := histOf(snap.TopSourceIPs, src); len(h) < 3 {
		t.Fatalf("history len = %d, want >= 3", len(h))
	} else if last := h[len(h)-1]; last != 1000 {
		t.Errorf("newest history sample = %v, want 1000 (current bps)", last)
	}

	// Drive well past the cap; history must stay bounded.
	for i := 4; i <= sourceHistoryLen+20; i++ {
		cumul += 1000
		snap = feed(i)
	}
	if h := histOf(snap.TopSourceIPs, src); len(h) != sourceHistoryLen {
		t.Fatalf("history len = %d, want capped at %d", len(h), sourceHistoryLen)
	}
}

// TestAggregatorEvictsIdleKeys proves a key that goes quiet is dropped after
// evictIdleTicks, bounding memory (AC-10).
func TestAggregatorEvictsIdleKeys(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	a.snapshot(t0, nil)

	src := netip.MustParseAddr("192.0.2.50")
	a.ingest(observation.Observation{
		Kind: observation.KindFlow, Feature: observation.FeatureFlowBytes,
		Flow:  observation.FlowKey{Src: src, Dst: netip.MustParseAddr("192.0.2.51")},
		Value: 500, At: t0,
	})

	for i := 1; i <= evictIdleTicks+2; i++ {
		a.snapshot(t0.Add(time.Duration(i)*time.Second), nil)
	}

	a.mu.Lock()
	_, present := a.sources[src]
	a.mu.Unlock()
	if present {
		t.Fatalf("idle source key was not evicted after %d idle ticks", evictIdleTicks)
	}
}
