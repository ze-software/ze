package validation

import (
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
)

// NoDuplicateRoutes checks that no peer announces the same prefix twice
// without an intermediate withdrawal. Double-announces indicate bugs in
// the scenario generator or route reflector.
type NoDuplicateRoutes struct {
	n          int
	announced  []map[netip.Prefix]bool
	violations []Violation
}

// NewNoDuplicateRoutes creates a no-duplicate-routes property for n peers.
func NewNoDuplicateRoutes(n int) *NoDuplicateRoutes {
	announced := make([]map[netip.Prefix]bool, n)
	for i := range n {
		announced[i] = make(map[netip.Prefix]bool)
	}
	return &NoDuplicateRoutes{n: n, announced: announced}
}

func (p *NoDuplicateRoutes) Name() string { return "no-duplicate-routes" }
func (p *NoDuplicateRoutes) Description() string {
	return "No double-announce without intermediate withdrawal"
}
func (p *NoDuplicateRoutes) RFC() string { return "" }

func (p *NoDuplicateRoutes) ProcessEvent(ev peer.Event) {
	if ev.PeerIndex < 0 || ev.PeerIndex >= p.n {
		return
	}
	switch ev.Type { //nolint:exhaustive // only route-sent and disconnected are relevant
	case peer.EventRouteSent:
		if p.announced[ev.PeerIndex][ev.Prefix] {
			p.violations = append(p.violations, Violation{
				Property:  p.Name(),
				Message:   fmt.Sprintf("peer %d announced %s twice without withdrawal", ev.PeerIndex, ev.Prefix),
				PeerIndex: ev.PeerIndex,
				Time:      ev.Time,
			})
		}
		p.announced[ev.PeerIndex][ev.Prefix] = true
	case peer.EventDisconnected:
		p.announced[ev.PeerIndex] = make(map[netip.Prefix]bool)
	}
}

func (p *NoDuplicateRoutes) Violations() []Violation { return p.violations }

func (p *NoDuplicateRoutes) Reset() {
	for i := range p.n {
		p.announced[i] = make(map[netip.Prefix]bool)
	}
	p.violations = nil
}
