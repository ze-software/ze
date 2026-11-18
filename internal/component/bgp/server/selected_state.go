// Design: docs/architecture/api/process-protocol.md -- selected state projection.
// Related: events.go -- each actual recipient receives its own projection.
package server

import (
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// selectedStatePeer projects already captured authority to one named recipient.
// The owner metadata was resolved by the engine from complete explicit grants;
// a state subscription, a send grant alone or a role at another peer adds none.
// The returned copy is owned by this delivery, and the input remains unchanged.
func selectedStatePeer(peer *plugin.PeerInfo, recipient string, state rpc.SessionState, reason string) plugin.PeerInfo {
	projected := *peer
	projected.SelectedDestination = false
	projected.OrdinaryReplayOwned = false
	if state != rpc.SessionStateUp {
		return projected
	}
	if peer.DestinationSession == 0 {
		return projected
	}
	if peer.SelectedOwner == "" {
		return projected
	}
	// This destination-common advisory does not make an observer an owner.
	// It suppresses auxiliary automatic replay of received losers on this UP.
	projected.OrdinaryReplayOwned = true
	if peer.SelectedOwner != recipient {
		return projected
	}
	projected.SelectedDestination = true
	if reason == rpc.ReasonAttached {
		// Legacy attached observers owe no initial-session replay. A newly
		// eligible ordinary owner instead needs its current selected snapshot.
		projected.InitialReplay = peer.DestinationSession
	}
	return projected
}

// selectedRefreshPeer projects a received control message to its actual recipient.
// The captured Session is descriptive; only its matching ordinary owner is
// eligible, and a refresh MUST NOT start another initial-routing replay.
// The delivery MUST use this copy without mutating the shared peer observation.
func selectedRefreshPeer(peer *plugin.PeerInfo, recipient string, direction rpc.MessageDirection) plugin.PeerInfo {
	projected := *peer
	projected.SelectedDestination = false
	projected.InitialReplay = 0
	if direction != rpc.DirectionReceived {
		return projected
	}
	if peer.DestinationSession == 0 {
		return projected
	}
	if peer.SelectedOwner == "" {
		return projected
	}
	if peer.SelectedOwner != recipient {
		return projected
	}
	projected.SelectedDestination = true
	return projected
}

const selectedStateFormatSuffix = "\x00selected"

// selectedStateFormatKey keeps both state/refresh recipient classes in the
// four-slot cache: JSON/text multiplied by ordinary owner/observer. Monitor keys
// stay unchanged.
func selectedStateFormatKey(encoding string, selected bool) string {
	if selected {
		return encoding + selectedStateFormatSuffix
	}
	return encoding
}
