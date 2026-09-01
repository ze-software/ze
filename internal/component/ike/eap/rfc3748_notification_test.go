// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework (RFC 3748)
// RFC: rfc/short/rfc3748.md -- Section 5.2: the Notification Type
//
// One duty, read three ways. RFC 3748 Section 5.2 obliges the peer to ANSWER a
// Type-2 Request, obliges it to answer with a Notification Response and never a
// Nak, and says the Notification "is not an error indication, and therefore does
// not change the state of the peer".
//
// The packet arrives unauthenticated, before any key exists, and its Type-Data is
// chosen by whoever sent it. So the tests drive the shapes an attacker sends
// rather than the shape a well-behaved authenticator sends: a zero-length
// message, a message that is not UTF-8, a message past the RFC's 1015-octet
// budget, one injected in the middle of a live method, and twenty-one of them in
// a row.
//
// VALIDATES: a Type-2 Request draws a five-octet Type-2 Response carrying the
// Request's Identifier and no Type-Data; the message reaches the caller through
// PeerResult; a malformed message draws the same Response and never a Nak; the
// peer still Naks an unacceptable authentication Type, so the absence of a Nak
// for Type 2 is Section 5.2 acting; a Notification injected mid-method leaves
// every method field untouched and the exchange still reaches its MSK; and a
// Notification Request costs a round like any other.
// PREVENTS: a peer that errors on a Type-2 Request and kills the IKE SA with it;
// a Response carrying Type-Data the RFC says is zero octets long; a Nak sent in
// answer to a Notification; a Notification that overwrites the MS-CHAPv2
// challenge state under a running method; a message logged at whatever length the
// sender chose; and a Notification flood that never ends the exchange.

package eap

import (
	"errors"
	"strings"
	"testing"
)

// notificationRequest frames the Type-2 Request an authenticator sends.
func notificationRequest(id uint8, message string) *Packet {
	return &Packet{
		Code:       CodeRequest,
		Identifier: id,
		Type:       TypeNotification,
		TypeData:   []byte(message),
	}
}

// wantNotificationResponse fails unless the result is the Notification Response
// RFC 3748 Section 5.2 specifies: Code 2, Type 2, the Request's Identifier, and
// Type-Data of zero octets, which puts the whole packet at five octets on the
// wire.
func wantNotificationResponse(t *testing.T, res PeerResult, id uint8, what string) {
	t.Helper()

	if res.Err != nil {
		t.Fatalf("%s: the peer reported %v; a Notification Request is answered, not refused", what, res.Err)
	}
	if res.Response == nil {
		t.Fatalf("%s: the peer sent no Response at all", what)
	}
	if res.Response.Code != CodeResponse {
		t.Fatalf("%s: Response Code = %d, want %d", what, res.Response.Code, CodeResponse)
	}
	// A Type of 2 is also what rules the Nak out: RFC 3748 Section 5.2 forbids a
	// Type-3 Response here, and this is the assertion that catches one.
	if res.Response.Type != TypeNotification {
		t.Fatalf("%s: Response Type = %d, want %d (Notification); Type %d would be the Nak Section 5.2 forbids",
			what, res.Response.Type, TypeNotification, TypeNAK)
	}
	if res.Response.Identifier != id {
		t.Fatalf("%s: Response Identifier = %d, want %d", what, res.Response.Identifier, id)
	}
	if len(res.Response.TypeData) != 0 {
		t.Fatalf("%s: Response Type-Data is %d octets, want zero", what, len(res.Response.TypeData))
	}
	if got := len(res.Response.Encode()); got != 5 {
		t.Fatalf("%s: the encoded Response is %d octets, want 5", what, got)
	}
	if !res.Notified {
		t.Fatalf("%s: the peer did not report a Notification, so the message never reaches the operator", what)
	}
}

// TestPeerAnswersANotificationRequest hands a peer in the identity state the
// Notification Request an authenticator sends before any method starts, and then
// one whose message is longer than the RFC's own budget.
func TestPeerAnswersANotificationRequest(t *testing.T) {
	const message = "your password expires in 3 days"

	// RFC requirement: RFC3748-5.2-1 positive -- RFC 3748 Section 5.2: "The peer
	// MUST respond to a Notification Request with a Notification Response unless
	// the EAP authentication method specification prohibits the use of
	// Notification messages", and "A Response MUST be sent in reply to the Request
	// with a Type field of 2 (Notification).  The Type-Data field of the Response
	// is zero octets in length." The peer answers Code 2, Type 2, the Request's
	// own Identifier and no Type-Data, which is five octets encoded, and it hands
	// the displayable message to its caller through PeerResult.
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	res := peer.Process(notificationRequest(0x2A, message))
	wantNotificationResponse(t, res, 0x2A, "a Notification Request in the identity state")
	if res.Notification != message {
		t.Fatalf("Notification = %q, want %q", res.Notification, message)
	}

	// The Notification changed nothing: the peer is still in the identity state,
	// so the Identity Request that follows draws the Identity Response it would
	// have drawn without it. RFC 3748 Section 5.2: the Notification "is not an
	// error indication, and therefore does not change the state of the peer".
	identity := peer.Process(&Packet{Code: CodeRequest, Identifier: 0x2B, Type: TypeIdentity})
	if identity.Response == nil || identity.Response.Type != TypeIdentity {
		t.Fatalf("the Identity Request after a Notification drew %+v, want an Identity Response", identity)
	}

	// A message past the RFC's budget is carried at the budget. RFC 3748 Section
	// 5.2: "Note that the default maximum length of a Notification Request is 1020
	// octets.  By default, this leaves at most 1015 octets for the human readable
	// message." The Response stays five octets whatever the message length.
	long := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(long.Close)

	oversize := long.Process(notificationRequest(7, strings.Repeat("A", 4000)))
	wantNotificationResponse(t, oversize, 7, "a Notification Request carrying a 4000-octet message")
	if len(oversize.Notification) != notificationMax {
		t.Fatalf("the reported message is %d octets, want %d", len(oversize.Notification), notificationMax)
	}
}

// TestPeerNeverNaksANotificationRequest drives the two message shapes the RFC's
// own description rules out: one of zero octets, where it asks for "a displayable
// message greater than zero octets in length", and one that is not the UTF-8 it
// asks for.
//
// Both are malformed, and neither excuses the peer from answering. A peer that
// validated the message before answering would refuse exactly the packets an
// attacker sends, and would refuse them with the one Response the section
// forbids.
func TestPeerNeverNaksANotificationRequest(t *testing.T) {
	cases := []struct {
		name    string
		message []byte
	}{
		{name: "a zero-length message", message: nil},
		{name: "a message that is not UTF-8", message: []byte{0xFF, 0xFE, 0x80, 0x00, 0xC3}},
	}

	// RFC requirement: RFC3748-5.2-1 negative -- the Response is not conditional on
	// the message being displayable. A Notification Request carrying zero octets,
	// and one carrying octets that are not UTF-8, each still draw the Type-2
	// Response with zero-octet Type-Data, so no message shape makes the peer
	// withhold the answer Section 5.2 obliges.
	//
	// RFC requirement: RFC3748-5.2-2 positive -- RFC 3748 Section 5.2: "In any
	// case, a Nak Response MUST NOT be sent in response to a Notification
	// Request." Neither malformed message draws a Type-3 Nak: the Response Type is
	// 2 in both cases, and the peer reports the message it was given.
	for i, tc := range cases {
		peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
		t.Cleanup(peer.Close)

		id := uint8(0x10 + i)
		res := peer.Process(&Packet{Code: CodeRequest, Identifier: id, Type: TypeNotification, TypeData: tc.message})
		wantNotificationResponse(t, res, id, tc.name)
		if res.Notification != string(tc.message) {
			t.Fatalf("%s: Notification = %q, want %q", tc.name, res.Notification, tc.message)
		}
		if res.Discarded {
			t.Fatalf("%s: the peer discarded a Notification Request", tc.name)
		}
	}
}

// TestPeerNaksAnAuthenticationTypeButNotANotification puts the two Types side by
// side on one peer.
//
// Without the first half, the absence of a Nak for Type 2 proves nothing: a peer
// with no Nak path at all would pass every assertion in this file.
func TestPeerNaksAnAuthenticationTypeButNotANotification(t *testing.T) {
	// RFC requirement: RFC3748-5.2-2 negative -- the peer DOES send a Type-3 Nak,
	// for a Request of an unacceptable authentication Type (40), from the same
	// state in which a Type-2 Request draws a Type-2 Response. So the Nak that
	// never appears for a Notification is Section 5.2 acting rather than a peer
	// that cannot compose a Nak.
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	nak := peer.Process(&Packet{Code: CodeRequest, Identifier: 3, Type: 40})
	if nak.Response == nil {
		t.Fatalf("a Request of Type 40 drew %+v, want a legacy Nak", nak)
	}
	if nak.Response.Type != TypeNAK {
		t.Fatalf("a Request of Type 40 drew Type %d, want %d (legacy Nak)", nak.Response.Type, TypeNAK)
	}

	notified := peer.Process(notificationRequest(4, "the tunnel closes at 18:00"))
	wantNotificationResponse(t, notified, 4, "a Notification Request on a peer that just Nak'd")
}

// notificationFlight records one driven EAP-MSCHAPv2 exchange: what the peer made
// of the terminal packet, and the key the authenticator derived for comparison.
type notificationFlight struct {
	peerFinal PeerResult
	authMSK   [64]byte
}

// driveMSCHAPv2WithNotification runs a real EAP-MSCHAPv2 exchange between
// PeerSession and Session, injecting a Notification Request before the round
// named by notifyBefore. A negative value injects none, which is the
// uninterrupted run this test compares against.
//
// The loop is bounded at eight rounds. EAP-MSCHAPv2 needs three, so the bound
// guards a state machine that stops advancing rather than limiting the method.
func driveMSCHAPv2WithNotification(t *testing.T, notifyBefore int) *notificationFlight {
	t.Helper()

	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)

	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	req := auth.Begin()
	for round := range 8 {
		if round == notifyBefore {
			injectNotification(t, peer, round)
		}

		pres := peer.Process(req)
		if pres.Err != nil {
			t.Fatalf("peer round %d: %v", round, pres.Err)
		}
		if pres.Response == nil {
			t.Fatalf("peer round %d produced no Response", round)
		}

		next := auth.Process(pres.Response)
		if next == nil {
			t.Fatalf("authenticator round %d returned nil, so the exchange ended with no terminal packet", round)
		}
		if next.Code == CodeSuccess || next.Code == CodeFailure {
			if next.Code != CodeSuccess {
				t.Fatalf("the authenticator ended the exchange with code %d, want %d", next.Code, CodeSuccess)
			}
			return &notificationFlight{peerFinal: peer.Process(next), authMSK: auth.MSK()}
		}
		req = next
	}

	t.Fatal("the EAP-MSCHAPv2 exchange did not terminate within eight rounds")
	return nil
}

// injectNotification hands the peer a Notification Request in the middle of a
// live method and asserts that every field the method owns came through
// unchanged.
//
// The fields are read directly rather than inferred from the exchange
// continuing, because MS-CHAPv2 recomputes the Authenticator Response from the
// peer challenge and the NT-Response held across rounds (handleMSCHAPv2Success,
// peer.go): a Notification that overwrote either would fail the exchange one
// round later, at a call site with nothing to say about why.
func injectNotification(t *testing.T, peer *PeerSession, round int) {
	t.Helper()

	state, committed := peer.state, peer.methodCommitted
	peerChallenge, authChallenge, ntResponse, msk := peer.peerChallenge, peer.authChallenge, peer.ntResponse, peer.msk

	res := peer.Process(notificationRequest(0xE0, "scheduled maintenance at 23:00"))
	wantNotificationResponse(t, res, 0xE0, "a Notification Request injected mid-method")

	if peer.state != state {
		t.Fatalf("round %d: the Notification moved the peer from state %d to state %d", round, state, peer.state)
	}
	if peer.methodCommitted != committed {
		t.Fatalf("round %d: the Notification changed methodCommitted from %v to %v", round, committed, peer.methodCommitted)
	}
	if peer.peerChallenge != peerChallenge {
		t.Fatalf("round %d: the Notification changed the peer challenge", round)
	}
	if peer.authChallenge != authChallenge {
		t.Fatalf("round %d: the Notification changed the authenticator challenge", round)
	}
	if peer.ntResponse != ntResponse {
		t.Fatalf("round %d: the Notification changed the NT-Response", round)
	}
	if peer.msk != msk {
		t.Fatalf("round %d: the Notification changed the MSK", round)
	}
}

// TestNotificationMidMethodLeavesTheMethodUntouched injects a Notification
// Request between the rounds of a live EAP-MSCHAPv2 exchange, once before each of
// the method's own rounds, and drives every exchange to its terminal packet.
//
// The MSK is compared against the AUTHENTICATOR's, not against the
// uninterrupted run's: the peer challenge is drawn from crypto/rand for each
// exchange (handleMSCHAPv2Challenge, peer.go), so two runs derive two different
// keys by design. What "the same MSK as an uninterrupted run" means here is the
// property that makes the exchange useful, and it is the property a corrupted
// method state destroys: both ends hold the same 64 octets at the end.
func TestNotificationMidMethodLeavesTheMethodUntouched(t *testing.T) {
	clean := driveMSCHAPv2WithNotification(t, -1)
	if !clean.peerFinal.Done {
		t.Fatalf("the uninterrupted exchange did not conclude: %+v", clean.peerFinal)
	}
	if clean.peerFinal.MSK == ([64]byte{}) {
		t.Fatal("the uninterrupted exchange concluded with an all-zero MSK")
	}
	if clean.peerFinal.MSK != clean.authMSK {
		t.Fatal("the uninterrupted exchange left the two ends holding different MSKs")
	}

	// Round 0 is the Identity Request, round 1 the MS-CHAPv2 Challenge, round 2
	// the MS-CHAPv2 Success. A Notification before each one covers the identity
	// state, the state where the method holds a challenge and an NT-Response, and
	// the round that verifies the Authenticator Response against both.
	for round := range 3 {
		fl := driveMSCHAPv2WithNotification(t, round)
		if !fl.peerFinal.Done {
			t.Fatalf("a Notification before round %d stopped the exchange concluding: %+v", round, fl.peerFinal)
		}
		if fl.peerFinal.MSK == ([64]byte{}) {
			t.Fatalf("a Notification before round %d left the peer with an all-zero MSK", round)
		}
		if fl.peerFinal.MSK != fl.authMSK {
			t.Fatalf("a Notification before round %d left the two ends holding different MSKs", round)
		}
	}
}

// TestNotificationRequestsCountAgainstTheRoundCap floods the peer with the one
// packet an attacker can send for free.
//
// A Notification Request is unauthenticated, needs no state on the sender, and
// changes no state on the peer, so nothing inside Section 5.2 ends a conversation
// made of them. PeerSession.Process counting every Request against maxEAPRounds
// is what does (R-1).
func TestNotificationRequestsCountAgainstTheRoundCap(t *testing.T) {
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	for round := 1; round <= maxEAPRounds; round++ {
		res := peer.Process(notificationRequest(uint8(round), "notice"))
		wantNotificationResponse(t, res, uint8(round), "a Notification Request inside the round cap")
	}

	over := peer.Process(notificationRequest(maxEAPRounds+1, "notice"))
	if !errors.Is(over.Err, ErrTooManyRounds) {
		t.Fatalf("Notification Request %d gave %v, want ErrTooManyRounds", maxEAPRounds+1, over.Err)
	}
	if over.Response != nil {
		t.Fatalf("the refused round still sent a Response of Type %d", over.Response.Type)
	}
}
