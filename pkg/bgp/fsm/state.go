// Package fsm implements the BGP Finite State Machine (RFC 4271 Section 8).
package fsm

import "fmt"

// State represents the BGP FSM state.
// Values are bit flags for efficient comparison and logging.
type State int

// FSM states per RFC 4271 Section 8.
const (
	StateIdle        State = 0x01 // Initial state, no connection
	StateActive      State = 0x02 // Listening for incoming connection
	StateConnect     State = 0x04 // Attempting outgoing connection
	StateOpenSent    State = 0x08 // OPEN sent, waiting for peer OPEN
	StateOpenConfirm State = 0x10 // OPEN received, waiting for KEEPALIVE
	StateEstablished State = 0x20 // Session established, exchanging routes
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
type Event int

// FSM events per RFC 4271 Section 8.
const (
	EventManualStart Event = iota
	EventManualStop
	EventConnectRetryTimerExpires
	EventHoldTimerExpires
	EventKeepaliveTimerExpires
	EventTCPConnectionConfirmed
	EventTCPConnectionFails
	EventBGPOpen
	EventBGPHeaderErr
	EventBGPOpenMsgErr
	EventNotifMsgVerErr
	EventNotifMsg
	EventKeepaliveMsg
	EventUpdateMsg
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
