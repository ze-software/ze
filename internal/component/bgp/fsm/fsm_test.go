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

// TestFSMExhaustiveTransitions verifies every (state, event) → next state
// combination per RFC 4271 Section 8.2.2.
//
// VALIDATES: All 90+ state×event combinations produce the correct next state,
// including "any other event" cases that RFC requires to transition to Idle.
//
// PREVENTS: Unexpected events being silently ignored instead of resetting the
// FSM to Idle, which could leave a session stuck in an intermediate state.
func TestFSMExhaustiveTransitions(t *testing.T) {
	tests := []struct {
		name    string
		from    State
		passive bool
		event   Event
		to      State
	}{
		// === IDLE state ===
		// RFC 4271 Section 8.2.2: ManualStart → Connect (active) or Active (passive)
		// All other events: no state change (stay Idle)
		{"Idle_ManualStart_active", StateIdle, false, EventManualStart, StateConnect},
		{"Idle_ManualStart_passive", StateIdle, true, EventManualStart, StateActive},
		{"Idle_ManualStop", StateIdle, false, EventManualStop, StateIdle},
		{"Idle_ConnectRetryTimerExpires", StateIdle, false, EventConnectRetryTimerExpires, StateIdle},
		{"Idle_HoldTimerExpires", StateIdle, false, EventHoldTimerExpires, StateIdle},
		{"Idle_KeepaliveTimerExpires", StateIdle, false, EventKeepaliveTimerExpires, StateIdle},
		{"Idle_TCPConnectionConfirmed", StateIdle, false, EventTCPConnectionConfirmed, StateIdle},
		{"Idle_TCPConnectionFails", StateIdle, false, EventTCPConnectionFails, StateIdle},
		{"Idle_BGPOpen", StateIdle, false, EventBGPOpen, StateIdle},
		{"Idle_BGPHeaderErr", StateIdle, false, EventBGPHeaderErr, StateIdle},
		{"Idle_BGPOpenMsgErr", StateIdle, false, EventBGPOpenMsgErr, StateIdle},
		{"Idle_NotifMsgVerErr", StateIdle, false, EventNotifMsgVerErr, StateIdle},
		{"Idle_NotifMsg", StateIdle, false, EventNotifMsg, StateIdle},
		{"Idle_KeepaliveMsg", StateIdle, false, EventKeepaliveMsg, StateIdle},
		{"Idle_UpdateMsg", StateIdle, false, EventUpdateMsg, StateIdle},
		{"Idle_UpdateMsgErr", StateIdle, false, EventUpdateMsgErr, StateIdle},

		// === CONNECT state ===
		// RFC 4271 Section 8.2.2: Start events ignored, TCPConfirmed → OpenSent,
		// TCPFails/errors → Idle, ConnectRetry → Connect, all others → Idle
		{"Connect_ManualStart", StateConnect, false, EventManualStart, StateConnect},
		{"Connect_ManualStop", StateConnect, false, EventManualStop, StateIdle},
		{"Connect_ConnectRetryTimerExpires", StateConnect, false, EventConnectRetryTimerExpires, StateConnect},
		{"Connect_HoldTimerExpires", StateConnect, false, EventHoldTimerExpires, StateIdle},
		{"Connect_KeepaliveTimerExpires", StateConnect, false, EventKeepaliveTimerExpires, StateIdle},
		{"Connect_TCPConnectionConfirmed", StateConnect, false, EventTCPConnectionConfirmed, StateOpenSent},
		{"Connect_TCPConnectionFails", StateConnect, false, EventTCPConnectionFails, StateIdle},
		{"Connect_BGPOpen", StateConnect, false, EventBGPOpen, StateIdle},
		{"Connect_BGPHeaderErr", StateConnect, false, EventBGPHeaderErr, StateIdle},
		{"Connect_BGPOpenMsgErr", StateConnect, false, EventBGPOpenMsgErr, StateIdle},
		{"Connect_NotifMsgVerErr", StateConnect, false, EventNotifMsgVerErr, StateIdle},
		{"Connect_NotifMsg", StateConnect, false, EventNotifMsg, StateIdle},
		{"Connect_KeepaliveMsg", StateConnect, false, EventKeepaliveMsg, StateIdle},
		{"Connect_UpdateMsg", StateConnect, false, EventUpdateMsg, StateIdle},
		{"Connect_UpdateMsgErr", StateConnect, false, EventUpdateMsgErr, StateIdle},

		// === ACTIVE state ===
		// RFC 4271 Section 8.2.2: Start events ignored, TCPConfirmed → OpenSent,
		// ConnectRetry (non-passive) → Connect, ConnectRetry (passive) → Active,
		// TCPFails/errors → Idle, all others → Idle
		{"Active_ManualStart", StateActive, false, EventManualStart, StateActive},
		{"Active_ManualStop", StateActive, false, EventManualStop, StateIdle},
		{"Active_ConnectRetryTimerExpires_nonpassive", StateActive, false, EventConnectRetryTimerExpires, StateConnect},
		{"Active_ConnectRetryTimerExpires_passive", StateActive, true, EventConnectRetryTimerExpires, StateActive},
		{"Active_HoldTimerExpires", StateActive, false, EventHoldTimerExpires, StateIdle},
		{"Active_KeepaliveTimerExpires", StateActive, false, EventKeepaliveTimerExpires, StateIdle},
		{"Active_TCPConnectionConfirmed", StateActive, false, EventTCPConnectionConfirmed, StateOpenSent},
		{"Active_TCPConnectionFails", StateActive, false, EventTCPConnectionFails, StateIdle},
		{"Active_BGPOpen", StateActive, false, EventBGPOpen, StateIdle},
		{"Active_BGPHeaderErr", StateActive, false, EventBGPHeaderErr, StateIdle},
		{"Active_BGPOpenMsgErr", StateActive, false, EventBGPOpenMsgErr, StateIdle},
		{"Active_NotifMsgVerErr", StateActive, false, EventNotifMsgVerErr, StateIdle},
		{"Active_NotifMsg", StateActive, false, EventNotifMsg, StateIdle},
		{"Active_KeepaliveMsg", StateActive, false, EventKeepaliveMsg, StateIdle},
		{"Active_UpdateMsg", StateActive, false, EventUpdateMsg, StateIdle},
		{"Active_UpdateMsgErr", StateActive, false, EventUpdateMsgErr, StateIdle},

		// === OPENSENT state ===
		// RFC 4271 Section 8.2.2: BGPOpen → OpenConfirm, HoldTimer/errors → Idle,
		// TCPFails → Idle (violation: RFC says Active), all others → Idle (FSM Error)
		{"OpenSent_ManualStart", StateOpenSent, false, EventManualStart, StateIdle},
		{"OpenSent_ManualStop", StateOpenSent, false, EventManualStop, StateIdle},
		{"OpenSent_ConnectRetryTimerExpires", StateOpenSent, false, EventConnectRetryTimerExpires, StateIdle},
		{"OpenSent_HoldTimerExpires", StateOpenSent, false, EventHoldTimerExpires, StateIdle},
		{"OpenSent_KeepaliveTimerExpires", StateOpenSent, false, EventKeepaliveTimerExpires, StateIdle},
		{"OpenSent_TCPConnectionConfirmed", StateOpenSent, false, EventTCPConnectionConfirmed, StateIdle},
		{"OpenSent_TCPConnectionFails", StateOpenSent, false, EventTCPConnectionFails, StateIdle},
		{"OpenSent_BGPOpen", StateOpenSent, false, EventBGPOpen, StateOpenConfirm},
		{"OpenSent_BGPHeaderErr", StateOpenSent, false, EventBGPHeaderErr, StateIdle},
		{"OpenSent_BGPOpenMsgErr", StateOpenSent, false, EventBGPOpenMsgErr, StateIdle},
		{"OpenSent_NotifMsgVerErr", StateOpenSent, false, EventNotifMsgVerErr, StateIdle},
		{"OpenSent_NotifMsg", StateOpenSent, false, EventNotifMsg, StateIdle},
		{"OpenSent_KeepaliveMsg", StateOpenSent, false, EventKeepaliveMsg, StateIdle},
		{"OpenSent_UpdateMsg", StateOpenSent, false, EventUpdateMsg, StateIdle},
		{"OpenSent_UpdateMsgErr", StateOpenSent, false, EventUpdateMsgErr, StateIdle},

		// === OPENCONFIRM state ===
		// RFC 4271 Section 8.2.2: KeepaliveMsg → Established, HoldTimer/errors → Idle,
		// TCPFails → Idle, all others → Idle (FSM Error)
		{"OpenConfirm_ManualStart", StateOpenConfirm, false, EventManualStart, StateIdle},
		{"OpenConfirm_ManualStop", StateOpenConfirm, false, EventManualStop, StateIdle},
		{"OpenConfirm_ConnectRetryTimerExpires", StateOpenConfirm, false, EventConnectRetryTimerExpires, StateIdle},
		{"OpenConfirm_HoldTimerExpires", StateOpenConfirm, false, EventHoldTimerExpires, StateIdle},
		{"OpenConfirm_KeepaliveTimerExpires", StateOpenConfirm, false, EventKeepaliveTimerExpires, StateOpenConfirm},
		{"OpenConfirm_TCPConnectionConfirmed", StateOpenConfirm, false, EventTCPConnectionConfirmed, StateIdle},
		{"OpenConfirm_TCPConnectionFails", StateOpenConfirm, false, EventTCPConnectionFails, StateIdle},
		{"OpenConfirm_BGPOpen", StateOpenConfirm, false, EventBGPOpen, StateIdle},
		{"OpenConfirm_BGPHeaderErr", StateOpenConfirm, false, EventBGPHeaderErr, StateIdle},
		{"OpenConfirm_BGPOpenMsgErr", StateOpenConfirm, false, EventBGPOpenMsgErr, StateIdle},
		{"OpenConfirm_NotifMsgVerErr", StateOpenConfirm, false, EventNotifMsgVerErr, StateIdle},
		{"OpenConfirm_NotifMsg", StateOpenConfirm, false, EventNotifMsg, StateIdle},
		{"OpenConfirm_KeepaliveMsg", StateOpenConfirm, false, EventKeepaliveMsg, StateEstablished},
		{"OpenConfirm_UpdateMsg", StateOpenConfirm, false, EventUpdateMsg, StateIdle},
		{"OpenConfirm_UpdateMsgErr", StateOpenConfirm, false, EventUpdateMsgErr, StateIdle},

		// === ESTABLISHED state ===
		// RFC 4271 Section 8.2.2: KeepaliveMsg/UpdateMsg → Established,
		// HoldTimer/errors/TCPFails → Idle, all others → Idle (FSM Error)
		{"Established_ManualStart", StateEstablished, false, EventManualStart, StateIdle},
		{"Established_ManualStop", StateEstablished, false, EventManualStop, StateIdle},
		{"Established_ConnectRetryTimerExpires", StateEstablished, false, EventConnectRetryTimerExpires, StateIdle},
		{"Established_HoldTimerExpires", StateEstablished, false, EventHoldTimerExpires, StateIdle},
		{"Established_KeepaliveTimerExpires", StateEstablished, false, EventKeepaliveTimerExpires, StateEstablished},
		{"Established_TCPConnectionConfirmed", StateEstablished, false, EventTCPConnectionConfirmed, StateIdle},
		{"Established_TCPConnectionFails", StateEstablished, false, EventTCPConnectionFails, StateIdle},
		{"Established_BGPOpen", StateEstablished, false, EventBGPOpen, StateIdle},
		{"Established_BGPHeaderErr", StateEstablished, false, EventBGPHeaderErr, StateIdle},
		{"Established_BGPOpenMsgErr", StateEstablished, false, EventBGPOpenMsgErr, StateIdle},
		{"Established_NotifMsgVerErr", StateEstablished, false, EventNotifMsgVerErr, StateIdle},
		{"Established_NotifMsg", StateEstablished, false, EventNotifMsg, StateIdle},
		{"Established_KeepaliveMsg", StateEstablished, false, EventKeepaliveMsg, StateEstablished},
		{"Established_UpdateMsg", StateEstablished, false, EventUpdateMsg, StateEstablished},
		{"Established_UpdateMsgErr", StateEstablished, false, EventUpdateMsgErr, StateIdle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New()
			f.SetPassive(tt.passive)
			f.setState(tt.from)

			err := f.Event(tt.event)
			require.NoError(t, err)
			require.Equal(t, tt.to, f.State(),
				"from %s + %s: expected %s, got %s",
				tt.from, tt.event, tt.to, f.State())
		})
	}
}

// TestFSMUnexpectedEventCallback verifies that the callback fires when an
// unexpected event causes a transition to Idle.
//
// VALIDATES: RFC 4271 Section 8.2.2 "any other event" transitions invoke the
// state callback so the reactor is notified of session teardown.
//
// PREVENTS: Silent FSM resets where the reactor never learns the session dropped.
func TestFSMUnexpectedEventCallback(t *testing.T) {
	// Each entry picks an event that hits the default→Idle handler in that state.
	tests := []struct {
		state State
		event Event
	}{
		{StateConnect, EventUpdateMsg},                    // no UPDATE before OPEN exchange
		{StateActive, EventUpdateMsg},                     // no UPDATE before OPEN exchange
		{StateOpenSent, EventUpdateMsg},                   // no UPDATE before OPEN confirmed
		{StateOpenConfirm, EventConnectRetryTimerExpires}, // architecturally unreachable
		{StateEstablished, EventConnectRetryTimerExpires}, // architecturally unreachable
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			f := New()
			f.setState(tt.state)

			var called bool
			var fromState, toState State

			f.SetCallback(func(from, to State) {
				called = true
				fromState = from
				toState = to
			})

			err := f.Event(tt.event)
			require.NoError(t, err)
			require.True(t, called, "callback should fire on unexpected event → Idle")
			require.Equal(t, tt.state, fromState)
			require.Equal(t, StateIdle, toState)
		})
	}
}
