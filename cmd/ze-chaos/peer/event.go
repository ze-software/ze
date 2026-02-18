// Package peer implements BGP session handling for the chaos testing tool.
package peer

import (
	"net/netip"
	"time"
)

// EventType identifies the kind of simulator event.
type EventType int

const (
	// EventEstablished is sent when the BGP session reaches Established state.
	EventEstablished EventType = iota
	// EventRouteSent is sent after each route announcement.
	EventRouteSent
	// EventRouteReceived is sent when a forwarded route is received from the RR.
	EventRouteReceived
	// EventRouteWithdrawn is sent when a withdrawal is received from the RR.
	EventRouteWithdrawn
	// EventEORSent is sent after the End-of-RIB marker is sent.
	EventEORSent
	// EventDisconnected is sent when the peer's TCP connection closes.
	EventDisconnected
	// EventError is sent when the peer encounters a fatal error.
	EventError
	// EventChaosExecuted is sent when a chaos action is executed on a peer.
	EventChaosExecuted
	// EventReconnecting is sent when a peer is about to reconnect after chaos.
	EventReconnecting
	// EventWithdrawalSent is sent when explicit withdrawals are sent by chaos.
	EventWithdrawalSent
)

// Event represents a simulator lifecycle or route event.
type Event struct {
	// Type identifies the event kind.
	Type EventType

	// PeerIndex identifies which peer generated the event.
	PeerIndex int

	// Time is when the event occurred.
	Time time.Time

	// Prefix is set for route events (EventRouteSent, EventRouteReceived, EventRouteWithdrawn).
	Prefix netip.Prefix

	// Err is set for EventError.
	Err error

	// Count is set for EventEORSent and EventWithdrawalSent.
	Count int

	// ChaosAction is the action name for EventChaosExecuted events.
	ChaosAction string

	// Families is set for EventEORSent with the list of families sent.
	Families []string
}
