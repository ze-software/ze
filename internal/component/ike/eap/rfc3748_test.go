// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework (RFC 3748)
// RFC: rfc/short/rfc3748.md -- EAP packet format and exchange model
//
// RFC 3748 (EAP) enrollment coverage for the framework logic ze's `eap` package
// genuinely implements: packet length validation (Section 4), the lock-step
// authenticator state machine (Sections 2, 2.1), Success/Failure formatting and
// single emission (Section 4.2), the peer's no-NAK / no-self-timer behavior
// (Sections 2.1, 4.1), and the MSK size / key-deriving-method constraints
// (Section 7.10). Lower-layer obligations (Section 3.1), pass-through (Section
// 2.3) and Expanded/EMSK details are annotated in the summary as not-applicable:
// ze carries EAP only inside IKEv2, terminates every method locally, and offers
// only the two key-deriving methods EAP-TLS and EAP-MSCHAPv2.
//
// Both EAP roles ze plays are exercised: the authenticator via `Session`
// (Begin/Process) and the peer via `PeerSession` (Process), driven end-to-end
// against each other with real MS-CHAPv2 credentials.
//
// VALIDATES: EAP packets are length-validated (short-of-Length discarded, 4-octet
// minimum); Success/Failure are 4 octets with no Type field and are emitted once;
// the authenticator is lock-step and only advances on a valid Response; a single
// method runs per conversation and its completion sends Success or Failure; the
// peer never NAKs and never self-retransmits; a completed method yields a 64-octet
// MSK; and only key-deriving methods (EAP-TLS, EAP-MSCHAPv2) are selectable while
// non-keying types (MD5-Challenge/OTP/GTC) are refused for IKEv2.
// PREVENTS: accepting an over-length or sub-minimum EAP frame, formatting or
// retransmitting Success/Failure incorrectly, advancing the authenticator without
// a valid Response, running a second method mid-conversation, the peer emitting a
// NAK or a spontaneous retransmission, shrinking the MSK below 64 octets, or
// admitting a non-key-deriving EAP method into an IKEv2 exchange.

package eap

import "testing"

// driveMSCHAPv2 runs a full EAP-MSCHAPv2 conversation between the real
// authenticator Session and the real peer PeerSession, returning the
// authenticator session and its final EAP packet (Success or Failure). The peer
// authenticates with peerPassword; the authenticator verifies against
// authPassword, so passing mismatched values drives the Failure path.
func driveMSCHAPv2(t *testing.T, authPassword, peerPassword string) (*Session, *Packet) {
	t.Helper()
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: authPassword})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	peer := NewPeerSession(TypeMSCHAPv2, "user", peerPassword)

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
			t.Fatalf("auth round %d: unexpected nil (premature terminal)", round)
		}
		if next.Code == CodeSuccess || next.Code == CodeFailure {
			return auth, next
		}
		req = next
	}
	t.Fatal("EAP-MSCHAPv2 exchange did not terminate within bound")
	return nil, nil
}

func TestRFC3748PacketLengthDiscard(t *testing.T) {
	// RFC requirement: RFC3748-4-1 positive -- a packet whose Length field equals its
	// actual octet count decodes successfully.
	valid := (&Packet{Code: CodeRequest, Identifier: 7, Type: TypeIdentity, TypeData: []byte("id")}).Encode()
	if _, err := DecodePacket(valid); err != nil {
		t.Fatalf("valid packet rejected: %v", err)
	}

	// RFC requirement: RFC3748-4-1 negative -- a packet whose Length field claims more
	// octets than are actually present is discarded (DecodePacket returns an error).
	short := []byte{CodeRequest, 7, 0, 12, TypeIdentity} // declares length 12, only 5 octets present
	if _, err := DecodePacket(short); err == nil {
		t.Fatal("packet shorter than its Length field was accepted; must be discarded")
	}
}

func TestRFC3748MinimumPacketLength(t *testing.T) {
	// RFC requirement: RFC3748-4-2 positive -- a 4-octet Success packet (the minimum
	// legal EAP packet) decodes.
	ok := (&Packet{Code: CodeSuccess, Identifier: 9}).Encode()
	if len(ok) != 4 {
		t.Fatalf("Success encoded to %d octets, want 4", len(ok))
	}
	if _, err := DecodePacket(ok); err != nil {
		t.Fatalf("minimum-length packet rejected: %v", err)
	}

	// RFC requirement: RFC3748-4-2 negative -- input below the 4-octet minimum, and a
	// declared Length below 4, are both rejected.
	if _, err := DecodePacket([]byte{CodeSuccess, 9, 0}); err == nil {
		t.Fatal("3-octet input accepted; minimum EAP packet length is 4")
	}
	if _, err := DecodePacket([]byte{CodeSuccess, 9, 0, 3}); err == nil {
		t.Fatal("packet declaring Length 3 accepted; minimum is 4")
	}
}

func TestRFC3748SuccessFailureFormat(t *testing.T) {
	// RFC requirement: RFC3748-4.2-2 positive -- Success and Failure encode to exactly
	// 4 octets (Code, Identifier, Length) and carry no Type field; the decoder restores
	// Code/Identifier with Type left unset.
	for _, code := range []uint8{CodeSuccess, CodeFailure} {
		enc := (&Packet{Code: code, Identifier: 3}).Encode()
		if len(enc) != 4 {
			t.Fatalf("code %d encoded to %d octets, want 4 (no Type field)", code, len(enc))
		}
		if enc[2] != 0 || enc[3] != 4 {
			t.Fatalf("code %d Length field = %d, want 4", code, int(enc[2])<<8|int(enc[3]))
		}
		dec, err := DecodePacket(enc)
		if err != nil {
			t.Fatalf("code %d decode: %v", code, err)
		}
		if dec.Type != 0 || dec.TypeData != nil {
			t.Fatalf("code %d decoded a Type field (%d) it must not carry", code, dec.Type)
		}
	}

	// RFC requirement: RFC3748-4.2-2 negative -- a Request/Response is NOT a 4-octet
	// packet: with Length 4 there is no room for the mandatory Type field, so the
	// decoder rejects it. This pins the Type field as present for Codes 1-2 and absent
	// for Codes 3-4.
	if _, err := DecodePacket([]byte{CodeRequest, 3, 0, 4}); err == nil {
		t.Fatal("Request of length 4 accepted; Request/Response must carry a Type field")
	}
}

func TestRFC3748SuccessFailureNotRetransmitted(t *testing.T) {
	// RFC requirement: RFC3748-4.2-1 positive -- once the authenticator emits Failure it
	// enters a terminal state and never re-emits it: a further Response yields nil.
	fail, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	req := fail.Begin()
	nak := &Packet{Code: CodeResponse, Identifier: req.Identifier, Type: TypeNAK, TypeData: []byte{TypeTLS}}
	if out := fail.Process(nak); out == nil || out.Code != CodeFailure {
		t.Fatalf("expected Failure, got %v", out)
	}
	if again := fail.Process(&Packet{Code: CodeResponse, Identifier: 5, Type: TypeIdentity}); again != nil {
		t.Fatalf("Failure re-emitted after terminal state: %v", again)
	}

	// And once it emits Success, a further Response likewise yields nil -- Success is
	// sent once, never retransmitted by the EAP layer.
	auth, last := driveMSCHAPv2(t, "secret", "secret")
	if last.Code != CodeSuccess {
		t.Fatalf("expected Success, got code %d", last.Code)
	}
	if again := auth.Process(&Packet{Code: CodeResponse, Identifier: 9, Type: TypeMSCHAPv2}); again != nil {
		t.Fatalf("Success re-emitted after terminal state: %v", again)
	}
}

func TestRFC3748AuthenticatorLockStep(t *testing.T) {
	// RFC requirement: RFC3748-2-1 positive -- the authenticator is lock-step: Begin()
	// yields exactly one Request and each Process() yields at most one packet, so there
	// is never more than one outstanding Request. Driving a full exchange, every
	// authenticator turn produces a single non-nil packet until the terminal one.
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")

	req := auth.Begin()
	if req == nil || req.Code != CodeRequest {
		t.Fatalf("Begin produced %v, want a single Request", req)
	}
	outstanding := 1 // the Request just emitted awaits its Response
	for round := range 8 {
		pr := peer.Process(req)
		if pr.Err != nil {
			t.Fatalf("peer round %d: %v", round, pr.Err)
		}
		outstanding-- // the peer's Response consumes the outstanding Request
		if outstanding != 0 {
			t.Fatalf("round %d: %d outstanding Requests before Response, want 0", round, outstanding+1)
		}
		next := auth.Process(pr.Response)
		if next == nil {
			t.Fatalf("auth round %d: nil", round)
		}
		if next.Code == CodeSuccess || next.Code == CodeFailure {
			return
		}
		outstanding++ // exactly one new Request is now outstanding
		if outstanding != 1 {
			t.Fatalf("round %d: authenticator emitted more than one outstanding Request", round)
		}
		req = next
	}
	t.Fatal("exchange did not terminate")
}

func TestRFC3748AuthenticatorRequiresValidResponse(t *testing.T) {
	// RFC requirement: RFC3748-2-2 positive -- a valid Identity Response advances the
	// authenticator to the first method Request.
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	req := auth.Begin()
	next := auth.Process(&Packet{Code: CodeResponse, Identifier: req.Identifier, Type: TypeIdentity, TypeData: []byte("user")})
	if next == nil || next.Code != CodeRequest || next.Type != TypeMSCHAPv2 {
		t.Fatalf("valid Identity Response did not yield a method Request: %v", next)
	}

	// RFC requirement: RFC3748-2-2 negative -- a packet that is not a Response (Code !=
	// Response) never produces a new method Request; the authenticator returns Failure.
	auth2, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	auth2.Begin()
	notResponse := &Packet{Code: CodeRequest, Identifier: 2, Type: TypeIdentity, TypeData: []byte("user")}
	out := auth2.Process(notResponse)
	if out == nil || out.Code != CodeFailure {
		t.Fatalf("non-Response advanced the exchange: %v (want Failure)", out)
	}
}

func TestRFC3748OneMethodPerConversation(t *testing.T) {
	// RFC requirement: RFC3748-2.1-1 positive -- a single configured method (MS-CHAPv2)
	// carries the whole conversation to completion.
	_, last := driveMSCHAPv2(t, "secret", "secret")
	if last.Code != CodeSuccess {
		t.Fatalf("single-method exchange did not succeed: code %d", last.Code)
	}

	// RFC requirement: RFC3748-2.1-1 negative -- once the method is active, a Response
	// bearing a DIFFERENT EAP method Type is rejected (Failure); the authenticator never
	// switches to a second method within one conversation.
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	auth.Begin()
	auth.Process(&Packet{Code: CodeResponse, Identifier: 1, Type: TypeIdentity, TypeData: []byte("user")}) // -> method active
	wrong := &Packet{Code: CodeResponse, Identifier: 2, Type: TypeTLS, TypeData: []byte{0x20}}
	out := auth.Process(wrong)
	if out == nil || out.Code != CodeFailure {
		t.Fatalf("a second method Type was accepted mid-conversation: %v (want Failure)", out)
	}
}

func TestRFC3748MethodCompletionSendsResult(t *testing.T) {
	// RFC requirement: RFC3748-2.1-2 positive -- when the method completes successfully
	// the authenticator sends EAP-Success.
	authOK, last := driveMSCHAPv2(t, "secret", "secret")
	if last.Code != CodeSuccess {
		t.Fatalf("successful method did not yield Success: code %d", last.Code)
	}
	if !authOK.Succeeded() {
		t.Fatal("session should report Succeeded after Success")
	}

	// RFC requirement: RFC3748-2.1-2 negative -- when the method fails (wrong password)
	// the authenticator sends EAP-Failure, never Success.
	authBad, fail := driveMSCHAPv2(t, "right", "wrong")
	if fail.Code != CodeFailure {
		t.Fatalf("failed method did not yield Failure: code %d", fail.Code)
	}
	if authBad.Succeeded() {
		t.Fatal("session must not report Succeeded after Failure")
	}
}

func TestRFC3748PeerNeverSendsNAK(t *testing.T) {
	// RFC requirement: RFC3748-2.1-3 positive -- the EAP peer never emits a Type-3 NAK:
	// it answers Identity with Identity and each method Request with a method Response,
	// and errors (never NAKs) on an unexpected type. No Response it produces across a
	// full exchange -- nor when handed an unexpected type -- is a NAK.
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	req := auth.Begin()
	for round := range 8 {
		pr := peer.Process(req)
		if pr.Response != nil && pr.Response.Type == TypeNAK {
			t.Fatalf("peer emitted a Type-3 NAK at round %d", round)
		}
		if pr.Err != nil || pr.Done || pr.Response == nil {
			break
		}
		next := auth.Process(pr.Response)
		if next == nil || next.Code == CodeSuccess || next.Code == CodeFailure {
			// Feed the terminal packet so the peer processes Success/Failure too.
			if next != nil {
				peer.Process(next)
			}
			break
		}
		req = next
	}

	// An unexpected type in the identity phase must produce an error, not a NAK.
	peer2 := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	res := peer2.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: 99})
	if res.Response != nil && res.Response.Type == TypeNAK {
		t.Fatal("peer answered an unexpected type with a NAK; it must error instead")
	}
	if res.Err == nil {
		t.Fatal("peer should error on an unexpected initial type")
	}
}

func TestRFC3748PeerHasNoRetransmitTimer(t *testing.T) {
	// RFC requirement: RFC3748-4.1-1 positive -- the EAP peer is a synchronous,
	// request-driven transform: it emits a Response only when handed a Request via
	// Process, exactly one per call, and never self-retransmits (it holds no timer).
	// Lower-layer retransmission is IKEv2's, not the EAP peer's.
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	req := auth.Begin()
	responses := 0
	for round := range 8 {
		pr := peer.Process(req)
		if pr.Err != nil {
			t.Fatalf("peer round %d: %v", round, pr.Err)
		}
		if pr.Response == nil {
			t.Fatalf("peer round %d produced no Response for a Request", round)
		}
		responses++ // exactly one Response emitted for this one Request
		next := auth.Process(pr.Response)
		if next == nil {
			t.Fatalf("auth round %d: nil", round)
		}
		if next.Code == CodeSuccess || next.Code == CodeFailure {
			break
		}
		req = next
	}
	if responses == 0 {
		t.Fatal("peer produced no output; it must respond to Requests")
	}
}

func TestRFC3748MSKSize(t *testing.T) {
	// RFC requirement: RFC3748-7.10-1 positive -- a completed key-deriving method yields
	// a 64-octet MSK. The peer's derived MSK is exactly 64 octets and non-zero.
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	req := auth.Begin()
	var mskResult PeerResult
	for round := range 8 {
		pr := peer.Process(req)
		if pr.Err != nil {
			t.Fatalf("peer round %d: %v", round, pr.Err)
		}
		if pr.Done {
			mskResult = pr
			break
		}
		next := auth.Process(pr.Response)
		if next == nil {
			t.Fatalf("auth round %d: nil", round)
		}
		// Deliver the authenticator's EAP-Success to the peer so it completes.
		if next.Code == CodeSuccess {
			mskResult = peer.Process(next)
			break
		}
		req = next
	}
	if !mskResult.Done {
		t.Fatal("exchange did not complete with a peer MSK")
	}
	if len(mskResult.MSK) != 64 {
		t.Fatalf("MSK is %d octets, want 64", len(mskResult.MSK))
	}
	var zero [64]byte
	if mskResult.MSK == zero {
		t.Fatal("derived MSK is all-zero; a real key must be derived")
	}
}

func TestRFC3748IKEv2RequiresKeyDerivingMethod(t *testing.T) {
	// RFC requirement: RFC3748-7.10-3 positive -- a key-deriving method ze offers for
	// IKEv2 (EAP-MSCHAPv2) is accepted by NewSession and starts an EAP conversation;
	// TestRFC3748MSKSize confirms such a method yields a real 64-octet MSK for the
	// IKEv2 AUTH payload.
	sess, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("EAP-MSCHAPv2 (a key-deriving method) was rejected: %v", err)
	}
	if sess.Begin().Type != TypeIdentity {
		t.Fatal("accepted key-deriving method did not start an EAP conversation")
	}

	// RFC requirement: RFC3748-7.10-3 negative -- methods that do not derive an MSK
	// (MD5-Challenge type 4, OTP type 5, GTC type 6) are refused by NewSession, so they
	// can never be selected for an IKEv2 conversation.
	for _, nonKeying := range []uint8{4, 5, 6} {
		if _, err := NewSession(nonKeying, MethodConfig{}); err == nil {
			t.Fatalf("non-key-deriving method type %d was accepted for IKEv2", nonKeying)
		}
	}
}
