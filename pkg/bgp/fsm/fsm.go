package fsm

import (
	"sync"
)

// StateCallback is called when the FSM changes state.
type StateCallback func(from, to State)

// FSM implements the BGP Finite State Machine per RFC 4271 Section 8.
//
// The FSM handles state transitions based on events such as:
// - ManualStart/ManualStop.
// - TCP connection events.
// - BGP message receipt (OPEN, KEEPALIVE, UPDATE, NOTIFICATION).
// - Timer expiration (HoldTimer, KeepaliveTimer, ConnectRetryTimer).
type FSM struct {
	mu sync.RWMutex

	state    State
	passive  bool          // Passive mode (listen only, no outgoing connection)
	callback StateCallback // Called on state change
}

// New creates a new FSM in the IDLE state.
func New() *FSM {
	return &FSM{
		state: StateIdle,
	}
}

// State returns the current FSM state.
func (f *FSM) State() State {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.state
}

// setState changes the state (internal, no callback).
func (f *FSM) setState(s State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = s
}

// SetCallback sets the state change callback.
func (f *FSM) SetCallback(cb StateCallback) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callback = cb
}

// SetPassive sets passive mode (listen only).
func (f *FSM) SetPassive(passive bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.passive = passive
}

// IsPassive returns true if the FSM is in passive mode.
func (f *FSM) IsPassive() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.passive
}

// change transitions to a new state and calls the callback.
func (f *FSM) change(to State) {
	from := f.state
	f.state = to

	if f.callback != nil && from != to {
		// Call callback without holding lock
		cb := f.callback
		f.mu.Unlock()
		cb(from, to)
		f.mu.Lock()
	}
}

// Event processes an FSM event and returns any error.
// State transitions follow RFC 4271 Section 8.
func (f *FSM) Event(event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch f.state {
	case StateIdle:
		f.handleIdle(event)
	case StateConnect:
		f.handleConnect(event)
	case StateActive:
		f.handleActive(event)
	case StateOpenSent:
		f.handleOpenSent(event)
	case StateOpenConfirm:
		f.handleOpenConfirm(event)
	case StateEstablished:
		f.handleEstablished(event)
	}

	return nil
}

// handleIdle processes events in IDLE state.
func (f *FSM) handleIdle(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in IDLE state per RFC 4271.
	case EventManualStart:
		// Start connection attempt
		if f.passive {
			f.change(StateActive)
		} else {
			f.change(StateConnect)
		}
	case EventManualStop:
		// Already idle, no-op
	default:
		// Ignore other events in IDLE state
	}
}

// handleConnect processes events in CONNECT state.
func (f *FSM) handleConnect(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in CONNECT state per RFC 4271.
	case EventManualStop:
		f.change(StateIdle)

	case EventConnectRetryTimerExpires:
		// Retry connection, stay in CONNECT
		// (actual reconnect logic handled externally)

	case EventTCPConnectionConfirmed:
		// TCP connected, send OPEN (done externally), go to OPENSENT
		f.change(StateOpenSent)

	case EventTCPConnectionFails:
		// Connection failed, back to IDLE
		f.change(StateIdle)

	case EventBGPHeaderErr, EventBGPOpenMsgErr, EventNotifMsgVerErr, EventNotifMsg:
		f.change(StateIdle)

	default:
		// Ignore other events in CONNECT state
	}
}

// handleActive processes events in ACTIVE state (passive peer).
func (f *FSM) handleActive(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in ACTIVE state per RFC 4271.
	case EventManualStop:
		f.change(StateIdle)

	case EventTCPConnectionConfirmed:
		// Incoming connection accepted, send OPEN, go to OPENSENT
		f.change(StateOpenSent)

	case EventConnectRetryTimerExpires:
		// May switch to CONNECT mode
		if !f.passive {
			f.change(StateConnect)
		}

	case EventBGPHeaderErr, EventBGPOpenMsgErr, EventNotifMsgVerErr, EventNotifMsg:
		f.change(StateIdle)

	default:
		// Ignore other events in ACTIVE state
	}
}

// handleOpenSent processes events in OPENSENT state.
func (f *FSM) handleOpenSent(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in OPENSENT state per RFC 4271.
	case EventManualStop:
		f.change(StateIdle)

	case EventBGPOpen:
		// Valid OPEN received, send KEEPALIVE (done externally), go to OPENCONFIRM
		f.change(StateOpenConfirm)

	case EventHoldTimerExpires:
		// Hold timer expired waiting for OPEN
		f.change(StateIdle)

	case EventBGPHeaderErr, EventBGPOpenMsgErr, EventNotifMsgVerErr, EventNotifMsg:
		// Error or notification received
		f.change(StateIdle)

	case EventTCPConnectionFails:
		// TCP connection lost
		f.change(StateIdle)

	default:
		// Ignore other events in OPENSENT state
	}
}

// handleOpenConfirm processes events in OPENCONFIRM state.
func (f *FSM) handleOpenConfirm(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in OPENCONFIRM state per RFC 4271.
	case EventManualStop:
		f.change(StateIdle)

	case EventKeepaliveMsg:
		// KEEPALIVE received, session established
		f.change(StateEstablished)

	case EventHoldTimerExpires:
		// Hold timer expired waiting for KEEPALIVE
		f.change(StateIdle)

	case EventNotifMsg, EventNotifMsgVerErr:
		// Notification received
		f.change(StateIdle)

	case EventBGPHeaderErr, EventBGPOpenMsgErr:
		// Error in message
		f.change(StateIdle)

	case EventTCPConnectionFails:
		// TCP connection lost
		f.change(StateIdle)

	default:
		// Ignore other events in OPENCONFIRM state
	}
}

// handleEstablished processes events in ESTABLISHED state.
func (f *FSM) handleEstablished(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in ESTABLISHED state per RFC 4271.
	case EventManualStop:
		f.change(StateIdle)

	case EventKeepaliveMsg:
		// KEEPALIVE received, stay in ESTABLISHED
		// (reset hold timer externally)

	case EventUpdateMsg:
		// UPDATE received, stay in ESTABLISHED
		// (process update externally)

	case EventHoldTimerExpires:
		// Hold timer expired, peer is dead
		f.change(StateIdle)

	case EventNotifMsg, EventNotifMsgVerErr:
		// Notification received, session ends
		f.change(StateIdle)

	case EventUpdateMsgErr:
		// UPDATE error, send NOTIFICATION (done externally), close
		f.change(StateIdle)

	case EventBGPHeaderErr:
		// Header error
		f.change(StateIdle)

	case EventTCPConnectionFails:
		// TCP connection lost
		f.change(StateIdle)

	default:
		// Ignore other events in ESTABLISHED state
	}
}
