// Design: docs/architecture/chaos-web-dashboard.md — active set peer visibility
// Overview: state.go — PeerState and DashboardState use ActiveSet

package web

import (
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// ActiveSetEntry tracks a peer's presence in the active set.
type ActiveSetEntry struct {
	PeerIndex  int
	Pinned     bool
	PromotedAt time.Time
	LastActive time.Time
	Priority   PromotionPriority
}

// PromotionPriority indicates why a peer was promoted to the active set.
type PromotionPriority int

const (
	// PriorityManual means the user manually added or pinned the peer.
	PriorityManual PromotionPriority = iota
	// PriorityLow is for minor events (missing routes).
	PriorityLow
	// PriorityMedium is for notable events (chaos, reconnecting, withdrawals).
	PriorityMedium
	// PriorityHigh is for critical events (disconnected, error).
	PriorityHigh
)

// ActiveSet manages the visible subset of peers in the dashboard table.
type ActiveSet struct {
	MaxVisible int
	entries    map[int]*ActiveSetEntry // peerIndex -> entry
}

// NewActiveSet creates an active set with the given capacity.
func NewActiveSet(maxVisible int) *ActiveSet {
	if maxVisible < 10 {
		maxVisible = 10
	}
	return &ActiveSet{
		MaxVisible: maxVisible,
		entries:    make(map[int]*ActiveSetEntry),
	}
}

// Promote adds or refreshes a peer in the active set with the given priority.
// Returns true if the peer was newly added (not already present).
func (a *ActiveSet) Promote(peerIndex int, priority PromotionPriority, now time.Time) bool {
	if e, ok := a.entries[peerIndex]; ok {
		e.LastActive = now
		if priority > e.Priority {
			e.Priority = priority
		}
		return false
	}

	// Evict oldest non-pinned peer if at capacity.
	if len(a.entries) >= a.MaxVisible {
		evicted := a.findEvictionCandidate(now)
		if evicted < 0 {
			return false // All pinned, can't evict.
		}
		delete(a.entries, evicted)
	}

	a.entries[peerIndex] = &ActiveSetEntry{
		PeerIndex:  peerIndex,
		PromotedAt: now,
		LastActive: now,
		Priority:   priority,
	}
	return true
}

// findEvictionCandidate returns the peer index of the best eviction target,
// or -1 if all peers are pinned.
func (a *ActiveSet) findEvictionCandidate(_ time.Time) int {
	var (
		candidate = -1
		oldest    time.Time
	)
	for idx, e := range a.entries {
		if e.Pinned {
			continue
		}
		if candidate < 0 || e.LastActive.Before(oldest) {
			candidate = idx
			oldest = e.LastActive
		}
	}
	return candidate
}

// Pin marks a peer as pinned. If not in the active set, promotes it first.
func (a *ActiveSet) Pin(peerIndex int, now time.Time) {
	if e, ok := a.entries[peerIndex]; ok {
		e.Pinned = true
		return
	}
	a.Promote(peerIndex, PriorityManual, now)
	if e, ok := a.entries[peerIndex]; ok {
		e.Pinned = true
	}
}

// Unpin removes the pin from a peer, making it subject to decay.
func (a *ActiveSet) Unpin(peerIndex int) {
	if e, ok := a.entries[peerIndex]; ok {
		e.Pinned = false
	}
}

// IsPinned returns true if the peer is pinned.
func (a *ActiveSet) IsPinned(peerIndex int) bool {
	e, ok := a.entries[peerIndex]
	return ok && e.Pinned
}

// Contains returns true if the peer is in the active set.
func (a *ActiveSet) Contains(peerIndex int) bool {
	_, ok := a.entries[peerIndex]
	return ok
}

// Decay removes expired non-pinned peers from the active set.
// Returns the indices of removed peers.
func (a *ActiveSet) Decay(now time.Time) []int {
	ttl := a.adaptiveTTL()
	var removed []int
	for idx, e := range a.entries {
		if e.Pinned {
			continue
		}
		if now.Sub(e.LastActive) > ttl {
			removed = append(removed, idx)
		}
	}
	for _, idx := range removed {
		delete(a.entries, idx)
	}
	return removed
}

// adaptiveTTL returns the current decay TTL based on fill ratio.
func (a *ActiveSet) adaptiveTTL() time.Duration {
	fill := float64(len(a.entries)) / float64(a.MaxVisible)
	switch {
	case fill > 0.8:
		return 5 * time.Second
	case fill > 0.5:
		return 30 * time.Second
	default:
		return 120 * time.Second
	}
}

// AdaptiveTTL returns the current decay TTL (exported for testing/display).
func (a *ActiveSet) AdaptiveTTL() time.Duration {
	return a.adaptiveTTL()
}

// SetMaxVisible updates the maximum number of visible peers.
func (a *ActiveSet) SetMaxVisible(n int) {
	if n < 1 {
		n = 1
	}
	a.MaxVisible = n
}

// Len returns the number of peers in the active set.
func (a *ActiveSet) Len() int {
	return len(a.entries)
}

// Indices returns all peer indices in the active set.
func (a *ActiveSet) Indices() []int {
	result := make([]int, 0, len(a.entries))
	for idx := range a.entries {
		result = append(result, idx)
	}
	return result
}

// Entry returns the entry for a peer, or nil if not in the active set.
func (a *ActiveSet) Entry(peerIndex int) *ActiveSetEntry {
	return a.entries[peerIndex]
}

// PromotionPriorityForEvent returns the priority for auto-promoting a peer
// based on the event type.
func PromotionPriorityForEvent(evType peer.EventType) (PromotionPriority, bool) {
	switch evType {
	case peer.EventDisconnected, peer.EventError:
		return PriorityHigh, true
	case peer.EventDroppedEvents:
		return PriorityHigh, true
	case peer.EventChaosExecuted, peer.EventReconnecting, peer.EventWithdrawalSent, peer.EventRouteAction:
		return PriorityMedium, true
	case peer.EventRouteWithdrawn:
		return PriorityLow, true
	case peer.EventEstablished, peer.EventRouteSent, peer.EventRouteReceived, peer.EventEORSent:
		return 0, false
	}
	return 0, false
}
