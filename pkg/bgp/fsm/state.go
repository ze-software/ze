// Package fsm implements the BGP Finite State Machine (RFC 4271 Section 8).
//
// RFC 4271 Section 8 defines the BGP FSM with six states and 28 events.
// This implementation follows the mandatory events and state transitions
// specified in Section 8.2.2.
package fsm

import "fmt"

// State represents the BGP FSM state.
// Values are bit flags for efficient comparison and logging.
//
// RFC 4271 Section 8.2.2 defines six states:
//   - Idle: Initial state, refuses all incoming connections
//   - Connect: Waiting for TCP connection to complete
//   - Active: Trying to acquire peer by listening for TCP connection
//   - OpenSent: TCP connection established, OPEN sent, waiting for peer OPEN
//   - OpenConfirm: OPEN received, waiting for KEEPALIVE
//   - Established: Peers can exchange UPDATE, NOTIFICATION, KEEPALIVE
type State int

// FSM states per RFC 4271 Section 8.2.2.
const (
	// StateIdle: RFC 4271 Section 8.2.2 "Idle state"
	// Initially, BGP FSM is in Idle state. In this state, BGP FSM refuses
	// all incoming BGP connections. No resources are allocated to the peer.
	StateIdle State = 0x01

	// StateActive: RFC 4271 Section 8.2.2 "Active State"
	// BGP FSM is trying to acquire a peer by listening for, and accepting,
	// a TCP connection. Entered from Idle via ManualStart_with_PassiveTcpEstablishment.
	StateActive State = 0x02

	// StateConnect: RFC 4271 Section 8.2.2 "Connect State"
	// BGP FSM is waiting for the TCP connection to be completed.
	// Entered from Idle via ManualStart or AutomaticStart events.
	StateConnect State = 0x04

	// StateOpenSent: RFC 4271 Section 8.2.2 "OpenSent"
	// BGP FSM waits for an OPEN message from its peer.
	// Entered after TCP connection succeeds and OPEN message is sent.
	StateOpenSent State = 0x08

	// StateOpenConfirm: RFC 4271 Section 8.2.2 "OpenConfirm State"
	// BGP waits for a KEEPALIVE or NOTIFICATION message.
	// Entered after receiving a valid OPEN and sending KEEPALIVE.
	StateOpenConfirm State = 0x10

	// StateEstablished: RFC 4271 Section 8.2.2 "Established State"
	// BGP FSM can exchange UPDATE, NOTIFICATION, and KEEPALIVE messages.
	// Entered after receiving KEEPALIVE in OpenConfirm state.
	StateEstablished State = 0x20
)

var stateNames = map[State]string{
	StateIdle:        "IDLE",
	StateActive:      "ACTIVE",
	StateConnect:     "CONNECT",
	StateOpenSent:    "OPENSENT",
	StateOpenConfirm: "OPENCONFIRM",
	StateEstablished: "ESTABLISHED",
}

// String returns a human-readable state name.
func (s State) String() string {
	if name, ok := stateNames[s]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", s)
}

// Event represents a BGP FSM event.
//
// RFC 4271 Section 8.1 defines 28 events for the BGP FSM.
// Events 1-2 are mandatory administrative events (Section 8.1.2).
// Events 9-11 are mandatory timer events (Section 8.1.3).
// Events 16-18 are mandatory TCP events (Section 8.1.4).
// Events 19, 21-22, 24-28 are mandatory message events (Section 8.1.5).
//
// This implementation includes mandatory events. Optional events
// (3-8, 12-15, 20, 23) are not implemented.
type Event int

// FSM events per RFC 4271 Section 8.1.
// Event numbers in comments refer to RFC 4271 Section 8.2.1.4.
const (
	// EventManualStart: RFC 4271 Section 8.1.2 Event 1 (Mandatory)
	// Local system administrator manually starts the peer connection.
	EventManualStart Event = iota

	// EventManualStop: RFC 4271 Section 8.1.2 Event 2 (Mandatory)
	// Local system administrator manually stops the peer connection.
	EventManualStop

	// EventConnectRetryTimerExpires: RFC 4271 Section 8.1.3 Event 9 (Mandatory)
	// Generated when the ConnectRetryTimer expires.
	EventConnectRetryTimerExpires

	// EventHoldTimerExpires: RFC 4271 Section 8.1.3 Event 10 (Mandatory)
	// Generated when the HoldTimer expires.
	EventHoldTimerExpires

	// EventKeepaliveTimerExpires: RFC 4271 Section 8.1.3 Event 11 (Mandatory)
	// Generated when the KeepaliveTimer expires.
	EventKeepaliveTimerExpires

	// EventTCPConnectionConfirmed: RFC 4271 Section 8.1.4 Event 17 (Mandatory)
	// Local system received confirmation that TCP connection is established.
	// Note: RFC also defines Event 16 (Tcp_CR_Acked) which is similar.
	EventTCPConnectionConfirmed

	// EventTCPConnectionFails: RFC 4271 Section 8.1.4 Event 18 (Mandatory)
	// Local system received TCP connection failure notice.
	EventTCPConnectionFails

	// EventBGPOpen: RFC 4271 Section 8.1.5 Event 19 (Mandatory)
	// A valid OPEN message has been received.
	EventBGPOpen

	// EventBGPHeaderErr: RFC 4271 Section 8.1.5 Event 21 (Mandatory)
	// A received BGP message header is not valid.
	EventBGPHeaderErr

	// EventBGPOpenMsgErr: RFC 4271 Section 8.1.5 Event 22 (Mandatory)
	// An OPEN message has been received with errors.
	EventBGPOpenMsgErr

	// EventNotifMsgVerErr: RFC 4271 Section 8.1.5 Event 24 (Mandatory)
	// A NOTIFICATION message with "version error" is received.
	EventNotifMsgVerErr

	// EventNotifMsg: RFC 4271 Section 8.1.5 Event 25 (Mandatory)
	// A NOTIFICATION message is received (error code != version error).
	EventNotifMsg

	// EventKeepaliveMsg: RFC 4271 Section 8.1.5 Event 26 (Mandatory)
	// A KEEPALIVE message is received.
	EventKeepaliveMsg

	// EventUpdateMsg: RFC 4271 Section 8.1.5 Event 27 (Mandatory)
	// A valid UPDATE message is received.
	EventUpdateMsg

	// EventUpdateMsgErr: RFC 4271 Section 8.1.5 Event 28 (Mandatory)
	// An invalid UPDATE message is received.
	EventUpdateMsgErr
)

var eventNames = map[Event]string{
	EventManualStart:              "ManualStart",
	EventManualStop:               "ManualStop",
	EventConnectRetryTimerExpires: "ConnectRetryTimerExpires",
	EventHoldTimerExpires:         "HoldTimerExpires",
	EventKeepaliveTimerExpires:    "KeepaliveTimerExpires",
	EventTCPConnectionConfirmed:   "TCPConnectionConfirmed",
	EventTCPConnectionFails:       "TCPConnectionFails",
	EventBGPOpen:                  "BGPOpen",
	EventBGPHeaderErr:             "BGPHeaderErr",
	EventBGPOpenMsgErr:            "BGPOpenMsgErr",
	EventNotifMsgVerErr:           "NotifMsgVerErr",
	EventNotifMsg:                 "NotifMsg",
	EventKeepaliveMsg:             "KeepaliveMsg",
	EventUpdateMsg:                "UpdateMsg",
	EventUpdateMsgErr:             "UpdateMsgErr",
}

// String returns a human-readable event name.
func (e Event) String() string {
	if name, ok := eventNames[e]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", e)
}
