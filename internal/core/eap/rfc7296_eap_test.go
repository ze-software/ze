// VALIDATES: RFC 7296 Section 3.16 EAP message format for the packets Ze produces.
// It covers the Identifier echo in a response, the four-octet form of Success and
// Failure, and the Type of a response.
// PREVENTS: an EAP response that answers the wrong request, a Success or Failure that
// carries a Type octet, and a response whose Type is neither Nak nor the requested
// type. Each of the three breaks the exchange for a conformant peer.
package eap

import (
	"encoding/binary"
	"testing"
)

// eapfmtNewMSCHAPv2Server returns an authenticator-side MS-CHAPv2 method that builds
// real Challenge and Success packets for the peer under test.
func eapfmtNewMSCHAPv2Server(password string) *mschapv2Method {
	return &mschapv2Method{password: password}
}

// RFC requirement: RFC7296-3.16-1 positive -- the peer copies the request Identifier into
// every response it builds (peer.go:139 for Identity, peer.go:225 for the MS-CHAPv2
// Challenge, peer.go:248 for the MS-CHAPv2 Success acknowledgement). Packet.Encode
// places that value in octet 1 (eap.go:56). The IKE payload that carries those octets
// is asserted in internal/component/ike/wire/payload_eap_carrier_test.go.
// RFC requirement: RFC7296-3.16-1 negative -- the value is read from the request rather than
// counted. The three request identifiers below are 200, 7, and 91, so a round counter
// or a constant fails every round.
func TestEapfmtResponseIdentifierMatchesRequest(t *testing.T) {
	const password = "TestPassword"
	ps := NewPeerSession(TypeMSCHAPv2, "testuser", password)
	server := eapfmtNewMSCHAPv2Server(password)

	// Round 1: Identity request with an identifier no counter would reach first.
	identityReq := &Packet{Code: CodeRequest, Identifier: 200, Type: TypeIdentity}
	res := ps.Process(identityReq)
	if res.Err != nil {
		t.Fatalf("identity round: %v", res.Err)
	}
	if res.Response == nil {
		t.Fatal("identity round produced no response")
	}
	if res.Response.Identifier != identityReq.Identifier {
		t.Fatalf("identity response identifier %d, want %d", res.Response.Identifier, identityReq.Identifier)
	}

	// Round 2: MS-CHAPv2 Challenge with a lower identifier than round 1.
	challenge := server.Start(7)
	if challenge == nil {
		t.Fatal("server built no challenge")
	}
	res = ps.Process(challenge)
	if res.Err != nil {
		t.Fatalf("challenge round: %v", res.Err)
	}
	if res.Response == nil {
		t.Fatal("challenge round produced no response")
	}
	if res.Response.Identifier != challenge.Identifier {
		t.Fatalf("challenge response identifier %d, want %d", res.Response.Identifier, challenge.Identifier)
	}

	// Round 3: MS-CHAPv2 Success with a third unrelated identifier.
	serverRes := server.Process(res.Response)
	if serverRes.Err != nil {
		t.Fatalf("server refused the peer response: %v", serverRes.Err)
	}
	if serverRes.Response == nil {
		t.Fatal("server built no success packet")
	}
	successReq := serverRes.Response
	successReq.Identifier = 91
	res = ps.Process(successReq)
	if res.Err != nil {
		t.Fatalf("success round: %v", res.Err)
	}
	if res.Response == nil {
		t.Fatal("success round produced no response")
	}
	if res.Response.Identifier != successReq.Identifier {
		t.Fatalf("success response identifier %d, want %d", res.Response.Identifier, successReq.Identifier)
	}
}

// RFC requirement: RFC7296-3.16-3 positive -- Packet.Encode gives a Success or a Failure a
// four-octet buffer with the length octets set to 4 (eap.go:45-52). It copies neither
// Type nor TypeData into that buffer. What the IKE payload does with those four octets
// is asserted in internal/component/ike/wire/payload_eap_carrier_test.go.
// RFC requirement: RFC7296-3.16-3 negative -- the four-octet result belongs to the code, and
// the Request below keeps its Type octet.
func TestEapfmtSuccessAndFailureCarryNoTypeField(t *testing.T) {
	for _, code := range []uint8{CodeSuccess, CodeFailure} {
		// Type and TypeData are set on purpose. A conformant encoder drops both.
		p := &Packet{Code: code, Identifier: 42, Type: TypeMSCHAPv2, TypeData: []byte{9, 9, 9}}
		enc := p.Encode()
		if len(enc) != 4 {
			t.Fatalf("code %d: Encode produced %d octets, want 4", code, len(enc))
		}
		if got := binary.BigEndian.Uint16(enc[2:4]); got != 4 {
			t.Fatalf("code %d: EAP Length %d, want 4", code, got)
		}
		if len(enc[4:]) != 0 {
			t.Fatalf("code %d: body forwarded to the payload is %x, want empty", code, enc[4:])
		}
	}

	// A Request keeps the Type octet, so the four-octet rule above is a property of
	// the Success and Failure codes.
	req := &Packet{Code: CodeRequest, Identifier: 42, Type: TypeMSCHAPv2, TypeData: []byte{9, 9, 9}}
	enc := req.Encode()
	if len(enc) != 8 {
		t.Fatalf("request Encode produced %d octets, want 8", len(enc))
	}
	if enc[4] != TypeMSCHAPv2 {
		t.Fatalf("request type octet is %d, want %d", enc[4], TypeMSCHAPv2)
	}
}

// RFC requirement: RFC7296-3.16-4 positive -- every response the peer builds carries the type
// of the request it answers. The Identity branch sets Type to Identity only after an
// Identity request arrives (peer.go:134-143). The MS-CHAPv2 branch sets Type to
// MS-CHAPv2 only after the type guard passes (peer.go:171-173, peer.go:226).
// RFC requirement: RFC7296-3.16-4 negative -- a mismatched request yields no response at all.
// The final round below sends a valid MS-CHAPv2 body under the EAP-TLS type.
func TestEapfmtPeerResponseTypeIsNakOrMatchesRequest(t *testing.T) {
	const password = "TestPassword"
	ps := NewPeerSession(TypeMSCHAPv2, "testuser", password)
	server := eapfmtNewMSCHAPv2Server(password)

	requests := []*Packet{
		{Code: CodeRequest, Identifier: 1, Type: TypeIdentity},
		server.Start(2),
	}
	for _, req := range requests {
		res := ps.Process(req)
		if res.Err != nil {
			t.Fatalf("type %d: %v", req.Type, res.Err)
		}
		if res.Response == nil {
			t.Fatalf("type %d: no response", req.Type)
		}
		if res.Response.Code != CodeResponse {
			t.Fatalf("type %d: code %d, want %d", req.Type, res.Response.Code, CodeResponse)
		}
		if res.Response.Type != TypeNAK && res.Response.Type != req.Type {
			t.Fatalf("response type %d, want Nak (%d) or the requested %d", res.Response.Type, TypeNAK, req.Type)
		}
	}

	// A well-formed MS-CHAPv2 challenge body under the wrong EAP type. Without the
	// type guard the peer would answer it as MS-CHAPv2 and break the obligation.
	wrong := server.Start(3)
	wrong.Type = TypeTLS
	res := ps.Process(wrong)
	if res.Response != nil {
		t.Fatalf("mismatched request drew a response of type %d", res.Response.Type)
	}
	// RFC 3748 Section 2.1 asks the peer to silently discard a Request of a Type
	// other than the one under way. Until 2026-09-01 this drew an error, which
	// the engine reads as a reason to kill the SA, so one unauthenticated packet
	// ended the exchange. The property the case proves is unchanged: the
	// mismatched Request draws no response.
	if !res.Discarded {
		t.Fatal("mismatched request was not discarded")
	}
	if res.Err != nil {
		t.Fatalf("the discard must carry no error, or the packet ends the SA: %v", res.Err)
	}
}
