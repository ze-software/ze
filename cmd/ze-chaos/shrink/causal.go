// Package shrink provides test case minimization for ze-chaos.
//
// Given a failing event log, the shrink engine finds the smallest
// subsequence that still triggers the same property violation.
package shrink

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// RemoveWithDependents returns a copy of events with the event at removeIdx
// removed, along with any subsequent events whose preconditions are broken
// by the removal. This preserves causal consistency: route events require
// an established peer, withdrawals require a prior announcement, etc.
func RemoveWithDependents(events []peer.Event, removeIdx int) []peer.Event {
	if removeIdx < 0 || removeIdx >= len(events) {
		return events
	}

	// Track which peers are established as we process events.
	established := make(map[int]bool)

	result := make([]peer.Event, 0, len(events))

	for j, ev := range events {
		if j == removeIdx {
			// Skip the removed event without updating state.
			continue
		}

		// For events after the removal point, check preconditions.
		if j > removeIdx {
			skip := false
			switch ev.Type { //nolint:exhaustive // only relevant types checked
			case peer.EventRouteSent, peer.EventRouteReceived, peer.EventRouteWithdrawn,
				peer.EventEORSent, peer.EventWithdrawalSent, peer.EventChaosExecuted,
				peer.EventError:
				// These all require the peer to be established.
				if !established[ev.PeerIndex] {
					skip = true
				}
			case peer.EventDisconnected:
				// Can't disconnect a peer that isn't connected.
				if !established[ev.PeerIndex] {
					skip = true
				}
			}
			if skip {
				continue
			}
		}

		// Update tracked state.
		switch ev.Type { //nolint:exhaustive // only state-changing types matter
		case peer.EventEstablished:
			established[ev.PeerIndex] = true
		case peer.EventDisconnected:
			established[ev.PeerIndex] = false
		}

		result = append(result, ev)
	}

	return result
}
