// Design: docs/architecture/behavior/fsm.md — BGP finite state machine
// RFC: rfc/short/rfc4271.md — FSM states and transitions (Section 8)
//
// Package fsm implements the BGP Finite State Machine (RFC 4271 Section 8).
//
// RFC 4271 Section 8 defines the BGP FSM with six states and 28 events.
// This implementation follows the mandatory events and state transitions
// specified in Section 8.2.2.
package fsm

import "github.com/ze-software/ze/internal/core/textbuf"

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
	var b textbuf.Buffer
	return b.Reset().Str("UNKNOWN(").Int(int64(s)).Byte(')').String()
}

// Event represents a BGP FSM event.
//
// RFC 4271 Section 8.1 defines 28 events for the BGP FSM.
// Events 1-2 are mandatory administrative events (Section 8.1.2).
// Events 9-11 are mandatory timer events (Section 8.1.3).
// Events 16-18 are mandatory TCP events (Section 8.1.4).
// Events 19, 21-22, 24-28 are mandatory message events (Section 8.1.5).
//
// This implementation includes every mandatory event, plus three optional
// ones the ConnectRetryCounter needs in order to be right: Event 6
// (AutomaticStart_with_DampPeerOscillations) for a damped retry, Event 8
// (AutomaticStop) for a teardown the system chose, and Event 23
// (OpenCollisionDump) for a connection lost to collision resolution. The
// remaining optional events (3-5, 7, 12-15, 20) are not implemented.
//
// Six further events come from draft-ietf-idr-bgp-bfd-strict-mode Section 4,
// registered as FSM events 30 to 35 by its Section 13.3. They are declared at
// the end of the const block below.
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
	// NOTE: Never generated in production — see ARCHITECTURAL NOTES in fsm.go.
	// Peer-level exponential backoff (peer.go run loop) replaces this timer.
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

	// EventAutomaticStartWithDampPeerOscillations: RFC 4271 Section 8.1.2
	// Event 6 (Optional) — AutomaticStart_with_DampPeerOscillations.
	//
	// The system, not the operator, restarts a connection attempt after a
	// damping delay. Ze's damping is the exponential backoff in the peer
	// reconnect loop (internal/component/bgp/reactor/peer_run.go, run), which
	// stands in for the RFC's ConnectRetryTimer.
	//
	// It exists so the retry cycle is distinguishable from Event 1. RFC 4271
	// Section 8.2.2 makes Event 1 in Idle "set ConnectRetryCounter to zero",
	// and ze fires a start event once per connection cycle because each cycle
	// builds a new FSM. Firing Event 1 there would zero the counter on every
	// retry and leave it structurally incapable of counting one. The RFC gives
	// Events 6 and 7 no action list at all ("The method of preventing
	// persistent peer oscillation is outside the scope of this document"), and
	// in particular no ConnectRetryCounter clause, which is exactly the
	// semantics a damped retry needs.
	EventAutomaticStartWithDampPeerOscillations

	// EventAutomaticStop: RFC 4271 Section 8.1.2 Event 8 (Optional)
	// The local system, not the operator, stops the BGP connection. The RFC's
	// own example is a prefix maximum being exceeded; ze also raises it for a
	// BFD session going down (reactor/peer_bfd.go) and for a forward-pool
	// out-of-resources teardown (reactor/forward_pool_congestion.go).
	//
	// It is distinct from EventManualStop for one reason that shows: RFC 4271
	// Section 8.2.2 makes Event 2 set the ConnectRetryCounter to zero and
	// Event 8 increment it. A daemon that stopped a peer because the peer was
	// unreachable has just recorded a failed attempt, not been told by an
	// operator to forget the ones before it.
	EventAutomaticStop

	// EventOpenCollisionDump: RFC 4271 Section 8.1.5 Event 23 (Optional)
	// A connection collision (Section 6.8) was resolved against this
	// connection and it must be closed. Ze raises it from
	// Session.CloseWithNotification, whose only caller is the collision
	// resolution in reactor/peer_connection.go.
	//
	// Like Event 8 it exists so the ConnectRetryCounter is right: RFC 4271
	// Section 8.2.2 lists "increments the ConnectRetryCounter by 1" in the
	// Event 23 action list of OpenSent, OpenConfirm and Established, where
	// Event 2 would have zeroed it.
	EventOpenCollisionDump

	// The six BFD strict-mode events of
	// draft-ietf-idr-bgp-bfd-strict-mode Section 4, registered by its
	// Section 13.3 as BGP-4 FSM events 30 to 35. Each is Optional in the
	// draft's own words and each is implemented here, because Ze runs the
	// strict-mode procedures of Section 8. The reactor raises the first
	// three from the BFD subscription channel (reactor/peer_bfd.go), the
	// fifth from the BfdHoldTimer, and the sixth from a config reload.

	// EventBfdAdminDown is draft-ietf-idr-bgp-bfd-strict-mode Section 4
	// Event 30, "The BFD session associated with this BGP session has
	// transitioned to the AdminDown state".
	//
	// It is grouped with EventBfdUp in every state handler, because the draft
	// groups them: an administratively disabled BFD session says nothing about
	// the forwarding path (RFC 5882 Section 4.2), so a strict session must not
	// be held out of Established waiting for it.
	EventBfdAdminDown

	// EventBfdDown is draft-ietf-idr-bgp-bfd-strict-mode Section 4 Event 31,
	// "The BFD session associated with this BGP session has transitioned to
	// the Down state".
	EventBfdDown

	// EventBfdUp is draft-ietf-idr-bgp-bfd-strict-mode Section 4 Event 32,
	// "The BFD session associated with this BGP session has transitioned to
	// the Up state".
	EventBfdUp

	// EventBfdDisabled is draft-ietf-idr-bgp-bfd-strict-mode Section 4
	// Event 33, "The BfdEnabled session attribute has been changed to FALSE".
	//
	// It has NO producer, by the draft's own instruction. Section 4's Event 35
	// note routes the disable case elsewhere: "When BFD has been disabled, the
	// local system will trigger a BfdAdminDown event instead", and
	// Session.raiseBFDStrictConfigChanged does exactly that. The event is
	// declared and handled because the draft defines it and a peer
	// implementation may raise its own; every state handler answers it beside
	// Event 30, which is the answer Sections 8.3.1, 8.4.1, 8.5.1, 8.6.1 and
	// 8.7.1 each give the two together.
	EventBfdDisabled

	// EventBfdHoldTimerExpires is draft-ietf-idr-bgp-bfd-strict-mode Section 4
	// Event 34, "The BFD holdtimer, which is set when the negotiated BGP hold
	// time is zero, has expired".
	EventBfdHoldTimerExpires

	// EventBfdStrictConfigChanged is draft-ietf-idr-bgp-bfd-strict-mode
	// Section 4 Event 35, "The configuration for the BFD strict configuration
	// for the BGP session has been changed".
	//
	// draft-ietf-idr-bgp-bfd-strict-mode Section 4 MUST NOT: "If BfdEnabled is
	// FALSE, this event MUST NOT occur. When BFD has been disabled, the local
	// system will trigger a BfdAdminDown event instead". The one producer,
	// Peer.raiseBFDStrictConfigChanged (reactor/peer_bfd.go), enforces it.
	EventBfdStrictConfigChanged
)

// BfdSubState is the strict-mode sub-state of
// draft-ietf-idr-bgp-bfd-strict-mode Section 8.1. It tracks a pending BFD Up
// event while the BGP FSM waits in one of RFC 4271's own states.
//
// SubStateNone is not "unset": it is the sub-state of every session that is
// not waiting for BFD, which is every session where BFD strict-mode was not
// negotiated. A session that IS waiting always carries one of the other two.
type BfdSubState int

// The strict-mode sub-states per draft-ietf-idr-bgp-bfd-strict-mode Section 8.1.
//
// The draft names four. Two of them, ConnectDelayOpenBfdUpPending and
// ActiveDelayOpenBfdUpPending, are entered only by Event 20 (an OPEN received
// while the DelayOpenTimer runs, Sections 8.3.5 and 8.4.5). Ze implements no
// DelayOpenTimer, which RFC 4271 Section 8.2.1.3 permits and the fsm.go header
// records, so neither is reachable and neither is declared here. The Connect
// and Active handlers still answer every BFD event, and their answer is the
// draft's "not in the sub-state" branch.
const (
	// SubStateNone says the FSM is not waiting for a BFD Up event.
	SubStateNone BfdSubState = iota

	// SubStateOpenSentBfdUpPending is draft-ietf-idr-bgp-bfd-strict-mode
	// Section 8.1 "OpenSentBfdUpPending". The peer's OPEN arrived, the local
	// system withheld its KEEPALIVE, and the next BFD Up event advances the
	// session to OpenConfirm.
	SubStateOpenSentBfdUpPending

	// SubStateOpenSentConfirmedBfdUpPending is
	// draft-ietf-idr-bgp-bfd-strict-mode Section 8.1
	// "OpenSentConfirmedBfdUpPending". The peer's KEEPALIVE arrived
	// while the local system was still waiting for BFD, so the next BFD Up
	// event advances the session straight to Established (Section 8.5.6).
	SubStateOpenSentConfirmedBfdUpPending
)

var bfdSubStateNames = map[BfdSubState]string{
	SubStateNone:                          "NONE",
	SubStateOpenSentBfdUpPending:          "OPENSENT-BFD-UP-PENDING",
	SubStateOpenSentConfirmedBfdUpPending: "OPENSENT-CONFIRMED-BFD-UP-PENDING",
}

// String returns a human-readable sub-state name.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 11: "Implementations SHOULD
// provide visibility for these sub-states in its display of the BGP finite
// state machine". This is the name that reaches an operator, through
// Peer.bfdSubState (reactor/peer_bfd.go) and the `bfd-sub-state` field of
// `show bgp peer list` and `show bgp peer detail` (plugins/cmd/peer/peer.go).
func (s BfdSubState) String() string {
	if name, ok := bfdSubStateNames[s]; ok {
		return name
	}
	var b textbuf.Buffer
	return b.Reset().Str("UNKNOWN(").Int(int64(s)).Byte(')').String()
}

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

	EventAutomaticStartWithDampPeerOscillations: "AutomaticStartWithDampPeerOscillations",
	EventAutomaticStop:                          "AutomaticStop",
	EventOpenCollisionDump:                      "OpenCollisionDump",

	EventBfdAdminDown:           "BfdAdminDown",
	EventBfdDown:                "BfdDown",
	EventBfdUp:                  "BfdUp",
	EventBfdDisabled:            "BfdDisabled",
	EventBfdHoldTimerExpires:    "BfdHoldTimerExpires",
	EventBfdStrictConfigChanged: "BfdStrictConfigChanged",
}

// String returns a human-readable event name.
func (e Event) String() string {
	if name, ok := eventNames[e]; ok {
		return name
	}
	var b2 textbuf.Buffer
	return b2.Reset().Str("UNKNOWN(").Int(int64(e)).Byte(')').String()
}
