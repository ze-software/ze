// Design: docs/architecture/behavior/fsm.md — received-message FSM error diagnostics
// Related: session_handlers.go — message admission and FSM event delivery
// RFC: rfc/short/rfc6608.md — state-specific subcodes and one-octet message Data

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// notifyFSMMessageError writes the diagnostic for an unexpected received
// message. The caller MUST retain the receiving state before delivering the FSM
// event: that event may already have moved the session to Idle.
//
// RFC 6608 Section 4: "The Data field is a 1-octet, unsigned integer that
// indicates the type of the unexpected message."
func (s *Session) notifyFSMMessageError(state fsm.State, received msgtype.MessageType) {
	subcode := fsmMessageErrorSubcode(state)
	data := []byte{byte(received)}
	if subcode == message.NotifyFSMUnspecified {
		// RFC 6608 assigns specific subcodes only to the three named states.
		// BFD strict pending errors in Connect/Active retain Unspecified.
		data = nil
	}
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()
	s.logNotifyErr(conn, message.NotifyFSMError, subcode, data)
}

// fsmMessageErrorSubcode selects the states RFC 6608 Section 4 names; its
// Unspecified value remains the defined answer for an error in another state.
func fsmMessageErrorSubcode(state fsm.State) uint8 {
	// RFC 6608 defines diagnostics for this subset of the FSM states.
	//exhaustive:ignore
	switch state {
	case fsm.StateOpenSent:
		// RFC 6608 Section 4: "If a BGP speaker receives an unexpected message
		// (e.g., KEEPALIVE/UPDATE/ROUTE-REFRESH message) on a session in OpenSent
		// state, it MUST send to the neighbor a NOTIFICATION message with the
		// Error Code Finite State Machine Error and the Error Subcode
		// "Receive Unexpected Message in OpenSent State"."
		return message.NotifyFSMUnexpectedOpenSent
	case fsm.StateOpenConfirm:
		// RFC 6608 Section 4: "If a BGP speaker receives an unexpected message
		// (e.g., OPEN/UPDATE/ROUTE-REFRESH message) on a session in OpenConfirm
		// state, it MUST send a NOTIFICATION message with the Error Code Finite
		// State Machine Error and the Error Subcode "Receive Unexpected Message
		// in OpenConfirm State" to the neighbor."
		return message.NotifyFSMUnexpectedOpenConfirm
	case fsm.StateEstablished:
		// RFC 6608 Section 4: "If a BGP speaker receives an unexpected message
		// (e.g., OPEN message) on a session in Established State, it MUST send
		// to the neighbor a NOTIFICATION message with the Error Code Finite
		// State Machine Error and the Error Subcode "Receive Unexpected Message
		// in Established State"."
		return message.NotifyFSMUnexpectedEstablished
	}
	return message.NotifyFSMUnspecified
}
