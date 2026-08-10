// Design: docs/research/l2tpv2-implementation-guide.md -- PPP FSM (RFC 1661 §4)
// Related: lcp.go -- LCP packet codec driven by this FSM
// Related: lcp_options.go -- LCP option negotiation invoked from this FSM

package ppp

// The state machine in this file is the "LCP FSM" defined in RFC 1661
// §4.1-4.4, but per RFC 1661 §2 the same FSM is reused by every
// Network Control Protocol (IPCP, IPv6CP, CCP, ...). Only the option
// codec and the code/identifier space differ between protocols; the
// states, events, actions and transition table are identical. NCP code
// uses these types directly -- no wrapping, no adapter.

// LCP FSM state. RFC 1661 §4.2 defines the ten states. State numbers
// match the order in the RFC's transition table for direct cross-
// reference.
type LCPState uint8

// LCP FSM states.
const (
	LCPStateInitial  LCPState = 0 // lower layer down, no Open
	LCPStateStarting LCPState = 1 // lower layer down, Open issued
	LCPStateClosed   LCPState = 2 // lower layer up, no Open
	LCPStateStopped  LCPState = 3 // lower layer up, Open finished, awaiting peer
	LCPStateClosing  LCPState = 4 // sending Terminate-Request, no Open
	LCPStateStopping LCPState = 5 // sending Terminate-Request, Open is current
	LCPStateReqSent  LCPState = 6 // Configure-Request sent, no Configure-Ack received
	LCPStateAckRcvd  LCPState = 7 // Configure-Request sent, Configure-Ack received
	LCPStateAckSent  LCPState = 8 // Configure-Ack sent, no Configure-Ack received
	LCPStateOpened   LCPState = 9 // Configure-Ack sent and received
)

// String returns the canonical state name from RFC 1661 §4.2.
func (s LCPState) String() string {
	switch s {
	case LCPStateInitial:
		return "initial"
	case LCPStateStarting:
		return "starting"
	case LCPStateClosed:
		return "closed"
	case LCPStateStopped:
		return "stopped"
	case LCPStateClosing:
		return "closing"
	case LCPStateStopping:
		return "stopping"
	case LCPStateReqSent:
		return "req-sent"
	case LCPStateAckRcvd:
		return "ack-rcvd"
	case LCPStateAckSent:
		return "ack-sent"
	case LCPStateOpened:
		return "opened"
	}
	return "unknown"
}

// LCP FSM event. RFC 1661 §4.1 defines the 16 events. Names match the
// RFC abbreviations; positive/negative variants encode the
// "acceptable" branch on receive events.
type lCPEvent uint8

// LCP FSM events.
const (
	LCPEventUp       lCPEvent = iota // lower layer is Up
	LCPEventDown                     // lower layer is Down
	LCPEventOpen                     // administrative Open
	LCPEventClose                    // administrative Close
	LCPEventTOPlus                   // timeout, restart counter > 0
	LCPEventTOMinus                  // timeout, restart counter expired
	LCPEventRCRPlus                  // receive Configure-Request good (acks)
	LCPEventRCRMinus                 // receive Configure-Request bad (naks/rejects)
	LCPEventRCA                      // receive Configure-Ack
	LCPEventRCN                      // receive Configure-Nak or Configure-Reject
	LCPEventRTR                      // receive Terminate-Request
	LCPEventRTA                      // receive Terminate-Ack
	LCPEventRUC                      // receive Unknown Code
	LCPEventRXJPlus                  // receive permitted Code-Reject / Protocol-Reject
	LCPEventRXJMinus                 // receive impermissible Code-Reject / Protocol-Reject
	LCPEventRXR                      // receive Echo-Request, Echo-Reply, or Discard-Request
)

// LCP FSM action. RFC 1661 §4.4 defines the actions.
type LCPAction uint8

// LCP FSM actions.
const (
	LCPActTLU LCPAction = iota // This-Layer-Up: notify upper layers
	LCPActTLD                  // This-Layer-Down: notify upper layers
	LCPActTLS                  // This-Layer-Started: lower layer should bring itself up
	LCPActTLF                  // This-Layer-Finished: lower layer is finished
	LCPActIRC                  // Initialize-Restart-Count
	LCPActZRC                  // Zero-Restart-Count and arm timer
	LCPActSCR                  // Send-Configure-Request
	LCPActSCA                  // Send-Configure-Ack (with current packet's options)
	LCPActSCN                  // Send-Configure-Nak or Configure-Reject (with current packet's options)
	LCPActSTR                  // Send-Terminate-Request
	LCPActSTA                  // Send-Terminate-Ack
	LCPActSCJ                  // Send-Code-Reject
	LCPActSER                  // Send-Echo-Reply
)

// String returns a short tag for an action, used in test names and logs.
func (a LCPAction) String() string {
	switch a {
	case LCPActTLU:
		return "tlu"
	case LCPActTLD:
		return "tld"
	case LCPActTLS:
		return "tls"
	case LCPActTLF:
		return "tlf"
	case LCPActIRC:
		return "irc"
	case LCPActZRC:
		return "zrc"
	case LCPActSCR:
		return "scr"
	case LCPActSCA:
		return "sca"
	case LCPActSCN:
		return "scn"
	case LCPActSTR:
		return "str"
	case LCPActSTA:
		return "sta"
	case LCPActSCJ:
		return "scj"
	case LCPActSER:
		return "ser"
	}
	return "?"
}

// lCPTransition encodes the destination state and ordered actions
// returned by the transition function for a given (state, event)
// pair. A zero-length Actions slice with NewState == prior state
// means the event was ignored at this state.
type lCPTransition struct {
	NewState LCPState
	Actions  []LCPAction
}

// LCPDoTransition computes the FSM transition for the given current
// state and incoming event. Returns the new state and the ordered
// list of actions the caller should perform.
//
// The mapping is the verbatim RFC 1661 §4.1 transition table. Where
// the table prescribes an event/state combination as illegal or
// undefined, ze treats the event as a no-op (no actions, state
// unchanged).
//
// Phase 10 caller responsibility: when this function returns a no-op
// transition (NewState == input state AND len(Actions) == 0) for an
// event that was actually delivered (not a synthetic poll), log at
// debug level with state, event, peer identity, and a hex sample of
// the offending packet. Such no-ops indicate either a buggy peer
// (unexpected packet at this stage) or a hostile one (probing). The
// FSM itself cannot log because it has no logger handle and must
// stay pure for table-driven testing.
//
// Pure function -- no I/O, no allocation beyond the returned slice.
// Suitable for table-driven testing.
func LCPDoTransition(state LCPState, ev lCPEvent) lCPTransition {
	switch state {
	case LCPStateInitial:
		switch ev {
		case LCPEventUp:
			return lCPTransition{NewState: LCPStateClosed}
		case LCPEventOpen:
			return lCPTransition{NewState: LCPStateStarting, Actions: []LCPAction{LCPActTLS}}
		case LCPEventClose:
			return lCPTransition{NewState: LCPStateInitial}
		default:
			return lCPTransition{NewState: state}
		}

	case LCPStateStarting:
		switch ev {
		case LCPEventUp:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActIRC, LCPActSCR}}
		case LCPEventClose:
			return lCPTransition{NewState: LCPStateInitial, Actions: []LCPAction{LCPActTLF}}
		case LCPEventOpen:
			return lCPTransition{NewState: LCPStateStarting}
		default:
			return lCPTransition{NewState: state}
		}

	case LCPStateClosed:
		switch ev {
		case LCPEventDown:
			return lCPTransition{NewState: LCPStateInitial}
		case LCPEventOpen:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActIRC, LCPActSCR}}
		case LCPEventClose:
			return lCPTransition{NewState: LCPStateClosed}
		case LCPEventRCRPlus, LCPEventRCRMinus, LCPEventRCA, LCPEventRCN, LCPEventRTR:
			return lCPTransition{NewState: LCPStateClosed, Actions: []LCPAction{LCPActSTA}}
		case LCPEventRUC:
			return lCPTransition{NewState: LCPStateClosed, Actions: []LCPAction{LCPActSCJ}}
		case LCPEventRXJPlus, LCPEventRXJMinus, LCPEventRXR, LCPEventRTA:
			return lCPTransition{NewState: LCPStateClosed}
		default:
			return lCPTransition{NewState: state}
		}

	case LCPStateStopped:
		switch ev {
		case LCPEventDown:
			return lCPTransition{NewState: LCPStateStarting, Actions: []LCPAction{LCPActTLS}}
		case LCPEventOpen:
			return lCPTransition{NewState: LCPStateStopped}
		case LCPEventClose:
			return lCPTransition{NewState: LCPStateClosed}
		case LCPEventRCRPlus:
			return lCPTransition{NewState: LCPStateAckSent, Actions: []LCPAction{LCPActIRC, LCPActSCR, LCPActSCA}}
		case LCPEventRCRMinus:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActIRC, LCPActSCR, LCPActSCN}}
		case LCPEventRCA, LCPEventRCN, LCPEventRTR:
			return lCPTransition{NewState: LCPStateStopped, Actions: []LCPAction{LCPActSTA}}
		case LCPEventRUC:
			return lCPTransition{NewState: LCPStateStopped, Actions: []LCPAction{LCPActSCJ}}
		case LCPEventRXJPlus, LCPEventRXR, LCPEventRTA:
			return lCPTransition{NewState: LCPStateStopped}
		case LCPEventRXJMinus:
			return lCPTransition{NewState: LCPStateStopped, Actions: []LCPAction{LCPActTLF}}
		default:
			return lCPTransition{NewState: state}
		}

	case LCPStateClosing:
		switch ev {
		case LCPEventDown:
			return lCPTransition{NewState: LCPStateInitial}
		case LCPEventOpen:
			return lCPTransition{NewState: LCPStateStopping}
		case LCPEventClose:
			return lCPTransition{NewState: LCPStateClosing}
		case LCPEventTOPlus:
			return lCPTransition{NewState: LCPStateClosing, Actions: []LCPAction{LCPActSTR}}
		case LCPEventTOMinus:
			return lCPTransition{NewState: LCPStateClosed, Actions: []LCPAction{LCPActTLF}}
		case LCPEventRTR:
			return lCPTransition{NewState: LCPStateClosing, Actions: []LCPAction{LCPActSTA}}
		case LCPEventRTA:
			return lCPTransition{NewState: LCPStateClosed, Actions: []LCPAction{LCPActTLF}}
		case LCPEventRUC:
			return lCPTransition{NewState: LCPStateClosing, Actions: []LCPAction{LCPActSCJ}}
		case LCPEventRCRPlus, LCPEventRCRMinus, LCPEventRCA, LCPEventRCN, LCPEventRXJPlus, LCPEventRXJMinus, LCPEventRXR:
			return lCPTransition{NewState: LCPStateClosing}
		default:
			return lCPTransition{NewState: state}
		}

	case LCPStateStopping:
		switch ev {
		case LCPEventDown:
			return lCPTransition{NewState: LCPStateStarting}
		case LCPEventOpen:
			return lCPTransition{NewState: LCPStateStopping}
		case LCPEventClose:
			return lCPTransition{NewState: LCPStateClosing}
		case LCPEventTOPlus:
			return lCPTransition{NewState: LCPStateStopping, Actions: []LCPAction{LCPActSTR}}
		case LCPEventTOMinus:
			return lCPTransition{NewState: LCPStateStopped, Actions: []LCPAction{LCPActTLF}}
		case LCPEventRTR:
			return lCPTransition{NewState: LCPStateStopping, Actions: []LCPAction{LCPActSTA}}
		case LCPEventRTA:
			return lCPTransition{NewState: LCPStateStopped, Actions: []LCPAction{LCPActTLF}}
		case LCPEventRUC:
			return lCPTransition{NewState: LCPStateStopping, Actions: []LCPAction{LCPActSCJ}}
		case LCPEventRCRPlus, LCPEventRCRMinus, LCPEventRCA, LCPEventRCN, LCPEventRXJPlus, LCPEventRXJMinus, LCPEventRXR:
			return lCPTransition{NewState: LCPStateStopping}
		default:
			return lCPTransition{NewState: state}
		}

	case LCPStateReqSent:
		switch ev {
		case LCPEventDown:
			return lCPTransition{NewState: LCPStateStarting}
		case LCPEventOpen:
			return lCPTransition{NewState: LCPStateReqSent}
		case LCPEventClose:
			return lCPTransition{NewState: LCPStateClosing, Actions: []LCPAction{LCPActIRC, LCPActSTR}}
		case LCPEventTOPlus:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActSCR}}
		case LCPEventTOMinus:
			return lCPTransition{NewState: LCPStateStopped, Actions: []LCPAction{LCPActTLF}}
		case LCPEventRCRPlus:
			return lCPTransition{NewState: LCPStateAckSent, Actions: []LCPAction{LCPActSCA}}
		case LCPEventRCRMinus:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActSCN}}
		case LCPEventRCA:
			return lCPTransition{NewState: LCPStateAckRcvd, Actions: []LCPAction{LCPActIRC}}
		case LCPEventRCN:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActIRC, LCPActSCR}}
		case LCPEventRTR:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActSTA}}
		case LCPEventRTA, LCPEventRXJPlus, LCPEventRXR:
			return lCPTransition{NewState: LCPStateReqSent}
		case LCPEventRUC:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActSCJ}}
		case LCPEventRXJMinus:
			return lCPTransition{NewState: LCPStateStopped, Actions: []LCPAction{LCPActTLF}}
		default:
			return lCPTransition{NewState: state}
		}

	case LCPStateAckRcvd:
		switch ev {
		case LCPEventDown:
			return lCPTransition{NewState: LCPStateStarting}
		case LCPEventOpen:
			return lCPTransition{NewState: LCPStateAckRcvd}
		case LCPEventClose:
			return lCPTransition{NewState: LCPStateClosing, Actions: []LCPAction{LCPActIRC, LCPActSTR}}
		case LCPEventTOPlus:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActSCR}}
		case LCPEventTOMinus:
			return lCPTransition{NewState: LCPStateStopped, Actions: []LCPAction{LCPActTLF}}
		case LCPEventRCRPlus:
			return lCPTransition{NewState: LCPStateOpened, Actions: []LCPAction{LCPActSCA, LCPActTLU}}
		case LCPEventRCRMinus:
			return lCPTransition{NewState: LCPStateAckRcvd, Actions: []LCPAction{LCPActSCN}}
		case LCPEventRCA, LCPEventRCN:
			// RFC 1661 §4.1: in Ack-Rcvd, RCA/RCN trigger SCR (cross
			// or stale Ack) -> stay in Req-Sent.
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActSCR}}
		case LCPEventRTR:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActSTA}}
		case LCPEventRTA, LCPEventRXJPlus, LCPEventRXR:
			return lCPTransition{NewState: LCPStateAckRcvd}
		case LCPEventRUC:
			return lCPTransition{NewState: LCPStateAckRcvd, Actions: []LCPAction{LCPActSCJ}}
		case LCPEventRXJMinus:
			return lCPTransition{NewState: LCPStateStopped, Actions: []LCPAction{LCPActTLF}}
		default:
			return lCPTransition{NewState: state}
		}

	case LCPStateAckSent:
		switch ev {
		case LCPEventDown:
			return lCPTransition{NewState: LCPStateStarting}
		case LCPEventOpen:
			return lCPTransition{NewState: LCPStateAckSent}
		case LCPEventClose:
			return lCPTransition{NewState: LCPStateClosing, Actions: []LCPAction{LCPActIRC, LCPActSTR}}
		case LCPEventTOPlus:
			return lCPTransition{NewState: LCPStateAckSent, Actions: []LCPAction{LCPActSCR}}
		case LCPEventTOMinus:
			return lCPTransition{NewState: LCPStateStopped, Actions: []LCPAction{LCPActTLF}}
		case LCPEventRCRPlus:
			return lCPTransition{NewState: LCPStateAckSent, Actions: []LCPAction{LCPActSCA}}
		case LCPEventRCRMinus:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActSCN}}
		case LCPEventRCA:
			return lCPTransition{NewState: LCPStateOpened, Actions: []LCPAction{LCPActIRC, LCPActTLU}}
		case LCPEventRCN:
			return lCPTransition{NewState: LCPStateAckSent, Actions: []LCPAction{LCPActIRC, LCPActSCR}}
		case LCPEventRTR:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActSTA}}
		case LCPEventRTA, LCPEventRXJPlus, LCPEventRXR:
			return lCPTransition{NewState: LCPStateAckSent}
		case LCPEventRUC:
			return lCPTransition{NewState: LCPStateAckSent, Actions: []LCPAction{LCPActSCJ}}
		case LCPEventRXJMinus:
			return lCPTransition{NewState: LCPStateStopped, Actions: []LCPAction{LCPActTLF}}
		default:
			return lCPTransition{NewState: state}
		}

	case LCPStateOpened:
		switch ev {
		case LCPEventDown:
			return lCPTransition{NewState: LCPStateStarting, Actions: []LCPAction{LCPActTLD}}
		case LCPEventOpen:
			return lCPTransition{NewState: LCPStateOpened}
		case LCPEventClose:
			return lCPTransition{NewState: LCPStateClosing, Actions: []LCPAction{LCPActTLD, LCPActIRC, LCPActSTR}}
		case LCPEventRCRPlus:
			return lCPTransition{NewState: LCPStateAckSent, Actions: []LCPAction{LCPActTLD, LCPActSCR, LCPActSCA}}
		case LCPEventRCRMinus:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActTLD, LCPActSCR, LCPActSCN}}
		case LCPEventRCA, LCPEventRCN:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActTLD, LCPActSCR}}
		case LCPEventRTR:
			return lCPTransition{NewState: LCPStateStopping, Actions: []LCPAction{LCPActTLD, LCPActZRC, LCPActSTA}}
		case LCPEventRTA:
			return lCPTransition{NewState: LCPStateReqSent, Actions: []LCPAction{LCPActTLD, LCPActSCR}}
		case LCPEventRUC:
			return lCPTransition{NewState: LCPStateOpened, Actions: []LCPAction{LCPActSCJ}}
		case LCPEventRXJPlus, LCPEventRXR:
			return lCPTransition{NewState: LCPStateOpened, Actions: []LCPAction{LCPActSER}}
		case LCPEventRXJMinus:
			return lCPTransition{NewState: LCPStateStopping, Actions: []LCPAction{LCPActTLD, LCPActIRC, LCPActSTR}}
		default:
			return lCPTransition{NewState: state}
		}
	}
	// No-op for events not listed at the current state.
	return lCPTransition{NewState: state}
}
