// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE wire codec
// RFC: rfc/short/rfc2205.md
// Related: wire.go -- DecodeMessage calls checkMandatoryObjects last, after every object is read
// Related: build.go -- the encoders that emit these objects in the BNF order
// Related: engine.go -- handlePacket drops the message this refuses, with a log line
//
// RFC 2205 Section 3.1 defines each message type with a BNF whose square
// brackets mark the optional objects. Everything unbracketed is mandatory, and
// a message that omits one is malformed. This file holds that list, one entry
// per message type ze processes.
package rsvpte

import "fmt"

// checkMandatoryObjects reports a message that is missing an object the RFC 2205
// Section 3.1 BNF writes UNBRACKETED for its message type.
//
// RFC 2205 Appendix B: "Similarly, each node is required to verify the correct
// construction of each RSVP message it receives." The same paragraph settles
// what the node then does, and it is not an error message: "Should a programming
// error allow an RSVP to create a malformed message, the error is not generally
// reported to end systems in an ERROR_SPEC object; instead, the error is simply
// logged locally, and perhaps reported through network management mechanisms."
// So this returns an error, the caller drops the message and logs it, and no
// PathErr or ResvErr goes back. Appendix B defines no error code for an absent
// mandatory object either, which is the same answer read from the other side.
//
// TIME_VALUES is the object that makes the check load-bearing. RFC 2205 Section
// 3.7 makes it the only object that tells a receiver the period the sender
// refreshes at, so a PATH accepted without one falls back to the 30-second
// default (receivedRefreshPeriod in engine.go) and a sender refreshing every 300
// seconds has its state expired here between two of its own refreshes.
//
// The sender descriptor is bracketed in every message below, so SENDER_TEMPLATE
// is never required here. The handlers that cannot work without it check it
// themselves (handlePath, handlePathTear in engine.go).
func checkMandatoryObjects(msg *ParsedMessage) error {
	var required []uint8
	switch msg.Header.MsgType {
	// RFC 2205 Section 3.1.3: "<Path Message> ::= <Common Header> [ <INTEGRITY> ]
	// <SESSION> <RSVP_HOP> <TIME_VALUES> [ <POLICY_DATA> ... ]
	// [ <sender descriptor> ]".
	case MsgTypePath:
		required = []uint8{ClassSession, ClassRSVPHop, ClassTimeValues}
	// RFC 2205 Section 3.1.4: "<Resv Message> ::= <Common Header> [ <INTEGRITY> ]
	// <SESSION> <RSVP_HOP> <TIME_VALUES> [ <RESV_CONFIRM> ] [ <SCOPE> ]
	// [ <POLICY_DATA> ... ] <STYLE> <flow descriptor list>".
	case MsgTypeResv:
		required = []uint8{ClassSession, ClassRSVPHop, ClassTimeValues, ClassStyle}
	// RFC 2205 Section 3.1.5: "<PathTear Message> ::= <Common Header>
	// [ <INTEGRITY> ] <SESSION> <RSVP_HOP> [ <sender descriptor> ]".
	case MsgTypePathTear:
		required = []uint8{ClassSession, ClassRSVPHop}
	// RFC 2205 Section 3.1.7: "<PathErr message> ::= <Common Header>
	// [ <INTEGRITY> ] <SESSION> <ERROR_SPEC> [ <POLICY_DATA> ...]
	// [ <sender descriptor> ]".
	case MsgTypePathErr:
		required = []uint8{ClassSession, ClassErrorSpec}
	// ResvErr, ResvTear and ResvConf reach no handler in engine.go, so ze states
	// no requirement for them rather than one nothing reads.
	default:
		return nil
	}

	for _, classNum := range required {
		name, present := objectPresence(msg, classNum)
		if !present {
			return fmt.Errorf("%w: %s in message type %d", errObjectAbsent, name, msg.Header.MsgType)
		}
	}
	return nil
}

// objectPresence reports whether msg carried an object of this class, with the
// RFC's name for the class for the log line the caller writes.
//
// It panics on a class no list above names. The lists are compile-time constants
// in this file, so an unnamed class is a programmer error rather than anything a
// peer can produce, and returning false would refuse every message of that type.
func objectPresence(msg *ParsedMessage, classNum uint8) (string, bool) {
	switch classNum {
	case ClassSession:
		return "SESSION", msg.HasSession
	case ClassRSVPHop:
		return "RSVP_HOP", msg.HasHop
	case ClassTimeValues:
		return "TIME_VALUES", msg.HasTimeValues
	case ClassStyle:
		return "STYLE", msg.HasStyle
	case ClassErrorSpec:
		return "ERROR_SPEC", msg.HasErrorSpec
	}
	panic("BUG: rsvp: mandatory object class with no presence flag")
}
