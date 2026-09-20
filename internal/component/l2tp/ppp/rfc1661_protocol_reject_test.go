// Design: internal/component/l2tp/ppp/session_run.go -- the Protocol-Reject sender
// Related: rfc1661_test.go -- the shared RFC 1661 session fixture and frame recorder
// Related: rfc/short/rfc1661.md -- RFC1661-3.6-3 and RFC1661-5.7-1

package ppp

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// protoMPLSCP is MPLSCP (RFC 3032), a control protocol ze does not speak. It
// satisfies both RFC 1661 Section 2 parity rules, so a frame carrying it is a
// legal PPP frame naming a protocol the implementation does not support, which
// is the exact input Section 5.7 answers.
const protoMPLSCP uint16 = 0x8281

// rejectedProtocol reads the Rejected-Protocol field of a Protocol-Reject.
// RFC 1661 Section 5.7 says of it: "The Rejected-Protocol field is two octets,
// and contains the PPP Protocol field of the packet which is being rejected".
func rejectedProtocol(t *testing.T, pkt LCPPacket) uint16 {
	t.Helper()
	if len(pkt.Data) < 2 {
		t.Fatalf("Protocol-Reject Data is %d octets, want at least the 2-octet Rejected-Protocol", len(pkt.Data))
	}
	return binary.BigEndian.Uint16(pkt.Data[:2])
}

// VALIDATES: a frame naming a protocol ze does not support, received while LCP
//
//	is Opened, is answered with an LCP Protocol-Reject that names the rejected
//	protocol and carries the rejected packet back.
//
// PREVENTS: the silent drop that stood here until 2026-09-20, which left the
//
//	peer retransmitting a protocol ze will never answer until its own
//	Configure-Request budget ran out.
//
// RFC requirement: RFC1661-5.7-1 positive -- handleFrame (session_run.go)
// routes a protocol it does not recognize to rejectUnsupportedProtocol, which
// answers an unsupported protocol in the Opened state with sendProtocolReject:
// one LCP frame, code 8, whose Rejected-Protocol is the protocol of the frame
// received and whose Rejected-Information is that frame's Information field.
//
// RFC requirement: RFC1661-3.6-3 positive -- the same answer is what Section
// 3.6 states for a protocol packet the implementation does not support while
// LCP is Opened, and supportsProtocol (session_run.go) is what tells that case
// apart from a supported protocol whose NCP has not opened yet.
func TestRFC1661UnsupportedProtocolRejectedInOpened(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	info := []byte{0x01, 0x02, 0x00, 0x04}
	frame := make([]byte, MaxFrameLen)
	n := WriteFrame(frame, 0, protoMPLSCP, info)

	if term := s.handleFrame(frame[:n]); term {
		t.Fatal("handleFrame terminated the session on an unsupported protocol")
	}
	if c := rec.count(); c != 1 {
		t.Fatalf("wrote %d frames in answer to an unsupported protocol, want 1 Protocol-Reject", c)
	}
	got := decodeFrames(t, rec)[0]
	if got.Proto != ProtoLCP {
		t.Fatalf("answer carried protocol 0x%04x, want LCP 0x%04x", got.Proto, ProtoLCP)
	}
	if got.Pkt.Code != LCPProtocolReject {
		t.Fatalf("answer carried LCP code %d, want Protocol-Reject %d", got.Pkt.Code, LCPProtocolReject)
	}
	if rp := rejectedProtocol(t, got.Pkt); rp != protoMPLSCP {
		t.Fatalf("Rejected-Protocol = 0x%04x, want 0x%04x", rp, protoMPLSCP)
	}
	if !bytes.Equal(got.Pkt.Data[2:], info) {
		t.Fatalf("Rejected-Information = % x, want the rejected frame's Information field % x", got.Pkt.Data[2:], info)
	}
	if st := s.currentState(); st != LCPStateOpened {
		t.Fatalf("state = %s, want opened (answering a Protocol-Reject moves no LCP state)", st)
	}
}

// VALIDATES: the same unsupported protocol, received while LCP has not reached
//
//	Opened, is dropped without a Protocol-Reject.
//
// PREVENTS: a sender that answers every unsupported protocol whatever the LCP
//
//	state, which would put Protocol-Reject packets on a link that has not
//	finished negotiating.
//
// RFC requirement: RFC1661-5.7-1 negative -- rejectUnsupportedProtocol
// (session_run.go) reads the LCP state before it answers, because Section 5.7
// says "Protocol-Reject packets can only be sent in the LCP Opened state", so
// an unsupported protocol arriving in Ack-Sent writes no frame at all.
//
// RFC requirement: RFC1661-3.6-3 negative -- Section 3.6 makes the answer
// conditional in the same words, "While LCP is in the Opened state", so the
// same frame in a state that is not Opened earns no Protocol-Reject.
func TestRFC1661UnsupportedProtocolNotRejectedBeforeOpened(t *testing.T) {
	for _, state := range []LCPState{LCPStateReqSent, LCPStateAckSent, LCPStateStopped} {
		s, rec, _ := newRFC1661Session(state)
		frame := make([]byte, MaxFrameLen)
		n := WriteFrame(frame, 0, protoMPLSCP, []byte{0x01, 0x02, 0x00, 0x04})

		if term := s.handleFrame(frame[:n]); term {
			t.Fatalf("state %s: handleFrame terminated the session", state)
		}
		if c := rec.count(); c != 0 {
			t.Fatalf("state %s: wrote %d frames, want 0 (a Protocol-Reject is Opened-only)", state, c)
		}
	}
}

// rejectFrame drives one unsupported frame carrying info through an Opened
// session and answers the raw Protocol-Reject frame ze wrote for it.
func rejectFrame(t *testing.T, s *pppSession, rec *frameRecorder, info []byte) []byte {
	t.Helper()
	frame := make([]byte, MaxFrameLen)
	n := WriteFrame(frame, 0, protoMPLSCP, info)
	before := rec.count()
	if term := s.handleFrame(frame[:n]); term {
		t.Fatal("handleFrame terminated the session on an unsupported protocol")
	}
	all := rec.all()
	if len(all) != before+1 {
		t.Fatalf("wrote %d frames for one unsupported protocol, want 1", len(all)-before)
	}
	return all[len(all)-1]
}

// VALIDATES: two Protocol-Rejects sent on one session carry two different
//
//	Identifiers.
//
// PREVENTS: the constant Identifier every other LCP sender in this file writes,
//
//	which would let a peer match one reply to the wrong rejected packet.
//
// RFC requirement: RFC1661-5.7-3 positive -- sendProtocolReject (session_run.go)
// advances protocolRejectID (session.go) before it writes each packet, so the
// second Protocol-Reject on a session carries an Identifier the first did not.
func TestRFC1661ProtocolRejectIdentifierChanges(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	info := []byte{0x01, 0x02, 0x00, 0x04}

	rejectFrame(t, s, rec, info)
	rejectFrame(t, s, rec, info)

	got := decodeFrames(t, rec)
	if len(got) != 2 {
		t.Fatalf("recorded %d frames, want 2 Protocol-Rejects", len(got))
	}
	if got[0].Pkt.Identifier == got[1].Pkt.Identifier {
		t.Fatalf("both Protocol-Rejects carry Identifier %d; the Identifier MUST change for each one",
			got[0].Pkt.Identifier)
	}
}

// VALIDATES: the Rejected-Information of a Protocol-Reject is cut so the LCP
//
//	packet fits the MRU the peer asked for.
//
// PREVENTS: a copy bounded only by the 1500-octet frame buffer, which would
//
//	send a peer that negotiated a 100-octet MRU a frame it MUST discard, so the
//	rejection would never be read.
//
// RFC requirement: RFC1661-5.7-4 positive -- sendProtocolReject (session_run.go)
// reads negotiatedMRU and copies at most that many octets less the 4-octet LCP
// header and the 2-octet Rejected-Protocol, so the LCP Length of the answer is
// the peer's MRU exactly rather than the length of the packet rejected.
func TestRFC1661ProtocolRejectInformationTruncatedToMRU(t *testing.T) {
	const peerMRU = 100
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.negotiatedMRU = peerMRU

	info := bytes.Repeat([]byte{0x5a}, 400)
	frame := rejectFrame(t, s, rec, info)

	if len(frame) != frameLen(peerMRU) {
		t.Fatalf("the answer is %d octets, want %d: the Information field MUST fit the peer's MRU of %d",
			len(frame), frameLen(peerMRU), peerMRU)
	}
	pkt := decodeFrames(t, rec)[0].Pkt
	kept := pkt.Data[2:]
	if want := peerMRU - lcpHeaderLen - 2; len(kept) != want {
		t.Fatalf("Rejected-Information is %d octets, want %d", len(kept), want)
	}
	if !bytes.Equal(kept, info[:len(kept)]) {
		t.Fatal("the Rejected-Information kept is not the head of the packet rejected")
	}
}

// VALIDATES: a rejected packet that already fits the peer's MRU is carried back
//
//	whole.
//
// PREVENTS: a truncation applied unconditionally, which would cut the copy on
//
//	every Protocol-Reject and leave the peer unable to tell which packet ze
//	refused.
//
// RFC requirement: RFC1661-5.7-4 negative -- the cut in sendProtocolReject
// (session_run.go) is bounded by the MRU and applies only where the copy
// exceeds it, so a short rejected packet reaches the peer entire.
func TestRFC1661ProtocolRejectInformationKeptWhenItFits(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	info := bytes.Repeat([]byte{0x33}, 40)

	rejectFrame(t, s, rec, info)

	pkt := decodeFrames(t, rec)[0].Pkt
	if !bytes.Equal(pkt.Data[2:], info) {
		t.Fatalf("Rejected-Information is %d octets, want the whole %d-octet packet back",
			len(pkt.Data[2:]), len(info))
	}
}
