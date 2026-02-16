package validation

import (
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
)

// HoldTimerEnforcement checks that when a hold-timer-expiry chaos event
// fires, the BGP session is torn down (EventDisconnected follows).
//
// RFC 4271 Section 4.4: "If the Hold Timer expires, the local system
// sends NOTIFICATION, closes the connection, and changes its state to Idle.".
type HoldTimerEnforcement struct {
	n int
	// pendingExpiry tracks peers with hold-timer-expiry chaos that haven't disconnected yet.
	// Maps peer index → time of chaos event.
	pendingExpiry map[int]time.Time
	violations    []Violation
}

// NewHoldTimerEnforcement creates a hold-timer-enforcement property for n peers.
func NewHoldTimerEnforcement(n int) *HoldTimerEnforcement {
	return &HoldTimerEnforcement{
		n:             n,
		pendingExpiry: make(map[int]time.Time),
	}
}

func (p *HoldTimerEnforcement) Name() string { return "hold-timer-enforcement" }
func (p *HoldTimerEnforcement) Description() string {
	return "Session torn down after hold-timer expiry"
}
func (p *HoldTimerEnforcement) RFC() string { return "RFC 4271 Section 4.4" }

func (p *HoldTimerEnforcement) ProcessEvent(ev peer.Event) {
	switch ev.Type { //nolint:exhaustive // only chaos-executed and disconnected are relevant
	case peer.EventChaosExecuted:
		if ev.ChaosAction == "hold-timer-expiry" {
			p.pendingExpiry[ev.PeerIndex] = ev.Time
		}
	case peer.EventDisconnected:
		delete(p.pendingExpiry, ev.PeerIndex)
	}
}

func (p *HoldTimerEnforcement) Violations() []Violation {
	violations := make([]Violation, 0, len(p.pendingExpiry))
	for peerIdx, chaosTime := range p.pendingExpiry {
		violations = append(violations, Violation{
			Property:  p.Name(),
			RFC:       p.RFC(),
			Message:   fmt.Sprintf("peer %d: hold-timer-expiry at %s but session not torn down", peerIdx, chaosTime.Format(time.RFC3339)),
			PeerIndex: peerIdx,
			Time:      chaosTime,
		})
	}
	// Include any already-detected violations.
	return append(violations, p.violations...)
}

func (p *HoldTimerEnforcement) Reset() {
	p.pendingExpiry = make(map[int]time.Time)
	p.violations = nil
}
