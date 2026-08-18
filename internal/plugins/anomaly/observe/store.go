// Design: docs/architecture/anomaly/anomaly-3-observe.md -- behavioral anomaly incident lifecycle ring
//
// Related: register.go feeds this ring from the anomalyevent subscriptions and
// drives sweepStale from the sweep worker; show.go reads it through activeStore.
//
// The detector (internal/plugins/anomaly/detect) keeps a Detected-only report ring
// with no end time, so an operator cannot see when an incident finished. This
// store holds the LIFECYCLE instead: open on Detected, finalize on Cleared, and
// finalize on the stale timeout when a Cleared never arrives.

package observe

import (
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/anomalyevent"
)

// incident is one behavioral anomaly lifecycle. Its subject is a SOURCE prefix
// (the behavioral entity the detector judged), not a destination tuple: that is
// the difference from the volumetric ddos incident record.
//
// Active is true between open and finalize. EndTime is set by finalize alone, so
// an incident that is not active always carries an end time.
type incident struct {
	ID            int                          `json:"id"`
	Interface     string                       `json:"interface,omitempty"`
	Entity        netip.Prefix                 `json:"entity"`
	Cohort        string                       `json:"cohort,omitempty"`
	FiredFeatures []anomalyevent.FeatureSignal `json:"fired-features,omitempty"`
	Score         float64                      `json:"score"`
	Severity      anomalyevent.Severity        `json:"severity,omitempty"`
	StartTime     time.Time                    `json:"start-time"`
	EndTime       time.Time                    `json:"end-time,omitzero"`
	Active        bool                         `json:"active"`
}

// store is the bounded incident lifecycle ring. Safe for concurrent use: one
// mutex guards every field, and every method takes it.
//
// The ring never grows past capacity, because open evicts before it appends. The
// memory ceiling is therefore capacity incidents, and capacity comes from the
// incident-ring-size leaf, which YANG and Config.Validate cap at 100000.
type store struct {
	mu           sync.Mutex
	ring         []incident
	capacity     int
	nextID       int
	staleTimeout time.Duration
}

// newStore builds an empty ring holding at most capacity incidents and finalizing
// an incident that stays open longer than staleTimeout. Both values come from
// config, which Config.Validate range-checks, so capacity is at least 1 here.
func newStore(capacity int, staleTimeout time.Duration) *store {
	return &store{
		ring:         make([]incident, 0, capacity),
		capacity:     capacity,
		nextID:       1,
		staleTimeout: staleTimeout,
	}
}

// open records a newly confirmed incident. Every Detected opens a NEW incident,
// a re-fire on an entity that already cleared included: two confirmations are two
// lifecycles, and finalize matches the newest active one.
func (s *store) open(e *anomalyevent.AnomalyDetected) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// StartTime is the detector's confirm timestamp, which is more accurate than
	// this store's receive time. An emitter that leaves At unset would date the
	// incident to 1970, which makes it instantly stale and reports a duration of
	// decades, so an unset At falls back to now rather than to the zero time.
	started := e.At
	if started.IsZero() {
		started = time.Now()
	}

	inc := incident{
		ID:            s.nextID,
		Interface:     e.Interface,
		Entity:        e.Entity,
		Cohort:        e.Cohort,
		FiredFeatures: e.FiredFeatures,
		Score:         e.Score,
		Severity:      e.Severity,
		StartTime:     started,
		Active:        true,
	}
	s.nextID++

	if len(s.ring) >= s.capacity {
		s.evictOldest()
	}
	s.ring = append(s.ring, inc)
}

// finalize closes the newest still-active incident for entity, which is the one
// this clear belongs to. A clear with no matching active incident is a no-op: a
// reconfigure rebuilds the ring, so an in-flight open record can be gone by the
// time its clear arrives.
func (s *store) finalize(entity netip.Prefix) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := len(s.ring) - 1; i >= 0; i-- {
		if s.ring[i].Active && s.ring[i].Entity == entity {
			s.ring[i].Active = false
			s.ring[i].EndTime = time.Now()
			return
		}
	}
}

// activeCount returns the number of incidents that are still open.
func (s *store) activeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for i := range s.ring {
		if s.ring[i].Active {
			count++
		}
	}
	return count
}

// count returns the number of incidents held in the ring, active and finalized.
// Cheaper than len(list()), which copies the whole ring.
func (s *store) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ring)
}

// list returns a copy of the ring newest-first, which is the order an operator
// reads it in.
func (s *store) list() []incident {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]incident, len(s.ring))
	for i := range s.ring {
		out[len(s.ring)-1-i] = s.ring[i]
	}
	return out
}

// sweepStale finalizes every incident that has stayed open longer than the stale
// timeout. It is the only close path for an entity the detector evicted after its
// idle window, because that eviction emits no clear: without the sweep those
// incidents read active forever and active-count only climbs. The scan is bounded
// by the ring capacity.
func (s *store) sweepStale() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for i := range s.ring {
		if s.ring[i].Active && now.Sub(s.ring[i].StartTime) > s.staleTimeout {
			s.ring[i].Active = false
			s.ring[i].EndTime = now
		}
	}
}

// evictOldest drops one incident to make room. It prefers the oldest FINALIZED
// incident, because a finalized record is complete history while an active one is
// still being written, and it drops the head when every incident is active. The
// caller MUST hold s.mu.
func (s *store) evictOldest() {
	for i := range s.ring {
		if !s.ring[i].Active {
			s.ring = append(s.ring[:i], s.ring[i+1:]...)
			return
		}
	}
	s.ring = s.ring[1:]
}
