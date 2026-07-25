package observe

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
)

// VALIDATES: AC-9 -- the confidence from AttackCharacterized is recorded onto the
// incident the matching AttackDetected opened (matched by victim prefix), and a
// characterize with no matching victim is a harmless no-op.
func TestStoreCharacterizeSetsConfidence(t *testing.T) {
	s := newStore(10, time.Hour)
	victim := netip.MustParsePrefix("203.0.113.42/32")

	s.open(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: victim},
		Family: ddosevent.FamilyGenericFlood,
	})
	s.characterize(&ddosevent.AttackCharacterized{
		Target:     ddosevent.VectorTuple{DstPrefix: victim},
		Family:     ddosevent.FamilyReflection,
		Confidence: 88,
	})

	list := s.list()
	if len(list) != 1 {
		t.Fatalf("incidents: got %d, want 1", len(list))
	}
	if list[0].Confidence != 88 {
		t.Errorf("confidence: got %d, want 88", list[0].Confidence)
	}

	// A characterize for an unknown victim must not panic or mutate anything.
	s.characterize(&ddosevent.AttackCharacterized{
		Target:     ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("198.51.100.9/32")},
		Confidence: 50,
	})
	if s.list()[0].Confidence != 88 {
		t.Error("unmatched characterize must not change an existing incident's confidence")
	}
}

// VALIDATES: confidence attaches even when AttackDetected opened the incident with
// NO victim (trafficusage had not resolved one at confirm) but characterization
// later derived the victim from the flow ring -- matched by interface to the still-
// unresolved active incident, target left empty so AttackCleared still finalizes it.
// PREVENTS: the confidence score being silently dropped whenever the fast signal's
// victim differs from the flow-derived one (an unresolved or IPv6-only victim);
// this is the timing race that made the QEMU confidence test flaky under load.
func TestStoreCharacterizeAttachesToUnresolvedIncident(t *testing.T) {
	s := newStore(10, time.Hour)

	// AttackDetected opened the incident with an empty target on "lo".
	s.open(&ddosevent.AttackDetected{
		Interface: "lo",
		Target:    ddosevent.VectorTuple{}, // trafficusage resolved no victim
		Family:    ddosevent.FamilyGenericFlood,
	})
	// Characterization derived the victim from the flow ring.
	s.characterize(&ddosevent.AttackCharacterized{
		Interface:  "lo",
		Target:     ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("127.0.0.2/32")},
		Family:     ddosevent.FamilyReflection,
		Confidence: 90,
	})

	list := s.list()
	if len(list) != 1 {
		t.Fatalf("incidents: got %d, want 1", len(list))
	}
	if list[0].Confidence != 90 {
		t.Errorf("confidence: got %d, want 90 (attached to the unresolved incident)", list[0].Confidence)
	}
	if list[0].Target.DstPrefix.IsValid() {
		t.Errorf("target must stay empty so AttackCleared (empty target) finalizes it, got %s", list[0].Target.DstPrefix)
	}

	// The fallback must not steal an unrelated interface's characterization.
	s.open(&ddosevent.AttackDetected{
		Interface: "eth1",
		Target:    ddosevent.VectorTuple{},
		Family:    ddosevent.FamilyGenericFlood,
	})
	s.characterize(&ddosevent.AttackCharacterized{
		Interface:  "eth9", // no active incident on eth9
		Target:     ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("203.0.113.7/32")},
		Confidence: 44,
	})
	for _, inc := range s.list() {
		if inc.Interface == "eth1" && inc.Confidence != 0 {
			t.Errorf("eth1 incident must not receive eth9's characterization, got %d", inc.Confidence)
		}
	}
}

// VALIDATES: characterization attaches to the CURRENT attack's incident, not a
// stale resolved-target incident lingering from a prior attack on the same victim
// (AttackCleared's empty target cannot finalize a resolved-target incident, so it
// stays Active until sweepStale). Newest-first + interface scoping picks the
// current one.
// PREVENTS: on repeated attacks against the same victim, the confidence score
// being written to the previous, stale incident instead of the live one.
func TestStoreCharacterizePrefersCurrentIncidentOverStale(t *testing.T) {
	s := newStore(10, time.Hour)
	victim := netip.MustParsePrefix("203.0.113.5/32")

	// Stale incident from a prior attack: opened with a RESOLVED target on eth0,
	// never finalized.
	s.open(&ddosevent.AttackDetected{
		Interface: "eth0",
		Target:    ddosevent.VectorTuple{DstPrefix: victim},
		Family:    ddosevent.FamilyGenericFlood,
	})
	// Current attack on the same interface: opened with an EMPTY target
	// (trafficusage had not resolved the victim at confirm).
	s.open(&ddosevent.AttackDetected{
		Interface: "eth0",
		Target:    ddosevent.VectorTuple{},
		Family:    ddosevent.FamilyGenericFlood,
	})
	// Characterization for the current attack derives the same victim prefix.
	s.characterize(&ddosevent.AttackCharacterized{
		Interface:  "eth0",
		Target:     ddosevent.VectorTuple{DstPrefix: victim},
		Confidence: 77,
	})

	for _, inc := range s.list() {
		if inc.Target.DstPrefix.IsValid() {
			if inc.Confidence != 0 {
				t.Errorf("stale resolved-target incident must stay at 0, got %d", inc.Confidence)
			}
		} else if inc.Confidence != 77 {
			t.Errorf("current (empty-target) incident should carry the score 77, got %d", inc.Confidence)
		}
	}
}

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

func TestStoreRecordsDirection(t *testing.T) {
	// VALIDATES: AC-13 -- the incident carries the victim direction from the event.
	s := newStore(10, time.Hour)
	s.open(&ddosevent.AttackDetected{
		Interface: "eth0",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("203.0.113.5/32")},
		Direction: ddosevent.DirectionRemote,
	})
	list := s.list()
	if len(list) != 1 {
		t.Fatalf("incidents: got %d, want 1", len(list))
	}
	if list[0].Direction != ddosevent.DirectionRemote {
		t.Errorf("incident direction: got %q, want remote", list[0].Direction)
	}
}

func itoa(i int) string {
	return []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}[i]
}
