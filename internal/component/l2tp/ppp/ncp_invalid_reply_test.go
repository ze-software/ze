// Related: ncp.go -- handleNCPPacket and the three absorb callbacks under test
// Related: rfc1661_test.go -- newRFC1661Session and the frame recorder

package ppp

import (
	"bytes"
	"testing"
)

// ipcpPacket builds one IPCP packet for handleIPCPPacket.
func ipcpPacket(code, id uint8, data []byte) LCPPacket {
	return LCPPacket{Code: code, Identifier: id, Data: data}
}

// newIPCPSession returns a session whose IPCP automaton sits in Req-Sent, the
// state RFC 1661 Section 4.3 answers an RCN event in with "irc,scr/6": a fresh
// Configure-Request onto the wire. A packet the RFC discards therefore leaves
// both the state and the recorder untouched, and a packet it accepts does not.
func newIPCPSession(t *testing.T) (*pppSession, *frameRecorder, LCPPacket) {
	t.Helper()
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.localIPv4 = ipcpTestLocal
	s.setNCPState(AddressFamilyIPv4, LCPStateReqSent)
	if !s.sendNCPConfigureRequest(AddressFamilyIPv4) {
		t.Fatal("send initial IPCP Configure-Request")
	}
	frames := decodeFrames(t, rec)
	if len(frames) != 1 || frames[0].Proto != ProtoIPCP || frames[0].Pkt.Code != LCPConfigureRequest {
		t.Fatalf("initial IPCP request = %+v", frames)
	}
	return s, rec, frames[0].Pkt
}

// VALIDATES: an IPCP Configure-Reject whose option list does not parse is
//
//	dropped without a reply, without a state change and without tearing the
//	session down.
//
// PREVENTS: the teardown that stood here until 2026-09-20. absorbIPCPReject
// returned the same true for "the peer rejected IP-Address" and for "ze could
// not read the packet", and handleNCPPacket turned the second into
// s.fail("peer Configure-Reject of mandatory option"). Any unauthenticated
// peer could end an established session with two octets.
//
// RFC 1661 Section 5.4 says of a received Configure-Reject: "the Configuration
// Options in a Configure-Reject MUST be a proper subset of those in the last
// transmitted Configure-Request. Invalid packets are silently discarded." RFC
// 1661 Section 4.3 says the same of the event it raises: "An out of sequence
// or otherwise invalid packet is silently discarded."
// So the session stays up and the negotiation keeps the state it already had.
func TestNCPInvalidConfigureRejectIsSilentlyDiscarded(t *testing.T) {
	s, rec, request := newIPCPSession(t)

	// The reply names the current request, but its option length cannot
	// contain the option header.
	term := s.handleIPCPPacket(ipcpPacket(LCPConfigureReject, request.Identifier, []byte{IPCPOptIPAddress, 1}))

	if term {
		t.Fatal("an unreadable IPCP Configure-Reject ended the session; RFC 1661 Section 5.4 silently discards it")
	}
	if got := s.ncpState(AddressFamilyIPv4); got != LCPStateReqSent {
		t.Fatalf("IPCP state = %s, want Req-Sent unchanged: RFC 1661 Section 4.3 discards the packet 'without affecting the automaton'", got)
	}
	if c := rec.count(); c != 1 {
		t.Fatalf("frame count after unreadable Configure-Reject = %d, want only the initial request", c)
	}
}

// VALIDATES: a well-formed IPCP Configure-Reject naming IP-Address still ends
//
//	the session, so the discard above is about readability and not about the
//	peer's refusal.
//
// RFC 1661 Section 5.4: "Reception of a valid Configure-Reject indicates that
// when a new Configure-Request is sent, it MUST NOT include any of the
// Configuration Options listed in the Configure-Reject." Ze cannot bring IPCP
// up without the IP-Address option, so a valid Reject of it ends the session.
func TestNCPValidConfigureRejectOfIPAddressStillFails(t *testing.T) {
	s, _, request := newIPCPSession(t)

	if term := s.handleIPCPPacket(ipcpPacket(LCPConfigureReject, request.Identifier, request.Data)); !term {
		t.Fatal("a valid IPCP Configure-Reject of IP-Address did not end the session")
	}
}

// VALIDATES: an IPCP Configure-Nak whose option list does not parse feeds the
//
//	automaton nothing.
//
// PREVENTS: absorbIPCPNak returning early on the parse error while
// handleNCPPacket ran the RCN event anyway, which restarted the negotiation
// from a packet RFC 1661 Section 4.3 says never happened.
//
// RFC 1661 Section 5.3 says of a received Configure-Nak: "On reception of a
// Configure-Nak, the Identifier field MUST match that of the last transmitted
// Configure-Request. Invalid packets are silently discarded."
// So the RCN event never runs and the negotiation is not restarted.
func TestNCPInvalidConfigureNakIsSilentlyDiscarded(t *testing.T) {
	s, rec, request := newIPCPSession(t)

	term := s.handleIPCPPacket(ipcpPacket(LCPConfigureNak, request.Identifier, []byte{IPCPOptIPAddress, 1}))

	if term {
		t.Fatal("an unreadable IPCP Configure-Nak ended the session")
	}
	if got := s.ncpState(AddressFamilyIPv4); got != LCPStateReqSent {
		t.Fatalf("IPCP state = %s, want Req-Sent unchanged", got)
	}
	if c := rec.count(); c != 1 {
		t.Fatalf("frame count after unreadable Configure-Nak = %d, want only the initial request", c)
	}
}

// VALIDATES: a well-formed IPCP Configure-Nak is absorbed and does drive the
//
//	automaton, so the silence above is the invalid packet and not the code
//	path.
//
// RFC 1661 Section 4.3 gives Req-Sent the RCN action "irc,scr/6", so a valid
// Nak draws a fresh Configure-Request.
func TestNCPValidConfigureNakDrivesTheAutomaton(t *testing.T) {
	s, rec, request := newIPCPSession(t)

	data := []byte{IPCPOptIPAddress, 6, 10, 0, 0, 254}
	if term := s.handleIPCPPacket(ipcpPacket(LCPConfigureNak, request.Identifier, data)); term {
		t.Fatal("a valid IPCP Configure-Nak ended the session")
	}
	frames := decodeFrames(t, rec)
	if len(frames) != 2 {
		t.Fatalf("frame count after valid Configure-Nak = %d, want initial and replacement requests", len(frames))
	}
	reply := frames[1]
	if reply.Proto != ProtoIPCP || reply.Pkt.Code != LCPConfigureRequest ||
		reply.Pkt.Identifier == request.Identifier || !bytes.Equal(reply.Pkt.Data, data) {
		t.Fatalf("replacement request = %+v, want a new identifier and the suggested address", reply)
	}
}
