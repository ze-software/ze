// Design: rfc/short/rfc5880.md -- operator observability into live sessions
// Overview: engine.go -- Loop struct and session registry
// Related: loop.go -- express-loop that mutates the state Snapshot reads
//
// Snapshot copies live session state out of the engine for read-only
// consumers (CLI show commands, Prometheus gauges, external tools).
// Every call allocates a fresh slice; the returned values are Go copies
// of RFC 5880 Section 6.8.1 state variables so readers never hold a
// pointer into a running session.Machine.
package engine

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
)

// Snapshot returns a read-only copy of every live session's observable
// state. Sessions are sorted by (mode, vrf, peer) so `show bfd sessions`
// produces deterministic output across calls.
//
// Safe for concurrent use. The method acquires l.mu briefly to walk the
// session map, then builds the result outside the lock.
func (l *Loop) Snapshot() []api.SessionState {
	l.mu.Lock()
	out := make([]api.SessionState, 0, len(l.sessions))
	for _, entry := range l.sessions {
		out = append(out, entry.snapshot())
	}
	l.mu.Unlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Mode != out[j].Mode {
			return out[i].Mode < out[j].Mode
		}
		if out[i].VRF != out[j].VRF {
			return out[i].VRF < out[j].VRF
		}
		return out[i].Peer < out[j].Peer
	})
	return out
}

// SessionDetail returns the SessionState for a single session matched by
// peer address. Comparison is case-insensitive on the string form of
// api.Key.Peer, which matches how an operator would spell the argument
// to `show bfd session <peer>`. The bool return is false if no session
// with that peer currently exists.
//
// Safe for concurrent use.
func (l *Loop) SessionDetail(peer string) (api.SessionState, bool) {
	target := strings.ToLower(peer)
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, entry := range l.sessions {
		if strings.ToLower(key.Peer.String()) == target {
			return entry.snapshot(), true
		}
	}
	return api.SessionState{}, false
}

// snapshot copies this entry into an api.SessionState. Called while
// l.mu is held; the method never touches locks itself.
func (e *sessionEntry) snapshot() api.SessionState {
	m := e.machine
	key := m.Key()
	s := api.SessionState{
		Peer:              key.Peer.String(),
		VRF:               key.VRF,
		Mode:              key.Mode.String(),
		Interface:         key.Interface,
		State:             m.State().String(),
		Diag:              m.LocalDiag().String(),
		LocalDiscr:        m.LocalDiscriminator(),
		RemoteDiscr:       m.RemoteDiscriminator(),
		TxInterval:        m.TransmitInterval(),
		RxInterval:        m.RemoteMinRxInterval(),
		DetectionInterval: m.DetectionInterval(),
		DetectMult:        m.DetectMult(),
		LastReceived:      m.LastReceived(),
		CreatedAt:         e.createdAt,
		Refcount:          m.Refcount(),
		Profile:           e.profile,
		TxPackets:         e.txPackets,
		RxPackets:         e.rxPackets,
	}
	if key.Local.IsValid() {
		s.Local = key.Local.String()
	}
	if len(e.transitions) > 0 {
		s.Transitions = make([]api.TransitionRecord, len(e.transitions))
		copy(s.Transitions, e.transitions)
	}
	return s
}
