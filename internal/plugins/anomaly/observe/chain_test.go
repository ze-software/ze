// Design: docs/architecture/anomaly/anomaly-3-observe.md -- facts to a finished incident.
package observe

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/observation"
	"github.com/ze-software/ze/internal/plugins/anomaly/detect"
)

// publishSyntheticFlow sends one flow-byte observation to the global feed, the same
// shape a real flow source (flowexport) publishes.
func publishSyntheticFlow(source, destination string, port uint16, octets float64) {
	observation.Global().Publish(observation.Observation{
		Kind:    observation.KindFlow,
		Feature: observation.FeatureFlowBytes,
		Flow: observation.FlowKey{
			Src:     netip.MustParseAddr(source),
			Dst:     netip.MustParseAddr(destination),
			DstPort: port,
		},
		Value: octets,
	})
}

// TestChainObserveLifecycle proves AC-10 end to end with real data: a same-/24
// normal cohort plus one high-fan-out, pure-outbound outlier is published as real
// observations, flows through a real trafficfeature.Service into the real detector,
// and the incident the detector confirms lands in this plugin's lifecycle store with
// a start time. When the outlier returns to normal the detector clears it, and the
// store records an end time.
//
// VALIDATES: the whole point of this plugin -- an operator can see a FINISHED
// incident's duration, which `show anomaly detect` cannot show because the
// detector's report ring records confirmations only.
// PREVENTS: a break anywhere in facts->judgment->lifecycle that the per-layer unit
// tests cannot see: the event contract, the subscription, and the finalize key all
// have to agree for this to pass.
func TestChainObserveLifecycle(t *testing.T) {
	// -short skip only. The verify gate runs `go test` WITHOUT -short (Makefile
	// GO_TEST), so this test still runs in CI; -short only lets local iteration skip
	// its real one-second trafficfeature windows. No coverage is dropped.
	if testing.Short() {
		t.Skip("integration test drives real 1s trafficfeature windows (~20s); run without -short")
	}

	bus := newObserveTestBus()
	incidents := newStore(100, time.Hour) // stale sweep must not be what finalizes here
	unsubscribe := subscribeStore(bus, incidents)
	defer unsubscribe()

	cfg := detect.DefaultConfig()
	cfg.Enabled = true
	cfg.ConfirmDuration = 2  // confirm after 2 consecutive above-threshold windows
	cfg.ClearConsecutive = 2 // and clear after 2 below-threshold windows
	tick, stop := detect.ChainForTest(cfg, bus)
	defer stop()

	// A same-/24 cohort. Each normal source is paired 1:1 with a partner in another
	// /24 so its traffic is balanced (in and out, a finite ratio) and its fan-out is
	// 1. Only the outlier later goes pure-outbound and fans out.
	type pair struct{ source, partner string }
	normals := []pair{
		{"10.0.0.1", "203.0.113.101"},
		{"10.0.0.2", "203.0.113.102"},
		{"10.0.0.3", "203.0.113.103"},
		{"10.0.0.4", "203.0.113.104"},
		{"10.0.0.5", "203.0.113.105"},
	}
	const outlier = "10.0.0.9" // same /24, so cohort rarity measures it against its peers
	const outlierPartner = "203.0.113.109"
	outlierEntity := netip.MustParsePrefix(outlier + "/32")
	destinations := []string{
		"203.0.113.10", "203.0.113.11", "203.0.113.12", "203.0.113.13",
		"203.0.113.14", "203.0.113.15", "203.0.113.16", "203.0.113.17",
		"203.0.113.18", "203.0.113.19", "203.0.113.20", "203.0.113.21",
	}

	// balanced injects one outbound and one inbound flow, so the source's out/in
	// ratio is about 1.
	balanced := func(source, partner string) {
		publishSyntheticFlow(source, partner, 443, 1000)
		publishSyntheticFlow(partner, source, 443, 1000)
	}
	window := func() {
		time.Sleep(1100 * time.Millisecond) // let trafficfeature finalize a 1s window
		tick()
	}
	normalWindow := func() {
		for _, n := range normals {
			balanced(n.source, n.partner)
		}
		balanced(outlier, outlierPartner)
		window()
	}

	// Warmup: every source behaves normally, so the baselines establish and the
	// new-peer flag expires.
	for range 7 {
		normalWindow()
	}
	if got := incidents.count(); got != 0 {
		t.Fatalf("a normal cohort opened %d incidents during warmup, want 0", got)
	}

	// Attack: normals stay balanced, the outlier goes pure-outbound across many
	// destinations on distinct rare ports -- exfiltration and scan behavior.
	attackDeadline := time.Now().Add(45 * time.Second)
	for incidents.count() == 0 {
		if time.Now().After(attackDeadline) {
			t.Fatal("no incident reached the lifecycle store from real trafficfeature data")
		}
		for _, n := range normals {
			balanced(n.source, n.partner)
		}
		for i, destination := range destinations {
			publishSyntheticFlow(outlier, destination, uint16(10000+i), 5000)
		}
		window()
	}

	open := incidents.list()[0]
	if open.Entity != outlierEntity {
		t.Fatalf("incident entity = %s, want the outlier %s", open.Entity, outlierEntity)
	}
	if !open.Active {
		t.Error("the incident must be active while the anomaly continues")
	}
	if open.StartTime.IsZero() {
		t.Error("the incident must carry a start-time")
	}
	if !open.EndTime.IsZero() {
		t.Errorf("end-time = %s, want zero while the incident is active", open.EndTime)
	}
	if open.Score <= 0 {
		t.Errorf("score = %v, want the detector's combined score", open.Score)
	}

	// Recovery: the outlier behaves normally again, so the detector clears it. It has
	// to keep sending, because an entity that goes silent is evicted with no clear --
	// that path is the stale sweep's, proven by TestObserveStaleSweepTickerFinalizes.
	clearDeadline := time.Now().Add(45 * time.Second)
	for incidents.activeCount() != 0 {
		if time.Now().After(clearDeadline) {
			t.Fatal("the incident never finalized after the outlier returned to normal")
		}
		normalWindow()
	}

	finalized := incidents.list()[len(incidents.list())-1]
	if finalized.Entity != outlierEntity {
		t.Fatalf("finalized entity = %s, want %s", finalized.Entity, outlierEntity)
	}
	if finalized.Active {
		t.Error("the cleared incident must report active false")
	}
	if finalized.EndTime.IsZero() {
		t.Fatal("the cleared incident must carry an end-time: that duration is what this plugin adds")
	}
	if !finalized.EndTime.After(finalized.StartTime) {
		t.Errorf("end-time %s is not after start-time %s", finalized.EndTime, finalized.StartTime)
	}

	// The lifecycle came from a real AnomalyCleared, not from the stale sweep: the
	// store's stale timeout is an hour and this test ran for seconds.
	if elapsed := finalized.EndTime.Sub(finalized.StartTime); elapsed > time.Hour {
		t.Errorf("incident duration %s exceeds the stale timeout; the sweep finalized it, not the clear", elapsed)
	}
	// The severity rides the event contract, so an empty one means the detector's
	// grading never reached the store.
	if finalized.Severity == "" {
		t.Error("severity is empty; the graded severity must reach the lifecycle record")
	}
}
