// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP peer (client/initiator) side
// RFC: rfc/short/rfc3748.md -- Section 5.3.1: the legacy Nak, and Section 2.1: its boundary
//
// The legacy Nak is how EAP negotiates. An authenticator offers a method, a peer
// that does not run it names the one it does, and the authenticator offers that
// instead. Ze answered every such offer with an error until 2026-09-01, which the
// IKE engine reads as a reason to kill the SA (handleEAPResponse,
// internal/component/ike/engine/fsm.go): the negotiation the RFC describes ended
// as a dead tunnel.
//
// Three questions decide every case here, and each has its own test. WHICH Types
// earn a Nak: 4-253, 255 and, through Section 5.7, 254. WHAT the Nak says: one
// octet naming the configured method, never the zero that tells an authenticator
// to stop. WHEN the peer may still send one: until it has answered a Request of
// its method's own Type, which is the boundary Section 2.1 draws.
//
// The authenticator half is the same conversation seen from the other end: a Nak
// it receives ends the exchange, and the Types the peer asked for are the only
// word the peer gets to say about why.
//
// VALIDATES: an unacceptable authentication Type draws a six-octet Type-3
// Response naming the configured method with the Request's Identifier; a Type-254
// Request draws that same LEGACY Nak and never an Expanded one; Types 1, 2 and 26
// draw no Nak; an EAP-TLS peer names 13 where an MS-CHAPv2 peer names 26; a
// malformed Request of the method's own Type reports its error rather than
// Nakking; a Request arriving after the method started is discarded; and the
// authenticator records the Types a received Nak asked for.
// PREVENTS: an error where the protocol has a Response, which costs the IKE SA; a
// Nak naming a constant rather than the configured method; a Nak carrying the
// zero that ends the negotiation; an Expanded Nak claiming support ze does not
// have; a Nak used as a general purpose error indication; a Nak sent after the
// peer committed to its method, which Section 2.1 forbids; and an EAP-Failure
// that leaves the operator with no way to learn which method the far end wanted.

package eap

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// wantLegacyNak fails unless the result is the legacy Nak RFC 3748 Section 5.3.1
// specifies: Code 2, Type 3, the Request's Identifier, and one Type-Data octet
// naming the desired authentication Type, which puts the whole packet at six
// octets on the wire.
//
// The zero octet is refused explicitly, and the refusal reads the EXPECTED Type
// rather than the octet on the wire. Section 5.3.1 gives zero a meaning of its
// own, "no viable alternatives", and it is also what an uninitialized field
// holds, so a peer that had lost its configured method would send a valid-looking
// Nak that ends the negotiation (ai/rules/principles.md). The bytes.Equal below
// already pins the wire octet to `desired`, so a caller asking for zero is the
// only way such a Nak can pass, and that is the case this refuses.
func wantLegacyNak(t *testing.T, res PeerResult, id, desired uint8, what string) {
	t.Helper()

	if desired == 0 {
		t.Fatalf("%s: the expected desired Type is 0, which tells the authenticator to stop rather than naming a method", what)
	}
	if res.Err != nil {
		t.Fatalf("%s: the peer reported %v; the RFC answers this Request with a Nak", what, res.Err)
	}
	if res.Discarded {
		t.Fatalf("%s: the peer discarded the Request; the RFC answers it with a Nak", what)
	}
	if res.Response == nil {
		t.Fatalf("%s: the peer sent no Response at all", what)
	}
	if res.Response.Code != CodeResponse {
		t.Fatalf("%s: Nak Code = %d, want %d", what, res.Response.Code, CodeResponse)
	}
	// A Type of 3 is also what rules the EXPANDED Nak out: ze reads no Expanded
	// Type, so composing a Type-254 Response would claim a support it lacks.
	if res.Response.Type != TypeNAK {
		t.Fatalf("%s: Response Type = %d, want %d (legacy Nak); Type %d would claim Expanded support ze does not have",
			what, res.Response.Type, TypeNAK, TypeExpandedEAP)
	}
	if res.Response.Identifier != id {
		t.Fatalf("%s: Nak Identifier = %d, want %d", what, res.Response.Identifier, id)
	}
	if !bytes.Equal(res.Response.TypeData, []byte{desired}) {
		t.Fatalf("%s: Nak Type-Data = %#x, want %#x", what, res.Response.TypeData, []byte{desired})
	}
	if got := len(res.Response.Encode()); got != 6 {
		t.Fatalf("%s: the encoded Nak is %d octets, want 6", what, got)
	}
}

// nakMSCHAPv2Challenge is a well-formed MS-CHAPv2 Challenge: opcode 1, MS-ID,
// 2-octet MS-Length, then a 16-octet challenge behind its own length octet.
func nakMSCHAPv2Challenge() []byte {
	return append([]byte{1, 2, 0, 22, 16}, make([]byte, 16)...)
}

// nakPeerCommitted returns an MS-CHAPv2 peer that has answered the Identity
// Request AND a Request of its method's own Type, which is the "initial non-Nak
// Response" of RFC 3748 Section 2.1.
func nakPeerCommitted(t *testing.T) *PeerSession {
	t.Helper()

	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	if res := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}); res.Response == nil {
		t.Fatalf("the Identity Request drew %+v, want an Identity Response", res)
	}

	challenge := peer.Process(&Packet{Code: CodeRequest, Identifier: 2, Type: TypeMSCHAPv2, TypeData: nakMSCHAPv2Challenge()})
	if challenge.Response == nil {
		t.Fatalf("the MS-CHAPv2 Challenge drew %+v, want an MS-CHAPv2 Response", challenge)
	}
	if challenge.Response.Type != TypeMSCHAPv2 {
		t.Fatalf("the MS-CHAPv2 Challenge drew Type %d, want %d", challenge.Response.Type, TypeMSCHAPv2)
	}
	if !peer.methodCommitted {
		t.Fatal("the peer answered a Request of its method's Type without recording the commitment")
	}
	return peer
}

// TestPeerNaksAnUnacceptableAuthenticationType drives the three Types that stand
// for the range: 40 inside 4-253, 4 (MD5-Challenge) at its lower edge, and 255,
// the value the sentence names separately.
//
// Type 4 is the case that pays for itself. MD5-Challenge is the method an
// authenticator offers first by habit, and the peer here is configured for
// EAP-MSCHAPv2, so Type 4 is one more unacceptable Type to it. Ze has run
// MD5-Challenge since 2026-09-01 (eap_md5challenge.go), and naks() answers false
// only for the CONFIGURED method, so a peer running something else still refuses
// it. Refusing it by the protocol's own mechanism is what lets the authenticator
// offer the method ze does run.
//
// Each Type is driven from both states in which the peer has not yet committed to
// a method: the identity state, and after the Identity Response has gone out.
func TestPeerNaksAnUnacceptableAuthenticationType(t *testing.T) {
	unacceptable := []uint8{40, 4, 255}

	// RFC requirement: RFC3748-5.3.1-1 positive -- RFC 3748 Section 5.3.1: "Where a
	// peer receives a Request for an unacceptable authentication Type (4-253,255),
	// or a peer lacking support for Expanded Types receives a Request for Type 254,
	// a Nak Response (Type 3) MUST be sent." Types 40, 4 and 255 each draw a Type-3
	// Response, from the identity state and from the state after the Identity
	// Response.
	//
	// RFC requirement: RFC3748-5.3.1-2 positive -- RFC 3748 Section 5.3.1: "The
	// Type-Data field of the Nak Response (Type 3) MUST contain one or more octets
	// indicating the desired authentication Type(s), one octet per Type, or the
	// value zero (0) to indicate no proposed alternative." Each Nak carries exactly
	// one octet, holding 26 for a peer configured with EAP-MSCHAPv2, so the encoded
	// packet is six octets and the octet is never the zero that ends the
	// negotiation.
	for _, offered := range unacceptable {
		fresh := NewPeerSession(TypeMSCHAPv2, "user", "secret")
		t.Cleanup(fresh.Close)

		res := fresh.Process(&Packet{Code: CodeRequest, Identifier: 9, Type: offered})
		wantLegacyNak(t, res, 9, TypeMSCHAPv2, "a Request of Type "+strconv.Itoa(int(offered))+" in the identity state")

		answered := NewPeerSession(TypeMSCHAPv2, "user", "secret")
		t.Cleanup(answered.Close)

		if id := answered.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}); id.Response == nil {
			t.Fatalf("the Identity Request drew %+v, want an Identity Response", id)
		}
		after := answered.Process(&Packet{Code: CodeRequest, Identifier: 2, Type: offered})
		wantLegacyNak(t, after, 2, TypeMSCHAPv2, "a Request of Type "+strconv.Itoa(int(offered))+" after the Identity Response")
	}
}

// TestPeerDoesNotNakATypeItHandles drives the three Types that sit outside the
// Nak's range or name the peer's own method.
//
// Without this, every Nak assertion in this file is satisfied by a peer that
// answers a Nak to everything, which would break the Identity exchange and the
// method itself.
func TestPeerDoesNotNakATypeItHandles(t *testing.T) {
	// RFC requirement: RFC3748-5.3.1-1 negative -- Type 1 (Identity), Type 2
	// (Notification) and Type 26 (the configured method) draw no Nak. Section
	// 5.3.1 bounds the Nak to "an unacceptable authentication Type (4-253,255)"
	// and Section 5 numbers the authentication Types "4 and above", so each of the
	// three is answered with a Response of the Request's own Type instead.
	cases := []struct {
		name     string
		request  *Packet
		wantType uint8
	}{
		{
			name:     "an Identity Request",
			request:  &Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity},
			wantType: TypeIdentity,
		},
		{
			name:     "a Notification Request",
			request:  &Packet{Code: CodeRequest, Identifier: 2, Type: TypeNotification, TypeData: []byte("notice")},
			wantType: TypeNotification,
		},
		{
			name:     "an MS-CHAPv2 Challenge",
			request:  &Packet{Code: CodeRequest, Identifier: 3, Type: TypeMSCHAPv2, TypeData: nakMSCHAPv2Challenge()},
			wantType: TypeMSCHAPv2,
		},
	}

	for _, tc := range cases {
		peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
		t.Cleanup(peer.Close)

		res := peer.Process(tc.request)
		if res.Response == nil {
			t.Fatalf("%s drew %+v, want a Response", tc.name, res)
		}
		if res.Response.Type == TypeNAK {
			t.Fatalf("%s drew a legacy Nak asking for %#x", tc.name, res.Response.TypeData)
		}
		if res.Response.Type != tc.wantType {
			t.Fatalf("%s drew Type %d, want %d", tc.name, res.Response.Type, tc.wantType)
		}
	}
}

// TestNakNamesTheConfiguredMethod drives the same unacceptable Type at an EAP-TLS
// peer and at an EAP-MSCHAPv2 peer.
//
// The Nak's whole value is the octet: an authenticator reads it and offers that
// method next. An octet that were a constant would negotiate one method for every
// deployment, and would do it while looking exactly like this one on the wire.
func TestNakNamesTheConfiguredMethod(t *testing.T) {
	// RFC requirement: RFC3748-5.3.1-2 negative -- the desired-Type octet is read
	// from the peer's configured method rather than written as a constant. An
	// EAP-TLS peer answers the same Type-40 Request with 13, where the EAP-MSCHAPv2
	// peer answers 26.
	tlsPeer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{})
	t.Cleanup(tlsPeer.Close)

	tlsRes := tlsPeer.Process(&Packet{Code: CodeRequest, Identifier: 12, Type: 40})
	wantLegacyNak(t, tlsRes, 12, TypeTLS, "a Request of Type 40 at an EAP-TLS peer")

	mschapPeer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(mschapPeer.Close)

	mschapRes := mschapPeer.Process(&Packet{Code: CodeRequest, Identifier: 12, Type: 40})
	wantLegacyNak(t, mschapRes, 12, TypeMSCHAPv2, "a Request of Type 40 at an EAP-MSCHAPv2 peer")

	if bytes.Equal(tlsRes.Response.TypeData, mschapRes.Response.TypeData) {
		t.Fatalf("both peers asked for %#x, so the Nak names a constant rather than the configured method",
			tlsRes.Response.TypeData)
	}
}

// TestPeerNaksAnExpandedTypeRequestWithALegacyNak drives the Type that reaches
// the Nak through Section 5.7 rather than through the 4-253 range.
//
// Ze reads no Expanded Type, so composing an Expanded Nak would claim a support
// it does not have: Section 5.3.2 reserves that Response for "a peer supporting
// Expanded Types", and Section 5 puts that support at SHOULD.
func TestPeerNaksAnExpandedTypeRequestWithALegacyNak(t *testing.T) {
	// RFC requirement: RFC3748-5.3.1-1 positive -- RFC 3748 Section 5.7: "Peers not
	// equipped to interpret the Expanded Type MUST send a Nak as described in
	// Section 5.3.1, and negotiate a more suitable authentication method." A
	// Type-254 Request draws the LEGACY Nak of Section 5.3.1: Code 2, Type 3, one
	// octet naming the configured method, six octets encoded. No packet the peer
	// produces carries Type 254.
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	res := peer.Process(&Packet{
		Code:       CodeRequest,
		Identifier: 21,
		Type:       TypeExpandedEAP,
		TypeData:   []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
	})
	wantLegacyNak(t, res, 21, TypeMSCHAPv2, "a Request of Type 254 (Expanded)")

	if res.Response.Type == TypeExpandedEAP {
		t.Fatal("the peer emitted a Type-254 packet, claiming Expanded Type support it does not have")
	}
	if bytes.IndexByte(res.Response.Encode(), TypeExpandedEAP) >= 0 {
		t.Fatalf("the encoded Nak %#x carries the octet 254", res.Response.Encode())
	}
}

// TestNakIdentifierMatchesTheRequest drives four Identifiers, including the two
// edges of the one-octet field.
func TestNakIdentifierMatchesTheRequest(t *testing.T) {
	identifiers := []uint8{0, 1, 0x7F, 0xFF}

	// RFC requirement: RFC3748-5.3.1-3 positive -- RFC 3748 Section 5.3.1: "The
	// Identifier field of a legacy Nak Response MUST match the Identifier field of
	// the Request packet that it is sent in response to." Each of four Requests of
	// Type 40, carrying Identifiers 0, 1, 127 and 255, draws a Nak carrying that
	// same value.
	naks := make([]*Packet, 0, len(identifiers))
	for _, id := range identifiers {
		peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
		t.Cleanup(peer.Close)

		res := peer.Process(&Packet{Code: CodeRequest, Identifier: id, Type: 40})
		wantLegacyNak(t, res, id, TypeMSCHAPv2, "a Request of Type 40 carrying Identifier "+strconv.Itoa(int(id)))
		naks = append(naks, res.Response)
	}

	// RFC requirement: RFC3748-5.3.1-3 negative -- the Identifier is copied from
	// the Request rather than held at a value of the peer's own. Four Requests
	// bearing four different Identifiers draw four Naks bearing four different
	// Identifiers, which a constant or a counter of the peer's could not produce.
	seen := make(map[uint8]int, len(naks))
	for i, nak := range naks {
		if first, held := seen[nak.Identifier]; held {
			t.Fatalf("Nak %d repeats Identifier %d, first sent as Nak %d", i, nak.Identifier, first)
		}
		seen[nak.Identifier] = i
	}
}

// TestPeerDoesNotNakAMethodError hands the peer a Request of its OWN method Type
// whose payload is truncated: an MS-CHAPv2 Challenge that declares a 16-octet
// challenge and carries four.
//
// A Nak here would be convenient, because it is the one Response the peer can
// always compose. That is exactly what Section 5.3.1 rules out.
func TestPeerDoesNotNakAMethodError(t *testing.T) {
	// RFC requirement: RFC3748-5.3.1-4 positive -- RFC 3748 Section 5.3.1: "Since
	// the legacy Nak Type is valid only in Responses and has very limited
	// functionality, it MUST NOT be used as a general purpose error indication,
	// such as for communication of error messages, or negotiation of parameters
	// specific to a particular EAP method." A malformed Request of the method's own
	// Type reports the method's error and sends no packet, so no Nak carries the
	// failure.
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	if res := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}); res.Response == nil {
		t.Fatalf("the Identity Request drew %+v, want an Identity Response", res)
	}

	truncated := []byte{1, 2, 0, 22, 16, 0, 0, 0, 0}
	res := peer.Process(&Packet{Code: CodeRequest, Identifier: 2, Type: TypeMSCHAPv2, TypeData: truncated})
	if res.Err == nil {
		t.Fatalf("a truncated MS-CHAPv2 Challenge drew %+v, want the method's error", res)
	}
	if res.Response != nil {
		t.Fatalf("a truncated MS-CHAPv2 Challenge drew a Response of Type %d; the error owes no packet", res.Response.Type)
	}

	// RFC requirement: RFC3748-5.3.1-4 negative -- the same peer DOES Nak a Request
	// of an unacceptable authentication Type, so the refusal above is the
	// malformed-payload path acting rather than a peer with no Nak to send.
	offered := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(offered.Close)

	nak := offered.Process(&Packet{Code: CodeRequest, Identifier: 2, Type: 40})
	wantLegacyNak(t, nak, 2, TypeMSCHAPv2, "a Request of Type 40 on a peer that refused a malformed Challenge")
}

// TestRFC3748PeerNaksBeforeItCommitsToAMethod drives the same Type-40 Request on
// each side of the boundary RFC 3748 Section 2.1 draws.
//
// "A peer MUST NOT send a Nak (legacy or expanded) in reply to a Request after an
// initial non-Nak Response has been sent." The initial non-Nak Response is the
// peer's first Response to an authentication METHOD Request, not the Identity
// Response (A-1). Section 5.4 settles it: "The Response MAY be either of Type 4
// (MD5-Challenge), Nak (Type 3), or Expanded Nak (Type 254)" describes a Nak sent
// in answer to the Request that FOLLOWS the Identity Response, so a reading that
// counted the Identity Response would make Section 5.3.1's MUST unreachable in
// every conversation ze can have.
//
// This test replaces the body of TestRFC3748PeerNeverSendsNAK, which asserted
// that ze answers an unexpected Type with an error and never with a Nak.
func TestRFC3748PeerNaksBeforeItCommitsToAMethod(t *testing.T) {
	// RFC requirement: RFC3748-2.1-3 positive -- before the peer has answered a
	// Request of its method's own Type, a Request of Type 40 draws a legacy Nak.
	// The Identity Response has already gone out, so it is not the "initial non-Nak
	// Response" the sentence bounds the Nak with.
	early := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(early.Close)

	if res := early.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}); res.Response == nil {
		t.Fatalf("the Identity Request drew %+v, want an Identity Response", res)
	}
	wantLegacyNak(t, early.Process(&Packet{Code: CodeRequest, Identifier: 2, Type: 40}), 2, TypeMSCHAPv2,
		"a Request of Type 40 before the peer answered a method Request")

	// RFC requirement: RFC3748-2.1-3 negative -- once the peer has answered an
	// MS-CHAPv2 Challenge with an MS-CHAPv2 Response, the SAME Type-40 Request
	// draws no Nak. RFC 3748 Section 2.1: "A peer MUST NOT send a Nak (legacy or
	// expanded) in reply to a Request after an initial non-Nak Response has been
	// sent." It is discarded instead: no Response, no error, and the exchange waits
	// for the next packet.
	committed := nakPeerCommitted(t)
	res := committed.Process(&Packet{Code: CodeRequest, Identifier: 3, Type: 40})
	if !res.Discarded {
		t.Fatalf("a Request of Type 40 after the method started drew %+v, want a silent discard", res)
	}
	if res.Response != nil {
		t.Fatalf("the peer answered with Type %d after it committed to its method; Section 2.1 forbids a Nak here",
			res.Response.Type)
	}
	if res.Err != nil {
		t.Fatalf("the discard carried %v; an error would end the IKE SA on one unauthenticated packet", res.Err)
	}
}

// TestPeerDiscardsAnIdentityRequery sends the Identity Request again once the
// method is under way.
//
// It is the case where the two Section 2.1 sentences meet: the Request must be
// discarded, and the discard must not be a Nak. Type 1 is below the 4-253 range
// and is not 254 or 255, so Section 5.3.1 never reaches it (AC-9).
func TestPeerDiscardsAnIdentityRequery(t *testing.T) {
	peer := nakPeerCommitted(t)

	res := peer.Process(&Packet{Code: CodeRequest, Identifier: 3, Type: TypeIdentity})

	if !res.Discarded {
		t.Fatalf("an Identity Requery drew %+v, want a silent discard", res)
	}
	if res.Response != nil {
		t.Fatalf("an Identity Requery drew a Response of Type %d, want no packet at all", res.Response.Type)
	}
	if res.Err != nil {
		t.Fatalf("the discard carried %v; an error would end the IKE SA on one unauthenticated packet", res.Err)
	}
}

// TestAuthenticatorRecordsTheTypesANakAskedFor drives ze's authenticator with the
// Nak a peer sends, on both of the paths that can receive one.
//
// The exchange ends either way, because ze's authenticator offers exactly one
// method. What the operator needs is the reason, and an EAP-Failure has no field
// to carry one: RFC 3748 Section 4.2 gives it a Code, an Identifier and a Length.
// Session.Err is that field's only stand-in (A-5), and the Types the Nak carried
// are the only word the peer gets to say about why it refused.
func TestAuthenticatorRecordsTheTypesANakAskedFor(t *testing.T) {
	// The peer asks for EAP-TLS and for one-time password, in that order, against
	// an authenticator offering EAP-MSCHAPv2.
	desired := []byte{TypeTLS, 5}

	cases := []struct {
		name string
		// nakIdentifier is the outstanding Request's Identifier at the moment the
		// Nak arrives, which the authenticator checks before it reads the packet.
		nakIdentifier uint8
		openMethod    bool
	}{
		{name: "a Nak answering the Identity Request", nakIdentifier: 1},
		{name: "a Nak answering the method's first Request", nakIdentifier: 2, openMethod: true},
	}

	for _, tc := range cases {
		auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
		if err != nil {
			t.Fatalf("%s: NewSession: %v", tc.name, err)
		}
		t.Cleanup(auth.Close)

		auth.Begin()
		if tc.openMethod {
			opened := auth.Process(&Packet{Code: CodeResponse, Identifier: 1, Type: TypeIdentity, TypeData: []byte("user")})
			if opened == nil || opened.Type != TypeMSCHAPv2 {
				t.Fatalf("%s: the Identity Response drew %+v, want the MS-CHAPv2 Request", tc.name, opened)
			}
		}

		out := auth.Process(&Packet{
			Code:       CodeResponse,
			Identifier: tc.nakIdentifier,
			Type:       TypeNAK,
			TypeData:   desired,
		})

		if out == nil {
			t.Fatalf("%s: the authenticator answered with no packet at all", tc.name)
		}
		if out.Code != CodeFailure {
			t.Fatalf("%s: the authenticator answered code %d, want %d (EAP-Failure)", tc.name, out.Code, CodeFailure)
		}
		if auth.Succeeded() {
			t.Fatalf("%s: the authenticator reports success after a Nak", tc.name)
		}

		reason := auth.Err()
		if reason == nil {
			t.Fatalf("%s: the authenticator recorded no reason, so the operator reads only \"authentication failed\"", tc.name)
		}
		if !strings.Contains(reason.Error(), "type 13, 5") {
			t.Fatalf("%s: Err() = %q, which does not name the Types 13 and 5 the peer asked for", tc.name, reason)
		}
		if !strings.Contains(reason.Error(), "type 26") {
			t.Fatalf("%s: Err() = %q, which does not name the Type 26 that was offered", tc.name, reason)
		}
	}
}
