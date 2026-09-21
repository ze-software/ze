// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS conversation rules
// RFC: rfc/short/rfc5216.md -- Section 2.1.1 (Start and first Response) and Section 3 (packet fields)
//
// RFC 5216 Section 2.1.1 opens the conversation with one packet from each side:
//
//	"Once having received the peer's Identity, the EAP server MUST respond with
//	an EAP-TLS/Start packet, which is an EAP-Request packet with EAP-Type=EAP-TLS,
//	the Start (S) bit set, and no data."
//
//	"If the peer supports EAP-TLS and is configured to use it, it MUST respond to
//	the EAP-Request with an EAP-Response packet of EAP-Type=EAP-TLS."
//
// Section 3 then binds the packet fields: the Identifier changes on every
// Request and is copied into every Response, octets past the Length are
// padding, and the reserved flag bits are ignored on reception.
//
// VALIDATES: each of those sentences at the packet the real Session and the
// real PeerSession produce, and its boundary: no Start before the Identity, a
// Nak from a peer configured for another method, a Success that keeps the
// answered Identifier, padding that never reaches TypeData, and reserved bits
// that change nothing.
// PREVENTS: a Start that carries data or arrives unasked, a peer that answers
// EAP-TLS with another Type, an Identifier that is generated on the peer side
// rather than copied, and a decoder that hands padding to the TLS engine.
package eap

import (
	"bytes"
	"crypto/tls"
	"testing"
)

// eapTLSStartRequest is the Start packet of RFC 5216 Section 2.1.1 with the
// Identifier a test chooses: the S bit alone and no TLS data behind it.
func eapTLSStartRequest(identifier uint8) *Packet {
	return &Packet{Code: CodeRequest, Identifier: identifier, Type: TypeTLS, TypeData: []byte{eapTLSFlagS}}
}

// eapTLSPayload returns the TLS octets of an EAP-TLS TypeData: what follows the
// flags octet, and the four-octet length field when L is set.
func eapTLSPayload(td []byte) []byte {
	if len(td) == 0 {
		return nil
	}
	if td[0]&eapTLSFlagL != 0 {
		if len(td) < 5 {
			return nil
		}
		return td[5:]
	}
	return td[1:]
}

// newTrustedTLSPeer is the peer every successful flight in this file uses: its
// certificate and the authenticator's chain to the same CA.
func newTrustedTLSPeer(t *testing.T, pki *eapTLSPKI) *PeerSession {
	t.Helper()
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})
	return peer
}

// TestRFC5216StartAnswersTheIdentityResponse drives the authenticator through
// the Identity round and reads the packet it sends next.
//
// RFC requirement: RFC5216-2.1.1-3 positive -- RFC 5216 Section 2.1.1: "Once
// having received the peer's Identity, the EAP server MUST respond with an
// EAP-TLS/Start packet, which is an EAP-Request packet with EAP-Type=EAP-TLS,
// the Start (S) bit set, and no data." The packet answering the Identity
// Response is an EAP-Request of Type 13 whose TypeData is the single flags octet
// with only the S bit set.
func TestRFC5216StartAnswersTheIdentityResponse(t *testing.T) {
	pki := newEAPTLSPKI(t)
	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(sess.Close)

	identity := sess.Begin()
	start := sess.Process(&Packet{Code: CodeResponse, Identifier: identity.Identifier, Type: TypeIdentity, TypeData: []byte("client")})
	if start == nil {
		t.Fatal("the authenticator sent nothing after the Identity Response")
	}
	if start.Code != CodeRequest || start.Type != TypeTLS {
		t.Fatalf("the packet after the Identity Response is code %d type %d, want an EAP-Request (%d) of EAP-Type=EAP-TLS (%d)",
			start.Code, start.Type, CodeRequest, TypeTLS)
	}
	if !bytes.Equal(start.TypeData, []byte{eapTLSFlagS}) {
		t.Fatalf("the Start packet's TypeData is %x, want the flags octet %02x alone (S bit set, no data)",
			start.TypeData, eapTLSFlagS)
	}
}

// TestRFC5216NoStartBeforeTheIdentityIsReceived is the boundary of the Start
// obligation: the Start answers the Identity, so nothing else draws it.
//
// RFC requirement: RFC5216-2.1.1-3 negative -- the Start is owed "once having
// received the peer's Identity". The authenticator's first packet is the
// Identity Request and not a Start, and an Identity Response the authenticator
// discards (it answers no outstanding Request) draws no Start either.
func TestRFC5216NoStartBeforeTheIdentityIsReceived(t *testing.T) {
	pki := newEAPTLSPKI(t)
	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(sess.Close)

	first := sess.Begin()
	if first.Type == TypeTLS {
		t.Fatalf("the authenticator opened with an EAP-TLS packet (flags %x) before it received any Identity", first.TypeData)
	}
	if first.Type != TypeIdentity {
		t.Fatalf("the authenticator opened with Type %d, want the Identity Request (%d)", first.Type, TypeIdentity)
	}

	stale := sess.Process(&Packet{Code: CodeResponse, Identifier: first.Identifier + 1, Type: TypeIdentity, TypeData: []byte("client")})
	if stale != nil {
		t.Fatalf("an Identity Response answering no outstanding Request drew code %d type %d TypeData %x; no Identity was received, so no Start is owed",
			stale.Code, stale.Type, stale.TypeData)
	}
}

// TestRFC5216ConfiguredPeerAnswersEAPTLSWithEAPTLS gives an EAP-TLS peer the
// Start packet and reads the Type of its answer.
//
// RFC requirement: RFC5216-2.1.1-8 positive -- RFC 5216 Section 2.1.1: "If the
// peer supports EAP-TLS and is configured to use it, it MUST respond to the
// EAP-Request with an EAP-Response packet of EAP-Type=EAP-TLS." The Response to
// the Start is Code 2, Type 13, and carries a TLS handshake record.
func TestRFC5216ConfiguredPeerAnswersEAPTLSWithEAPTLS(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := newTrustedTLSPeer(t, pki)
	t.Cleanup(peer.Close)

	if id := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}); id.Response == nil {
		t.Fatalf("the Identity Request drew %+v, want an Identity Response", id)
	}
	res := peer.Process(eapTLSStartRequest(2))
	if res.Err != nil {
		t.Fatalf("the peer refused the Start: %v", res.Err)
	}
	if res.Response == nil {
		t.Fatal("the peer sent nothing in answer to the Start")
	}
	if res.Response.Code != CodeResponse || res.Response.Type != TypeTLS {
		t.Fatalf("the answer to the Start is code %d type %d, want an EAP-Response (%d) of EAP-Type=EAP-TLS (%d)",
			res.Response.Code, res.Response.Type, CodeResponse, TypeTLS)
	}
	if payload := eapTLSPayload(res.Response.TypeData); len(payload) < tlsRecordHeaderLen || payload[0] != tlsRecordHandshake {
		t.Fatalf("the answer to the Start carries TypeData %x, want the EAP-TLS header followed by a TLS handshake record (client_hello)",
			res.Response.TypeData)
	}
}

// TestRFC5216PeerNotConfiguredForEAPTLSDoesNotAnswerWithEAPTLS is the boundary
// of the same sentence: the EAP-TLS Response is owed by a peer "configured to
// use it", and a peer configured for another method Naks instead.
//
// RFC requirement: RFC5216-2.1.1-8 negative -- a peer configured for MS-CHAPv2
// answers the EAP-TLS Start with a legacy Nak naming its own method, and never
// with an EAP-Response of EAP-Type=EAP-TLS.
func TestRFC5216PeerNotConfiguredForEAPTLSDoesNotAnswerWithEAPTLS(t *testing.T) {
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	if id := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}); id.Response == nil {
		t.Fatalf("the Identity Request drew %+v, want an Identity Response", id)
	}
	res := peer.Process(eapTLSStartRequest(2))
	if res.Response != nil && res.Response.Type == TypeTLS {
		t.Fatalf("a peer configured for MS-CHAPv2 answered the EAP-TLS Start with an EAP-TLS Response (TypeData %x)",
			res.Response.TypeData)
	}
	wantLegacyNak(t, res, 2, TypeMSCHAPv2, "the EAP-TLS Start at a peer configured for MS-CHAPv2")
}

// TestRFC5216IdentifierChangesOnEveryRequest drives a full TLS 1.2 conversation
// and compares each Request's Identifier with the one before it.
//
// RFC requirement: RFC5216-3-3 positive -- RFC 5216 Section 3: "The Identifier
// field MUST be changed on each Request packet." Across the whole conversation
// no Request carries the Identifier of the Request before it.
//
// RFC requirement: RFC5216-3-3 negative -- the change is owed on a REQUEST. The
// terminal EAP-Success is not one, and it repeats the Identifier of the
// Response it answers rather than taking a fresh value.
func TestRFC5216IdentifierChangesOnEveryRequest(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := newTrustedTLSPeer(t, pki)
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, 40)
	if fl.successAt < 1 {
		t.Fatalf("the conversation did not end in EAP-Success (successAt=%d, peerErr=%v)", fl.successAt, fl.peerErr)
	}

	requests := 0
	for i := 1; i < len(fl.serverSent); i++ {
		prev, cur := fl.serverSent[i-1], fl.serverSent[i]
		if cur.Code != CodeRequest {
			continue
		}
		requests++
		if cur.Identifier == prev.Identifier {
			t.Errorf("Request %d carries Identifier %d, the same as the packet before it", i, cur.Identifier)
		}
	}
	if requests < 3 {
		t.Fatalf("only %d Requests followed another packet; the conversation is too short to show the rule", requests)
	}

	success := fl.serverSent[fl.successAt]
	answered := fl.peerSent[fl.successAt-1]
	if success.Identifier != answered.Identifier {
		t.Errorf("EAP-Success carries Identifier %d, want %d, the Identifier of the Response it answers: only a Request takes a new one",
			success.Identifier, answered.Identifier)
	}
}

// TestRFC5216PaddingPastTheLengthIsIgnored decodes an EAP-TLS packet that
// carries octets after its Length.
//
// RFC requirement: RFC5216-3-4 positive -- RFC 5216 Section 3: "Octets outside
// the range of the Length field should be treated as Data Link Layer padding and
// MUST be ignored on reception." A padded packet decodes without error, to the
// same Code, Identifier, Type and TypeData as the packet without padding.
//
// RFC requirement: RFC5216-3-4 negative -- the padding never reaches the data.
// The padding octets are shaped like a second EAP-TLS flags octet and a TLS
// record header, and none of them appears in TypeData, whose length is the
// Length field less the five header octets.
func TestRFC5216PaddingPastTheLengthIsIgnored(t *testing.T) {
	typeData := []byte{eapTLSFlagL | eapTLSFlagM, 0, 0, 0x10, 0, 0x16, 0x03, 0x03}
	wire := []byte{CodeResponse, 7, 0, byte(5 + len(typeData)), TypeTLS}
	wire = append(wire, typeData...)
	padding := []byte{eapTLSFlagS, 0x16, 0x03, 0x03, 0x00, 0x05, 0xff}
	padded := append(append([]byte{}, wire...), padding...)

	bare, err := DecodePacket(wire)
	if err != nil {
		t.Fatalf("the unpadded packet did not decode: %v", err)
	}
	got, err := DecodePacket(padded)
	if err != nil {
		t.Fatalf("the padded packet did not decode: %v", err)
	}
	if got.Code != bare.Code || got.Identifier != bare.Identifier || got.Type != bare.Type {
		t.Errorf("padding changed the header: got code %d id %d type %d, want code %d id %d type %d",
			got.Code, got.Identifier, got.Type, bare.Code, bare.Identifier, bare.Type)
	}
	if !bytes.Equal(got.TypeData, typeData) {
		t.Errorf("padded TypeData = %x, want %x", got.TypeData, typeData)
	}
	if len(got.TypeData) != len(typeData) {
		t.Errorf("TypeData is %d octets, want %d (Length 13 less the 5 header octets): %d octet(s) of padding reached the data",
			len(got.TypeData), len(typeData), len(got.TypeData)-len(typeData))
	}
	if bytes.Contains(got.TypeData, padding) {
		t.Errorf("the padding octets %x appear inside TypeData %x", padding, got.TypeData)
	}
}

// TestRFC5216ReservedFlagBitsAreIgnoredOnReception sets every reserved bit on
// packets sent in both directions and checks that each side reads only L, M and
// S.
//
// RFC requirement: RFC5216-3-5 positive -- RFC 5216 Section 3: "Implementations
// of this specification MUST set the reserved bits to zero, and MUST ignore them
// on reception." A Start whose flags octet carries S and all five reserved bits
// is still a Start: the peer answers it with its client_hello.
//
// RFC requirement: RFC5216-3-5 negative -- a reserved bit is not read as L, M or
// S. The peer's real client_hello, reassembled and re-flagged with the five
// reserved bits and nothing else, is handled by the authenticator as one whole
// unfragmented message: it is answered with the server's handshake flight and
// not with a fragment ACK, and no length field is read out of the TLS data.
func TestRFC5216ReservedFlagBitsAreIgnoredOnReception(t *testing.T) {
	const reserved = 0x1f

	pki := newEAPTLSPKI(t)
	peer := newTrustedTLSPeer(t, pki)
	t.Cleanup(peer.Close)
	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(sess.Close)

	identity := sess.Begin()
	idRes := peer.Process(identity)
	if idRes.Response == nil {
		t.Fatalf("the Identity Request drew %+v, want an Identity Response", idRes)
	}
	start := sess.Process(idRes.Response)
	if start == nil || start.Type != TypeTLS {
		t.Fatalf("the authenticator did not send a Start: %+v", start)
	}

	start.TypeData = []byte{eapTLSFlagS | reserved}
	hello := peer.Process(start)
	if hello.Err != nil || hello.Response == nil {
		t.Fatalf("the peer did not treat a Start with reserved bits set as a Start: err=%v response=%+v", hello.Err, hello.Response)
	}
	clientHello := eapTLSPayload(hello.Response.TypeData)
	if hello.Response.Type != TypeTLS || len(clientHello) < tlsRecordHeaderLen || clientHello[0] != tlsRecordHandshake {
		t.Fatalf("the answer to the re-flagged Start is type %d TypeData %x, want an EAP-TLS Response carrying the client_hello handshake record",
			hello.Response.Type, hello.Response.TypeData)
	}

	// The client_hello leaves in fragments when it carries TLS 1.3 key shares.
	// Collect the rest with the ACKs RFC 5216 Section 2.1.5 owes, so the
	// authenticator below receives the whole message in ONE packet.
	ackIdentifier := start.Identifier
	for fragment := hello.Response; fragment.TypeData[0]&eapTLSFlagM != 0; {
		ackIdentifier++
		more := peer.Process(&Packet{Code: CodeRequest, Identifier: ackIdentifier, Type: TypeTLS, TypeData: []byte{0}})
		if more.Err != nil || more.Response == nil {
			t.Fatalf("the peer did not continue its client_hello after a fragment ACK: err=%v response=%+v", more.Err, more.Response)
		}
		fragment = more.Response
		clientHello = append(clientHello, eapTLSPayload(fragment.TypeData)...)
	}

	whole := &Packet{Code: CodeResponse, Identifier: start.Identifier, Type: TypeTLS, TypeData: append([]byte{reserved}, clientHello...)}
	flight := sess.Process(whole)
	if flight == nil {
		t.Fatal("the authenticator discarded a client_hello whose flags octet carries only reserved bits")
	}
	if flight.Code != CodeRequest || flight.Type != TypeTLS {
		t.Fatalf("the authenticator answered the re-flagged client_hello with code %d type %d, want an EAP-TLS Request", flight.Code, flight.Type)
	}
	if len(flight.TypeData) == 1 {
		t.Fatalf("the authenticator answered the re-flagged client_hello with a bare fragment ACK (flags %02x): it read a reserved bit as M",
			flight.TypeData[0])
	}
	if payload := eapTLSPayload(flight.TypeData); len(payload) < tlsRecordHeaderLen || payload[0] != tlsRecordHandshake {
		t.Fatalf("the authenticator's answer %x is not its handshake flight", flight.TypeData)
	}
}

// TestRFC5216ResponseIdentifierMatchesTheRequest reads the Identifier of every
// Response a full conversation produces.
//
// RFC requirement: RFC5216-3-6 positive -- RFC 5216 Section 3, on the
// EAP-Response: "The Identifier field is one octet and MUST match the Identifier
// field from the corresponding request." Every Response the peer sends carries
// the Identifier of the Request it answers.
func TestRFC5216ResponseIdentifierMatchesTheRequest(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := newTrustedTLSPeer(t, pki)
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, 40)
	if fl.successAt < 1 {
		t.Fatalf("the conversation did not end in EAP-Success (successAt=%d, peerErr=%v)", fl.successAt, fl.peerErr)
	}
	if len(fl.peerSent) < 3 {
		t.Fatalf("the peer sent %d packets; too few to show the rule", len(fl.peerSent))
	}
	for i, res := range fl.peerSent {
		if req := fl.serverSent[i]; res.Identifier != req.Identifier {
			t.Errorf("Response %d carries Identifier %d, want %d, the Identifier of the Request it answers", i, res.Identifier, req.Identifier)
		}
	}
}

// TestRFC5216ResponseIdentifierIsCopiedNotCounted is the boundary of the same
// rule: the peer copies the Identifier, it does not maintain a counter of its
// own.
//
// RFC requirement: RFC5216-3-6 negative -- Requests arriving with Identifiers
// 0x80 and then 0x05 are answered with 0x80 and then 0x05. A peer that generated
// its own sequence would answer the second with 0x81.
func TestRFC5216ResponseIdentifierIsCopiedNotCounted(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := newTrustedTLSPeer(t, pki)
	t.Cleanup(peer.Close)

	idRes := peer.Process(&Packet{Code: CodeRequest, Identifier: 0x80, Type: TypeIdentity})
	if idRes.Response == nil || idRes.Response.Identifier != 0x80 {
		t.Fatalf("the Identity Request with Identifier 0x80 drew %+v, want a Response carrying 0x80", idRes.Response)
	}
	hello := peer.Process(eapTLSStartRequest(0x05))
	if hello.Response == nil {
		t.Fatalf("the Start drew no Response (err=%v)", hello.Err)
	}
	if hello.Response.Identifier != 0x05 {
		t.Fatalf("the Start with Identifier 0x05 drew a Response carrying %#02x; the peer generated an Identifier instead of copying the Request's",
			hello.Response.Identifier)
	}
}
