// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework (RFC 3748)
// RFC: rfc/short/rfc3748.md -- Sections 2.1, 4 and 4.2: the clauses the older units left unasserted
// Related: rfc3748_test.go -- the first units for the same rows
// Related: rfc3748_discard_test.go -- the Section 4.2 discards and their helpers
//
// Each test here proves one quoted sentence clause by clause, in both
// polarities, through the real Session and PeerSession entry points. The older
// units for these rows each proved one clause of a sentence that carries
// several: the authenticator side of a rule that binds the peer too, the
// Success half of a Success-or-Failure rule, or a Length rule read only on
// decode.
//
// VALIDATES: one method per conversation on both roles; no Request of another
// Type and no second method from the authenticator; the result packet after the
// one method; an unpermitted Failure discarded by the peer; the peer ending a
// conversation it decided to leave; Success and Failure carrying no data; the
// Length field counting the header; octets past Length ignored.
// PREVENTS: an authenticator that switches method or answers a provocation with
// a Request of the provoked Type; a peer that follows a second method; a peer
// that ends the exchange on a forged Failure; a peer that carries on after it
// refused the authenticator; data smuggled in a Success; an encoder writing only
// the data length; a decoder reading link-layer padding as Type-Data.

package eap

import (
	"bytes"
	"errors"
	"testing"
)

// clauseFlight is one full authenticator-to-peer conversation: every Request
// the authenticator sent after the identity round, and the terminal packet.
type clauseFlight struct {
	auth     *Session
	requests []*Packet
	last     *Packet
}

// driveClauseFlight runs EAP-MSCHAPv2 between the real Session and the real
// PeerSession and records every Request the authenticator emits, so a test can
// read the Type of each one rather than only the final packet.
func driveClauseFlight(t *testing.T, authPassword, peerPassword string) clauseFlight {
	t.Helper()

	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: authPassword})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)
	peer := NewPeerSession(TypeMSCHAPv2, "user", peerPassword)

	flight := clauseFlight{auth: auth}
	req := auth.Begin()
	for round := range 8 {
		pr := peer.Process(req)
		if pr.Err != nil {
			t.Fatalf("peer round %d: %v", round, pr.Err)
		}
		if pr.Response == nil {
			t.Fatalf("peer round %d: no response", round)
		}
		next := auth.Process(pr.Response)
		if next == nil {
			t.Fatalf("authenticator round %d: no packet before the result", round)
		}
		if next.Code != CodeRequest {
			flight.last = next
			return flight
		}
		flight.requests = append(flight.requests, next)
		req = next
	}
	t.Fatal("the EAP-MSCHAPv2 exchange did not end within 8 rounds")
	return flight
}

// wantOneMethodRequests fails unless every Request after the identity round
// carries the method Type. RFC 3748 Section 2.1 excepts a Notification-Request,
// and ze's authenticator sends none, so any other Type is a second method.
func wantOneMethodRequests(t *testing.T, requests []*Packet) {
	t.Helper()

	if len(requests) == 0 {
		t.Fatal("the authenticator sent no method Request")
	}
	for i, req := range requests {
		if req.Type != TypeMSCHAPv2 {
			t.Fatalf("method Request %d carries Type %d, want the method Type %d", i, req.Type, TypeMSCHAPv2)
		}
	}
}

// TestRFC3748NoNewRequestBeforeAValidResponse holds the authenticator at its
// outstanding method Request and feeds it Responses that are not valid for it.
func TestRFC3748NoNewRequestBeforeAValidResponse(t *testing.T) {
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	identity := peer.Process(auth.Begin())
	challenge := auth.Process(identity.Response)
	if challenge == nil || challenge.Code != CodeRequest {
		t.Fatalf("the identity round drew %v, want the method Request", challenge)
	}

	// RFC requirement: RFC3748-2-2 negative -- RFC 3748 Section 2: "other than the
	// initial Request, a new Request cannot be sent prior to receiving a valid
	// Response." While the Challenge is outstanding, a Response carrying another
	// Identifier and a Response carrying another Type are not valid for it, and
	// neither draws a packet.
	stale := &Packet{Code: CodeResponse, Identifier: challenge.Identifier + 1, Type: TypeMSCHAPv2, TypeData: []byte{0x02}}
	if out := auth.Process(stale); out != nil {
		t.Fatalf("a Response with another Identifier drew code %d type %d", out.Code, out.Type)
	}
	other := &Packet{Code: CodeResponse, Identifier: challenge.Identifier, Type: TypeTLS, TypeData: []byte{0x20}}
	if out := auth.Process(other); out != nil {
		t.Fatalf("a Response of another Type drew code %d type %d", out.Code, out.Type)
	}

	// RFC requirement: RFC3748-2-1 positive -- RFC 3748 Section 2: "EAP is a 'lock
	// step' protocol, so that other than the initial Request, a new Request
	// cannot be sent prior to receiving a valid Response." After the invalid
	// Responses the Challenge still stands: the peer's valid Response to it is
	// what draws the next packet.
	// RFC requirement: RFC3748-2-2 positive -- the valid Response draws the next
	// packet, and only one.
	answer := peer.Process(challenge)
	next := auth.Process(answer.Response)
	if next == nil {
		t.Fatal("the valid Response drew no packet")
	}
	if next.Code == CodeRequest && next.Identifier == challenge.Identifier {
		t.Fatalf("the next Request reuses Identifier %d, so it is not a new Request", next.Identifier)
	}
	if out := auth.Process(answer.Response); out != nil {
		t.Fatalf("the same Response a second time drew code %d; one valid Response draws one packet", out.Code)
	}
}

// TestRFC3748TheAuthenticatorSendsNoRequestOfAnotherType drives the whole
// method and then provokes the authenticator with a Response of another Type,
// mid-method and after completion.
func TestRFC3748TheAuthenticatorSendsNoRequestOfAnotherType(t *testing.T) {
	// RFC requirement: RFC3748-2.1-4 positive -- RFC 3748 Section 2.1: "Once a peer
	// has sent a Response of the same Type as the initial Request, an
	// authenticator MUST NOT send a Request of a different Type prior to
	// completion of the final round of a given method (with the exception of a
	// Notification-Request) and MUST NOT send a Request for an additional method
	// of any Type after completion of the initial authentication method".
	// Every Request of a completed MS-CHAPv2 conversation carries Type 26, and a
	// Response of Type 13 after the EAP-Success draws no packet at all.
	flight := driveClauseFlight(t, "secret", "secret")
	wantOneMethodRequests(t, flight.requests)
	if flight.last.Code != CodeSuccess {
		t.Fatalf("the conversation ended with code %d, want EAP-Success", flight.last.Code)
	}
	after := flight.auth.Process(&Packet{Code: CodeResponse, Identifier: flight.last.Identifier, Type: TypeTLS, TypeData: []byte{0x20}})
	if after != nil {
		t.Fatalf("a Response after the completed method drew code %d type %d; no additional method may start", after.Code, after.Type)
	}

	// RFC requirement: RFC3748-2.1-4 negative -- a Response of Type 13 to the
	// outstanding MS-CHAPv2 Request is the provocation to switch method. The
	// authenticator sends nothing for it, no Request of Type 13 in particular,
	// and the method Request still stands: the peer answers it and the same
	// method completes.
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	identity := peer.Process(auth.Begin())
	challenge := auth.Process(identity.Response)
	if challenge == nil || challenge.Type != TypeMSCHAPv2 {
		t.Fatalf("the identity round drew %v, want an MS-CHAPv2 Request", challenge)
	}
	provoked := auth.Process(&Packet{Code: CodeResponse, Identifier: challenge.Identifier, Type: TypeTLS, TypeData: []byte{0x20}})
	if provoked != nil {
		t.Fatalf("a Response of another Type drew code %d type %d mid-method", provoked.Code, provoked.Type)
	}
	answer := peer.Process(challenge)
	next := auth.Process(answer.Response)
	if next == nil || next.Type != TypeMSCHAPv2 {
		t.Fatalf("the method did not continue after the provocation: %v", next)
	}
}

// TestRFC3748OneMethodThenTheResult reads the whole sentence of Section 2.1:
// one method, then Success or Failure, and nothing after it.
func TestRFC3748OneMethodThenTheResult(t *testing.T) {
	// RFC requirement: RFC3748-2.1-2 positive -- RFC 3748 Section 2.1: "However,
	// the peer and authenticator MUST utilize only one authentication method
	// (Type 4 or greater) within an EAP conversation, after which the
	// authenticator MUST send a Success or Failure packet." Every method Request
	// carries the one method Type, the conversation ends in EAP-Success, and a
	// Response of another method Type afterwards draws nothing.
	good := driveClauseFlight(t, "secret", "secret")
	wantOneMethodRequests(t, good.requests)
	if good.last.Code != CodeSuccess {
		t.Fatalf("a verified method ended with code %d, want EAP-Success", good.last.Code)
	}
	if out := good.auth.Process(&Packet{Code: CodeResponse, Identifier: good.last.Identifier, Type: TypeMD5Challenge}); out != nil {
		t.Fatalf("a second method's Response after EAP-Success drew code %d", out.Code)
	}

	// RFC requirement: RFC3748-2.1-2 negative -- a method the peer fails (wrong
	// password) still runs as the one method, and its result is EAP-Failure,
	// never EAP-Success and never a Request that starts another method.
	bad := driveClauseFlight(t, "right", "wrong")
	wantOneMethodRequests(t, bad.requests)
	if bad.last.Code != CodeFailure {
		t.Fatalf("a refused method ended with code %d, want EAP-Failure", bad.last.Code)
	}
	if bad.auth.Succeeded() {
		t.Fatal("the authenticator reports success after EAP-Failure")
	}
	if out := bad.auth.Process(&Packet{Code: CodeResponse, Identifier: bad.last.Identifier, Type: TypeMD5Challenge}); out != nil {
		t.Fatalf("a second method's Response after EAP-Failure drew code %d", out.Code)
	}
}

// TestRFC3748ThePeerKeepsToOneMethod proves the peer's half of Section 2.1,
// which the authenticator unit for the same row does not reach.
func TestRFC3748ThePeerKeepsToOneMethod(t *testing.T) {
	// RFC requirement: RFC3748-2.1-1 negative -- RFC 3748 Section 2.1: "the peer
	// and authenticator MUST utilize only one authentication method (Type 4 or
	// greater) within an EAP conversation". A peer committed to MS-CHAPv2 is
	// handed an EAP-TLS Request: it sends no Response, so it does not take up a
	// second method.
	peer, success := mschapv2PeerAtSuccess(t)
	second := peer.Process(&Packet{Code: CodeRequest, Identifier: 3, Type: TypeTLS, TypeData: []byte{0x20}})
	eapdWantDiscarded(t, second, "an EAP-TLS Request to a peer committed to MS-CHAPv2")

	// RFC requirement: RFC3748-2.1-1 positive -- the one method carries on: the
	// same peer verifies the MS-CHAPv2 Success, acknowledges it with Type 26,
	// and concludes with a key on the EAP-Success.
	ack := peer.Process(success)
	if ack.Err != nil {
		t.Fatalf("the peer refused the MS-CHAPv2 Success: %v", ack.Err)
	}
	if ack.Response == nil || ack.Response.Type != TypeMSCHAPv2 {
		t.Fatalf("the peer acknowledged with %v, want an MS-CHAPv2 Response", ack.Response)
	}
	done := peer.Process(&Packet{Code: CodeSuccess, Identifier: success.Identifier})
	if !done.Done {
		t.Fatalf("the peer did not conclude the one method: %+v", done)
	}
}

// TestRFC3748PeerDiscardsAFailureWhereNoneIsPermitted proves the Failure half
// of the Section 4.2 discard. The Success half is
// TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet.
func TestRFC3748PeerDiscardsAFailureWhereNoneIsPermitted(t *testing.T) {
	// RFC requirement: RFC3748-4.2-8 positive -- RFC 3748 Section 4.2: "A peer EAP
	// implementation receiving a Success or Failure packet where sending one is
	// not explicitly permitted MUST silently discard it." Once both ends indicated
	// success, Section 4.2 permits no Failure. The peer drops it: no error, no
	// Response, no key, and the EAP-Success that follows still concludes.
	peer := eapdPeerAtMethodDone(t)
	eapdWantDiscarded(t, peer.Process(&Packet{Code: CodeFailure, Identifier: 5}), "an EAP-Failure after both success indications")
	done := peer.Process(&Packet{Code: CodeSuccess, Identifier: 5})
	if !done.Done {
		t.Fatalf("the discarded EAP-Failure ended the session: %+v", done)
	}

	// RFC requirement: RFC3748-4.2-8 negative -- in the middle of the method the
	// authenticator is permitted to refuse, so the same EAP-Failure is read: the
	// peer reports ErrEAPFailure and does not conclude.
	midway, _ := mschapv2PeerAtSuccess(t)
	mid := midway.Process(&Packet{Code: CodeFailure, Identifier: 4})
	if !errors.Is(mid.Err, ErrEAPFailure) {
		t.Fatalf("a permitted EAP-Failure gave %v, want ErrEAPFailure", mid.Err)
	}
	if mid.Discarded {
		t.Fatal("the peer discarded a permitted EAP-Failure")
	}
}

// TestRFC3748PeerEndsAConversationItRefuses drives the second trigger of the
// Section 4.2 sentence: the peer, not the authenticator, decides to stop.
func TestRFC3748PeerEndsAConversationItRefuses(t *testing.T) {
	// RFC requirement: RFC3748-4.2-5 positive -- RFC 3748 Section 4.2: "On the
	// peer, once the method completes unsuccessfully (that is, either the
	// authenticator sends a failure result indication, or the peer decides that
	// it does not want to continue the conversation, possibly after sending a
	// failure result indication), the peer MUST terminate the conversation and
	// indicate failure to the lower layer." An MS-CHAPv2 Success whose
	// Authenticator Response is wrong is the peer's decision to stop: it reports
	// an error to the lower layer, sends no acknowledgement, and the EAP-Success
	// that follows concludes nothing.
	peer, _ := mschapv2PeerAtSuccess(t)
	forged := mschapv2Success(3, "S=0000000000000000000000000000000000000000 M=ok")
	refused := peer.Process(forged)
	if refused.Err == nil {
		t.Fatal("the peer accepted a wrong Authenticator Response")
	}
	if refused.Response != nil {
		t.Fatal("the peer acknowledged a wrong Authenticator Response")
	}
	after := peer.Process(&Packet{Code: CodeSuccess, Identifier: 3})
	if after.Done {
		t.Fatal("the peer concluded after it refused the authenticator")
	}
	if after.MSK != ([64]byte{}) {
		t.Fatal("the peer handed out an MSK after it refused the authenticator")
	}
	if peer.Succeeded() {
		t.Fatal("the peer reports success after it refused the authenticator")
	}

	// RFC requirement: RFC3748-4.2-5 negative -- the matching Authenticator
	// Response does not end the conversation: the peer acknowledges it and
	// concludes on the EAP-Success, so the refusal above reads the value.
	good, success := mschapv2PeerAtSuccess(t)
	ack := good.Process(success)
	if ack.Err != nil || ack.Response == nil {
		t.Fatalf("the peer refused a valid Authenticator Response: %+v", ack)
	}
	if done := good.Process(&Packet{Code: CodeSuccess, Identifier: success.Identifier}); !done.Done {
		t.Fatalf("the peer did not conclude a successful method: %+v", done)
	}
}

// TestRFC3748SuccessAndFailureCarryNoData hands the encoder a Success and a
// Failure that carry data, and the decoder a Success whose Length covers data.
func TestRFC3748SuccessAndFailureCarryNoData(t *testing.T) {
	for _, code := range []uint8{CodeSuccess, CodeFailure} {
		// RFC requirement: RFC3748-4.2-2 positive -- RFC 3748 Section 4.2: "Success
		// and Failure packets MUST NOT contain additional data." A Packet that
		// carries a Type and Type-Data still encodes to the 4-octet header with
		// Length 4.
		wire := (&Packet{Code: code, Identifier: 7, Type: TypeMSCHAPv2, TypeData: []byte{1, 2, 3}}).Encode()
		if !bytes.Equal(wire, []byte{code, 7, 0, 4}) {
			t.Fatalf("code %d with data encoded to % x, want %02x 07 00 04", code, wire, code)
		}

		// RFC requirement: RFC3748-4.2-2 negative -- a received Success or Failure
		// whose Length covers three data octets is refused its data: the decoder
		// delivers no Type and no Type-Data.
		dec, err := DecodePacket([]byte{code, 7, 0, 7, 0x1a, 0x02, 0x03})
		if err != nil {
			t.Fatalf("code %d with data: %v", code, err)
		}
		if dec.Type != 0 || dec.TypeData != nil {
			t.Fatalf("code %d delivered Type %d and data % x", code, dec.Type, dec.TypeData)
		}
	}
}

// TestRFC3748LengthCountsTheWholePacket reads the Length field Ze writes and
// feeds the decoder a Length that counts only the data.
func TestRFC3748LengthCountsTheWholePacket(t *testing.T) {
	// RFC requirement: RFC3748-4-2 positive -- RFC 3748 Section 4: "The Length field
	// is two octets and indicates the length, in octets, of the EAP packet
	// including the Code, Identifier, Length, and Data fields." A Request with a
	// Type and three data octets encodes to 8 octets and says 8, and the decoder
	// reads those 8 back as the whole packet.
	wire := (&Packet{Code: CodeRequest, Identifier: 9, Type: TypeMD5Challenge, TypeData: []byte{1, 2, 3}}).Encode()
	length := int(wire[2])<<8 | int(wire[3])
	if length != 8 || len(wire) != 8 {
		t.Fatalf("the Length field says %d over %d octets, want 8 over 8", length, len(wire))
	}
	dec, err := DecodePacket(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dec.Type != TypeMD5Challenge || !bytes.Equal(dec.TypeData, []byte{1, 2, 3}) {
		t.Fatalf("decoded Type %d data % x, want 4 and 01 02 03", dec.Type, dec.TypeData)
	}

	// RFC requirement: RFC3748-4-2 negative -- a Length that counts the Data field
	// only (4: the Type and three octets) or the Type-Data only (3) leaves out the
	// header, and the decoder refuses both.
	for _, dataOnly := range []byte{4, 3} {
		bad := bytes.Clone(wire)
		bad[3] = dataOnly
		if _, err := DecodePacket(bad); err == nil {
			t.Fatalf("a Length of %d over an 8-octet packet was accepted", dataOnly)
		}
	}
}

// TestRFC3748PaddingPastLengthIsNotData feeds the authenticator an Identity
// Response carrying link-layer padding.
func TestRFC3748PaddingPastLengthIsNotData(t *testing.T) {
	identity := (&Packet{Code: CodeResponse, Identifier: 1, Type: TypeIdentity, TypeData: []byte("user")}).Encode()
	padded := append(bytes.Clone(identity), 'X', 'Y', 'Z')

	// RFC requirement: RFC3748-4-4 positive -- RFC 3748 Section 4: "Octets outside
	// the range of the Length field should be treated as Data Link Layer padding
	// and MUST be ignored upon reception." The three octets past Length reach
	// neither the Type-Data nor the identity the authenticator records.
	dec, err := DecodePacket(padded)
	if err != nil {
		t.Fatalf("a padded Response was refused: %v", err)
	}
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)
	auth.Begin()
	if next := auth.Process(dec); next == nil || next.Type != TypeMSCHAPv2 {
		t.Fatalf("the padded Identity Response drew %v, want the method Request", next)
	}
	if auth.Identity() != "user" {
		t.Fatalf("the authenticator recorded identity %q, want %q", auth.Identity(), "user")
	}

	// RFC requirement: RFC3748-4-4 negative -- the same octets inside the Length
	// field are data, not padding: with Length raised to cover them the decoder
	// delivers them, so the positive reads the Length field and not the buffer.
	covered := bytes.Clone(padded)
	covered[3] = byte(len(covered))
	inside, err := DecodePacket(covered)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(inside.TypeData) != "userXYZ" {
		t.Fatalf("octets inside Length decoded to %q, want %q", inside.TypeData, "userXYZ")
	}
}

// TestRFC3748MD5ChallengeAnswersOnlyARequestThatCarriesOne hands the peer an
// MD5-Challenge Request with a challenge and one without.
func TestRFC3748MD5ChallengeAnswersOnlyARequestThatCarriesOne(t *testing.T) {
	// RFC requirement: RFC3748-5.4-1 positive -- RFC 3748 Section 5.4: "The Request
	// contains a "challenge" message to the peer.  A Response MUST be sent in
	// reply to the Request." The peer answers the authenticator's challenge with
	// a Type-4 Response carrying the Request's Identifier and the MD5 value
	// computed outside this package.
	_, challenge := md5Authenticator(t)
	peer := md5Peer(t, md5TestSecret)
	res := peer.Process(challenge)
	if res.Response == nil {
		t.Fatalf("the MD5-Challenge Request drew %+v, want a Response", res)
	}
	if res.Response.Code != CodeResponse || res.Response.Type != TypeMD5Challenge {
		t.Fatalf("the reply is Code %d Type %d, want a Type-4 Response", res.Response.Code, res.Response.Type)
	}
	if res.Response.Identifier != challenge.Identifier {
		t.Fatalf("the Response carries Identifier %d, want the Request's %d", res.Response.Identifier, challenge.Identifier)
	}
	valueSize := int(challenge.TypeData[0])
	want := md5ExpectedResponse(challenge.Identifier, md5TestSecret, challenge.TypeData[1:1+valueSize])
	if !bytes.Equal(res.Response.TypeData[1:17], want) {
		t.Fatalf("the Response Value is % x, want % x", res.Response.TypeData[1:17], want)
	}

	// RFC requirement: RFC3748-5.4-1 negative -- a Type-4 Request that carries no
	// challenge (Value-Size 0, or no Type-Data at all) is not the Request the
	// sentence describes, and the peer sends no Response for it.
	for _, empty := range [][]byte{{0}, nil} {
		bare := md5Peer(t, md5TestSecret)
		out := bare.Process(&Packet{Code: CodeRequest, Identifier: 2, Type: TypeMD5Challenge, TypeData: empty})
		if out.Response != nil {
			t.Fatalf("a Request with Type-Data % x drew a Response", empty)
		}
		if out.Err == nil {
			t.Fatalf("a Request with Type-Data % x drew no refusal", empty)
		}
	}
}

// TestRFC3748MD5ChallengeServerRefusesAWrongValue runs MD5-Challenge between
// both of ze's roles with one secret, then with two.
func TestRFC3748MD5ChallengeServerRefusesAWrongValue(t *testing.T) {
	// RFC requirement: RFC3748-5.4-2 positive -- RFC 3748 Section 5.4: "EAP peer and
	// EAP server implementations MUST support the MD5-Challenge mechanism." Ze's
	// peer answers ze's server, the server verifies the value and sends
	// EAP-Success, and the peer concludes on it.
	auth, challenge := md5Authenticator(t)
	peer := md5Peer(t, md5TestSecret)
	res := peer.Process(challenge)
	if res.Response == nil {
		t.Fatalf("the peer drew %+v, want a Response", res)
	}
	success := auth.Process(res.Response)
	if success == nil || success.Code != CodeSuccess {
		t.Fatalf("the server answered %v (%v), want EAP-Success", success, auth.Err())
	}
	if final := peer.Process(success); !final.Done {
		t.Fatalf("the peer did not conclude on the EAP-Success: %+v", final)
	}

	// RFC requirement: RFC3748-5.4-2 negative -- support means the server checks
	// the value: a peer holding another secret draws EAP-Failure, so a server
	// that answered every Type-4 Response with EAP-Success goes red here.
	wrongAuth, wrongChallenge := md5Authenticator(t)
	wrongPeer := md5Peer(t, "another secret")
	wrong := wrongPeer.Process(wrongChallenge)
	if wrong.Response == nil {
		t.Fatalf("the peer drew %+v, want a Response", wrong)
	}
	failure := wrongAuth.Process(wrong.Response)
	if failure == nil || failure.Code != CodeFailure {
		t.Fatalf("a wrong MD5 value drew %v, want EAP-Failure", failure)
	}
	if wrongAuth.Succeeded() {
		t.Fatal("the server recorded success for a wrong MD5 value")
	}
}

// runTamperedPeerResponse drives EAP-TLS and changes one octet in the second
// peer Response that carries TLS records, the peer's authenticated flight, on
// its way to the authenticator. It reports whether the authenticator sent
// EAP-Success.
func runTamperedPeerResponse(t *testing.T, serverCfg MethodConfig, peer *PeerSession) bool {
	t.Helper()

	sess, err := NewSession(TypeTLS, serverCfg)
	if err != nil {
		t.Fatalf("create authenticator session: %v", err)
	}
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	req := sess.Begin()
	recordResponses := 0
	tampered := false
	for range 60 {
		pres := peer.Process(req)
		if pres.Err != nil || pres.Done || pres.Response == nil {
			break
		}
		resp := pres.Response
		if !tampered && resp.Type == TypeTLS && len(resp.TypeData) > 16 {
			recordResponses++
			if recordResponses == 2 {
				bad := &Packet{Code: resp.Code, Identifier: resp.Identifier, Type: resp.Type, TypeData: bytes.Clone(resp.TypeData)}
				bad.TypeData[len(bad.TypeData)-1] ^= 0xFF
				resp = bad
				tampered = true
			}
		}
		next := sess.Process(resp)
		if next == nil {
			break
		}
		if next.Code == CodeSuccess {
			return true
		}
		if next.Code == CodeFailure {
			break
		}
		req = next
	}
	if !tampered {
		t.Fatal("no second peer Response carried a TLS record, so nothing was tampered")
	}
	return false
}

// TestRFC3748TheServerValidatesThePeersMIC proves the authentication-server
// clause of Section 7.5, which the peer-side unit for the row does not reach.
func TestRFC3748TheServerValidatesThePeersMIC(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peerCfg := &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	}

	// RFC requirement: RFC3748-7.5-1 positive -- RFC 3748 Section 7.5: "If a
	// per-packet MIC is employed within an EAP method, then peers, authentication
	// servers, and authenticators not operating in pass-through mode MUST validate
	// the MIC." Ze's authenticator is the authentication server, and it accepts
	// the peer's unmodified EAP-TLS flight with EAP-Success.
	clean := runEAPTLSHandshake(t, pki.serverConfig(), NewPeerSessionTLS("eap-tls-client", peerCfg))
	if !clean.serverEAPSuccess {
		t.Fatal("the authenticator refused an unmodified EAP-TLS exchange")
	}

	// RFC requirement: RFC3748-7.5-1 negative -- one octet changed in the peer's
	// authenticated flight fails the TLS integrity check on the authenticator,
	// which never sends EAP-Success for it.
	if runTamperedPeerResponse(t, pki.serverConfig(), NewPeerSessionTLS("eap-tls-client", peerCfg)) {
		t.Fatal("the authenticator sent EAP-Success for a peer flight modified in flight")
	}
}

// TestRFC3748KeyDerivingMethodsAuthenticateThePeerToo proves the server half of
// Section 7.10 mutual authentication for EAP-TLS, and both halves for the
// other key-deriving method, EAP-MSCHAPv2.
func TestRFC3748KeyDerivingMethodsAuthenticateThePeerToo(t *testing.T) {
	pki := newEAPTLSPKI(t)

	// RFC requirement: RFC3748-7.10-4 positive -- RFC 3748 Section 7.10: "EAP
	// Methods deriving keys MUST provide for mutual authentication between the EAP
	// peer and the EAP Server." EAP-MSCHAPv2 derives an MSK, and the peer
	// concludes only after it verified the Authenticator Response while the
	// authenticator verified the NT-Response: both ends hold the same key.
	flight := driveClauseFlight(t, "secret", "secret")
	if flight.last.Code != CodeSuccess {
		t.Fatalf("a verified MS-CHAPv2 exchange ended with code %d", flight.last.Code)
	}
	if flight.auth.MSK() == ([64]byte{}) {
		t.Fatal("the MS-CHAPv2 authenticator derived no MSK")
	}

	// RFC requirement: RFC3748-7.10-4 negative -- each end refuses a far end it
	// cannot authenticate, and neither refusal reaches a key. EAP-TLS: the
	// authenticator refuses a client certificate from an untrusted CA. MS-CHAPv2:
	// the authenticator refuses a wrong NT-Response, and the peer refuses a wrong
	// Authenticator Response.
	rogue := NewPeerSessionTLS("rogue-client", &PeerTLSConfig{
		CertPEM:   pki.untrustedClientCertPEM,
		KeyPEM:    pki.untrustedClientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})
	res := runEAPTLSHandshake(t, pki.serverConfig(), rogue)
	if res.serverEAPSuccess {
		t.Fatal("the authenticator sent EAP-Success for an unauthenticated client")
	}
	if res.serverMSK != ([64]byte{}) {
		t.Fatal("the authenticator derived an MSK for an unauthenticated client")
	}

	wrongPeer := driveClauseFlight(t, "right", "wrong")
	if wrongPeer.last.Code != CodeFailure {
		t.Fatalf("a wrong NT-Response ended with code %d, want EAP-Failure", wrongPeer.last.Code)
	}
	if wrongPeer.auth.MSK() != ([64]byte{}) {
		t.Fatal("the authenticator kept an MSK for a peer it refused")
	}

	peer, _ := mschapv2PeerAtSuccess(t)
	refused := peer.Process(mschapv2Success(3, "S=0000000000000000000000000000000000000000 M=ok"))
	if refused.Err == nil {
		t.Fatal("the peer accepted an authenticator that does not know the password")
	}
	if final := peer.Process(&Packet{Code: CodeSuccess, Identifier: 3}); final.MSK != ([64]byte{}) {
		t.Fatal("the peer handed out an MSK for an authenticator it refused")
	}
}
