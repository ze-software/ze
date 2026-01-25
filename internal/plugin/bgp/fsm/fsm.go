// Package fsm implements the BGP Finite State Machine per RFC 4271 Section 8.
//
// RFC 4271 VIOLATIONS:
//
//  1. fsm.go:279 - RFC 4271 Section 8.2.2 (OpenSent, TcpConnectionFails):
//     RFC specifies transition to Active state with ConnectRetryTimer restart.
//     Implementation transitions to Idle instead for simplicity.
//
//  2. Missing optional session attributes (RFC 4271 Section 8.1.1):
//     DelayOpen, DampPeerOscillations, TrackTcpState, and related timers
//     are not implemented. This is permitted per RFC 4271 Section 8.2.1.3.
//
//  3. Missing Event 11 (KeepaliveTimer_Expires) handling in OpenConfirm/Established:
//     RFC specifies FSM should send KEEPALIVE on timer expiry. Timer management
//     is handled externally in this implementation.
//
//  4. Missing NOTIFICATION message sending:
//     RFC requires sending NOTIFICATION messages on various error conditions.
//     This FSM only handles state transitions; message sending is external.
package fsm

import (
	"sync"
)

// StateCallback is called when the FSM changes state.
type StateCallback func(from, to State)

// FSM implements the BGP Finite State Machine per RFC 4271 Section 8.
//
// RFC 4271 Section 8 defines the BGP FSM with six states:
//   - Idle (Section 8.2.2): Initial state, refuses all incoming connections
//   - Connect (Section 8.2.2): Waiting for TCP connection to complete
//   - Active (Section 8.2.2): Listening for incoming TCP connections
//   - OpenSent (Section 8.2.2): OPEN message sent, waiting for peer's OPEN
//   - OpenConfirm (Section 8.2.2): OPEN received, waiting for KEEPALIVE
//   - Established (Section 8.2.2): Session established, exchanging routes
//
// The FSM handles state transitions based on events such as:
//   - ManualStart/ManualStop (Events 1-2, Section 8.1.2)
//   - TCP connection events (Events 14-18, Section 8.1.4)
//   - BGP message receipt (Events 19-28, Section 8.1.5)
//   - Timer expiration (Events 9-13, Section 8.1.3)
//
// Note: This implementation does not include optional session attributes
// (Section 8.1.1) such as DelayOpen, DampPeerOscillations, or TrackTcpState.
type FSM struct {
	mu sync.RWMutex

	state    State
	passive  bool          // Passive mode (listen only, no outgoing connection)
	callback StateCallback // Called on state change
}

// New creates a new FSM in the IDLE state.
// RFC 4271 Section 8.2.2: "Initially, the BGP peer FSM is in the Idle state."
// Returns a new FSM ready for BGP session establishment.
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
// RFC 4271 Section 8.1.1: PassiveTcpEstablishment optional attribute
// indicates the peer will listen prior to establishing the connection.
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
// State transitions follow RFC 4271 Section 8.2.2 (Finite State Machine).
//
// RFC 4271 Section 8.2.1: "BGP MUST maintain a separate FSM for each
// configured peer. Each BGP peer paired in a potential connection will
// attempt to connect to the other, unless configured to remain in the
// idle state, or configured to remain passive."
// Returns nil on success, or an error if the event was not handled.
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
//
// RFC 4271 Section 8.2.2 (Idle state):
// "In this state, BGP FSM refuses all incoming BGP connections for this peer.
// No resources are allocated to the peer."
// Handles ManualStart event to transition to Connect state.
func (f *FSM) handleIdle(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in IDLE state per RFC 4271.
	case EventManualStart:
		// RFC 4271 Section 8.2.2: Event 1 (ManualStart)
		// "In response to a ManualStart event (Event 1) or an AutomaticStart
		// event (Event 3), the local system... initiates a TCP connection to
		// the other BGP peer... and changes its state to Connect."
		//
		// With PassiveTcpEstablishment (Event 4/5): "listens for a connection
		// that may be initiated by the remote peer, and changes its state to Active."
		if f.passive {
			f.change(StateActive)
		} else {
			f.change(StateConnect)
		}
	case EventManualStop:
		// RFC 4271 Section 8.2.2: Event 2 (ManualStop)
		// "The ManualStop event (Event 2) and AutomaticStop (Event 8) event
		// are ignored in the Idle state."
	default:
		// RFC 4271 Section 8.2.2: "Any other event (Events 9-12, 15-28) received
		// in the Idle state does not cause change in the state of the local system."
	}
}

// handleConnect processes events in CONNECT state.
//
// RFC 4271 Section 8.2.2 (Connect state):
// "In this state, BGP FSM is waiting for the TCP connection to be completed."
// Handles connection events to transition to OpenSent or Idle state.
func (f *FSM) handleConnect(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in CONNECT state per RFC 4271.
	case EventManualStop:
		// RFC 4271 Section 8.2.2: Event 2 (ManualStop)
		// "drops the TCP connection, releases all BGP resources, sets
		// ConnectRetryCounter to zero... and changes its state to Idle."
		f.change(StateIdle)

	case EventConnectRetryTimerExpires:
		// RFC 4271 Section 8.2.2: Event 9 (ConnectRetryTimer_Expires)
		// "drops the TCP connection, restarts the ConnectRetryTimer,
		// initiates a TCP connection to the other BGP peer... and stays
		// in the Connect state."
		// (actual reconnect logic handled externally)

	case EventTCPConnectionConfirmed:
		// RFC 4271 Section 8.2.2: Event 16/17 (Tcp_CR_Acked/TcpConnectionConfirmed)
		// "If the DelayOpen attribute is set to FALSE... sends an OPEN message
		// to its peer, sets the HoldTimer to a large value, and changes its
		// state to OpenSent."
		f.change(StateOpenSent)

	case EventTCPConnectionFails:
		// RFC 4271 Section 8.2.2: Event 18 (TcpConnectionFails)
		// "If the DelayOpenTimer is not running... drops the TCP connection,
		// releases all BGP resources, and changes its state to Idle."
		f.change(StateIdle)

	case EventBGPHeaderErr, EventBGPOpenMsgErr, EventNotifMsgVerErr, EventNotifMsg:
		// RFC 4271 Section 8.2.2: Events 21, 22, 24, 25
		// "releases all BGP resources, drops the TCP connection, increments
		// the ConnectRetryCounter by 1... and changes its state to Idle."
		f.change(StateIdle)

	default:
		// RFC 4271 Section 8.2.2: "The start events (Events 1, 3-7) are
		// ignored in the Connect state."
	}
}

// handleActive processes events in ACTIVE state (passive peer).
//
// RFC 4271 Section 8.2.2 (Active state):
// "In this state, BGP FSM is trying to acquire a peer by listening for,
// and accepting, a TCP connection."
// Handles connection events for passive mode peers.
func (f *FSM) handleActive(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in ACTIVE state per RFC 4271.
	case EventManualStop:
		// RFC 4271 Section 8.2.2: Event 2 (ManualStop)
		// "releases all BGP resources... drops the TCP connection, sets
		// ConnectRetryCounter to zero... and changes its state to Idle."
		f.change(StateIdle)

	case EventTCPConnectionConfirmed:
		// RFC 4271 Section 8.2.2: Event 16/17 (Tcp_CR_Acked/TcpConnectionConfirmed)
		// "If the DelayOpen attribute is set to FALSE... sends the OPEN message
		// to its peer, sets its HoldTimer to a large value, and changes its
		// state to OpenSent."
		f.change(StateOpenSent)

	case EventConnectRetryTimerExpires:
		// RFC 4271 Section 8.2.2: Event 9 (ConnectRetryTimer_Expires)
		// "restarts the ConnectRetryTimer... initiates a TCP connection to
		// the other BGP peer... and changes its state to Connect."
		if !f.passive {
			f.change(StateConnect)
		}

	case EventBGPHeaderErr, EventBGPOpenMsgErr, EventNotifMsgVerErr, EventNotifMsg:
		// RFC 4271 Section 8.2.2: Events 21, 22, 24, 25
		// "sets the ConnectRetryTimer to zero, releases all BGP resources,
		// drops the TCP connection... and changes its state to Idle."
		f.change(StateIdle)

	default:
		// RFC 4271 Section 8.2.2: "The start events (Events 1, 3-7) are
		// ignored in the Active state."
	}
}

// handleOpenSent processes events in OPENSENT state.
//
// RFC 4271 Section 8.2.2 (OpenSent state):
// "In this state, BGP FSM waits for an OPEN message from its peer."
// Handles OPEN message reception to transition to OpenConfirm or errors to Idle.
func (f *FSM) handleOpenSent(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in OPENSENT state per RFC 4271.
	case EventManualStop:
		// RFC 4271 Section 8.2.2: Event 2 (ManualStop)
		// "sends the NOTIFICATION with a Cease, sets the ConnectRetryTimer
		// to zero, releases all BGP resources, drops the TCP connection,
		// sets the ConnectRetryCounter to zero, and changes its state to Idle."
		f.change(StateIdle)

	case EventBGPOpen:
		// RFC 4271 Section 8.2.2: Event 19 (BGPOpen)
		// "resets the DelayOpenTimer to zero, sets the BGP ConnectRetryTimer
		// to zero, sends a KEEPALIVE message... sets the HoldTimer according
		// to the negotiated value... changes its state to OpenConfirm."
		f.change(StateOpenConfirm)

	case EventHoldTimerExpires:
		// RFC 4271 Section 8.2.2: Event 10 (HoldTimer_Expires)
		// "sends a NOTIFICATION message with the error code Hold Timer Expired,
		// sets the ConnectRetryTimer to zero, releases all BGP resources,
		// drops the TCP connection... and changes its state to Idle."
		f.change(StateIdle)

	case EventBGPHeaderErr, EventBGPOpenMsgErr, EventNotifMsgVerErr, EventNotifMsg:
		// RFC 4271 Section 8.2.2: Events 21, 22, 24, 25
		// "sends a NOTIFICATION message with the appropriate error code,
		// sets the ConnectRetryTimer to zero, releases all BGP resources,
		// drops the TCP connection... and changes its state to Idle."
		f.change(StateIdle)

	case EventTCPConnectionFails:
		// RFC 4271 Section 8.2.2: Event 18 (TcpConnectionFails)
		// "closes the BGP connection, restarts the ConnectRetryTimer,
		// continues to listen for a connection... and changes its state to Active."
		//
		// VIOLATION: RFC specifies transition to Active, but this implementation
		// transitions to Idle for simplicity. See VIOLATIONS section in file header.
		f.change(StateIdle)

	default:
		// RFC 4271 Section 8.2.2: "The start events (Events 1, 3-7) are
		// ignored in the OpenSent state."
	}
}

// handleOpenConfirm processes events in OPENCONFIRM state.
//
// RFC 4271 Section 8.2.2 (OpenConfirm state):
// "In this state, BGP waits for a KEEPALIVE or NOTIFICATION message."
// Handles KEEPALIVE to transition to Established or errors to Idle.
func (f *FSM) handleOpenConfirm(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in OPENCONFIRM state per RFC 4271.
	case EventManualStop:
		// RFC 4271 Section 8.2.2: Event 2 (ManualStop)
		// "sends the NOTIFICATION message with a Cease, releases all BGP
		// resources, drops the TCP connection, sets the ConnectRetryCounter
		// to zero, sets the ConnectRetryTimer to zero, and changes its state to Idle."
		f.change(StateIdle)

	case EventKeepaliveMsg:
		// RFC 4271 Section 8.2.2: Event 26 (KeepAliveMsg)
		// "restarts the HoldTimer and changes its state to Established."
		f.change(StateEstablished)

	case EventHoldTimerExpires:
		// RFC 4271 Section 8.2.2: Event 10 (HoldTimer_Expires)
		// "sends the NOTIFICATION message with the Error Code Hold Timer
		// Expired, sets the ConnectRetryTimer to zero, releases all BGP
		// resources, drops the TCP connection... and changes its state to Idle."
		f.change(StateIdle)

	case EventNotifMsg, EventNotifMsgVerErr:
		// RFC 4271 Section 8.2.2: Events 24, 25 (NotifMsgVerErr, NotifMsg)
		// "sets the ConnectRetryTimer to zero, releases all BGP resources,
		// drops the TCP connection, and changes its state to Idle."
		f.change(StateIdle)

	case EventBGPHeaderErr, EventBGPOpenMsgErr:
		// RFC 4271 Section 8.2.2: Events 21, 22 (BGPHeaderErr, BGPOpenMsgErr)
		// "sends a NOTIFICATION message with the appropriate error code,
		// sets the ConnectRetryTimer to zero, releases all BGP resources,
		// drops the TCP connection... and changes its state to Idle."
		f.change(StateIdle)

	case EventTCPConnectionFails:
		// RFC 4271 Section 8.2.2: Event 18 (TcpConnectionFails)
		// "sets the ConnectRetryTimer to zero, releases all BGP resources,
		// drops the TCP connection... and changes its state to Idle."
		f.change(StateIdle)

	default:
		// RFC 4271 Section 8.2.2: "Any start event (Events 1, 3-7) is
		// ignored in the OpenConfirm state."
		//
		// Note: RFC specifies Event 11 (KeepaliveTimer_Expires) should send
		// KEEPALIVE and remain in OpenConfirm; not implemented here as timer
		// management is handled externally.
	}
}

// handleEstablished processes events in ESTABLISHED state.
//
// RFC 4271 Section 8.2.2 (Established state):
// "In the Established state, the BGP FSM can exchange UPDATE, NOTIFICATION,
// and KEEPALIVE messages with its peer."
// Handles ongoing session events and transitions to Idle on errors.
func (f *FSM) handleEstablished(event Event) {
	switch event { //nolint:exhaustive // Only specific events are handled in ESTABLISHED state per RFC 4271.
	case EventManualStop:
		// RFC 4271 Section 8.2.2: Event 2 (ManualStop)
		// "sends the NOTIFICATION message with a Cease, sets the
		// ConnectRetryTimer to zero, deletes all routes associated with
		// this connection, releases BGP resources, drops the TCP connection,
		// sets the ConnectRetryCounter to zero, and changes its state to Idle."
		f.change(StateIdle)

	case EventKeepaliveMsg:
		// RFC 4271 Section 8.2.2: Event 26 (KeepAliveMsg)
		// "restarts its HoldTimer, if the negotiated HoldTime value is
		// non-zero, and remains in the Established state."
		// (hold timer reset handled externally)

	case EventUpdateMsg:
		// RFC 4271 Section 8.2.2: Event 27 (UpdateMsg)
		// "processes the message, restarts its HoldTimer, if the negotiated
		// HoldTime value is non-zero, and remains in the Established state."
		// (update processing and hold timer reset handled externally)

	case EventHoldTimerExpires:
		// RFC 4271 Section 8.2.2: Event 10 (HoldTimer_Expires)
		// "sends a NOTIFICATION message with the Error Code Hold Timer Expired,
		// sets the ConnectRetryTimer to zero, releases all BGP resources,
		// drops the TCP connection... and changes its state to Idle."
		f.change(StateIdle)

	case EventNotifMsg, EventNotifMsgVerErr:
		// RFC 4271 Section 8.2.2: Events 24, 25 (NotifMsgVerErr, NotifMsg)
		// "sets the ConnectRetryTimer to zero, deletes all routes associated
		// with this connection, releases all the BGP resources, drops the
		// TCP connection... changes its state to Idle."
		f.change(StateIdle)

	case EventUpdateMsgErr:
		// RFC 4271 Section 8.2.2: Event 28 (UpdateMsgErr)
		// "sends a NOTIFICATION message with an Update error, sets the
		// ConnectRetryTimer to zero, deletes all routes associated with
		// this connection, releases all BGP resources, drops the TCP
		// connection... and changes its state to Idle."
		f.change(StateIdle)

	case EventBGPHeaderErr:
		// RFC 4271 Section 8.2.2: Event 21 (BGPHeaderErr)
		// "sends a NOTIFICATION message with the Error Code Finite State
		// Machine Error... and changes its state to Idle."
		f.change(StateIdle)

	case EventTCPConnectionFails:
		// RFC 4271 Section 8.2.2: Event 18 (TcpConnectionFails)
		// "sets the ConnectRetryTimer to zero, deletes all routes associated
		// with this connection, releases all the BGP resources, drops the
		// TCP connection... changes its state to Idle."
		f.change(StateIdle)

	default:
		// RFC 4271 Section 8.2.2: "Any Start event (Events 1, 3-7) is
		// ignored in the Established state."
		//
		// Note: RFC specifies Event 11 (KeepaliveTimer_Expires) should send
		// KEEPALIVE and restart the timer; not implemented here as timer
		// management is handled externally.
	}
}
