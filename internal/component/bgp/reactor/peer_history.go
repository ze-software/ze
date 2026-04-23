// Design: plan/spec-diag-2-event-history.md -- per-peer BGP FSM transition history
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

// FSMHistory returns a snapshot of this peer's FSM transition history,
// newest first.
func (p *Peer) FSMHistory() []FSMTransition {
	if p.history == nil {
		return []FSMTransition{}
	}
	return p.history.snapshot()
}
