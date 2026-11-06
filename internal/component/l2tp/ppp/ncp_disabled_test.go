// Design: docs/guide/l2tp.md -- a disabled NCP answers with Protocol-Reject in Opened.

package ppp

import "testing"

// TestDisabledIPCPIsRejected: with `l2tp ncp enable-ipcp false` the IPCP automaton
// never leaves Initial, so every retransmission is answered here or never. RFC
// 1661 Section 5.7: "If the LCP automaton is in the Opened state, then this MUST
// be reported back to the peer by transmitting a Protocol-Reject."
func TestDisabledIPCPIsRejected(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.ncpStarted = true
	const retransmissions = 4
	for id := range uint8(retransmissions) {
		if s.handleFrame(lcpFrame(ProtoIPCP, LCPConfigureRequest, id, []byte{3, 6, 0, 0, 0, 0})) {
			t.Fatal("a disabled IPCP frame ended the session")
		}
	}
	replies := decodeFrames(t, rec)
	if len(replies) != retransmissions {
		t.Fatalf("replies = %d, want one Protocol-Reject per frame", len(replies))
	}
	for _, reply := range replies {
		if reply.Proto != ProtoLCP || reply.Pkt.Code != LCPProtocolReject {
			t.Fatalf("reply = %+v, want Protocol-Reject", reply)
		}
		if rejected := rejectedProtocol(t, reply.Pkt); rejected != ProtoIPCP {
			t.Fatalf("rejected protocol = %#x, want IPCP", rejected)
		}
	}
}

// TestDisabledIPCPOutsideOpenedStaysSilent is the negative: the reject is owed in
// Opened alone (RFC 1661 Section 5.7).
func TestDisabledIPCPOutsideOpenedStaysSilent(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	s.ncpStarted = true
	if s.handleFrame(lcpFrame(ProtoIPCP, LCPConfigureRequest, 1, []byte{3, 6, 0, 0, 0, 0})) {
		t.Fatal("a disabled IPCP frame ended the session")
	}
	if rec.count() != 0 {
		t.Fatalf("wrote %d frames outside Opened", rec.count())
	}
}
