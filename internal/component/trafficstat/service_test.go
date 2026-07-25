// VALIDATES: AC-1/AC-2 (service starts on first consumer without detector),
// AC-3 (service stops on last consumer), R-5 (refcount race safety).

package trafficstat

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/observation"
)

func waitForSnapshot(t *testing.T, svc *Service, check func(*Snapshot) bool) *Snapshot {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if snap := svc.Snapshot(); snap != nil && check(snap) {
			return snap
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for snapshot condition")
	return nil
}

// TestTrafficstatStartsWithoutDetector verifies the service starts on the
// first consumer and subscribes to the observation feed, with no DDoS
// detection logic required (AC-1, AC-2).
func TestTrafficstatStartsWithoutDetector(t *testing.T) {
	feed := observation.NewFeed()
	svc := NewService(feed)
	defer svc.Close()

	if svc.consumerCount() != 0 {
		t.Fatal("expected 0 consumers before attach")
	}

	id := svc.Attach()
	if svc.consumerCount() != 1 {
		t.Fatalf("expected 1 consumer after attach, got %d", svc.consumerCount())
	}

	snap := waitForSnapshot(t, svc, func(s *Snapshot) bool { return s != nil })
	if snap == nil {
		t.Fatal("expected non-nil snapshot after attach")
	}

	svc.Detach(id)
	if svc.consumerCount() != 0 {
		t.Fatalf("expected 0 consumers after detach, got %d", svc.consumerCount())
	}
}

// TestTrafficstatLazyStopOnLastConsumer verifies the service unsubscribes
// from the feed and stops its goroutine when the last consumer detaches (AC-3).
func TestTrafficstatLazyStopOnLastConsumer(t *testing.T) {
	feed := observation.NewFeed()
	svc := NewService(feed)
	defer svc.Close()

	id1 := svc.Attach()
	id2 := svc.Attach()

	if svc.consumerCount() != 2 {
		t.Fatalf("expected 2 consumers, got %d", svc.consumerCount())
	}

	svc.Detach(id1)
	if svc.consumerCount() != 1 {
		t.Fatalf("expected 1 consumer after first detach, got %d", svc.consumerCount())
	}
	if !svc.Running() {
		t.Fatal("service should still be running with 1 consumer")
	}

	svc.Detach(id2)
	if svc.consumerCount() != 0 {
		t.Fatalf("expected 0 consumers after last detach, got %d", svc.consumerCount())
	}

	deadline := time.Now().Add(3 * time.Second)
	for svc.Running() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if svc.Running() {
		t.Fatal("service should have stopped after last consumer detached")
	}
}

// TestTrafficstatRefcountRace hammers attach/detach concurrently to verify
// no panics, races, or goroutine leaks (R-5).
func TestTrafficstatRefcountRace(t *testing.T) {
	feed := observation.NewFeed()
	svc := NewService(feed)
	defer svc.Close()

	const goroutines = 20
	const cycles = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range cycles {
				id := svc.Attach()
				time.Sleep(time.Millisecond)
				svc.Detach(id)
			}
		}()
	}

	wg.Wait()

	if svc.consumerCount() != 0 {
		t.Fatalf("expected 0 consumers after all goroutines, got %d", svc.consumerCount())
	}
}

// TestTrafficstatDerivesRates verifies the service processes cumulative
// observations into a non-empty top-source-IPs snapshot (AC-9). Exact
// rate arithmetic is covered by window_test.go; this test proves the
// full service pipeline delivers data end-to-end.
func TestTrafficstatDerivesRates(t *testing.T) {
	feed := observation.NewFeed()
	svc := NewService(feed)
	defer svc.Close()

	id := svc.Attach()
	defer svc.Detach(id)

	now := time.Now()

	feed.Publish(observation.Observation{
		Kind:    observation.KindSourceIP,
		Iface:   "eth0",
		Feature: observation.FeatureRxBytes,
		Flow:    observation.FlowKey{Src: netip.MustParseAddr("198.51.100.1")},
		Value:   1000,
		At:      now,
	})

	feed.Publish(observation.Observation{
		Kind:    observation.KindSourceIP,
		Iface:   "eth0",
		Feature: observation.FeatureRxBytes,
		Flow:    observation.FlowKey{Src: netip.MustParseAddr("198.51.100.1")},
		Value:   3000,
		At:      now.Add(time.Second),
	})

	target := netip.MustParseAddr("198.51.100.1")
	snap := waitForSnapshot(t, svc, func(s *Snapshot) bool {
		for i := range s.TopSourceIPs {
			if s.TopSourceIPs[i].Addr == target {
				return true
			}
		}
		return false
	})
	if snap == nil {
		t.Fatal("expected snapshot")
	}
	if len(snap.TopSourceIPs) == 0 {
		t.Fatal("expected non-empty TopSourceIPs after cumulative observations")
	}
	found := false
	for _, te := range snap.TopSourceIPs {
		if te.Addr == netip.MustParseAddr("198.51.100.1") {
			if te.Bps <= 0 {
				t.Fatalf("expected positive Bps for 198.51.100.1, got %v", te.Bps)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("198.51.100.1 not present in TopSourceIPs")
	}
}

// TestTrafficstatTopNAndEviction verifies the top-N cap keeps the snapshot
// bounded even when many distinct source keys are observed (AC-10).
func TestTrafficstatTopNAndEviction(t *testing.T) {
	feed := observation.NewFeed()
	svc := NewService(feed)
	defer svc.Close()

	id := svc.Attach()
	defer svc.Detach(id)

	now := time.Now()

	// Publish flow deltas from MaxTopN+20 distinct source IPs so the
	// snapshot must enforce the cap.
	count := MaxTopN + 20
	for i := range count {
		src := netip.AddrFrom4([4]byte{10, 0, byte(i >> 8), byte(i)})
		dst := netip.MustParseAddr("203.0.113.1")
		feed.Publish(observation.Observation{
			Kind:    observation.KindFlow,
			Feature: observation.FeatureFlowBytes,
			Flow:    observation.FlowKey{Src: src, Dst: dst, DstPort: 80, Proto: 6},
			Value:   float64((i + 1) * 1000),
			At:      now,
		})
	}

	snap := waitForSnapshot(t, svc, func(s *Snapshot) bool {
		return len(s.TopSourceIPs) > 0
	})
	if snap == nil {
		t.Fatal("expected snapshot")
	}

	if len(snap.TopSourceIPs) == 0 {
		t.Fatal("expected non-empty TopSourceIPs")
	}
	if len(snap.TopSourceIPs) > MaxTopN {
		t.Errorf("top source IPs %d exceeds cap %d", len(snap.TopSourceIPs), MaxTopN)
	}
	if len(snap.TopDestIPs) == 0 {
		t.Fatal("expected non-empty TopDestIPs from flow observations")
	}
	if len(snap.TopPorts) == 0 {
		t.Fatal("expected non-empty TopPorts from flow observations")
	}
}

// TestTrafficstatDegradedNoTrafficUsage verifies the interface panel
// renders even when traffic-usage collectors are idle (AC-6).
func TestTrafficstatDegradedNoTrafficUsage(t *testing.T) {
	feed := observation.NewFeed()
	svc := NewService(feed)
	defer svc.Close()

	id := svc.Attach()
	defer svc.Detach(id)

	snap := waitForSnapshot(t, svc, func(s *Snapshot) bool { return s.Degraded })
	if snap == nil {
		t.Fatal("expected snapshot even with no collector data")
	}
	if !snap.Degraded {
		t.Error("expected Degraded=true when no collector observations arrive")
	}
}
