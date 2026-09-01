// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework (RFC 3748)
// RFC: rfc/short/rfc3748.md -- the rows the 2026-08-30 extraction walk added
//
// The extraction sign-off walk over rfc/full/rfc3748.txt read all 103 normative
// sentences of RFC 3748 and found ten obligations that ze already meets and that
// rfc/short/rfc3748.md did not yet declare. rfc3748_test.go proves the rows the
// summary already carried. This file proves only the ten the walk added, so a
// reader can tell what the walk changed from what preceded it.
//
// Both roles ze plays are driven against each other with real credentials: the
// authenticator through Session (Begin and Process) and the peer through
// PeerSession (Process). EAP-TLS uses the handshake harness in
// eap_tls_handshake_test.go.
//
// VALIDATES: octets past the Length field never reach the method; every new
// Request changes the Identifier; a Response echoes the outstanding Request's
// Identifier, and carries the Request's own Type or the legacy Nak; a peer whose
// method fails ends the conversation; a waiting peer acts on Success and on
// Failure rather than dropping either; no Request ever carries a NAK type; the
// Identity Response is not null terminated; EAP-TLS rejects a modified packet;
// and a key-deriving method authenticates both ends.
// PREVENTS: a decoder that reads Data Link Layer padding as Type-Data; an
// authenticator that holds the Identifier constant across two Requests; a peer
// that answers with a counter or a Type of its own; a peer that leaves a failed
// conversation open or silently drops the terminal packet it is waiting for; a NAK
// escaping inside a Request; a null-terminated Identity Response; an EAP-TLS
// exchange that survives a flipped octet; and a key-deriving method that
// authenticates one end only.

package eap

import (
	"bytes"
	"errors"
	"testing"
)

// mschapv2Flight records one complete EAP-MSCHAPv2 conversation between the real
// authenticator and the real peer: what the authenticator sent, what the peer
// answered, the terminal packet, and what the peer made of it.
type mschapv2Flight struct {
	requests  []*Packet
	responses []*Packet
	terminal  *Packet
	peerFinal PeerResult
}

// driveMSCHAPv2Flight runs an EAP-MSCHAPv2 exchange to its terminal packet and
// hands that packet to the peer, returning everything observed. The peer
// authenticates with peerPassword and the authenticator verifies against
// authPassword, so mismatched values drive the Failure path.
//
// The loop is bounded at eight rounds. EAP-MSCHAPv2 needs three, so the bound is
// a guard against a state machine that stops advancing rather than a limit the
// method can reach.
func driveMSCHAPv2Flight(t *testing.T, authPassword, peerPassword string) *mschapv2Flight {
	t.Helper()

	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: authPassword})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)

	peer := NewPeerSession(TypeMSCHAPv2, "user", peerPassword)
	t.Cleanup(peer.Close)

	fl := &mschapv2Flight{}
	req := auth.Begin()
	for round := range 8 {
		fl.requests = append(fl.requests, req)

		pres := peer.Process(req)
		if pres.Err != nil {
			t.Fatalf("peer round %d: %v", round, pres.Err)
		}
		if pres.Response == nil {
			t.Fatalf("peer round %d produced no Response", round)
		}
		fl.responses = append(fl.responses, pres.Response)

		next := auth.Process(pres.Response)
		if next == nil {
			t.Fatalf("authenticator round %d returned nil, so the exchange ended with no terminal packet", round)
		}
		if next.Code == CodeSuccess || next.Code == CodeFailure {
			fl.terminal = next
			fl.peerFinal = peer.Process(next)
			return fl
		}
		req = next
	}

	t.Fatal("the EAP-MSCHAPv2 exchange did not terminate within eight rounds")
	return nil
}

// TestRFC3748LengthBoundsTypeData drives the decoder with Data Link Layer padding
// after the framed packet.
func TestRFC3748LengthBoundsTypeData(t *testing.T) {
	framed := (&Packet{Code: CodeRequest, Identifier: 3, Type: TypeIdentity, TypeData: []byte("abc")}).Encode()
	padding := []byte{0xDE, 0xAD, 0xBE}
	padded := append(append([]byte(nil), framed...), padding...)

	// RFC requirement: RFC3748-4-4 positive -- RFC 3748 Section 4: "Octets outside
	// the range of the Length field should be treated as Data Link Layer padding
	// and MUST be ignored upon reception." The packet still decodes, and its
	// Type-Data is the range the Length field delimits.
	got, err := DecodePacket(padded)
	if err != nil {
		t.Fatalf("a padded packet was rejected: %v", err)
	}
	if string(got.TypeData) != "abc" {
		t.Fatalf("TypeData = %q, want %q", got.TypeData, "abc")
	}

	// RFC requirement: RFC3748-4-4 negative -- no padding octet reaches the method.
	// A decoder that sized Type-Data from the octets received rather than from the
	// Length field would deliver all three of them to the EAP method layer.
	for _, pad := range padding {
		if bytes.IndexByte(got.TypeData, pad) >= 0 {
			t.Fatalf("padding octet %#x reached Type-Data %#x", pad, got.TypeData)
		}
	}
}

// TestRFC3748NewRequestChangesIdentifier drives a full exchange and inspects every
// Request the authenticator sent.
func TestRFC3748NewRequestChangesIdentifier(t *testing.T) {
	fl := driveMSCHAPv2Flight(t, "secret", "secret")
	if len(fl.requests) < 2 {
		t.Fatalf("the exchange sent %d Request(s); this assertion needs at least two", len(fl.requests))
	}

	// RFC requirement: RFC3748-4.1-3 positive -- RFC 3748 Section 4.1: "Any new
	// (non-retransmission) Requests MUST modify the Identifier field." Each Request
	// carries an Identifier different from the one before it, so a peer can tell a
	// new Request from a retransmission of the last one.
	for i := 1; i < len(fl.requests); i++ {
		if fl.requests[i].Identifier == fl.requests[i-1].Identifier {
			t.Errorf("Request %d carries Identifier %d, the same as the Request before it",
				i, fl.requests[i].Identifier)
		}
	}

	// RFC requirement: RFC3748-4.1-3 negative -- the authenticator does not hold
	// the Identifier at one value. Every Request of this exchange carries a
	// distinct Identifier, which a stuck counter could not produce. The exchange
	// is three Requests long, far below the 256 the one-octet field holds, so this
	// stronger check cannot fail on wrap-around.
	seen := make(map[uint8]int, len(fl.requests))
	for i, req := range fl.requests {
		if first, held := seen[req.Identifier]; held {
			t.Errorf("Request %d repeats Identifier %d, first sent as Request %d", i, req.Identifier, first)
			continue
		}
		seen[req.Identifier] = i
	}
}

// TestRFC3748ResponseEchoesRequestIdentifier checks the peer's half of the
// Identifier contract, over a driven exchange and against a value it has never
// seen.
func TestRFC3748ResponseEchoesRequestIdentifier(t *testing.T) {
	fl := driveMSCHAPv2Flight(t, "secret", "secret")

	// RFC requirement: RFC3748-4.1-4 positive -- RFC 3748 Section 4.1: "The
	// Identifier field of the Response MUST match that of the currently
	// outstanding Request." Every Response of the exchange carries the Identifier
	// of the Request it answered.
	for i, resp := range fl.responses {
		if resp.Identifier != fl.requests[i].Identifier {
			t.Errorf("Response %d carries Identifier %d, want %d",
				i, resp.Identifier, fl.requests[i].Identifier)
		}
	}

	// RFC requirement: RFC3748-4.1-4 negative -- the peer does not substitute a
	// counter of its own. Handed an Identity Request bearing a value no exchange of
	// its own would produce, the Response carries that value rather than zero or a
	// round number.
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	const unusual uint8 = 0xC3
	res := peer.Process(&Packet{Code: CodeRequest, Identifier: unusual, Type: TypeIdentity})
	if res.Err != nil {
		t.Fatalf("the peer refused an Identity Request: %v", res.Err)
	}
	if res.Response == nil {
		t.Fatal("the peer produced no Response to an Identity Request")
	}
	if res.Response.Identifier != unusual {
		t.Fatalf("Response Identifier = %d, want %d: the peer substituted a value of its own",
			res.Response.Identifier, unusual)
	}
}

// TestRFC3748ResponseTypeMatchesRequest checks the peer's Type contract over a
// driven exchange and against a Type it does not implement.
func TestRFC3748ResponseTypeMatchesRequest(t *testing.T) {
	fl := driveMSCHAPv2Flight(t, "secret", "secret")

	// RFC requirement: RFC3748-4.1-5 positive -- RFC 3748 Section 4.1: "The Type
	// field of a Response MUST either match that of the Request, or correspond to a
	// legacy or Expanded Nak." Ze's peer takes the first branch every time: it
	// answers an Identity Request with Type 1 and an EAP-MSCHAPv2 Request with
	// Type 26.
	for i, resp := range fl.responses {
		if resp.Type != fl.requests[i].Type {
			t.Errorf("Response %d carries Type %d, want %d", i, resp.Type, fl.requests[i].Type)
		}
	}

	// RFC requirement: RFC3748-4.1-5 negative -- a Request whose Type is neither
	// Identity nor the one configured method takes the sentence's OTHER branch,
	// "or correspond to a legacy or Expanded Nak": the peer answers Type 99 with a
	// legacy Nak (Type 3) naming the method it runs. So the Response carries the
	// Request's own Type or the legacy Nak Type, and Type 99 is neither of them.
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	res := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: 99})
	if res.Err != nil {
		t.Fatalf("the peer refused a Request of Type 99 with %v, want a legacy Nak", res.Err)
	}
	if res.Response == nil {
		t.Fatal("the peer answered a Type 99 Request with no Response at all")
	}
	if res.Response.Type != TypeNAK {
		t.Fatalf("the peer answered a Type 99 Request with a Response of Type %d, want %d (legacy Nak)",
			res.Response.Type, TypeNAK)
	}
	if !bytes.Equal(res.Response.TypeData, []byte{TypeMSCHAPv2}) {
		t.Fatalf("the legacy Nak asks for %#x, want the configured method %#x",
			res.Response.TypeData, []byte{TypeMSCHAPv2})
	}
}

// TestRFC3748PeerEndsAnUnsuccessfulConversation drives a password mismatch, which
// makes the authenticator refuse, and then a matching pair.
func TestRFC3748PeerEndsAnUnsuccessfulConversation(t *testing.T) {
	failed := driveMSCHAPv2Flight(t, "server-secret", "peer-secret")
	if failed.terminal.Code != CodeFailure {
		t.Fatalf("the authenticator answered code %d for a wrong password, want %d",
			failed.terminal.Code, CodeFailure)
	}

	// RFC requirement: RFC3748-4.2-5 positive -- RFC 3748 Section 4.2: "the peer
	// MUST terminate the conversation and indicate failure to the lower layer."
	// The peer reports the failure through PeerResult.Err, which is what the IKE
	// engine reads to mark the SA dead (handleEAPResponse,
	// internal/component/ike/engine/fsm.go).
	if failed.peerFinal.Err == nil {
		t.Fatal("the peer reported no error after the authenticator refused it")
	}
	if !errors.Is(failed.peerFinal.Err, ErrEAPFailure) {
		t.Fatalf("the peer reported %v, want ErrEAPFailure", failed.peerFinal.Err)
	}
	if failed.peerFinal.Done {
		t.Fatal("the peer reported Done after an EAP-Failure")
	}

	// RFC requirement: RFC3748-4.2-5 negative -- a method that completes
	// SUCCESSFULLY does not take this path: the peer reports no failure and the
	// conversation ends on Done.
	ok := driveMSCHAPv2Flight(t, "secret", "secret")
	if ok.terminal.Code != CodeSuccess {
		t.Fatalf("the authenticator answered code %d for a matching password, want %d",
			ok.terminal.Code, CodeSuccess)
	}
	if ok.peerFinal.Err != nil {
		t.Fatalf("the peer reported %v after an EAP-Success", ok.peerFinal.Err)
	}
	if !ok.peerFinal.Done {
		t.Fatal("the peer did not conclude on an EAP-Success")
	}
}

// TestRFC3748PeerActsOnTheTerminalPacket checks that a peer waiting for the
// terminal packet consumes both codes rather than dropping either.
func TestRFC3748PeerActsOnTheTerminalPacket(t *testing.T) {
	// RFC requirement: RFC3748-4.2-6 positive -- RFC 3748 Section 4.2: "the peer
	// waits for a Success or Failure packet once the method completes, and MUST NOT
	// silently discard either of them." An EAP-Success is acted on: the peer
	// concludes and exports the 64-octet MSK the IKEv2 AUTH payload needs.
	ok := driveMSCHAPv2Flight(t, "secret", "secret")
	if !ok.peerFinal.Done {
		t.Fatal("the peer discarded the EAP-Success it was waiting for")
	}
	if ok.peerFinal.MSK == ([64]byte{}) {
		t.Fatal("the peer concluded with an all-zero MSK")
	}

	// RFC requirement: RFC3748-4.2-6 negative -- an EAP-Failure is not dropped
	// either. The peer answers it with ErrEAPFailure rather than an empty result,
	// which is what a silent discard would look like to the IKE engine.
	failed := driveMSCHAPv2Flight(t, "server-secret", "peer-secret")
	if failed.peerFinal.Err == nil {
		t.Fatal("the peer discarded the EAP-Failure it was waiting for")
	}
	if failed.peerFinal.Done {
		t.Fatal("the peer concluded on an EAP-Failure")
	}
}

// TestRFC3748NoNAKInARequest checks that neither NAK type ever leaves the
// authenticator inside a Request.
func TestRFC3748NoNAKInARequest(t *testing.T) {
	fl := driveMSCHAPv2Flight(t, "secret", "secret")

	// RFC requirement: RFC3748-5-1 positive -- RFC 3748 Section 5: "Nak (Type 3) or
	// Expanded Nak (Type 254) are valid only for Response packets, they MUST NOT be
	// sent in a Request." Every Request of the exchange carries the Identity type or
	// the method type.
	for i, req := range fl.requests {
		if req.Type == TypeNAK || req.Type == TypeExpandedEAP {
			t.Errorf("Request %d carries Type %d, which is valid only in a Response", i, req.Type)
		}
	}

	// RFC requirement: RFC3748-5-1 negative -- the authenticator does not answer a
	// NAK with a NAK. Handed a Response refusing its method, it ends the exchange
	// with an EAP-Failure and emits no further Request at all.
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)

	req := auth.Begin()
	out := auth.Process(&Packet{
		Code:       CodeResponse,
		Identifier: req.Identifier,
		Type:       TypeNAK,
		TypeData:   []byte{TypeTLS},
	})
	if out == nil {
		t.Fatal("the authenticator produced no packet in answer to a NAK")
	}
	if out.Code != CodeFailure {
		t.Fatalf("the authenticator answered a NAK with code %d, want %d", out.Code, CodeFailure)
	}
	if out.Code == CodeRequest && (out.Type == TypeNAK || out.Type == TypeExpandedEAP) {
		t.Fatalf("the authenticator sent a Request of Type %d", out.Type)
	}
}

// TestRFC3748IdentityResponseIsNotNullTerminated inspects the Identity Response
// the peer builds.
func TestRFC3748IdentityResponseIsNotNullTerminated(t *testing.T) {
	const identity = "eap-user@example.net"

	peer := NewPeerSession(TypeMSCHAPv2, identity, "secret")
	t.Cleanup(peer.Close)

	res := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity})
	if res.Err != nil {
		t.Fatalf("the peer refused an Identity Request: %v", res.Err)
	}
	if res.Response == nil {
		t.Fatal("the peer produced no Identity Response")
	}

	// RFC requirement: RFC3748-5.1-2 positive -- RFC 3748 Section 5.1: "The Identity
	// Response field MUST NOT be null terminated." The field is exactly the identity
	// the peer was configured with, so its length is the identity's length and no
	// terminator was appended.
	if string(res.Response.TypeData) != identity {
		t.Fatalf("Identity Response = %q, want %q", res.Response.TypeData, identity)
	}

	// RFC requirement: RFC3748-5.1-2 negative -- no NUL octet appears anywhere in
	// the field, so neither a terminator nor NUL padding to a fixed width reached
	// the wire.
	if bytes.IndexByte(res.Response.TypeData, 0x00) >= 0 {
		t.Fatalf("Identity Response %#x carries a NUL octet", res.Response.TypeData)
	}
}

// runTamperedEAPTLS drives an EAP-TLS exchange, flipping every bit of the last
// octet of the first authenticator Request that carries a TLS record, and reports
// whether the peer nonetheless concluded.
//
// The loop is bounded at sixty rounds, the same bound runEAPTLSHandshake uses: a
// fragmented certificate flight needs a few tens of rounds and the peer's own
// maxEAPRounds cap ends it sooner.
func runTamperedEAPTLS(t *testing.T, serverCfg MethodConfig, peer *PeerSession) bool {
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
	concluded := false
	tampered := false
	ended := false
	for range 60 {
		// The first TLS Request carrying real record bytes is the one to corrupt.
		// The EAP-TLS Start packet holds only its flags octet, so the length guard
		// steps over it. The copy keeps the authenticator's own fragment buffer
		// intact, so only what the peer sees is changed.
		if !tampered && req.Type == TypeTLS && len(req.TypeData) > 16 {
			bad := &Packet{
				Code:       req.Code,
				Identifier: req.Identifier,
				Type:       req.Type,
				TypeData:   append([]byte(nil), req.TypeData...),
			}
			bad.TypeData[len(bad.TypeData)-1] ^= 0xFF
			req = bad
			tampered = true
		}

		pres := peer.Process(req)
		if pres.Err != nil {
			ended = true
			break
		}
		if pres.Done {
			concluded = true
			ended = true
			break
		}
		if pres.Response == nil {
			ended = true
			break
		}

		next := sess.Process(pres.Response)
		if next == nil {
			ended = true
			break
		}
		if next.Code == CodeSuccess || next.Code == CodeFailure {
			peer.Process(next)
			concluded = peer.Succeeded()
			ended = true
			break
		}
		req = next
	}

	// Without this the helper reports "the peer did not conclude" for an exchange
	// that was never corrupted, and the negative case passes on the wrong reason.
	if !tampered {
		t.Fatal("no EAP-TLS Request carried a record long enough to corrupt, so nothing was tested")
	}
	if !ended {
		t.Fatal("the tampered EAP-TLS exchange did not terminate within sixty rounds")
	}
	return concluded
}

// TestRFC3748EAPTLSValidatesItsPerPacketMIC drives EAP-TLS clean and then with one
// octet changed in flight.
func TestRFC3748EAPTLSValidatesItsPerPacketMIC(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peerCfg := &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	}

	// RFC requirement: RFC3748-7.5-1 positive -- RFC 3748 Section 7.5: "If a
	// per-packet MIC is employed within an EAP method, then peers, authentication
	// servers, and authenticators not operating in pass-through mode MUST validate
	// the MIC." EAP-TLS carries its integrity check in the TLS record layer and
	// transcript, and an unmodified exchange passes it on both sides.
	clean := runEAPTLSHandshake(t, pki.serverConfig(), NewPeerSessionTLS("eap-tls-client", peerCfg))
	if !clean.serverEAPSuccess {
		t.Fatal("the authenticator refused an unmodified EAP-TLS exchange")
	}
	if !clean.peerDone {
		t.Fatal("the peer did not conclude an unmodified EAP-TLS exchange")
	}

	// RFC requirement: RFC3748-7.5-1 negative -- one octet changed inside an
	// EAP-TLS Request is refused. The peer never concludes, so the modified packet
	// cannot carry the exchange to an MSK.
	if runTamperedEAPTLS(t, pki.serverConfig(), NewPeerSessionTLS("eap-tls-client", peerCfg)) {
		t.Fatal("the peer concluded an EAP-TLS exchange whose record was modified in flight")
	}
}

// TestRFC3748KeyDerivingMethodAuthenticatesBothEnds drives EAP-TLS against a
// trusted authenticator and against one the peer cannot chain.
func TestRFC3748KeyDerivingMethodAuthenticatesBothEnds(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peerCfg := &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	}

	// RFC requirement: RFC3748-7.10-4 positive -- RFC 3748 Section 7.10: "EAP
	// Methods deriving keys MUST provide for mutual authentication between the EAP
	// peer and the EAP Server." After the exchange each side holds the other's
	// verified certificate, so neither authenticated alone.
	res := runEAPTLSHandshake(t, pki.serverConfig(), NewPeerSessionTLS("eap-tls-client", peerCfg))
	if !res.peerDone {
		t.Fatal("the peer did not conclude a trusted EAP-TLS exchange")
	}
	if len(res.serverState().PeerCertificates) == 0 {
		t.Fatal("the authenticator holds no peer certificate, so it did not authenticate the peer")
	}
	if len(res.peerState().PeerCertificates) == 0 {
		t.Fatal("the peer holds no authenticator certificate, so it did not authenticate the server")
	}

	// RFC requirement: RFC3748-7.10-4 negative -- a server the peer cannot chain to
	// its trust anchor gets no MSK. One-way authentication does not reach a key.
	rogue := MethodConfig{
		ServerCertPEM: pki.untrustedServerCertPEM,
		ServerKeyPEM:  pki.untrustedServerKeyPEM,
		CACertPEM:     pki.trustedCAPEM,
	}
	bad := runEAPTLSHandshake(t, rogue, NewPeerSessionTLS("eap-tls-client", peerCfg))
	if bad.peerDone {
		t.Fatal("the peer concluded against an authenticator it could not authenticate")
	}
	if bad.peerMSK != ([64]byte{}) {
		t.Fatal("the peer derived an MSK against an authenticator it could not authenticate")
	}
}
