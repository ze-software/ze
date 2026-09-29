// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework (RFC 3748)
// RFC: rfc/short/rfc3748.md -- Sections 2.2, 4.1, 4.2 and 7.10: clauses the older units left unasserted
// Related: rfc3748_clause_test.go -- the first pass over the same kind of clause
// Related: rfc3748_mschapv2_emsk_test.go -- the EAP-MSCHAPv2 EMSK and its exchange helper
//
// Each test here proves one clause of a quoted sentence through the real
// Session and PeerSession entry points: the MS-CHAPv2 MSK value rather than its
// size, the authenticator discarding a Nak the peer may no longer send, the
// peer sending nothing to an invalid Request, the peer concluding failure when
// the EAP-Failure is lost, and Notification Requests carrying nothing to a
// method.
//
// VALIDATES: the EAP-MSCHAPv2 MSK is the RFC 3079 key pair and zero padding,
// derived without the EMSK; an unexpected Nak is discarded and the method
// continues; an invalid Request draws no Response; a lost EAP-Failure leaves
// the peer failed rather than waiting for success; the authenticator sends no
// Notification Request, and the peer hands a Notification's data to no method.
// PREVENTS: an MSK with a short derived prefix or derived from the EMSK; a Nak
// that ends or derails a method in progress; a peer answering a Request it must
// treat as invalid; a peer that a forged Success rescues after a failure
// indication; method data smuggled in a Notification.

package eap

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // RFC 3079 defines the MPPE keys over SHA-1
	"encoding/hex"
	"errors"
	"testing"
)

// rfc3079MasterKey rebuilds the RFC 3079 Section 3.4 GetMasterKey output from
// the password hash hash and the NT-Response, with the magic written out here
// rather than read from the producer's variable.
func rfc3079MasterKey(passwordHashHash [16]byte, ntResponse [24]byte) []byte {
	h := sha1.New() //nolint:gosec // RFC 3079 defines the MPPE keys over SHA-1
	h.Write(passwordHashHash[:])
	h.Write(ntResponse[:])
	h.Write([]byte("This is the MPPE Master Key"))
	return h.Sum(nil)[:16]
}

// rfc3079StartKey rebuilds RFC 3079 Section 3.4 GetAsymmetricStartKey for a
// 128-bit key under the magic given, with both pads written out here.
func rfc3079StartKey(masterKey []byte, magic string) []byte {
	h := sha1.New() //nolint:gosec // RFC 3079 defines the MPPE keys over SHA-1
	h.Write(masterKey)
	h.Write(make([]byte, 40))
	h.Write([]byte(magic))
	h.Write(bytes.Repeat([]byte{0xf2}, 40))
	return h.Sum(nil)[:16]
}

// The two RFC 3079 Section 3.4 magic constants, copied from the RFC text. On
// the server side the first is the receive key and the second the send key.
const (
	rfc3079Magic2 = "On the client side, this is the send key; on the server side, it is the receive key."
	rfc3079Magic3 = "On the client side, this is the receive key; on the server side, it is the send key."
)

// wantMSCHAPv2MSK is the MSK the RFC 3079 key pair gives: the server receive
// key, the server send key, and 32 zero octets.
func wantMSCHAPv2MSK(masterKey []byte) [64]byte {
	var msk [64]byte
	copy(msk[0:16], rfc3079StartKey(masterKey, rfc3079Magic2))
	copy(msk[16:32], rfc3079StartKey(masterKey, rfc3079Magic3))
	return msk
}

// mustHex decodes a hex vector copied from an RFC, and fails the test when the
// copy is malformed.
func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad vector %q: %v", s, err)
	}
	return b
}

// TestRFC3748MSCHAPv2MSKIsTheRFC3079Derivation asserts the value of the
// EAP-MSCHAPv2 MSK, octet by octet, rather than its size.
//
// Method: the RFC 3079 Section 3.5.3 vector (password "clientPass" and its
// NT-Response) gives the MasterKey and the server send key as printed in the
// RFC; the receive key is rebuilt here from the printed MasterKey. Then one
// real exchange is driven and both ends' MSK is compared with the same
// construction over that exchange's NT-Response.
//
// RFC requirement: RFC3748-7.10-2 positive -- the EAP-MSCHAPv2 MSK both ends
// export is 64 octets: the RFC 3079 server receive key, the server send key,
// and 32 zero octets, equal on both ends; a MSK with any other derived prefix
// fails.
//
// RFC requirement: RFC3748-7.10-6 positive -- the EAP-MSCHAPv2 MSK is a function
// of the MPPE MasterKey alone, which the EMSK does not enter, so the MSK is not
// a key derived from the EMSK.
func TestRFC3748MSCHAPv2MSKIsTheRFC3079Derivation(t *testing.T) {
	// The RFC 3079 Section 3.5.3 vector, from its printed MasterKey. The printed
	// MasterKey does not follow from the printed NT-Response under the Section
	// 3.4 GetMasterKey pseudo-code, so the vector is used from the MasterKey on,
	// where it is consistent: its SendStartKey128 is the Magic3 key.
	masterVector := mustHex(t, "fdece3717a8c838cb388e527ae3cdd31")
	serverSendVector := mustHex(t, "8b7cdc149b993a1ba118cb153f56dccb")
	if control := rfc3079StartKey(masterVector, rfc3079Magic3); !bytes.Equal(control, serverSendVector) {
		t.Fatalf("control send key %x is not the RFC 3079 vector %x, so the construction here is wrong", control, serverSendVector)
	}
	send := GetAsymmetricStartKey([16]byte(masterVector), 16, false, true)
	if !bytes.Equal(send, serverSendVector) {
		t.Fatalf("server send key = %x, want the RFC 3079 Section 3.5.3 SendStartKey128 %x", send, serverSendVector)
	}

	// One real exchange: both ends hold the same construction over the
	// NT-Response that crossed.
	const password = "correct horse battery staple"
	sess, peer, _ := mschapv2EMSKExchange(t, password)

	master := rfc3079MasterKey(hashNtPasswordHash(ntPasswordHash(password)), peer.ntResponse)
	want := wantMSCHAPv2MSK(master)
	if got := sess.MSK(); got != want {
		t.Fatalf("authenticator MSK = %x,\n  want %x", got, want)
	}
	if peer.msk != want {
		t.Fatalf("peer MSK = %x,\n  want %x", peer.msk, want)
	}
	if bytes.Equal(want[0:16], want[16:32]) {
		t.Fatal("the receive and send keys are equal, so the control construction is wrong")
	}
}

// mschapv2Committed drives an EAP-MSCHAPv2 exchange up to the authenticator's
// Success Request, so the peer has sent its initial non-Nak Response and the
// method is under way. It answers the Request not yet handed to the peer.
func mschapv2Committed(t *testing.T, authPassword, peerPassword string) (*Session, *PeerSession, *Packet) {
	t.Helper()
	sess, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: authPassword})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(sess.Close)
	peer := NewPeerSession(TypeMSCHAPv2, "user", peerPassword)
	t.Cleanup(peer.Close)

	req := sess.Begin()
	for range 2 {
		res := peer.Process(req)
		if res.Response == nil {
			t.Fatalf("the peer did not answer %v: %v", req, res.Err)
		}
		req = sess.Process(res.Response)
		if req == nil {
			t.Fatal("the authenticator stopped answering before the method result")
		}
	}
	if req.Code != CodeRequest || req.Type != TypeMSCHAPv2 {
		t.Fatalf("third authenticator packet is %v, want the MS-CHAPv2 result Request", req)
	}
	return sess, peer, req
}

// TestRFC3748AuthenticatorDiscardsANakAfterTheInitialNonNakResponse asserts the
// Nak half of the Section 4.1 discard.
//
// Method: once the peer has answered the MS-CHAPv2 Challenge with a
// Response of that Type, a legacy Nak for the next Request is handed to the
// authenticator; then the peer's real answer is.
//
// RFC requirement: RFC3748-4.1-11 positive -- a Nak Response arriving after the
// peer's initial non-Nak Response draws no packet and does not end the
// exchange: the peer's real Response to the same Request still concludes it
// with EAP-Success.
func TestRFC3748AuthenticatorDiscardsANakAfterTheInitialNonNakResponse(t *testing.T) {
	sess, peer, req := mschapv2Committed(t, "secret", "secret")

	nak := &Packet{Code: CodeResponse, Identifier: req.Identifier, Type: TypeNAK, TypeData: []byte{TypeMD5Challenge}}
	if out := sess.Process(nak); out != nil {
		t.Fatalf("a Nak after the initial non-Nak Response drew %v, want a silent discard", out)
	}
	if sess.Succeeded() {
		t.Fatal("the discarded Nak concluded the exchange")
	}

	res := peer.Process(req)
	if res.Response == nil {
		t.Fatalf("the peer did not answer the result Request: %v", res.Err)
	}
	out := sess.Process(res.Response)
	if out == nil || out.Code != CodeSuccess {
		t.Fatalf("the peer's real Response after the discarded Nak drew %v, want EAP-Success", out)
	}
}

// TestRFC3748PeerAnswersNoInvalidRequest hands the peer Requests it must treat
// as invalid and asserts no Response goes out for any of them.
//
// Method: mid-method, a Request for another method; after EAP-Success, a
// Request for the method itself. Each is followed by the valid Request, which
// is answered, so the silence is the validity check acting.
//
// RFC requirement: RFC3748-4.1-1 negative -- a Request of another Type after the
// method is under way, and a method Request after the EAP-Success, each draw no
// Response and no error; the valid Request between them draws one.
func TestRFC3748PeerAnswersNoInvalidRequest(t *testing.T) {
	sess, peer, req := mschapv2Committed(t, "secret", "secret")

	other := &Packet{Code: CodeRequest, Identifier: req.Identifier, Type: TypeTLS, TypeData: []byte{0x20}}
	if res := peer.Process(other); res.Response != nil || res.Err != nil || !res.Discarded {
		t.Fatalf("a Request of another method mid-method drew %+v, want a silent discard", res)
	}

	res := peer.Process(req)
	if res.Response == nil {
		t.Fatalf("the valid result Request drew no Response: %v", res.Err)
	}
	success := sess.Process(res.Response)
	if success == nil || success.Code != CodeSuccess {
		t.Fatalf("the exchange did not conclude: %v", success)
	}
	if done := peer.Process(success); !done.Done {
		t.Fatalf("the peer did not conclude on EAP-Success: %+v", done)
	}

	after := &Packet{Code: CodeRequest, Identifier: success.Identifier + 1, Type: TypeMSCHAPv2, TypeData: req.TypeData}
	if res := peer.Process(after); res.Response != nil || res.Err != nil || !res.Discarded {
		t.Fatalf("a method Request after EAP-Success drew %+v, want a silent discard", res)
	}
}

// TestRFC3748PeerConcludesFailureWithoutTheFailurePacket drops the EAP-Failure
// that follows an MS-CHAPv2 failure indication and asserts the peer does not
// depend on it.
//
// Method: the peer holds the wrong password; the authenticator's MS-CHAPv2
// Failure Request is answered, the EAP-Failure it sends next is lost, and the
// next packet the peer reads is a forged EAP-Success, or a stray Request.
//
// RFC requirement: RFC3748-4.2-11 positive -- with the EAP-Failure lost, the
// peer that received the failure result indication ends the exchange with
// ErrEAPFailure on the next packet it reads, is not Done, and exports no MSK:
// it concluded failure without the Failure packet.
func TestRFC3748PeerConcludesFailureWithoutTheFailurePacket(t *testing.T) {
	cases := []struct {
		name string
		next func(id uint8) *Packet
	}{
		{"forged success", func(id uint8) *Packet { return &Packet{Code: CodeSuccess, Identifier: id} }},
		{"stray request", func(id uint8) *Packet {
			return &Packet{Code: CodeRequest, Identifier: id + 1, Type: TypeIdentity}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess, peer, req := mschapv2Committed(t, "the-real-password", "a-wrong-password")
			if len(req.TypeData) == 0 || req.TypeData[0] != mschapv2OpFailure {
				t.Fatalf("the authenticator sent %v, want the MS-CHAPv2 Failure Request", req)
			}
			ack := peer.Process(req)
			if ack.Response == nil || ack.Err != nil {
				t.Fatalf("the peer did not acknowledge the failure indication: %+v", ack)
			}
			lost := sess.Process(ack.Response)
			if lost == nil || lost.Code != CodeFailure {
				t.Fatalf("the authenticator sent %v after the acknowledgment, want EAP-Failure", lost)
			}

			res := peer.Process(tc.next(lost.Identifier))
			if !errors.Is(res.Err, ErrEAPFailure) {
				t.Fatalf("with the EAP-Failure lost, the peer returned %+v, want ErrEAPFailure", res)
			}
			if res.Done || res.MSK != ([64]byte{}) || res.Response != nil {
				t.Fatalf("the peer concluded or answered after a failure indication: %+v", res)
			}
		})
	}
}

// TestRFC3748TheAuthenticatorSendsNoNotificationRequest reads every Request
// ze's authenticator sends in a successful and in a failed conversation.
//
// RFC requirement: RFC3748-2.2-1 positive -- no Request of either conversation is
// a Notification Request, so ze's authenticator carries no method data in one.
func TestRFC3748TheAuthenticatorSendsNoNotificationRequest(t *testing.T) {
	for _, peerPassword := range []string{"secret", "wrong"} {
		fl := driveClauseFlight(t, "secret", peerPassword)
		if fl.last == nil {
			t.Fatalf("peer password %q: the conversation did not conclude", peerPassword)
		}
		for i, r := range fl.requests {
			if r.Code == CodeRequest && r.Type == TypeNotification {
				t.Fatalf("peer password %q: Request %d is a Notification Request", peerPassword, i)
			}
		}
	}
}

// TestRFC3748ANotificationRequestCarriesNothingToTheMethod injects a
// Notification Request whose Type-Data is the MS-CHAPv2 result message itself.
//
// Method: mid-method, before the real result Request, the Notification is
// handed to the peer; then a forged early EAP-Success; then the real Request.
//
// RFC requirement: RFC3748-2.2-1 negative -- the peer answers the Notification
// with an empty Notification Response and no MS-CHAPv2 Response, the method
// does not advance (an EAP-Success right after it is discarded), and the real
// result Request still concludes the exchange with the MSK both ends share.
func TestRFC3748ANotificationRequestCarriesNothingToTheMethod(t *testing.T) {
	sess, peer, req := mschapv2Committed(t, "secret", "secret")

	note := &Packet{Code: CodeRequest, Identifier: req.Identifier, Type: TypeNotification, TypeData: req.TypeData}
	res := peer.Process(note)
	if res.Response == nil || res.Response.Type != TypeNotification || len(res.Response.TypeData) != 0 {
		t.Fatalf("the Notification Request drew %+v, want an empty Notification Response", res.Response)
	}

	early := peer.Process(&Packet{Code: CodeSuccess, Identifier: req.Identifier})
	if early.Done || !early.Discarded {
		t.Fatalf("an EAP-Success after the Notification drew %+v: the Notification's data reached the method", early)
	}

	res = peer.Process(req)
	if res.Response == nil || res.Response.Type != TypeMSCHAPv2 {
		t.Fatalf("the real result Request drew %+v, want the MS-CHAPv2 acknowledgment", res.Response)
	}
	success := sess.Process(res.Response)
	if success == nil || success.Code != CodeSuccess {
		t.Fatalf("the exchange did not conclude: %v", success)
	}
	done := peer.Process(success)
	if !done.Done || done.MSK != sess.MSK() {
		t.Fatalf("the peer did not conclude with the shared MSK: %+v", done)
	}
}
