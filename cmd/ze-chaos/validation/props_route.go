// Design: docs/architecture/chaos-web-dashboard.md — property-based validation

package validation

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// RouteConsistency checks that after convergence, every eligible peer has
// every expected route. This is the core validation: the union of all other
// established peers' announced routes should match what each peer received.
//
// RFC 4271 Section 9: UPDATE processing.
type RouteConsistency struct {
	n       int
	model   *Model
	tracker *Tracker
}

// NewRouteConsistency creates a route-consistency property for n peers.
func NewRouteConsistency(n int) *RouteConsistency {
	return &RouteConsistency{n: n, model: NewModel(n), tracker: NewTracker(n)}
}

func (p *RouteConsistency) Name() string { return "route-consistency" }
func (p *RouteConsistency) Description() string {
	return "Every eligible peer has every expected route"
}
func (p *RouteConsistency) RFC() string { return "RFC 4271 Section 9" }

func (p *RouteConsistency) ProcessEvent(ev peer.Event) {
	switch ev.Type { //nolint:exhaustive // only session and route events are relevant
	case peer.EventEstablished:
		p.model.SetEstablished(ev.PeerIndex, true)
	case peer.EventRouteSent:
		p.model.Announce(ev.PeerIndex, ev.Prefix)
	case peer.EventRouteReceived:
		p.tracker.RecordReceive(ev.PeerIndex, ev.Prefix)
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
				Property:  p.Name(),
				RFC:       p.RFC(),
				Message:   fmt.Sprintf("peer %d missing route %s", i, pfx),
				PeerIndex: i,
			})
		}
		for _, pfx := range pr.Extra.All() {
			violations = append(violations, Violation{
				Property:  p.Name(),
				RFC:       p.RFC(),
				Message:   fmt.Sprintf("peer %d has unexpected route %s", i, pfx),
				PeerIndex: i,
			})
		}
	}
	return violations
}

func (p *RouteConsistency) Reset() {
	p.model = NewModel(p.n)
	p.tracker = NewTracker(p.n)
}
