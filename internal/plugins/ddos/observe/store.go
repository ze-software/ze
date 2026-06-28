// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- bounded incident store

package observe

import (
	"net/netip"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
)

type incident struct {
	ID         int                    `json:"id"`
	Interface  string                 `json:"interface"`
	Target     ddosevent.VectorTuple  `json:"target"`
	Family     ddosevent.AttackFamily `json:"family"`
	TopSources []netip.Addr           `json:"top-sources,omitempty"`
	PeakPps    float64                `json:"peak-pps"`
	PeakBps    float64                `json:"peak-bps"`
	StartTime  time.Time              `json:"start-time"`
	EndTime    time.Time              `json:"end-time,omitzero"`
	Active     bool                   `json:"active"`
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
