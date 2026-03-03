// Design: docs/architecture/chaos-web-dashboard.md — property-based validation

package validation

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/chaos/peer"
)

// MessageOrdering checks that route events only occur after a peer has
// reached Established state. Receiving or sending routes before session
// establishment indicates a protocol violation.
//
// RFC 4271 Section 8.2.2: OPEN must complete before UPDATEs.
type MessageOrdering struct {
	n           int
	established []bool
	violations  []Violation
}

// NewMessageOrdering creates a message-ordering property for n peers.
func NewMessageOrdering(n int) *MessageOrdering {
	return &MessageOrdering{
		n:           n,
		established: make([]bool, n),
	}
}

func (p *MessageOrdering) Name() string        { return "message-ordering" }
func (p *MessageOrdering) Description() string { return "No route events before session established" }
func (p *MessageOrdering) RFC() string         { return "RFC 4271 Section 8.2.2" }

func (p *MessageOrdering) ProcessEvent(ev peer.Event) {
	if ev.PeerIndex < 0 || ev.PeerIndex >= p.n {
		return
	}
	switch ev.Type { //nolint:exhaustive // only session and route events are relevant
	case peer.EventEstablished:
		p.established[ev.PeerIndex] = true
	case peer.EventDisconnected:
		p.established[ev.PeerIndex] = false
	case peer.EventRouteSent, peer.EventRouteReceived:
		if !p.established[ev.PeerIndex] {
			p.violations = append(p.violations, Violation{
				Property:  p.Name(),
				RFC:       p.RFC(),
				Message:   fmt.Sprintf("peer %d: route event %s before established", ev.PeerIndex, ev.Type),
				PeerIndex: ev.PeerIndex,
				Time:      ev.Time,
			})
		}
	}
}

func (p *MessageOrdering) Violations() []Violation { return p.violations }

func (p *MessageOrdering) Reset() {
	p.established = make([]bool, p.n)
	p.violations = nil
}
