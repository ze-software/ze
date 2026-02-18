// Package chaos implements chaos event scheduling and action types
// for the ze-chaos testing tool.
package chaos

import "fmt"

// ActionType identifies the kind of chaos event.
type ActionType int

const (
	// ActionTCPDisconnect closes the TCP connection immediately.
	ActionTCPDisconnect ActionType = iota
	// ActionNotificationCease sends a NOTIFICATION Cease before closing.
	ActionNotificationCease
	// ActionHoldTimerExpiry stops sending KEEPALIVEs so Ze detects expiry.
	ActionHoldTimerExpiry
	// ActionDisconnectDuringBurst closes the connection during initial route sending.
	ActionDisconnectDuringBurst
	// ActionReconnectStorm disconnects and rapidly reconnects 2 times.
	ActionReconnectStorm
	// ActionConnectionCollision opens a second TCP connection while the first is active.
	ActionConnectionCollision
	// ActionMalformedUpdate sends an UPDATE with invalid attributes (RFC 7606 testing).
	ActionMalformedUpdate
	// ActionConfigReload sends SIGHUP to the Ze process (requires --ze-pid).
	ActionConfigReload
)

// String returns the kebab-case name of the action type.
func (a ActionType) String() string {
	switch a {
	case ActionTCPDisconnect:
		return "tcp-disconnect"
	case ActionNotificationCease:
		return "notification-cease"
	case ActionHoldTimerExpiry:
		return "hold-timer-expiry"
	case ActionDisconnectDuringBurst:
		return "disconnect-during-burst"
	case ActionReconnectStorm:
		return "reconnect-storm"
	case ActionConnectionCollision:
		return "connection-collision"
	case ActionMalformedUpdate:
		return "malformed-update"
	case ActionConfigReload:
		return "config-reload"
	default:
		return fmt.Sprintf("unknown(%d)", a)
	}
}

// ActionTypeFromString parses a kebab-case action name into an ActionType.
// Returns the zero value (ActionTCPDisconnect) and false if the name is unknown.
func ActionTypeFromString(s string) (ActionType, bool) {
	switch s {
	case "tcp-disconnect":
		return ActionTCPDisconnect, true
	case "notification-cease":
		return ActionNotificationCease, true
	case "hold-timer-expiry":
		return ActionHoldTimerExpiry, true
	case "disconnect-during-burst":
		return ActionDisconnectDuringBurst, true
	case "reconnect-storm":
		return ActionReconnectStorm, true
	case "connection-collision":
		return ActionConnectionCollision, true
	case "malformed-update":
		return ActionMalformedUpdate, true
	case "config-reload":
		return ActionConfigReload, true
	default:
		return 0, false
	}
}

// NeedsReconnect returns true if this action type causes a session teardown
// that requires the peer to reconnect afterwards.
func (a ActionType) NeedsReconnect() bool {
	switch a {
	case ActionTCPDisconnect, ActionNotificationCease, ActionHoldTimerExpiry, ActionDisconnectDuringBurst, ActionReconnectStorm:
		return true
	case ActionConnectionCollision, ActionMalformedUpdate, ActionConfigReload:
		return false
	default:
		return false
	}
}

// ChaosAction describes a chaos event to execute on a peer.
type ChaosAction struct {
	// Type identifies what kind of chaos to perform.
	Type ActionType
}
