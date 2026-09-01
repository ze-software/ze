// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework (RFC 3748)
// RFC: rfc/short/rfc3748.md -- Sections 4 and 4.2: the packets that are discarded
//
// Four obligations share one subject: a packet the RFC says a role MUST NOT read.
// Three of them protect the EAP peer from an authenticator that answers "Success"
// before it has authenticated anything, and the fourth protects both roles from a
// Code the protocol never defined.
//
// A discard is not an error, and every test here asserts BOTH halves of that: the
// packet changes nothing, AND the session keeps working. The caller is why. The
// IKE engine reads PeerResult.Err first and puts the SA in StateDead for any
// non-nil value (handleEAPResponse, internal/component/ike/engine/fsm.go), so a
// "discard" that reported an error would let one forged packet end an exchange.
//
// VALIDATES: a Success is answered only after the method conversation concluded;
// a Failure is dropped once both ends indicated success; a Code outside 1-4 is
// dropped by the peer and by the authenticator; and each session continues after
// the drop.
// PREVENTS: a peer that returns Done and its MSK for a canned Success, which is
// the whole of a rogue authenticator's work; a peer that ends the exchange on a
// forged EAP-Failure it has already earned the right to ignore; an authenticator
// that answers an undefined Code with an EAP-Failure; and a discard implemented
// as an error, which trades a bypass for a denial of service.

package eap

import (
	"errors"
	"testing"
)

// eapdWantDiscarded fails unless the result is the empty PeerResult that a silent
// discard produces: no error for the engine to kill the SA on, no completion, no
// packet on the wire, and no key.
func eapdWantDiscarded(t *testing.T, res PeerResult, what string) {
	t.Helper()

	if res.Err != nil {
		t.Fatalf("%s: the peer reported %v; a silent discard reports nothing, and the IKE engine kills the SA for any error", what, res.Err)
	}
	if res.Done {
		t.Fatalf("%s: the peer reported Done", what)
	}
	if res.Response != nil {
		t.Fatalf("%s: the peer answered with code %d type %d; a silent discard sends nothing", what, res.Response.Code, res.Response.Type)
	}
	if res.MSK != ([64]byte{}) {
		t.Fatalf("%s: the peer handed out an MSK", what)
	}
	if !res.Discarded {
		t.Fatalf("%s: the peer reported no outcome at all; a drop says it dropped, so the caller can tell it from a branch nobody wrote", what)
	}
}

// eapdIdentityRequest is the EAP-Request/Identity an authenticator opens with.
func eapdIdentityRequest(id uint8) *Packet {
	return &Packet{Code: CodeRequest, Identifier: id, Type: TypeIdentity}
}

// eapdPeerAtMethodDone drives a real EAP-MSCHAPv2 conversation to the point where
// the peer has verified the Authenticator Response and acknowledged it. Both ends
// have now indicated success, which is the state RFC 3748 Section 4.2 makes the
// EAP-Success permitted in and the EAP-Failure forbidden in.
func eapdPeerAtMethodDone(t *testing.T) *PeerSession {
	t.Helper()

	peer, success := mschapv2PeerAtSuccess(t)
	res := peer.Process(success)
	if res.Err != nil {
		t.Fatalf("the peer refused a valid MS-CHAPv2 Success: %v", res.Err)
	}
	if res.Response == nil || res.Response.TypeData[0] != mschapv2OpSuccess {
		t.Fatal("the peer sent no MS-CHAPv2 Success acknowledgement, so it never indicated success back")
	}
	return peer
}

// TestRFC3748PeerDiscardsACannedSuccess sends the Success packet a rogue
// authenticator sends first: before the Identity Request, before any method, and
// with nothing authenticated.
func TestRFC3748PeerDiscardsACannedSuccess(t *testing.T) {
	// RFC requirement: RFC3748-4.2-7 positive -- RFC 3748 Section 4.2: "By
	// default, an EAP peer MUST silently discard a "canned" Success packet (a
	// Success packet sent immediately upon connection)." PeerSession.Process
	// drops it: the result is empty, the session does not report success, and the
	// peer goes on to answer the Identity Request that follows, which is what
	// separates a discard from an ended session.
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	res := peer.Process(&Packet{Code: CodeSuccess, Identifier: 1})
	eapdWantDiscarded(t, res, "a canned EAP-Success on a fresh session")
	if peer.Succeeded() {
		t.Fatal("the peer reports success after a canned EAP-Success, so a rogue authenticator bypassed the method")
	}

	next := peer.Process(eapdIdentityRequest(2))
	if next.Err != nil {
		t.Fatalf("the peer refused the Identity Request after the discard: %v", next.Err)
	}
	if next.Response == nil || next.Response.Type != TypeIdentity {
		t.Fatal("the peer did not answer the Identity Request after the discard, so the canned Success ended the conversation")
	}

	// RFC requirement: RFC3748-4.2-7 negative -- the same packet is acted on once
	// the method conversation has concluded. The peer reports Done and hands out
	// the MSK its method derived, so the discard above is a property of WHEN the
	// Success arrived and not a refusal of every Success.
	concluded := eapdPeerAtMethodDone(t)
	done := concluded.Process(&Packet{Code: CodeSuccess, Identifier: 5})
	if done.Err != nil {
		t.Fatalf("the peer refused the EAP-Success that ends a completed conversation: %v", done.Err)
	}
	if !done.Done {
		t.Fatal("the peer did not conclude on the EAP-Success that ends a completed conversation")
	}
	if done.MSK == ([64]byte{}) {
		t.Fatal("the peer concluded with an all-zero MSK")
	}
	if !concluded.Succeeded() {
		t.Fatal("the peer does not report success after a completed conversation")
	}
}

// TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet sends the Success in
// the middle of the method, after the MS-CHAPv2 Challenge round.
//
// That round is the sharp case. It derives the MSK from the peer's own password
// (handleMSCHAPv2Challenge), so ps.msk is already a real 64-octet key while the
// authenticator has proved nothing: the Authenticator Response that shows it
// knows the password has not arrived. A peer that answered here would hand the
// IKEv2 AUTH payload a key derived from an unauthenticated conversation.
func TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet(t *testing.T) {
	// RFC requirement: RFC3748-4.2-8 positive -- RFC 3748 Section 4.2: "A peer EAP
	// implementation receiving a Success or Failure packet where sending one is
	// not explicitly permitted MUST silently discard it." MS-CHAPv2 does not
	// permit the method to finish before the Authenticator Response, so
	// PeerSession.Process drops the Success: empty result, no MSK, and no success
	// reported.
	peer, success := mschapv2PeerAtSuccess(t)
	res := peer.Process(&Packet{Code: CodeSuccess, Identifier: 4})
	eapdWantDiscarded(t, res, "an EAP-Success in the middle of the MS-CHAPv2 method")
	if peer.Succeeded() {
		t.Fatal("the peer reports success while the Authenticator Response is still outstanding")
	}

	// RFC requirement: RFC3748-4.2-8 negative -- the method conversation is still
	// live after the discard, so the Authenticator Response is verified as usual
	// and the EAP-Success that follows IS permitted. The peer concludes with the
	// MSK, which shows the guard reads the point the method reached rather than
	// refusing every Success.
	ack := peer.Process(success)
	if ack.Err != nil {
		t.Fatalf("the peer refused a valid MS-CHAPv2 Success after the discard: %v", ack.Err)
	}
	if ack.Response == nil {
		t.Fatal("the peer sent no MS-CHAPv2 Success acknowledgement after the discard")
	}

	done := peer.Process(&Packet{Code: CodeSuccess, Identifier: 5})
	if done.Err != nil {
		t.Fatalf("the peer refused the permitted EAP-Success: %v", done.Err)
	}
	if !done.Done {
		t.Fatal("the peer did not conclude on the permitted EAP-Success")
	}
	if done.MSK == ([64]byte{}) {
		t.Fatal("the peer concluded with an all-zero MSK")
	}
}

// TestRFC3748PeerDiscardsAFailureAfterMutualSuccess sends the EAP-Failure that
// contradicts an authentication both ends already agreed on.
//
// No path through ze's own authenticator produces that packet: once the method
// succeeds it sends EAP-Success (Session.handleMethod). The packet is
// unauthenticated, so any party on the path can forge one, and this is the guard
// that makes forging it useless.
func TestRFC3748PeerDiscardsAFailureAfterMutualSuccess(t *testing.T) {
	// RFC requirement: RFC3748-4.2-9 positive -- RFC 3748 Section 4.2: "On the
	// peer, after success result indications have been exchanged by both sides, a
	// Failure packet MUST be silently discarded." The authenticator indicated
	// success with its MS-CHAPv2 Success packet and the peer acknowledged it, so
	// PeerSession.Process drops the EAP-Failure and the EAP-Success that follows
	// still concludes the exchange.
	peer := eapdPeerAtMethodDone(t)
	res := peer.Process(&Packet{Code: CodeFailure, Identifier: 5})
	eapdWantDiscarded(t, res, "an EAP-Failure after both ends indicated success")

	done := peer.Process(&Packet{Code: CodeSuccess, Identifier: 6})
	if done.Err != nil {
		t.Fatalf("the discarded EAP-Failure poisoned the session: %v", done.Err)
	}
	if !done.Done {
		t.Fatal("the peer did not conclude on the EAP-Success that followed the discarded EAP-Failure")
	}
	if done.MSK == ([64]byte{}) {
		t.Fatal("the peer concluded with an all-zero MSK")
	}

	// RFC requirement: RFC3748-4.2-9 negative -- before those indications are
	// exchanged, an EAP-Failure is read rather than dropped. The authenticator is
	// entitled to refuse at that point, so the peer reports ErrEAPFailure and the
	// IKE engine ends the SA on it.
	early := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	if res := early.Process(&Packet{Code: CodeFailure, Identifier: 1}); !errors.Is(res.Err, ErrEAPFailure) {
		t.Fatalf("an EAP-Failure before the method concluded gave %v, want ErrEAPFailure", res.Err)
	}

	midway, _ := mschapv2PeerAtSuccess(t)
	mid := midway.Process(&Packet{Code: CodeFailure, Identifier: 4})
	if !errors.Is(mid.Err, ErrEAPFailure) {
		t.Fatalf("an EAP-Failure in the middle of the method gave %v, want ErrEAPFailure", mid.Err)
	}
	if mid.Done {
		t.Fatal("the peer reported Done on an EAP-Failure")
	}
}

// TestRFC3748UndefinedCodesAreSilentlyDiscarded drives both roles ze plays with
// the Codes the protocol never defined.
func TestRFC3748UndefinedCodesAreSilentlyDiscarded(t *testing.T) {
	undefined := []uint8{0, 5, 6, 255}

	// RFC requirement: RFC3748-4-5 positive -- RFC 3748 Section 4: "Since EAP only
	// defines Codes 1-4, EAP packets with other codes MUST be silently discarded
	// by both authenticators and peers." The authenticator answers each with no
	// packet at all (Session.Process returns nil), and it is still at the identity
	// round afterwards: the Identity Response that follows opens the method as if
	// the discarded packets had never arrived.
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)
	auth.Begin()

	for _, code := range undefined {
		if out := auth.Process(&Packet{Code: code, Identifier: 1, Type: TypeIdentity, TypeData: []byte("user")}); out != nil {
			t.Fatalf("the authenticator answered Code %d with code %d; an undefined Code is discarded", code, out.Code)
		}
	}
	opened := auth.Process(&Packet{Code: CodeResponse, Identifier: 1, Type: TypeIdentity, TypeData: []byte("user")})
	if opened == nil {
		t.Fatal("the authenticator answered the Identity Response with nothing, so the discarded Codes ended the exchange")
	}
	if opened.Code != CodeRequest || opened.Type != TypeMSCHAPv2 {
		t.Fatalf("the authenticator answered the Identity Response with code %d type %d, want an MS-CHAPv2 Request", opened.Code, opened.Type)
	}

	// The peer half of the same sentence: each undefined Code is dropped, and the
	// peer still answers the Identity Request that follows.
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	for _, code := range undefined {
		eapdWantDiscarded(t, peer.Process(&Packet{Code: code, Identifier: 1}), "a packet with undefined Code")
	}
	answered := peer.Process(eapdIdentityRequest(2))
	if answered.Err != nil {
		t.Fatalf("the peer refused the Identity Request after the discards: %v", answered.Err)
	}
	if answered.Response == nil || answered.Response.Type != TypeIdentity {
		t.Fatal("the peer did not answer the Identity Request after the discards")
	}

	// RFC requirement: RFC3748-4-5 negative -- the four Codes EAP DOES define are
	// read rather than discarded, Code 4 included, which is the value next to the
	// first undefined one. The authenticator answers a Request, a Success and a
	// Failure with an EAP-Failure, because none of the three is a Response, and it
	// answers a Response with the next Request.
	for _, code := range []uint8{CodeRequest, CodeSuccess, CodeFailure} {
		defined, dErr := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
		if dErr != nil {
			t.Fatalf("NewSession: %v", dErr)
		}
		t.Cleanup(defined.Close)
		defined.Begin()

		out := defined.Process(&Packet{Code: code, Identifier: 1, Type: TypeIdentity})
		if out == nil {
			t.Fatalf("the authenticator discarded Code %d, which EAP defines", code)
		}
		if out.Code != CodeFailure {
			t.Fatalf("the authenticator answered Code %d with code %d, want %d (EAP-Failure)", code, out.Code, CodeFailure)
		}
	}

	// And the peer reads all three of the Codes addressed to it. The Request opens
	// the identity round, the Failure ends the conversation, and the Success is
	// read by the guard the tests above pin.
	reader := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	if res := reader.Process(eapdIdentityRequest(1)); res.Response == nil || res.Response.Code != CodeResponse {
		t.Fatal("the peer discarded an EAP-Request, which EAP defines")
	}
	if res := reader.Process(&Packet{Code: CodeFailure, Identifier: 2}); !errors.Is(res.Err, ErrEAPFailure) {
		t.Fatalf("the peer answered an EAP-Failure with %v, want ErrEAPFailure", res.Err)
	}
}

// RFC requirement: RFC3748-4.1-11 positive -- the authenticator silently discards
// a Response whose Type is neither the outstanding Request's nor a legacy Nak.
//
// RFC 3748 Section 4.1: "An EAP server receiving a Response not meeting these
// requirements MUST silently discard it."
func TestAuthenticatorDiscardsAResponseOfAnotherType(t *testing.T) {
	s, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "p"})
	if err != nil {
		t.Fatal(err)
	}
	s.Begin()
	if out := s.Process(&Packet{Code: CodeResponse, Identifier: 1, Type: TypeIdentity, TypeData: []byte("u")}); out == nil {
		t.Fatal("the identity response must draw the method's first request")
	}
	s.identifier = 2

	out := s.Process(&Packet{Code: CodeResponse, Identifier: 2, Type: TypeTLS, TypeData: []byte{0x20}})

	if out != nil {
		t.Fatalf("a Response of another Type drew %v, want a silent discard", out)
	}
	if s.Succeeded() {
		t.Fatal("the discarded Response completed the exchange")
	}
}

// RFC requirement: RFC3748-4.1-11 negative -- a Response carrying the method's
// own Type IS processed, so the discard above is the Type check acting rather
// than the authenticator refusing every Response.
func TestAuthenticatorProcessesAResponseOfTheMethodType(t *testing.T) {
	s := &Session{method: &doneMethod{}, state: stateMethod, identifier: 4}

	out := s.Process(&Packet{Code: CodeResponse, Identifier: 4, Type: TypeMSCHAPv2})

	if out == nil || out.Code != CodeSuccess {
		t.Fatalf("a Response of the method's Type must be processed, got %v", out)
	}
}

// RFC requirement: RFC3748-2.1-4 positive -- the peer silently discards a Request
// of a Type other than the one under way, answering with no packet and no error.
//
// RFC 3748 Section 2.1: "Once a peer has sent a Response of the same Type as the
// initial Request, an authenticator MUST NOT send a Request of a different Type
// prior to completion of the final round of a given method."
//
// The error this replaced was read by handleEAPResponse
// (internal/component/ike/engine/fsm.go) as a reason to set StateDead, so one
// unauthenticated packet ended the exchange.
func TestPeerDiscardsARequestOfAnotherType(t *testing.T) {
	ps := NewPeerSession(TypeMSCHAPv2, "u", "p")

	// The method must be UNDER WAY first. Section 2.1 binds from the moment the
	// peer has answered the initial Request, so an unknown Type arriving in the
	// identity state is a different obligation: RFC 3748 Section 5.3.1 owes it a
	// legacy Nak, which plan/spec-eap-notification-and-nak.md owns.
	if got := ps.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}); got.Response == nil {
		t.Fatal("the identity request must draw an identity response")
	}

	res := ps.Process(&Packet{Code: CodeRequest, Identifier: 2, Type: TypeTLS, TypeData: []byte{0x20}})

	if !res.Discarded {
		t.Fatal("a Request of another Type must be discarded")
	}
	if res.Err != nil {
		t.Fatalf("the discard must carry no error, or the packet ends the SA: %v", res.Err)
	}
	if res.Response != nil {
		t.Fatalf("the discard must draw no response, got type %d", res.Response.Type)
	}
}
