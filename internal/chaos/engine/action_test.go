package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestActionTypeString verifies string representation of each action type.
//
// VALIDATES: All action types have human-readable names.
// PREVENTS: Missing cases in String() method.
func TestActionTypeString(t *testing.T) {
	tests := []struct {
		action ActionType
		want   string
	}{
		{ActionTCPDisconnect, "tcp-disconnect"},
		{ActionNotificationCease, "notification-cease"},
		{ActionHoldTimerExpiry, "hold-timer-expiry"},
		{ActionDisconnectDuringBurst, "disconnect-during-burst"},
		{ActionReconnectStorm, "reconnect-storm"},
		{ActionConnectionCollision, "connection-collision"},
		{ActionMalformedUpdate, "malformed-update"},
		{ActionConfigReload, "config-reload"},
		{ActionSlowRead, "slow-read"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.action.String())
		})
	}
}

// TestActionTypeStringUnknown verifies unknown action types don't panic.
//
// VALIDATES: Unknown action types produce a reasonable string.
// PREVENTS: Panic on out-of-range action type.
func TestActionTypeStringUnknown(t *testing.T) {
	unknown := ActionType(99)
	s := unknown.String()
	assert.Contains(t, s, "unknown")
}

// TestActionNeedsReconnect verifies reconnect classification for each action type.
//
// VALIDATES: Disconnect-type actions require reconnect; withdrawal actions do not.
// PREVENTS: Missing reconnection after disconnect, or spurious reconnection after withdrawal.
func TestActionNeedsReconnect(t *testing.T) {
	tests := []struct {
		action ActionType
		want   bool
	}{
		{ActionTCPDisconnect, true},
		{ActionNotificationCease, true},
		{ActionHoldTimerExpiry, true},
		{ActionDisconnectDuringBurst, true},
		{ActionReconnectStorm, true},
		{ActionConnectionCollision, false},
		{ActionMalformedUpdate, false},
		{ActionConfigReload, false},
		{ActionSlowRead, false},
	}

	for _, tt := range tests {
		t.Run(tt.action.String(), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.action.NeedsReconnect())
		})
	}
}

// TestActionTypeFromString verifies round-trip parsing of action names.
//
// VALIDATES: All action names parse to the correct ActionType.
// PREVENTS: Missing or wrong entries in the actionTypes map.
func TestActionTypeFromString(t *testing.T) {
	tests := []struct {
		name   string
		want   ActionType
		wantOK bool
	}{
		{NameTCPDisconnect, ActionTCPDisconnect, true},
		{NameNotificationCease, ActionNotificationCease, true},
		{NameHoldTimerExpiry, ActionHoldTimerExpiry, true},
		{NameDisconnectDuringBurst, ActionDisconnectDuringBurst, true},
		{NameReconnectStorm, ActionReconnectStorm, true},
		{NameConnectionCollision, ActionConnectionCollision, true},
		{NameMalformedUpdate, ActionMalformedUpdate, true},
		{NameConfigReload, ActionConfigReload, true},
		{NameSlowRead, ActionSlowRead, true},
		{"unknown-action", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ActionTypeFromString(tt.name)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestChaosActionDefaults verifies zero-value ChaosAction is safe.
//
// VALIDATES: Default ChaosAction has a valid type.
// PREVENTS: Nil or undefined behavior with zero-value struct.
func TestChaosActionDefaults(t *testing.T) {
	var action ChaosAction
	assert.Equal(t, ActionTCPDisconnect, action.Type)
}
