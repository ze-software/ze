package peer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEventTypeString verifies all event types return kebab-case names.
//
// VALIDATES: EventType.String() returns human-readable kebab-case names.
// PREVENTS: Missing event type in String() causing "unknown-N" in logs.
func TestEventTypeString(t *testing.T) {
	tests := []struct {
		typ  EventType
		want string
	}{
		{EventEstablished, "established"},
		{EventRouteSent, "route-sent"},
		{EventRouteReceived, "route-received"},
		{EventRouteWithdrawn, "route-withdrawn"},
		{EventEORSent, "eor-sent"},
		{EventDisconnected, "disconnected"},
		{EventError, "error"},
		{EventChaosExecuted, "chaos-executed"},
		{EventReconnecting, "reconnecting"},
		{EventWithdrawalSent, "withdrawal-sent"},
		{EventRouteAction, "route-action"},
		{EventDroppedEvents, "dropped-events"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.typ.String())
		})
	}
}

// TestEventTypeStringUnknown verifies unknown EventType values return "unknown-N".
//
// VALIDATES: Out-of-range EventType doesn't panic, returns descriptive string.
// PREVENTS: Dashboard or JSON log crashing on unexpected event type.
func TestEventTypeStringUnknown(t *testing.T) {
	unknown := EventType(99)
	assert.Equal(t, "unknown-99", unknown.String())
}

// TestEventTypeStringCompleteness verifies all iota values 0..12 have non-unknown names.
//
// VALIDATES: No gaps in the String() switch — all event types are covered.
// PREVENTS: Adding a new EventType without updating String().
func TestEventTypeStringCompleteness(t *testing.T) {
	for i := range 12 {
		et := EventType(i)
		name := et.String()
		assert.NotEqual(t, fmt.Sprintf("unknown-%d", i), name,
			"EventType(%d) should have a name, not unknown", i)
	}
}

// TestEventBGPMessageBackwardCompat verifies that the zero-value BGPMessage
// field does not affect existing event processing.
//
// VALIDATES: AC-8 — existing consumers work with zero-value BGPMessage.
// PREVENTS: Adding BGPMessage field breaking existing code that copies events.
func TestEventBGPMessageBackwardCompat(t *testing.T) {
	ev := Event{
		Type:      EventRouteSent,
		PeerIndex: 3,
	}
	// Zero-value BGPMessage must be nil
	assert.Nil(t, ev.BGPMessage)

	// Event with BGPMessage set does not interfere with other fields
	evWithMsg := Event{
		Type:       EventRouteSent,
		PeerIndex:  5,
		BGPMessage: []byte{0xFF, 0xFF},
	}
	assert.Equal(t, 5, evWithMsg.PeerIndex)
	assert.Equal(t, EventRouteSent, evWithMsg.Type)
	assert.Equal(t, []byte{0xFF, 0xFF}, evWithMsg.BGPMessage)
}
