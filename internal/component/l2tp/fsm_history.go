// Design: plan/spec-diag-2-event-history.md -- L2TP tunnel/session FSM history

package l2tp

import "time"

const fsmHistoryCapacity = 16

// FSMTransition records one L2TP tunnel or session FSM state change.
type FSMTransition struct {
	Timestamp time.Time
	From      string
	To        string
	Trigger   string
}

// fsmHistoryRing is a fixed-size circular buffer of FSM transitions.
// NOT safe for concurrent use; callers must hold the reactor's tunnelsMu.
type fsmHistoryRing struct {
	records []FSMTransition
	head    int
	count   int
}

func newFSMHistoryRing() *fsmHistoryRing {
	return &fsmHistoryRing{records: make([]FSMTransition, fsmHistoryCapacity)}
}

func (h *fsmHistoryRing) append(t FSMTransition) {
	h.records[h.head] = t
	h.head = (h.head + 1) % len(h.records)
	if h.count < len(h.records) {
		h.count++
	}
}

func (h *fsmHistoryRing) snapshot() []FSMTransition {
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
