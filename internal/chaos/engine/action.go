// Design: docs/architecture/chaos-web-dashboard.md — chaos action scheduling
//
// Package chaos implements chaos event scheduling and action types
// for the ze-chaos testing tool.
package engine

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

// Kebab-case action names used in JSON events, web UI, and CLI.
const (
	NameTCPDisconnect         = "tcp-disconnect"
	NameNotificationCease     = "notification-cease"
	NameHoldTimerExpiry       = "hold-timer-expiry"
	NameDisconnectDuringBurst = "disconnect-during-burst"
	NameReconnectStorm        = "reconnect-storm"
	NameConnectionCollision   = "connection-collision"
	NameMalformedUpdate       = "malformed-update"
	NameConfigReload          = "config-reload"
)

// String returns the kebab-case name of the action type.
func (a ActionType) String() string {
	switch a {
	case ActionTCPDisconnect:
		return NameTCPDisconnect
	case ActionNotificationCease:
		return NameNotificationCease
	case ActionHoldTimerExpiry:
		return NameHoldTimerExpiry
	case ActionDisconnectDuringBurst:
		return NameDisconnectDuringBurst
	case ActionReconnectStorm:
		return NameReconnectStorm
	case ActionConnectionCollision:
		return NameConnectionCollision
	case ActionMalformedUpdate:
		return NameMalformedUpdate
	case ActionConfigReload:
		return NameConfigReload
	default:
		return fmt.Sprintf("unknown(%d)", a)
	}
}

// ActionTypeFromString parses a kebab-case action name into an ActionType.
// Returns the zero value (ActionTCPDisconnect) and false if the name is unknown.
func ActionTypeFromString(s string) (ActionType, bool) {
	switch s {
	case NameTCPDisconnect:
		return ActionTCPDisconnect, true
	case NameNotificationCease:
		return ActionNotificationCease, true
	case NameHoldTimerExpiry:
		return ActionHoldTimerExpiry, true
	case NameDisconnectDuringBurst:
		return ActionDisconnectDuringBurst, true
	case NameReconnectStorm:
		return ActionReconnectStorm, true
	case NameConnectionCollision:
		return ActionConnectionCollision, true
	case NameMalformedUpdate:
		return ActionMalformedUpdate, true
	case NameConfigReload:
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
