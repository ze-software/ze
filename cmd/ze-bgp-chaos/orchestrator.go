package main

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/validation"
)

// EventProcessor routes peer events to the validation model, tracker,
// and convergence tracker. It also maintains aggregate counters.
type EventProcessor struct {
	Model       *validation.Model
	Tracker     *validation.Tracker
	Convergence *validation.Convergence

	// Aggregate counters for the exit summary.
	Announced int
	Received  int
}

// Process handles a single peer event, updating all validation components.
func (ep *EventProcessor) Process(ev peer.Event) {
	switch ev.Type {
	case peer.EventEstablished:
		ep.Model.SetEstablished(ev.PeerIndex, true)

	case peer.EventRouteSent:
		ep.Model.Announce(ev.PeerIndex, ev.Prefix)
		ep.Convergence.RecordAnnounce(ev.PeerIndex, ev.Prefix, ev.Time)
		ep.Announced++

	case peer.EventRouteReceived:
		ep.Tracker.RecordReceive(ev.PeerIndex, ev.Prefix)
		ep.Convergence.RecordReceive(ev.PeerIndex, ev.Prefix, ev.Time)
		ep.Received++

	case peer.EventRouteWithdrawn:
		ep.Tracker.RecordWithdraw(ev.PeerIndex, ev.Prefix)

	case peer.EventDisconnected:
		ep.Model.Disconnect(ev.PeerIndex)
		ep.Tracker.ClearPeer(ev.PeerIndex)

	case peer.EventEORSent, peer.EventError:
		// EOR is informational; errors are logged by the caller.
	}
}
