package observe

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/anomalyevent"
)

// VALIDATES: AC-1 -- one Detected opens one incident keyed on the SOURCE entity
// prefix, and the matching Cleared finalizes it with an end time. The start time
// is the detector's confirm timestamp, not the store's receive time.
// PREVENTS: the lifecycle gap this plugin exists to close -- an incident an
// operator can see start but never see finish (the detect report ring's whole
// limitation).
func TestObserveIncidentLifecycle(t *testing.T) {
	s := newStore(100, time.Hour)
	entity := netip.MustParsePrefix("10.0.0.9/32")
	confirmed := time.Now().Add(-2 * time.Minute)

	s.open(&anomalyevent.AnomalyDetected{
		Interface:     "xe0",
		Entity:        entity,
		Cohort:        "10.0.0.0/24",
		FiredFeatures: []anomalyevent.FeatureSignal{{Name: "fanout", Z: 4.5}},
		Score:         9.5,
		Severity:      anomalyevent.SeverityHigh,
		At:            confirmed,
	})

	if got := s.activeCount(); got != 1 {
		t.Fatalf("activeCount after open = %d, want 1", got)
	}
	list := s.list()
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	if list[0].Entity != entity {
		t.Errorf("entity = %s, want %s", list[0].Entity, entity)
	}
	if !list[0].Active {
		t.Error("incident must be active between open and finalize")
	}
	if !list[0].StartTime.Equal(confirmed) {
		t.Errorf("start-time = %s, want the detector's At %s", list[0].StartTime, confirmed)
	}
	if !list[0].EndTime.IsZero() {
		t.Errorf("end-time = %s, want zero while active", list[0].EndTime)
	}
	if list[0].Cohort != "10.0.0.0/24" || list[0].Score != 9.5 {
		t.Errorf("incident = %+v, want the cohort and score from the event", list[0])
	}
	if list[0].Severity != anomalyevent.SeverityHigh {
		t.Errorf("severity = %q, want high", list[0].Severity)
	}
	if len(list[0].FiredFeatures) != 1 || list[0].FiredFeatures[0].Name != "fanout" {
		t.Errorf("fired-features = %+v, want the fanout signal from the event", list[0].FiredFeatures)
	}

	s.finalize(entity)

	if got := s.activeCount(); got != 0 {
		t.Errorf("activeCount after finalize = %d, want 0", got)
	}
	list = s.list()
	if list[0].Active {
		t.Error("incident must be finalized after a clear")
	}
	if list[0].EndTime.IsZero() {
		t.Error("end-time must be set by finalize -- it is the duration an operator reads")
	}
	if list[0].EndTime.Before(list[0].StartTime) {
		t.Errorf("end-time %s is before start-time %s", list[0].EndTime, list[0].StartTime)
	}
}

// VALIDATES: a clear for an entity that has no open incident is a harmless no-op,
// and it does not reopen or corrupt a finalized record.
// PREVENTS: a reconfigure that rebuilt the ring mid-incident turning the late
// Cleared into a panic or a second mutation.
func TestObserveFinalizeUnknownEntityIsNoop(t *testing.T) {
	s := newStore(10, time.Hour)
	entity := netip.MustParsePrefix("10.0.0.1/32")
	s.open(&anomalyevent.AnomalyDetected{Entity: entity, At: time.Now()})
	s.finalize(entity)
	first := s.list()[0].EndTime

	s.finalize(netip.MustParsePrefix("198.51.100.7/32"))
	s.finalize(entity) // already finalized: no active incident left to match

	if got := s.list()[0].EndTime; !got.Equal(first) {
		t.Errorf("end-time changed to %s after a redundant finalize, want %s", got, first)
	}
	if s.count() != 1 {
		t.Errorf("count = %d, want 1 (a stray clear must not add a record)", s.count())
	}
}

// VALIDATES: AC-2 -- the ring never holds more than incident-ring-size incidents,
// the oldest are evicted, and list stays newest-first.
// PREVENTS: unbounded memory growth under incident churn (R-2).
func TestObserveRingEviction(t *testing.T) {
	const capacity = 3
	s := newStore(capacity, time.Hour)

	for i := range 5 {
		entity := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, 0, byte(i)}), 32)
		s.open(&anomalyevent.AnomalyDetected{Entity: entity, Score: float64(i), At: time.Now()})
		s.finalize(entity)
	}

	list := s.list()
	if len(list) != capacity {
		t.Fatalf("list len = %d, want %d (the ring cap)", len(list), capacity)
	}
	// Newest first: scores 4, 3, 2 survive; 0 and 1 were evicted.
	for i, want := range []float64{4, 3, 2} {
		if list[i].Score != want {
			t.Errorf("list[%d].score = %v, want %v (newest-first ordering)", i, list[i].Score, want)
		}
	}
}

// VALIDATES: eviction drops the oldest FINALIZED incident and keeps an active one,
// so a full ring cannot lose the incident that is still happening.
// PREVENTS: an operator losing the live incident to history while the ring is full.
func TestObserveEvictionPrefersFinalized(t *testing.T) {
	s := newStore(2, time.Hour)
	live := netip.MustParsePrefix("10.0.0.1/32")
	done := netip.MustParsePrefix("10.0.0.2/32")

	s.open(&anomalyevent.AnomalyDetected{Entity: live, At: time.Now()}) // oldest, stays active
	s.open(&anomalyevent.AnomalyDetected{Entity: done, At: time.Now()})
	s.finalize(done)

	third := netip.MustParsePrefix("10.0.0.3/32")
	s.open(&anomalyevent.AnomalyDetected{Entity: third, At: time.Now()})

	list := s.list()
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	held := map[netip.Prefix]bool{list[0].Entity: true, list[1].Entity: true}
	if !held[live] {
		t.Error("eviction dropped the still-active incident; it must drop the finalized one first")
	}
	if held[done] {
		t.Error("the finalized incident should have been evicted, not the active one")
	}
}

// VALIDATES: AC-3 -- an incident that never receives a Cleared is finalized by the
// stale sweep, with Active false and an end time set.
// PREVENTS: R-1 -- the detector evicts an idle entity without emitting Cleared, so
// without the sweep those incidents read Active forever and active-count only
// climbs.
func TestObserveStaleSweep(t *testing.T) {
	const staleTimeout = 50 * time.Millisecond
	s := newStore(100, staleTimeout)
	entity := netip.MustParsePrefix("10.0.0.9/32")
	s.open(&anomalyevent.AnomalyDetected{Entity: entity, At: time.Now()})

	if got := s.activeCount(); got != 1 {
		t.Fatalf("activeCount after open = %d, want 1", got)
	}
	s.sweepStale()
	if got := s.activeCount(); got != 1 {
		t.Fatalf("activeCount = %d, want 1: a fresh incident must survive the sweep", got)
	}

	time.Sleep(2 * staleTimeout)
	s.sweepStale()

	if got := s.activeCount(); got != 0 {
		t.Errorf("activeCount after the stale sweep = %d, want 0", got)
	}
	inc := s.list()[0]
	if inc.Active {
		t.Error("a stale incident must be finalized with no Cleared event")
	}
	if inc.EndTime.IsZero() {
		t.Error("the stale sweep must set end-time, or the incident shows no duration")
	}
}

// VALIDATES: AC-1 keying -- several entities are tracked independently, and a
// clear for one finalizes only that one.
// PREVENTS: a single clear closing every open incident, which would hide
// concurrent anomalies.
func TestObserveMultipleIncidents(t *testing.T) {
	s := newStore(100, time.Hour)
	first := netip.MustParsePrefix("10.0.0.1/32")
	second := netip.MustParsePrefix("10.0.0.2/32")

	s.open(&anomalyevent.AnomalyDetected{Entity: first, At: time.Now()})
	s.open(&anomalyevent.AnomalyDetected{Entity: second, At: time.Now()})
	if got := s.activeCount(); got != 2 {
		t.Fatalf("activeCount = %d, want 2", got)
	}

	s.finalize(first)
	if got := s.activeCount(); got != 1 {
		t.Errorf("activeCount after one clear = %d, want 1", got)
	}
	for _, inc := range s.list() {
		if inc.Entity == second && !inc.Active {
			t.Error("clearing the first entity must not finalize the second")
		}
		if inc.Entity == first && inc.Active {
			t.Error("the cleared entity must be finalized")
		}
	}
}

// VALIDATES: a re-fire after a clear opens a SECOND lifecycle for the same entity,
// and the next clear finalizes the newest one rather than rewriting history.
// PREVENTS: R-3 -- a repeat offender collapsing into one incident, which loses the
// count and the per-episode duration.
func TestObserveRefireOpensSecondLifecycle(t *testing.T) {
	s := newStore(10, time.Hour)
	entity := netip.MustParsePrefix("10.0.0.9/32")

	s.open(&anomalyevent.AnomalyDetected{Entity: entity, Score: 1, At: time.Now()})
	s.finalize(entity)
	firstEnd := s.list()[0].EndTime

	s.open(&anomalyevent.AnomalyDetected{Entity: entity, Score: 2, At: time.Now()})
	if s.count() != 2 {
		t.Fatalf("count = %d, want 2 (a re-fire is a new lifecycle)", s.count())
	}
	s.finalize(entity)

	list := s.list() // newest first
	if list[0].Score != 2 || list[1].Score != 1 {
		t.Fatalf("scores = %v/%v, want 2/1 newest-first", list[0].Score, list[1].Score)
	}
	if !list[1].EndTime.Equal(firstEnd) {
		t.Errorf("the first episode's end-time changed to %s, want %s", list[1].EndTime, firstEnd)
	}
	if list[0].EndTime.IsZero() {
		t.Error("the second episode must carry its own end-time")
	}
}

// VALIDATES: an event with no At falls back to the receive time, so the incident is
// never dated to the zero time.
// PREVENTS: a zero timestamp reading as a valid answer -- a 1970 start time makes
// the incident instantly stale and reports a duration of decades.
func TestObserveOpenWithoutTimestamp(t *testing.T) {
	s := newStore(10, time.Hour)
	before := time.Now()
	s.open(&anomalyevent.AnomalyDetected{Entity: netip.MustParsePrefix("10.0.0.9/32")})

	got := s.list()[0].StartTime
	if got.IsZero() {
		t.Fatal("start-time is the zero time; an unset At must fall back to now")
	}
	if got.Before(before) {
		t.Errorf("start-time = %s, want at or after %s", got, before)
	}
}
