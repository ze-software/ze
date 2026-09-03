// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP payload (Section 3.16)
// RFC: rfc/short/rfc7296.md -- Section 3.16: the EAP payload IKE_AUTH carries
//
// VALIDATES: the octets an eap.Packet encodes to survive the IKE EAP payload that
// carries them. The response Identifier stays in octet 1, and a Success or a Failure
// still reaches the wire as four octets with a Length of four.
// PREVENTS: an EAP payload writer that renumbers, drops, or pads the octets the EAP
// peer produced. Either fault answers a conformant peer with a packet it refuses.
//
// These two assertions lived in the EAP peer's own package until 2026-09-03, when
// that package moved to internal/core/eap and lost the right to import IKE wire code.
// The EAP-layer half of each assertion stayed with the peer, in
// internal/core/eap/rfc7296_eap_test.go, and the carrier half is here. Nothing is
// tagged here: RFC7296-3.16-1 and RFC7296-3.16-3 keep their positive and negative
// tags on the peer's units, which is where the behavior those requirements bind is
// produced.
package wire

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/core/eap"
)

// eapfmtCarrierOctets writes an encoded EAP packet as an IKE EAP payload the way the
// engine bridges do, and returns those octets. The responder bridge keeps octet 0,
// octet 1, and octet 4 onward (engine/responder_eap.go). The initiator bridge keeps
// octet 1 and octet 4 onward, and it always sets the code to Response (engine/auth.go).
// Neither bridge keeps the two length octets. The bridges are engine code and stay
// outside this test.
func eapfmtCarrierOctets(t *testing.T, p *eap.Packet) []byte {
	t.Helper()
	enc := p.Encode()
	if len(enc) < 4 {
		t.Fatalf("Encode produced %d octets, want at least 4", len(enc))
	}
	payload := &PayloadEAP{Code: enc[0], Identifier: enc[1], EAPData: enc[4:]}
	buf := make([]byte, payload.Len())
	n := payload.WriteTo(buf, 0)
	return buf[:n]
}

// TestEapfmtCarrierKeepsTheResponseIdentifier asserts that the Identifier the peer
// copied from a Request reaches octet 1 of the IKE EAP payload.
//
// The identifier is 200, which no round counter reaches on the first round, so a
// writer that numbered the payload itself fails the check rather than passing it by
// coincidence.
func TestEapfmtCarrierKeepsTheResponseIdentifier(t *testing.T) {
	const password = "TestPassword"
	session := eap.NewPeerSession(eap.TypeMSCHAPv2, "testuser", password)

	identityReq := &eap.Packet{Code: eap.CodeRequest, Identifier: 200, Type: eap.TypeIdentity}
	res := session.Process(identityReq)
	if res.Err != nil {
		t.Fatalf("identity round: %v", res.Err)
	}
	if res.Response == nil {
		t.Fatal("identity round produced no response")
	}

	octets := eapfmtCarrierOctets(t, res.Response)
	if octets[1] != identityReq.Identifier {
		t.Fatalf("wire octet 1 is %d, want %d", octets[1], identityReq.Identifier)
	}
}

// TestEapfmtCarrierSuccessAndFailureStayFourOctets asserts that a Success and a
// Failure reach the wire as four octets carrying a Length of four.
//
// Type and TypeData are set on purpose. The encoder drops both for these two codes,
// and a carrier that appended a body of its own would push the payload past four
// octets and the Length past four.
func TestEapfmtCarrierSuccessAndFailureStayFourOctets(t *testing.T) {
	for _, code := range []uint8{eap.CodeSuccess, eap.CodeFailure} {
		p := &eap.Packet{Code: code, Identifier: 42, Type: eap.TypeMSCHAPv2, TypeData: []byte{9, 9, 9}}
		octets := eapfmtCarrierOctets(t, p)
		if len(octets) != 4 {
			t.Fatalf("code %d: wire payload holds %d octets, want 4", code, len(octets))
		}
		if got := binary.BigEndian.Uint16(octets[2:4]); got != 4 {
			t.Fatalf("code %d: wire EAP Length %d, want 4", code, got)
		}
	}

	// A Request keeps its Type octet, so the four-octet rule above is a property of
	// the Success and Failure codes rather than of the carrier.
	req := &eap.Packet{Code: eap.CodeRequest, Identifier: 42, Type: eap.TypeMSCHAPv2, TypeData: []byte{9, 9, 9}}
	octets := eapfmtCarrierOctets(t, req)
	if len(octets) != 8 {
		t.Fatalf("request wire payload holds %d octets, want 8", len(octets))
	}
	if octets[4] != eap.TypeMSCHAPv2 {
		t.Fatalf("request type octet is %d, want %d", octets[4], eap.TypeMSCHAPv2)
	}
}
