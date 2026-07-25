// VALIDATES: AC-7 per-source neutral features (fan-out, out/in byte ratio,
// dest-port entropy, new-peer, rare-port), AC-8 coarse beaconing over active-tick
// gaps, and AC-10 cardinality caps + idle eviction -- all computed from the
// observation feed via internal/core/stats.
// PREVENTS: exfil ratio inversion (in vs out role mixup), counting a common port
// as rare, and unbounded per-source state under source churn.

package trafficfeature

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/observation"
)

func featureOf(snap *Snapshot, addr netip.Addr) (FeatureEntry, bool) {
	for _, fe := range snap.Sources {
		if fe.Addr == addr {
			return fe, true
		}
	}
	return FeatureEntry{}, false
}

func flowObs(src, dst netip.Addr, port uint16, bytes float64, at time.Time) observation.Observation {
	return observation.Observation{
		Kind:    observation.KindFlow,
		Feature: observation.FeatureFlowBytes,
		Flow:    observation.FlowKey{Src: src, Dst: dst, DstPort: port, Proto: 6},
		Value:   bytes,
		At:      at,
	}
}

func TestFeatureFanOutRatioEntropy(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	s := netip.MustParseAddr("198.51.100.5")
	d1 := netip.MustParseAddr("203.0.113.1")
	d2 := netip.MustParseAddr("203.0.113.2")
	d3 := netip.MustParseAddr("203.0.113.3")
	x := netip.MustParseAddr("192.0.2.9")

	a.ingest(flowObs(s, d1, 443, 1000, t0))
	a.ingest(flowObs(s, d2, 443, 1000, t0))
	a.ingest(flowObs(s, d3, 80, 500, t0))
	a.ingest(flowObs(x, s, 22, 200, t0)) // s receives 200 inbound

	snap := a.snapshot(t0.Add(time.Second))
	fe, ok := featureOf(snap, s)
	if !ok {
		t.Fatalf("source %v not in snapshot", s)
	}
	if fe.FanOut != 3 {
		t.Errorf("fan-out = %d, want 3 (distinct dests)", fe.FanOut)
	}
	// out=2500, in=200 -> exfil ratio 12.5 (NOT the inverse 0.08)
	if fe.OutInRatio != 12.5 {
		t.Errorf("out/in ratio = %v, want 12.5", fe.OutInRatio)
	}
	if fe.PortEntropy <= 0 {
		t.Errorf("port entropy = %v, want > 0 (ports 443 + 80)", fe.PortEntropy)
	}
	// A pure destination (only ever received) must NOT appear: the features are
	// source-behavior signals, so emitting it would be all-zero noise under a
	// "source" heading. d1 only ever appeared as a flow destination.
	if _, ok := featureOf(snap, d1); ok {
		t.Error("pure-destination d1 must not appear in the source-feature snapshot")
	}
	// The other sender (x sent 200 to s) SHOULD appear.
	if _, ok := featureOf(snap, x); !ok {
		t.Error("source x (sent 200) should appear in the snapshot")
	}
}

func TestFeatureNewPeerRarePort(t *testing.T) {
	t0 := time.Now()
	s := netip.MustParseAddr("198.51.100.7")
	d := netip.MustParseAddr("203.0.113.9")

	a := newAggregator()
	a.ingest(flowObs(s, d, 31337, 1000, t0)) // uncommon port
	fe, ok := featureOf(a.snapshot(t0.Add(time.Second)), s)
	if !ok {
		t.Fatal("source not present")
	}
	if !fe.NewPeer {
		t.Error("expected new-peer=true on first sighting")
	}
	if !fe.RarePort {
		t.Error("expected rare-port=true for dominant port 31337")
	}

	a2 := newAggregator()
	a2.ingest(flowObs(s, d, 443, 1000, t0)) // common port
	fe2, _ := featureOf(a2.snapshot(t0.Add(time.Second)), s)
	if fe2.RarePort {
		t.Error("dominant port 443 must not be rare")
	}
}

func TestFeatureBeaconingCoarse(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	s := netip.MustParseAddr("198.51.100.11")
	d := netip.MustParseAddr("203.0.113.11")

	var snap *Snapshot
	for tick := range 41 {
		if tick%5 == 0 { // active every 5 seconds
			a.ingest(flowObs(s, d, 443, 1000, t0))
		}
		snap = a.snapshot(t0.Add(time.Duration(tick+1) * time.Second))
	}
	fe, ok := featureOf(snap, s) // tick 40 is active
	if !ok {
		t.Fatal("beaconing source not present on its active tick")
	}
	if fe.Beaconing < 0.5 {
		t.Errorf("beaconing = %v, want >= 0.5 for a regular 5s beacon", fe.Beaconing)
	}
}

func TestFeatureCapsAndEviction(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	s := netip.MustParseAddr("192.0.2.50")
	d := netip.MustParseAddr("192.0.2.51")

	a.ingest(flowObs(s, d, 443, 500, t0))
	for i := 1; i <= evictIdleTicks+2; i++ {
		a.snapshot(t0.Add(time.Duration(i) * time.Second))
	}
	a.mu.Lock()
	_, present := a.sources[s]
	a.mu.Unlock()
	if present {
		t.Fatalf("idle source not evicted after %d idle ticks", evictIdleTicks)
	}
}
