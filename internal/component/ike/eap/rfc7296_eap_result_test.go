// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework tests

package eap

import "testing"

// eapReachMethod drives a session past the identity round, so the next Process call
// lands in the method state where success and failure are decided.
func eapReachMethod(t *testing.T, password string) (*Session, *Packet) {
	t.Helper()
	sess, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: password})
	if err != nil {
		t.Fatalf("create MSCHAPv2 session: %v", err)
	}
	req := sess.Begin()
	next := sess.Process(&Packet{
		Code:       CodeResponse,
		Identifier: req.Identifier,
		Type:       TypeIdentity,
		TypeData:   []byte("testuser"),
	})
	if next == nil {
		t.Fatal("the identity round produced no method request")
	}
	return sess, next
}

// eapRunToResult plays the authenticator and the peer against each other until the
// session emits Success or Failure. MSCHAPv2 takes several rounds, so a single Process
// call lands mid-method rather than on the result.
func eapRunToResult(t *testing.T, sess *Session, peer *PeerSession, req *Packet) *Packet {
	t.Helper()
	for range 8 {
		if req.Code == CodeSuccess || req.Code == CodeFailure {
			return req
		}
		reply := peer.Process(req)
		if reply.Response == nil {
			t.Fatalf("the peer produced no answer to an EAP code-%d packet", req.Code)
		}
		next := sess.Process(reply.Response)
		if next == nil {
			t.Fatal("the authenticator produced no further EAP packet before a result")
		}
		req = next
	}
	t.Fatal("the EAP exchange did not reach Success or Failure within its round budget")
	return nil
}

// VALIDATES: a completed EAP method makes the responder emit an EAP Success packet, and
// the session then reports success and holds an MSK.
// PREVENTS: an EAP exchange that authenticates the peer and never tells it so, which
// leaves the initiator waiting for a Success that RFC 7296 requires.
// RFC requirement: RFC7296-2.16-14 positive -- RFC 7296 Section 2.16: "Once the protocol
// exchange defined by the chosen EAP authentication method has successfully terminated,
// the responder MUST send an EAP payload containing the Success message".
func TestEapResultSuccessIsSent(t *testing.T) {
	const password = "testpass"
	sess, methodReq := eapReachMethod(t, password)

	peer := NewPeerSession(TypeMSCHAPv2, "testuser", password)
	final := eapRunToResult(t, sess, peer, methodReq)
	if final.Code != CodeSuccess {
		t.Errorf("the final EAP packet carries code %d, want Success (%d)", final.Code, CodeSuccess)
	}
	if !sess.Succeeded() {
		t.Error("the session emitted Success without recording success")
	}
	if sess.MSK() == ([64]byte{}) {
		t.Error("a successful EAP method left the MSK zero, so no AUTH can be derived from it")
	}
}

// VALIDATES: a failed EAP method makes the responder emit an EAP Failure packet.
// PREVENTS: a refused peer being answered with silence or with Success, either of which
// leaves the initiator unable to tell that authentication was denied.
// RFC requirement: RFC7296-2.16-15 positive -- RFC 7296 Section 2.16: "Similarly, if the
// authentication method has failed, the responder MUST send an EAP payload containing
// the Failure message."
// RFC requirement: RFC7296-2.16-14 negative -- the same session emits Failure, not
// Success, and reports Succeeded false. Without this the Success test above would also
// pass against a responder that answered every exchange with Success.
func TestEapResultFailureIsSent(t *testing.T) {
	sess, methodReq := eapReachMethod(t, "the-right-password")

	// The peer answers with the WRONG credential, so the method refuses it.
	peer := NewPeerSession(TypeMSCHAPv2, "testuser", "the-wrong-password")
	final := eapRunToResult(t, sess, peer, methodReq)
	if final.Code != CodeFailure {
		t.Errorf("the final EAP packet carries code %d, want Failure (%d)", final.Code, CodeFailure)
	}
	if sess.Succeeded() {
		t.Error("the session reports success after emitting Failure")
	}
	if sess.MSK() != ([64]byte{}) {
		t.Error("a failed EAP method produced an MSK, which would key an AUTH the peer never earned")
	}
}

// RFC requirement: RFC7296-2.16-15 negative -- a session that SUCCEEDS emits Success and
// not Failure. The two results are therefore distinguished by the exchange, and neither
// test above would pass against a responder that emitted one code unconditionally.
func TestEapResultSuccessIsNotFailure(t *testing.T) {
	const password = "shared"
	sess, methodReq := eapReachMethod(t, password)

	peer := NewPeerSession(TypeMSCHAPv2, "testuser", password)
	final := eapRunToResult(t, sess, peer, methodReq)
	if final.Code == CodeFailure {
		t.Error("a correct credential drew EAP Failure")
	}
}
