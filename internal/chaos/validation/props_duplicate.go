// Design: docs/architecture/chaos-web-dashboard.md — property-based validation

package validation

import (
	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// NoDuplicateRoutes checks that no peer announces the same route twice
// without an intermediate withdrawal. Double-announces indicate bugs in
// the scenario generator or route server.
type NoDuplicateRoutes struct {
	n          int
	announced  []map[routeKey]bool
	violations []Violation
}

// NewNoDuplicateRoutes creates a no-duplicate-routes property for n peers.
func NewNoDuplicateRoutes(n int) *NoDuplicateRoutes {
	announced := make([]map[routeKey]bool, n)
	for i := range n {
		announced[i] = make(map[routeKey]bool)
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
	// This independent property tracks only duplicate-announcement history.
	//exhaustive:ignore
	switch ev.Type {
	case peer.EventRouteSent:
		route := makeRouteKey(ev.Prefix, ev.Family)
		if p.announced[ev.PeerIndex][route] {
			p.violations = append(p.violations, Violation{
				Property: p.Name(),
				Message: func() string {
					var b textbuf.Buffer
					return b.Reset().Str("peer ").Int(int64(ev.PeerIndex)).Str(" announced ").Str(route.label()).Str(" twice without withdrawal").String()
				}(),
				PeerIndex: ev.PeerIndex,
				Time:      ev.Time,
			})
		}
		p.announced[ev.PeerIndex][route] = true
	case peer.EventRouteWithdrawn:
		delete(p.announced[ev.PeerIndex], makeRouteKey(ev.Prefix, ev.Family))
	case peer.EventWithdrawalSent:
		for _, prefix := range ev.WithdrawnPrefixes {
			delete(p.announced[ev.PeerIndex], makeRouteKey(prefix, ev.Family))
		}
	case peer.EventDisconnected:
		clear(p.announced[ev.PeerIndex])
	}
}

func (p *NoDuplicateRoutes) Violations() []Violation { return p.violations }

func (p *NoDuplicateRoutes) Reset() {
	for i := range p.n {
		clear(p.announced[i])
	}
	p.violations = nil
}
