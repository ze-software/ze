// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework (RFC 3748)
// RFC: rfc/short/rfc3748.md -- the Section 4.2 result-indication rows
//
// RFC 3748 Section 4.2 states when each terminal packet is owed and when it is
// forbidden. Six obligations are proven here, and all six are driven through one
// EAP-MSCHAPv2 conversation between the real authenticator and the real peer,
// because the method's result indications are what the section turns on: the
// MS-CHAPv2 Success packet is the authenticator's success indication, the peer's
// Success acknowledgement is the peer's, and the MS-CHAPv2 Failure packet is the
// authenticator's failure indication (RFC 2759 Sections 5 and 6).
//
// VALIDATES: no terminal packet leaves mid-method; a failure indication is
// followed by an EAP-Failure whatever the peer answers it with; the two success
// indications are followed by an EAP-Success; a peer that failed to authenticate
// is refused and never granted a Success; a peer that never received its
// terminal packet neither concludes success nor loses its place; and a peer that
// has ended its session discards a Success that arrives afterwards.
// PREVENTS: an authenticator that answers a mid-method round with Success or
// Failure; a last word that is not followed by the EAP-Failure it owes; a
// Success sent on the authenticator's indication alone; an EAP-Success granted
// to a peer whose credentials were refused; a peer that treats a lost Success as
// authentication; and a rogue Success accepted after the peer refused.

package eap

import (
	"errors"
	"testing"
)

// mschapv2Exchange is one live EAP-MSCHAPv2 conversation stopped at the packet
// the authenticator sent in answer to the peer's MS-CHAPv2 Response: the success
// indication when the passwords agree, the failure indication when they do not.
type mschapv2Exchange struct {
	auth *Session
	peer *PeerSession

	// rounds holds every packet the authenticator sent, in order, up to and
	// including indication.
	rounds []*Packet

	// indication is the last of those: the method's result indication.
	indication *Packet
}

// openMSCHAPv2Exchange drives the Identity and Challenge rounds and stops on the
// authenticator's answer to the peer's MS-CHAPv2 Response.
func openMSCHAPv2Exchange(t *testing.T, authPassword, peerPassword string) *mschapv2Exchange {
	t.Helper()

	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: authPassword})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)

	peer := NewPeerSession(TypeMSCHAPv2, "user", peerPassword)
	t.Cleanup(peer.Close)

	ex := &mschapv2Exchange{auth: auth, peer: peer}

	identity := auth.Begin()
	ex.rounds = append(ex.rounds, identity)
	identityRes := peer.Process(identity)
	if identityRes.Err != nil || identityRes.Response == nil {
		t.Fatalf("the peer did not answer the Identity Request: %+v", identityRes)
	}

	challenge := auth.Process(identityRes.Response)
	if challenge == nil || challenge.Type != TypeMSCHAPv2 {
		t.Fatalf("the Identity Response drew %+v, want an MS-CHAPv2 Challenge", challenge)
	}
	ex.rounds = append(ex.rounds, challenge)
	challengeRes := peer.Process(challenge)
	if challengeRes.Err != nil || challengeRes.Response == nil {
		t.Fatalf("the peer did not answer the Challenge: %+v", challengeRes)
	}

	indication := auth.Process(challengeRes.Response)
	if indication == nil {
		t.Fatal("the MS-CHAPv2 Response drew no packet at all")
	}
	ex.rounds = append(ex.rounds, indication)
	ex.indication = indication
	return ex
}

// TestRFC3748NoTerminalPacketLeavesMidMethod inspects every packet an
// authenticator sends before its method is entitled to finish, then the one it
// sends when the method is.
func TestRFC3748NoTerminalPacketLeavesMidMethod(t *testing.T) {
	ex := openMSCHAPv2Exchange(t, "secret", "secret")

	for i, p := range ex.rounds {
		if p.Code != CodeRequest {
			t.Fatalf("packet %d of the method conversation has Code %d, want a Request (%d)", i, p.Code, CodeRequest)
		}
	}
	if ex.indication.Type != TypeMSCHAPv2 {
		t.Fatalf("the answer to the MS-CHAPv2 Response has Type %d, want the method's own %d", ex.indication.Type, TypeMSCHAPv2)
	}

	ack := ex.peer.Process(ex.indication)
	if ack.Err != nil || ack.Response == nil {
		t.Fatalf("the peer did not acknowledge the success indication: %+v", ack)
	}
	terminal := ex.auth.Process(ack.Response)
	if terminal == nil || terminal.Code != CodeSuccess {
		t.Fatalf("the acknowledged success indication drew %+v, want an EAP-Success", terminal)
	}
}

// TestRFC3748FailureFollowsTheFailureIndication answers one failure indication
// two different ways and reads what the authenticator sends next each time.
func TestRFC3748FailureFollowsTheFailureIndication(t *testing.T) {
	acknowledged := openMSCHAPv2Exchange(t, "server-secret", "peer-secret")
	ack := acknowledged.peer.Process(acknowledged.indication)
	if ack.Response == nil {
		t.Fatalf("the peer did not answer the failure indication: %+v", ack)
	}
	if got := acknowledged.auth.Process(ack.Response); got == nil || got.Code != CodeFailure {
		t.Fatalf("the acknowledged failure indication drew %+v, want an EAP-Failure", got)
	}

	garbled := openMSCHAPv2Exchange(t, "server-secret", "peer-secret")
	nonsense := &Packet{
		Code:       CodeResponse,
		Identifier: garbled.indication.Identifier,
		Type:       TypeMSCHAPv2,
		TypeData:   []byte{0xFF, 0xFF, 0xFF, 0xFF},
	}
	if got := garbled.auth.Process(nonsense); got == nil || got.Code != CodeFailure {
		t.Fatalf("a failure indication answered with nonsense drew %+v, want an EAP-Failure", got)
	}

	succeeding := openMSCHAPv2Exchange(t, "secret", "secret")
	goodAck := succeeding.peer.Process(succeeding.indication)
	if goodAck.Response == nil {
		t.Fatalf("the peer did not acknowledge the success indication: %+v", goodAck)
	}
	if got := succeeding.auth.Process(goodAck.Response); got == nil || got.Code != CodeFailure {
		if got == nil || got.Code != CodeSuccess {
			t.Fatalf("an exchange with no failure indication drew %+v, want an EAP-Success", got)
		}
	} else {
		t.Fatal("an exchange with no failure indication ended in an EAP-Failure")
	}
}

// TestRFC3748SuccessFollowsBothSuccessIndications answers the authenticator's
// success indication with the peer's own, then with a refusal.
func TestRFC3748SuccessFollowsBothSuccessIndications(t *testing.T) {
	agreed := openMSCHAPv2Exchange(t, "secret", "secret")
	ack := agreed.peer.Process(agreed.indication)
	if ack.Err != nil || ack.Response == nil {
		t.Fatalf("the peer did not acknowledge the success indication: %+v", ack)
	}
	if len(ack.Response.TypeData) == 0 || ack.Response.TypeData[0] != mschapv2OpSuccess {
		t.Fatalf("the peer answered the success indication with OpCode %#x, want the Success OpCode %#x", ack.Response.TypeData, mschapv2OpSuccess)
	}
	terminal := agreed.auth.Process(ack.Response)
	if terminal == nil || terminal.Code != CodeSuccess {
		t.Fatalf("both success indications drew %+v, want an EAP-Success", terminal)
	}

	refused := openMSCHAPv2Exchange(t, "secret", "secret")
	peerRefusal := &Packet{
		Code:       CodeResponse,
		Identifier: refused.indication.Identifier,
		Type:       TypeMSCHAPv2,
		TypeData:   []byte{mschapv2OpFailure, 0, 0, 4},
	}
	if got := refused.auth.Process(peerRefusal); got == nil || got.Code == CodeSuccess {
		t.Fatalf("the authenticator answered a peer that refused its success indication with %+v", got)
	}
}

// TestRFC3748FailedAuthenticationIsRefusedNotGranted drives one conversation the
// peer cannot pass and one it can, and reads every packet of each.
func TestRFC3748FailedAuthenticationIsRefusedNotGranted(t *testing.T) {
	failed := driveMSCHAPv2Flight(t, "server-secret", "peer-secret")
	if failed.terminal == nil || failed.terminal.Code != CodeFailure {
		t.Fatalf("a peer that could not authenticate drew %+v, want an EAP-Failure", failed.terminal)
	}
	for i, p := range failed.requests {
		if p.Code == CodeSuccess {
			t.Fatalf("packet %d of a failed conversation is an EAP-Success", i)
		}
	}

	passed := driveMSCHAPv2Flight(t, "secret", "secret")
	if passed.terminal == nil || passed.terminal.Code != CodeSuccess {
		t.Fatalf("a peer that authenticated drew %+v, want an EAP-Success", passed.terminal)
	}
}

// TestRFC3748PeerOutlivesALostTerminalPacket completes a method, drops the
// terminal packet, and delivers it late.
func TestRFC3748PeerOutlivesALostTerminalPacket(t *testing.T) {
	ex := openMSCHAPv2Exchange(t, "secret", "secret")
	ack := ex.peer.Process(ex.indication)
	if ack.Err != nil || ack.Response == nil {
		t.Fatalf("the peer did not acknowledge the success indication: %+v", ack)
	}
	terminal := ex.auth.Process(ack.Response)
	if terminal == nil || terminal.Code != CodeSuccess {
		t.Fatalf("the acknowledged success indication drew %+v, want an EAP-Success", terminal)
	}

	if ex.peer.Succeeded() {
		t.Fatal("the peer concluded success before the EAP-Success reached it")
	}

	// The EAP-Success is dropped on the path. Two packets arrive in its place and
	// are discarded, which is the interval the loss opens.
	for _, stray := range []*Packet{
		{Code: 0x7F, Identifier: terminal.Identifier},
		{Code: 0x7F, Identifier: terminal.Identifier},
	} {
		res := ex.peer.Process(stray)
		if !res.Discarded {
			t.Fatalf("a packet with an undefined Code drew %+v, want a discard", res)
		}
		if res.Err != nil {
			t.Fatalf("a packet arriving while the terminal packet was lost ended the session: %v", res.Err)
		}
	}
	if ex.peer.Succeeded() {
		t.Fatal("the peer concluded success from the packets that arrived in place of the EAP-Success")
	}

	late := ex.peer.Process(terminal)
	if late.Err != nil {
		t.Fatalf("the retransmitted EAP-Success was refused: %v", late.Err)
	}
	if !late.Done {
		t.Fatalf("the retransmitted EAP-Success drew %+v, want a completed exchange", late)
	}
	var zero [64]byte
	if late.MSK == zero {
		t.Fatal("the completed exchange handed out an all-zero MSK")
	}
}

// TestRFC3748PeerDiscardsASuccessAfterItEndedTheSession refuses one exchange,
// then sends the peer a Success, and compares that with a peer whose method
// completed.
func TestRFC3748PeerDiscardsASuccessAfterItEndedTheSession(t *testing.T) {
	ended := openMSCHAPv2Exchange(t, "secret", "secret")
	failure := &Packet{Code: CodeFailure, Identifier: ended.indication.Identifier}
	if res := ended.peer.Process(failure); !errors.Is(res.Err, ErrEAPFailure) {
		t.Fatalf("the EAP-Failure drew %+v, want the session to end", res)
	}

	rogue := &Packet{Code: CodeSuccess, Identifier: ended.indication.Identifier}
	res := ended.peer.Process(rogue)
	if !res.Discarded {
		t.Fatalf("a Success arriving after the peer ended the session drew %+v, want a discard", res)
	}
	if res.Done {
		t.Fatal("a Success arriving after the peer ended the session completed the exchange")
	}
	var zero [64]byte
	if res.MSK != zero {
		t.Fatal("a Success arriving after the peer ended the session handed out an MSK")
	}
	if ended.peer.Succeeded() {
		t.Fatal("the peer reports success after it ended the session")
	}

	completed := driveMSCHAPv2Flight(t, "secret", "secret")
	if !completed.peerFinal.Done {
		t.Fatalf("a peer whose method completed refused the EAP-Success: %+v", completed.peerFinal)
	}
	if completed.peerFinal.Discarded {
		t.Fatal("a peer whose method completed discarded the EAP-Success it was waiting for")
	}
}
