package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestResponderFirstRequestMatchesWhatTheInitiatorExpects pairs the two producers that
// have to agree about a Message ID, and it drives the direction the handshake never
// exercises: a request raised by the ORIGINAL RESPONDER.
//
// VALIDATES: the id the original responder puts on its first self-initiated request is
// the id the original initiator classifies as a new request.
// PREVENTS: an original responder that can never start an exchange. Its DPD probe, its
// Delete, its Child SA rekey and its IKE SA rekey all carry that id, and classifyInbound
// (msgid.go) matches sa.ExpectedMsgID EXACTLY, so a mismatch drops every one of them.
// MEASURED 2026-08-18 in QEMU before the fix: test/ipsec/ipsec-child-rekey-xfrm.ci had
// the responding daemon send "child-sa: rekey initiated msgid=2", the initiating daemon
// answer nothing, and the Child SA reach "hard lifetime expired" five seconds later.
//
// NOT TAGGED with an RFC requirement id. The obligation it proves is stated in Section
// 2.2 in indicative prose that carries no RFC 2119 keyword, so rfc/short/rfc7296.md --
// whose rows are keyword sentences quoted verbatim -- has no row for it. RFC7296-2.2-1
// is about RETRANSMISSION reusing an id and RFC7296-2.2-2 about the 32-bit ceiling;
// neither is this. The unextracted obligation is raised rather than mislabeled here
// (ai/rules/rfc-compliance.md, Extraction Completeness).
//
// Section 2.2: "Each endpoint in the IKE
// Security Association maintains two 'current' Message IDs: the next one to be used for
// a request it initiates and the next one it expects to see in a request from the other
// end." The two counters are INDEPENDENT, so "each integer n may appear as the Message
// ID in four distinct messages: the nth request from the original IKE initiator, the
// corresponding response, the nth request from the original IKE responder, and the
// corresponding response" (rfc/full/rfc7296.txt, Section 2.2). The responder has
// generated no request of its own when IKE_AUTH completes, so its next one is 0.
//
// RFC requirement: RFC7296-2.2-3 positive -- the two counters are independent, so a
// responder that has raised no request of its own sends its first at Message ID 0, which
// is exactly what a conforming initiator expects. finishResponderEstablish (responder.go)
// leaves NextMsgID alone and advances only the inbound counter.
func TestResponderFirstRequestMatchesWhatTheInitiatorExpects(t *testing.T) {
	log := slogutil.DiscardLogger()

	// The original RESPONDER, at the moment it answers IKE_AUTH at Message ID 1.
	responder := testSA()
	responder.IsInitiator = false
	ps := &PeerSession{peerName: responder.PeerName}
	ps.finishResponderEstablish(responder, 1, []byte("cached response"), nil, nil, nil, log)

	// The original INITIATOR, at the same moment. handleAuthResponse (fsm.go) sets
	// NextMsgID to 1 for IKE_AUTH and then calls advanceMsgID, and nothing on the
	// initiator's handshake path calls cacheResponse, so its inbound counter is
	// untouched: the initiator has answered no request.
	initiator := testSA()
	initiator.IsInitiator = true
	initiator.NextMsgID = 1
	initiator.advanceMsgID()

	if initiator.ExpectedMsgID != 0 {
		t.Fatalf("the initiator expects request id %d after IKE_AUTH, want 0; this test's "+
			"premise about the initiator's inbound counter is gone", initiator.ExpectedMsgID)
	}
	if got := classifyInbound(initiator, responder.NextMsgID, false, nil); got != inboundNewRequest {
		t.Errorf("the initiator classified the responder's first request (id %d) as %v, want "+
			"inboundNewRequest; the original responder can raise no exchange at all -- no DPD "+
			"probe, no Delete, no Child SA rekey, no IKE SA rekey (it expects id %d)",
			responder.NextMsgID, got, initiator.ExpectedMsgID)
	}

	// The counters are independent, so answering the peer's requests must not move the
	// responder's OWN request id. Asserted separately from the pairing above, because a
	// producer that happened to land on 0 by another route would satisfy that check
	// while still coupling the two counters.
	if responder.NextMsgID != 0 {
		t.Errorf("the responder's first self-initiated request is id %d, want 0; it was "+
			"derived from the id of a request the PEER sent, and Section 2.2 gives each end "+
			"its own counter", responder.NextMsgID)
	}
	// The peer's counter still advances on the responder, and it is the other half of the
	// pair: the initiator's next request is 2 and the responder must expect exactly that.
	if responder.ExpectedMsgID != initiator.NextMsgID {
		t.Errorf("the responder expects request id %d while the initiator's next request is "+
			"%d", responder.ExpectedMsgID, initiator.NextMsgID)
	}
}
