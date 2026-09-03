// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework
// RFC: rfc/short/rfc3748.md -- Section 2.1: what an unexpected packet earns
//
// Every EAP packet in an IKE_AUTH exchange is unauthenticated, so anybody on the
// path can send one. RFC 3748 Section 2.1 answers that twice, and this file
// drives both answers plus the bound that keeps the second one cheap.
//
// The authenticator half: "Since spoofed EAP Request packets may be sent by an
// attacker, an authenticator receiving an unexpected Nak SHOULD discard it and
// log the event." A Nak arriving after the peer answered a method Request is
// exactly that Nak, and ending the exchange on it hands an attacker an
// EAP-Failure for one packet.
//
// The peer half: an authenticator "MUST NOT send a Request for an additional
// method of any Type after completion of the initial authentication method; a
// peer receiving such Requests MUST treat them as invalid, and silently discard
// them." A peer that reports an error instead hands the same attacker the IKE SA,
// because handleEAPResponse (internal/component/ike/engine/fsm.go) reads any
// non-nil PeerResult.Err as StateDead.
//
// The bound: the desired-Type octets of a Nak reach Session.Err and from there an
// operator's log line. wireEAPToPacket (internal/component/ike/engine/fsm.go)
// caps them nowhere, so the message is bounded here.
//
// VALIDATES: an unexpected Nak is discarded, recorded in Session.Err, and leaves
// the exchange free to complete; a Nak sent before the peer answered a method
// Request still ends the exchange with an EAP-Failure; a Request arriving after
// the peer's EAP-Success is discarded and keeps the peer's success; and the
// desired-Type list of a Nak is rendered at a fixed maximum length whatever the
// packet carries.
// PREVENTS: one spoofed Nak ending a live authentication; one spoofed Request
// killing an authenticated IKE SA; an unexpected Nak discarded with no diagnosis
// at all; and a log line whose size an attacker chooses.

package eap

import (
	"strings"
	"testing"
)

// spoofDrive is one EAP-MSCHAPv2 exchange stopped at a chosen point, holding the
// two objects a test still needs to act on.
type spoofDrive struct {
	auth *Session
	peer *PeerSession

	// outstanding is the authenticator's last EAP-Request, the one whose
	// Identifier a Response must echo for Session.Process to read it at all.
	outstanding *Packet
}

// spoofAuthAnsweredMethod drives the authenticator and the peer until the peer
// has answered a Request of the method's own Type, which is the "initial non-Nak
// Response" of RFC 3748 Section 2.1, and stops with the next Request outstanding.
func spoofAuthAnsweredMethod(t *testing.T) *spoofDrive {
	t.Helper()

	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)

	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	identity := peer.Process(auth.Begin())
	if identity.Response == nil {
		t.Fatalf("the Identity Request drew %+v, want an Identity Response", identity)
	}

	challenge := auth.Process(identity.Response)
	if challenge == nil || challenge.Type != TypeMSCHAPv2 {
		t.Fatalf("the Identity Response drew %+v, want the MS-CHAPv2 Challenge", challenge)
	}

	answer := peer.Process(challenge)
	if answer.Response == nil || answer.Response.Type != TypeMSCHAPv2 {
		t.Fatalf("the MS-CHAPv2 Challenge drew %+v, want an MS-CHAPv2 Response", answer)
	}

	next := auth.Process(answer.Response)
	if next == nil {
		t.Fatal("the MS-CHAPv2 Response drew no packet at all")
	}
	if next.Code != CodeRequest {
		t.Fatalf("the MS-CHAPv2 Response drew code %d, want a further Request; the exchange must still be open", next.Code)
	}
	if !auth.methodAnswered {
		t.Fatal("the authenticator read an MS-CHAPv2 Response without recording that the peer answered a method Request")
	}

	return &spoofDrive{auth: auth, peer: peer, outstanding: next}
}

// TestAuthenticatorDiscardsAnUnexpectedNak drives the same Nak on each side of
// the boundary RFC 3748 Section 2.1 draws, and then completes the exchange the
// unexpected one tried to end.
//
// The completion is what makes the discard a discard rather than a quieter
// failure. A session that answered nothing but had moved to its failure state
// would satisfy a nil return and nothing else in this test.
func TestAuthenticatorDiscardsAnUnexpectedNak(t *testing.T) {
	drive := spoofAuthAnsweredMethod(t)

	spoofed := &Packet{
		Code:       CodeResponse,
		Identifier: drive.outstanding.Identifier,
		Type:       TypeNAK,
		TypeData:   []byte{TypeTLS, 5},
	}
	if out := drive.auth.Process(spoofed); out != nil {
		t.Fatalf("the unexpected Nak drew code %d; RFC 3748 Section 2.1 asks the authenticator to discard it, and an EAP-Failure lets one spoofed packet end the exchange", out.Code)
	}

	reason := drive.auth.Err()
	if reason == nil {
		t.Fatal("the unexpected Nak was discarded with no diagnosis; Section 2.1 asks the authenticator to log the event")
	}
	if !strings.Contains(reason.Error(), "discarded") {
		t.Fatalf("Err() = %q, which does not say the packet was discarded", reason)
	}
	if !strings.Contains(reason.Error(), "13, 5") {
		t.Fatalf("Err() = %q, which does not name the Types 13 and 5 the Nak asked for", reason)
	}

	// The exchange the spoofed packet tried to end still completes. The peer
	// answers the Request that was outstanding when the Nak arrived, exactly as a
	// peer that never saw the Nak would.
	ack := drive.peer.Process(drive.outstanding)
	if ack.Response == nil {
		t.Fatalf("the outstanding Request drew %+v after the spoofed Nak, want the peer's Response", ack)
	}
	final := drive.auth.Process(ack.Response)
	if final == nil || final.Code != CodeSuccess {
		t.Fatalf("the exchange ended with %+v, want an EAP-Success; the spoofed Nak was not discarded", final)
	}
	if !drive.auth.Succeeded() {
		t.Fatal("the authenticator does not report success after an exchange that reached EAP-Success")
	}

	// The boundary case, driven on the same authenticator configuration: a Nak
	// arriving BEFORE the peer has answered a method Request is the legitimate
	// refusal of Section 5.3.1 and still ends the exchange with an EAP-Failure.
	// Without this arm, an authenticator that discarded every Nak would pass.
	early, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(early.Close)

	req := early.Begin()
	out := early.Process(&Packet{
		Code:       CodeResponse,
		Identifier: req.Identifier,
		Type:       TypeNAK,
		TypeData:   []byte{TypeTLS, 5},
	})
	if out == nil || out.Code != CodeFailure {
		t.Fatalf("a Nak answering the Identity Request drew %+v, want an EAP-Failure", out)
	}
}

// TestPeerDiscardsARequestAfterTheExchangeCompleted drives a Request at a peer
// that has already read its EAP-Success.
//
// Three Types are driven, so the discard is the terminal state acting rather than
// one Type's own arm: a method Request, a Notification Request that would
// otherwise draw a Response, and an Identity Requery.
func TestPeerDiscardsARequestAfterTheExchangeCompleted(t *testing.T) {
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)

	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	req := auth.Begin()
	for round := range 8 {
		res := peer.Process(req)
		if res.Response == nil {
			t.Fatalf("peer round %d drew %+v, want a Response", round, res)
		}
		next := auth.Process(res.Response)
		if next == nil {
			t.Fatalf("authenticator round %d returned no packet", round)
		}
		if next.Code == CodeSuccess {
			if done := peer.Process(next); !done.Done {
				t.Fatalf("the peer answered the EAP-Success with %+v, want Done", done)
			}
			break
		}
		req = next
	}
	if !peer.Succeeded() {
		t.Fatal("the EAP-MSCHAPv2 exchange did not reach the peer's success state")
	}

	late := []*Packet{
		{Code: CodeRequest, Identifier: 30, Type: TypeMD5Challenge, TypeData: append([]byte{16}, make([]byte, 16)...)},
		{Code: CodeRequest, Identifier: 31, Type: TypeNotification, TypeData: []byte("notice")},
		{Code: CodeRequest, Identifier: 32, Type: TypeIdentity},
	}
	for _, request := range late {
		res := peer.Process(request)
		if res.Err != nil {
			t.Fatalf("a Type-%d Request after the EAP-Success drew %v; handleEAPResponse reads any Err as StateDead, so one unauthenticated packet would kill the IKE SA", request.Type, res.Err)
		}
		if !res.Discarded {
			t.Fatalf("a Type-%d Request after the EAP-Success drew %+v, want a silent discard", request.Type, res)
		}
		if res.Response != nil {
			t.Fatalf("a Type-%d Request after the EAP-Success drew a Response of Type %d; the RFC asks for silence", request.Type, res.Response.Type)
		}
		if !peer.Succeeded() {
			t.Fatalf("a Type-%d Request after the EAP-Success unwound the peer's success", request.Type)
		}
	}
}

// nakFlood builds a Nak Type-Data field of the requested length, filled with
// authentication Types so that no octet is the zero Section 5.3.1 gives its own
// meaning.
func nakFlood(octets int) []byte {
	flood := make([]byte, octets)
	for i := range flood {
		flood[i] = typeAuthenticationLow + uint8(i%4)
	}
	return flood
}

// desiredTypesReported returns the rendered desired-Type list an error message
// carries, with the truncation marker stripped, and whether it was marked
// truncated.
func desiredTypesReported(t *testing.T, message string) (types []string, truncated bool) {
	t.Helper()

	const marker = "asking for type "
	_, list, found := strings.Cut(message, marker)
	if !found {
		t.Fatalf("the message %q names no desired-Type list", message)
	}

	// The list runs to the end of the sentence. Only nakUnexpected writes
	// anything after it, and it opens that continuation with a comma-free clause
	// starting at " after".
	if end := strings.Index(list, " after"); end >= 0 {
		list = list[:end]
	}
	list, truncated = strings.CutSuffix(list, " (truncated)")
	return strings.Split(list, ", "), truncated
}

// TestNakDesiredTypeListIsBounded floods both producers of the desired-Type list
// with a Nak far larger than any EAP packet the RFC provides for.
//
// The assertion that matters is the one comparing two floods of different sizes:
// a message whose length does not move when the packet doubles is bounded, and no
// single-packet assertion can say that.
func TestNakDesiredTypeListIsBounded(t *testing.T) {
	refusal := func(t *testing.T, octets int) error {
		t.Helper()

		auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
		if err != nil {
			t.Fatalf("NewSession: %v", err)
		}
		t.Cleanup(auth.Close)

		req := auth.Begin()
		if out := auth.Process(&Packet{Code: CodeResponse, Identifier: req.Identifier, Type: TypeNAK, TypeData: nakFlood(octets)}); out == nil {
			t.Fatal("the Nak drew no packet at all")
		}
		reason := auth.Err()
		if reason == nil {
			t.Fatal("the Nak was recorded with no reason")
		}
		return reason
	}

	small := refusal(t, 40_000)
	large := refusal(t, 80_000)
	if len(small.Error()) != len(large.Error()) {
		t.Fatalf("a 40000-octet Nak reported %d characters and an 80000-octet Nak reported %d; the message grows with the packet, so an attacker chooses the size of the log line",
			len(small.Error()), len(large.Error()))
	}

	types, truncated := desiredTypesReported(t, small.Error())
	if !truncated {
		t.Fatalf("Err() = %q, which does not say the desired-Type list was truncated", small)
	}
	if len(types) != desiredTypeMax {
		t.Fatalf("the message named %d desired Types, want %d (RFC 3748 Section 5.3.1 numbers the authentication Types 4 and above, so 252 octets name every one of them)", len(types), desiredTypeMax)
	}

	// The unexpected-Nak path renders the same list and is bounded with it.
	drive := spoofAuthAnsweredMethod(t)
	if out := drive.auth.Process(&Packet{
		Code:       CodeResponse,
		Identifier: drive.outstanding.Identifier,
		Type:       TypeNAK,
		TypeData:   nakFlood(40_000),
	}); out != nil {
		t.Fatalf("the unexpected Nak drew code %d, want a discard", out.Code)
	}
	unexpected := drive.auth.Err()
	if unexpected == nil {
		t.Fatal("the unexpected Nak was discarded with no diagnosis")
	}
	if len(unexpected.Error()) > len(small.Error())+200 {
		t.Fatalf("the unexpected-Nak message is %d characters against the refusal's %d, so its desired-Type list is unbounded",
			len(unexpected.Error()), len(small.Error()))
	}
	unexpectedTypes, unexpectedTruncated := desiredTypesReported(t, unexpected.Error())
	if !unexpectedTruncated {
		t.Fatalf("Err() = %q, which does not say the desired-Type list was truncated", unexpected)
	}
	if len(unexpectedTypes) != desiredTypeMax {
		t.Fatalf("the unexpected-Nak message named %d desired Types, want %d", len(unexpectedTypes), desiredTypeMax)
	}

	// A Nak that fits the bound is reported whole and is never marked truncated,
	// so the cap reads the length rather than always cutting.
	short, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(short.Close)

	req := short.Begin()
	if out := short.Process(&Packet{Code: CodeResponse, Identifier: req.Identifier, Type: TypeNAK, TypeData: []byte{TypeTLS, 5, 6}}); out == nil {
		t.Fatal("the Nak drew no packet at all")
	}
	shortTypes, shortTruncated := desiredTypesReported(t, short.Err().Error())
	if shortTruncated {
		t.Fatalf("Err() = %q marks a three-octet Nak truncated", short.Err())
	}
	if len(shortTypes) != 3 {
		t.Fatalf("a three-octet Nak reported %d desired Types, want 3", len(shortTypes))
	}
}
