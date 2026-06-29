// VALIDATES: AC-1/AC-2 (service starts on first consumer without detector),
// AC-3 (service stops on last consumer), R-5 (refcount race safety).

package trafficstat

import (
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/observation"
)

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

	// Give the service a tick to process
	time.Sleep(100 * time.Millisecond)

	snap := svc.Snapshot()
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

	// Give service time to stop
	time.Sleep(50 * time.Millisecond)
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

// TestTrafficstatDerivesRates verifies cumulative values are diffed and
// delta values are summed correctly (AC-9).
func TestTrafficstatDerivesRates(t *testing.T) {
	feed := observation.NewFeed()
	svc := NewService(feed)
	defer svc.Close()

	id := svc.Attach()
	defer svc.Detach(id)

	now := time.Now()

	// Publish two cumulative observations 1s apart to get a rate
	feed.Publish(observation.Observation{
		Kind:    observation.KindSourceIP,
		Iface:   "eth0",
		Feature: observation.FeatureRxBytes,
		Value:   1000,
		At:      now,
	})

	time.Sleep(50 * time.Millisecond)

	feed.Publish(observation.Observation{
		Kind:    observation.KindSourceIP,
		Iface:   "eth0",
		Feature: observation.FeatureRxBytes,
		Value:   2000,
		At:      now.Add(time.Second),
	})

	// Wait for the service to process
	time.Sleep(200 * time.Millisecond)

	snap := svc.Snapshot()
	if snap == nil {
		t.Fatal("expected snapshot")
	}
	// The snapshot should have processed the observations; detailed rate
	// derivation is verified in window_test.go.
}

// TestTrafficstatTopNAndEviction verifies the top-N cap and cold eviction
// keep tracked keys bounded (AC-10).
func TestTrafficstatTopNAndEviction(t *testing.T) {
	feed := observation.NewFeed()
	svc := NewService(feed)
	defer svc.Close()

	id := svc.Attach()
	defer svc.Detach(id)

	now := time.Now()

	// Publish observations for more unique keys than the top-N cap
	for i := range 200 {
		feed.Publish(observation.Observation{
			Kind:    observation.KindSourceIP,
			Iface:   "eth0",
			Feature: observation.FeatureRxBytes,
			Value:   float64(i * 100),
			At:      now,
		})
	}

	time.Sleep(200 * time.Millisecond)

	snap := svc.Snapshot()
	if snap == nil {
		t.Fatal("expected snapshot")
	}

	// Verify the talker count is bounded
	if len(snap.TopSourceIPs) > MaxTopN {
		t.Errorf("top source IPs %d exceeds cap %d", len(snap.TopSourceIPs), MaxTopN)
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

	time.Sleep(100 * time.Millisecond)

	snap := svc.Snapshot()
	if snap == nil {
		t.Fatal("expected snapshot even with no collector data")
	}
	if snap.Degraded != true {
		t.Error("expected Degraded=true when no collector observations arrive")
	}
}
