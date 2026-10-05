// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP role admission.
package eap

import (
	"errors"
	"testing"
)

// TestRFC3748DiscardedCodesPreserveExchange inserts more than the peer's round
// limit before every leg of a real MS-CHAPv2 conversation. Snapshots cover the
// identifier, state, keys, method and budget, not just the lack of a reply.
// RFC requirement: RFC3748-4-5 positive -- undefined Codes leave both EAP sessions unchanged even when more than maxEAPRounds arrive between legitimate packets.
// RFC requirement: RFC3748-4-5 negative -- legitimate MS-CHAPv2 packets still complete the same exchange with matching nonzero MSKs after the discarded Codes.
// MUTATION: move the peer round counter before Code admission, or answer a wrong-role authenticator packet with failure.
func TestRFC3748DiscardedCodesPreserveExchange(t *testing.T) {
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(auth.Close)
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)
	request := auth.Begin()

	// Ten valid legs bound the test independently of the implementation cap.
	for range 10 {
		state, rounds, method := peer.state, peer.rounds, peer.method
		msk, emsk, committed := peer.msk, peer.emsk, peer.methodCommitted
		challenge, authChallenge, ntResponse := peer.peerChallenge, peer.authChallenge, peer.ntResponse
		for range maxEAPRounds + 1 {
			for _, code := range []uint8{0, 5, 255, CodeResponse} {
				// RFC 3748 Sections 2.2, 2.3 and 4: role and Code admission.
				got := peer.Process(&Packet{Code: code, Identifier: request.Identifier, Type: TypeMSCHAPv2})
				eapdWantDiscarded(t, got, "undefined or wrong-role Code")
				if peer.state != state || peer.rounds != rounds || peer.method != method {
					t.Fatalf("Code %d changed peer state, method or round budget", code)
				}
				if peer.msk != msk || peer.emsk != emsk || peer.methodCommitted != committed {
					t.Fatalf("Code %d changed peer keys or method commitment", code)
				}
				if peer.peerChallenge != challenge || peer.authChallenge != authChallenge || peer.ntResponse != ntResponse {
					t.Fatalf("Code %d changed MS-CHAPv2 conversation state", code)
				}
			}
		}

		// RFC 3748 Section 4: a defined Code addressed to the peer proceeds.
		answer := peer.Process(request)
		if answer.Err != nil {
			t.Fatalf("legitimate request after discards: %v", answer.Err)
		}
		if answer.Done {
			if !auth.Succeeded() || !peer.Succeeded() {
				t.Fatal("both roles must complete the authentication")
			}
			if answer.MSK == ([64]byte{}) || answer.MSK != auth.MSK() {
				t.Fatal("completed exchange must retain matching nonzero MSKs")
			}
			return
		}
		if answer.Response == nil {
			t.Fatal("legitimate request produced no response")
		}

		beforeAuth := *auth
		authMethod, ok := auth.method.(*mschapv2Method)
		if !ok {
			t.Fatal("exchange did not select MS-CHAPv2")
		}
		beforeMethod := *authMethod
		for range maxEAPRounds + 1 {
			for _, code := range []uint8{0, 5, 255, CodeRequest, CodeSuccess, CodeFailure} {
				// RFC 3748 Sections 2.2, 2.3 and 4: role and Code admission.
				got := auth.Process(&Packet{Code: code, Identifier: answer.Response.Identifier, Type: TypeMSCHAPv2})
				if got != nil {
					t.Fatalf("authenticator answered Code %d with %+v", code, got)
				}
				if *auth != beforeAuth || *authMethod != beforeMethod {
					t.Fatalf("Code %d changed authenticator state, identifier or method", code)
				}
			}
		}
		// RFC 3748 Section 4: a defined Response proceeds at the same Identifier.
		request = auth.Process(answer.Response)
		if request == nil {
			t.Fatal("legitimate response produced no next request")
		}
	}
	t.Fatal("exchange did not finish within ten valid legs")
}

// TestPeerRoundLimitStillApplies sends accepted Notification Requests up to the
// cap, with an ignored Code at that boundary, then checks the next valid round.
func TestPeerRoundLimitStillApplies(t *testing.T) {
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)
	for range maxEAPRounds {
		// RFC 3748 Section 5.2: every Notification Request must be answered.
		if got := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeNotification}); got.Response == nil {
			t.Fatalf("accepted round did not answer: %+v", got)
		}
	}
	// RFC 3748 Section 4: an undefined Code is ignored even at the budget limit.
	eapdWantDiscarded(t, peer.Process(&Packet{Code: 255}), "undefined Code at round limit")
	// RFC 3748 Section 5.2: another legitimate packet consumes the next round.
	if got := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeNotification}); !errors.Is(got.Err, ErrTooManyRounds) {
		t.Fatalf("accepted traffic escaped round cap: %+v", got)
	}
}
