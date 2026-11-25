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
	return newConvergenceDeadline(n, deadline, nil)
}

func newConvergenceDeadline(n int, deadline time.Duration, reachable Reachability) *ConvergenceDeadline {
	return &ConvergenceDeadline{
		n:           n,
		deadline:    deadline,
		convergence: NewConvergenceWithReachability(n, deadline, reachable),
	}
}

func (p *ConvergenceDeadline) Name() string        { return "convergence-deadline" }
func (p *ConvergenceDeadline) Description() string { return "All routes converge within deadline" }
func (p *ConvergenceDeadline) RFC() string         { return "" }

func (p *ConvergenceDeadline) ProcessEvent(ev peer.Event) {
	if ev.Time.After(p.lastTime) {
		p.lastTime = ev.Time
	}
	// Observe events that create, resolve, or cancel delivery expectations.
	//exhaustive:ignore
	switch ev.Type {
	case peer.EventRouteSent:
		if ev.Prefix.IsValid() {
			p.convergence.RecordAnnounce(ev.PeerIndex, ev.Prefix, ev.Time, ev.Family)
		}
	case peer.EventRouteReceived:
		p.convergence.recordReceive(ev.PeerIndex, makeRouteKey(ev.Prefix, ev.Family), ev.Time)
	case peer.EventWithdrawalSent:
		for _, prefix := range ev.WithdrawnPrefixes {
			p.convergence.recordWithdraw(ev.PeerIndex, makeRouteKey(prefix, ev.Family), ev.Time)
		}
	case peer.EventDisconnected:
		p.convergence.disconnect(ev.PeerIndex, ev.Time)
	}
}

func (p *ConvergenceDeadline) Violations() []Violation {
	slow := p.convergence.CheckDeadline(p.lastTime)
	violations := make([]Violation, 0, len(slow))
	for _, s := range slow {
		var b textbuf.Buffer
		b.Str("route ")
		if s.Family.AFI != 0 {
			b.Str(routeFamilyName(s.Family)).Byte(' ')
		}
		b.Str(s.Prefix.String()).Str(" from peer ").Int(int64(s.Source))
		if s.Received {
			b.Str(" received late by peer ")
		} else {
			b.Str(" not received by peer ")
		}
		b.Int(int64(s.Peer)).Str(" after ").Str(s.Age.String())
		b.Str(" (deadline ").Str(p.deadline.String()).Byte(')')
		violations = append(violations, Violation{
			Property:  p.Name(),
			Message:   b.String(),
			PeerIndex: s.Peer,
			Time:      p.lastTime,
		})
	}
	return violations
}

func (p *ConvergenceDeadline) Reset() {
	p.convergence = NewConvergenceWithReachability(p.n, p.deadline, p.convergence.reachable)
	p.lastTime = time.Time{}
}
