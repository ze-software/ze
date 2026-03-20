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
	// ActionSlowRead toggles slow reading to create TCP backpressure on Ze's writes.
	ActionSlowRead
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
	NameSlowRead              = "slow-read"
)

// actionNames maps ActionType to kebab-case name. Package-level to avoid
// allocation on every String() call.
var actionNames = map[ActionType]string{
	ActionTCPDisconnect:         NameTCPDisconnect,
	ActionNotificationCease:     NameNotificationCease,
	ActionHoldTimerExpiry:       NameHoldTimerExpiry,
	ActionDisconnectDuringBurst: NameDisconnectDuringBurst,
	ActionReconnectStorm:        NameReconnectStorm,
	ActionConnectionCollision:   NameConnectionCollision,
	ActionMalformedUpdate:       NameMalformedUpdate,
	ActionConfigReload:          NameConfigReload,
	ActionSlowRead:              NameSlowRead,
}

// actionTypes maps kebab-case name to ActionType. Package-level to avoid
// allocation on every ActionTypeFromString() call.
var actionTypes = map[string]ActionType{
	NameTCPDisconnect:         ActionTCPDisconnect,
	NameNotificationCease:     ActionNotificationCease,
	NameHoldTimerExpiry:       ActionHoldTimerExpiry,
	NameDisconnectDuringBurst: ActionDisconnectDuringBurst,
	NameReconnectStorm:        ActionReconnectStorm,
	NameConnectionCollision:   ActionConnectionCollision,
	NameMalformedUpdate:       ActionMalformedUpdate,
	NameConfigReload:          ActionConfigReload,
	NameSlowRead:              ActionSlowRead,
}

// actionReconnect tracks which action types cause session teardown.
var actionReconnect = map[ActionType]bool{
	ActionTCPDisconnect:         true,
	ActionNotificationCease:     true,
	ActionHoldTimerExpiry:       true,
	ActionDisconnectDuringBurst: true,
	ActionReconnectStorm:        true,
}

// String returns the kebab-case name of the action type.
func (a ActionType) String() string {
	if name, ok := actionNames[a]; ok {
		return name
	}
	return fmt.Sprintf("unknown(%d)", a)
}

// ActionTypeFromString parses a kebab-case action name into an ActionType.
// Returns the zero value (ActionTCPDisconnect) and false if the name is unknown.
func ActionTypeFromString(s string) (ActionType, bool) {
	if t, ok := actionTypes[s]; ok {
		return t, true
	}
	return 0, false
}

// NeedsReconnect returns true if this action type causes a session teardown
// that requires the peer to reconnect afterwards.
func (a ActionType) NeedsReconnect() bool {
	return actionReconnect[a]
}

// ChaosAction describes a chaos event to execute on a peer.
type ChaosAction struct {
	// Type identifies what kind of chaos to perform.
	Type ActionType
}
