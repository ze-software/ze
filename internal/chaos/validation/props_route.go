// Design: docs/architecture/chaos-web-dashboard.md — property-based validation

package validation

import (
	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// RouteConsistency checks that after convergence, every eligible peer has
// every expected route. Eligible sources' announcements should match what
// each peer received.
//
// RFC 4271 Section 9: UPDATE processing.
type RouteConsistency struct {
	n       int
	model   *Model
	tracker *Tracker
}

// NewRouteConsistency creates a route-consistency property for n peers.
func NewRouteConsistency(n int) *RouteConsistency {
	return newRouteConsistency(n, nil)
}

func newRouteConsistency(n int, reachable Reachability) *RouteConsistency {
	return &RouteConsistency{n: n, model: NewModelWithReachability(n, reachable), tracker: NewTracker(n)}
}

func (p *RouteConsistency) Name() string { return "route-consistency" }
func (p *RouteConsistency) Description() string {
	return "Every eligible peer has every expected route"
}
func (p *RouteConsistency) RFC() string { return "RFC 4271 Section 9" }

func (p *RouteConsistency) ProcessEvent(ev peer.Event) {
	// This independent property observes route consistency inputs.
	//exhaustive:ignore
	switch ev.Type {
	case peer.EventEstablished:
		p.model.SetEstablished(ev.PeerIndex, true)
	case peer.EventRouteSent:
		p.model.AnnounceFamily(ev.PeerIndex, ev.Prefix, ev.Family)
	case peer.EventRouteReceived:
		p.tracker.RecordReceive(ev.PeerIndex, ev.Prefix)
	case peer.EventWithdrawalSent:
		for _, prefix := range ev.WithdrawnPrefixes {
			p.model.Withdraw(ev.PeerIndex, prefix)
		}
	case peer.EventRouteWithdrawn:
		p.tracker.RecordWithdraw(ev.PeerIndex, ev.Prefix)
	case peer.EventDisconnected:
		p.model.Disconnect(ev.PeerIndex)
		p.tracker.ClearPeer(ev.PeerIndex)
	}
}

func (p *RouteConsistency) Violations() []Violation {
	result := Check(p.model, p.tracker)
	if result.Pass {
		return nil
	}
	var violations []Violation
	for i, pr := range result.Peers {
		for _, pfx := range pr.Missing.All() {
			violations = append(violations, Violation{
				Property: p.Name(),
				RFC:      p.RFC(),
				Message: func() string {
					var b textbuf.Buffer
					return b.Reset().Str("peer ").Int(int64(i)).Str(" missing route ").Str(pfx.String()).String()
				}(),
				PeerIndex: i,
			})
		}
		for _, pfx := range pr.Extra.All() {
			violations = append(violations, Violation{
				Property: p.Name(),
				RFC:      p.RFC(),
				Message: func() string {
					var b textbuf.Buffer
					return b.Reset().Str("peer ").Int(int64(i)).Str(" has unexpected route ").Str(pfx.String()).String()
				}(),
				PeerIndex: i,
			})
		}
	}
	return violations
}

func (p *RouteConsistency) Reset() {
	p.model = NewModelWithReachability(p.n, p.model.reachable)
	p.tracker = NewTracker(p.n)
}
