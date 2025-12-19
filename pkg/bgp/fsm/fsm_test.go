package fsm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStateConstants verifies FSM state constants match RFC 4271 Section 8.
//
// VALIDATES: State values are distinct and use expected bit flags for
// efficient state comparison and logging.
//
// PREVENTS: State value collisions causing incorrect FSM behavior.
func TestStateConstants(t *testing.T) {
	// States should be distinct
	states := []State{
		StateIdle,
		StateConnect,
		StateActive,
		StateOpenSent,
		StateOpenConfirm,
		StateEstablished,
	}

	seen := make(map[State]bool)
	for _, s := range states {
		require.False(t, seen[s], "duplicate state value: %s", s)
		seen[s] = true
	}

	// Verify expected values (bit flags)
	require.Equal(t, State(0x01), StateIdle)
	require.Equal(t, State(0x02), StateActive)
	require.Equal(t, State(0x04), StateConnect)
	require.Equal(t, State(0x08), StateOpenSent)
	require.Equal(t, State(0x10), StateOpenConfirm)
	require.Equal(t, State(0x20), StateEstablished)
}

// TestStateString verifies state string representations.
//
// VALIDATES: States have human-readable names for logging and debugging.
//
// PREVENTS: Cryptic numeric states in logs making debugging difficult.
func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateIdle, "IDLE"},
		{StateConnect, "CONNECT"},
		{StateActive, "ACTIVE"},
		{StateOpenSent, "OPENSENT"},
		{StateOpenConfirm, "OPENCONFIRM"},
		{StateEstablished, "ESTABLISHED"},
		{State(0xFF), "UNKNOWN(255)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.state.String())
		})
	}
}

// TestEventConstants verifies FSM event constants.
//
// VALIDATES: All RFC 4271 events are defined with distinct values.
//
// PREVENTS: Event value collisions causing incorrect state transitions.
func TestEventConstants(t *testing.T) {
	// Events should be distinct
	events := []Event{
		EventManualStart,
		EventManualStop,
		EventConnectRetryTimerExpires,
		EventHoldTimerExpires,
		EventKeepaliveTimerExpires,
		EventTCPConnectionConfirmed,
		EventTCPConnectionFails,
		EventBGPOpen,
		EventBGPHeaderErr,
		EventBGPOpenMsgErr,
		EventNotifMsgVerErr,
		EventNotifMsg,
		EventKeepaliveMsg,
		EventUpdateMsg,
		EventUpdateMsgErr,
	}

	seen := make(map[Event]bool)
	for _, e := range events {
		require.False(t, seen[e], "duplicate event value: %s", e)
		seen[e] = true
	}
}

// TestEventString verifies event string representations.
//
// VALIDATES: Events have human-readable names for logging.
//
// PREVENTS: Cryptic numeric events in logs.
func TestEventString(t *testing.T) {
	require.Equal(t, "ManualStart", EventManualStart.String())
	require.Equal(t, "ManualStop", EventManualStop.String())
	require.Equal(t, "HoldTimerExpires", EventHoldTimerExpires.String())
	require.Equal(t, "BGPOpen", EventBGPOpen.String())
	require.Equal(t, "UNKNOWN(255)", Event(255).String())
}

// TestFSMCreation verifies FSM initialization.
//
// VALIDATES: FSM starts in IDLE state per RFC 4271.
//
// PREVENTS: FSM starting in wrong state causing protocol violations.
func TestFSMCreation(t *testing.T) {
	fsm := New()

	require.Equal(t, StateIdle, fsm.State())
}

// TestFSMTransitionIdleToConnect verifies ManualStart transition.
//
// VALIDATES: RFC 4271 Section 8.2.2 - ManualStart event in IDLE state
// initiates TCP connection (transitions to CONNECT for active peer).
//
// PREVENTS: Peer not starting connection when configured.
func TestFSMTransitionIdleToConnect(t *testing.T) {
	fsm := New()

	err := fsm.Event(EventManualStart)
	require.NoError(t, err)
	require.Equal(t, StateConnect, fsm.State())
}

// TestFSMTransitionConnectToOpenSent verifies TCP connection transition.
//
// VALIDATES: RFC 4271 Section 8.2.2 - TCP connection confirmed in CONNECT
// state sends OPEN and transitions to OPENSENT.
//
// PREVENTS: Missing OPEN message after TCP connection.
func TestFSMTransitionConnectToOpenSent(t *testing.T) {
	fsm := New()
	fsm.setState(StateConnect) // Start in CONNECT

	err := fsm.Event(EventTCPConnectionConfirmed)
	require.NoError(t, err)
	require.Equal(t, StateOpenSent, fsm.State())
}

// TestFSMTransitionOpenSentToOpenConfirm verifies OPEN received transition.
//
// VALIDATES: RFC 4271 Section 8.2.2 - Receiving OPEN in OPENSENT sends
// KEEPALIVE and transitions to OPENCONFIRM.
//
// PREVENTS: Session not progressing after peer OPEN received.
func TestFSMTransitionOpenSentToOpenConfirm(t *testing.T) {
	fsm := New()
	fsm.setState(StateOpenSent)

	err := fsm.Event(EventBGPOpen)
	require.NoError(t, err)
	require.Equal(t, StateOpenConfirm, fsm.State())
}

// TestFSMTransitionOpenConfirmToEstablished verifies KEEPALIVE transition.
//
// VALIDATES: RFC 4271 Section 8.2.2 - Receiving KEEPALIVE in OPENCONFIRM
// transitions to ESTABLISHED (session active).
//
// PREVENTS: Session not becoming active after handshake.
func TestFSMTransitionOpenConfirmToEstablished(t *testing.T) {
	fsm := New()
	fsm.setState(StateOpenConfirm)

	err := fsm.Event(EventKeepaliveMsg)
	require.NoError(t, err)
	require.Equal(t, StateEstablished, fsm.State())
}

// TestFSMTransitionToIdleOnError verifies error handling.
//
// VALIDATES: RFC 4271 - Errors cause transition back to IDLE.
//
// PREVENTS: FSM getting stuck in error state, failing to recover.
func TestFSMTransitionToIdleOnError(t *testing.T) {
	tests := []struct {
		name       string
		startState State
		event      Event
	}{
		{"HoldTimerExpires from OpenSent", StateOpenSent, EventHoldTimerExpires},
		{"HoldTimerExpires from OpenConfirm", StateOpenConfirm, EventHoldTimerExpires},
		{"HoldTimerExpires from Established", StateEstablished, EventHoldTimerExpires},
		{"Notification from Established", StateEstablished, EventNotifMsg},
		{"TCPFails from Connect", StateConnect, EventTCPConnectionFails},
		{"HeaderError from OpenSent", StateOpenSent, EventBGPHeaderErr},
		{"OpenMsgError from OpenSent", StateOpenSent, EventBGPOpenMsgErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm := New()
			fsm.setState(tt.startState)

			err := fsm.Event(tt.event)
			require.NoError(t, err)
			require.Equal(t, StateIdle, fsm.State(), "should transition to IDLE on error")
		})
	}
}

// TestFSMTransitionManualStop verifies manual stop handling.
//
// VALIDATES: ManualStop event from any state transitions to IDLE.
//
// PREVENTS: Unable to stop peer, resource leaks.
func TestFSMTransitionManualStop(t *testing.T) {
	states := []State{
		StateConnect,
		StateActive,
		StateOpenSent,
		StateOpenConfirm,
		StateEstablished,
	}

	for _, s := range states {
		t.Run(s.String(), func(t *testing.T) {
			fsm := New()
			fsm.setState(s)

			err := fsm.Event(EventManualStop)
			require.NoError(t, err)
			require.Equal(t, StateIdle, fsm.State())
		})
	}
}

// TestFSMKeepaliveInEstablished verifies keepalive handling.
//
// VALIDATES: Receiving KEEPALIVE in ESTABLISHED stays in ESTABLISHED.
//
// PREVENTS: Unexpected state change breaking active session.
func TestFSMKeepaliveInEstablished(t *testing.T) {
	fsm := New()
	fsm.setState(StateEstablished)

	err := fsm.Event(EventKeepaliveMsg)
	require.NoError(t, err)
	require.Equal(t, StateEstablished, fsm.State())
}

// TestFSMUpdateInEstablished verifies UPDATE handling.
//
// VALIDATES: Receiving UPDATE in ESTABLISHED stays in ESTABLISHED.
//
// PREVENTS: UPDATE message causing state change.
func TestFSMUpdateInEstablished(t *testing.T) {
	fsm := New()
	fsm.setState(StateEstablished)

	err := fsm.Event(EventUpdateMsg)
	require.NoError(t, err)
	require.Equal(t, StateEstablished, fsm.State())
}

// TestFSMCallback verifies state change callback is called.
//
// VALIDATES: FSM can notify external code of state changes.
//
// PREVENTS: State changes going unnoticed by peer/reactor.
func TestFSMCallback(t *testing.T) {
	fsm := New()

	var callbackCalled bool
	var oldState, newState State

	fsm.SetCallback(func(from, to State) {
		callbackCalled = true
		oldState = from
		newState = to
	})

	err := fsm.Event(EventManualStart)
	require.NoError(t, err)

	require.True(t, callbackCalled, "callback should be called on state change")
	require.Equal(t, StateIdle, oldState)
	require.Equal(t, StateConnect, newState)
}

// TestFSMActiveMode verifies passive peer behavior.
//
// VALIDATES: Passive peers transition to ACTIVE instead of CONNECT.
//
// PREVENTS: Passive peers initiating connections.
func TestFSMActiveMode(t *testing.T) {
	fsm := New()
	fsm.SetPassive(true)

	err := fsm.Event(EventManualStart)
	require.NoError(t, err)
	require.Equal(t, StateActive, fsm.State())
}

// TestFSMActiveToOpenSent verifies incoming connection handling.
//
// VALIDATES: Active mode peer accepts incoming connection.
//
// PREVENTS: Passive peer rejecting valid incoming connections.
func TestFSMActiveToOpenSent(t *testing.T) {
	fsm := New()
	fsm.setState(StateActive)

	err := fsm.Event(EventTCPConnectionConfirmed)
	require.NoError(t, err)
	require.Equal(t, StateOpenSent, fsm.State())
}
