package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// errUnknownNotifyType is an unassigned value in the ERROR half of the notify number
// space (RFC 7296 Section 3.10.1: types 0 to 16383 report errors). No RFC assigns it,
// so ze must not recognize it.
const errUnknownNotifyType uint16 = 9999

// errUnknownStatusType is an unassigned value in the STATUS half of the same space.
const errUnknownStatusType uint16 = 40961

// VALIDATES: a CREATE_CHILD_SA request the responder refuses draws an error notify
// rather than silence, and the IKE SA survives it.
// RFC requirement: RFC7296-2.21.3-1 positive -- handleCreateChildSAOwned answers a
// refused rekey through respondError (inbound.go).
// The peer therefore learns the outcome.
// It does not spend its single request window on retransmissions, and it does not
// then close a working IKE SA.
// The answer is encrypted, echoes the request Message ID, and the SA stays
// established.
// RFC requirement: RFC7296-2.21.3-1 negative -- a request the responder CAN satisfy
// draws a normal CREATE_CHILD_SA response carrying an SA payload and no error notify,
// so the notify is emitted for the failing case alone.
func TestErrRefusedChildRekeyIsAnswered(t *testing.T) {
	log := slogutil.DiscardLogger()

	t.Run("a refused rekey draws NO_PROPOSAL_CHOSEN", func(t *testing.T) {
		link := errLink(t)
		ini := link.ini
		resp := link.resp
		peerTr := link.peerTr
		myTr := link.myTr
		dp := &rkyDP{}
		ps := &PeerSession{peerName: "err", espGroup: testESPGroup()}
		old, err := createFirstChildSA(resp, testESPGroup(), "10.0.0.2", "10.0.0.1", 1, dp, log)
		if err != nil {
			t.Fatalf("createFirstChildSA: %v", err)
		}
		ps.setChildSA(old)

		// An offer whose ESP proposal names an algorithm the responder does not accept.
		inner := rkyChildRekeyRequestUnmatched(old.InboundSPI, 0x0A0B0C0D, testNonce(21))
		msg := &wire.Message{Header: wire.Header{MessageID: resp.ExpectedMsgID}}
		msgID := resp.ExpectedMsgID
		out := ps.handleCreateChildSAOwned(resp, msg, inner, false, myTr, dp, log)
		if out.newChild != nil {
			t.Fatal("an unmatched offer still produced a replacement Child SA")
		}

		got := rtxRecv(t, peerTr)
		if got == nil {
			t.Fatal("a refused rekey drew no answer at all")
		}
		answer := parseMsg(t, got)
		if answer.Header.MessageID != msgID {
			t.Errorf("the answer's message ID = %d, want the request's %d", answer.Header.MessageID, msgID)
		}
		if answer.Header.Flags&wire.FlagResponse == 0 {
			t.Error("the answer is not marked as a response, so it starts a new exchange")
		}
		if !carriesSKPayload(answer) {
			t.Error("the answer to an authenticated request is not encrypted")
		}
		types := errNotifyIn(t, ini, got)
		if len(types) != 1 || types[0] != wire.NotifyNoProposalChosen {
			t.Errorf("the answer carries notifies %v, want exactly NO_PROPOSAL_CHOSEN", types)
		}
		if resp.State != StateEstablished {
			t.Errorf("the IKE SA is %v after refusing one rekey, want it still established", resp.State)
		}
		// RFC 7296 Section 2.1: a retransmission of the same request draws the same
		// bytes, so the refusal is cached like any other response.
		if !resp.lastResponseSet || resp.lastResponseID != msgID {
			t.Error("the refusal was not cached, so a retransmitted request is reprocessed")
		}
	})

	t.Run("a satisfiable rekey draws a normal response", func(t *testing.T) {
		link := errLink(t)
		ini := link.ini
		resp := link.resp
		peerTr := link.peerTr
		myTr := link.myTr
		dp := &rkyDP{}
		ps := &PeerSession{peerName: "err-ok", espGroup: testESPGroup()}
		old, err := createFirstChildSA(resp, testESPGroup(), "10.0.0.2", "10.0.0.1", 1, dp, log)
		if err != nil {
			t.Fatalf("createFirstChildSA: %v", err)
		}
		ps.setChildSA(old)

		inner := rkyChildRekeyRequest(old.InboundSPI, 0x0A0B0C0D, testNonce(22))
		msg := &wire.Message{Header: wire.Header{MessageID: resp.ExpectedMsgID}}
		out := ps.handleCreateChildSAOwned(resp, msg, inner, false, myTr, dp, log)
		if out.newChild == nil {
			t.Fatal("a satisfiable rekey installed no replacement")
		}
		got := rtxRecv(t, peerTr)
		if got == nil {
			t.Fatal("a satisfiable rekey drew no answer")
		}
		for _, ntype := range errNotifyIn(t, ini, got) {
			if wire.NotifyIsError(ntype) {
				t.Errorf("a satisfiable rekey drew error notify %d", ntype)
			}
		}
	})
}

// VALIDATES: a request for a NEW Child SA, which ze does not create, is answered.
// RFC requirement: RFC7296-2.21.3-1 positive -- a CREATE_CHILD_SA that is neither a
// Child rekey nor an IKE rekey was logged and dropped. It now draws NO_PROPOSAL_CHOSEN,
// which RFC 7296 Section 3.10.1 blesses as the "generic Child SA error when Child SA
// cannot be created for some other reason".
// RFC requirement: RFC7296-2.21.3-1 negative -- the same handler answers a RESPONSE
// with nothing, because RFC 7296 Section 3.1 forbids generating a response to a
// response. So the notify is bound to the request case.
func TestErrNewChildRequestIsAnswered(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ini := link.ini
	resp := link.resp
	peerTr := link.peerTr
	myTr := link.myTr
	remote := link.remote
	ps := &PeerSession{peerName: "err-new", espGroup: testESPGroup()}

	// A CREATE_CHILD_SA carrying only a nonce: no REKEY_SA notify, no KE payload.
	inner := []wire.PayloadEntry{{Payload: &wire.PayloadNonce{NonceData: testNonce(31)}}}
	msg := &wire.Message{Header: wire.Header{MessageID: resp.ExpectedMsgID}}
	ps.handleCreateChildSAOwned(resp, msg, inner, false, myTr, nil, log)

	got := rtxRecv(t, peerTr)
	if got == nil {
		t.Fatal("a request for a new Child SA drew no answer")
	}
	types := errNotifyIn(t, ini, got)
	if len(types) != 1 || types[0] != wire.NotifyNoProposalChosen {
		t.Errorf("the answer carries notifies %v, want exactly NO_PROPOSAL_CHOSEN", types)
	}

	// Negative. The same shape delivered as a RESPONSE draws nothing.
	ps2 := &PeerSession{peerName: "err-new-resp", espGroup: testESPGroup()}
	msg2 := &wire.Message{Header: wire.Header{MessageID: resp.ExpectedMsgID, Flags: wire.FlagResponse}}
	ps2.handleCreateChildSAOwned(resp, msg2, inner, true, myTr, nil, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "after a CREATE_CHILD_SA response")
}

// VALIDATES: a malformed inner chain draws INVALID_SYNTAX, and a malformed OUTER
// message draws nothing at all.
// RFC requirement: RFC7296-2.21.2-1 positive -- a request whose decrypted inner chain
// is truncated is rejected in its entirety and answered with INVALID_SYNTAX, which is
// one of the two notifications RFC 7296 Section 2.21.2 permits for that case.
// RFC requirement: RFC7296-3.10.1-3 positive -- INVALID_SYNTAX is the answer to an
// error no other status type covers, and it is sent inside an encrypted packet after
// the Message ID and the cryptographic checksum were both validated.
// RFC requirement: RFC7296-3.10.1-3 negative -- an OUTER parse failure, which happens
// before any decryption, draws NOTHING.
// RFC 7296 Section 3.10.1 permits this status only
// "for and in an encrypted packet if the Message ID and cryptographic checksum were valid".
// Neither was checked at that point.
// An answer there turns a 28-byte forgery into a guaranteed reply.
// That is the denial-of-service the sentence exists to prevent.
// This negative is the only proof that the precondition is honored.
func TestErrInnerParseFailureDrawsInvalidSyntaxAndOuterDrawsNothing(t *testing.T) {
	log := slogutil.DiscardLogger()

	t.Run("a truncated inner chain draws INVALID_SYNTAX", func(t *testing.T) {
		link := errLink(t)
		ini := link.ini
		resp := link.resp
		ps := link.ps
		peerTr := link.peerTr
		myTr := link.myTr
		msgID := resp.ExpectedMsgID

		// A well-formed encrypted INFORMATIONAL whose inner chain declares a payload
		// longer than the plaintext holds.
		// The SK payload decrypts and its integrity check passes.
		// Only the chain inside it is malformed.
		req := errTruncatedInnerRequest(t, ini, msgID)
		out := ps.handleOwnedInbound(resp, transport.Packet{Data: req}, myTr, nil, log)
		if out.peerAlive {
			t.Error("a malformed request reached the exchange handlers")
		}
		got := rtxRecv(t, peerTr)
		if got == nil {
			t.Fatal("a malformed inner chain drew no answer")
		}
		types := errNotifyIn(t, ini, got)
		if len(types) != 1 || types[0] != wire.NotifyInvalidSyntax {
			t.Errorf("the answer carries notifies %v, want exactly INVALID_SYNTAX", types)
		}
	})

	t.Run("an outer parse failure draws nothing", func(t *testing.T) {
		link := errLink(t)
		resp := link.resp
		ps := link.ps
		peerTr := link.peerTr
		myTr := link.myTr
		remote := link.remote
		// A 28-byte header claiming a length far past the datagram. Message.ReadFrom
		// refuses it before any SK payload is located.
		forged := ntfRequest(resp.InitiatorSPI, resp.ResponderSPI, wire.ExchangeInformational, resp.ExpectedMsgID, false)
		forged[24], forged[25], forged[26], forged[27] = 0, 0, 0xFF, 0xFF
		before := errorNotifySuppressedCount("outer-parse-unauthenticated")

		ps.handleOwnedInbound(resp, transport.Packet{Data: forged}, myTr, nil, log)
		rtxExpectSilence(t, peerTr, myTr, remote, "after an outer parse failure")

		if errorNotifySuppressedCount("outer-parse-unauthenticated") <= before {
			t.Error("the outer-parse guard did not record a suppression, so the drop is untraced")
		}
	})
}

// VALIDATES: an unprotected datagram at the cached Message ID draws no cached response.
// rfc-test-change-approved: the two tags below read RFC7296-2.21.4-5, which governs a
// peer receiving an unprotected NOTIFY payload. This test drives a forged unprotected
// IKE_AUTH request through classifyInbound and sends no Notify at all, so the ledger was
// crediting a Notify obligation with proof from a request test. RFC7296-2.4-12 is the
// requirement these assertions actually exercise. Ratchet-safe: 2.21.4-5 keeps both
// polarities at both tiers from notify_error_test.go and
// test/ipsec/ipsec-error-notify-no-loop.ci. Owner approved 2026-08-14.
// RFC requirement: RFC7296-2.4-12 positive -- classifyInbound runs before the message
// is authenticated, so a forged unprotected datagram carrying the cached Message ID
// used to replay the whole cached response. Both SPIs and the Message ID travel in the
// clear in every IKE header, so one observed datagram is all the forgery needs. The
// Encrypted-payload guard in the retransmit branch refuses it without a decrypt.
// RFC requirement: RFC7296-2.4-12 negative -- a genuine duplicate that DOES carry an
// Encrypted payload still draws the cached response byte for byte, so the guard
// separates a forgery from a retransmission rather than disabling the cache.
func TestErrUnprotectedMessageDrawsNoCachedResponse(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ini := link.ini
	resp := link.resp
	ps := link.ps
	peerTr := link.peerTr
	myTr := link.myTr
	remote := link.remote

	// Establish a cache entry the way a real exchange does.
	msgID := resp.ExpectedMsgID
	real := rtxIKEDelete(t, ini, msgID)
	ps.handleOwnedInbound(resp, transport.Packet{Data: real}, myTr, nil, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the first delivery drew no answer, so there is no cache to replay")
	}
	if !resp.lastResponseSet {
		t.Fatal("the first delivery cached nothing")
	}
	resp.State = StateEstablished

	// The forgery: a bare header at the cached Message ID, with no Encrypted payload.
	forged := ntfRequest(resp.InitiatorSPI, resp.ResponderSPI, wire.ExchangeInformational, resp.lastResponseID, false)
	before := errorNotifySuppressedCount("unprotected-retransmit")
	out := ps.handleOwnedInbound(resp, transport.Packet{Data: forged}, myTr, nil, log)
	if out.peerAlive {
		t.Error("an unprotected forgery credited peer liveness")
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "after an unprotected message at the cached message id")
	if errorNotifySuppressedCount("unprotected-retransmit") <= before {
		t.Error("the unprotected-retransmit guard recorded nothing")
	}

	// Negative. The genuine duplicate still replays the cache byte for byte.
	second := ps.handleOwnedInbound(resp, transport.Packet{Data: real}, myTr, nil, log)
	if second.peerAlive {
		t.Error("the duplicate was reprocessed rather than replayed")
	}
	replay := rtxRecv(t, peerTr)
	if replay == nil {
		t.Fatal("a genuine duplicate drew nothing, so the guard broke the cache")
	}
	if !bytes.Equal(replay, resp.lastResponse) {
		t.Error("the replay does not match the cached response byte for byte")
	}
}

// VALIDATES: an unprotected message changes no IKE SA or Child SA state.
// RFC requirement: RFC7296-2.21.4-6 positive -- each state mutation on an established
// SA sits behind decryptAndParse.
// The one pre-authentication branch reads only the response cache.
// An unprotected datagram therefore leaves four things untouched: the state, both
// message-ID counters, the outstanding-request flag, and the Child SA.
// It does not reset the dead-peer timer either.
// RFC requirement: RFC7296-2.21.4-6 negative -- the SAME notify delivered inside a
// valid Encrypted payload DOES reach the handlers and DOES credit liveness. So the
// freeze above is caused by the missing protection and not by notifies being inert.
// This row governs the UNPROTECTED message. RFC7296-2.21.4-7 governs the protected
// INFORMATIONAL notify, and the two look identical unless the scope is named.
func TestErrUnprotectedNotifyChangesNoState(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ini := link.ini
	resp := link.resp
	ps := link.ps
	myTr := link.myTr

	child := &ChildSA{InboundSPI: 0xAB, OutboundSPI: 0xCD}
	ps.setChildSA(child)
	beforeState, beforeExpected, beforeNext := resp.State, resp.ExpectedMsgID, resp.NextMsgID
	beforeOutstanding := resp.requestOutstanding

	forged := ntfRequest(resp.InitiatorSPI, resp.ResponderSPI, wire.ExchangeInformational, resp.ExpectedMsgID, false)
	out := ps.handleOwnedInbound(resp, transport.Packet{Data: forged}, myTr, nil, log)

	if out.peerAlive {
		t.Error("an unprotected message credited peer liveness, so it reset the dead-peer timer")
	}
	if resp.State != beforeState {
		t.Errorf("state = %v, want the unchanged %v", resp.State, beforeState)
	}
	if resp.ExpectedMsgID != beforeExpected {
		t.Errorf("ExpectedMsgID = %d, want the unchanged %d", resp.ExpectedMsgID, beforeExpected)
	}
	if resp.NextMsgID != beforeNext {
		t.Errorf("NextMsgID = %d, want the unchanged %d", resp.NextMsgID, beforeNext)
	}
	if resp.requestOutstanding != beforeOutstanding {
		t.Error("the outstanding-request flag moved")
	}
	if ps.getChildSA() != child {
		t.Error("the Child SA was replaced")
	}

	// Negative. The same INFORMATIONAL inside a valid SK payload does reach the handler.
	real := rtxIKEDelete(t, ini, resp.ExpectedMsgID)
	protected := ps.handleOwnedInbound(resp, transport.Packet{Data: real}, myTr, nil, log)
	if !protected.peerAlive {
		t.Error("a protected INFORMATIONAL did not reach the handler, so the freeze above proves nothing")
	}
}

// VALIDATES: a PROTECTED INFORMATIONAL request carrying only a notify is answered and
// changes no SA state.
// RFC requirement: RFC7296-2.21.4-7 positive -- this row closes the last paragraph of
// Section 2.21.4, which is about "a suspicious message from an IP address ... with
// which it has an IKE SA" answered by "an IKE Notify payload in an IKE INFORMATIONAL
// exchange over that SA". So it binds the recipient of a PROTECTED notify, and it is a
// DIFFERENT message from the unprotected INVALID_IKE_SPI that RFC7296-2.21.4-6 binds.
// handleInformationalOwned acts on Delete payloads alone, so the notify changes nothing
// while the request is still answered, which RFC7296-1.4-4 requires.
// RFC requirement: RFC7296-2.21.4-7 negative -- the same protected INFORMATIONAL
// carrying a Delete DOES change state, so the notify's inertness is specific to
// notifies and is not an artifact of the whole exchange being ignored.
func TestErrProtectedInformationalNotifyChangesNoState(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ini := link.ini
	resp := link.resp
	ps := link.ps
	peerTr := link.peerTr
	myTr := link.myTr

	child := &ChildSA{InboundSPI: 0x11, OutboundSPI: 0x22}
	ps.setChildSA(child)
	beforeState := resp.State

	inner := []wire.PayloadEntry{{Payload: &wire.PayloadNotify{
		NotifyMsgType: wire.NotifyInvalidSelectors,
	}}}
	msgID := resp.ExpectedMsgID
	req, err := buildEncryptedMessageEx(ini, inner, msgID, wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("build protected informational: %v", err)
	}
	out := ps.handleOwnedInbound(resp, transport.Packet{Data: req}, myTr, nil, log)
	if !out.peerAlive {
		t.Fatal("the protected INFORMATIONAL never reached the handler")
	}
	if resp.State != beforeState {
		t.Errorf("state = %v after a protected notify, want the unchanged %v", resp.State, beforeState)
	}
	if ps.getChildSA() != child {
		t.Error("a protected notify replaced the Child SA")
	}
	if rtxRecv(t, peerTr) == nil {
		t.Error("the protected INFORMATIONAL request drew no response, which RFC7296-1.4-4 requires")
	}

	// Negative. A Delete in the same exchange does change state.
	del := rtxIKEDelete(t, ini, resp.ExpectedMsgID)
	ps.handleOwnedInbound(resp, transport.Packet{Data: del}, myTr, nil, log)
	if resp.State == beforeState {
		t.Error("a protected Delete changed nothing, so the notify's inertness proves nothing")
	}
}

// VALIDATES: the initiator does not fail authentication when the IKE_AUTH response
// piggybacks an error notify for the Child SA exchange.
// RFC requirement: RFC7296-2.21.2-2 positive -- handleAuthResponse aborts on
// AUTHENTICATION_FAILED alone.
// Each other notify type falls through the collecting walk.
// AUTH is verified, and the SA is established.
// The accepted-offer consistency check is guarded on a non-nil offer.
//
// A response that carries an error notify in place of SAr2 therefore skips that
// check. It does not fail on it.
// RFC 7296 Section 2.21.2 requires exactly that:
// "the initiator MUST NOT fail the authentication because of this".
// RFC requirement: RFC7296-2.21.2-2 negative -- the same response carrying
// AUTHENTICATION_FAILED instead DOES kill the SA. So the positive is not "every notify
// is ignored", and the one notify the RFC names as fatal stays fatal.
func TestErrInitiatorSurvivesPiggybackedErrorNotify(t *testing.T) {
	for _, tc := range []struct {
		name       string
		notifyType uint16
		wantDead   bool
	}{
		{"NO_PROPOSAL_CHOSEN is survivable", wire.NotifyNoProposalChosen, false},
		{"FAILED_CP_REQUIRED is survivable", wire.NotifyFailedCPRequired, false},
		{"TS_UNACCEPTABLE is survivable", wire.NotifyTSUnacceptable, false},
		{"AUTHENTICATION_FAILED is fatal", wire.NotifyAuthenticationFailed, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := errDriveAuthResponseWithNotify(t, tc.notifyType)
			if tc.wantDead && state != StateDead {
				t.Errorf("state = %v, want dead", state)
			}
			if !tc.wantDead && state != StateEstablished {
				t.Errorf("state = %v, want established: the initiator failed authentication "+
					"over a piggybacked error notify", state)
			}
		})
	}
}

// VALIDATES: an unrecognized ERROR notify in a response fails the whole request, while
// an unrecognized STATUS notify is ignored.
// RFC requirement: RFC7296-3.10.1-1 positive -- an error type in the 0 to 16383 range
// that ze does not recognize, arriving in an IKE_AUTH response, ends the exchange. RFC
// 7296 Section 3.10.1 MUST: the receiver "MUST assume that the corresponding request
// has failed entirely", so continuing into AUTH verification would be wrong.
// RFC requirement: RFC7296-3.10.1-1 negative -- an unrecognized STATUS type at or above
// 16384, in the same position in the same response, is ignored and the SA establishes.
// So the failure is caused by the error range and not by the type being unknown.
// RFC requirement: RFC7296-3.10.1-2 positive -- that ignored status type reaches
// logIgnoredNotifies. RFC 7296 Section 3.10.1 asks for both halves:
// "MUST be ignored", and a log of each one at SHOULD level.
// The exchange completes around it.
// RFC requirement: RFC7296-3.10.1-2 negative -- an unrecognized error type in a
// REQUEST is likewise ignored rather than fatal, so the fail-entirely rule is scoped to
// responses as the RFC scopes it.
func TestErrUnrecognizedNotifyHandling(t *testing.T) {
	log := slogutil.DiscardLogger()

	t.Run("an unrecognized error type in a response fails the request", func(t *testing.T) {
		if state := errDriveAuthResponseWithNotify(t, errUnknownNotifyType); state != StateDead {
			t.Errorf("state = %v, want dead: an unrecognized error notify did not fail the request", state)
		}
	})

	t.Run("an unrecognized status type in a response is ignored", func(t *testing.T) {
		if state := errDriveAuthResponseWithNotify(t, errUnknownStatusType); state != StateEstablished {
			t.Errorf("state = %v, want established: an unrecognized STATUS notify failed the request", state)
		}
	})

	t.Run("the helper classifies both halves", func(t *testing.T) {
		errNotify := []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: errUnknownNotifyType}}}
		if got, ok := unrecognizedErrorNotify(errNotify); !ok || got != errUnknownNotifyType {
			t.Errorf("unrecognizedErrorNotify = %d ok=%v, want %d true", got, ok, errUnknownNotifyType)
		}
		statusNotify := []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: errUnknownStatusType}}}
		if got, ok := unrecognizedErrorNotify(statusNotify); ok {
			t.Errorf("an unrecognized STATUS type reported as a fatal error notify: %d", got)
		}
		known := []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyNoProposalChosen}}}
		if got, ok := unrecognizedErrorNotify(known); ok {
			t.Errorf("a RECOGNIZED error type reported as unrecognized: %d", got)
		}
		if _, ok := unrecognizedErrorNotify(nil); ok {
			t.Error("an empty chain reported an unrecognized notify")
		}
		if err := failIfUnrecognizedErrorNotify(nil, "peer", log); err != nil {
			t.Errorf("an empty chain failed the request: %v", err)
		}
	})

	t.Run("an unrecognized error type in a REQUEST is ignored", func(t *testing.T) {
		link := errLink(t)
		ini := link.ini
		resp := link.resp
		ps := link.ps
		peerTr := link.peerTr
		myTr := link.myTr
		inner := []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: errUnknownNotifyType}}}
		msgID := resp.ExpectedMsgID
		req, err := buildEncryptedMessageEx(ini, inner, msgID, wire.ExchangeInformational, initiatorFlag(ini))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		before := resp.State
		out := ps.handleOwnedInbound(resp, transport.Packet{Data: req}, myTr, nil, log)
		if !out.peerAlive {
			t.Fatal("the request never reached the handler")
		}
		if resp.State != before {
			t.Errorf("state = %v, want the unchanged %v: an unrecognized error type in a "+
				"REQUEST must be ignored, not fatal", resp.State, before)
		}
		if rtxRecv(t, peerTr) == nil {
			t.Error("the INFORMATIONAL request carrying an ignored notify drew no response")
		}
	})
}

// VALIDATES: the pre-adoption responder window does not replay its cached IKE_AUTH
// response to an unauthenticated datagram, and the replay it does allow is bounded.
// rfc-test-change-approved: retagged from RFC7296-2.21.4-5, which governs a peer
// receiving an unprotected NOTIFY payload. This test forges an IKE_AUTH request and
// sends no Notify, so the ledger was crediting a Notify obligation with proof from a
// request test. RFC7296-2.4-12 is what these assertions exercise, and the quotation
// below is corrected to the clause that actually applies. Ratchet-safe: 2.21.4-5 keeps
// both polarities at both tiers from notify_error_test.go and
// test/ipsec/ipsec-error-notify-no-loop.ci. Owner approved 2026-08-14.
// RFC requirement: RFC7296-2.4-12 positive -- handleResponderInbound's established
// arm sends sa.lastResponse to pkt.RemoteAddr, the OBSERVED source, so an attacker
// chooses the destination. It ran on an outer-header Message ID nobody authenticated,
// which made a 28-byte forgery draw a several-hundred-octet IKE_AUTH response at a
// chosen target. The Encrypted-payload guard refuses the forgery, and the per-SA token
// bucket bounds what survives it. RFC 7296 Section 2.4 requires the bound: "a node
// needs to limit the rate at which it will take actions based on unprotected messages".
// RFC requirement: RFC7296-2.4-12 negative -- a genuine retransmission that carries
// an Encrypted payload IS still answered from the cache, so the guard separates a
// forgery from a retransmission rather than disabling the window.
func TestErrResponderWindowDoesNotReflectToObservedSource(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ini := link.ini
	resp := link.resp
	ps := link.ps
	peerTr := link.peerTr
	myTr := link.myTr
	remote := link.remote
	if !resp.lastResponseSet {
		t.Fatal("the responder cached no IKE_AUTH response, so there is nothing to reflect")
	}
	resp.State = StateEstablished

	// The forgery: a bare header at the cached Message ID, from an address the
	// attacker chose. It carries no Encrypted payload.
	forged := ntfRequest(resp.InitiatorSPI, resp.ResponderSPI, wire.ExchangeIKEAuth, resp.lastResponseID, false)
	before := errorNotifySuppressedCount("unprotected-retransmit")
	ps.handleResponderInbound(resp, parseMsg(t, forged), transport.Packet{Data: forged, RemoteAddr: remote}, myTr, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "after an unprotected forgery in the responder window")
	if errorNotifySuppressedCount("unprotected-retransmit") <= before {
		t.Error("the Encrypted-payload guard recorded nothing, so the drop has another cause")
	}

	// Negative. A genuine retransmission carrying an Encrypted payload is answered.
	real := rtxIKEDelete(t, ini, resp.lastResponseID)
	ps.handleResponderInbound(resp, parseMsg(t, real), transport.Packet{Data: real, RemoteAddr: remote}, myTr, log)
	replay := rtxRecv(t, peerTr)
	if replay == nil {
		t.Fatal("a protected retransmission drew nothing, so the guard broke the window")
	}
	if !bytes.Equal(replay, resp.lastResponse) {
		t.Error("the replay does not match the cached response byte for byte")
	}

	// The bucket bounds the replay even for protected datagrams, because nothing here
	// has verified the integrity check yet.
	sent := 0
	for range cachedReplayBurst + 20 {
		ps.handleResponderInbound(resp, parseMsg(t, real), transport.Packet{Data: real, RemoteAddr: remote}, myTr, log)
	}
	if err := myTr.Send(rtxSentinel, remote); err != nil {
		t.Fatalf("send sentinel: %v", err)
	}
	for {
		got := rtxRecv(t, peerTr)
		if got == nil {
			t.Fatal("the sentinel never arrived")
		}
		if bytes.Equal(got, rtxSentinel) {
			break
		}
		sent++
	}
	if sent > cachedReplayBurst+1 {
		t.Errorf("the window replayed %d cached responses for a burst of %d", sent, cachedReplayBurst)
	}
}

// VALIDATES: a peer-initiated IKE SA rekey the responder cannot satisfy draws an error
// notify, and the notify names the right failure.
// RFC requirement: RFC7296-2.21.3-1 positive -- the IKE-rekey arm of
// handleCreateChildSAOwned answered every failure with silence except the KE-group
// mismatch, which respondIKERekey handles itself. A request carrying KEi but no Ni is
// malformed, so it draws INVALID_SYNTAX, which RFC 7296 Section 3.10.1 assigns to a
// message where "some type, length, or value was out of range".
// RFC requirement: RFC7296-2.21.3-1 negative -- the notify type is chosen from the
// failure rather than fixed: a malformed request draws INVALID_SYNTAX while a
// well-formed request ze cannot satisfy draws NO_PROPOSAL_CHOSEN. A single constant for
// both would leak which of the two happened, which Section 3.10.1 asks implementations
// to avoid.
func TestErrRefusedIKERekeyIsAnswered(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ini := link.ini
	resp := link.resp
	peerTr := link.peerTr
	myTr := link.myTr
	ps := &PeerSession{peerName: "err-ike-rekey", espGroup: testESPGroup()}

	// A CREATE_CHILD_SA carrying a KE payload and no Nonce: an IKE SA rekey request
	// that is missing a mandatory payload.
	inner := []wire.PayloadEntry{{Payload: &wire.PayloadKE{
		DHGroup:         14,
		KeyExchangeData: make([]byte, 256),
	}}}
	msg := &wire.Message{Header: wire.Header{MessageID: resp.ExpectedMsgID}}
	out := ps.handleCreateChildSAOwned(resp, msg, inner, false, myTr, nil, log)
	if out.newSA != nil {
		t.Fatal("a malformed IKE rekey request produced a replacement SA")
	}

	got := rtxRecv(t, peerTr)
	if got == nil {
		t.Fatal("a refused IKE SA rekey drew no answer")
	}
	types := errNotifyIn(t, ini, got)
	if len(types) != 1 || types[0] != wire.NotifyInvalidSyntax {
		t.Errorf("the answer carries notifies %v, want exactly INVALID_SYNTAX", types)
	}
	// rfc-test-change-approved: 2026-08-01 Thomas approved splitting this assertion by
	// notify type, after the RFC text was put beside it. It asserted StateEstablished for
	// BOTH failure classes, which is right for one and forbidden for the other.
	//
	// RFC 7296 Section 2.21.3 (rfc/full/rfc7296.txt:3339-3345): "If a peer parsing a
	// request notices that it is badly formatted (after it has passed the message
	// authentication code checks and window checks) and it returns an INVALID_SYNTAX
	// notification, then this error notification is considered fatal in both peers,
	// meaning that the IKE SA is deleted without needing an explicit Delete payload."
	// Section 2.21.2 (:3239-3242) lists INVALID_SYNTAX among the errors that "lead to a
	// deletion of the IKE SA without requiring an explicit INFORMATIONAL exchange".
	//
	// This request passed the MAC and window checks and drew INVALID_SYNTAX, so the SA
	// MUST be gone. Ze kept it and stayed half-open against a peer that had already
	// discarded it, until DPD noticed.
	//
	// The blanket assertion is not merely relaxed. NO_PROPOSAL_CHOSEN keeps the SA up,
	// and that half is pinned below, so the test now discriminates the two classes the
	// tagged requirement is actually about instead of expecting one answer for both.
	if resp.State != StateDead {
		t.Errorf("the IKE SA is %v after INVALID_SYNTAX, want StateDead: "+
			"RFC 7296 Section 2.21.3 makes that notification fatal in both peers", resp.State)
	}

	// Negative. notifyForRefusal separates the two failure classes rather than
	// answering everything with one constant.
	if got := notifyForRefusal(errMalformedRequest); got != wire.NotifyInvalidSyntax {
		t.Errorf("a malformed request maps to notify %d, want INVALID_SYNTAX", got)
	}
	if got := notifyForRefusal(crypto.ErrNoProposalChosen); got != wire.NotifyNoProposalChosen {
		t.Errorf("an unmatched proposal maps to notify %d, want NO_PROPOSAL_CHOSEN", got)
	}
}
