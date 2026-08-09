// Design: docs/architecture/diagnostics/event-history-rings.md -- per-peer BGP FSM transition history
// Related: peer_run.go -- FSM callback appends transitions

package reactor

import (
	"sync"
	"time"
)

const peerHistoryCapacity = 32

// FSMTransition records one BGP peer FSM state change.
type FSMTransition struct {
	Timestamp time.Time
	From      string
	To        string
	Reason    string
}

// fsmHistory is a fixed-size circular buffer of FSM transitions.
// Safe for concurrent use (append from FSM goroutine, snapshot from
// CLI handler goroutine).
type fsmHistory struct {
	mu      sync.Mutex
	records []FSMTransition
	head    int
	count   int
}

func newFSMHistory() *fsmHistory {
	return &fsmHistory{records: make([]FSMTransition, peerHistoryCapacity)}
}

func (h *fsmHistory) append(t FSMTransition) {
	h.mu.Lock()
	h.records[h.head] = t
	h.head = (h.head + 1) % len(h.records)
	if h.count < len(h.records) {
		h.count++
	}
	h.mu.Unlock()
}

// snapshot returns all transitions newest-first.
func (h *fsmHistory) snapshot() []FSMTransition {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.count == 0 {
		return []FSMTransition{}
	}
	out := make([]FSMTransition, h.count)
	for i := range h.count {
		idx := (h.head - 1 - i + len(h.records)) % len(h.records)
		out[i] = h.records[idx]
	}
	return out
}

// newest returns the most recent transition. The bool is false when the peer
// has never transitioned. O(1): unlike snapshot it does not copy the ring, so
// callers that only need the latest change (per-peer status polling) do not
// allocate peerHistoryCapacity records per peer per call.
func (h *fsmHistory) newest() (FSMTransition, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.count == 0 {
		return FSMTransition{}, false
	}
	idx := (h.head - 1 + len(h.records)) % len(h.records)
	return h.records[idx], true
}

// FSMHistory returns a snapshot of this peer's FSM transition history,
// newest first.
func (p *Peer) FSMHistory() []FSMTransition {
	if p.history == nil {
		return []FSMTransition{}
	}
	return p.history.snapshot()
}

// LastStateChange returns the time of this peer's most recent FSM transition.
// Returns the zero time when the peer has never transitioned, which is the
// "never came up" case callers must render as empty rather than as an epoch.
//
// Distinct from EstablishedAt: that is cleared by ClearStats on teardown, so it
// cannot express when a currently-down peer last changed state.
func (p *Peer) LastStateChange() time.Time {
	if p.history == nil {
		return time.Time{}
	}
	t, ok := p.history.newest()
	if !ok {
		return time.Time{}
	}
	return t.Timestamp
}
