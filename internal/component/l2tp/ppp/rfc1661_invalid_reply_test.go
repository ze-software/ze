// Related: ncp.go -- handleNCPPacket and the three absorb callbacks under test
// Related: rfc1661_test.go -- newRFC1661Session and the frame recorder

package ppp

import "testing"

// ipcpRejectFrame builds one IPCP packet carrying data, ready for
// handleIPCPPacket.
func ipcpPacket(code, id uint8, data []byte) LCPPacket {
	return LCPPacket{Code: code, Identifier: id, Data: data}
}

// newIPCPSession returns a session whose IPCP automaton sits in Req-Sent, the
// state RFC 1661 Section 4.3 answers an RCN event in with "irc,scr/6": a fresh
// Configure-Request onto the wire. A packet the RFC discards therefore leaves
// both the state and the recorder untouched, and a packet it accepts does not.
func newIPCPSession(t *testing.T) (*pppSession, *frameRecorder) {
	t.Helper()
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.setNCPState(AddressFamilyIPv4, LCPStateReqSent)
	return s, rec
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
	s, rec := newIPCPSession(t)

	// IP-Address at Length 1 counts fewer octets than its own header holds,
	// so the option list does not parse. The packet is not truncated, so it
	// reaches absorbIPCPReject rather than the framing guard above it.
	term := s.handleIPCPPacket(ipcpPacket(LCPConfigureReject, 0x41, []byte{IPCPOptIPAddress, 1}))

	if term {
		t.Fatal("an unreadable IPCP Configure-Reject ended the session; RFC 1661 Section 5.4 silently discards it")
	}
	if got := s.ncpState(AddressFamilyIPv4); got != LCPStateReqSent {
		t.Fatalf("IPCP state = %s, want Req-Sent unchanged: RFC 1661 Section 4.3 discards the packet 'without affecting the automaton'", got)
	}
	if c := rec.count(); c != 0 {
		t.Fatalf("ze answered an unreadable Configure-Reject with %d frames, want silence", c)
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
	s, _ := newIPCPSession(t)

	data := []byte{IPCPOptIPAddress, 6, 10, 0, 0, 1}
	if term := s.handleIPCPPacket(ipcpPacket(LCPConfigureReject, 0x42, data)); !term {
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
	s, rec := newIPCPSession(t)

	term := s.handleIPCPPacket(ipcpPacket(LCPConfigureNak, 0x43, []byte{IPCPOptIPAddress, 1}))

	if term {
		t.Fatal("an unreadable IPCP Configure-Nak ended the session")
	}
	if got := s.ncpState(AddressFamilyIPv4); got != LCPStateReqSent {
		t.Fatalf("IPCP state = %s, want Req-Sent unchanged", got)
	}
	if c := rec.count(); c != 0 {
		t.Fatalf("ze answered an unreadable Configure-Nak with %d frames, want silence", c)
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
	s, rec := newIPCPSession(t)

	data := []byte{IPCPOptIPAddress, 6, 10, 0, 0, 1}
	if term := s.handleIPCPPacket(ipcpPacket(LCPConfigureNak, 0x44, data)); term {
		t.Fatal("a valid IPCP Configure-Nak ended the session")
	}
	if c := rec.count(); c == 0 {
		t.Fatal("a valid IPCP Configure-Nak drew no Configure-Request; RFC 1661 Section 4.3 gives Req-Sent the RCN action irc,scr")
	}
}
