package observe

import (
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
)

func TestIncidentLifecycle(t *testing.T) {
	// VALIDATES: AC-1 -- incident opened on detect, finalized on clear
	s := newStore(100, time.Hour)

	event := &ddosevent.AttackDetected{
		Interface: "xe0",
		Target: ddosevent.VectorTuple{
			DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
			Proto:     17,
			DstPort:   53,
		},
		Family:    ddosevent.FamilyUDPFlood,
		PeakRxPps: 500000,
		PeakRxBps: 32000000,
	}
	s.open(event)

	if s.activeCount() != 1 {
		t.Errorf("ActiveCount: got %d, want 1", s.activeCount())
	}

	incidents := s.list()
	if len(incidents) != 1 {
		t.Fatalf("List len: got %d, want 1", len(incidents))
	}
	if !incidents[0].Active {
		t.Error("incident should be active")
	}
	if incidents[0].Interface != "xe0" {
		t.Errorf("Interface: got %q, want xe0", incidents[0].Interface)
	}
	if incidents[0].Family != ddosevent.FamilyUDPFlood {
		t.Errorf("Family: got %q, want udp-flood", incidents[0].Family)
	}

	s.finalize(event.Target)

	if s.activeCount() != 0 {
		t.Errorf("ActiveCount after finalize: got %d, want 0", s.activeCount())
	}
	incidents = s.list()
	if incidents[0].Active {
		t.Error("incident should be finalized")
	}
	if incidents[0].EndTime.IsZero() {
		t.Error("EndTime should be set after finalize")
	}
}

func TestIncidentRingEviction(t *testing.T) {
	// VALIDATES: AC-8 -- oldest record evicted when ring is full
	s := newStore(3, time.Hour)

	for i := range 5 {
		event := &ddosevent.AttackDetected{
			Interface: "xe0",
			Target: ddosevent.VectorTuple{
				DstPrefix: netip.MustParsePrefix("10.0.0." + itoa(i) + "/32"),
				Proto:     17,
			},
			PeakRxPps: float64(i * 1000),
		}
		s.open(event)
		s.finalize(event.Target)
	}

	incidents := s.list()
	if len(incidents) != 3 {
		t.Fatalf("List len: got %d, want 3 (ring cap)", len(incidents))
	}
	// Newest first
	if incidents[0].PeakPps != 4000 {
		t.Errorf("newest PeakPps: got %f, want 4000", incidents[0].PeakPps)
	}
}

func TestStaleOpenIncidentFinalized(t *testing.T) {
	// VALIDATES: R-2 -- stale open incidents are finalized by timeout
	s := newStore(100, 50*time.Millisecond)

	event := &ddosevent.AttackDetected{
		Interface: "xe0",
		Target: ddosevent.VectorTuple{
			DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
			Proto:     17,
		},
	}
	s.open(event)

	if s.activeCount() != 1 {
		t.Fatalf("ActiveCount: got %d, want 1", s.activeCount())
	}

	time.Sleep(100 * time.Millisecond)
	s.sweepStale()

	if s.activeCount() != 0 {
		t.Errorf("ActiveCount after sweep: got %d, want 0", s.activeCount())
	}
}

func TestMultipleIncidents(t *testing.T) {
	s := newStore(100, time.Hour)

	e1 := &ddosevent.AttackDetected{
		Interface: "xe0",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17},
		PeakRxPps: 1000,
	}
	e2 := &ddosevent.AttackDetected{
		Interface: "xe1",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.2/32"), Proto: 6},
		PeakRxPps: 2000,
	}
	s.open(e1)
	s.open(e2)

	if s.activeCount() != 2 {
		t.Errorf("ActiveCount: got %d, want 2", s.activeCount())
	}

	s.finalize(e1.Target)
	if s.activeCount() != 1 {
		t.Errorf("ActiveCount after partial finalize: got %d, want 1", s.activeCount())
	}
}

func itoa(i int) string {
	return []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}[i]
}
