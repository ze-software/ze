// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- the MS-CHAPv2 Failure packet
// Detail: eap_mschapv2.go sendFailure -- the packet this file pins
// Related: peer.go handleMSCHAPv2Failure, parseMSCHAPv2Failure -- the receiving half
//
// VALIDATES: an NT-Response the authenticator cannot verify produces an MS-CHAPv2
// Failure packet (OpCode 4) whose Message carries E=691 and a fresh 32-digit C=
// challenge, that the exchange ends behind that packet, and that Ze's own peer
// reads the packet and surfaces the error code.
// PREVENTS: the authenticator answering a bad password with a bare EAP-Failure,
// which carries no field for a reason at all (RFC 3748 Section 4.2). The peer
// then learns nothing beyond "it failed", and a peer that follows RFC 2759 waits
// for a Failure packet Ze never sends.

package eap

import (
	"errors"
	"strings"
	"testing"
)

// mschapv2Refusal drives an authenticator Session and a peer through Identity,
// Challenge and Response with the two sides holding the given passwords, and
// returns the session together with the packet the authenticator answered the
// peer's Response with.
func mschapv2Refusal(t *testing.T, peerPassword, serverPassword string) (*Session, *Packet) {
	t.Helper()

	sess, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: serverPassword})
	if err != nil {
		t.Fatalf("new authenticator session: %v", err)
	}
	t.Cleanup(sess.Close)

	peer := NewPeerSession(TypeMSCHAPv2, "testuser", peerPassword)

	identity := peer.Process(sess.Begin())
	if identity.Err != nil {
		t.Fatalf("identity: %v", identity.Err)
	}
	challenge := sess.Process(identity.Response)
	if challenge == nil {
		t.Fatal("the authenticator sent no MS-CHAPv2 Challenge")
	}
	answer := peer.Process(challenge)
	if answer.Err != nil {
		t.Fatalf("challenge: %v", answer.Err)
	}
	if answer.Response == nil {
		t.Fatal("the peer sent no MS-CHAPv2 Response to the Challenge")
	}

	verdict := sess.Process(answer.Response)
	if verdict == nil {
		t.Fatal("the authenticator answered the MS-CHAPv2 Response with nothing")
	}
	return sess, verdict
}

// mschapv2Failure frames an MS-CHAPv2 Failure packet carrying the given Message.
// Wire format, RFC 2759 Section 6 over draft-kamath EAP encapsulation:
//
//	 0        1        2        3        4                 msLen
//	+--------+--------+--------+--------+------------------+
//	| OpCode | MS-ID  |   MS-Length     |     Message      |
//	+--------+--------+--------+--------+------------------+
//	    4       copied   total octets     "E= R= C= V= M="
func mschapv2Failure(msID uint8, message string) *Packet {
	msLen := 4 + len(message)
	td := make([]byte, msLen)
	td[0] = mschapv2OpFailure
	td[1] = msID
	td[2] = byte(msLen >> 8)
	td[3] = byte(msLen)
	copy(td[4:], message)

	return &Packet{
		Code:       CodeRequest,
		Identifier: 4,
		Type:       TypeMSCHAPv2,
		TypeData:   td,
	}
}

// failureField returns the value of one space-delimited field of a Failure
// Message, and reports whether the field is present.
func failureField(message, prefix string) (string, bool) {
	for field := range strings.SplitSeq(message, " ") {
		if value, found := strings.CutPrefix(field, prefix); found {
			return value, true
		}
	}
	return "", false
}

// TestRFC2759AuthenticatorRefusesWithErrorCode691 checks that an NT-Response the
// authenticator cannot verify produces an MS-CHAPv2 Failure packet carrying
// E=691, and that the exchange ends behind it.
//
// RFC 2759 Section 6: "The "eeeeeeeeee" is the ASCII representation of a decimal
// error code (need not be 10 digits) corresponding to one of those listed below",
// among which "691 ERROR_AUTHENTICATION_FAILURE".
//
// RFC requirement: RFC2759-x-12 positive -- a mismatched NT-Response makes the
// authenticator send an MS-CHAPv2 Failure whose E= field is 691, and the session
// is terminated with no MSK (eap_mschapv2.go sendFailure, eap.go finalRequest).
func TestRFC2759AuthenticatorRefusesWithErrorCode691(t *testing.T) {
	sess, verdict := mschapv2Refusal(t, "WrongPassword", "TestPassword")

	if verdict.Code != CodeRequest {
		t.Fatalf("verdict Code = %d, want %d (an EAP-Request carrying the Failure packet)", verdict.Code, CodeRequest)
	}
	if verdict.Type != TypeMSCHAPv2 {
		t.Fatalf("verdict Type = %d, want %d", verdict.Type, TypeMSCHAPv2)
	}
	if len(verdict.TypeData) <= 4 {
		t.Fatalf("the Failure packet carries no Message: %d octets", len(verdict.TypeData))
	}
	if got := verdict.TypeData[0]; got != mschapv2OpFailure {
		t.Fatalf("verdict OpCode = %d, want %d", got, mschapv2OpFailure)
	}

	message := string(verdict.TypeData[4:])
	code, ok := failureField(message, "E=")
	if !ok {
		t.Fatalf("the Failure Message carries no E= error code: %q", message)
	}
	if code != "691" {
		t.Fatalf("E= error code is %q, want \"691\" (ERROR_AUTHENTICATION_FAILURE)", code)
	}

	// Terminating the session. The decision is taken here: no success, no key,
	// and the cause recorded for the operator.
	if sess.Succeeded() {
		t.Fatal("the authenticator reports success after refusing the NT-Response")
	}
	if sess.MSK() != ([64]byte{}) {
		t.Fatal("the authenticator derived an MSK for a peer it refused")
	}
	if err := sess.Err(); err == nil {
		t.Fatal("the authenticator refused the peer and recorded no cause")
	}

	// RFC 3748 Section 4.2: "After the authenticator sends a failure result
	// indication to the peer, regardless of the response from the peer, it MUST
	// subsequently send a Failure packet." The peer's acknowledgement of the
	// Failure packet draws that EAP-Failure and nothing else.
	ack := &Packet{Code: CodeResponse, Identifier: verdict.Identifier, Type: TypeMSCHAPv2, TypeData: []byte{mschapv2OpFailure, verdict.TypeData[1], 0, 4}}
	last := sess.Process(ack)
	if last == nil {
		t.Fatal("the authenticator owed an EAP-Failure and sent nothing")
	}
	if last.Code != CodeFailure {
		t.Fatalf("the packet after the Failure carries Code %d, want %d (EAP-Failure)", last.Code, CodeFailure)
	}
	if sess.Succeeded() {
		t.Fatal("the authenticator reports success after sending EAP-Failure")
	}
	if next := sess.Process(ack); next != nil {
		t.Fatalf("the ended exchange answered a further Response with Code %d", next.Code)
	}
}

// TestRFC2759AuthenticatorAcceptsMatchingNTResponse is the other half of the
// pair: the Failure packet MUST follow a mismatch and MUST NOT follow a match.
//
// RFC 2759 Section 5 gives the answer a verified Response earns: a Success
// packet whose Message field "contains a 42-octet authenticator response string
// and a printable message".
//
// RFC requirement: RFC2759-x-12 negative -- an NT-Response the authenticator
// verifies produces the Success packet and no E=691 Failure, and the session is
// not terminated (eap_mschapv2.go handleResponse).
func TestRFC2759AuthenticatorAcceptsMatchingNTResponse(t *testing.T) {
	sess, verdict := mschapv2Refusal(t, "TestPassword", "TestPassword")

	if got := verdict.TypeData[0]; got != mschapv2OpSuccess {
		t.Fatalf("verdict OpCode = %d, want %d (Success)", got, mschapv2OpSuccess)
	}
	if strings.Contains(string(verdict.TypeData[4:]), "E=691") {
		t.Fatalf("a verified NT-Response was answered with an E=691 Failure: %q", string(verdict.TypeData[4:]))
	}
	if err := sess.Err(); err != nil {
		t.Fatalf("the authenticator recorded a failure for a verified NT-Response: %v", err)
	}
}

// TestRFC2759FailureCarriesFreshChallenge checks the C= field of the Failure
// packet Ze sends: present, exactly 32 uppercase hexadecimal digits, and a fresh
// value on each refusal. It then feeds that packet to Ze's own peer, which MUST
// read it rather than reject the OpCode.
//
// RFC 2759 Section 6: "The "cccccccccccccccccccccccccccccccc" is the ASCII
// representation of a hexadecimal challenge value.  This field MUST be exactly 32
// octets long and MUST be present."
//
// RFC requirement: RFC2759-x-6 positive -- the Failure packet carries a C= field
// of exactly 32 uppercase hexadecimal digits, drawn fresh for each refusal
// (eap_mschapv2.go sendFailure).
func TestRFC2759FailureCarriesFreshChallenge(t *testing.T) {
	const uppercaseHex = "0123456789ABCDEF"

	seen := map[string]bool{}
	for round := range 2 {
		_, verdict := mschapv2Refusal(t, "WrongPassword", "TestPassword")
		if len(verdict.TypeData) <= 4 {
			t.Fatalf("round %d: the refusal carries no Failure Message: Code %d, %d octets of type data", round, verdict.Code, len(verdict.TypeData))
		}

		message := string(verdict.TypeData[4:])
		challenge, ok := failureField(message, "C=")
		if !ok {
			t.Fatalf("round %d: the Failure Message carries no C= challenge: %q", round, message)
		}
		if len(challenge) != 32 {
			t.Fatalf("round %d: C= challenge is %d octets, want exactly 32: %q", round, len(challenge), challenge)
		}
		for _, digit := range challenge {
			if !strings.ContainsRune(uppercaseHex, digit) {
				t.Fatalf("round %d: C= challenge holds %q, want uppercase hexadecimal digits: %q", round, digit, challenge)
			}
		}
		if seen[challenge] {
			t.Fatalf("round %d: C= challenge %q repeats, so it is not fresh", round, challenge)
		}
		seen[challenge] = true
	}

	// The peer half: a conformant Failure packet reaches the peer as the
	// authenticator's error code, not as an unreadable OpCode.
	peer, _ := mschapv2PeerAtSuccess(t)
	_, verdict := mschapv2Refusal(t, "WrongPassword", "TestPassword")
	if len(verdict.TypeData) <= 4 {
		t.Fatalf("the refusal carries no Failure packet for the peer to read: Code %d", verdict.Code)
	}

	res := peer.Process(&Packet{
		Code:       CodeRequest,
		Identifier: 4,
		Type:       TypeMSCHAPv2,
		TypeData:   verdict.TypeData,
	})
	if res.Err != nil {
		t.Fatalf("the peer refused a conformant Failure packet: %v", res.Err)
	}
	if res.Response == nil || res.Response.TypeData[0] != mschapv2OpFailure {
		t.Fatal("the peer did not acknowledge the Failure packet, so the authenticator has no round for the EAP-Failure it owes")
	}

	// The reason reaches the operator on the round after, riding with the
	// EAP-Failure that carries no field for one of its own.
	after := peer.Process(&Packet{Code: CodeFailure, Identifier: 5})
	var failure *mschapv2FailureError
	if !errors.As(after.Err, &failure) {
		t.Fatalf("the peer did not read the E= error code out of the Failure packet: %v", after.Err)
	}
	if failure.code != 691 {
		t.Fatalf("the peer read error code %d, want 691", failure.code)
	}
	if peer.Succeeded() {
		t.Fatal("the peer reports success after an MS-CHAPv2 Failure")
	}
}

// TestRFC2759PeerRefusesFailureWithoutConformantChallenge walks the Failure
// packets whose C= field breaks the mandate. Each MUST be named as a malformed
// challenge rather than read as an ordinary refusal.
//
// RFC 2759 Section 6: "This field MUST be exactly 32 octets long and MUST be
// present."
//
// The check is paired deliberately: Ze writes the field in sendFailure and reads
// it here, so a defect on one side has a second place to show itself. Case is
// not part of the mandate for C=, so a lowercase challenge from another
// authenticator is accepted; only Ze's own output is held to uppercase.
//
// RFC requirement: RFC2759-x-6 negative -- a Failure packet whose C= field is
// absent, short, long or not hexadecimal is refused as a malformed challenge,
// and the session ends (peer.go parseMSCHAPv2Failure).
func TestRFC2759PeerRefusesFailureWithoutConformantChallenge(t *testing.T) {
	const conformant = "E=691 R=0 C=" + "0123456789ABCDEF0123456789ABCDEF" + " V=3 M=Authentication failure"

	cases := []struct {
		name    string
		message string
	}{
		{"absent", "E=691 R=0 V=3 M=Authentication failure"},
		{"short", "E=691 R=0 C=0123456789ABCDEF0123456789ABCD V=3 M=Authentication failure"},
		{"long", "E=691 R=0 C=0123456789ABCDEF0123456789ABCDEF01 V=3 M=Authentication failure"},
		{"not-hexadecimal", "E=691 R=0 C=" + strings.Repeat("Z", 32) + " V=3 M=Authentication failure"},
		{"empty-message", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer, _ := mschapv2PeerAtSuccess(t)

			res := peer.Process(mschapv2Failure(7, tc.message))
			if !errors.Is(res.Err, errFailureChallenge) {
				t.Fatalf("a Failure packet with a %s C= field was not named as one: %v", tc.name, res.Err)
			}
			if res.Response != nil {
				t.Fatal("the peer acknowledged a Failure packet it could not read")
			}
			if peer.Succeeded() {
				t.Fatal("the peer reports success after a malformed Failure packet")
			}

			// The session ended: a later EAP-Success MUST NOT hand out the MSK.
			after := peer.Process(&Packet{Code: CodeSuccess, Identifier: 5})
			if after.Done {
				t.Fatal("the peer completed after a Failure packet")
			}
		})
	}

	// The conformant field is read as a refusal, which is what makes the four
	// cases above a discrimination rather than a blanket rejection.
	peer, _ := mschapv2PeerAtSuccess(t)
	if res := peer.Process(mschapv2Failure(7, conformant)); res.Err != nil {
		t.Fatalf("a conformant C= field was refused: %v", res.Err)
	}
	var failure *mschapv2FailureError
	if after := peer.Process(&Packet{Code: CodeFailure, Identifier: 5}); !errors.As(after.Err, &failure) {
		t.Fatalf("a conformant Failure packet was not read as a refusal: %v", after.Err)
	}
}
