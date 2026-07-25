// Design: docs/architecture/chaos-web-dashboard.md — property-based validation

package validation

import (
	"time"

	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/core/textbuf"
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
		p.convergence.RecordAnnounce(ev.PeerIndex, ev.Prefix, ev.Time, ev.Family)
	case peer.EventRouteReceived:
		p.convergence.RecordReceive(ev.PeerIndex, ev.Prefix, ev.Time)
	}
}

func (p *ConvergenceDeadline) Violations() []Violation {
	slow := p.convergence.CheckDeadline(p.lastTime)
	violations := make([]Violation, 0, len(slow))
	for _, s := range slow {
		var b textbuf.Buffer
		violations = append(violations, Violation{
			Property:  p.Name(),
			Message:   b.Reset().Str("route ").Str(s.Prefix.String()).Str(" from peer ").Int(int64(s.Source)).Str(" not received by peer ").Int(int64(s.Peer)).Str(" after ").Str(s.Age.String()).Str(" (deadline ").Str(p.deadline.String()).Byte(')').String(),
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
