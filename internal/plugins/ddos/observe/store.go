// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- bounded incident store

package observe

import (
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
)

type incident struct {
	ID        int                    `json:"id"`
	Interface string                 `json:"interface"`
	Target    ddosevent.VectorTuple  `json:"target"`
	Family    ddosevent.AttackFamily `json:"family"`
	// Direction records whether the victim is local (box-owned) or remote (transit),
	// as classified by the detector; surfaced on `show ddos incidents`.
	Direction  ddosevent.Direction `json:"direction,omitempty"`
	TopSources []netip.Addr        `json:"top-sources,omitempty"`
	PeakPps    float64             `json:"peak-pps"`
	PeakBps    float64             `json:"peak-bps"`
	// Confidence (0-100) is recorded from the Stage-2 AttackCharacterized event when
	// it lands for this incident's target (see store.characterize). Zero (omitted)
	// while an incident is still on the coarse AttackDetected only.
	Confidence int       `json:"confidence,omitempty"`
	StartTime  time.Time `json:"start-time"`
	EndTime    time.Time `json:"end-time,omitzero"`
	Active     bool      `json:"active"`
}

type store struct {
	mu           sync.Mutex
	ring         []incident
	cap          int
	nextID       int
	staleTimeout time.Duration
}

func newStore(capacity int, staleTimeout time.Duration) *store {
	return &store{
		ring:         make([]incident, 0, capacity),
		cap:          capacity,
		nextID:       1,
		staleTimeout: staleTimeout,
	}
}

func (s *store) open(e *ddosevent.AttackDetected) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inc := incident{
		ID:         s.nextID,
		Interface:  e.Interface,
		Target:     e.Target,
		Family:     e.Family,
		Direction:  e.Direction,
		TopSources: e.TopSources,
		PeakPps:    e.PeakRxPps,
		PeakBps:    e.PeakRxBps,
		StartTime:  time.Now(),
		Active:     true,
	}
	s.nextID++

	if len(s.ring) >= s.cap {
		s.evictOldest()
	}
	s.ring = append(s.ring, inc)
}

func (s *store) finalize(target ddosevent.VectorTuple) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := len(s.ring) - 1; i >= 0; i-- {
		if s.ring[i].Active && s.ring[i].Target.DstPrefix == target.DstPrefix {
			s.ring[i].Active = false
			s.ring[i].EndTime = time.Now()
			return
		}
	}
}

// characterize records the confidence from the Stage-2 AttackCharacterized event
// onto the open incident it belongs to. Characterized arrives after the incident
// was opened by AttackDetected. A miss (no matching active incident) is a no-op --
// confidence stays 0 (omitted).
func (s *store) characterize(e *ddosevent.AttackCharacterized) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Attach to the incident THIS characterization belongs to: the NEWEST still-
	// active incident on the same interface whose target either matches the
	// characterized victim or is still unresolved (empty). Two cases fold together:
	//   * AttackDetected opened the incident with NO victim (trafficusage had not
	//     resolved one at confirm -- a timing race, or an IPv6-only victim it
	//     cannot see) but characterization derived the victim from the flow ring.
	//     The target is left EMPTY on purpose: AttackCleared also carries an empty
	//     target (see detector.emitCleared), so leaving it empty keeps finalize's
	//     match working.
	//   * A resolved-target incident from a PRIOR attack can linger Active because
	//     AttackCleared's empty target never finalizes a resolved-target incident
	//     until sweepStale. Scanning newest-first, scoped to the interface, attaches
	//     to the CURRENT attack's incident rather than that stale one -- exact-match
	//     alone would wrongly attribute the score to the stale incident.
	// (Two truly-concurrent attacks on one interface, one resolved + one empty, are
	// rare enough that newest-wins is acceptable; responders gate on the event's
	// confidence, not the store, so this is observability only.)
	for i := len(s.ring) - 1; i >= 0; i-- {
		inc := &s.ring[i]
		if !inc.Active || inc.Interface != e.Interface {
			continue
		}
		if inc.Target.DstPrefix == e.Target.DstPrefix || !inc.Target.DstPrefix.IsValid() {
			inc.Confidence = e.Confidence
			if e.Direction != "" {
				inc.Direction = e.Direction // characterized direction is authoritative
			}
			return
		}
	}
}

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

// count returns the number of incidents currently held in the ring (active and
// finalized). Cheaper than len(list()), which copies the whole ring.
func (s *store) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ring)
}

func (s *store) list() []incident {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]incident, len(s.ring))
	for i := range s.ring {
		out[len(s.ring)-1-i] = s.ring[i]
	}
	return out
}

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

func (s *store) evictOldest() {
	for i := range s.ring {
		if !s.ring[i].Active {
			s.ring = append(s.ring[:i], s.ring[i+1:]...)
			return
		}
	}
	s.ring = s.ring[1:]
}
