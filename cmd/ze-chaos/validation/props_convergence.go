package validation

import (
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// ConvergenceDeadline checks that all announced routes are received by
// their destination peers within a configurable deadline.
type ConvergenceDeadline struct {
	n           int
	deadline    time.Duration
	convergence *Convergence
	lastTime    time.Time
}

// NewConvergenceDeadline creates a convergence-deadline property for n peers.
func NewConvergenceDeadline(n int, deadline time.Duration) *ConvergenceDeadline {
	return &ConvergenceDeadline{
		n:           n,
		deadline:    deadline,
		convergence: NewConvergence(n, deadline),
	}
}

func (p *ConvergenceDeadline) Name() string        { return "convergence-deadline" }
func (p *ConvergenceDeadline) Description() string { return "All routes converge within deadline" }
func (p *ConvergenceDeadline) RFC() string         { return "" }

func (p *ConvergenceDeadline) ProcessEvent(ev peer.Event) {
	p.lastTime = ev.Time
	switch ev.Type { //nolint:exhaustive // only route-sent and route-received are relevant
	case peer.EventRouteSent:
		p.convergence.RecordAnnounce(ev.PeerIndex, ev.Prefix, ev.Time)
	case peer.EventRouteReceived:
		p.convergence.RecordReceive(ev.PeerIndex, ev.Prefix, ev.Time)
	}
}

func (p *ConvergenceDeadline) Violations() []Violation {
	slow := p.convergence.CheckDeadline(p.lastTime)
	violations := make([]Violation, 0, len(slow))
	for _, s := range slow {
		violations = append(violations, Violation{
			Property:  p.Name(),
			Message:   fmt.Sprintf("route %s from peer %d not received by peer %d after %s (deadline %s)", s.Prefix, s.Source, s.Peer, s.Age, p.deadline),
			PeerIndex: s.Peer,
			Time:      p.lastTime,
		})
	}
	return violations
}

func (p *ConvergenceDeadline) Reset() {
	p.convergence = NewConvergence(p.n, p.deadline)
	p.lastTime = time.Time{}
}
